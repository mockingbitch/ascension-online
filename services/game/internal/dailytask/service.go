package dailytask

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
	"tiendao/game/internal/economy"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/platform/postgres"
)

var (
	ErrNoCharacter    = errors.New("dailytask: character not found")
	ErrUnknownTask    = errors.New("dailytask: unknown task")
	ErrNotComplete    = errors.New("dailytask: task not complete")
	ErrAlreadyClaimed = errors.New("dailytask: reward already claimed")
)

// Record advances every task of `kind` for today. Call INSIDE the gameplay transaction that caused it.
// elapsedSeconds is only used by settle tasks with min_elapsed_seconds. Nil catalogue → no-op.
func (c *Catalogue) Record(ctx context.Context, q *db.Queries, charID uuid.UUID, now time.Time, kind Kind, elapsedSeconds int64) error {
	if c == nil {
		return nil
	}
	for _, t := range c.tasks {
		if t.Kind != kind || (t.MinElapsedSeconds > 0 && elapsedSeconds < t.MinElapsedSeconds) {
			continue
		}
		if err := q.IncrementDailyTask(ctx, db.IncrementDailyTaskParams{
			CharacterID: charID, GameDay: gameday.PG(now), TaskID: t.ID, Delta: 1, Target: t.Target,
		}); err != nil {
			return fmt.Errorf("record task %s: %w", t.ID, err)
		}
	}
	return nil
}

// View is a task with today's progress.
type View struct {
	Task
	Progress int32 `json:"progress"`
	Complete bool  `json:"complete"`
	Claimed  bool  `json:"claimed"`
}

// Board is today's task list.
type Board struct {
	GameDay  string    `json:"gameDay"`
	ResetsAt time.Time `json:"resetsAt"`
	Tasks    []View    `json:"tasks"`
}

// Service lists and claims daily tasks.
type Service struct {
	Pool *pgxpool.Pool
	Cat  *Catalogue
	Now  func() time.Time
}

// Board returns today's tasks and progress.
func (s *Service) Board(ctx context.Context, userID uuid.UUID) (Board, error) {
	q := db.New(s.Pool)
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Board{}, ErrNoCharacter
	} else if err != nil {
		return Board{}, err
	}
	now := s.Now()
	rows, err := q.ListDailyTasks(ctx, db.ListDailyTasksParams{CharacterID: ch.ID, GameDay: gameday.PG(now)})
	if err != nil {
		return Board{}, err
	}
	byID := map[string]db.DailyTaskProgress{}
	for _, r := range rows {
		byID[r.TaskID] = r
	}
	b := Board{GameDay: gameday.Of(now).Format("2006-01-02"), ResetsAt: gameday.NextReset(now), Tasks: []View{}}
	for _, t := range s.Cat.All() {
		r := byID[t.ID]
		v := View{Task: t, Progress: r.Progress, Claimed: r.ClaimedAt != nil}
		if t.Kind == Checkin {
			v.Progress = t.Target // visiting counts
		}
		v.Complete = v.Progress >= t.Target
		b.Tasks = append(b.Tasks, v)
	}
	return b, nil
}

// Claim grants a completed task's rewards once per game day.
func (s *Service) Claim(ctx context.Context, userID uuid.UUID, taskID string) (View, error) {
	t, ok := s.Cat.Get(taskID)
	if !ok {
		return View{}, ErrUnknownTask
	}
	var out View
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, err := q.GetCharacterByUser(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNoCharacter
		} else if err != nil {
			return err
		}
		now := s.Now()
		day := gameday.PG(now)
		if t.Kind == Checkin { // materialize the row; the upsert also takes the row lock
			if err := q.IncrementDailyTask(ctx, db.IncrementDailyTaskParams{CharacterID: ch.ID, GameDay: day, TaskID: t.ID, Delta: 1, Target: t.Target}); err != nil {
				return err
			}
		}
		row, err := q.GetDailyTaskForUpdate(ctx, db.GetDailyTaskForUpdateParams{CharacterID: ch.ID, GameDay: day, TaskID: t.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotComplete
		} else if err != nil {
			return err
		}
		if row.ClaimedAt != nil {
			return ErrAlreadyClaimed
		}
		if row.Progress < t.Target {
			return ErrNotComplete
		}
		key := fmt.Sprintf("task:%s:%s", gameday.Of(now).Format("20060102"), t.ID)
		ref := economy.Ref{Reason: "daily_task", Type: "daily_task", ID: t.ID, IdemKey: key}
		for _, r := range t.Rewards {
			if r.ItemID != "" {
				if _, err := economy.GrantItem(ctx, q, ch.ID, r.ItemID, r.Quantity, ref); err != nil {
					return err
				}
			} else if _, err := economy.Credit(ctx, q, ch.ID, r.Currency, r.Amount, ref); err != nil {
				return err
			}
		}
		if err := q.MarkDailyTaskClaimed(ctx, db.MarkDailyTaskClaimedParams{CharacterID: ch.ID, GameDay: day, TaskID: t.ID, ClaimedAt: &now}); err != nil {
			return err
		}
		out = View{Task: *t, Progress: row.Progress, Complete: true, Claimed: true}
		return nil
	})
	return out, err
}
