// Package cultivation implements timestamp-based idle settlement and the SINGLE credit path for điểm tu vi
// (docs/design/data-model-the-gioi-chung.md §2.4, §4).
//
// There is NO per-player ticker: idle progress is computed from server timestamps when the player logs in or
// performs an action that needs up-to-date progression. Every source of points goes through CreditLocked.
package cultivation

import (
	"time"

	"tiendao/game/internal/fixedpoint"
)

// SecondsPerHour is the divisor of the hourly idle rate; rate_carry keeps the sub-milli remainder (0..3599).
const SecondsPerHour = 3600

// SettleInput is a pure snapshot; Settle has no I/O so it is trivially testable.
type SettleInput struct {
	Now              time.Time // SERVER time. Never client time.
	LastCultivatedAt time.Time
	RateCarry        int64         // remainder of rate × seconds not yet turned into milli, 0..3599
	RateMilliPerHour int64         // idle rate (contribution.yaml)
	OfflineCap       time.Duration // Tiên Duyên window (+ pending Tụ Linh Phù) at settlement time
}

// SettleResult is the raw idle amount; caps/bottleneck are applied afterwards by Credit.
type SettleResult struct {
	ElapsedSeconds   int64
	EligibleSeconds  int64
	RawMilli         int64
	RateCarry        int64
	LastCultivatedAt time.Time
	HitOfflineCap    bool
	ClockAnomaly     bool // last_cultivated_at in the future → treated as 0 elapsed
}

// Settle computes raw idle points: raw = (rate × eligible_seconds + carry) / 3600, carry = … % 3600.
func Settle(in SettleInput) SettleResult {
	res := SettleResult{LastCultivatedAt: in.Now, RateCarry: in.RateCarry}

	elapsed := in.Now.Sub(in.LastCultivatedAt)
	if elapsed < 0 {
		res.ClockAnomaly = true
		res.LastCultivatedAt = in.LastCultivatedAt // never move the stored timestamp backwards nor past now
		return res
	}
	res.ElapsedSeconds = int64(elapsed / time.Second)

	eligible := res.ElapsedSeconds
	if capS := int64(in.OfflineCap / time.Second); eligible > capS {
		eligible = max(0, capS)
		res.HitOfflineCap = true
	}
	res.EligibleSeconds = eligible

	total := fixedpoint.SatAdd(fixedpoint.MulDiv(in.RateMilliPerHour, eligible, 1), in.RateCarry)
	res.RawMilli = total / SecondsPerHour
	res.RateCarry = total % SecondsPerHour
	// Only whole seconds were consumed; keep the sub-second part for the next settlement.
	res.LastCultivatedAt = in.LastCultivatedAt.Add(time.Duration(res.ElapsedSeconds) * time.Second)
	return res
}
