// Package content loads every catalogue file and validates cross-references between them.
// Processes (api, worker, balancesim) load content ONLY through LoadAll.
package content

import (
	"errors"
	"fmt"
	"path/filepath"

	"tiendao/game/internal/chat"
	"tiendao/game/internal/contribution"
	"tiendao/game/internal/cosmetic"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/daolu"
	"tiendao/game/internal/faction"
	"tiendao/game/internal/factionwork"
	"tiendao/game/internal/fortune"
	"tiendao/game/internal/item"
	"tiendao/game/internal/lecture"
	"tiendao/game/internal/moderation"
	"tiendao/game/internal/payment"
	"tiendao/game/internal/protector"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/secretrealm"
	"tiendao/game/internal/shop"
	"tiendao/game/internal/spiritroot"
	"tiendao/game/internal/trade"
	"tiendao/game/internal/trial"
	"tiendao/game/internal/vip"
	"tiendao/game/internal/world"
)

// Bundle is the immutable set of game catalogues.
type Bundle struct {
	Realms       *realm.Catalogue
	VIP          *vip.Table
	Items        *item.Catalogue
	SpiritRoots  *spiritroot.Catalogue
	DailyTasks   *dailytask.Catalogue
	Contribution *contribution.Config
	World        *world.Catalogue
	Moderation   *moderation.Catalogue
	Chat         *chat.Catalogue
	Lectures     *lecture.Config
	Factions     *faction.Catalogue
	Shop         *shop.Catalogue
	Fortunes     *fortune.Catalogue
	Trials       *trial.Catalogue
	Cosmetics    *cosmetic.Catalogue    // non-progress sinks (decisions 2026-11-07)
	FactionWorks *factionwork.Catalogue // cúng góp thế lực (decisions 2026-11-07)
	SecretRealms *secretrealm.Catalogue // bí cảnh thử luyện (Phase 3.4)
	Protector    *protector.Config      // hộ pháp flow (Phase 3.4)
	Payments     *payment.Catalogue     // nạp Tiên Duyên packages + framework settings (Phase 4)
	DaoLu        *daolu.Config          // đạo lữ song tu (Phase 5.1)
	Trade        *trade.Config          // trao đổi vật phẩm (Phase 5.2)
}

// LoadAll loads content/catalog/*.yaml from dir and validates cross-file references.
func LoadAll(dir string) (*Bundle, error) {
	var b Bundle
	var err error
	if b.Realms, err = realm.LoadFile(filepath.Join(dir, "realms.yaml")); err != nil {
		return nil, err
	}
	if b.VIP, err = vip.LoadFile(filepath.Join(dir, "vip.yaml")); err != nil {
		return nil, err
	}
	if b.Items, err = item.LoadFile(filepath.Join(dir, "items.yaml")); err != nil {
		return nil, err
	}
	if b.SpiritRoots, err = spiritroot.LoadFile(filepath.Join(dir, "spirit_roots.yaml")); err != nil {
		return nil, err
	}
	if b.DailyTasks, err = dailytask.LoadFile(filepath.Join(dir, "daily_tasks.yaml")); err != nil {
		return nil, err
	}
	if b.Contribution, err = contribution.LoadFile(filepath.Join(dir, "contribution.yaml")); err != nil {
		return nil, err
	}
	if b.World, err = world.LoadFile(filepath.Join(dir, "world_milestones.yaml")); err != nil {
		return nil, err
	}
	if b.Moderation, err = moderation.LoadFile(filepath.Join(dir, "moderation.yaml")); err != nil {
		return nil, err
	}
	if b.Chat, err = chat.LoadFile(filepath.Join(dir, "chat.yaml")); err != nil {
		return nil, err
	}
	if b.Lectures, err = lecture.LoadFile(filepath.Join(dir, "lectures.yaml")); err != nil {
		return nil, err
	}
	if b.Factions, err = faction.LoadFile(filepath.Join(dir, "factions.yaml")); err != nil {
		return nil, err
	}
	if b.Shop, err = shop.LoadFile(filepath.Join(dir, "shop.yaml")); err != nil {
		return nil, err
	}
	if b.Fortunes, err = fortune.LoadFile(filepath.Join(dir, "fortunes.yaml")); err != nil {
		return nil, err
	}
	if b.Trials, err = trial.LoadFile(filepath.Join(dir, "trials.yaml")); err != nil {
		return nil, err
	}
	if b.Cosmetics, err = cosmetic.LoadFile(filepath.Join(dir, "cosmetics.yaml")); err != nil {
		return nil, err
	}
	if b.FactionWorks, err = factionwork.LoadFile(filepath.Join(dir, "faction_works.yaml")); err != nil {
		return nil, err
	}
	if b.SecretRealms, err = secretrealm.LoadFile(filepath.Join(dir, "secret_realms.yaml")); err != nil {
		return nil, err
	}
	if b.Protector, err = protector.LoadFile(filepath.Join(dir, "protector.yaml")); err != nil {
		return nil, err
	}
	if b.Payments, err = payment.LoadFile(filepath.Join(dir, "payments.yaml")); err != nil {
		return nil, err
	}
	if b.DaoLu, err = daolu.LoadFile(filepath.Join(dir, "dao_lu.yaml")); err != nil {
		return nil, err
	}
	if b.Trade, err = trade.LoadFile(filepath.Join(dir, "trade.yaml")); err != nil {
		return nil, err
	}
	if err := b.crossCheck(); err != nil {
		return nil, err
	}
	return &b, nil
}

