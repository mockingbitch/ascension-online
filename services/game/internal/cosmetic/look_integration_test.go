//go:build integration

package cosmetic_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"tiendao/game/internal/appearance"
	"tiendao/game/internal/content"
	"tiendao/game/internal/cosmetic"
)

func lookEnv(t *testing.T) *env {
	t.Helper()
	e := setup(t, 0)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	e.svc.Looks, e.svc.Realms = cb.Appearances, cb.Realms
	return e
}

// look is a request as the web sends it: body + outfit + pose (accessory and robe come from the outfit).
func look(body, outfit, pose string) *appearance.Look {
	return &appearance.Look{BodyID: body, OutfitID: outfit, PoseID: pose}
}

func eq(l []string) cosmetic.Appearance { return cosmetic.Appearance{CaveDecorationIDs: l} }

func withLook(l *appearance.Look) cosmetic.Appearance {
	a := eq([]string{})
	a.Look = l
	return a
}

// The look rides the existing equip flow: validated for the path, idempotent by key, 24 h body cooldown, and a path
// change shows the paired body and grants one free re-pick (decisions 2026-11-09 #4). Display only: nothing else moves.
// Since "Bộ trang phục vẽ sẵn" a look is body + outfit + pose; changing only the outfit never starts or hits the cooldown.
func TestEquipLookCooldownRepickIdempotent(t *testing.T) {
	e := lookEnv(t)
	v, err := e.svc.View(e.ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if v.Equipped.Look == nil || v.Equipped.Look.BodyID != "tien_nam_thu_sinh" || v.Equipped.Look.OutfitID != "thu_sinh_ngoc_bich" || v.BodyRetryAt != nil {
		t.Fatalf("default look: %+v retry=%v", v.Equipped.Look, v.BodyRetryAt)
	}

	// First in-game body change is free; the cooldown starts. Hair and robe are derived from the outfit.
	want := withLook(look("tien_nu_y_su", "y_su_tu_yen", "ban_gia"))
	r, err := e.svc.Equip(e.ctx, e.user, want, "look-key-0001")
	if err != nil {
		t.Fatal(err)
	}
	if got := *r.State.Equipped.Look; got != (appearance.Look{BodyID: "tien_nu_y_su", OutfitID: "y_su_tu_yen", HairID: "day_lua", RobePaletteID: "tu", PoseID: "ban_gia"}) || r.State.BodyRetryAt == nil {
		t.Fatalf("after change: %+v retry=%v", got, r.State.BodyRetryAt)
	}
	if n := e.scalar(t, "SELECT count(*) FROM character_appearance WHERE character_id=$1 AND hair_id='day_lua' AND robe_palette_id='tu'", e.char); n != 1 {
		t.Fatalf("stored parts: %d", n)
	}
	// Same key + same loadout = replay (also when an older client sends the outfit as its parts); same key + another
	// loadout = 409; nothing written twice.
	if again, err := e.svc.Equip(e.ctx, e.user, want, "look-key-0001"); err != nil || !again.Replayed {
		t.Fatalf("replay: %+v %v", again, err)
	}
	parts := withLook(&appearance.Look{BodyID: "tien_nu_y_su", HairID: "day_lua", RobePaletteID: "tu", PoseID: "ban_gia"})
	if again, err := e.svc.Equip(e.ctx, e.user, parts, "look-key-0001"); err != nil || !again.Replayed {
		t.Fatalf("replay by parts: %+v %v", again, err)
	}
	other := withLook(look("tien_nu_y_su", "y_su_ngoc_bich", "ban_gia"))
	if _, err := e.svc.Equip(e.ctx, e.user, other, "look-key-0001"); !errors.Is(err, cosmetic.ErrKeyReuse) {
		t.Fatalf("key reuse: %v", err)
	}
	if n := e.scalar(t, "SELECT count(*) FROM cosmetic_equips WHERE character_id=$1 AND look_set", e.char); n != 1 {
		t.Fatalf("equip rows = %d", n)
	}

	// Another outfit of the same body inside the cooldown is free; another body inside 24 h is refused with the retry time.
	r, err = e.svc.Equip(e.ctx, e.user, other, "look-key-0002")
	if err != nil {
		t.Fatalf("outfit change: %v", err)
	}
	if l := r.State.Equipped.Look; l.OutfitID != "y_su_ngoc_bich" || l.RobePaletteID != "ngoc" || l.HairID != "hoa_mai" {
		t.Fatalf("outfit change: %+v", l)
	}
	var cd *appearance.CooldownError
	_, err = e.svc.Equip(e.ctx, e.user, withLook(look("tien_nu_an_tu", "an_tu_bach_y", "kiet_gia")), "look-key-0003")
	if !errors.As(err, &cd) || !cd.RetryAt.Equal(e.clock.Add(24*time.Hour)) {
		t.Fatalf("cooldown: %v", err)
	}
	// Refused: wrong path, an outfit of another body, an unknown outfit, a free mix (parts of no outfit), no outfit.
	for key, tc := range map[string]struct {
		l    *appearance.Look
		want error
	}{
		"look-key-0004": {look("vu_nu_tho_san", "tho_san_hoang_tho", "toa_thach"), appearance.ErrWrongPath},
		"look-key-0005": {look("tien_nu_y_su", "an_tu_bach_y", "ban_gia"), appearance.ErrUnknown},
		"look-key-0011": {look("tien_nu_y_su", "vuong_mien", "ban_gia"), appearance.ErrUnknown},
		"look-key-0012": {&appearance.Look{BodyID: "tien_nu_y_su", HairID: "kim_quan", RobePaletteID: "tu", PoseID: "ban_gia"}, appearance.ErrUnknown},
		"look-key-0013": {&appearance.Look{BodyID: "tien_nu_y_su", PoseID: "ban_gia"}, appearance.ErrUnknown},
	} {
		if _, err := e.svc.Equip(e.ctx, e.user, withLook(tc.l), key); !errors.Is(err, tc.want) {
			t.Fatalf("%s %+v: err = %v, want %v", key, tc.l, err, tc.want)
		}
	}
	// An equip without a look keeps it (old clients, cosmetics-only saves).
	if r, err := e.svc.Equip(e.ctx, e.user, eq([]string{}), "look-key-0006"); err != nil || r.State.Equipped.Look.OutfitID != "y_su_ngoc_bich" {
		t.Fatalf("no look: %+v %v", r.State.Equipped.Look, err)
	}

	// Path change (tiên → Vu, e.g. joining Vu Tộc): the paired body shows at once in its default outfit, and one
	// re-pick ignores the cooldown.
	e.exec(t, "UPDATE character_progressions SET realm_id='vu_1', stage_id='vu_1.tang_1' WHERE character_id=$1", e.char)
	v, err = e.svc.View(e.ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if v.Equipped.Look.BodyID != "vu_nu_tho_san" || v.Equipped.Look.OutfitID != "tho_san_hoang_tho" || v.BodyRetryAt != nil {
		t.Fatalf("after path change: %+v retry=%v", v.Equipped.Look, v.BodyRetryAt)
	}
	// Editing only the outfit keeps the pending re-pick.
	if _, err := e.svc.Equip(e.ctx, e.user, withLook(look("vu_nu_tho_san", "tho_san_mac_ung", "toa_thach")), "look-key-0007"); err != nil {
		t.Fatal(err)
	}
	r, err = e.svc.Equip(e.ctx, e.user, withLook(look("vu_nu_luc_si", "luc_si_huyet_son", "trung_binh_tan")), "look-key-0008")
	if err != nil {
		t.Fatalf("free re-pick: %v", err)
	}
	if r.State.Equipped.Look.BodyID != "vu_nu_luc_si" || r.State.Equipped.Look.OutfitID != "luc_si_huyet_son" || r.State.BodyRetryAt == nil {
		t.Fatalf("re-pick: %+v", r.State)
	}
	if _, err := e.svc.Equip(e.ctx, e.user, withLook(look("vu_nu_tho_san", "tho_san_hoang_tho", "toa_thach")), "look-key-0009"); !errors.As(err, &cd) {
		t.Fatalf("cooldown after re-pick: %v", err)
	}
	// Outfit-only change still allowed inside the cooldown.
	if _, err := e.svc.Equip(e.ctx, e.user, withLook(look("vu_nu_luc_si", "luc_si_hoang_tho", "trung_binh_tan")), "look-key-0014"); err != nil {
		t.Fatalf("outfit in cooldown: %v", err)
	}
	// A day later the cooldown is over.
	*e.clock = e.clock.Add(25 * time.Hour)
	if _, err := e.svc.Equip(e.ctx, e.user, withLook(look("vu_nu_tho_san", "tho_san_hoang_tho", "toa_thach")), "look-key-0010"); err != nil {
		t.Fatalf("after cooldown: %v", err)
	}
	// Display only: no currency moved.
	if e.balance(t) != 0 || e.scalar(t, "SELECT count(*) FROM currency_ledger WHERE character_id=$1", e.char) != 0 {
		t.Fatal("a look must never touch currency")
	}
}

// A row stored before outfits existed (free mix of accessory + robe) resolves to the body's default outfit; a stored
// pair that matches an outfit resolves to it.
func TestLegacyLookResolvesToOutfit(t *testing.T) {
	e := lookEnv(t)
	if _, err := e.svc.Equip(e.ctx, e.user, eq([]string{}), "legacy-key-01"); err != nil {
		t.Fatal(err)
	}
	e.exec(t, "UPDATE character_appearance SET body_id='tien_nu_y_su', hair_id='kim_quan', robe_palette_id='bach', pose_id='kiet_gia' WHERE character_id=$1", e.char)
	v, err := e.svc.View(e.ctx, e.user)
	if err != nil {
		t.Fatal(err)
	}
	if l := v.Equipped.Look; l.OutfitID != "y_su_ngoc_bich" || l.HairID != "hoa_mai" || l.RobePaletteID != "ngoc" || l.PoseID != "kiet_gia" {
		t.Fatalf("legacy mix: %+v", l)
	}
	e.exec(t, "UPDATE character_appearance SET hair_id='tram_go', robe_palette_id='tho' WHERE character_id=$1", e.char)
	if v, err = e.svc.View(e.ctx, e.user); err != nil || v.Equipped.Look.OutfitID != "y_su_hoang_tho" {
		t.Fatalf("stored pair: %+v %v", v.Equipped.Look, err)
	}
}

// Two concurrent body changes with different keys serialise on the appearance row lock: one wins, the other sees the
// fresh body_changed_at and gets the cooldown (anti-cheat review 2026-11-09).
func TestEquipLookConcurrentBodyChanges(t *testing.T) {
	e := lookEnv(t)
	// start the cooldown clock with a first change
	if _, err := e.svc.Equip(e.ctx, e.user, cosmetic.Appearance{CaveDecorationIDs: []string{}, Look: look("tien_nu_y_su", "y_su_tu_yen", "ban_gia")}, "conc-key-000"); err != nil {
		t.Fatal(err)
	}
	*e.clock = e.clock.Add(25 * time.Hour)
	bodies := []*appearance.Look{look("tien_nu_an_tu", "an_tu_mac_nguyet", "kiet_gia"), look("tien_nam_an_si", "an_si_mac_hac", "ban_gia")}
	var wg sync.WaitGroup
	errs := make([]error, len(bodies))
	for i, l := range bodies {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.svc.Equip(e.ctx, e.user, cosmetic.Appearance{CaveDecorationIDs: []string{}, Look: l}, "conc-key-00"+string(rune('1'+i)))
		}()
	}
	wg.Wait()
	ok, cooled := 0, 0
	var cd *appearance.CooldownError
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.As(err, &cd):
			cooled++
		default:
			t.Fatalf("unexpected: %v", err)
		}
	}
	if ok != 1 || cooled != 1 {
		t.Fatalf("ok=%d cooldown=%d, want 1/1", ok, cooled)
	}
}
