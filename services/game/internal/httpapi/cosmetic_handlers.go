package httpapi

import (
	"errors"
	"net/http"

	"tiendao/game/internal/appearance"
	"tiendao/game/internal/cosmetic"
	"tiendao/game/internal/factionwork"
)

// cosmeticCatalog: GET /cosmetics/catalog — public, immutable catalogue (names, kinds, prices, windows, perk tiers) so
// clients resolve the ids carried by characters, chat messages and leaderboards.
func (d Deps) cosmeticCatalog(w http.ResponseWriter, _ *http.Request) {
	if d.Content == nil || d.Content.Cosmetics == nil {
		unavailable(w)
		return
	}
	c := d.Content.Cosmetics
	w.Header().Set("Cache-Control", "public, max-age=300")
	// appearances: bodies, hair accessories, robe colours, poses (appearances.yaml; display only, decisions 2026-11-09).
	writeJSON(w, 200, map[string]any{"settings": c.Settings, "seasons": c.Seasons, "series": c.AllSeries(), "cosmetics": c.All(),
		"appearances": d.Content.Appearances})
}

// cosmeticsView: GET /cosmetics — owned, buyable now, equipped.
func (d Deps) cosmeticsView(w http.ResponseWriter, r *http.Request) {
	if d.Cosmetics == nil {
		unavailable(w)
		return
	}
	v, err := d.Cosmetics.View(r.Context(), userFrom(r.Context()))
	if errors.Is(err, cosmetic.ErrNoCharacter) {
		writeError(w, 404, "character_not_found", "create a character first")
		return
	} else if err != nil {
		d.internal(w, r, "cosmetics", err)
		return
	}
	writeJSON(w, 200, v)
}

func (d Deps) cosmeticError(w http.ResponseWriter, r *http.Request, op string, err error) {
	if writeLookError(w, err) {
		return
	}
	switch {
	case errors.Is(err, cosmetic.ErrUnknown):
		writeError(w, 404, "unknown_cosmetic", "món trang sức không tồn tại")
	case errors.Is(err, cosmetic.ErrNotForSale):
		writeError(w, 409, "not_for_sale", "món này chỉ đến từ Tiên Duyên, không bán")
	case errors.Is(err, cosmetic.ErrUnavailable):
		writeError(w, 409, "unavailable", "món này chưa đến mùa hoặc đã hết thời bán")
	case errors.Is(err, cosmetic.ErrTierLocked):
		writeError(w, 409, "tier_locked", "cần Tiên Duyên cao hơn để mua món này")
	case errors.Is(err, cosmetic.ErrAlreadyOwned):
		writeError(w, 409, "already_owned", "đạo hữu đã có món này")
	case errors.Is(err, cosmetic.ErrInsufficient):
		writeError(w, 409, "insufficient_stones", "không đủ linh thạch")
	case errors.Is(err, cosmetic.ErrNotOwned):
		writeError(w, 409, "not_owned", "chưa sở hữu món này")
	case errors.Is(err, cosmetic.ErrBadLoadout):
		writeError(w, 400, "invalid_loadout", "cách trang trí không hợp lệ")
	case errors.Is(err, cosmetic.ErrKeyReuse):
		writeError(w, 409, "idempotency_key_reused", "Idempotency-Key đã dùng cho một giao dịch khác")
	case errors.Is(err, cosmetic.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	default:
		d.internal(w, r, op, err)
	}
}

// writeLookError maps appearance (body / accessory / robe / pose) errors; false = not a look error.
func writeLookError(w http.ResponseWriter, err error) bool {
	var cd *appearance.CooldownError
	switch {
	case errors.As(err, &cd):
		writeErrorDetails(w, 409, "body_cooldown", "Vừa đổi dáng người, hãy chờ thêm một lúc", map[string]any{"retryAt": cd.RetryAt.UTC()})
	case errors.Is(err, appearance.ErrUnknown):
		writeError(w, 400, "unknown_look", "ngoại hình không tồn tại")
	case errors.Is(err, appearance.ErrWrongPath):
		writeError(w, 422, "wrong_path", "ngoại hình này không dành cho đạo đồ của đạo hữu")
	case errors.Is(err, appearance.ErrNotOwned):
		writeError(w, 409, "not_owned", "chưa sở hữu ngoại hình này")
	default:
		return false
	}
	return true
}

// cosmeticsBuy: POST /cosmetics/buy {cosmeticId} + Idempotency-Key. Price comes from cosmetics.yaml only.
func (d Deps) cosmeticsBuy(w http.ResponseWriter, r *http.Request) {
	if d.Cosmetics == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		CosmeticID string `json:"cosmeticId"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := d.Cosmetics.Buy(r.Context(), userFrom(r.Context()), body.CosmeticID, key)
	if err != nil {
		d.cosmeticError(w, r, "cosmetic buy", err)
		return
	}
	d.Metrics.Purchase("cosmetic", res.Replayed)
	d.markActive(r, !res.Replayed)
	writeJSON(w, 200, res)
}

// cosmeticsEquip: POST /cosmetics/equip {titleId, avatarFrameId, chatAuraId, caveDecorationIds, look?} + Idempotency-Key —
// the whole loadout ("" / [] = unequip); `look` {bodyId, outfitId, poseId} is optional (absent = keep; accessory + robe come from the outfit).
func (d Deps) cosmeticsEquip(w http.ResponseWriter, r *http.Request) {
	if d.Cosmetics == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var body cosmetic.Appearance
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := d.Cosmetics.Equip(r.Context(), userFrom(r.Context()), body, key)
	if err != nil {
		d.cosmeticError(w, r, "cosmetic equip", err)
		return
	}
	writeJSON(w, 200, res)
}

// factionWorks: GET /factions/works — works definitions, every faction's pool, the caller's faction detail.
func (d Deps) factionWorks(w http.ResponseWriter, r *http.Request) {
	if d.FactionWorks == nil {
		unavailable(w)
		return
	}
	v, err := d.FactionWorks.View(r.Context(), userFrom(r.Context()))
	if errors.Is(err, factionwork.ErrNoCharacter) {
		writeError(w, 404, "character_not_found", "create a character first")
		return
	} else if err != nil {
		d.internal(w, r, "faction works", err)
		return
	}
	writeJSON(w, 200, v)
}

// factionDonate: POST /factions/donate {amount} + Idempotency-Key. Visual works only: no merit, rank, điểm duyên, tu vi.
func (d Deps) factionDonate(w http.ResponseWriter, r *http.Request) {
	if d.FactionWorks == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		Amount int64 `json:"amount"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := d.FactionWorks.Donate(r.Context(), userFrom(r.Context()), body.Amount, key)
	switch {
	case errors.Is(err, factionwork.ErrBadAmount):
		writeError(w, 400, "invalid_amount", "số linh thạch cúng góp không hợp lệ")
	case errors.Is(err, factionwork.ErrNoFaction):
		writeError(w, 409, "no_faction", "đạo hữu chưa gia nhập thế lực nào")
	case errors.Is(err, factionwork.ErrDailyCap):
		writeError(w, 409, "daily_cap", "hôm nay đã cúng góp đủ hạn mức")
	case errors.Is(err, factionwork.ErrInsufficient):
		writeError(w, 409, "insufficient_stones", "không đủ linh thạch")
	case errors.Is(err, factionwork.ErrKeyReuse):
		writeError(w, 409, "idempotency_key_reused", "Idempotency-Key đã dùng cho một giao dịch khác")
	case errors.Is(err, factionwork.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case err != nil:
		d.internal(w, r, "faction donate", err)
	default:
		writeJSON(w, 200, res)
	}
}
