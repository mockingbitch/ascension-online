package realm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var shippedPath = filepath.Join("..", "..", "..", "..", "content", "catalog", "realms.yaml")

func loadShipped(t *testing.T) *Catalogue {
	t.Helper()
	c, err := LoadFile(shippedPath)
	if err != nil {
		t.Fatalf("shipped realms.yaml invalid: %v", err)
	}
	return c
}

// expected per-stage requirements (points) per tier, balance doc §11.1 / data-model §3.2.
type reqRun struct{ points, count int64 }

var shippedTable = []struct {
	tier   int
	realm  string
	entry  int64
	stages int
	runs   []reqRun
}{
	// Luyện Khí: stage_weights 1…9 over 100 points (early game front-loaded, decisions 2026-11-07).
	{1, "luyen_khi", 0, 9, []reqRun{{2, 1}, {4, 1}, {6, 1}, {8, 1}, {11, 1}, {14, 1}, {16, 1}, {18, 1}, {21, 1}}},
	{2, "truc_co", 100, 4, []reqRun{{137, 2}, {138, 2}}},
	{3, "kim_dan", 650, 4, []reqRun{{350, 4}}},
	{4, "nguyen_anh", 2050, 8, []reqRun{{181, 6}, {182, 2}}},
	{5, "hoa_than", 3500, 8, []reqRun{{237, 4}, {238, 4}}},
	{6, "luyen_hu", 5400, 8, []reqRun{{262, 4}, {263, 4}}},
	{7, "hop_the", 7500, 8, []reqRun{{281, 6}, {282, 2}}},
	{8, "dai_thua", 9750, 12, []reqRun{{208, 8}, {209, 4}}},
	{9, "do_kiep", 12250, 12, []reqRun{{220, 2}, {221, 10}}},
	{10, "nhan_tien", 14900, 12, []reqRun{{237, 6}, {238, 6}}},
	{11, "dia_tien", 17750, 12, []reqRun{{262, 6}, {263, 6}}},
	{12, "kim_tien", 20900, 12, []reqRun{{287, 6}, {288, 6}}},
	{13, "thai_at_kim_tien", 24350, 16, []reqRun{{240, 6}, {241, 10}}},
	{14, "dai_la_kim_tien", 28200, 16, []reqRun{{265, 6}, {266, 10}}},
	{15, "chuan_thanh", 32450, 64, []reqRun{{300, 64}}},
}

