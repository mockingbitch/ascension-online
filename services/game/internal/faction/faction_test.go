package faction

import (
	"errors"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"tiendao/game/internal/db"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/world"
)

const catalogDir = "../../../../content/catalog/"

func shipped(t *testing.T) (*Catalogue, *world.Catalogue, *realm.Catalogue) {
	t.Helper()
	c, err := LoadFile(catalogDir + "factions.yaml")
	if err != nil {
		t.Fatal(err)
	}
	w, err := world.LoadFile(catalogDir + "world_milestones.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r, err := realm.LoadFile(catalogDir + "realms.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return c, w, r
}

func TestShippedFactionsAreValid(t *testing.T) {
	c, w, r := shipped(t)
	if err := c.CrossCheck(r, w); err != nil {
		t.Fatal(err)
	}
	if c.Settings.SwitchCooldown != 7*24*time.Hour {
		t.Fatalf("switch cooldown = %s, want 7 days (checklist 2.2)", c.Settings.SwitchCooldown)
	}
	open := 0
	for i := range c.Factions {
		if c.Factions[i].OpenIn(w, 0) {
			open++
		}
	}
	if open != 5 {
		t.Fatalf("%d factions open at launch, want the 5 starting factions", open)
	}
	// Decision 2026-11-07: Nhân Tộc opens with Nữ Oa (era tao_nhan).
	if f, _ := c.Get("nhan_toc"); f.OpenFromEra != "tao_nhan" {
		t.Fatalf("nhan_toc open_from_era = %q, want tao_nhan", f.OpenFromEra)
	}
	// Release gate (decision 12): placeholder names are allowed in dev, listed here for the release checklist.
	if p := c.Placeholders(); len(p) > 0 {
		t.Logf("release gate: placeholder faction names %v", p)
	}
}

func TestOpenIn(t *testing.T) {
	c, w, _ := shipped(t)
	seq := func(id string) int { s, _ := w.EraSeq(id); return s }
	cases := []struct {
		faction, era string
		open         bool
	}{
		{"huyen_khung_mon", "khai_giang", true},
		{"huyen_khung_mon", "tao_nhan", true},
		{"huyen_khung_mon", "thanh_nhan", false},
		{"thu_nhat_vien", "tao_nhan", false},
		{"thu_nhat_vien", "thanh_nhan", true},
		{"kim_sa_giao", "phong_than", true},
		{"thuong_lan_dinh", "dai_chien", true},
		{"thuong_lan_dinh", "luan_hoi", false},
		{"vu_toc", "tay_du", true},
		{"nhan_toc", "luan_hoi", false},
		{"nhan_toc", "tao_nhan", true},
		{"nhan_toc", "tay_du", true},
	}
	for _, tc := range cases {
		f, _ := c.Get(tc.faction)
		if got := f.OpenIn(w, seq(tc.era)); got != tc.open {
			t.Errorf("%s in %s: open = %v, want %v", tc.faction, tc.era, got, tc.open)
		}
	}
}

func TestCanJoin(t *testing.T) {
	c, w, _ := shipped(t)
	now := time.Date(2027, 1, 10, 0, 0, 0, 0, time.UTC)
	recent := now.Add(-time.Hour)
	old := now.Add(-8 * 24 * time.Hour)
	later := now.Add(time.Hour)
	thanh, _ := w.EraSeq("thanh_nhan")
	cases := []struct {
		name     string
		r        Request
		wantPath realm.PathID
		err      error
	}{
		{"first join keeps path", Request{Kind: KindJoin, Target: "kim_sa_mon", CurPath: "tien"}, "tien", nil},
		{"join vu converts", Request{Kind: KindJoin, Target: "vu_toc", CurPath: "tien"}, "vu", nil},
		{"wrong path", Request{Kind: KindJoin, Target: "kim_sa_mon", CurPath: "tien", WantPath: "vu"}, "", ErrPathNotAllowed},
		{"unknown", Request{Kind: KindJoin, Target: "nope", CurPath: "tien"}, "", ErrUnknownFaction},
		{"closed (never opened)", Request{Kind: KindJoin, Target: "nhan_toc", CurPath: "tien"}, "", ErrFactionClosed},
		{"closed (split era)", Request{Kind: KindJoin, Target: "huyen_khung_mon", CurPath: "tien", EraSeq: thanh}, "", ErrFactionClosed},
		{"branch opens later", Request{Kind: KindJoin, Target: "lap_cuc_vien", CurPath: "tien"}, "", ErrFactionClosed},
		{"already member", Request{Kind: KindJoin, Member: &Member{FactionID: "vu_toc"}, Target: "kim_sa_mon"}, "", ErrAlreadyMember},
		{"switch no member", Request{Kind: KindSwitch, Target: "kim_sa_mon"}, "", ErrNotMember},
		{"switch same", Request{Kind: KindSwitch, Member: &Member{FactionID: "kim_sa_mon"}, Target: "kim_sa_mon"}, "", ErrSameFaction},
		{"switch after first join (free)", Request{Kind: KindSwitch, Member: &Member{FactionID: "kim_sa_mon"}, Target: "vu_toc", CurPath: "tien"}, "vu", nil},
		{"switch cooldown", Request{Kind: KindSwitch, Member: &Member{FactionID: "kim_sa_mon", LastSwitchAt: &recent}, Target: "thuong_lan_dinh", CurPath: "tien"}, "", ErrCooldown},
		{"switch after cooldown", Request{Kind: KindSwitch, Member: &Member{FactionID: "kim_sa_mon", LastSwitchAt: &old}, Target: "thuong_lan_dinh", CurPath: "tien"}, "tien", nil},
		{"locked permanent", Request{Kind: KindSwitch, Member: &Member{FactionID: "kim_sa_mon", LockReason: "fengshen"}, Target: "vu_toc"}, "", ErrLocked},
		{"locked timed", Request{Kind: KindSwitch, Member: &Member{FactionID: "kim_sa_mon", LockReason: "admin", LockedUntil: &later}, Target: "vu_toc", CurPath: "tien"}, "", ErrLocked},
		{"lock expired", Request{Kind: KindSwitch, Member: &Member{FactionID: "kim_sa_mon", LockReason: "admin", LockedUntil: &recent}, Target: "vu_toc", CurPath: "tien"}, "vu", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.r.Now = now
			got, err := c.CanJoin(w, tc.r)
			if !errors.Is(err, tc.err) || got != tc.wantPath {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, err, tc.wantPath, tc.err)
			}
		})
	}
}