func (b *Bundle) crossCheck() error {
	var errs []error
	for _, s := range b.Realms.Stages() {
		for _, req := range s.Breakthrough.Requirements {
			if _, ok := b.Items.Get(req.ItemID); !ok {
				errs = append(errs, fmt.Errorf("stage %q requires unknown item %q", s.ID, req.ItemID))
			}
		}
	}
	// The world counts at most the personal ceiling per character-day.
	if b.World.Settings.WorldCreditCapMilli > b.Contribution.DailyCeilingMilli {
		errs = append(errs, fmt.Errorf("world_credit_cap_points exceeds contribution daily_ceiling_points"))
	}
	// Speed-item cap must be reachable through the idle-free activity bucket only (data-model §4.5).
	if b.Contribution.SpeedItemDailyMilli > b.Contribution.Buckets[contribution.Activity].CapMilli {
		errs = append(errs, fmt.Errorf("speed_items cap exceeds the activity bucket"))
	}
	for _, t := range b.DailyTasks.All() {
		for _, r := range t.Rewards {
			if r.ItemID == "" {
				continue
			}
			if _, ok := b.Items.Get(r.ItemID); !ok {
				errs = append(errs, fmt.Errorf("daily task %q rewards unknown item %q", t.ID, r.ItemID))
			}
		}
	}
	for _, ch := range b.Chat.Channels {
		if _, err := b.Realms.Stage(ch.MinStage); err != nil {
			errs = append(errs, fmt.Errorf("chat channel %q: min_stage: %w", ch.ID, err))
		}
	}
	for _, cat := range b.Chat.MaskCategories {
		if !b.Moderation.HasCategory(cat) {
			errs = append(errs, fmt.Errorf("chat mask_categories: moderation.yaml has no rule in category %q", cat))
		}
	}
	if _, ok := b.Chat.Channel(b.Lectures.ChatChannel); !ok {
		errs = append(errs, fmt.Errorf("lectures hall.chat_channel %q is not a chat.yaml channel", b.Lectures.ChatChannel))
	}
	// Each lecture điểm duyên milestone is one lecture_attendance credit; more marks than its daily max would never pay.
	if a, ok := b.VIP.Activity(vip.ActLecture); !ok || int64(len(b.Lectures.VIPMilestonesMilli)) > a.DailyMaxTimes {
		errs = append(errs, fmt.Errorf("lectures vip_milestones_points: more marks than vip.yaml %s daily_max_times", vip.ActLecture))
	}
	// Lectures pay from the activity bucket; their daily share must fit inside it.
	if b.Lectures.DailyCapMilli > b.Contribution.Buckets[contribution.Activity].CapMilli {
		errs = append(errs, fmt.Errorf("lectures daily_cap_points exceeds the activity bucket"))
	}
	// Shop (data-model §8.4): no odds items, shop-safe effects only, a non-progress cosmetic sink (decision G).
	if err := b.Shop.CrossCheck(b.Items, vip.MaxTier); err != nil {
		errs = append(errs, err)
	}
	// Speed items (§8.1): per-item daily limits alone must already respect the 10-point speed cap.
	if got := b.Items.SpeedBudgetMilli(); got > b.Contribution.SpeedItemDailyMilli {
		errs = append(errs, fmt.Errorf("items: Σ instant_points_milli × max_use_per_game_day = %d exceeds speed_items cap %d", got, b.Contribution.SpeedItemDailyMilli))
	}
	// Gieo quẻ buff (§9.4) must fit the idle buff cap; incense streak rewards fit the activity bucket.
	if b.Fortunes.MaxRateBonusBP() > b.Contribution.MaxBuffBonusBP {
		errs = append(errs, fmt.Errorf("fortunes rate_bonus_bp exceeds contribution idle.max_buff_bonus_bp"))
	}
	if b.DailyTasks.MaxStreakPointsMilli() > b.Contribution.Buckets[contribution.Activity].CapMilli {
		errs = append(errs, fmt.Errorf("daily task streak points exceed the activity bucket"))
	}
	// Factions: paths, eras, split/migrate effect ids and era windows (data model §6.4, F7).
	if err := b.Factions.CrossCheck(b.Realms, b.World); err != nil {
		errs = append(errs, err)
	} else if err := b.World.SetEraEffects(b.Factions.EraSplits(b.World)); err != nil {
		// on_close: split factions are split when the world era first reaches their close_from_era (decisions 2026-11-07).
		errs = append(errs, err)
	}
	// Trials (Phase 3.1): each gate must be a real MAJOR breakthrough on every path (tier → tier+1).
	for _, k := range b.Trials.All() {
		for _, p := range b.Realms.Paths() {
			first, ok := b.Realms.StageAt(p.ID, k.Gate.Tier, 1)
			if !ok {
				errs = append(errs, fmt.Errorf("trial %q: gate.tier %d has no realm on path %q", k.ID, k.Gate.Tier, p.ID))
				continue
			}
			last := b.Realms.LastOfRealm(first)
			if last.NextKind != realm.Major || b.Realms.Next(last) == nil {
				errs = append(errs, fmt.Errorf("trial %q: gate.tier %d has no major breakthrough on path %q", k.ID, k.Gate.Tier, p.ID))
			}
		}
	}
	// Cosmetics & faction works (decisions 2026-11-07 "Linh thạch sinks"): vip.yaml perk ids resolve to tien_duyen
	// cosmetics (one id space), era windows exist, decoration slugs don't collide, and the shop sells no cosmetic item.
	var grants []cosmetic.PerkGrant
	for _, t := range b.VIP.Tiers() {
		for _, p := range t.Perks {
			grants = append(grants, cosmetic.PerkGrant{Perk: p, Tier: t.Tier})
		}
	}
	if err := b.Cosmetics.BindPerks(grants); err != nil {
		errs = append(errs, err)
	}
	if err := b.Cosmetics.CrossCheck(b.World, vip.MaxTier); err != nil {
		errs = append(errs, err)
	}
	if err := b.FactionWorks.CrossCheck(b.Cosmetics); err != nil {
		errs = append(errs, err)
	}
	for _, l := range b.Shop.All() {
		if d, ok := b.Items.Get(l.ItemID); ok && d.Category == "cosmetic" {
			errs = append(errs, fmt.Errorf("shop listing %q sells cosmetic item %q: cosmetics are sold through cosmetics.yaml", l.ID, d.ID))
		}
	}
	errs = append(errs, b.crossCheckPhase34()...)
	errs = append(errs, b.crossCheckDaoLu()...)
	errs = append(errs, b.crossCheckTrade()...)
	// Top-up (Phase 4): package points come from the vip.yaml rate (the same function TopupLocked credits with).
	if err := b.Payments.Bind(b.VIP.PointsForVND, vip.MaxGrantDelta); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("content: cross-reference errors: %w", errors.Join(errs...))
	}
	return nil
}

