package appearance

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"tiendao/game/internal/realm"
)

func catalogDir(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "content", "catalog")
}

func shipped(t *testing.T) *Catalogue {
	t.Helper()
	c, err := LoadFile(filepath.Join(catalogDir(t), "appearances.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestShippedAppearances(t *testing.T) {
	c := shipped(t)
	realms, err := realm.LoadFile(filepath.Join(catalogDir(t), "realms.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.CrossCheck(realms); err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, b := range c.Bodies {
		count[string(b.Path)+"/"+b.Sex]++
	}
	// decisions 2026-11-09 #2: tiên 4 nam + 4 nữ, Vu 2 + 2, all free.
	for k, min := range map[string]int{"tien/nam": 4, "tien/nu": 4, "vu/nam": 2, "vu/nu": 2} {
		if count[k] < min {
			t.Errorf("%s: %d bodies, want ≥ %d", k, count[k], min)
		}
	}
	if c.Settings.BodyChangeCooldownSeconds != 86400 {
		t.Errorf("cooldown = %d, want 24 h (decision #4)", c.Settings.BodyChangeCooldownSeconds)
	}
	// decision "Bộ trang phục vẽ sẵn": exactly 3 outfits per body, ids unique across bodies, parts allowed on the
	// path, robe colours distinct within a body, the default outfit among them; female bodies' briefs stay modest.
	seen := map[string]bool{}
	for _, b := range c.Bodies {
		if len(b.Outfits) != OutfitsPerBody {
			t.Errorf("%s: %d outfits", b.ID, len(b.Outfits))
		}
		robes := map[string]bool{}
		for _, o := range b.Outfits {
			if seen[o.ID] {
				t.Errorf("outfit id %s duplicated", o.ID)
			}
			seen[o.ID] = true
			if !c.hairOK(o.HairID, b.Path) || !c.robeOK(o.RobePaletteID, b.Path) {
				t.Errorf("%s/%s: parts not allowed on %s", b.ID, o.ID, b.Path)
			}
			if robes[o.RobePaletteID] {
				t.Errorf("%s: robe %s twice", b.ID, o.RobePaletteID)
			}
			robes[o.RobePaletteID] = true
			if len(o.Art) < 40 {
				t.Errorf("%s/%s: art brief too short for the image model", b.ID, o.ID)
			}
			if b.Sex == "nu" {
				for _, bad := range []string{"strapless", "cleavage", "sweetheart", "bare shoulder", "slit", "revealing"} {
					if strings.Contains(strings.ToLower(o.Art), bad) {
						t.Errorf("%s/%s: %q breaks the modest rule", b.ID, o.ID, bad)
					}
				}
			}
		}
		if d, ok := b.Outfit(b.DefaultOutfit); !ok || d.HairID != b.DefaultHair || d.RobePaletteID != b.DefaultRobe {
			t.Errorf("%s: default outfit %q / parts %s+%s", b.ID, b.DefaultOutfit, b.DefaultHair, b.DefaultRobe)
		}
	}
	// Every body resolves to itself on its own path and to its pair on the other.
	for _, b := range c.Bodies {
		if got := c.Resolve(Look{BodyID: b.ID}, b.Path); got.BodyID != b.ID {
			t.Errorf("%s on own path → %s", b.ID, got.BodyID)
		}
		other := realm.PathID("vu")
		if b.Path == "vu" {
			other = "tien"
		}
		got := c.Resolve(Look{BodyID: b.ID}, other)
		if got.BodyID != b.Pair {
			t.Errorf("%s on %s → %s, want pair %s", b.ID, other, got.BodyID, b.Pair)
		}
		if err := c.Validate(got, other); err != nil {
			t.Errorf("resolved look of %s on %s invalid: %v", b.ID, other, err)
		}
	}
}

const minimal = `version: 1
settings: { body_change_cooldown_seconds: 60 }
bodies:
  - { id: a, path: tien, sex: nam, name: A, svg_hair: bui_cao, default_outfit: a1, default_pose: p, pair: b, default_free: true,
      outfits: [{ id: a1, name: A1, robe_palette: r, hair_accessory: h, art: x }, { id: a2, name: A2, robe_palette: r2, hair_accessory: h2, art: x },
                { id: a3, name: A3, robe_palette: r3, hair_accessory: h, art: x }] }
  - { id: b, path: vu, sex: nam, name: B, svg_hair: chom, default_outfit: b1, default_pose: q, pair: a, default_free: true,
      outfits: [{ id: b1, name: B1, robe_palette: r, hair_accessory: h, art: x }, { id: b2, name: B2, robe_palette: r3, hair_accessory: h, art: x },
                { id: b3, name: B3, robe_palette: rv, hair_accessory: h, art: x }] }
  - { id: c, path: vu, sex: nam, name: C, svg_hair: chom, default_outfit: c1, default_pose: q, pair: a, default_free: true,
      outfits: [{ id: c1, name: C1, robe_palette: r, hair_accessory: h, art: x }, { id: c2, name: C2, robe_palette: r3, hair_accessory: h, art: x },
                { id: c3, name: C3, robe_palette: rv, hair_accessory: h, art: x }] }
hair_accessories:
  - { id: h, name: H, paths: [tien, vu], default_free: true }
  - { id: h2, name: H2, paths: [tien], default_free: true }
robe_palettes:
  - { id: r, name: R, paths: [tien, vu], default_free: true }
  - { id: r2, name: R2, paths: [tien], default_free: true }
  - { id: r3, name: R3, paths: [tien, vu], default_free: true }
  - { id: rv, name: RV, paths: [vu], default_free: true }
poses:
  - { id: p, name: P, path: tien, default_free: true }
  - { id: q, name: Q, path: vu, default_free: true }
`

func TestLoadRejects(t *testing.T) {
	cases := map[string]struct{ from, to, want string }{
		"effect on body":   {"pair: b, default_free: true,", "pair: b, default_free: true, rate_bonus_bp: 100,", "gameplay effect"},
		"effect on outfit": {"hair_accessory: h2, art: x }", "hair_accessory: h2, art: x, rate_bonus_bp: 1 }", "gameplay effect"},
		"two outfits":      {",\n                { id: a3, name: A3, robe_palette: r3, hair_accessory: h, art: x }]", "]", "exactly 3"},
		"four outfits":     {"{ id: a3, name: A3, robe_palette: r3, hair_accessory: h, art: x }]", "{ id: a3, name: A3, robe_palette: r3, hair_accessory: h, art: x }, { id: a4, name: A4, robe_palette: r, hair_accessory: h2, art: x }]", "exactly 3"},
		"dup outfit id":    {"{ id: c3, name: C3", "{ id: b3, name: C3", "duplicate outfit id"},
		"outfit robe path": {"{ id: b3, name: B3, robe_palette: rv", "{ id: b3, name: B3, robe_palette: r2", "robe_palette"},
		"outfit hair path": {"{ id: b3, name: B3, robe_palette: rv, hair_accessory: h,", "{ id: b3, name: B3, robe_palette: rv, hair_accessory: h2,", "hair_accessory"},
		"same robe twice":  {"{ id: a3, name: A3, robe_palette: r3", "{ id: a3, name: A3, robe_palette: r", "used twice"},
		"no art":           {"hair_accessory: h2, art: x }", "hair_accessory: h2, art: \"\" }", "art required"},
		"default outfit":   {"default_outfit: a1", "default_outfit: b1", "default_outfit"},
		"effect on option": {"{ id: h2, name: H2, paths: [tien], default_free: true }", "{ id: h2, name: H2, paths: [tien], default_free: true, tribulation_bonus_bp: 5 }", "gameplay effect"},
		"effect points":    {"{ id: p, name: P, path: tien, default_free: true }", "{ id: p, name: P, path: tien, default_free: true, rank_points: 1 }", "gameplay effect"},
		"unknown field":    {"{ id: r, name: R, paths: [tien, vu], default_free: true }", "{ id: r, name: R, paths: [tien, vu], default_free: true, glow: big }", "unknown field"},
		"not free":         {"{ id: h2, name: H2, paths: [tien], default_free: true }", "{ id: h2, name: H2, paths: [tien], default_free: false }", "default_free"},
		"pair same path":   {"default_pose: q, pair: a, default_free: true,\n      outfits: [{ id: b1", "default_pose: q, pair: c, default_free: true,\n      outfits: [{ id: b1", "other path"},
		"bad svg hair":     {"svg_hair: chom", "svg_hair: mohawk", "svg_hair"},
		"pose wrong path":  {"default_pose: q, pair", "default_pose: p, pair", "default_pose"},
		"robe unknown":     {"robe_palette: rv, hair_accessory: h, art: x }] }\n  - { id: c", "robe_palette: zz, hair_accessory: h, art: x }] }\n  - { id: c", "robe_palette"},
		"bad sex":          {"sex: nam, name: B", "sex: x, name: B", "sex"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src := strings.Replace(minimal, tc.from, tc.to, 1)
			if src == minimal {
				t.Fatalf("replacement %q not found", tc.from)
			}
			_, err := Load(strings.NewReader(src))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
	if _, err := Load(strings.NewReader(minimal)); err != nil {
		t.Fatalf("minimal: %v", err)
	}
}

func TestResolveAndValidate(t *testing.T) {
	c, err := Load(strings.NewReader(minimal))
	if err != nil {
		t.Fatal(err)
	}
	// Nothing chosen → path default body with its default outfit.
	if got := c.Resolve(Look{}, "tien"); got != (Look{BodyID: "a", OutfitID: "a1", HairID: "h", RobePaletteID: "r", PoseID: "p"}) {
		t.Errorf("default tien = %+v", got)
	}
	// A stored (hair, robe) pair names its outfit; OutfitID wins when present.
	if got := c.Resolve(Look{BodyID: "a", HairID: "h2", RobePaletteID: "r2", PoseID: "p"}, "tien"); got.OutfitID != "a2" {
		t.Errorf("by parts = %+v", got)
	}
	if got := c.Resolve(Look{BodyID: "a", OutfitID: "a3", HairID: "h2", RobePaletteID: "r2"}, "tien"); got.OutfitID != "a3" || got.RobePaletteID != "r3" || got.HairID != "h" {
		t.Errorf("by id = %+v", got)
	}
	// A pre-outfit free mix (h2 + r matches no outfit) → the body's default outfit.
	if got := c.Resolve(Look{BodyID: "a", HairID: "h2", RobePaletteID: "r", PoseID: "p"}, "tien"); got.OutfitID != "a1" || got.HairID != "h" {
		t.Errorf("free mix = %+v", got)
	}
	// Path change: pair body b with its default outfit; pose p (tiên) → q.
	got := c.Resolve(Look{BodyID: "a", OutfitID: "a2", HairID: "h2", RobePaletteID: "r2", PoseID: "p"}, "vu")
	if got != (Look{BodyID: "b", OutfitID: "b1", HairID: "h", RobePaletteID: "r", PoseID: "q"}) {
		t.Errorf("after path change = %+v", got)
	}
	// Unknown stored ids fall back.
	if got := c.Resolve(Look{BodyID: "gone", HairID: "gone"}, "vu"); got.BodyID != "b" || got.OutfitID != "b1" {
		t.Errorf("retired ids = %+v", got)
	}
	// Normalize: outfit → parts (client parts ignored); parts → outfit (older client).
	if n := c.Normalize(Look{BodyID: "a", OutfitID: "a2", HairID: "h", RobePaletteID: "r", PoseID: "p"}); n.HairID != "h2" || n.RobePaletteID != "r2" {
		t.Errorf("normalize by id = %+v", n)
	}
	if n := c.Normalize(Look{BodyID: "a", HairID: "h", RobePaletteID: "r3", PoseID: "p"}); n.OutfitID != "a3" {
		t.Errorf("normalize by parts = %+v", n)
	}
	for name, tc := range map[string]struct {
		l    Look
		want error
	}{
		"ok":               {Look{"a", "a2", "h2", "r2", "p"}, nil},
		"wrong path":       {Look{"b", "b1", "h", "r", "q"}, ErrWrongPath},
		"pose path":        {Look{"a", "a2", "h2", "r2", "q"}, ErrWrongPath},
		"other body":       {Look{"a", "b2", "h", "r3", "p"}, ErrUnknown},
		"unknown outfit":   {Look{"a", "nope", "h", "r", "p"}, ErrUnknown},
		"no outfit":        {Look{"a", "", "h", "r", "p"}, ErrUnknown},
		"parts mismatched": {Look{"a", "a2", "h", "r", "p"}, ErrUnknown},
		"empty":            {Look{}, ErrUnknown},
	} {
		if err := c.Validate(tc.l, "tien"); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	// Every outfit of every body validates on its path once normalized.
	for _, b := range c.Bodies {
		for _, o := range b.Outfits {
			if err := c.Validate(c.Normalize(Look{BodyID: b.ID, OutfitID: o.ID, PoseID: b.DefaultPose}), b.Path); err != nil {
				t.Errorf("%s/%s: %v", b.ID, o.ID, err)
			}
		}
	}
}

func TestPlanCooldownAndRepick(t *testing.T) {
	c, err := Load(strings.NewReader(minimal))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-30 * time.Second)
	old := now.Add(-2 * time.Minute)
	toA := Look{BodyID: "a", OutfitID: "a1", HairID: "h", RobePaletteID: "r", PoseID: "p"}
	toB := Look{BodyID: "b", OutfitID: "b1", HairID: "h", RobePaletteID: "r", PoseID: "q"}

	// Same body as shown: no body change, stored body kept (even when empty).
	// Changing only the outfit inside the cooldown: allowed, no body change.
	ch, err := c.Plan(Look{}, &recent, Look{BodyID: "a", OutfitID: "a2", HairID: "h2", RobePaletteID: "r2", PoseID: "p"}, "tien", now)
	if err != nil || ch.BodyChanged || ch.Store.BodyID != "" || ch.Store.HairID != "h2" || ch.Store.RobePaletteID != "r2" {
		t.Fatalf("same body: %+v %v", ch, err)
	}
	// Never changed in game (onboarding pick) → free.
	if ch, err := c.Plan(Look{BodyID: "b"}, nil, toB, "vu", now); err != nil || ch.BodyChanged {
		t.Fatalf("same: %+v %v", ch, err)
	}
	// Path changed (stored a = tiên, now Vu shows pair b): keeping b does not consume the re-pick.
	ch, err = c.Plan(Look{BodyID: "a"}, &recent, toB, "vu", now)
	if err != nil || ch.BodyChanged || ch.Store.BodyID != "a" {
		t.Fatalf("keep pair: %+v %v", ch, err)
	}
	// Cooldown: a change within the window is refused with the retry time.
	var cd *CooldownError
	if _, err := c.Plan(Look{BodyID: "a"}, &recent, Look{BodyID: "x"}, "tien", now); err == nil {
		t.Fatal("unknown body must not be planned as no-op")
	} else if !errors.As(err, &cd) {
		// x is not a body: Validate runs before Plan in the service; here it is just "a different body".
		t.Fatalf("err = %v", err)
	}
	if !cd.RetryAt.Equal(recent.Add(time.Minute)) {
		t.Errorf("retryAt = %v", cd.RetryAt)
	}
	if ch, err := c.Plan(Look{BodyID: "a"}, &old, Look{BodyID: "x"}, "tien", now); err != nil || !ch.BodyChanged {
		t.Fatalf("after cooldown: %+v %v", ch, err)
	}
	// Path changed: one re-pick to another Vu body is free even inside the cooldown …
	toC := Look{BodyID: "c", OutfitID: "c1", HairID: "h", RobePaletteID: "r", PoseID: "q"}
	ch, err = c.Plan(Look{BodyID: "a"}, &recent, toC, "vu", now)
	if err != nil || !ch.BodyChanged || ch.Store.BodyID != "c" {
		t.Fatalf("re-pick: %+v %v", ch, err)
	}
	// … and once stored on the new path, the cooldown applies again.
	if _, err := c.Plan(ch.Store, &now, toB, "vu", now.Add(time.Second)); !errors.As(err, &cd) {
		t.Fatalf("after re-pick: %v", err)
	}
	_ = toA
}
