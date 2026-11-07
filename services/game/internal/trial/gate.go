package trial

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tiendao/game/internal/db"
)

// ValidPass returns the character's valid, unconsumed pass for kind at now (nil if none). With lock, the row is
// locked FOR UPDATE — call after the progression lock (lock order: progression → daily_contribution → trial_runs).
func ValidPass(ctx context.Context, q *db.Queries, charID uuid.UUID, kind string, now time.Time, lock bool) (*db.TrialRun, error) {
	var r db.TrialRun
	var err error
	if lock {
		r, err = q.GetValidTrialPassForUpdate(ctx, db.GetValidTrialPassForUpdateParams{CharacterID: charID, Kind: kind, Now: &now})
	} else {
		r, err = q.GetValidTrialPass(ctx, db.GetValidTrialPassParams{CharacterID: charID, Kind: kind, Now: &now})
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("trial pass: %w", err)
	}
	return &r, nil
}

// ErrPassGone: the pass was consumed concurrently (cannot happen under the progression lock).
var ErrPassGone = errors.New("trial: pass already consumed")

// ConsumePass marks the pass used by a successful breakthrough (ref = "bt:<idempotency key>").
func ConsumePass(ctx context.Context, q *db.Queries, runID uuid.UUID, ref string, now time.Time) error {
	n, err := q.ConsumeTrialPass(ctx, db.ConsumeTrialPassParams{ID: runID, ConsumedAt: &now, ConsumedRef: text(ref)})
	if err != nil {
		return fmt.Errorf("consume trial pass: %w", err)
	}
	if n != 1 {
		return ErrPassGone
	}
	return nil
}