// crossCheckPhase34: bí cảnh loot, pháp bảo and hộ pháp (Phase 3.4, decision F, economy rules).
//   - every loot item exists; material drops are not artifacts and raise no odds; artifact drops are artifacts;
//   - every artifact's tribulation bonus fits realms.yaml item_bonus_max_bp (one equipped ⇒ the items part never
//     exceeds the cap by content alone), and every artifact is obtainable from some secret realm (its only source);
//   - odds items appear in no other source (daily task rewards) — the shop already refuses them (Shop.CrossCheck);
//   - a realm's min_tier exists on every path; protector daily_assists covers vip.yaml protector_assist daily_max_times.
func (b *Bundle) crossCheckPhase34() []error {
	var errs []error
	t := b.Realms.Tribulation()
	dropped := map[string]bool{}
	for _, r := range b.SecretRealms.All() {
		for _, p := range b.Realms.Paths() {
			if _, ok := b.Realms.StageAt(p.ID, r.MinTier, 1); !ok {
				errs = append(errs, fmt.Errorf("secret realm %q: min_tier %d has no realm on path %q", r.ID, r.MinTier, p.ID))
			}
		}
		materials, artifacts := r.ItemIDs()
		for _, id := range materials {
			d, ok := b.Items.Get(id)
			if !ok {
				errs = append(errs, fmt.Errorf("secret realm %q drops unknown item %q", r.ID, id))
			} else if d.Category == item.CategoryArtifact || d.RaisesOdds() || d.Category == "cosmetic" {
				errs = append(errs, fmt.Errorf("secret realm %q: band item %q must be a plain material (pháp bảo go in artifacts)", r.ID, id))
			}
		}
		for _, id := range artifacts {
			d, ok := b.Items.Get(id)
			if !ok {
				errs = append(errs, fmt.Errorf("secret realm %q drops unknown artifact %q", r.ID, id))
			} else if d.Category != item.CategoryArtifact {
				errs = append(errs, fmt.Errorf("secret realm %q: artifacts entry %q is not category artifact", r.ID, id))
			}
			dropped[id] = true
		}
	}
	for _, d := range b.Items.All() {
		if d.Category != item.CategoryArtifact {
			continue
		}
		if d.TribulationBonusBP > t.ItemBonusMaxBP {
			errs = append(errs, fmt.Errorf("artifact %q: tribulation_bonus_bp %d exceeds realms.yaml item_bonus_max_bp %d (decision F)",
				d.ID, d.TribulationBonusBP, t.ItemBonusMaxBP))
		}
		if !dropped[d.ID] {
			errs = append(errs, fmt.Errorf("artifact %q drops from no secret realm (bí cảnh is its only source)", d.ID))
		}
	}
	for _, task := range b.DailyTasks.All() {
		for _, r := range task.Rewards {
			if d, ok := b.Items.Get(r.ItemID); ok && (d.Category == item.CategoryArtifact || d.RaisesOdds()) {
				errs = append(errs, fmt.Errorf("daily task %q rewards odds item %q: odds items drop only from bí cảnh", task.ID, d.ID))
			}
		}
	}
	if a, ok := b.VIP.Activity(vip.ActProtectorAssist); ok && int64(b.Protector.DailyAssists) < a.DailyMaxTimes {
		errs = append(errs, fmt.Errorf("protector.yaml daily_assists %d < vip.yaml %s daily_max_times %d (unreachable điểm duyên)",
			b.Protector.DailyAssists, vip.ActProtectorAssist, a.DailyMaxTimes))
	}
	return errs
}

