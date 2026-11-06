package world

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/contribution"
	"tiendao/game/internal/db"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/realm"
)

// LockID is the advisory lock that makes the world day job single-runner across workers (§5.6).
const LockID = 7_310_002

const medianPage = 1000

// Service runs the world clock jobs. Phase 1: per-day totals, day close + server median, fold into world_state.
type Service struct {
	Pool         *pgxpool.Pool
	Realms       *realm.Catalogue
	Cat          *Catalogue
	Contribution *contribution.Config // ceiling for the day-close audit (nil → audit skips C1)
	Log          *slog.Logger         // nil → slog.Default()
	Now          func() time.Time
}

// State is the admin/API view of world_state.
type State struct {
	StartedAt           *time.Time `json:"startedAt"`
	ServerDay           int        `json:"serverDay"`
	ProgressMilli       int64      `json:"progressMilli"`
	ClosedThroughDay    string     `json:"closedThroughDay,omitempty"`
	MedianPositionMilli int64      `json:"medianPositionMilli"`
}

func stateOf(ws db.WorldState, now time.Time) State {
	st := State{StartedAt: ws.StartedAt, ServerDay: ServerDay(ws.StartedAt, now), ProgressMilli: ws.ProgressMilli,
		MedianPositionMilli: ws.MedianPositionMilli}
	if ws.ClosedThroughDay.Valid {
		st.ClosedThroughDay = ws.ClosedThroughDay.Time.Format(time.DateOnly)
	}
	return st
}

// Start sets world_state.started_at (server day 0) once; later calls are no-ops. Audited.
func (s *Service) Start(ctx context.Context, admin uuid.UUID) (State, error) {
	var out State
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		before, err := q.GetWorldStateForUpdate(ctx)
		if err != nil {
			return fmt.Errorf("lock world state: %w", err)
		}
		now := s.Now().UTC()
		ws, err := q.StartWorld(ctx, &now)
		if err != nil {
			return err
		}
		out = stateOf(ws, now)
		if before.StartedAt != nil {
			return nil // idempotent: already started, nothing to audit
		}
		after, _ := json.Marshal(out)
		return q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{AdminUserID: admin, Action: "world.start", TargetType: "world",
			TargetID: "1", Before: []byte("{}"), After: after, Reason: pgtype.Text{}})
	})
	return out, err
}

// TickResult reports what a world tick did.
type TickResult struct {
	Ran        bool // false when another worker holds the lock
	Recomputed int  // game days recomputed
	Closed     int  // game days closed this tick
	Violations int  // audit violations found on closed days (C1 ceiling, C3 credits = points)
}

// MaxCatchUpDays bounds how many game days one tick recomputes after a long outage (the rest follow next ticks).
const MaxCatchUpDays = 400

// Tick recomputes per-day totals for every game day from closed_through_day + 1 (or the launch day, or yesterday
// before launch) through today — days without any row get a DAU 0 row — closes finished days IN ORDER (with the
// server median and an invariant audit), and folds closed days into world_state. Replay-safe: totals are
// recomputed, closed rows never change, folding is guarded by closed_through_day under the world_state row lock.
func (s *Service) Tick(ctx context.Context) (TickResult, error) {
	var res TickResult
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, "SELECT pg_try_advisory_xact_lock($1)", LockID).Scan(&res.Ran); err != nil || !res.Ran {
			return err
		}
		q := db.New(tx)
		now := s.Now().UTC()
		set := s.Cat.Settings
		ws, err := q.GetWorldStateForUpdate(ctx)
		if err != nil {
			return fmt.Errorf("lock world state: %w", err)
		}
		days := DaysToProcess(ws.ClosedThroughDay, ws.StartedAt, now)

		// 1. Recompute every open day in the range (closed ones are left untouched by the upsert guard).
		for _, d := range days {
			if err := q.UpsertWorldDailyTotal(ctx, db.UpsertWorldDailyTotalParams{GameDay: date(d), CapMilli: set.WorldCreditCapMilli, ComputedAt: now}); err != nil {
				return fmt.Errorf("recompute %s: %w", d.Format(time.DateOnly), err)
			}
			res.Recomputed++
		}
		// 2. Close finished days in order; stop at the first day still open for late commits.
		for _, d := range days {
			if now.Before(ClosableAt(d, set.DayCloseGrace)) {
				break
			}
			row, err := q.GetWorldDailyTotal(ctx, date(d))
			if err != nil {
				return err
			}
			if row.ClosedAt != nil {
				continue
			}
			med, err := s.median(ctx, q, d)
			if err != nil {
				return err
			}
			if err := q.CloseWorldDay(ctx, db.CloseWorldDayParams{GameDay: date(d), MedianPositionMilli: pgtype.Int8{Int64: med, Valid: true}, ClosedAt: &now}); err != nil {
				return err
			}
			res.Closed++
			n, err := s.audit(ctx, q, d)
			if err != nil {
				return err
			}
			res.Violations += n
		}
		// 3. Fold newly closed days into world_state.
		closed, err := q.ListClosedDaysAfter(ctx, ws.ClosedThroughDay)
		if err != nil || len(closed) == 0 {
			return err
		}
		progress, median, medianDay := ws.ProgressMilli, ws.MedianPositionMilli, ws.MedianGameDay
		for _, c := range closed {
			if ws.StartedAt != nil && !c.GameDay.Time.Before(gameday.Of(*ws.StartedAt)) {
				progress += c.PerCapitaMilli
			}
			if c.MedianPositionMilli.Valid {
				median, medianDay = c.MedianPositionMilli.Int64, c.GameDay
			}
		}
		return q.FoldWorldProgress(ctx, db.FoldWorldProgressParams{ProgressMilli: progress, ClosedThroughDay: closed[len(closed)-1].GameDay,
			MedianPositionMilli: median, MedianGameDay: medianDay})
	})
	return res, err
}

