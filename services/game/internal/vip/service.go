package vip

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/db"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/platform/postgres"
)

// ActivityToday is one earning activity with today's progress (GET /vip/status).
type ActivityToday struct {
	ID            string `json:"id"`
	Source        string `json:"source"`
	Name          string `json:"name"`
	Points        int64  `json:"points"`
	DailyMaxTimes int64  `json:"dailyMaxTimes"`
	Live          bool   `json:"live"`
	TimesToday    int64  `json:"timesToday"`
	PointsToday   int64  `json:"pointsToday"`
}

// Status is a player's Tiên Duyên state.
type Status struct {
	DisplayName      string          `json:"displayName"`
	PointName        string          `json:"pointName"`
	Points           int64           `json:"points"`
	Benefits         Benefits        `json:"benefits"`
	NextTier         *int            `json:"nextTier,omitempty"`
	PointsToNext     int64           `json:"pointsToNext,omitempty"`
	GiftClaimedToday bool            `json:"giftClaimedToday"`
	EarnedToday      int64           `json:"earnedToday"`  // in-game điểm duyên this game day (no top-up/admin)
	EarnDailyCap     int64           `json:"earnDailyCap"` // vip.yaml earn.daily_cap_points
	Activities       []ActivityToday `json:"activities"`
	ResetsAt         time.Time       `json:"resetsAt"`
}

var (
	// ErrInvalidGrant is returned for zero/too-large/negative-result changes.
	ErrInvalidGrant = errors.New("vip: invalid grant")
	// ErrKeyReuse: the idempotency key was already used with a different payload.
	ErrKeyReuse = errors.New("vip: idempotency key reused with different payload")
	// ErrUnknownActivity: the activity is not in vip.yaml or not live.
	ErrUnknownActivity = errors.New("vip: unknown earning activity")
	// ErrLedgerNotFound: refund of a ledger row that does not exist.
	ErrLedgerNotFound = errors.New("vip: ledger row not found")
	// ErrNotRefundable: only top-up purchase rows can be refunded.
	ErrNotRefundable = errors.New("vip: ledger row is not a top-up purchase")
	// ErrAlreadyRefunded: the purchase was already reversed (by another key).
	ErrAlreadyRefunded = errors.New("vip: purchase already refunded")
	// ErrOrderConflict: the payment order was already credited with a different user or amount.
	ErrOrderConflict = errors.New("vip: top-up order already credited differently")
)

// MaxGrantDelta bounds a single ledger change (typo guard, not a game number): Tiên Duyên 12 needs 500,000.
const MaxGrantDelta = 500_000

// Service reads and mutates điểm duyên.
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

// StatusFor builds the level part of Status from points (pure). Today's earning is filled by withToday.
func (t *Table) StatusFor(points int64) Status {
	tier := t.TierForPoints(points)
	st := Status{DisplayName: t.DisplayName, PointName: t.PointName, Points: points, Benefits: t.Benefits(tier),
		EarnDailyCap: t.EarnDailyCap, Activities: []ActivityToday{}}
	if tier < len(t.tiers)-1 {
		next := tier + 1
		st.NextTier = &next
		st.PointsToNext = t.tiers[next].PointsRequired - points
	}
	return st
}

// withToday fills today's earning progress from the ledger rows of the game day (pure).
func (t *Table) withToday(st Status, rows []db.ListVIPEarnDayRow, now time.Time) Status {
	byID := map[string]db.ListVIPEarnDayRow{}
	for _, r := range rows {
		byID[r.ActivityID] = r
		st.EarnedToday += r.Points
	}
	st.Activities = make([]ActivityToday, 0, len(t.activities))
	for _, a := range t.activities {
		r := byID[a.ID]
		st.Activities = append(st.Activities, ActivityToday{ID: a.ID, Source: a.Source, Name: a.Name, Points: a.Points,
			DailyMaxTimes: a.DailyMaxTimes, Live: a.Live, TimesToday: r.Times, PointsToday: r.Points})
	}
	st.ResetsAt = gameday.NextReset(now)
	return st
}

func (t *Table) statusQ(ctx context.Context, q *db.Queries, userID uuid.UUID, points int64, now time.Time) (Status, error) {
	rows, err := q.ListVIPEarnDay(ctx, db.ListVIPEarnDayParams{UserID: userID, GameDay: gameday.PG(now)})
	if err != nil {
		return Status{}, fmt.Errorf("list earn day: %w", err)
	}
	return t.withToday(t.StatusFor(points), rows, now), nil
}