// crossCheckDaoLu: đạo lữ song tu (Phase 5.1).
//   - min_tier exists on every path;
//   - a full song tu window fits the community bucket (the 150 ceiling is enforced by CreditLocked anyway);
//   - dao_lu.yaml song tu credits community-bucket source dual_cultivation; vip.yaml dual_cultivation is live (loader).
func (b *Bundle) crossCheckDaoLu() []error {
	var errs []error
	for _, p := range b.Realms.Paths() {
		if _, ok := b.Realms.StageAt(p.ID, b.DaoLu.Bond.MinTier, 1); !ok {
			errs = append(errs, fmt.Errorf("dao_lu bond.min_tier %d has no realm on path %q", b.DaoLu.Bond.MinTier, p.ID))
		}
	}
	if b.Contribution.Sources[contribution.SourceDualCultivation] != contribution.Community {
		errs = append(errs, fmt.Errorf("contribution.yaml: dual_cultivation must map to the community bucket"))
	}
	if got, lim := b.DaoLu.MaxSongTuMilli(), b.Contribution.Buckets[contribution.Community].CapMilli; got > lim {
		errs = append(errs, fmt.Errorf("dao_lu song_tu: a full window pays %d milli > community bucket cap %d", got, lim))
	}
	return errs
}

// crossCheckTrade: trao đổi (Phase 5.2, owner decision "Luật trao đổi"). Only linh thạch and plain materials: every
// trade.yaml item is a tradable plain material, and is never a breakthrough requirement item or a shop listing (nor an
// artifact/cosmetic/effect item — trade.CrossCheck); every items.yaml `tradable: true` item has a trade.yaml value; the
// receive tier gate exists on every path.
func (b *Bundle) crossCheckTrade() []error {
	var errs []error
	forbidden := trade.ForbiddenItems{}
	for _, s := range b.Realms.Stages() {
		for _, req := range s.Breakthrough.Requirements {
			forbidden[req.ItemID] = "breakthrough requirement item"
		}
	}
	for _, l := range b.Shop.All() {
		if why := forbidden[l.ItemID]; why != "" {
			forbidden[l.ItemID] = why + ", sold in shop.yaml"
		} else {
			forbidden[l.ItemID] = "sold in shop.yaml"
		}
	}
	if err := b.Trade.CrossCheck(b.Items, forbidden); err != nil {
		errs = append(errs, err)
	}
	for _, g := range []int{b.Trade.Gates.Send.MinTier, b.Trade.Gates.Receive.MinTier} {
		for _, p := range b.Realms.Paths() {
			if _, ok := b.Realms.StageAt(p.ID, g, 1); !ok {
				errs = append(errs, fmt.Errorf("trade gates min_tier %d has no realm on path %q", g, p.ID))
			}
		}
	}
	return errs
}
