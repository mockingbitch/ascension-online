package breakthrough

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

	"tiendao/game/internal/contribution"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/db"
	"tiendao/game/internal/fixedpoint"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/realm"
)

// ErrNotReady carries the preview explaining what is missing.
type ErrNotReady struct{ Preview Preview }

func (e *ErrNotReady) Error() string { return "breakthrough: requirements not met" }

// ErrQuoteChanged: the tribulation loss now exceeds the client's acceptLossMilli guard. Nothing was written.
type ErrQuoteChanged struct{ Preview Preview }

func (e *ErrQuoteChanged) Error() string { return "breakthrough: quote changed" }

// ErrQuoteRequired: a non-guaranteed tribulation attempt must carry acceptLossMilli (the player saw the stakes).
type ErrQuoteRequired struct{ Preview Preview }

func (e *ErrQuoteRequired) Error() string {
	return "breakthrough: acceptLossMilli required for a tribulation"
}

// Result is returned after an attempt (authoritative).
type Result struct {
	Success          bool                    `json:"success"`
	Kind             realm.BreakthroughKind  `json:"kind"`
	From             string                  `json:"from"`
	To               string                  `json:"to"`
	SuccessBP        int64                   `json:"successBp"`
	Consumed         []realm.ItemRequirement `json:"consumed"`
	Tribulation      bool                    `json:"tribulation"`
	Guaranteed       bool                    `json:"guaranteed"`
	LossMilli        int64                   `json:"lossMilli"` // cultivation removed by a failed tribulation
	StageID          string                  `json:"stageId"`   // stage after the attempt
	CultivationMilli int64                   `json:"cultivationMilli"`
	StagesAdvanced   []string                `json:"stagesAdvanced"` // minor stages passed by the carry-over
	FailStreak       int32                   `json:"failStreak"`
	CooldownUntil    *time.Time              `json:"cooldownUntil"`
	Replayed         bool                    `json:"replayed"`
}

// Service resolves breakthroughs (GDD §6, §20).
type Service struct {
	Pool        *pgxpool.Pool
	Cultivation *cultivation.Service
	RNG         RNG
}

func (s *Service) cat() *realm.Catalogue { return s.Cultivation.Realms }

// Preview is read-only: it evaluates the SAME settlement the attempt runs first (projection: idle, caps,
// catch-up, bottleneck, auto-advance), so the disclosed loss is the loss the attempt would apply now.
func (s *Service) Preview(ctx context.Context, userID uuid.UUID) (Preview, error) {
	q := db.New(s.Pool)
	ch, err := q.GetCharacterByUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Preview{}, cultivation.ErrNoCharacter
	} else if err != nil {
		return Preview{}, err
	}
	p, err := q.GetProgression(ctx, ch.ID)
	if err != nil {
		return Preview{}, err
	}
	proj, err := s.Cultivation.Project(ctx, q, userID, p)
	if err != nil {
		return Preview{}, err
	}
	stage := proj.Credit.After.Stage
	inv, err := inventory(ctx, q, ch.ID, s.cat().LastOfRealm(stage), false)
	if err != nil {
		return Preview{}, err
	}
	pv := Evaluate(s.cat(), stage, State{
		CultivationMilli: proj.Credit.After.StoredMilli, FailStreak: int64(p.BreakthroughFailStreak),
		CooldownUntil: p.BreakthroughCooldownUntil, Inventory: inv,
	}, s.Cultivation.Now().UTC())
	s.quote(&pv, stage, proj.Credit.After.StoredMilli)
	return pv, nil
}

// quote marks the preview as settlement-inclusive and fills the tribulation guard bound.
func (s *Service) quote(pv *Preview, stage *realm.Stage, stored int64) {
	pv.SettlementIncluded = true
	if t := pv.Tribulation; t != nil && !t.Guaranteed {
		cfg := s.Cultivation.Contribution
		t.MaxLossMilli = QuoteBoundMilli(s.cat(), stage, stored, cfg.IdleRateMilliPerHour, cfg.CatchUp.MultiplierBP)
	}
}

