package world

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var repoRoot = filepath.Join("..", "..", "..", "..")

func shipped(t *testing.T) *Catalogue {
	t.Helper()
	c, err := LoadFile(filepath.Join(repoRoot, "content", "catalog", "world_milestones.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestShippedMilestones(t *testing.T) {
	c := shipped(t)
	if len(c.Eras) != 9 || len(c.Milestones) != 10 {
		t.Fatalf("eras %d milestones %d", len(c.Eras), len(c.Milestones))
	}
	// Decision B thresholds (target_day × 67), in order.
	want := map[string][2]int64{"khai_giang": {0, 0}, "giang_dao_2": {30, 2_010_000}, "giang_dao_3": {60, 4_020_000},
		"dai_chien": {100, 6_700_000}, "nu_oa": {130, 8_710_000}, "thanh_nhan": {140, 9_380_000}, "nhan_toc": {150, 10_050_000},
		"phong_than": {240, 16_080_000}, "thien_dinh": {310, 20_770_000}, "tay_du": {330, 22_110_000}}
	for i, m := range c.Milestones {
		w, ok := want[m.ID]
		if !ok || int64(m.TargetDay) != w[0] || m.ThresholdMilli != w[1] || m.Seq != i {
			t.Errorf("%s: day %d threshold %d seq %d", m.ID, m.TargetDay, m.ThresholdMilli, m.Seq)
		}
		if ref := int64(m.TargetDay) * c.Settings.ReferencePerActiveMilli; ref != m.ThresholdMilli {
			t.Logf("note: %s threshold %d differs from target_day × a_ref = %d", m.ID, m.ThresholdMilli, ref)
		}
	}
	// set_era for nhan_toc, thien_dinh, tay_du; luan_hoi via dai_chien's on_end.
	for _, id := range []string{"nhan_toc", "thien_dinh", "tay_du"} {
		m, _ := c.Milestone(id)
		if m.Effects[0].Kind != EffectSetEra || m.Effects[0].EraID != id {
			t.Errorf("%s: first effect %+v", id, m.Effects[0])
		}
	}
	m, ev, ok := c.Event("dai_chien")
	if !ok || m.ID != "dai_chien" || ev.DurationDays != 15 || len(ev.OnEnd) != 1 || ev.OnEnd[0].EraID != "luan_hoi" {
		t.Fatalf("dai_chien event %+v", ev)
	}
	th, _ := c.Milestone("thanh_nhan")
	split, migrate := th.Effects[2], th.Effects[3]
	if split.Kind != EffectSplitFaction || split.From != "huyen_khung_mon" || strings.Join(split.Into, ",") != "thu_nhat_vien,lap_cuc_vien,van_luu_vien" ||
		split.NoticeDays != 3 || split.ChooseDays != 7 {
		t.Fatalf("split %+v", split)
	}
	if migrate.Kind != EffectMigrateFaction || migrate.From != "kim_sa_mon" || migrate.To != "kim_sa_giao" || migrate.NoticeDays != 3 {
		t.Fatalf("migrate %+v", migrate)
	}
}

// Every announce key and world.event_end.<event> must exist in the story announcements file (§5.7).
func TestShippedAnnouncements(t *testing.T) {
	c := shipped(t)
	b, err := os.ReadFile(filepath.Join(repoRoot, "content", "story", "announcements", "world-milestones.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Templates map[string][]string `yaml:"templates"`
	}
	if err := yaml.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if err := c.CheckAnnouncements(func(k string) bool { return len(f.Templates[k]) > 0 }); err != nil {
		t.Fatal(err)
	}
	if err := c.CheckAnnouncements(func(k string) bool { return k != "world.event_end.dai_chien" }); err == nil ||
		!strings.Contains(err.Error(), "world.event_end.dai_chien") {
		t.Fatalf("missing key not reported: %v", err)
	}
}

const header = `version: 1
settings:
  progress_mode: per_active_capita
  world_credit_cap_points: 100
  exclude_catch_up_bonus: true
  reference_points_per_active_day: 67
  min_pct_bp: 7000
  max_pct_bp: 15000
  day_close_grace_seconds: 300
  median_window_days: 7
eras:
  - { id: a, name: A }
  - { id: b, name: B }
`

func TestLoaderRejects(t *testing.T) {
	ok := `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: world.m0, effects: [{ kind: set_era, era_id: a }] }
  - { id: m1, name: M1, target_day: 10, threshold_points_per_active: 670, announce: world.m1,
      effects: [{ kind: start_event, event_id: e1, duration_days: 2, on_end: [{ kind: set_era, era_id: b }] }] }
`
	if _, err := Load(strings.NewReader(header + ok)); err != nil {
		t.Fatalf("valid catalogue rejected: %v", err)
	}
	cases := map[string]string{
		"empty": `milestones: []`,
		"first day": `milestones:
  - { id: m0, name: M0, target_day: 5, threshold_points_per_active: 0, announce: k }`,
		"duplicate id": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: k }
  - { id: m0, name: M1, target_day: 1, threshold_points_per_active: 1, announce: k }`,
		"day order": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: k }
  - { id: m1, name: M1, target_day: 20, threshold_points_per_active: 10, announce: k }
  - { id: m2, name: M2, target_day: 10, threshold_points_per_active: 20, announce: k }`,
		"threshold order": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: k }
  - { id: m1, name: M1, target_day: 10, threshold_points_per_active: 20, announce: k }
  - { id: m2, name: M2, target_day: 20, threshold_points_per_active: 10, announce: k }`,
		"zero later threshold": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: k }
  - { id: m1, name: M1, target_day: 10, threshold_points_per_active: 0, announce: k }`,
		"missing announce": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0 }`,
		"unknown era": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: k, effects: [{ kind: set_era, era_id: zz }] }`,
		"era backwards": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: k, effects: [{ kind: set_era, era_id: b }] }
  - { id: m1, name: M1, target_day: 1, threshold_points_per_active: 1, announce: k, effects: [{ kind: set_era, era_id: a }] }`,
		"unknown kind": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: k, effects: [{ kind: nuke }] }`,
		"foreign field": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: k, effects: [{ kind: set_era, era_id: a, duration_days: 1 }] }`,
		"event no duration": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: k, effects: [{ kind: start_event, event_id: e }] }`,
		"duplicate event": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: k, effects: [{ kind: start_event, event_id: e, duration_days: 1 }] }
  - { id: m1, name: M1, target_day: 1, threshold_points_per_active: 1, announce: k, effects: [{ kind: start_event, event_id: e, duration_days: 1 }] }`,
		"on_end not set_era": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: k,
      effects: [{ kind: start_event, event_id: e, duration_days: 1, on_end: [{ kind: start_event, event_id: f, duration_days: 1 }] }] }`,
		"split self": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: k,
      effects: [{ kind: split_faction, from: x, into: [x, y], notice_days: 3, choose_days: 7 }] }`,
		"split no choose": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: k,
      effects: [{ kind: split_faction, from: x, into: [y, z], notice_days: 3 }] }`,
		"migrate same": `milestones:
  - { id: m0, name: M0, target_day: 0, threshold_points_per_active: 0, announce: k,
      effects: [{ kind: migrate_faction, from: x, to: x, notice_days: 3 }] }`,
		"typo field": `milestones:
  - { id: m0, name: M0, target_dai: 0, threshold_points_per_active: 0, announce: k }`,
	}
	for name, body := range cases {
		if _, err := Load(strings.NewReader(header + body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// Milestones are append-only after launch: world_milestones_reached.seq (UNIQUE) = catalogue index, so a shipped
// milestone must keep its position. Extend this list when appending; never reorder or remove (review L4).
func TestShippedMilestoneSeqIsStable(t *testing.T) {
	c, err := LoadFile("../../../../content/catalog/world_milestones.yaml")
	if err != nil {
		t.Fatal(err)
	}
	shipped := []string{"khai_giang", "giang_dao_2", "giang_dao_3", "dai_chien", "nu_oa", "thanh_nhan", "nhan_toc",
		"phong_than", "thien_dinh", "tay_du"}
	if len(c.Milestones) < len(shipped) {
		t.Fatalf("%d milestones, %d shipped: milestones may not be removed", len(c.Milestones), len(shipped))
	}
	for i, id := range shipped {
		m, ok := c.Milestone(id)
		if !ok || m.Seq != i {
			t.Fatalf("milestone %q must stay at seq %d", id, i)
		}
	}
}

func TestSetEraEffects(t *testing.T) {
	c := shipped(t)
	split := Effect{Kind: EffectSplitFaction, From: "a", Into: []string{"b", "c"}, NoticeDays: 3, ChooseDays: 7}
	for name, in := range map[string]map[string][]Effect{
		"unknown era":  {"khong_co": {split}},
		"not a split":  {"luan_hoi": {{Kind: EffectSetEra, EraID: "tay_du"}}},
		"one branch":   {"luan_hoi": {{Kind: EffectSplitFaction, From: "a", Into: []string{"b"}}}},
		"days too big": {"luan_hoi": {{Kind: EffectSplitFaction, From: "a", Into: []string{"b", "c"}, NoticeDays: 31}}},
	} {
		if err := c.SetEraEffects(in); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	second := split
	second.From = "d"
	if err := c.SetEraEffects(map[string][]Effect{"thanh_nhan": {second}, "luan_hoi": {split}}); err != nil {
		t.Fatal(err)
	}
	luan, _ := c.EraSeq("luan_hoi")
	than, _ := c.EraSeq("thanh_nhan")
	// Indexes follow era order (stable world_effects.effect_index), not map order.
	if l, th := c.EraEffects(luan), c.EraEffects(than); len(l) != 1 || l[0].Index != 0 || len(th) != 1 || th[0].Index != 1 || th[0].Effect.From != "d" {
		t.Fatalf("era effects %+v %+v", l, th)
	}
	if c.EraEffects(-1) != nil || c.EraEffects(len(c.Eras)) != nil {
		t.Fatal("out of range seq")
	}
}
