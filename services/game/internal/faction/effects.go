package faction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/world"
)

// Outbox topics written by the effect runner.
const (
	// TopicAnnouncement carries a faction.* announcement (rendered by the worker drain into a WS envelope).
	TopicAnnouncement = "faction.announcement"
	// TopicBulkChanged means a batch of members moved; the drain rebuilds the faction ranking once.
	TopicBulkChanged = "faction.bulk_changed"
)

// Announcement template keys (content/story/announcements/world-milestones.yaml).
const (
	KeySplitNotice   = "faction.split_notice"
	KeySplitDone     = "faction.split_done"
	KeySplitFinal    = "faction.split_final"
	KeyMigrateNotice = "faction.migrate_notice"
	KeyMigrateDone   = "faction.migrate_done"
)

// AnnouncementKeys lists every template key the runner may emit (worker-start check).
func AnnouncementKeys() []string {
	return []string{KeySplitNotice, KeySplitDone, KeySplitFinal, KeyMigrateNotice, KeyMigrateDone}
}

// AnnouncementEvent is the outbox payload of faction.announcement.
type AnnouncementEvent struct {
	Key         string           `json:"key"`
	Kind        world.EffectKind `json:"kind"`
	EffectID    int64            `json:"effectId"`
	From        string           `json:"from"`
	FromName    string           `json:"fromName"`
	Targets     []string         `json:"targets"`     // branches (split) or the new faction (migrate/default)
	TargetNames []string         `json:"targetNames"` // display names, same order
	Days        int              `json:"days"`
	DueAt       time.Time        `json:"dueAt"`
	DeadlineAt  *time.Time       `json:"deadlineAt,omitempty"`
}

// Defaults for the runner's bounds.
const (
	DefaultBatch    = 500
	DefaultMaxSteps = 40
)

// Effects executes the faction world effects (data model §5.6 step 5): notices, migrate_faction moves, split_faction
// pending marks and the default assignment after the deadline (each undecided member to the branch with the fewest
// members placed by this split so far, AssignDefaults). Each step is one short tx under the world
// advisory lock (single runner with the world tick), works on ≤ Batch members and is replay-safe: rows move only
// from the source/pending state, history keys are 'effect:<id>', and noticed_at / applied_at / finalized_at are each written
// once (W8); defaults are recomputed per batch from this split's own history rows, so a replay assigns the same way.
type Effects struct {
	Pool     *pgxpool.Pool
	Cat      *Catalogue
	World    *world.Catalogue // eras: a split's branches already closed by the current era are skipped (OpenBranches)
	Log      *slog.Logger
	Now      func() time.Time
	Batch    int // members per step (0 → DefaultBatch)
	MaxSteps int // steps per Run (0 → DefaultMaxSteps); the rest continues on the next run
}

// EffectsResult reports what one Run did.
type EffectsResult struct {
	Steps     int
	Notices   int   // notices announced
	Moved     int64 // members moved (migrate + split default)
	Pending   int64 // members marked as having to choose
	Applied   int   // effects applied (migrate done / split choice opened)
	Finalized int   // splits finalized
}

// Run performs up to MaxSteps steps and stops when nothing is due.
func (e *Effects) Run(ctx context.Context) (EffectsResult, error) {
	var res EffectsResult
	maxSteps := e.MaxSteps
	if maxSteps <= 0 {
		maxSteps = DefaultMaxSteps
	}
	for res.Steps < maxSteps {
		did, err := e.step(ctx, &res)
		if err != nil || !did {
			return res, err
		}
		res.Steps++
	}
	return res, nil
}

func (e *Effects) batch() int32 {
	if e.Batch > 0 {
		return int32(e.Batch)
	}
	return DefaultBatch
}

func (e *Effects) log() *slog.Logger {
	if e.Log != nil {
		return e.Log
	}
	return slog.Default()
}

func (e *Effects) name(id string) string {
	if f, ok := e.Cat.Get(id); ok {
		return f.Name
	}
	return id
}