// Attempt performs one idempotent realm breakthrough attempt.
//
// Policy (docs/decisions.md): items are consumed on success AND failure. Pre-tribulation majors: failure keeps
// cultivation, +1 pity streak, rule cooldown. Tribulation: failure loses loss_bp of the current stage's stored
// cultivation (never drops a stage), +1 streak, 24h cooldown; after guarantee_after consecutive fails the next
// attempt succeeds. Success carries cultivation above the requirement into the next realm (auto-advanced) and
// releases any late-join reserve into the new realm. acceptLossMilli (optional) is a GUARD only: if the loss the
// server would apply is larger, nothing is written and ErrQuoteChanged returns a fresh preview.
func (s *Service) Attempt(ctx context.Context, userID uuid.UUID, idemKey string, acceptLossMilli *int64) (Result, error) {
	var out Result
	err := postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		ch, err := q.GetCharacterByUser(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return cultivation.ErrNoCharacter
		} else if err != nil {
			return err
		}
		// 1. Lock order: progression → daily_contribution (inside settle) → inventory rows (by item_id).
		p, err := q.GetProgressionForUpdate(ctx, ch.ID)
		if err != nil {
			return fmt.Errorf("lock progression: %w", err)
		}
		// 2. Idempotency after lock.
		if att, err := q.GetBreakthroughAttemptByKey(ctx, db.GetBreakthroughAttemptByKeyParams{CharacterID: ch.ID, IdempotencyKey: idemKey}); err == nil {
			var consumed []realm.ItemRequirement
			_ = json.Unmarshal(att.Consumed, &consumed)
			out = Result{Success: att.Success, Kind: realm.BreakthroughKind(att.Kind), From: att.FromStageID, To: att.ToStageID,
				SuccessBP: int64(att.SuccessBp), Consumed: consumed, Tribulation: att.Tribulation, Guaranteed: att.Guaranteed,
				LossMilli: att.LossMilli, StageID: p.StageID, CultivationMilli: p.CultivationMilli, StagesAdvanced: []string{},
				FailStreak: p.BreakthroughFailStreak, CooldownUntil: p.BreakthroughCooldownUntil, Replayed: true}
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// 3. Bring cultivation up to date inside the same tx (same pure path as the preview).
		p, _, err = s.Cultivation.SettleLocked(ctx, q, userID, p, "bt:"+idemKey)
		if err != nil {
			return err
		}
		stage, err := s.cat().Stage(p.StageID)
		if err != nil {
			return err
		}
		inv, err := inventory(ctx, q, ch.ID, s.cat().LastOfRealm(stage), true)
		if err != nil {
			return err
		}
		now := s.Cultivation.Now().UTC()
		pv := Evaluate(s.cat(), stage, State{
			CultivationMilli: p.CultivationMilli, FailStreak: int64(p.BreakthroughFailStreak), CooldownUntil: p.BreakthroughCooldownUntil, Inventory: inv,
		}, now)
		s.quote(&pv, stage, p.CultivationMilli)
		if !pv.Ready {
			return &ErrNotReady{Preview: pv}
		}
		// Guard (compare only — the applied loss is always the server's LossMilli, never min(loss, accept)).
		trib := pv.Tribulation
		if trib != nil && !trib.Guaranteed {
			if acceptLossMilli == nil {
				return &ErrQuoteRequired{Preview: pv}
			}
			if trib.LossMilli > *acceptLossMilli {
				return &ErrQuoteChanged{Preview: pv} // rolls back the settlement too: nothing is written
			}
		}
		next := s.cat().Next(stage)

		// 4. Consume items (both outcomes).
		for _, req := range pv.Consumes {
			left, err := q.ConsumeInventory(ctx, db.ConsumeInventoryParams{CharacterID: ch.ID, ItemID: req.ItemID, Qty: req.Quantity})
			if err != nil {
				return fmt.Errorf("consume %s: %w", req.ItemID, err) // ErrNoRows here would mean a race; lock prevents it
			}
			if err := q.InsertInventoryTransaction(ctx, db.InsertInventoryTransactionParams{
				CharacterID: ch.ID, ItemID: req.ItemID, Delta: -req.Quantity, QuantityAfter: left,
				Reason: "breakthrough_cost", RefType: text("breakthrough"), RefID: text(stage.ID), IdempotencyKey: "bt:" + idemKey,
			}); err != nil {
				return err
			}
		}

		// 5. Server-side roll, persisted for audit (odds breakdown, loss, guarantee).
		o := Roll(pv.SuccessBP, s.RNG)
		guaranteed := trib != nil && trib.Guaranteed
		var loss int64
		if trib != nil && !o.Success {
			loss = trib.LossMilli
		}
		consumed, _ := json.Marshal(pv.Consumes)
		odds, _ := json.Marshal(pv.Odds)
		if _, err := q.InsertBreakthroughAttempt(ctx, db.InsertBreakthroughAttemptParams{
			CharacterID: ch.ID, IdempotencyKey: idemKey, FromStageID: stage.ID, ToStageID: next.ID, Kind: string(pv.Kind),
			SuccessBp: int32(o.SuccessBP), RollBp: int32(o.RollBP), Success: o.Success, Consumed: consumed,
			Tribulation: trib != nil, Guaranteed: guaranteed && o.Success, CultivationBeforeMilli: p.CultivationMilli, LossMilli: loss, Odds: odds,
		}); err != nil {
			return err
		}

		if err := s.Cultivation.Tasks.Record(ctx, q, ch.ID, now, dailytask.BreakthroughAttempt, 0); err != nil {
			return err
		}
		out = Result{Success: o.Success, Kind: pv.Kind, From: stage.ID, To: next.ID, SuccessBP: o.SuccessBP, Consumed: pv.Consumes,
			Tribulation: trib != nil, Guaranteed: guaranteed && o.Success, LossMilli: loss, StagesAdvanced: []string{}}
		if o.Success {
			return s.applySuccess(ctx, q, ch, p, stage, next, idemKey, now, &out)
		}
		var cd time.Time
		if trib != nil {
			cd = now.Add(time.Duration(trib.CooldownSeconds) * time.Second)
			if err := q.ApplyTribulationFailure(ctx, db.ApplyTribulationFailureParams{CharacterID: ch.ID, LossMilli: loss, CooldownUntil: &cd}); err != nil {
				return err
			}
			if err := cultivation.EnqueueLeaderboard(ctx, q, ch.ID, leaderboard.Score(stage, p.CultivationMilli-loss)); err != nil {
				return err
			}
		} else {
			cd = now.Add(time.Duration(pv.CooldownSeconds) * time.Second)
			if err := q.RecordBreakthroughFailure(ctx, db.RecordBreakthroughFailureParams{CharacterID: ch.ID, BreakthroughCooldownUntil: &cd}); err != nil {
				return err
			}
		}
		out.StageID, out.CultivationMilli, out.FailStreak, out.CooldownUntil = stage.ID, p.CultivationMilli-loss, p.BreakthroughFailStreak+1, &cd
		return nil
	})
	return out, err
}