// The shipped content must always validate — this test is the content CI gate.
func TestShippedCatalogueIsValid(t *testing.T) {
	c := loadShipped(t)
	if c.DefaultPath() != "tien" || len(c.Paths()) != 2 || c.LastPlayableTier() != 15 {
		t.Fatalf("paths/default/last tier: %+v %s %d", c.Paths(), c.DefaultPath(), c.LastPlayableTier())
	}
	if got := c.First("tien").ID; got != "luyen_khi.tang_1" {
		t.Fatalf("first stage = %s", got)
	}
	if got := c.First("tien").Name; got != "Luyện Khí Tầng 1" {
		t.Fatalf("prefixed name = %q", got)
	}
	for _, p := range c.Paths() {
		ladder := c.Ladder(p.ID)
		if len(ladder) != 205 {
			t.Fatalf("path %s: %d stages, want 205", p.ID, len(ladder))
		}
		top := ladder[len(ladder)-1]
		if c.Position(top, top.RequiredMilli) != 51_650_000 {
			t.Fatalf("path %s: ladder top %d", p.ID, c.Position(top, top.RequiredMilli))
		}
		if c.Next(top) != nil || top.NextKind != "" {
			t.Fatalf("top stage must have no next")
		}
	}
	for _, row := range shippedTable {
		first, ok := c.StageAt("tien", row.tier, 1)
		if !ok || first.RealmID != row.realm {
			t.Fatalf("tier %d: realm %v", row.tier, first)
		}
		if first.CumulativeBeforeMilli != row.entry*PointMilli || first.StageCount != row.stages {
			t.Errorf("tier %d: entry %d count %d", row.tier, first.CumulativeBeforeMilli, first.StageCount)
		}
		var want []int64
		for _, r := range row.runs {
			for range r.count {
				want = append(want, r.points*PointMilli)
			}
		}
		for i := 1; i <= row.stages; i++ {
			s, _ := c.StageAt("tien", row.tier, i)
			if s.RequiredMilli != want[i-1] {
				t.Errorf("%s: required %d want %d", s.ID, s.RequiredMilli, want[i-1])
			}
			last := i == row.stages
			if last != (s.CapMilli > 0) {
				t.Errorf("%s: cap %d only on last stage", s.ID, s.CapMilli)
			}
			if last && s.CapMilli != s.RequiredMilli*12/10 {
				t.Errorf("%s: cap %d want 120%%", s.ID, s.CapMilli)
			}
			if !last && s.NextKind != Auto {
				t.Errorf("%s: non-last stage must be auto", s.ID)
			}
			if last && row.tier < 15 && (s.NextKind != Major || s.Tribulation != (row.tier >= 5)) {
				t.Errorf("%s: next %q tribulation %v", s.ID, s.NextKind, s.Tribulation)
			}
		}
	}
	s, _ := c.Stage("luyen_khi.tang_9")
	if s.NextKind != Major || len(s.Breakthrough.Requirements) == 0 || s.Breakthrough.SuccessBP != 7000 {
		t.Fatalf("luyen_khi.tang_9 should be a major breakthrough with requirements: %+v", s)
	}
	if _, err := c.Stage("chuan_thanh.vien_man_16"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Stage("thanh_nhan.dac_dao"); err == nil {
		t.Fatal("reserved Thánh Nhân stage must not be holdable")
	}
	if s, _ := c.Stage("nguyen_anh.trung_ky_2"); s.Name != "Trung Kỳ Nhị Trọng" {
		t.Fatalf("generated name %q", s.Name)
	}
	if s, _ := c.Stage("truc_co.so_ky"); s.Name != "Sơ Kỳ" {
		t.Fatalf("layers=1 name %q", s.Name)
	}
}

// Release gate: no placeholder names may ship (CONTENT_RELEASE=1).
func TestShippedReleaseGate(t *testing.T) {
	if os.Getenv("CONTENT_RELEASE") != "1" {
		t.Skip("set CONTENT_RELEASE=1 to enforce the placeholder gate")
	}
	c := loadShipped(t)
	for _, r := range c.Realms() {
		if r.NamePlaceholder {
			t.Errorf("realm %s has a placeholder name", r.ID)
		}
	}
	for _, p := range c.Paths() {
		if p.NamePlaceholder {
			t.Errorf("path %s has a placeholder name", p.ID)
		}
	}
}

func TestConvertRoundTrip(t *testing.T) {
	c := loadShipped(t)
	for _, s := range c.Ladder("tien") {
		v, err := c.Convert(s, "vu")
		if err != nil {
			t.Fatal(err)
		}
		back, err := c.Convert(v, "tien")
		if err != nil || back != s {
			t.Fatalf("round trip %s → %s → %v", s.ID, v.ID, back)
		}
		if v.RequiredMilli != s.RequiredMilli || v.CapMilli != s.CapMilli || v.CumulativeBeforeMilli != s.CumulativeBeforeMilli ||
			v.Tribulation != s.Tribulation || v.NextKind != s.NextKind || v.Index != s.Index {
			t.Fatalf("%s vs %s differ", s.ID, v.ID)
		}
	}
}

func TestBottleneckAndAdvance(t *testing.T) {
	c := loadShipped(t)
	first, _ := c.Stage("luyen_khi.tang_1")
	// Whole Luyện Khí: 100 points + 20% of the last stage (21 → 4.2).
	if got := c.BottleneckRoomMilli(first, 0); got != 100_000+4_200 {
		t.Fatalf("room from tang_1 = %d", got)
	}
	last, _ := c.Stage("luyen_khi.tang_9")
	if got := c.BottleneckRoomMilli(last, last.CapMilli+5); got != 0 {
		t.Fatalf("room beyond cap = %d", got)
	}
	s, stored, passed := c.Advance(first, 13_000) // 2+4+6 = 12 → tang_4 with 1 point
	if s.ID != "luyen_khi.tang_4" || stored != 1_000 || len(passed) != 3 || passed[0] != first {
		t.Fatalf("advance → %s %d %d", s.ID, stored, len(passed))
	}
	s, stored, _ = c.Advance(first, 10_000_000) // never leaves the realm
	if s != last || stored != 10_000_000-(last.CumulativeBeforeMilli-first.CumulativeBeforeMilli) {
		t.Fatalf("advance must stop at the realm's last stage: %s %d", s.ID, stored)
	}
	if c.Position(s, 0) != last.CumulativeBeforeMilli {
		t.Fatal("position")
	}
	if c.FirstOfRealm(last) != first || c.LastOfRealm(first) != last {
		t.Fatal("first/last of realm")
	}
}

func TestTribulationBase(t *testing.T) {
	c := loadShipped(t)
	want := map[string]int64{"hoa_than": 8000, "luyen_hu": 7700, "do_kiep": 6800, "dai_la_kim_tien": 5300}
	for realmID, bp := range want {
		var last *Stage
		for _, s := range c.Ladder("tien") {
			if s.RealmID == realmID && s.IsLastOfRealm() {
				last = s
			}
		}
		if got := c.TribulationBaseBP(last); got != bp {
			t.Errorf("%s: base %d want %d", realmID, got, bp)
		}
	}
	nl, _ := c.Stage("nguyen_anh.vien_man_2")
	if nl.Tribulation || c.TribulationBaseBP(nl) != 0 {
		t.Fatal("leaving Nguyên Anh is not a tribulation")
	}
}

func TestFloorApplies(t *testing.T) {
	c := &Catalogue{trib: Tribulation{FromTier: 5, BaseBP: 8000, StepBP: 300, FloorBP: 5000}}
	if got := c.TribulationBaseBP(&Stage{Tier: 25, Tribulation: true}); got != 5000 {
		t.Fatalf("floor: %d", got)
	}
}

func mutate(t *testing.T, from, to string) error {
	t.Helper()
	b, err := os.ReadFile(shippedPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, from) {
		t.Fatalf("fixture text %q not found", from)
	}
	_, err = Load(strings.NewReader(strings.Replace(s, from, to, 1)))
	return err
}

func TestRejectsBadCatalogue(t *testing.T) {
	cases := []struct{ from, to, want string }{
		{"version: 2", "version: 1", "unsupported version"},
		{"{ id: vu,   name: Vu Đạo, name_placeholder: true }", "{ id: vu,   name: Vu Đạo, default: true }", "exactly one path"},
		{"minor_breakthrough: { auto: true }", "minor_breakthrough: { auto: false }", "auto must be true"},
		{"entry_points: 5400", "entry_points: 3400", "strictly increase"},
		{"- { id: vu_15, path: vu, tier: 15,", "- { id: vu_15, path: vu, tier: 14,", "already has a realm"},
		{"max_success_bp: 9000", "max_success_bp: 9100", "max_success_bp"},
		{"loss_bp: 2500", "loss_bp: 2000", "loss_bp"},
		{"quote_window_seconds: 600", "quote_window_seconds: 7200", "quote_window_seconds"},
		{"loss_bp: 2500", "loss_bp: 6000", "loss_bp"},
		{"max_bp: 500", "max_bp: 1500", "max_total_bonus_bp"},
		{"names: { tien: Lôi Kiếp, vu: Huyết Kiếp }", "names: { tien: Lôi Kiếp }", "missing path"},
		{"stages: { phases: bon_ky, layers: 16 }", "stages: { phases: bon_ky, layers: 17 }", "layers"},
		{"{ tier: 6,  entry_points: 5400,  stages: { phases: bon_ky, layers: 2 } }",
			"{ tier: 6,  entry_points: 5400,  stages: { phases: bon_ky, layers: 2 }, major_breakthrough: { success_bp: 5000 } }", "must not set success_bp"},
		{"overflow_cap_bp: 12000", "overflow_cap_bp: 9000", "overflow_cap_bp"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			err := mutate(t, tc.from, tc.to)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestExtraStagesAppend(t *testing.T) {
	b, _ := os.ReadFile(shippedPath)
	s := strings.Replace(string(b), "extra_stages: []", "extra_stages: [{ id: vien_man_17, name: Viên Mãn Thập Thất Trọng, required_points: 300 }]", 1)
	c, err := Load(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	last := c.Ladder("tien")[len(c.Ladder("tien"))-1]
	prev, _ := c.Stage("chuan_thanh.vien_man_16")
	if last.ID != "chuan_thanh.vien_man_17" || prev.CapMilli != 0 || last.CapMilli != 360_000 || prev.NextKind != Auto {
		t.Fatalf("extra stage: %+v prev %+v", last, prev)
	}
	if fmt.Sprint(len(c.Ladder("vu"))) != "206" {
		t.Fatal("extra stages apply to every path")
	}
}

func TestUnknownFieldRejected(t *testing.T) {
	if err := mutate(t, "version: 2", "version: 2\ntypo: 1"); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestSplitSpan(t *testing.T) {
	cases := []struct {
		span    int64
		weights []int64
		want    []int64
	}{
		{300, []int64{1, 1, 1, 1, 1, 1, 1, 1, 1}, []int64{33, 33, 33, 33, 33, 33, 34, 34, 34}}, // even split, old behaviour
		{100, []int64{1, 2, 3, 4, 5, 6, 7, 8, 9}, []int64{2, 4, 6, 8, 11, 14, 16, 18, 21}},
		{10, []int64{1, 1, 8}, []int64{1, 1, 8}},
		{3, []int64{1, 1, 1000}, []int64{0, 0, 3}}, // caller rejects stages below 1 point
	}
	for _, tc := range cases {
		got := splitSpan(tc.span, tc.weights)
		var sum int64
		for i := range got {
			sum += got[i]
			if got[i] != tc.want[i] {
				t.Errorf("splitSpan(%d, %v) = %v want %v", tc.span, tc.weights, got, tc.want)
				break
			}
		}
		if sum != tc.span {
			t.Errorf("splitSpan(%d, %v) sums to %d", tc.span, tc.weights, sum)
		}
	}
}

func TestStageWeightsValidation(t *testing.T) {
	b, err := os.ReadFile(shippedPath)
	if err != nil {
		t.Fatal(err)
	}
	for name, w := range map[string]string{
		"wrong length":    "[1, 2, 3]",
		"zero weight":     "[0, 1, 1, 1, 1, 1, 1, 1, 1]",
		"decreasing":      "[2, 1, 1, 1, 1, 1, 1, 1, 1]",
		"stage < 1 point": "[1, 1, 1, 1, 1, 1, 1, 1, 1000]",
	} {
		s := strings.Replace(string(b), "stage_weights: [1, 2, 3, 4, 5, 6, 7, 8, 9]", "stage_weights: "+w, 1)
		if s == string(b) {
			t.Fatal("shipped realms.yaml no longer has the Luyện Khí stage_weights line")
		}
		if _, err := Load(strings.NewReader(s)); err == nil {
			t.Errorf("%s: want a validation error", name)
		}
	}
}
