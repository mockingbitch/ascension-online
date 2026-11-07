package world

import (
	"testing"
	"time"
)

func TestNextReachRule(t *testing.T) {
	c := shipped(t) // min 70%, max 150%
	start := time.Date(2027, 1, 10, 1, 0, 0, 0, time.UTC)
	day := func(d float64) time.Time { return start.Add(time.Duration(d * float64(24*time.Hour))) }
	only := func(ids ...string) map[string]bool {
		m := map[string]bool{}
		for _, id := range ids {
			m[id] = true
		}
		return m
	}
	i64 := func(v int64) *int64 { return &v }
	in := func(v int) *int { return &v }
	k1 := only("khai_giang") // next: giang_dao_2 (day 30, 2,010 pts) → window [21, 45]
	cases := []struct {
		name     string
		s        Snapshot
		now      time.Time
		want     string
		reason   Reason
		thrMilli int64
	}{
		{"not started", Snapshot{}, day(1), "", "", 0},
		{"launch milestone at day 0", Snapshot{StartedAt: &start}, start, "khai_giang", ReasonThreshold, 0},
		{"clock before start", Snapshot{StartedAt: &start}, start.Add(-time.Second), "", "", 0},
		{"threshold met before 70% floor", Snapshot{StartedAt: &start, Reached: k1, ProgressMilli: 5_000_000}, day(20.99), "", "", 0},
		{"threshold met at 70% floor", Snapshot{StartedAt: &start, Reached: k1, ProgressMilli: 2_010_000}, day(21), "giang_dao_2", ReasonThreshold, 2_010_000},
		{"one milli short", Snapshot{StartedAt: &start, Reached: k1, ProgressMilli: 2_009_999}, day(30), "", "", 0},
		{"150% ceiling regardless of progress", Snapshot{StartedAt: &start, Reached: k1}, day(45), "giang_dao_2", ReasonMaxTime, 2_010_000},
		{"just before ceiling", Snapshot{StartedAt: &start, Reached: k1}, day(45).Add(-time.Second), "", "", 0},
		{"previous required: only the first unreached is considered",
			Snapshot{StartedAt: &start, Reached: only("khai_giang", "giang_dao_3"), ProgressMilli: 1 << 40}, day(400), "giang_dao_2", ReasonThreshold, 2_010_000},
		{"at most one per tick even far behind", Snapshot{StartedAt: &start, ProgressMilli: 1 << 40}, day(1000), "khai_giang", ReasonThreshold, 0},
		{"all reached", Snapshot{StartedAt: &start, Reached: only("khai_giang", "giang_dao_2", "giang_dao_3", "dai_chien", "nu_oa",
			"thanh_nhan", "nhan_toc", "phong_than", "thien_dinh", "tay_du")}, day(1000), "", "", 0},
		{"world paused", Snapshot{StartedAt: &start, Reached: k1, Paused: true}, day(100), "", "", 0},
		{"milestone paused holds max_time", Snapshot{StartedAt: &start, Reached: k1,
			Overrides: map[string]Override{"giang_dao_2": {Paused: true}}}, day(100), "", "", 0},
		{"admin force ignores time and progress", Snapshot{StartedAt: &start, Reached: k1,
			Overrides: map[string]Override{"giang_dao_2": {Force: true}}}, day(1), "giang_dao_2", ReasonAdmin, 2_010_000},
		{"force on a later milestone waits for order", Snapshot{StartedAt: &start, Reached: k1,
			Overrides: map[string]Override{"giang_dao_3": {Force: true}}}, day(1), "", "", 0},
		{"override threshold", Snapshot{StartedAt: &start, Reached: k1, ProgressMilli: 1_000_000,
			Overrides: map[string]Override{"giang_dao_2": {ThresholdMilli: i64(1_000_000)}}}, day(21), "giang_dao_2", ReasonThreshold, 1_000_000},
		{"override target day shifts the floor", Snapshot{StartedAt: &start, Reached: k1, ProgressMilli: 3_000_000,
			Overrides: map[string]Override{"giang_dao_2": {TargetDay: in(40)}}}, day(27), "", "", 0},
		{"override target day shifts the ceiling", Snapshot{StartedAt: &start, Reached: k1,
			Overrides: map[string]Override{"giang_dao_2": {TargetDay: in(40)}}}, day(60), "giang_dao_2", ReasonMaxTime, 2_010_000},
	}
	for _, tc := range cases {
		d, ok := c.Next(tc.s, tc.now)
		got := ""
		if ok {
			got = d.Milestone.ID
		}
		if got != tc.want || (ok && (d.Reason != tc.reason || d.ThresholdMilli != tc.thrMilli)) {
			t.Errorf("%s: got %q %s %d, want %q %s %d", tc.name, got, d.Reason, d.ThresholdMilli, tc.want, tc.reason, tc.thrMilli)
		}
	}
}

func TestWindowProgressEstimate(t *testing.T) {
	c := shipped(t)
	start := time.Date(2027, 1, 10, 1, 0, 0, 0, time.UTC)
	e, l := c.Window(30, start)
	if !e.Equal(start.AddDate(0, 0, 21)) || !l.Equal(start.AddDate(0, 0, 45)) {
		t.Fatalf("window %v %v", e, l)
	}
	if ProgressBP(0, 0) != 10_000 || ProgressBP(1_005_000, 2_010_000) != 5_000 || ProgressBP(3, 2) != 10_000 || ProgressBP(0, 5) != 0 {
		t.Fatal("progress bp")
	}
	now := start.AddDate(0, 0, 10)
	if Estimate(now, 0, 2_010_000, 0, e, l) != nil {
		t.Fatal("no pace → nil")
	}
	if got := Estimate(now, 670_000, 2_010_000, 67_000, e, l); !got.Equal(start.AddDate(0, 0, 30)) {
		t.Fatalf("estimate %v", got)
	}
	if got := Estimate(now, 2_010_000, 2_010_000, 0, e, l); !got.Equal(e) {
		t.Fatalf("met early → clamped to earliest: %v", got)
	}
	if got := Estimate(now, 0, 2_010_000, 1, e, l); !got.Equal(l) {
		t.Fatalf("slow → clamped to latest: %v", got)
	}
}