// Status returns the user's Tiên Duyên status with today's earning progress.
func (s *Service) Status(ctx context.Context, userID uuid.UUID) (Status, error) {
	q := db.New(s.Pool)
	now := s.now()
	pts, err := q.GetVIPPoints(ctx, userID)
	if err != nil {
		return Status{}, err
	}
	st, err := s.Table.statusQ(ctx, q, userID, pts, now)
	if err != nil {
		return Status{}, err
	}
	if st.GiftClaimedToday, err = s.GiftClaimedToday(ctx, userID, now); err != nil {
		return Status{}, err
	}
	return st, nil
}

// ───── ledger primitives (caller holds the transaction) ─────

// entry is one vip_point_ledger row to write.
type entry struct {
	delta      int64
	reason     string // purchase | refund | activity | admin_grant | admin_revoke | promo
	source     string // topup | admin | an activity source
	activityID string
	reverses   int64
	refType    string
	refID      string
	idemKey    string
	now        time.Time
}

// lockAccount creates the account if needed and locks it FOR UPDATE. Every điểm duyên change serializes here
// (lock order: after any progression / daily-task row lock).
func lockAccount(ctx context.Context, q *db.Queries, userID uuid.UUID) (int64, error) {
	if err := q.EnsureVIPAccount(ctx, userID); err != nil {
		return 0, err
	}
	acc, err := q.GetVIPAccountForUpdate(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("lock vip account: %w", err)
	}
	return acc.Points, nil
}

// findKey returns the ledger row already written under idemKey, or nil.
func findKey(ctx context.Context, q *db.Queries, userID uuid.UUID, idemKey string) (*db.VipPointLedger, error) {
	prev, err := q.GetVIPLedgerByKey(ctx, db.GetVIPLedgerByKeyParams{UserID: userID, IdempotencyKey: idemKey})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &prev, nil
}

// write applies e to a LOCKED account holding `points`: balance + ledger row in the caller's transaction.
func write(ctx context.Context, q *db.Queries, userID uuid.UUID, points int64, e entry) (after, ledgerID int64, err error) {
	if e.delta == 0 || e.delta > MaxGrantDelta || e.delta < -MaxGrantDelta {
		return 0, 0, ErrInvalidGrant
	}
	if points+e.delta < 0 {
		return 0, 0, fmt.Errorf("%w: would go negative", ErrInvalidGrant)
	}
	after, err = q.AddVIPPoints(ctx, db.AddVIPPointsParams{UserID: userID, Delta: e.delta})
	if err != nil {
		return 0, 0, err
	}
	if ledgerID, err = q.InsertVIPLedger(ctx, db.InsertVIPLedgerParams{
		UserID: userID, Delta: e.delta, PointsAfter: after, Reason: e.reason, Source: e.source,
		ActivityID: text(e.activityID), GameDay: gameday.PG(e.now),
		ReversesLedgerID: pgtype.Int8{Int64: e.reverses, Valid: e.reverses != 0},
		RefType:          text(e.refType), RefID: text(e.refID), IdempotencyKey: e.idemKey,
	}); err != nil {
		return 0, 0, fmt.Errorf("insert vip ledger: %w", err)
	}
	return after, ledgerID, nil
}

// ───── in-game earning ─────

// Ref identifies what earned the điểm duyên (ledger ref_type/ref_id) and the idempotency key of the credit.
type Ref struct {
	Type, ID, IdemKey string
}

// Earned is the outcome of EarnLocked.
type Earned struct {
	Points   int64 `json:"points"` // 0 when the activity or the day is capped
	Replayed bool  `json:"replayed"`
}

