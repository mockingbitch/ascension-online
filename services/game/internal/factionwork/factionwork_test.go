package factionwork

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestShippedFactionWorksAreValid(t *testing.T) {
	c, err := LoadFile("../../../../content/catalog/faction_works.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.All()) == 0 || c.Settings.DailyCapPerCharacter < c.Settings.MinAmount {
		t.Fatalf("%+v", c.Settings)
	}
	var start int64
	for _, w := range c.All() {
		if w.Start != start || w.Goal != w.Stages[len(w.Stages)-1].At {
			t.Fatalf("work %s start %d goal %d", w.ID, w.Start, w.Goal)
		}
		start += w.Goal
	}
}

const y = `version: 1
settings: { currency: spirit_stone, daily_cap_per_character: 500, min_amount: 10, donor_board_size: 5 }
works:
  - { id: a, name: A, stages: [{ at: 100, decoration: { slug: a1, name: A1 } }, { at: 300, decoration: { slug: a2, name: A2 } }] }
  - { id: b, name: B, stages: [{ at: 1000, decoration: { slug: b1, name: B1 } }] }
`

func cat(t *testing.T) *Catalogue {
	c, err := Load(strings.NewReader(y))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestProgressOf(t *testing.T) {
	c := cat(t)
	cases := []struct {
		total   int64
		work    int
		inWork  int64
		unlocks []string
	}{
		{0, 0, 0, []string{}},
		{99, 0, 99, []string{}},
		{100, 0, 100, []string{"a1"}},
		{300, 1, 0, []string{"a1", "a2"}},
		{1299, 1, 999, []string{"a1", "a2"}},
		{1300, 2, 0, []string{"a1", "a2", "b1"}},
		{9999, 2, 0, []string{"a1", "a2", "b1"}},
	}
	for _, tc := range cases {
		p := c.ProgressOf(tc.total)
		if p.WorkIndex != tc.work || p.WorkDonated != tc.inWork || !slices.Equal(p.Unlocked, tc.unlocks) {
			t.Errorf("total %d: %+v", tc.total, p)
		}
	}
}

func TestCheck(t *testing.T) {
	c := cat(t)
	cases := []struct {
		amount, today, balance int64
		want                   error
	}{
		{10, 0, 10, nil},
		{9, 0, 100, ErrBadAmount},
		{501, 0, 1000, ErrBadAmount},
		{0, 0, 100, ErrBadAmount},
		{-50, 0, 100, ErrBadAmount},
		{500, 0, 500, nil},
		{100, 450, 1000, ErrDailyCap},
		{50, 450, 1000, nil},
		{100, 0, 99, ErrInsufficient},
	}
	for _, tc := range cases {
		if err := c.Check(tc.amount, tc.today, tc.balance); !errors.Is(err, tc.want) {
			t.Errorf("%+v: got %v", tc, err)
		}
	}
}

func TestLoadRejects(t *testing.T) {
	for want, bad := range map[string]string{
		`field "rate_bonus_bp" is a gameplay effect`: strings.Replace(y, "{ id: b, name: B,", "{ id: b, name: B, rate_bonus_bp: 100,", 1),
		"at must increase":                           strings.Replace(y, "at: 300", "at: 100", 1),
		"duplicate decoration slug":                  strings.Replace(y, "slug: b1", "slug: a1", 1),
		"min_amount":                                 strings.Replace(y, "min_amount: 10", "min_amount: 600", 1),
	} {
		if _, err := Load(strings.NewReader(bad)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("want %q, got %v", want, err)
		}
	}
}
