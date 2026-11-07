package daolu

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func shipped(t *testing.T) *Config {
	t.Helper()
	c, err := LoadFile("../../../../content/catalog/dao_lu.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestShippedDaoLu(t *testing.T) {
	c := shipped(t)
	if c.SongTu.WindowSeconds != 7200 {
		t.Fatalf("decision: song tu window is 2h/day, got %d s", c.SongTu.WindowSeconds)
	}
	if c.Bond.AbsenceDays != 90 {
		t.Fatalf("decision: absence 90 days, got %d", c.Bond.AbsenceDays)
	}
}

func TestLoadRejects(t *testing.T) {
	base := `version: 1
bond: { min_account_age_days: 14, min_tier: 2, proposal_expiry_hours: 72, daily_proposals: 3, incoming_list_limit: 20, max_incoming_pending: 10, decline_cooldown_days: 7, absence_days: 90, rebond_cooldown_days: 7 }
song_tu: { window_seconds: 7200, rate_milli_per_hour: 5000, heartbeat_seconds: 30, max_gap_seconds: 75, duyen_min_seconds: 1800 }
`
	if _, err := Load(strings.NewReader(base)); err != nil {
		t.Fatalf("base: %v", err)
	}
	for name, bad := range map[string][2]string{
		"window":  {"window_seconds: 7200", "window_seconds: 99999"},
		"gap":     {"max_gap_seconds: 75", "max_gap_seconds: 30"},
		"absence": {"absence_days: 90", "absence_days: 5"},
		"typo":    {"daily_proposals: 3", "daily_proposal: 3"},
		"duyen":   {"duyen_min_seconds: 1800", "duyen_min_seconds: 9000"},
	} {
		if _, err := Load(strings.NewReader(strings.Replace(base, bad[0], bad[1], 1))); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestPayMilliSplitInvariant(t *testing.T) {
	c := shipped(t)
	if got := c.PayMilli(c.SongTu.WindowSeconds); got != c.MaxSongTuMilli() || got != 10_000 {
		t.Fatalf("full window = %d milli (want 10 points)", got)
	}
	if c.PayMilli(c.SongTu.WindowSeconds+5000) != c.MaxSongTuMilli() || c.PayMilli(-5) != 0 {
		t.Fatal("pay must clamp to the window and never go negative")
	}
	// Any split of claims pays exactly the single claim (cumulative formula).
	for _, splits := range [][]int64{{1, 1, 1}, {7, 13, 600, 1}, {3599, 3601}, {7200}} {
		var paid, total int64
		for _, s := range splits {
			total += c.PayMilli(paid+s) - c.PayMilli(paid)
			paid += s
		}
		if total != c.PayMilli(paid) {
			t.Fatalf("split %v pays %d, single %d", splits, total, c.PayMilli(paid))
		}
	}
}

func TestBlockers(t *testing.T) {
	c := shipped(t)
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	ok := Party{AccountCreatedAt: now.Add(-30 * day), Tier: 2, LastEndedAt: time.Unix(0, 0)}
	if b := c.Blockers(ok, now); len(b) != 0 {
		t.Fatalf("eligible party blocked: %v", b)
	}
	cases := []struct {
		p    Party
		want string
	}{
		{Party{AccountCreatedAt: now.Add(-13 * day), Tier: 2}, BlockAccountAge},
		{Party{AccountCreatedAt: now.Add(-30 * day), Tier: 1}, BlockTier},
		{Party{AccountCreatedAt: now.Add(-30 * day), Tier: 3, Bonded: true}, BlockBonded},
		{Party{AccountCreatedAt: now.Add(-30 * day), Tier: 3, LastEndedAt: now.Add(-6 * day)}, BlockCooldown},
	}
	for _, cs := range cases {
		if b := c.Blockers(cs.p, now); !slices.Contains(b, cs.want) || len(b) != 1 {
			t.Errorf("%+v → %v, want [%s]", cs.p, b, cs.want)
		}
	}
	// Cooldown over after rebond_cooldown_days.
	if b := c.Blockers(Party{AccountCreatedAt: now.Add(-30 * day), Tier: 3, LastEndedAt: now.Add(-7 * day)}, now); len(b) != 0 {
		t.Fatalf("cooldown should be over: %v", b)
	}
	if u := c.CooldownUntil(now.Add(-2*day), now); !u.Equal(now.Add(5 * day)) {
		t.Fatalf("cooldown until %v", u)
	}
	if !c.CooldownUntil(time.Unix(0, 0), now).IsZero() {
		t.Fatal("never bonded → no cooldown")
	}
}
