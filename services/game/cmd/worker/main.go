// Command worker drains the transactional outbox and runs scheduled jobs:
//   - leaderboard.dirty → re-read progression from Postgres, ZADD to Redis (coalesced per batch)
//   - announcement.major_breakthrough → (WebSocket fan-out later; logged for now)
//   - periodic leaderboard reconcile: full rebuild from Postgres
//   - world day job: recompute per-day totals, close finished game days (server median), fold into world_state
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
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"tiendao/game/internal/chat"
	"tiendao/game/internal/config"
	"tiendao/game/internal/content"
	"tiendao/game/internal/db"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/platform/redisx"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/realtime"
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
	w := &worker{log: log, rdb: rdb, realms: cb.Realms, tpl: tpl, lb: &leaderboard.Service{Pool: pool, Redis: rdb, Realms: cb.Realms},
		world: &world.Service{Pool: pool, Realms: cb.Realms, Cat: cb.World, Contribution: cb.Contribution, Log: log, Now: time.Now},
		chat:  &chat.Service{Pool: pool, Cat: cb.Chat, Realms: cb.Realms, Mod: cb.Moderation, Now: time.Now}}

	// Rebuild once at start: Redis may be empty after a restart, and a catalogue retune changes scores.
	w.reconcile(ctx)
	w.worldTick(ctx)

	worldTick := time.NewTicker(cfg.WorldInterval)
	defer worldTick.Stop()
	drain := time.NewTicker(cfg.WorkerInterval)
	defer drain.Stop()
	reconcile := time.NewTicker(cfg.ReconcileInterval)
	defer reconcile.Stop()
	log.Info("worker started", "interval", cfg.WorkerInterval, "reconcile", cfg.ReconcileInterval)
	for {
		select {
		case <-ctx.Done():
			log.Info("worker stopping")
			return nil
		case <-reconcile.C:
			w.reconcile(ctx)
			w.chatSweep(ctx)
		case <-worldTick.C:
			w.worldTick(ctx)
		case <-drain.C:
			// Drain until the queue is empty, then wait for the next tick.
			for {
				n, err := w.drainOnce(ctx)
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
}

// chatSweep deletes chat messages past chat.yaml retention_days. Idempotent; concurrent workers only race on
// already-deleted rows.
func (w *worker) chatSweep(ctx context.Context) {
	n, err := w.chat.Sweep(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		w.log.Error("chat sweep", "err", err)
		return
	}
	if n > 0 {
		w.log.Info("chat sweep", "deleted", n)
	}
}

func (w *worker) worldTick(ctx context.Context) {
	res, err := w.world.Tick(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		w.log.Error("world tick", "err", err)
		return
	}
	if res.Closed > 0 || res.Violations > 0 {
		w.log.Info("world days closed", "closed", res.Closed, "violations", res.Violations)
	}
}

// reconcileLockID is the Postgres advisory lock that makes reconcile single-runner across workers.
const reconcileLockID = 7_310_001

func (w *worker) reconcile(ctx context.Context) {
	conn, err := w.lb.Pool.Acquire(ctx)
	if err != nil {
		w.log.Error("leaderboard reconcile: acquire", "err", err)
		return
	}
	defer conn.Release()
	var got bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", reconcileLockID).Scan(&got); err != nil || !got {
		return // another worker is reconciling
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", reconcileLockID) //nolint:errcheck

	start := time.Now()
	n, err := w.lb.Rebuild(ctx)
	if err != nil {
		w.log.Error("leaderboard reconcile", "err", err)
		return
	}
	w.log.Info("leaderboard reconciled", "entries", n, "dur", time.Since(start))
}

// drainOnce claims a batch with SKIP LOCKED, handles it and marks it published in one tx.
// Handlers MUST be idempotent: a crash after side effects but before commit replays the batch.
// (Leaderboard refresh re-reads Postgres, so a replay can only write the current — correct — score.)
func (w *worker) drainOnce(ctx context.Context) (int, error) {
	var n int
	err := postgres.InTx(ctx, w.lb.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		events, err := q.ClaimPendingOutbox(ctx, batchSize)
		if err != nil || len(events) == 0 {
			return err
		}
		dirty := map[uuid.UUID]struct{}{}
		ids := make([]int64, 0, len(events))
		for _, e := range events {
			switch e.Topic {
			case "leaderboard.dirty":
				if id, err := uuid.Parse(e.AggregateID); err == nil {
					dirty[id] = struct{}{}
				}
			case "announcement.major_breakthrough":
				// Best effort, at-most-once fan-out: a lost announcement is acceptable, a duplicate is noise.
				w.announce(ctx, e.ID, e.Payload)
			default:
				w.log.Warn("unknown outbox topic", "topic", e.Topic, "id", e.ID)
			}
			ids = append(ids, e.ID)
		}
		if len(dirty) > 0 {
			list := make([]uuid.UUID, 0, len(dirty))
			for id := range dirty {
				list = append(list, id)
			}
			if err := w.lb.Refresh(ctx, list); err != nil {
				return err // keep events pending; retried next tick
			}
		}
		n = len(ids)
		return q.MarkOutboxPublished(ctx, ids)
	})
	return n, err
}

func (w *worker) announce(ctx context.Context, eventID int64, payload []byte) {
	var p struct {
		CharacterID string `json:"characterId"`
		Name        string `json:"name"`
		RealmID     string `json:"realmId"`
		StageID     string `json:"stageId"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		w.log.Warn("bad announcement payload", "id", eventID, "err", err)
		return
	}
	realmName := p.RealmID
	for _, r := range w.realms.Realms() {
		if r.ID == p.RealmID {
			realmName = r.Name
		}
	}
	text := w.tpl.Render("major_breakthrough", map[string]string{"name": p.Name, "realm": realmName})
	err := realtime.Publish(ctx, w.rdb, realtime.Envelope{
		Type: "announcement.major_breakthrough", ID: strconv.FormatInt(eventID, 10), TS: time.Now().UTC(),
		Data: map[string]string{"characterId": p.CharacterID, "characterName": p.Name, "realmId": p.RealmID, "stageId": p.StageID, "text": text},
	})
	if err != nil {
		w.log.Warn("publish announcement", "err", err)
	}
}
