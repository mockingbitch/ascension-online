package vip

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
	"tiendao/game/internal/platform/postgres"
)

// Status is a player's VIP state.
type Status struct {
	Points           int64    `json:"points"`
	Benefits         Benefits `json:"benefits"`
	NextTier         *int     `json:"nextTier,omitempty"`
	PointsToNext     int64    `json:"pointsToNext,omitempty"`
	GiftClaimedToday bool     `json:"giftClaimedToday"`
}

var (
	// ErrInvalidGrant is returned for zero/too-large/negative-result grants.
	ErrInvalidGrant = errors.New("vip: invalid grant")
	// ErrKeyReuse: the idempotency key was already used with a different payload.
	ErrKeyReuse = errors.New("vip: idempotency key reused with different payload")
)

// MaxGrantDelta bounds a single grant (typo guard): VIP 12 needs 50,000 points.
const MaxGrantDelta = 100_000

// Service reads and mutates VIP points.
type Service struct {
	Pool  *pgxpool.Pool
	Table *Table
	Now   func() time.Time // nil → time.Now
}

func (s *Service) now() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

// StatusFor builds Status from points (pure).
func (t *Table) StatusFor(points int64) Status {
	tier := t.TierForPoints(points)
	st := Status{Points: points, Benefits: t.Benefits(tier)}
	if tier < len(t.tiers)-1 {
		next := tier + 1
		st.NextTier = &next
		st.PointsToNext = t.tiers[next].PointsRequired - points
	}
	return st
}

// Status returns the user's VIP status.
func (s *Service) Status(ctx context.Context, userID uuid.UUID) (Status, error) {
	pts, err := db.New(s.Pool).GetVIPPoints(ctx, userID)
	if err != nil {
		return Status{}, err
	}
	st := s.Table.StatusFor(pts)
	if st.GiftClaimedToday, err = s.GiftClaimedToday(ctx, userID, s.now()); err != nil {
		return Status{}, err
	}
	return st, nil
}

// Grant changes a user's VIP points (admin grant/revoke, later: verified purchase) idempotently,
// writing the ledger and — for admin actions — an audit row in the same transaction.
func (s *Service) Grant(ctx context.Context, adminID *uuid.UUID, userID uuid.UUID, delta int64, reason, refType, refID, idemKey string) (Status, error) {
	if delta == 0 || delta > MaxGrantDelta || delta < -MaxGrantDelta {
		return Status{}, ErrInvalidGrant
	}
	var out Status
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		if err := q.EnsureVIPAccount(ctx, userID); err != nil {
			return err
		}
		acc, err := q.GetVIPAccountForUpdate(ctx, userID)
		if err != nil {
			return err
		}
		if prev, err := q.GetVIPLedgerByKey(ctx, db.GetVIPLedgerByKeyParams{UserID: userID, IdempotencyKey: idemKey}); err == nil {
			if prev.Delta != delta {
				return ErrKeyReuse
			}
			out = s.Table.StatusFor(acc.Points) // replay
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if acc.Points+delta < 0 {
			return fmt.Errorf("%w: would go negative", ErrInvalidGrant)
		}
		after, err := q.AddVIPPoints(ctx, db.AddVIPPointsParams{UserID: userID, Delta: delta})
		if err != nil {
			return err
		}
		if err := q.InsertVIPLedger(ctx, db.InsertVIPLedgerParams{
			UserID: userID, Delta: delta, PointsAfter: after, Reason: reason,
			RefType: text(refType), RefID: text(refID), IdempotencyKey: idemKey,
		}); err != nil {
			return err
		}
		if adminID != nil {
			before, _ := json.Marshal(map[string]int64{"points": acc.Points})
			afterJ, _ := json.Marshal(map[string]int64{"points": after})
			if err := q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{
				AdminUserID: *adminID, Action: "vip.grant", TargetType: "user", TargetID: userID.String(),
				Before: before, After: afterJ, Reason: text(reason + ": " + refID),
			}); err != nil {
				return err
			}
		}
		out = s.Table.StatusFor(after)
		return nil
	})
	return out, err
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