// step claims the next effect with work at now and does one bounded piece of it. false = nothing to do (or another
// runner holds the lock).
func (e *Effects) step(ctx context.Context, res *EffectsResult) (bool, error) {
	did := false
	err := postgres.InTx(ctx, e.Pool, func(tx pgx.Tx) error {
		var locked bool
		if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", world.LockID).Scan(&locked); err != nil || !locked {
			return err
		}
		q := db.New(tx)
		now := e.Now().UTC()
		row, err := q.ClaimFactionEffect(ctx, now)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		} else if err != nil {
			return fmt.Errorf("claim faction effect: %w", err)
		}
		did = true
		var eff world.Effect
		if err := json.Unmarshal(row.Snapshot, &eff); err != nil {
			return fmt.Errorf("effect %d snapshot: %w", row.ID, err)
		}
		ann := AnnouncementEvent{Kind: eff.Kind, EffectID: row.ID, From: eff.From, FromName: e.name(eff.From), DueAt: row.DueAt, DeadlineAt: row.DeadlineAt}
		into := eff.Into
		if eff.Kind == world.EffectSplitFaction {
			if into, err = e.openBranches(ctx, q, eff.Into); err != nil {
				return err
			}
		}
		day := 24 * time.Hour
		switch {
		case row.NoticedAt == nil:
			if err := q.MarkWorldEffectNoticed(ctx, db.MarkWorldEffectNoticedParams{NoticedAt: &now, ID: row.ID}); err != nil {
				return err
			}
			if eff.NoticeDays == 0 {
				return nil // no notice period: straight to the move
			}
			// A late notice (outage, failing tick) still gets its full period, and the choice window follows (H1).
			if due := now.Add(time.Duration(eff.NoticeDays) * day); due.After(row.DueAt) {
				var deadline *time.Time
				if row.DeadlineAt != nil {
					d := due.Add(time.Duration(eff.ChooseDays) * day)
					deadline = &d
				}
				if err := q.RescheduleWorldEffect(ctx, db.RescheduleWorldEffectParams{DueAt: due, DeadlineAt: deadline, ID: row.ID}); err != nil {
					return err
				}
				ann.DueAt, ann.DeadlineAt = due, deadline
				e.log().Warn("faction effect notice late; rescheduled", "effectId", row.ID, "dueAt", due)
			}
			res.Notices++
			ann.Key, ann.Days = KeySplitNotice, eff.NoticeDays
			ann.Targets = into
			if eff.Kind == world.EffectMigrateFaction {
				ann.Key, ann.Targets = KeyMigrateNotice, []string{eff.To}
			}
			return e.announce(ctx, q, ann)

		case row.AppliedAt == nil && eff.Kind == world.EffectMigrateFaction:
			n, err := q.MigrateFactionBatch(ctx, db.MigrateFactionBatchParams{FromFaction: eff.From, ToFaction: eff.To, EffectID: row.ID, Now: now, Lim: e.batch()})
			if err != nil {
				return fmt.Errorf("migrate batch: %w", err)
			}
			res.Moved += n
			if err := e.bulk(ctx, q, row.ID, n); err != nil {
				return err
			}
			if n >= int64(e.batch()) {
				return nil // more to move next step
			}
			if err := q.MarkWorldEffectApplied(ctx, db.MarkWorldEffectAppliedParams{AppliedAt: &now, ID: row.ID}); err != nil {
				return err
			}
			res.Applied++
			ann.Key, ann.Targets = KeyMigrateDone, []string{eff.To}
			e.log().Info("faction migrated", "effectId", row.ID, "from", eff.From, "to", eff.To)
			return e.announce(ctx, q, ann)

		case row.AppliedAt == nil: // split_faction due: everyone in the source must choose before the deadline
			n, err := q.SetPendingSplitBatch(ctx, db.SetPendingSplitBatchParams{FromFaction: eff.From, EffectID: pgtype.Int8{Int64: row.ID, Valid: true}, Now: now, Lim: e.batch()})
			if err != nil {
				return fmt.Errorf("split pending batch: %w", err)
			}
			res.Pending += n
			if n >= int64(e.batch()) {
				return nil
			}
			// The window opens now: never shorter than choose_days, whatever the delay (H1).
			minDeadline := now.Add(time.Duration(eff.ChooseDays) * day)
			if err := q.MarkWorldEffectApplied(ctx, db.MarkWorldEffectAppliedParams{AppliedAt: &now, MinDeadlineAt: &minDeadline, ID: row.ID}); err != nil {
				return err
			}
			if row.DeadlineAt == nil || minDeadline.After(*row.DeadlineAt) {
				ann.DeadlineAt = &minDeadline
			}
			res.Applied++
			ann.Key, ann.Targets, ann.Days = KeySplitDone, into, eff.ChooseDays
			e.log().Info("faction split opened", "effectId", row.ID, "from", eff.From, "deadline", ann.DeadlineAt)
			return e.announce(ctx, q, ann)

		default: // split past its deadline: spread everyone still pending over the branches (F8, H2)
			pending, err := q.ListPendingSplitBatch(ctx, db.ListPendingSplitBatchParams{EffectID: pgtype.Int8{Int64: row.ID, Valid: true}, Lim: e.batch()})
			if err != nil {
				return fmt.Errorf("split pending list: %w", err)
			}
			placedRows, err := q.CountSplitPlacements(ctx, pgtype.Int8{Int64: row.ID, Valid: true})
			if err != nil {
				return err
			}
			placed := map[string]int64{}
			for _, p := range placedRows {
				placed[p.ToFactionID] = p.Placed
			}
			targets := AssignDefaults(into, placed, len(pending)) // only branches still open in the current era
			ids := make([]uuid.UUID, len(pending))
			used := map[string]bool{}
			for i, p := range pending {
				ids[i] = p.CharacterID
				used[targets[i]] = true
			}
			var n int64
			if len(pending) > 0 {
				if n, err = q.AssignSplitDefaults(ctx, db.AssignSplitDefaultsParams{CharacterIds: ids, ToFactions: targets, Now: now,
					EffectID: row.ID, FromFaction: eff.From}); err != nil {
					return fmt.Errorf("split default batch: %w", err)
				}
			}
			res.Moved += n
			if err := e.bulk(ctx, q, row.ID, n); err != nil {
				return err
			}
			if len(pending) >= int(e.batch()) {
				return nil
			}
			if err := q.MarkWorldEffectFinalized(ctx, db.MarkWorldEffectFinalizedParams{FinalizedAt: &now, ID: row.ID}); err != nil {
				return err
			}
			res.Finalized++
			// Branches that received anyone at all, in catalogue order (the final batch's view: placed + this batch).
			ann.Key = KeySplitFinal
			for _, b := range eff.Into {
				if used[b] || placed[b] > 0 {
					ann.Targets = append(ann.Targets, b)
				}
			}
			return e.announce(ctx, q, ann)
		}
	})
	return did, err
}