// EarnLocked credits one occurrence of an in-game activity inside the caller's transaction (decision D, invariant
// V2): locks vip_accounts, replays an already-used key, then pays EarnAmount (per-activity daily_max_times and
// the all-activity daily cap, both counted from today's ledger rows under the lock). A capped occurrence writes
// nothing and returns 0. The caller's own idempotency guard (lecture slot row, task claimed_at, settlement key)
// must make sure the same occurrence never reaches here twice with a different key.
func (t *Table) EarnLocked(ctx context.Context, q *db.Queries, userID uuid.UUID, activityID string, ref Ref, now time.Time) (Earned, error) {
	a, ok := t.actByID[activityID]
	if !ok || !a.Live {
		return Earned{}, fmt.Errorf("%w: %q", ErrUnknownActivity, activityID)
	}
	points, err := lockAccount(ctx, q, userID)
	if err != nil {
		return Earned{}, err
	}
	if prev, err := findKey(ctx, q, userID, ref.IdemKey); err != nil {
		return Earned{}, err
	} else if prev != nil {
		if prev.ActivityID.String != activityID {
			return Earned{}, ErrKeyReuse
		}
		return Earned{Points: prev.Delta, Replayed: true}, nil
	}
	rows, err := q.ListVIPEarnDay(ctx, db.ListVIPEarnDayParams{UserID: userID, GameDay: gameday.PG(now)})
	if err != nil {
		return Earned{}, fmt.Errorf("list earn day: %w", err)
	}
	var times, earned int64
	for _, r := range rows {
		earned += r.Points
		if r.ActivityID == activityID {
			times = r.Times
		}
	}
	pay, err := t.EarnAmount(activityID, times, earned)
	if err != nil || pay == 0 {
		return Earned{}, err
	}
	if _, _, err := write(ctx, q, userID, points, entry{delta: pay, reason: "activity", source: a.Source, activityID: a.ID,
		refType: ref.Type, refID: ref.ID, idemKey: ref.IdemKey, now: now}); err != nil {
		return Earned{}, err
	}
	return Earned{Points: pay}, nil
}

// ───── top-up and refund (decision 10) ─────

// topupRefType is the ledger ref_type of a purchase; ref_id is the payment order (unique per order).
const topupRefType = "topup_order"

// TopupKey is the idempotency key of an order's purchase row: the order itself is the anchor, so the same order is
// credited once whatever key or retry path the caller uses (backed by vip_point_ledger_purchase_order_uq).
func TopupKey(orderRef string) string { return "topup:" + orderRef }

// PurchaseByOrder returns the purchase ledger row of a payment order (pgx.ErrNoRows when not credited).
func PurchaseByOrder(ctx context.Context, q *db.Queries, orderRef string) (db.VipPointLedger, error) {
	return q.GetVIPPurchaseByOrder(ctx, db.GetVIPPurchaseByOrderParams{RefType: text(topupRefType), RefID: text(orderRef)})
}

