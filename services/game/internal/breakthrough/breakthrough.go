// Package breakthrough implements readiness checks, odds and server-side resolution of REALM (major)
// breakthroughs, including the tribulation (lôi kiếp) from tribulation.from_tier on (GDD §6, §20;
// docs/design/data-model-the-gioi-chung.md §3.6). Minor stages auto-advance in the credit path and never come here.
// Pure functions only; persistence/transactions live in the service layer.
package breakthrough

import (
	"time"

	"tiendao/game/internal/fixedpoint"
	"tiendao/game/internal/realm"
)

// RNG is injected so resolution is deterministic in tests and auditable in prod.
// Production uses math/rand/v2 seeded from crypto/rand (see NewRNG).
type RNG interface {
	IntN(n int) int
}

// Missing describes one unmet requirement, with the exact shortfall for the UI.
// For kind "cultivation", Have/Need are milli measured from the start of the current realm.
type Missing struct {
	Kind    string     `json:"kind"` // cultivation | item | cooldown | final_stage
	ItemID  string     `json:"itemId,omitempty"`
	Have    int64      `json:"have"`
	Need    int64      `json:"need"`
	ReadyAt *time.Time `json:"readyAt,omitempty"`
}

// OddsPart is one disclosed component of the success chance.
type OddsPart struct {
	Source string `json:"source"` // base | pity | surplus | items | protector
	BP     int64  `json:"bp"`
}

// TribulationPreview discloses the tribulation stakes before the attempt.
type TribulationPreview struct {
	Name           string `json:"name"`
	Guaranteed     bool   `json:"guaranteed"`
	FailStreak     int64  `json:"failStreak"`
	GuaranteeAfter int    `json:"guaranteeAfter"`
	LossBP         int64  `json:"lossBp"`
	LossMilli      int64  `json:"lossMilli"` // applied on failure only, if attempted now; 0 when guaranteed
	// MaxLossMilli is the guard bound the client sends as acceptLossMilli: the loss after quote_window_seconds of
	// further idle accrual (×catch-up, capped by bình cảnh). ≥ LossMilli. Never used as a value by the server.
	MaxLossMilli    int64 `json:"maxLossMilli"`
	CooldownSeconds int64 `json:"cooldownSeconds"`
}

// Preview is the "quote" shown before the player confirms. It includes the pending idle settlement
// (SettlementIncluded), so the shown loss equals the loss the attempt would apply now.
type Preview struct {
	From                      string                  `json:"from"`
	To                        string                  `json:"to,omitempty"`
	Kind                      realm.BreakthroughKind  `json:"kind,omitempty"`
	Ready                     bool                    `json:"ready"`
	Missing                   []Missing               `json:"missing"`
	Consumes                  []realm.ItemRequirement `json:"consumes"`
	SettlementIncluded        bool                    `json:"settlementIncluded"`
	ProjectedCultivationMilli int64                   `json:"projectedCultivationMilli"`
	SuccessBP                 int64                   `json:"successBp"` // effective, incl. pity/bonuses — always disclosed
	Odds                      []OddsPart              `json:"odds"`
	CooldownSeconds           int64                   `json:"cooldownSeconds"` // started on failure
	Tribulation               *TribulationPreview     `json:"tribulation,omitempty"`
}

// State is the minimal character state needed to evaluate a breakthrough.
type State struct {
	CultivationMilli int64 // stored in the current stage (after the pending settlement)
	FailStreak       int64
	CooldownUntil    *time.Time
	Inventory        map[string]int64
	// Phase 3 slots — always 0 in Phase 1 (pháp bảo/đan hộ thân, online fellow faction members).
	ItemBonusBP      int64
	ProtectorMembers int64
}

// EffectiveSuccessBP = base + min(pityMax, failStreak × pityStep), clamped to [0, 10000]. Pre-tribulation majors.
func EffectiveSuccessBP(rule realm.BreakthroughRule, failStreak, pityMaxBP int64) int64 {
	bonus := min(pityMaxBP, failStreak*rule.PityStepBP)
	return fixedpoint.Clamp(rule.SuccessBP+bonus, 0, fixedpoint.BPScale)
}

// SurplusBonusBP: each 1% stored above the requirement → bp_per_surplus_bp/10000 %, capped at max_bp.
func SurplusBonusBP(t realm.Tribulation, required, stored int64) int64 {
	if required <= 0 || stored <= required {
		return 0
	}
	surplusBP := fixedpoint.MulDiv(stored-required, fixedpoint.BPScale, required)
	return min(t.SurplusBonus.MaxBP, fixedpoint.MulBP(surplusBP, t.SurplusBonus.BPPerSurplusBP))
}

// TribulationOdds returns the success chance and its breakdown for a tribulation stage:
// guaranteed after `guarantee_after` consecutive fails; else min(max_success, base + min(max_bonus, Σ bonuses)).
func TribulationOdds(cat *realm.Catalogue, last *realm.Stage, st State) (int64, []OddsPart, bool) {
	t := cat.Tribulation()
	base := cat.TribulationBaseBP(last)
	surplus := SurplusBonusBP(t, last.RequiredMilli, st.CultivationMilli)
	items := min(t.ItemBonusMaxBP, max(0, st.ItemBonusBP))
	protector := min(t.ProtectorMaxBP, max(0, st.ProtectorMembers)*t.ProtectorPerBP)
	odds := []OddsPart{{"base", base}, {"surplus", surplus}, {"items", items}, {"protector", protector}}
	if st.FailStreak >= int64(t.GuaranteeAfter) {
		return fixedpoint.BPScale, odds, true
	}
	bonus := min(t.MaxTotalBonusBP, surplus+items+protector)
	return min(t.MaxSuccessBP, base+bonus), odds, false
}