// applySuccess moves into the next realm: carry-over (≤ 20% of the last stage) is auto-advanced, then the
// late-join reserve is released into the new realm's bottleneck room.
func (s *Service) applySuccess(ctx context.Context, q *db.Queries, ch db.Character, p db.CharacterProgression,
	from, next *realm.Stage, idemKey string, now time.Time, out *Result) error {
	cat := s.cat()
	carry := max(0, p.CultivationMilli-from.RequiredMilli) // ≤ overflow by the bottleneck cap
	st, stored, passed := cat.Advance(next, carry)
	after, released, more := cultivation.ReleaseGrant(cat, cultivation.Progress{Stage: st, StoredMilli: stored, GrantReserveMilli: p.GrantReserveMilli})
	passed = append(passed, more...)
	if err := q.AdvanceStage(ctx, db.AdvanceStageParams{
		CharacterID: ch.ID, RealmID: after.Stage.RealmID, StageID: after.Stage.ID, CultivationMilli: after.StoredMilli,
		GrantReserveMilli: after.GrantReserveMilli, StageReachedAt: now,
	}); err != nil {
		return err
	}
	if released > 0 {
		if _, err := q.InsertCultivationCredit(ctx, db.InsertCultivationCreditParams{
			CharacterID: ch.ID, GameDay: gameday.PG(now), Source: string(contribution.SourceLateJoinGrant), StageID: next.ID,
			RawMilli: released, MultiplierBp: fixedpoint.BPScale, CreditedMilli: released,
			RefType: text("breakthrough"), RefID: text(from.ID), IdempotencyKey: string(contribution.SourceLateJoinGrant) + ":breakthrough:" + idemKey,
		}); err != nil {
			return fmt.Errorf("insert grant release: %w", err)
		}
	}
	if err := s.recordSuccess(ctx, q, ch, from, next, now); err != nil {
		return err
	}
	if err := cultivation.RecordStagesReached(ctx, q, cat, ch.ID, passed, "auto", now); err != nil {
		return err
	}
	if err := cultivation.EnqueueLeaderboard(ctx, q, ch.ID, leaderboard.Score(after.Stage, after.StoredMilli)); err != nil {
		return err
	}
	out.StageID, out.CultivationMilli = after.Stage.ID, after.StoredMilli
	for _, ps := range passed {
		out.StagesAdvanced = append(out.StagesAdvanced, ps.ID)
	}
	return nil
}