// openBranches is Catalogue.OpenBranches at the current world era (read under the world advisory lock the step holds).
func (e *Effects) openBranches(ctx context.Context, q *db.Queries, into []string) ([]string, error) {
	if e.World == nil {
		return nil, errors.New("faction effects: World catalogue not set")
	}
	ws, err := q.GetWorldState(ctx)
	if err != nil {
		return nil, fmt.Errorf("world state: %w", err)
	}
	return e.Cat.OpenBranches(e.World, into, eraSeqOf(e.World, ws)), nil
}

func (e *Effects) announce(ctx context.Context, q *db.Queries, a AnnouncementEvent) error {
	for _, t := range a.Targets {
		a.TargetNames = append(a.TargetNames, e.name(t))
	}
	payload, _ := json.Marshal(a)
	return q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{Topic: TopicAnnouncement, AggregateID: strconv.FormatInt(a.EffectID, 10), Payload: payload})
}

func (e *Effects) bulk(ctx context.Context, q *db.Queries, effectID, moved int64) error {
	if moved == 0 {
		return nil
	}
	payload, _ := json.Marshal(map[string]int64{"effectId": effectID, "moved": moved})
	return q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{Topic: TopicBulkChanged, AggregateID: strconv.FormatInt(effectID, 10), Payload: payload})
}