func TestDefaultBranch(t *testing.T) {
	into := []string{"a", "b", "c"}
	cases := []struct {
		members map[string]int64
		want    string
	}{
		{map[string]int64{}, "a"},                         // all empty → first in order
		{map[string]int64{"a": 3, "b": 1, "c": 2}, "b"},   // fewest
		{map[string]int64{"a": 2, "b": 1, "c": 1}, "b"},   // tie → earlier in order
		{map[string]int64{"a": 5, "b": 5}, "c"},           // missing = 0
		{map[string]int64{"a": 0, "b": 0, "c": 0}, "a"},   // all tied
		{map[string]int64{"a": 10, "b": 9, "c": 10}, "b"}, // middle
	}
	for _, tc := range cases {
		if got := DefaultBranch(into, tc.members); got != tc.want {
			t.Errorf("DefaultBranch(%v) = %q, want %q", tc.members, got, tc.want)
		}
	}
}

func TestLoadRejects(t *testing.T) {
	base := "version: 1\nsettings: { switch_cooldown_seconds: 604800, first_join_free: true }\nfactions:\n"
	cases := map[string]string{
		"bad id":        "  - { id: Bad, name: X, allowed_paths: [tien], open_from_era: a, color: aura-jade }\n",
		"no paths":      "  - { id: a, name: X, allowed_paths: [], open_from_era: a, color: aura-jade }\n",
		"no on_close":   "  - { id: a, name: X, allowed_paths: [tien], open_from_era: a, close_from_era: b, color: aura-jade }\n",
		"bad color":     "  - { id: a, name: X, allowed_paths: [tien], open_from_era: a, color: '#fff' }\n",
		"duplicate":     "  - { id: a, name: X, allowed_paths: [tien], color: c }\n  - { id: a, name: Y, allowed_paths: [tien], color: c }\n",
		"unknown field": "  - { id: a, name: X, allowed_paths: [tien], color: c, bonus_bp: 100 }\n",
		"close no open": "  - { id: a, name: X, allowed_paths: [tien], close_from_era: b, on_close: effect, color: c }\n",
		"split no days": "  - { id: a, name: X, allowed_paths: [tien], open_from_era: a, close_from_era: b, on_close: split, color: c }\n",
		"split one day": "  - { id: a, name: X, allowed_paths: [tien], open_from_era: a, close_from_era: b, on_close: split, split: { notice_days: 3 }, color: c }\n",
		"split range":   "  - { id: a, name: X, allowed_paths: [tien], open_from_era: a, close_from_era: b, on_close: split, split: { notice_days: 3, choose_days: 31 }, color: c }\n",
		"split w/o":     "  - { id: a, name: X, allowed_paths: [tien], open_from_era: a, close_from_era: b, on_close: effect, split: { notice_days: 3, choose_days: 7 }, color: c }\n",
	}
	for name, body := range cases {
		if _, err := Load(strings.NewReader(base + body)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := Load(strings.NewReader("version: 1\nsettings: { switch_cooldown_seconds: -1, first_join_free: true }\nfactions:\n  - { id: a, name: X, allowed_paths: [tien], color: c }\n")); err == nil {
		t.Error("negative cooldown accepted")
	}
}

func TestCrossCheckRejects(t *testing.T) {
	_, w, r := shipped(t)
	load := func(y string) *Catalogue {
		c, err := Load(strings.NewReader(y))
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	// Shipped roster minus the split branches: the thanh_nhan split points at unknown factions.
	b, _ := c0(t)
	cut := b[:strings.Index(b, "  - id: thu_nhat_vien")]
	if err := load(cut).CrossCheck(r, w); err == nil || !strings.Contains(err.Error(), `unknown faction "thu_nhat_vien"`) {
		t.Fatalf("missing branches: %v", err)
	}
	// No Vu faction at launch.
	noVu := strings.Replace(b, "allowed_paths: [vu]", "allowed_paths: [tien]", 1)
	if err := load(noVu).CrossCheck(r, w); err == nil || !strings.Contains(err.Error(), `path "vu" has no faction open`) {
		t.Fatalf("no vu faction: %v", err)
	}
	// Unknown era.
	badEra := strings.Replace(b, "    open_from_era: khai_giang", "    open_from_era: khong_co", 1)
	if err := load(badEra).CrossCheck(r, w); err == nil || !strings.Contains(err.Error(), "unknown open_from_era") {
		t.Fatalf("bad era: %v", err)
	}
	// A closing faction with on_close: effect that no effect moves.
	orphan := strings.Replace(b, "\n    on_close: split ", "\n    on_close: effect ", 1)
	orphan = splitLine.ReplaceAllString(orphan, "")
	if err := load(orphan).CrossCheck(r, w); err == nil || !strings.Contains(err.Error(), "no split_faction/migrate_faction moves it") {
		t.Fatalf("orphan close: %v", err)
	}
	// on_close: split with a single possible branch (thuong_lan_dinh on path vu: only vu_toc) is a migrate, refused.
	tl := b[strings.Index(b, "  - id: thuong_lan_dinh"):]
	tl = tl[:strings.Index(tl, "  - id: tinh_ha_yeu_dinh")]
	oneBranch := strings.Replace(b, tl, strings.Replace(tl, "allowed_paths: [tien]", "allowed_paths: [vu]", 1), 1)
	if err := load(oneBranch).CrossCheck(r, w); err == nil || !strings.Contains(err.Error(), "needs at least two factions") {
		t.Fatalf("one branch: %v", err)
	}
	// Every branch closes later (tinh_ha_yeu_dinh closing at thanh_nhan too): refused, defaults could strand members.
	th := "    allowed_paths: [tien]             # ⚠ Yêu path (G3) not decided; tiên for now\n    open_from_era: khai_giang\n"
	if !strings.Contains(b, th) {
		t.Fatal("fixture: tinh_ha_yeu_dinh block changed")
	}
	allClose := strings.Replace(b, th, th+"    close_from_era: thanh_nhan\n    on_close: keep_members\n", 1)
	if err := load(allClose).CrossCheck(r, w); err == nil || !strings.Contains(err.Error(), "needs a target that never closes") {
		t.Fatalf("no lasting branch: %v", err)
	}
}

var splitLine = regexp.MustCompile(`(?m)^    split: .*\n`)

// Decision 2026-11-07: the thuong_lan_dinh branches are computed from the catalogue (open in luan_hoi, allow tiên,
// not closing there), never listed — catalogue order = default tie-break order.
func TestCloseSplitTargets(t *testing.T) {
	c, w, r := shipped(t)
	tl, _ := c.Get("thuong_lan_dinh")
	want := []string{"huyen_khung_mon", "kim_sa_mon", "tinh_ha_yeu_dinh"} // not vu_toc (path), nhan_toc (opens at tao_nhan), itself
	if got := c.CloseSplitTargets(w, tl); !slices.Equal(got, want) {
		t.Fatalf("targets %v, want %v", got, want)
	}
	splits := c.EraSplits(w)
	if len(splits) != 1 || len(splits["luan_hoi"]) != 1 {
		t.Fatalf("era splits %+v", splits)
	}
	e := splits["luan_hoi"][0]
	if e.Kind != world.EffectSplitFaction || e.From != "thuong_lan_dinh" || !slices.Equal(e.Into, want) || e.NoticeDays != 3 || e.ChooseDays != 7 {
		t.Fatalf("split effect %+v", e)
	}
	// Computed, not hard-coded: a faction opening in luan_hoi joins the branches (YAML order kept).
	b, _ := c0(t)
	c2, err := Load(strings.NewReader(strings.Replace(b, "    open_from_era: tao_nhan", "    open_from_era: luan_hoi", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if err := c2.CrossCheck(r, w); err != nil {
		t.Fatal(err)
	}
	tl2, _ := c2.Get("thuong_lan_dinh")
	if got := c2.CloseSplitTargets(w, tl2); !slices.Equal(got, append(slices.Clone(want), "nhan_toc")) {
		t.Fatalf("targets with nhan_toc open at luan_hoi: %v", got)
	}
	// The world catalogue accepts them as era-start effects.
	if err := w.SetEraEffects(splits); err != nil {
		t.Fatal(err)
	}
	seq, _ := w.EraSeq("luan_hoi")
	if ee := w.EraEffects(seq); len(ee) != 1 || ee[0].Effect.From != "thuong_lan_dinh" {
		t.Fatalf("era effects %+v", ee)
	}
}

func TestOpenBranches(t *testing.T) {
	c, w, _ := shipped(t)
	seq := func(id string) int { s, _ := w.EraSeq(id); return s }
	into := []string{"huyen_khung_mon", "kim_sa_mon", "tinh_ha_yeu_dinh"}
	if got := c.OpenBranches(w, into, seq("luan_hoi")); !slices.Equal(got, into) {
		t.Fatalf("luan_hoi: %v", got)
	}
	if got := c.OpenBranches(w, into, seq("thanh_nhan")); !slices.Equal(got, []string{"tinh_ha_yeu_dinh"}) {
		t.Fatalf("thanh_nhan: %v", got)
	}
	// Defensive fallback (the loader prevents it): nothing open → all branches, never an empty list.
	if got := c.OpenBranches(w, []string{"huyen_khung_mon"}, seq("tay_du")); !slices.Equal(got, []string{"huyen_khung_mon"}) {
		t.Fatalf("fallback: %v", got)
	}
	// The thanh_nhan branches never close: unaffected.
	viens := []string{"thu_nhat_vien", "lap_cuc_vien", "van_luu_vien"}
	if got := c.OpenBranches(w, viens, seq("tay_du")); !slices.Equal(got, viens) {
		t.Fatalf("viện: %v", got)
	}
}

func c0(t *testing.T) (string, error) {
	t.Helper()
	b, err := readFile(catalogDir + "factions.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return b, nil
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

func TestAssignDefaults(t *testing.T) {
	into := []string{"a", "b", "c"}
	cases := []struct {
		placed map[string]int64
		n      int
		want   []string
	}{
		{map[string]int64{}, 5, []string{"a", "b", "c", "a", "b"}},
		{map[string]int64{"a": 1, "b": 2}, 4, []string{"c", "a", "c", "a"}},
		{map[string]int64{"a": 10}, 3, []string{"b", "c", "b"}},
		{map[string]int64{"a": 1}, 0, []string{}},
	}
	for _, tc := range cases {
		got := AssignDefaults(into, tc.placed, tc.n)
		if !slices.Equal(got, tc.want) {
			t.Errorf("AssignDefaults(%v, %d) = %v, want %v", tc.placed, tc.n, got, tc.want)
		}
	}
	// Invariant: the result never leaves the branches further apart than the input (max − min shrinks or ≤ 1).
	placed := map[string]int64{"a": 7, "b": 0, "c": 3}
	got := AssignDefaults(into, placed, 20)
	final := map[string]int64{"a": 7, "b": 0, "c": 3}
	for _, b := range got {
		final[b]++
	}
	if hi, lo := max(final["a"], final["b"], final["c"]), min(final["a"], final["b"], final["c"]); hi-lo > 1 {
		t.Fatalf("uneven: %v", final)
	}
	if placed["b"] != 0 {
		t.Fatal("input mutated")
	}
}

func TestMeritSince(t *testing.T) {
	launch := time.Date(2027, 1, 10, 1, 0, 0, 0, time.UTC) // 08:00 ICT 2027-01-10
	eraLater := time.Date(2027, 5, 20, 15, 0, 0, 0, time.UTC)
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) } // gameday.Of values
	cases := []struct {
		name string
		ws   db.WorldState
		want time.Time
	}{
		{"pre-launch", db.WorldState{}, time.Time{}},
		{"launched, no era yet", db.WorldState{StartedAt: &launch}, day(2027, 1, 10)},
		{"first era started on launch day", db.WorldState{StartedAt: &launch, EraStartedAt: &launch}, day(2027, 1, 10)},
		// 15:00 UTC = 22:00 ICT on 2027-05-20: that day mixes two eras → count from the next game day.
		{"later era mid-day", db.WorldState{StartedAt: &launch, EraStartedAt: &eraLater}, day(2027, 5, 21)},
	}
	for _, tc := range cases {
		if got := MeritSince(tc.ws); !got.Equal(tc.want) {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}