func (s *Service) recordSuccess(ctx context.Context, q *db.Queries, ch db.Character, from, to *realm.Stage, now time.Time) error {
	payload, _ := json.Marshal(map[string]any{"from": from.ID, "tribulation": from.Tribulation})
	for _, kind := range []string{"stage_reached", "realm_reached"} {
		if err := q.InsertProgressionEvent(ctx, db.InsertProgressionEventParams{CharacterID: ch.ID, Kind: kind, StageID: text(to.ID), Payload: payload, OccurredAt: now}); err != nil {
			return err
		}
	}
	ann, _ := json.Marshal(map[string]any{"characterId": ch.ID.String(), "name": ch.Name, "realmId": to.RealmID, "stageId": to.ID, "tribulation": from.Tribulation})
	return q.InsertOutboxEvent(ctx, db.InsertOutboxEventParams{Topic: "announcement.major_breakthrough", AggregateID: ch.ID.String(), Payload: ann})
}

// inventory reads (optionally locks) the requirement items of the realm's major breakthrough.
func inventory(ctx context.Context, q *db.Queries, charID uuid.UUID, last *realm.Stage, lock bool) (map[string]int64, error) {
	ids := make([]string, 0, len(last.Breakthrough.Requirements))
	for _, r := range last.Breakthrough.Requirements {
		ids = append(ids, r.ItemID)
	}
	inv := map[string]int64{}
	if len(ids) == 0 {
		return inv, nil
	}
	if lock {
		rows, err := q.GetInventoryItemsForUpdate(ctx, db.GetInventoryItemsForUpdateParams{CharacterID: charID, ItemIds: ids})
		for _, r := range rows {
			inv[r.ItemID] = r.Quantity
		}
		return inv, err
	}
	rows, err := q.GetInventoryItems(ctx, db.GetInventoryItemsParams{CharacterID: charID, ItemIds: ids})
	for _, r := range rows {
		inv[r.ItemID] = r.Quantity
	}
	return inv, err
}

func text(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
