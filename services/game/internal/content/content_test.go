package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const shippedDir = "../../../../content/catalog"

func TestShippedContentIsConsistent(t *testing.T) {
	if _, err := LoadAll(shippedDir); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownItemReferenceFails(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"realms.yaml", "vip.yaml", "spirit_roots.yaml", "daily_tasks.yaml", "contribution.yaml", "world_milestones.yaml", "moderation.yaml", "chat.yaml", "lectures.yaml", "factions.yaml", "shop.yaml", "fortunes.yaml", "trials.yaml", "cosmetics.yaml", "faction_works.yaml", "secret_realms.yaml", "protector.yaml", "payments.yaml", "dao_lu.yaml", "trade.yaml", "appearances.yaml", "announcements.yaml"} {
		b, err := os.ReadFile(filepath.Join(shippedDir, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	items := "version: 1\nitems:\n  - { id: linh_thao, name: Linh Thảo, category: herb, rarity: pham }\n"
	if err := os.WriteFile(filepath.Join(dir, "items.yaml"), []byte(items), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadAll(dir)
	if err == nil || !strings.Contains(err.Error(), "unknown item") {
		t.Fatalf("expected cross-ref error, got %v", err)
	}
}

func TestUnknownSplitFactionFails(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"realms.yaml", "vip.yaml", "items.yaml", "spirit_roots.yaml", "daily_tasks.yaml", "contribution.yaml", "world_milestones.yaml", "moderation.yaml", "chat.yaml", "lectures.yaml", "factions.yaml", "shop.yaml", "fortunes.yaml", "trials.yaml", "cosmetics.yaml", "faction_works.yaml", "secret_realms.yaml", "protector.yaml", "payments.yaml", "dao_lu.yaml", "trade.yaml", "appearances.yaml", "announcements.yaml"} {
		b, err := os.ReadFile(filepath.Join(shippedDir, f))
		if err != nil {
			t.Fatal(err)
		}
		if f == "world_milestones.yaml" {
			b = []byte(strings.Replace(string(b), "into: [thu_nhat_vien, lap_cuc_vien, van_luu_vien]", "into: [thu_nhat_vien, lap_cuc_vien, khong_co_vien]", 1))
		}
		if err := os.WriteFile(filepath.Join(dir, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, err := LoadAll(dir)
	if err == nil || !strings.Contains(err.Error(), `unknown faction "khong_co_vien"`) {
		t.Fatalf("expected faction cross-ref error, got %v", err)
	}
}

// shippedWith copies the shipped catalogue into a temp dir, applying string replacements per file.
func shippedWith(t *testing.T, edits map[string][2]string) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := os.ReadDir(shippedDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(shippedDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if ed, ok := edits[e.Name()]; ok {
			if !strings.Contains(string(b), ed[0]) {
				t.Fatalf("%s: %q not found", e.Name(), ed[0])
			}
			b = []byte(strings.Replace(string(b), ed[0], ed[1], 1))
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestShopRejectsOddsItems(t *testing.T) {
	// A shipped pháp bảo (items.yaml, bí cảnh drop) is fine; listing it in the shop is not (S1, Phase 3.4).
	dir := shippedWith(t, map[string][2]string{
		"shop.yaml": {"listings:\n", "listings:\n  - { id: loi_van_phu, item_id: loi_van_phu, lot_size: 1, price: 999, daily_limit_lots: 1 }\n"},
	})
	_, err := LoadAll(dir)
	if err == nil || !strings.Contains(err.Error(), "raises breakthrough/tribulation odds") {
		t.Fatalf("expected odds-item rejection, got %v", err)
	}
}

func TestShopRejectsArtifactsAndUnknownItems(t *testing.T) {
	dir := shippedWith(t, map[string][2]string{
		"shop.yaml": {"listings:\n", "listings:\n  - { id: kiem, item_id: ty_loi_chau, lot_size: 1, price: 9 }\n  - { id: ma, item_id: khong_co, lot_size: 1, price: 9 }\n"},
	})
	_, err := LoadAll(dir)
	if err == nil || !strings.Contains(err.Error(), "odds") || !strings.Contains(err.Error(), "unknown item") {
		t.Fatalf("expected artifact + unknown item rejection, got %v", err)
	}
}

func TestShopRejectsTierGateOnProgressItems(t *testing.T) {
	dir := shippedWith(t, map[string][2]string{
		"shop.yaml": {"price: 900, daily_limit_lots: 3 }", "price: 900, daily_limit_lots: 3, min_tier: 2 }"},
	})
	if _, err := LoadAll(dir); err == nil || !strings.Contains(err.Error(), "min_tier is only allowed on cosmetic") {
		t.Fatalf("expected min_tier rejection, got %v", err)
	}
}

// Decision G sink now lives in cosmetics.yaml: the shop may not sell a cosmetic item (one cosmetic id space).
func TestShopRejectsCosmeticListings(t *testing.T) {
	dir := shippedWith(t, map[string][2]string{
		"items.yaml": {"items:\n", "items:\n  - { id: khung_cu, name: Khung Cũ, category: cosmetic, rarity: huyen, unique_per_character: true }\n"},
		"shop.yaml":  {"listings:\n", "listings:\n  - { id: khung_cu, item_id: khung_cu, lot_size: 1, price: 3000 }\n"},
	})
	if _, err := LoadAll(dir); err == nil || !strings.Contains(err.Error(), "cosmetics are sold through cosmetics.yaml") {
		t.Fatalf("expected cosmetic listing rejection, got %v", err)
	}
}

// vip.yaml perk ids are the cosmetics id space: an unknown perk cosmetic fails, and so does an orphan tien_duyen entry.
func TestPerkIDsMustResolveToCosmetics(t *testing.T) {
	dir := shippedWith(t, map[string][2]string{
		"vip.yaml": {`perks: ["title:vip_1"]`, `perks: ["title:vip_khong_co"]`},
	})
	_, err := LoadAll(dir)
	if err == nil || !strings.Contains(err.Error(), `perk "title:vip_khong_co" (tier 1) has no cosmetics.yaml entry`) ||
		!strings.Contains(err.Error(), `cosmetic "title:vip_1" is source tien_duyen but no vip.yaml tier grants it`) {
		t.Fatalf("expected perk cross-check errors, got %v", err)
	}
	// A perk pointing at a purchasable cosmetic is refused too (perks are tien_duyen-only).
	dir = shippedWith(t, map[string][2]string{"vip.yaml": {`perks: ["title:vip_1"]`, `perks: ["title:vip_1", "title:so_tam"]`}})
	if _, err := LoadAll(dir); err == nil || !strings.Contains(err.Error(), "perks must be source tien_duyen") {
		t.Fatalf("expected shop-cosmetic perk rejection, got %v", err)
	}
}

// Cosmetics have no gameplay effect: any effect-like field fails the content check (cosmetics.yaml and items.yaml).
func TestCosmeticsRejectEffects(t *testing.T) {
	dir := shippedWith(t, map[string][2]string{
		"cosmetics.yaml": {"rarity: linh,  price: 1500 }", "rarity: linh,  price: 1500, rate_bonus_bp: 300 }"},
	})
	if _, err := LoadAll(dir); err == nil || !strings.Contains(err.Error(), `field "rate_bonus_bp" is a gameplay effect`) {
		t.Fatalf("expected effect rejection in cosmetics.yaml, got %v", err)
	}
	dir = shippedWith(t, map[string][2]string{
		"items.yaml": {"items:\n", "items:\n  - { id: khung_x, name: Khung X, category: cosmetic, rarity: huyen, offline_cap_bonus_hours: 2 }\n"},
	})
	if _, err := LoadAll(dir); err == nil || !strings.Contains(err.Error(), "cosmetic items may not carry effect fields") {
		t.Fatalf("expected effect rejection in items.yaml, got %v", err)
	}
	dir = shippedWith(t, map[string][2]string{
		"faction_works.yaml": {"{ at: 100000, decoration:", "{ at: 100000, merit_points: 5, decoration:"},
	})
	if _, err := LoadAll(dir); err == nil || !strings.Contains(err.Error(), `field "merit_points" is a gameplay effect`) {
		t.Fatalf("expected effect rejection in faction_works.yaml, got %v", err)
	}
}

func TestCosmeticEraWindowsAndSlugs(t *testing.T) {
	dir := shippedWith(t, map[string][2]string{"cosmetics.yaml": {"from_era: phong_than", "from_era: khong_co_era"}})
	if _, err := LoadAll(dir); err == nil || !strings.Contains(err.Error(), `unknown from_era "khong_co_era"`) {
		t.Fatalf("expected era rejection, got %v", err)
	}
	dir = shippedWith(t, map[string][2]string{"faction_works.yaml": {"slug: su_tu_da", "slug: tung_hac"}})
	if _, err := LoadAll(dir); err == nil || !strings.Contains(err.Error(), `decoration slug "tung_hac" also used in cosmetics.yaml`) {
		t.Fatalf("expected slug collision, got %v", err)
	}
}

func TestSpeedBudgetMustFitCap(t *testing.T) {
	dir := shippedWith(t, map[string][2]string{"items.yaml": {"max_use_per_game_day: 2", "max_use_per_game_day: 3"}})
	if _, err := LoadAll(dir); err == nil || !strings.Contains(err.Error(), "exceeds speed_items cap") {
		t.Fatalf("expected speed budget rejection, got %v", err)
	}
}

func TestFortuneBuffMustFitCap(t *testing.T) {
	dir := shippedWith(t, map[string][2]string{"contribution.yaml": {"max_buff_bonus_bp: 2000", "max_buff_bonus_bp: 100"}})
	if _, err := LoadAll(dir); err == nil || !strings.Contains(err.Error(), "max_buff_bonus_bp") {
		t.Fatalf("expected buff cap rejection, got %v", err)
	}
}

func TestTrialGateMustBeAMajorBreakthrough(t *testing.T) {
	// Tier 15 (Chuẩn Thánh) has no player breakthrough: a trial cannot gate it.
	dir := shippedWith(t, map[string][2]string{"trials.yaml": {"gate: { tier: 3 }", "gate: { tier: 15 }"}})
	_, err := LoadAll(dir)
	if err == nil || !strings.Contains(err.Error(), "has no major breakthrough") {
		t.Fatalf("expected trial gate rejection, got %v", err)
	}
}

// Phase 3.4 (decision F, economy rules): artifact bonuses fit item_bonus_max_bp, every artifact drops from bí cảnh (its
// only source), bí cảnh band items are plain materials, and no daily task hands out an odds item.
func TestPhase34ArtifactRules(t *testing.T) {
	for name, tc := range map[string]struct {
		edits map[string][2]string
		want  string
	}{
		"bonus above item cap": {map[string][2]string{"items.yaml": {"tribulation_bonus_bp: 500", "tribulation_bonus_bp: 800"}}, "exceeds realms.yaml item_bonus_max_bp"},
		"artifact with no drop": {map[string][2]string{"items.yaml": {"items:\n",
			"items:\n  - { id: kiem_moi, name: Kiếm Mới, category: artifact, rarity: huyen, tribulation_bonus_bp: 100, description: x }\n"}},
			"drops from no secret realm"},
		"artifact as band item": {map[string][2]string{"secret_realms.yaml": {"items: [{ item_id: linh_thao, min: 1, max: 3 }]",
			"items: [{ item_id: loi_van_phu, min: 1, max: 1 }]"}}, "must be a plain material"},
		"unknown drop":                  {map[string][2]string{"secret_realms.yaml": {"item_id: cuu_tieu_hoan,", "item_id: khong_co,"}}, "unknown artifact"},
		"protector assists below duyên": {map[string][2]string{"protector.yaml": {"daily_assists: 2", "daily_assists: 1"}}, "unreachable điểm duyên"},
	} {
		_, err := LoadAll(shippedWith(t, tc.edits))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want %q, got %v", name, tc.want, err)
		}
	}
}

// Trao đổi (Phase 5.2): a catalogue edit can never open pháp bảo, requirement items, shop items, cosmetics or premium
// currency to trading, and every tradable: true item needs a trade.yaml value.
func TestTradeCrossCheckRefusesForbiddenAssets(t *testing.T) {
	cases := []struct {
		name string
		file string
		edit [2]string
		want string
	}{
		{"artifact listed", "trade.yaml", [2]string{"  - { id: linh_thao,    kind: item,     value: 5 }",
			"  - { id: linh_thao,    kind: item,     value: 5 }\n  - { id: thanh_dong_kinh, kind: item, value: 50 }"}, "thanh_dong_kinh"},
		{"requirement item listed", "trade.yaml", [2]string{"  - { id: linh_thao,    kind: item,     value: 5 }",
			"  - { id: linh_thao,    kind: item,     value: 5 }\n  - { id: truc_co_dan, kind: item, value: 400 }"}, "breakthrough requirement item"},
		{"shop speed item made tradable", "items.yaml", [2]string{"tradable: false                   # shop speed item",
			"tradable: true                   # shop speed item"}, "tu_linh_dan"},
		{"premium currency", "trade.yaml", [2]string{"  - { id: spirit_stone, kind: currency, value: 1 }",
			"  - { id: spirit_stone, kind: currency, value: 1 }\n  - { id: premium, kind: currency, value: 1 }"}, "only tradable currency"},
		{"tradable item without value", "trade.yaml", [2]string{"  - { id: linh_thao,    kind: item,     value: 5 }\n", ""}, `item "linh_thao" is tradable: true`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := LoadAll(shippedWith(t, map[string][2]string{c.file: c.edit}))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error containing %q, got %v", c.want, err)
			}
		})
	}
}

func TestAppearanceUnknownPathFails(t *testing.T) {
	dir := shippedWith(t, map[string][2]string{
		"appearances.yaml": {"{ id: tram_go,   name: Trâm gỗ,    paths: [tien],", "{ id: tram_go,   name: Trâm gỗ,    paths: [tien, yeu],"},
	})
	_, err := LoadAll(dir)
	if err == nil || !strings.Contains(err.Error(), `unknown path "yeu"`) {
		t.Fatalf("expected appearance path error, got %v", err)
	}
}
