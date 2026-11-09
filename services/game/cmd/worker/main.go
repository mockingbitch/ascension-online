// Command worker drains the transactional outbox and runs scheduled jobs:
//   - leaderboard.dirty → re-read progression from Postgres, ZADD to Redis (coalesced per batch)
//   - announcement.major_breakthrough → (WebSocket fan-out later; logged for now)
//   - periodic leaderboard reconcile: full rebuild from Postgres
//   - world day job: recompute per-day totals, close finished game days (server median), fold into world_state,
//     end ranged events and reach story milestones (world.milestone_reached / world.event_ended → WS)
//   - faction effects right after each world tick: notices, migrate_faction moves, split_faction choice window and
//     default branch (faction.announcement → WS); faction ranking ZSETs refreshed from faction.changed /
//     faction.bulk_changed / leaderboard.dirty and rebuilt on reconcile
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"tiendao/game/internal/announce"
	"tiendao/game/internal/chat"
	"tiendao/game/internal/config"
	"tiendao/game/internal/content"
	"tiendao/game/internal/db"
	"tiendao/game/internal/faction"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/ops"
	"tiendao/game/internal/platform/metrics"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/platform/redisx"
	"tiendao/game/internal/protector"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/realtime"
	"tiendao/game/internal/vip"
	"tiendao/game/internal/world"
)

