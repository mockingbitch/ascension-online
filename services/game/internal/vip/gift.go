package vip

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tiendao/game/internal/db"
	"tiendao/game/internal/economy"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/platform/postgres"
)

var (
	ErrNoGift          = errors.New("vip: no daily gift at this level")
	ErrGiftClaimed     = errors.New("vip: daily gift already claimed today")
	ErrGiftNoCharacter = errors.New("vip: character required to receive the gift")
)

// GiftResult is returned after claiming.
type GiftResult struct {
	Tier        int       `json:"tier"`
	Amount      int64     `json:"amount"`
	Currency    string    `json:"currency"`
	Balance     int64     `json:"balance"`
	NextResetAt time.Time `json:"nextResetAt"`
}

// ClaimDailyGift credits today's VIP spirit-stone gift once per game day (PK on user+day).
// The tier is resolved at claim time.
func (s *Service) ClaimDailyGift(ctx context.Context, userID uuid.UUID, now time.Time) (GiftResult, error) {
	var out GiftResult
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, err := q.GetCharacterByUser(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrGiftNoCharacter
		} else if err != nil {
			return err
		}
		pts, err := q.GetVIPPoints(ctx, userID)
		if err != nil {
			return err
		}
		b := s.Table.Benefits(s.Table.TierForPoints(pts))
		if b.DailyGift <= 0 {
			return ErrNoGift
		}
		day := gameday.PG(now)
		if _, err := q.InsertDailyGiftClaim(ctx, db.InsertDailyGiftClaimParams{UserID: userID, ClaimDay: day, Tier: int16(b.Tier), Amount: b.DailyGift}); errors.Is(err, pgx.ErrNoRows) {
			return ErrGiftClaimed
		} else if err != nil {
			return err
		}
		key := fmt.Sprintf("vip_gift:%s", gameday.Of(now).Format("20060102"))
		bal, err := economy.Credit(ctx, q, ch.ID, economy.SpiritStone, b.DailyGift, economy.Ref{Reason: "vip_daily_gift", Type: "vip_tier", ID: fmt.Sprint(b.Tier), IdemKey: key})
		if err != nil {
			return err
		}
		out = GiftResult{Tier: b.Tier, Amount: b.DailyGift, Currency: economy.SpiritStone, Balance: bal, NextResetAt: gameday.NextReset(now)}
		return nil
	})
	return out, err
}

// GiftClaimedToday reports whether today's gift was claimed.
func (s *Service) GiftClaimedToday(ctx context.Context, userID uuid.UUID, now time.Time) (bool, error) {
	_, err := db.New(s.Pool).GetDailyGiftClaim(ctx, db.GetDailyGiftClaimParams{UserID: userID, ClaimDay: gameday.PG(now)})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