// LossMilli is what a failed tribulation removes from the current stage: stored × loss_bp / 10000.
func LossMilli(cat *realm.Catalogue, stored int64) int64 {
	return fixedpoint.MulBP(max(0, stored), cat.Tribulation().LossBP)
}

// QuoteBoundMilli is the guard bound for a tribulation quote: the loss after `window` more seconds of idle accrual
// at rateMilliPerHour × multiplierBP, with stored capped by the stage's bình cảnh (never below the current loss).
func QuoteBoundMilli(cat *realm.Catalogue, s *realm.Stage, stored, rateMilliPerHour, multiplierBP int64) int64 {
	window := cat.Tribulation().QuoteWindowSeconds
	accrual := fixedpoint.MulBP(fixedpoint.MulDiv(rateMilliPerHour, window, 3600)+1, max(multiplierBP, fixedpoint.BPScale))
	bound := stored + accrual
	if s.CapMilli > 0 {
		bound = min(bound, s.CapMilli)
	}
	return LossMilli(cat, max(stored, bound))
}

// Evaluate builds a Preview for a character at stage `from` (any stage of the realm) at server time now.
// The breakthrough is always the realm's major one; minor stages before it auto-advance.
func Evaluate(cat *realm.Catalogue, from *realm.Stage, st State, now time.Time) Preview {
	p := Preview{From: from.ID, Missing: []Missing{}, Consumes: []realm.ItemRequirement{}, Odds: []OddsPart{},
		ProjectedCultivationMilli: st.CultivationMilli}
	last := cat.LastOfRealm(from)
	next := cat.Next(last)
	if next == nil || last.NextKind != realm.Major {
		p.Missing = append(p.Missing, Missing{Kind: "final_stage"})
		return p
	}
	p.To, p.Kind = next.ID, realm.Major
	if len(last.Breakthrough.Requirements) > 0 {
		p.Consumes = last.Breakthrough.Requirements
	}
	onLast := from == last
	surplusStored := int64(0)
	if onLast {
		surplusStored = st.CultivationMilli
	}
	if last.Tribulation {
		t := cat.Tribulation()
		bp, odds, guaranteed := TribulationOdds(cat, last, State{CultivationMilli: surplusStored, FailStreak: st.FailStreak,
			ItemBonusBP: st.ItemBonusBP, ProtectorMembers: st.ProtectorMembers})
		p.SuccessBP, p.Odds, p.CooldownSeconds = bp, odds, t.CooldownSeconds
		tp := &TribulationPreview{Name: t.Names[from.Path], Guaranteed: guaranteed, FailStreak: st.FailStreak,
			GuaranteeAfter: t.GuaranteeAfter, LossBP: t.LossBP, CooldownSeconds: t.CooldownSeconds}
		if !guaranteed {
			tp.LossMilli = LossMilli(cat, st.CultivationMilli)
			tp.MaxLossMilli = tp.LossMilli
		}
		p.Tribulation = tp
	} else {
		rule := last.Breakthrough
		p.SuccessBP = EffectiveSuccessBP(rule, st.FailStreak, cat.Settings.PityMaxBonusBP)
		p.Odds = []OddsPart{{"base", rule.SuccessBP}, {"pity", p.SuccessBP - rule.SuccessBP}}
		p.CooldownSeconds = rule.CooldownSeconds
	}

	first := cat.FirstOfRealm(from)
	have := from.CumulativeBeforeMilli - first.CumulativeBeforeMilli + st.CultivationMilli
	need := last.CumulativeBeforeMilli - first.CumulativeBeforeMilli + last.RequiredMilli
	if !onLast || st.CultivationMilli < last.RequiredMilli {
		p.Missing = append(p.Missing, Missing{Kind: "cultivation", Have: min(have, need-1), Need: need})
	}
	for _, req := range last.Breakthrough.Requirements {
		if have := st.Inventory[req.ItemID]; have < req.Quantity {
			p.Missing = append(p.Missing, Missing{Kind: "item", ItemID: req.ItemID, Have: have, Need: req.Quantity})
		}
	}
	if st.CooldownUntil != nil && now.Before(*st.CooldownUntil) {
		at := *st.CooldownUntil
		p.Missing = append(p.Missing, Missing{Kind: "cooldown", ReadyAt: &at})
	}
	p.Ready = len(p.Missing) == 0
	return p
}

// Outcome is the auditable result persisted in breakthrough_attempts.
type Outcome struct {
	SuccessBP int64
	RollBP    int64 // 0..9999
	Success   bool
}

// Roll resolves an attempt. Call ONLY after Evaluate(...).Ready and inside the locking tx.
func Roll(successBP int64, rng RNG) Outcome {
	roll := int64(rng.IntN(fixedpoint.BPScale))
	return Outcome{SuccessBP: successBP, RollBP: roll, Success: roll < successBP}
}
