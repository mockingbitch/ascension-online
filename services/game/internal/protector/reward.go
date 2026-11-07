package protector

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"tiendao/game/internal/db"
	"tiendao/game/internal/vip"
)

// ErrPermanent marks an event that can never succeed (bad payload, unknown activity, integrity violation).
var ErrPermanent = errors.New("protector: permanent reward error")

// RewardLocked credits one counted protector's điểm duyên (vip.yaml protector_assist, capped there and by the 20/day
// earn cap) on the game day of the attempt. Idempotent by the assist id: a replayed outbox batch returns the stored
// credit. Permanent failures are wrapped in ErrPermanent.
func RewardLocked(ctx context.Context, q *db.Queries, t *vip.Table, ev AssistCountedEvent) (int64, error) {
	uid, err := uuid.Parse(ev.ProtectorUserID)
	if err != nil || ev.AssistID <= 0 {
		return 0, fmt.Errorf("%w: bad payload (user %q, assist %d)", ErrPermanent, ev.ProtectorUserID, ev.AssistID)
	}
	id := strconv.FormatInt(ev.AssistID, 10)
	got, err := t.EarnLocked(ctx, q, uid, vip.ActProtectorAssist, vip.Ref{Type: "protector_assist", ID: id, IdemKey: "protector:" + id}, ev.OccurredAt)
	if err != nil {
		var pg *pgconn.PgError
		if errors.Is(err, vip.ErrUnknownActivity) || errors.Is(err, vip.ErrKeyReuse) || (errors.As(err, &pg) && strings.HasPrefix(pg.Code, "23")) {
			return 0, fmt.Errorf("%w: %v", ErrPermanent, err)
		}
		return 0, fmt.Errorf("protector reward: %w", err)
	}
	return got.Points, nil
}

// RewardBatch credits a drained batch inside the worker's transaction, each event in its own savepoint and in user-id
// order (vip_accounts lock order across workers). A permanent failure is logged and skipped (the event is still marked
// published by the caller — a dead letter in the log); a transient one fails the batch so it is retried.
// It returns the number of dead letters (exported as a worker metric).
func RewardBatch(ctx context.Context, tx pgx.Tx, t *vip.Table, evs []AssistCountedEvent, log *slog.Logger) (int, error) {
	slices.SortFunc(evs, func(a, b AssistCountedEvent) int { return strings.Compare(a.ProtectorUserID, b.ProtectorUserID) })
	dead := 0
	for _, ev := range evs {
		sp, err := tx.Begin(ctx) // savepoint
		if err != nil {
			return dead, err
		}
		if _, err := RewardLocked(ctx, db.New(sp), t, ev); err != nil {
			_ = sp.Rollback(ctx)
			if errors.Is(err, ErrPermanent) {
				if log != nil {
					log.ErrorContext(ctx, "protector reward dropped (dead letter)", "assistId", ev.AssistID, "user", ev.ProtectorUserID, "err", err)
				}
				dead++
				continue
			}
			return dead, err
		}
		if err := sp.Commit(ctx); err != nil {
			return dead, err
		}
	}
	return dead, nil
}