const batchSize = 200

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("worker exited", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()
	if err := cfg.ValidateWorker(); err != nil {
		return err
	}
	if cfg.DatabaseURL == "" || cfg.RedisURL == "" {
		return errors.New("DATABASE_URL and REDIS_URL are required for worker")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cb, err := content.LoadAll(cfg.ContentDir)
	if err != nil {
		return err
	}
	pool, err := postgres.Connect(ctx, cfg.DatabaseURL, 5)
	if err != nil {
		return err
	}
	defer pool.Close()
	rdb, err := redisx.Connect(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer rdb.Close()

	tpl, err := realtime.LoadTemplates(filepath.Join(cfg.StoryDir, "announcements", "major-breakthrough.yaml"))
	if err != nil {
		return err
	}
	worldTpl, err := realtime.LoadTemplates(filepath.Join(cfg.StoryDir, "announcements", "world-milestones.yaml"))
	if err != nil {
		return err
	}
	// Every milestone announce key and world.event_end.<event> must have a template (§5.7 worker-start check).
	if err := cb.World.CheckAnnouncements(func(k string) bool { return len(worldTpl[k]) > 0 }); err != nil {
		return err
	}
	for _, k := range faction.AnnouncementKeys() {
		if len(worldTpl[k]) == 0 {
			return errors.New("faction: announcement template " + k + " missing")
		}
	}
	// World announcements by effect tier (GĐ-D): every key present, every line ≤ max_chars after substitution.
	if err := announce.CheckTemplates(tpl, cb.Realms, cb.Announce); err != nil {
		return err
	}
	annTpl := realtime.Templates{} // its own copy: the world/faction templates merged below are never checked against it
	for k, v := range tpl {
		annTpl[k] = v
	}
	ann := &announce.Service{Pool: pool, Redis: rdb, Tpl: annTpl, Realms: cb.Realms, Cfg: cb.Announce, Now: time.Now, Log: log}
	for k, v := range worldTpl {
		tpl[k] = v
	}
	w := &worker{log: log, rdb: rdb, realms: cb.Realms, tpl: tpl, lb: &leaderboard.Service{Pool: pool, Redis: rdb, Realms: cb.Realms},
		world:    &world.Service{Pool: pool, Realms: cb.Realms, Cat: cb.World, Contribution: cb.Contribution, Log: log, Now: time.Now},
		chat:     &chat.Service{Pool: pool, Cat: cb.Chat, Realms: cb.Realms, Mod: cb.Moderation, Now: time.Now},
		effects:  &faction.Effects{Pool: pool, Cat: cb.Factions, World: cb.World, Log: log, Now: time.Now},
		factions: &faction.Ranking{Pool: pool, Redis: rdb, Realms: cb.Realms, Cat: cb.Factions, CapMilli: cb.World.Settings.WorldCreditCapMilli},
		fcat:     cb.Factions, vip: cb.VIP, ann: ann}

	// Metrics on an internal port (METRICS_ADDR, default :9091; "off" disables). Never routed by nginx.
	reg := metrics.NewRegistry()
	w.m = metrics.NewWorker(reg)
	metrics.RegisterPool(reg, pool)
	metrics.RegisterRedis(reg, rdb)
	metrics.Serve(ctx, envOr("METRICS_ADDR", ":9091"), reg, log)
	w.ledger = &ops.LedgerScanner{Pool: pool}
	w.probe(ctx)

	paySweep, err := newPaymentSweep(cfg, cb, pool, log) // nạp Tiên Duyên: expiry + reconciliation (payment.go)
	if err != nil {
		return err
	}
	defer paySweep.Stop()
	social := newSocialSweep(cb, pool, log) // netsig 30-day purge + đạo lữ absence endings (daolu.go)
	trades := newTradeSweep(cb, pool, log)  // lapsed trade offers → expired (trade.go)

	// Rebuild once at start: Redis may be empty after a restart, and a catalogue retune changes scores.
	w.reconcile(ctx)
	w.worldTick(ctx)

	worldTick := time.NewTicker(cfg.WorldInterval)
	defer worldTick.Stop()
	drain := time.NewTicker(cfg.WorkerInterval)
	defer drain.Stop()
	reconcile := time.NewTicker(cfg.ReconcileInterval)
	defer reconcile.Stop()
	probe := time.NewTicker(15 * time.Second) // outbox backlog gauges + ledger flow counters (one small query each)
	defer probe.Stop()
	log.Info("worker started", "interval", cfg.WorkerInterval, "reconcile", cfg.ReconcileInterval)
	for {
		select {
		case <-ctx.Done():
			log.Info("worker stopping")
			return nil
		case <-reconcile.C:
			w.reconcile(ctx)
			if n, err := w.ann.Prune(ctx); err != nil {
				log.Warn("announcement prune", "err", err)
			} else if n > 0 {
				log.Info("announcements pruned", "rows", n)
			}
			w.chatSweep(ctx)
			social.run(ctx)
			trades.run(ctx)
		case <-worldTick.C:
			w.worldTick(ctx)
		case <-paySweep.C:
			paySweep.run(ctx)
		case <-probe.C:
			w.probe(ctx)
		case <-drain.C:
			// Digests of finished announcement windows go out on the drain cadence (best effort).
			if err := w.ann.FlushDigests(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Warn("announcement digest", "err", err)
			}
			// Drain until the queue is empty, then wait for the next tick.
			for {
				start := time.Now()
				n, err := w.drainOnce(ctx)
				if n > 0 || err != nil {
					w.m.Job("outbox_drain", start, err)
				}
				if err != nil {
					if !errors.Is(err, context.Canceled) {
						log.Error("outbox drain", "err", err)
					}
					break
				}
				if n < batchSize {
					break
				}
			}
		}
	}
}

type worker struct {
	log    *slog.Logger
	lb     *leaderboard.Service
	rdb    *redis.Client
	realms *realm.Catalogue
	tpl    realtime.Templates
	world  *world.Service
	chat   *chat.Service

	effects  *faction.Effects
	factions *faction.Ranking
	fcat     *faction.Catalogue
	vip      *vip.Table        // protector_assist điểm duyên (protector.assist_counted)
	ann      *announce.Service // world announcements: render, pace (digest), history, publish

	m      *metrics.Worker    // nil-safe
	ledger *ops.LedgerScanner // economy flow counters
}

// probe refreshes the outbox backlog gauges and folds new ledger rows into the economy flow counters.
func (w *worker) probe(ctx context.Context) {
	start := time.Now()
	n, age, err := ops.OutboxBacklog(ctx, w.lb.Pool)
	if err == nil {
		w.m.Outbox(n, age)
		err = w.ledger.Scan(ctx, w.m)
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	w.m.Job("probe", start, err)
	if err != nil {
		w.log.Error("metrics probe", "err", err)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// chatSweep deletes chat messages past chat.yaml retention_days. Idempotent; concurrent workers only race on
// already-deleted rows.
func (w *worker) chatSweep(ctx context.Context) {
	start := time.Now()
	n, err := w.chat.Sweep(ctx)
	if !errors.Is(err, context.Canceled) {
		w.m.Job("chat_sweep", start, err)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		w.log.Error("chat sweep", "err", err)
		return
	}
	if n > 0 {
		w.log.Info("chat sweep", "deleted", n)
	}
}

func (w *worker) worldTick(ctx context.Context) {
	start := time.Now()
	res, err := w.world.Tick(ctx)
	if !errors.Is(err, context.Canceled) {
		w.m.Job("world_tick", start, err)
	}
	switch {
	case errors.Is(err, context.Canceled):
		return
	case err != nil:
		// Keep going: a failing tick must not hold faction effects back (their windows would shrink, review H1).
		w.log.Error("world tick", "err", err)
	case res.Closed > 0 || res.Violations > 0 || res.EventsEnded > 0 || res.Reached != "":
		w.log.Info("world tick", "closed", res.Closed, "violations", res.Violations, "eventsEnded", res.EventsEnded, "reached", res.Reached)
	}
	// Faction effects run right after the tick (same advisory lock, one short tx per bounded step).
	start = time.Now()
	fx, err := w.effects.Run(ctx)
	if !errors.Is(err, context.Canceled) {
		w.m.Job("faction_effects", start, err)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		w.log.Error("faction effects", "err", err)
	} else if fx.Steps > 0 {
		w.log.Info("faction effects", "steps", fx.Steps, "notices", fx.Notices, "pending", fx.Pending, "moved", fx.Moved,
			"applied", fx.Applied, "finalized", fx.Finalized)
	}
	// Faction ranking rebuilds requested by the outbox drain (bulk moves, member counts) run here, outside the drain
	// transaction, under the reconcile lock (review M2).
	w.withReconcileLock(ctx, func() {
		start := time.Now()
		did, err := w.factions.RunRequested(ctx)
		if did != "" || err != nil {
			w.m.Job("faction_ranking", start, err)
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			w.log.Error("faction ranking rebuild", "err", err)
		} else if did != "" {
			w.log.Info("faction ranking rebuilt", "scope", did)
		}
	})
}

// announceFaction publishes faction.announcement after commit (outbox drain), text from world-milestones.yaml.
func (w *worker) announceFaction(ctx context.Context, eventID int64, payload []byte) {
	var p faction.AnnouncementEvent
	if err := json.Unmarshal(payload, &p); err != nil {
		w.log.Warn("bad faction announcement payload", "id", eventID, "err", err)
		w.m.DeadLetter(faction.TopicAnnouncement)
		return
	}
	target := ""
	if len(p.TargetNames) > 0 {
		target = p.TargetNames[0]
	}
	text := w.tpl.Render(p.Key, map[string]string{"faction": p.FromName, "branches": strings.Join(p.TargetNames, ", "),
		"target": target, "days": strconv.Itoa(p.Days)})
	w.publish(ctx, realtime.Envelope{Type: faction.TopicAnnouncement, ID: strconv.FormatInt(eventID, 10), TS: time.Now().UTC(),
		Data: map[string]any{"key": p.Key, "kind": p.Kind, "from": p.From, "targets": p.Targets, "dueAt": p.DueAt,
			"deadlineAt": p.DeadlineAt, "text": text}})
}

// announceMilestone publishes world.milestone_reached after commit (outbox drain), text from world-milestones.yaml.
func (w *worker) announceMilestone(ctx context.Context, eventID int64, payload []byte) {
	var p world.MilestoneReachedEvent
	if err := json.Unmarshal(payload, &p); err != nil {
		w.log.Warn("bad milestone payload", "id", eventID, "err", err)
		w.m.DeadLetter(world.TopicMilestoneReached)
		return
	}
	text := w.tpl.Render(p.Announce, map[string]string{"day": strconv.Itoa(p.ServerDay), "era": p.EraName})
	w.publish(ctx, realtime.Envelope{Type: world.TopicMilestoneReached, ID: strconv.FormatInt(eventID, 10), TS: time.Now().UTC(),
		Data: map[string]any{"milestoneId": p.MilestoneID, "name": p.Name, "seq": p.Seq, "reason": p.Reason, "serverDay": p.ServerDay,
			"eraId": p.EraID, "eraName": p.EraName, "text": text}})
}

// announceEventEnd publishes world.event_ended after commit (outbox drain).
func (w *worker) announceEventEnd(ctx context.Context, eventID int64, payload []byte) {
	var p world.EventEndedEvent
	if err := json.Unmarshal(payload, &p); err != nil {
		w.log.Warn("bad event-end payload", "id", eventID, "err", err)
		w.m.DeadLetter(world.TopicEventEnded)
		return
	}
	text := w.tpl.Render(p.Announce, map[string]string{"day": strconv.Itoa(p.ServerDay), "era": p.EraName})
	w.publish(ctx, realtime.Envelope{Type: world.TopicEventEnded, ID: strconv.FormatInt(eventID, 10), TS: time.Now().UTC(),
		Data: map[string]any{"eventId": p.EventID, "milestoneId": p.MilestoneID, "serverDay": p.ServerDay, "eraId": p.EraID,
			"eraName": p.EraName, "text": text}})
}

func (w *worker) publish(ctx context.Context, e realtime.Envelope) {
	if err := realtime.Publish(ctx, w.rdb, e); err != nil {
		w.log.Warn("publish announcement", "type", e.Type, "err", err)
	}
}

// reconcileLockID is the Postgres advisory lock that makes reconcile single-runner across workers.
const reconcileLockID = 7_310_001

// withReconcileLock runs fn while holding the reconcile advisory lock; it skips fn when another worker holds it.
func (w *worker) withReconcileLock(ctx context.Context, fn func()) {
	conn, err := w.lb.Pool.Acquire(ctx)
	if err != nil {
		w.log.Error("reconcile lock: acquire", "err", err)
		return
	}
	defer conn.Release()
	var got bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", reconcileLockID).Scan(&got); err != nil || !got {
		return // another worker is reconciling
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", reconcileLockID) //nolint:errcheck
	fn()
}

func (w *worker) reconcile(ctx context.Context) {
	w.withReconcileLock(ctx, func() {
		start := time.Now()
		n, err := w.lb.Rebuild(ctx)
		if err != nil {
			w.m.Job("reconcile", start, err)
			w.log.Error("leaderboard reconcile", "err", err)
			return
		}
		f, err := w.factions.Rebuild(ctx)
		w.m.Job("reconcile", start, err)
		if err != nil {
			w.log.Error("faction ranking reconcile", "err", err)
			return
		}
		w.log.Info("leaderboard reconciled", "entries", n, "factionMembers", f, "dur", time.Since(start))
	})
}

// drainOnce claims a batch with SKIP LOCKED, handles it and marks it published in one tx.
// Handlers MUST be idempotent: a crash after side effects but before commit replays the batch.
// (Leaderboard refresh re-reads Postgres, so a replay can only write the current — correct — score.)
func (w *worker) drainOnce(ctx context.Context) (int, error) {
	var n int
	var topics []string
	// world news is announced only after this batch commits (review H1/M1): collected here, run below
	type annEvent struct {
		id      int64
		payload []byte
	}
	var annEvents []annEvent
	var rankMoves []announce.RankMove
	err := postgres.InTx(ctx, w.lb.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		events, err := q.ClaimPendingOutbox(ctx, batchSize)
		if err != nil || len(events) == 0 {
			return err
		}
		dirty := map[uuid.UUID]struct{}{}
		factionDirty := map[uuid.UUID]struct{}{} // membership changed: per-faction ZSETs + counts
		bulk := false                            // a world effect moved a batch: rebuild the faction ranking
		ids := make([]int64, 0, len(events))
		topics = make([]string, 0, len(events))
		var assists []protector.AssistCountedEvent // credited in this tx, sorted by user (vip_accounts lock order)
		for _, e := range events {
			switch e.Topic {
			case "leaderboard.dirty":
				if id, err := uuid.Parse(e.AggregateID); err == nil {
					dirty[id] = struct{}{}
				}
			case "announcement.major_breakthrough":
				// Best effort, at-most-once fan-out: a lost announcement is acceptable, a duplicate is noise. The history
				// row is written in this transaction (idempotent per event), the publish follows it.
				annEvents = append(annEvents, annEvent{e.ID, e.Payload})
			case world.TopicMilestoneReached:
				w.announceMilestone(ctx, e.ID, e.Payload)
			case world.TopicEventEnded:
				w.announceEventEnd(ctx, e.ID, e.Payload)
			case faction.TopicChanged:
				if id, err := uuid.Parse(e.AggregateID); err == nil {
					factionDirty[id] = struct{}{}
				}
			case faction.TopicBulkChanged:
				bulk = true
			case protector.TopicAssistCounted:
				var ev protector.AssistCountedEvent
				if err := json.Unmarshal(e.Payload, &ev); err != nil {
					w.log.Warn("bad protector payload", "id", e.ID, "err", err)
					w.m.DeadLetter(e.Topic)
				} else {
					assists = append(assists, ev)
				}
			case faction.TopicAnnouncement:
				w.announceFaction(ctx, e.ID, e.Payload)
			default:
				w.log.Warn("unknown outbox topic", "topic", e.Topic, "id", e.ID)
				w.m.DeadLetter("unknown")
			}
			ids = append(ids, e.ID)
			topics = append(topics, e.Topic)
		}
		if len(dirty) > 0 {
			list := make([]uuid.UUID, 0, len(dirty))
			for id := range dirty {
				list = append(list, id)
			}
			old := w.ranks(ctx, list)
			if err := w.lb.Refresh(ctx, list); err != nil {
				return err // keep events pending; retried next tick
			}
			// Thiên Bảng news: entering the top / reaching rank 1 (≤ once per player per cooldown), after commit
			rankMoves = w.rankMoves(ctx, q, list, old)
		}
		if err := w.refreshFactions(ctx, dirty, factionDirty, bulk); err != nil {
			return err
		}
		// Protector điểm duyên LAST, after every Redis call of this batch: vip_accounts row locks are held only until
		// the commit right below (no external call while holding them). Per-event savepoints; permanent failures are
		// logged and skipped, transient ones keep the batch pending (EarnLocked is idempotent per assist).
		if len(assists) > 0 && w.vip != nil {
			dead, err := protector.RewardBatch(ctx, tx, w.vip, assists, w.log)
			if err != nil {
				return err
			}
			for range dead {
				w.m.DeadLetter(protector.TopicAssistCounted)
			}
		}
		n = len(ids)
		return q.MarkOutboxPublished(ctx, ids)
	})
	if err == nil {
		for _, t := range topics {
			w.m.Handled(t)
		}
		// committed: now the news (own statements, best effort — a failure never touches the outbox)
		aq := db.New(w.lb.Pool)
		for _, a := range annEvents {
			if aerr := w.ann.Breakthrough(ctx, aq, a.id, a.payload); aerr != nil {
				w.log.Warn("announcement", "id", a.id, "err", aerr)
				w.m.DeadLetter("announcement.major_breakthrough")
			}
		}
		if aerr := w.ann.Rank(ctx, aq, rankMoves); aerr != nil {
			w.log.Warn("rank announcement", "err", aerr)
		}
	}
	return n, err
}

// ranks reads the current Thiên Bảng rank of each character (0 = unranked) before a refresh.
func (w *worker) ranks(ctx context.Context, ids []uuid.UUID) map[uuid.UUID]int64 {
	out := make(map[uuid.UUID]int64, len(ids))
	for _, id := range ids {
		if r, _, err := w.lb.RankOf(ctx, id); err == nil {
			out[id] = r
		}
	}
	return out
}

// rankMoves pairs the ranks before and after a refresh with each character's đạo hiệu (display only).
func (w *worker) rankMoves(ctx context.Context, q *db.Queries, ids []uuid.UUID, old map[uuid.UUID]int64) []announce.RankMove {
	now := w.ranks(ctx, ids)
	var moved []uuid.UUID
	for _, id := range ids {
		if announce.RankNews(old[id], now[id], w.ann.Cfg.RankTop) != "" {
			moved = append(moved, id)
		}
	}
	if len(moved) == 0 {
		return nil
	}
	rows, err := q.GetLeaderboardRows(ctx, moved)
	if err != nil {
		return nil
	}
	out := make([]announce.RankMove, 0, len(rows))
	for _, r := range rows {
		out = append(out, announce.RankMove{CharacterID: r.ID, Name: r.Name, Old: old[r.ID], New: now[r.ID]})
	}
	return out
}

// refreshFactions updates the faction caches for one drained batch (replay-safe): per-character member ZSET moves
// re-read Postgres; member counts and bulk moves only REQUEST a rebuild, done by the world ticker (review M2).
func (w *worker) refreshFactions(ctx context.Context, dirty, factionDirty map[uuid.UUID]struct{}, bulk bool) error {
	if bulk {
		return w.factions.RequestRebuild(ctx, true)
	}
	list := make([]uuid.UUID, 0, len(dirty)+len(factionDirty))
	for id := range dirty {
		list = append(list, id)
	}
	for id := range factionDirty {
		if _, ok := dirty[id]; !ok {
			list = append(list, id)
		}
	}
	if err := w.factions.RefreshMembers(ctx, list); err != nil {
		return err
	}
	if len(factionDirty) > 0 {
		return w.factions.RequestRebuild(ctx, false)
	}
	return nil
}