// DaysToProcess lists the game days a tick must recompute: from closed_through_day + 1 — or the launch day, or
// yesterday before launch — through today, at most MaxCatchUpDays (oldest first). Pure.
func DaysToProcess(closedThrough pgtype.Date, startedAt *time.Time, now time.Time) []time.Time {
	today := gameday.Of(now)
	from := today.AddDate(0, 0, -1)
	switch {
	case closedThrough.Valid:
		from = closedThrough.Time.AddDate(0, 0, 1)
	case startedAt != nil && gameday.Of(*startedAt).Before(from):
		from = gameday.Of(*startedAt)
	}
	if from.After(today) {
		from = today
	}
	var days []time.Time
	for d := from; !d.After(today) && len(days) < MaxCatchUpDays; d = d.AddDate(0, 0, 1) {
		days = append(days, d)
	}
	return days
}

// audit checks the contribution invariants of a closed day and logs every violation at error level.
func (s *Service) audit(ctx context.Context, q *db.Queries, d time.Time) (int, error) {
	log := s.Log
	if log == nil {
		log = slog.Default()
	}
	n := 0
	if s.Contribution != nil {
		over, err := q.ListContributionOverCeiling(ctx, db.ListContributionOverCeilingParams{GameDay: date(d), CeilingMilli: pgtype.Int8{Int64: s.Contribution.DailyCeilingMilli, Valid: true}})
		if err != nil {
			return 0, fmt.Errorf("audit ceiling: %w", err)
		}
		for _, o := range over {
			log.Error("invariant C1 violated: daily points over ceiling", "characterId", o.CharacterID, "gameDay", d.Format(time.DateOnly),
				"pointsMilli", o.PointsMilli.Int64, "ceilingMilli", s.Contribution.DailyCeilingMilli)
		}
		n += len(over)
	}
	bad, err := q.AuditCreditsForDay(ctx, date(d))
	if err != nil {
		return 0, fmt.Errorf("audit credits: %w", err)
	}
	for _, b := range bad {
		log.Error("invariant C3 violated: daily_contribution != Σ cultivation_credits", "characterId", b.CharacterID,
			"gameDay", d.Format(time.DateOnly), "pointsMilli", b.PointsMilli, "creditedMilli", b.CreditedMilli)
	}
	return n + len(bad), nil
}

// median is the lower median ladder position of characters with points in the window ending at day d.
func (s *Service) median(ctx context.Context, q *db.Queries, d time.Time) (int64, error) {
	from := d.AddDate(0, 0, -(s.Cat.Settings.MedianWindowDays - 1))
	var positions []int64
	after := uuid.Nil
	for {
		rows, err := q.ListActivePositionsPage(ctx, db.ListActivePositionsPageParams{After: after, FromDay: date(from), ToDay: date(d), Lim: medianPage})
		if err != nil {
			return 0, err
		}
		for _, r := range rows {
			st, err := s.Realms.Stage(r.StageID)
			if err != nil {
				return 0, err
			}
			positions = append(positions, s.Realms.Position(st, r.CultivationMilli))
			after = r.CharacterID
		}
		if len(rows) < medianPage {
			return Median(positions), nil
		}
	}
}

func date(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }
