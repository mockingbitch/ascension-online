package cultivation

import (
	"testing"
	"time"
)

// Gieo quẻ buff (§9.4): only eligible seconds inside [Start, End) get the bonus; never retroactive.
func TestSettleBuff(t *testing.T) {
	const rate = 3600 // 1 milli per second → easy arithmetic
	in := func(last, now time.Duration, cap time.Duration, b Buff) SettleInput {
		return SettleInput{Now: t0.Add(now), LastCultivatedAt: t0.Add(last), RateMilliPerHour: rate, OfflineCap: cap, Buff: b}
	}
	h := time.Hour
	buff := func(from, to time.Duration, bp int64) Buff {
		return Buff{BonusBP: bp, Start: t0.Add(from), End: t0.Add(to)}
	}
	for _, tc := range []struct {
		name      string
		in        SettleInput
		raw, secs int64
	}{
		{"no buff", in(0, 2*h, 8*h, Buff{}), 7200, 0},
		{"full overlap +20%", in(0, 2*h, 8*h, buff(0, 24*h, 2000)), 7200 + 1440, 7200},
		{"buff starts mid-window", in(0, 2*h, 8*h, buff(h, 24*h, 1000)), 7200 + 360, 3600},
		{"buff ended before window", in(5*h, 6*h, 8*h, buff(0, 4*h, 2000)), 3600, 0},
		{"buff starts after now (never retroactive)", in(0, 2*h, 8*h, buff(3*h, 24*h, 2000)), 7200, 0},
		{"capped window: only the first 8h are eligible", in(0, 12*h, 8*h, buff(6*h, 24*h, 1000)), 28800 + 720, 7200},
		{"zero bonus", in(0, 2*h, 8*h, buff(0, 24*h, 0)), 7200, 0},
	} {
		got := Settle(tc.in)
		if got.RawMilli != tc.raw || got.BuffSeconds != tc.secs {
			t.Errorf("%s: raw %d secs %d, want %d %d", tc.name, got.RawMilli, got.BuffSeconds, tc.raw, tc.secs)
		}
	}
}

// Invariant: a buff never lowers the idle amount, and splitting a settlement at any point never yields more than
// one settlement over the same span (carry included).
func TestSettleBuffSplitNeverExceedsSingle(t *testing.T) {
	b := Buff{BonusBP: 1000, Start: t0.Add(90 * time.Minute), End: t0.Add(20 * time.Hour)}
	single := Settle(SettleInput{Now: t0.Add(6 * time.Hour), LastCultivatedAt: t0, RateMilliPerHour: 2083, OfflineCap: 8 * time.Hour, Buff: b})
	for split := time.Duration(1); split < 6*time.Hour; split += 17 * time.Minute {
		a := Settle(SettleInput{Now: t0.Add(split), LastCultivatedAt: t0, RateMilliPerHour: 2083, OfflineCap: 8 * time.Hour, Buff: b})
		c := Settle(SettleInput{Now: t0.Add(6 * time.Hour), LastCultivatedAt: a.LastCultivatedAt, RateCarry: a.RateCarry,
			RateMilliPerHour: 2083, OfflineCap: 8 * time.Hour, Buff: b})
		if a.RawMilli+c.RawMilli > single.RawMilli {
			t.Fatalf("split at %v: %d > %d", split, a.RawMilli+c.RawMilli, single.RawMilli)
		}
	}
	plain := Settle(SettleInput{Now: t0.Add(6 * time.Hour), LastCultivatedAt: t0, RateMilliPerHour: 2083, OfflineCap: 8 * time.Hour})
	if single.RawMilli < plain.RawMilli {
		t.Fatal("buff lowered the idle amount")
	}
}