// TopupLocked credits a verified top-up (1.000đ = PointsPer1000VND điểm duyên) inside the caller's transaction.
// The payment webhook / reconciliation (internal/payment) calls this after verifying the order. A second call
// for the same order replays (returns the current balance) when user and amount match, else ErrOrderConflict.
func (t *Table) TopupLocked(ctx context.Context, q *db.Queries, userID uuid.UUID, amountVND int64, orderRef string, now time.Time) (after int64, replay bool, err error) {
	pay := t.PointsForVND(amountVND)
	if pay <= 0 || orderRef == "" {
		return 0, false, ErrInvalidGrant
	}
	check := func(prev db.VipPointLedger) error {
		if prev.UserID != userID || prev.Delta != pay {
			return ErrOrderConflict
		}
		return nil
	}
	// Another user's credit for this order is found without taking their lock.
	if prev, err := q.GetVIPPurchaseByOrder(ctx, db.GetVIPPurchaseByOrderParams{RefType: text(topupRefType), RefID: text(orderRef)}); err == nil {
		if err := check(prev); err != nil {
			return 0, false, err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, err
	}
	points, err := lockAccount(ctx, q, userID)
	if err != nil {
		return 0, false, err
	}
	// Under the lock: concurrent credits of the same order serialize here.
	if prev, err := q.GetVIPPurchaseByOrder(ctx, db.GetVIPPurchaseByOrderParams{RefType: text(topupRefType), RefID: text(orderRef)}); err == nil {
		if err := check(prev); err != nil {
			return 0, false, err
		}
		return points, true, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, err
	}
	after, _, err = write(ctx, q, userID, points, entry{delta: pay, reason: "purchase", source: SourceTopup,
		refType: topupRefType, refID: orderRef, idemKey: TopupKey(orderRef), now: now})
	return after, false, err
}

// Refund is the outcome of RefundLocked.
type Refund struct {
	UserID         uuid.UUID `json:"userId"`
	LedgerID       int64     `json:"ledgerId"`       // the purchase row reversed
	RefundLedgerID int64     `json:"refundLedgerId"` // the refund row
	PointsRemoved  int64     `json:"pointsRemoved"`
	PointsBefore   int64     `json:"pointsBefore"`
	PointsAfter    int64     `json:"pointsAfter"`
	Replayed       bool      `json:"replayed"`
}

// RefundLocked reverses exactly one top-up purchase row (invariant V4): one 'refund' row with delta = −original,
// reverses_ledger_id = original (UNIQUE → at most once). The level is derived from points, so it may drop; gifts
// already claimed are not clawed back. Fails rather than clamping if the balance can no longer cover the refund
// (an admin revoke happened in between) — that case needs a manual decision.
func RefundLocked(ctx context.Context, q *db.Queries, ledgerID int64, idemKey string, now time.Time) (Refund, error) {
	orig, err := q.GetVIPLedgerByID(ctx, ledgerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Refund{}, ErrLedgerNotFound
	} else if err != nil {
		return Refund{}, err
	}
	if orig.Source != SourceTopup || orig.Reason != "purchase" || orig.Delta <= 0 {
		return Refund{}, ErrNotRefundable
	}
	out := Refund{UserID: orig.UserID, LedgerID: ledgerID, PointsRemoved: orig.Delta}
	points, err := lockAccount(ctx, q, orig.UserID)
	if err != nil {
		return Refund{}, err
	}
	out.PointsBefore = points
	if prev, err := findKey(ctx, q, orig.UserID, idemKey); err != nil {
		return Refund{}, err
	} else if prev != nil {
		if prev.ReversesLedgerID.Int64 != ledgerID {
			return Refund{}, ErrKeyReuse
		}
		// Report what the stored refund did, not today's balance.
		out.RefundLedgerID, out.PointsBefore, out.PointsAfter, out.Replayed = prev.ID, prev.PointsAfter-prev.Delta, prev.PointsAfter, true
		return out, nil
	}
	if _, err := q.GetVIPLedgerReversal(ctx, pgtype.Int8{Int64: ledgerID, Valid: true}); err == nil {
		return Refund{}, ErrAlreadyRefunded
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Refund{}, err
	}
	// ref_type/ref_id also carry the purchase id so migration 000009 can rebuild reverses_ledger_id after a down/up.
	out.PointsAfter, out.RefundLedgerID, err = write(ctx, q, orig.UserID, points, entry{delta: -orig.Delta, reason: "refund", source: SourceTopup,
		reverses: ledgerID, refType: "vip_ledger", refID: strconv.FormatInt(ledgerID, 10), idemKey: idemKey, now: now})
	if err != nil {
		return Refund{}, err
	}
	return out, nil
}

// Topup records a verified top-up in its own transaction (tests; the gateway path uses TopupLocked).
func (s *Service) Topup(ctx context.Context, userID uuid.UUID, amountVND int64, orderRef string) (Status, error) {
	var out Status
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		after, _, err := s.Table.TopupLocked(ctx, q, userID, amountVND, orderRef, s.now())
		if err != nil {
			return err
		}
		out, err = s.Table.statusQ(ctx, q, userID, after, s.now())
		return err
	})
	return out, err
}

// Refund reverses a top-up purchase (admin only), idempotent under idemKey, with an admin_audit_logs row.
func (s *Service) Refund(ctx context.Context, adminID uuid.UUID, ledgerID int64, note, idemKey string) (Refund, Status, error) {
	var out Refund
	var st Status
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		now := s.now()
		var err error
		if out, err = s.Table.RefundTx(ctx, q, adminID, ledgerID, note, idemKey, now); err != nil {
			return err
		}
		st, err = s.Table.statusQ(ctx, q, out.UserID, out.PointsAfter, now)
		return err
	})
	return out, st, err
}

// RefundTx is Refund inside the caller's transaction (the payment refund path locks its order first): RefundLocked,
// then on a fresh refund the perk strip (level drop) and the vip.refund audit row.
func (t *Table) RefundTx(ctx context.Context, q *db.Queries, adminID uuid.UUID, ledgerID int64, note, idemKey string, now time.Time) (Refund, error) {
	out, err := RefundLocked(ctx, q, ledgerID, idemKey, now)
	if err != nil || out.Replayed {
		return out, err
	}
	if err := t.stripPerksLocked(ctx, q, out.UserID, out.PointsBefore, out.PointsAfter, now); err != nil {
		return Refund{}, err
	}
	beforeJ, _ := json.Marshal(map[string]int64{"points": out.PointsBefore, "tier": int64(t.TierForPoints(out.PointsBefore))})
	afterJ, _ := json.Marshal(map[string]int64{"points": out.PointsAfter, "tier": int64(t.TierForPoints(out.PointsAfter)),
		"refundLedgerId": out.RefundLedgerID})
	if err := q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{
		AdminUserID: adminID, Action: "vip.refund", TargetType: "user", TargetID: out.UserID.String(),
		Before: beforeJ, After: afterJ, Reason: text(fmt.Sprintf("ledger %d: %s", ledgerID, note)),
	}); err != nil {
		return Refund{}, err
	}
	return out, nil
}

