// Package ritual holds the daily rituals: the morning incense streak (Incense) and gieo quẻ (Fortune, fortune.go).
//
// Morning incense ("Dâng hương tĩnh tâm", data-model §9.2): once per game day a character
// offers incense; consecutive days build a streak whose reward (daily_tasks.yaml streak_rewards, points_milli) goes
// through the single credit path (cultivation.CreditLocked, source ritual → activity bucket, 150/day ceiling).
// The offering completes and claims the `incense` daily task in the same transaction (điểm duyên daily_task_claim).
package ritual

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/contribution"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/db"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/vip"
)

var (
	ErrNoCharacter = errors.New("ritual: character not found")
	ErrNoIncense   = errors.New("ritual: no incense task in daily_tasks.yaml")
)

// Result is POST /tasks/daily/incense (authoritative state: streak, task, cultivation).
type Result struct {
	Streak         dailytask.StreakView `json:"streak"`
	RawMilli       int64                `json:"rawMilli"`
	EarnedMilli    int64                `json:"earnedMilli"` // after bucket/ceiling/bình cảnh
	VIPPoints      int64                `json:"vipPoints"`
	StagesAdvanced []string             `json:"stagesAdvanced"`
	Task           dailytask.View       `json:"task"`
	Cultivation    cultivation.Snapshot `json:"cultivation"`
	Replayed       bool                 `json:"replayed"` // already offered today: same result, nothing written
}

// Service owns the incense transaction.
type Service struct {
	Pool        *pgxpool.Pool
	Tasks       *dailytask.Catalogue
	Cultivation *cultivation.Service
	VIP         *vip.Table // nil → no điểm duyên
	Now         func() time.Time
}

// Incense offers today's incense. Idempotent per game day: a second call returns the stored result (Replayed).
// Order: lock progression → ritual_streaks (guard: last_day = today) → CreditLocked(ritual) → task row (progress +
// claimed_at) → EarnLocked(daily_task_claim). Lock order §2.3: progression → daily_contribution → … → vip_accounts.
func (s *Service) Incense(ctx context.Context, userID uuid.UUID) (Result, error) {
	t, ok := s.Tasks.ByKind(dailytask.Incense)
	if !ok {
		return Result{}, ErrNoIncense
	}
	var out Result
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, err := q.GetCharacterByUser(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCharacter
		} else if err != nil {
			return err
		}
		p, err := q.GetProgressionForUpdate(ctx, ch.ID)
		if err != nil {
			return fmt.Errorf("lock progression: %w", err)
		}
		now := s.Now().UTC()
		today := gameday.Of(now)
		dayKey := today.Format("20060102")
		creditKey := "incense:" + dayKey
		var row *db.RitualStreak
		if r, err := q.GetRitualStreak(ctx, ch.ID); err == nil {
			row = &r
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if row != nil && row.LastDay.Time.Equal(today) { // already offered today (the progression lock serializes)
			out, err = s.result(ctx, q, userID, ch.ID, p, t, row, now)
			if err != nil {
				return err
			}
			out.Replayed = true
			if c, err := q.GetCultivationCreditByKey(ctx, db.GetCultivationCreditByKeyParams{CharacterID: ch.ID,
				IdempotencyKey: string(contribution.SourceRitual) + ":" + creditKey}); err == nil {
				out.RawMilli, out.EarnedMilli = c.RawMilli, c.CreditedMilli
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			return nil
		}
		var last time.Time
		var prev, best int32
		if row != nil {
			last, prev, best = row.LastDay.Time, row.Streak, row.Best
		}
		streak := dailytask.NextStreak(last, prev, today)
		next := db.RitualStreak{CharacterID: ch.ID, LastDay: gameday.PG(now), Streak: streak, Best: max(best, streak), UpdatedAt: now}
		if err := q.UpsertRitualStreak(ctx, db.UpsertRitualStreakParams{CharacterID: ch.ID, LastDay: next.LastDay, Streak: next.Streak,
			Best: next.Best, UpdatedAt: now}); err != nil {
			return fmt.Errorf("upsert streak: %w", err)
		}
		raw := t.StreakPoints(streak)
		var credit cultivation.CreditOutcome
		p, credit, err = s.Cultivation.CreditLocked(ctx, q, p, contribution.SourceRitual, raw,
			cultivation.Ref{Type: "ritual", ID: t.ID, IdemKey: creditKey}, now)
		if err != nil {
			return err
		}
		if err := q.IncrementDailyTask(ctx, db.IncrementDailyTaskParams{CharacterID: ch.ID, GameDay: gameday.PG(now), TaskID: t.ID, Delta: 1, Target: t.Target}); err != nil {
			return err
		}
		if err := q.MarkDailyTaskClaimed(ctx, db.MarkDailyTaskClaimedParams{CharacterID: ch.ID, GameDay: gameday.PG(now), TaskID: t.ID, ClaimedAt: &now}); err != nil {
			return err
		}
		var vipPts int64
		if s.VIP != nil { // same ledger key shape as dailytask.Claim
			got, err := s.VIP.EarnLocked(ctx, q, userID, vip.ActDailyTask, vip.Ref{Type: "daily_task", ID: t.ID, IdemKey: fmt.Sprintf("task:%s:%s", dayKey, t.ID)}, now)
			if err != nil {
				return fmt.Errorf("earn điểm duyên: %w", err)
			}
			vipPts = got.Points
		}
		out, err = s.result(ctx, q, userID, ch.ID, p, t, &next, now)
		if err != nil {
			return err
		}
		out.RawMilli, out.EarnedMilli, out.VIPPoints = raw, credit.Applied.CreditedMilli, vipPts
		for _, st := range credit.Passed {
			out.StagesAdvanced = append(out.StagesAdvanced, st.ID)
		}
		return nil
	})
	return out, err
}

func (s *Service) result(ctx context.Context, q *db.Queries, userID, charID uuid.UUID, p db.CharacterProgression, t *dailytask.Task,
	row *db.RitualStreak, now time.Time) (Result, error) {
	snap, err := s.Cultivation.SnapshotLocked(ctx, q, userID, p, now)
	if err != nil {
		return Result{}, err
	}
	sv := dailytask.StreakOf(t, row, now)
	return Result{
		Streak: *sv, StagesAdvanced: []string{}, Cultivation: snap,
		Task: dailytask.View{Task: *t, Progress: t.Target, Complete: true, Claimed: true, Streak: sv},
	}, nil
}
