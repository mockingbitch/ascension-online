package cultivation

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestSettle(t *testing.T) {
	tests := []struct {
		name string
		in   SettleInput
		want SettleResult
	}{
		{
			name: "one hour at 2083/h",
			in:   SettleInput{Now: t0.Add(time.Hour), LastCultivatedAt: t0, RateMilliPerHour: 2083, OfflineCap: 8 * time.Hour},
			want: SettleResult{ElapsedSeconds: 3600, EligibleSeconds: 3600, RawMilli: 2083, LastCultivatedAt: t0.Add(time.Hour)},
		},
		{
			name: "offline cap applies (8h of 10h)",
			in:   SettleInput{Now: t0.Add(10 * time.Hour), LastCultivatedAt: t0, RateMilliPerHour: 2083, OfflineCap: 8 * time.Hour},
			want: SettleResult{ElapsedSeconds: 36000, EligibleSeconds: 28800, RawMilli: 16664, LastCultivatedAt: t0.Add(10 * time.Hour), HitOfflineCap: true},
		},
		{
			name: "sub-milli carry",
			in:   SettleInput{Now: t0.Add(100 * time.Second), LastCultivatedAt: t0, RateMilliPerHour: 2083, RateCarry: 100, OfflineCap: time.Hour},
			want: SettleResult{ElapsedSeconds: 100, EligibleSeconds: 100, RawMilli: 57, RateCarry: 3200, LastCultivatedAt: t0.Add(100 * time.Second)},
		},
		{
			name: "future timestamp is a clock anomaly, no reward",
			in:   SettleInput{Now: t0, LastCultivatedAt: t0.Add(time.Hour), RateMilliPerHour: 2083, RateCarry: 7, OfflineCap: time.Hour},
			want: SettleResult{RateCarry: 7, LastCultivatedAt: t0.Add(time.Hour), ClockAnomaly: true},
		},
		{
			name: "sub-second elapsed is preserved for next settlement",
			in:   SettleInput{Now: t0.Add(1500 * time.Millisecond), LastCultivatedAt: t0, RateMilliPerHour: 3600, OfflineCap: time.Hour},
			want: SettleResult{ElapsedSeconds: 1, EligibleSeconds: 1, RawMilli: 1, LastCultivatedAt: t0.Add(time.Second)},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Settle(tc.in); got != tc.want {
				t.Fatalf("\n got %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

// Settling in many small steps must yield EXACTLY the single settlement (carry makes it lossless).
func TestSettleSplitEqualsSingle(t *testing.T) {
	in := SettleInput{LastCultivatedAt: t0, RateMilliPerHour: 2083, OfflineCap: 24 * time.Hour}
	single := in
	single.Now = t0.Add(10_000 * time.Second)
	whole := Settle(single)

	cur := in
	var total int64
	for i := 1; i <= 10_000; i += 7 {
		cur.Now = t0.Add(time.Duration(i) * time.Second)
		r := Settle(cur)
		total += r.RawMilli
		cur.RateCarry, cur.LastCultivatedAt = r.RateCarry, r.LastCultivatedAt
	}
	cur.Now = single.Now
	total += Settle(cur).RawMilli
	if total != whole.RawMilli {
		t.Fatalf("split=%d single=%d", total, whole.RawMilli)
	}
}

// 24h of eligible time at the shipped rate stays below 50 points (rate × 24 ≤ idle cap).
func TestDailyIdleBudget(t *testing.T) {
	r := Settle(SettleInput{Now: t0.Add(24 * time.Hour), LastCultivatedAt: t0, RateMilliPerHour: 2083, OfflineCap: 24 * time.Hour})
	if r.RawMilli != 49_992 {
		t.Fatalf("24h raw = %d", r.RawMilli)
	}
}