// StatusTx is the user's status (with today's earning) inside the caller's transaction.
func (t *Table) StatusTx(ctx context.Context, q *db.Queries, userID uuid.UUID, now time.Time) (Status, error) {
	pts, err := q.GetVIPPoints(ctx, userID)
	if err != nil {
		return Status{}, err
	}
	return t.statusQ(ctx, q, userID, pts, now)
}

// cosmeticPerkKeys are the perk keys that are equippable cosmetics (cosmetics.yaml kinds).
var cosmeticPerkKeys = map[string]bool{"title": true, "avatar_frame": true, "chat_aura": true, "cave_decoration": true}

// PerksAbove lists the cosmetic perk ids granted by tiers above `tier` (what a character at `tier` may not wear).
func (t *Table) PerksAbove(tier int) []string {
	var ids []string
	for _, tr := range t.tiers {
		if tr.Tier <= tier {
			continue
		}
		for _, p := range tr.Perks {
			if key, _, ok := strings.Cut(p, ":"); ok && cosmeticPerkKeys[key] {
				ids = append(ids, p)
			}
		}
	}
	return ids
}

// stripPerksLocked unequips the perk cosmetics a level drop took away (anti-cheat H1), in the caller's transaction,
// after the vip_accounts lock (lock order vip → character_appearance). No-op when the level did not drop.
func (t *Table) stripPerksLocked(ctx context.Context, q *db.Queries, userID uuid.UUID, before, after int64, now time.Time) error {
	newTier := t.TierForPoints(after)
	if newTier >= t.TierForPoints(before) {
		return nil
	}
	ids := t.PerksAbove(newTier)
	if len(ids) == 0 {
		return nil
	}
	if _, err := q.StripAppearancePerks(ctx, db.StripAppearancePerksParams{Ids: ids, Now: now, UserID: userID}); err != nil {
		return fmt.Errorf("strip perk cosmetics: %w", err)
	}
	return nil
}

// ───── admin grant ─────

// Grant changes a user's điểm duyên by an admin (grant/revoke) idempotently, writing the ledger (source admin)
// and an audit row in the same transaction.
func (s *Service) Grant(ctx context.Context, adminID *uuid.UUID, userID uuid.UUID, delta int64, reason, refType, refID, idemKey string) (Status, error) {
	switch {
	case (reason == "admin_grant" || reason == "promo") && delta > 0:
	case reason == "admin_revoke" && delta < 0:
	default:
		return Status{}, fmt.Errorf("%w: reason %q with delta %d", ErrInvalidGrant, reason, delta)
	}
	var out Status
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		now := s.now()
		before, err := lockAccount(ctx, q, userID)
		if err != nil {
			return err
		}
		after := before
		if prev, err := findKey(ctx, q, userID, idemKey); err != nil {
			return err
		} else if prev != nil {
			if prev.Delta != delta || prev.Source != SourceAdmin {
				return ErrKeyReuse
			}
		} else {
			if after, _, err = write(ctx, q, userID, before, entry{delta: delta, reason: reason, source: SourceAdmin,
				refType: refType, refID: refID, idemKey: idemKey, now: now}); err != nil {
				return err
			}
			if err := s.Table.stripPerksLocked(ctx, q, userID, before, after, now); err != nil {
				return err
			}
			if adminID != nil {
				beforeJ, _ := json.Marshal(map[string]int64{"points": before})
				afterJ, _ := json.Marshal(map[string]int64{"points": after})
				if err := q.InsertAdminAudit(ctx, db.InsertAdminAuditParams{
					AdminUserID: *adminID, Action: "vip.grant", TargetType: "user", TargetID: userID.String(),
					Before: beforeJ, After: afterJ, Reason: text(reason + ": " + refID),
				}); err != nil {
					return err
				}
			}
		}
		out, err = s.Table.statusQ(ctx, q, userID, after, now)
		return err
	})
	return out, err
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
