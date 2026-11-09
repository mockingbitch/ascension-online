package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"tiendao/game/internal/appearance"
	"tiendao/game/internal/breakthrough"
	"tiendao/game/internal/character"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/realm"
)

// ───── catalogue (public, cacheable) ─────

func (d Deps) listRealms(w http.ResponseWriter, _ *http.Request) {
	r := d.Content.Realms
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, 200, map[string]any{"version": r.Version, "settings": r.Settings, "paths": r.Paths(),
		"tribulation": r.Tribulation(), "realms": r.Realms()})
}

func (d Deps) listItems(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, 200, map[string]any{"items": d.Content.Items.All()})
}

func (d Deps) listVIPTiers(w http.ResponseWriter, _ *http.Request) {
	t := d.Content.VIP
	tiers := make([]map[string]any, 0, len(t.Tiers()))
	for _, tr := range t.Tiers() {
		tiers = append(tiers, map[string]any{"tier": tr.Tier, "pointsRequired": tr.PointsRequired, "benefits": t.Benefits(tr.Tier)})
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, 200, map[string]any{"tiers": tiers, "displayName": t.DisplayName, "pointName": t.PointName,
		"pointsPer1000Vnd": t.PointsPer1000VND, "earnDailyCap": t.EarnDailyCap, "activities": t.Activities()})
}

// ───── VIP (Tiên Duyên) ─────

func (d Deps) vipStatus(w http.ResponseWriter, r *http.Request) {
	if d.VIP == nil {
		unavailable(w)
		return
	}
	st, err := d.VIP.Status(r.Context(), userFrom(r.Context()))
	if err != nil {
		d.internal(w, r, "vip status", err)
		return
	}
	writeJSON(w, 200, st)
}

// ───── character ─────

func (d Deps) getCharacter(w http.ResponseWriter, r *http.Request) {
	if d.Characters == nil {
		unavailable(w)
		return
	}
	v, err := d.Characters.Get(r.Context(), userFrom(r.Context()))
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, 404, "character_not_found", "create a character first")
		return
	} else if err != nil {
		d.internal(w, r, "get character", err)
		return
	}
	writeJSON(w, 200, v)
}

func (d Deps) createCharacter(w http.ResponseWriter, r *http.Request) {
	if d.Characters == nil {
		unavailable(w)
		return
	}
	var body struct {
		Name      string       `json:"name"`
		FactionID string       `json:"factionId"` // onboarding (§9.6): create + join in one tx; needs Idempotency-Key
		Path      realm.PathID `json:"path"`
		// onboarding look (appearances.yaml; optional, display only): {bodyId, outfitId, poseId}
		Appearance *appearance.Look `json:"appearance"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	var v character.View
	var err error
	if body.FactionID == "" {
		if body.Path != "" || body.Appearance != nil {
			writeError(w, 400, "invalid_body", "path and appearance need a factionId")
			return
		}
		v, err = d.Characters.Create(r.Context(), userFrom(r.Context()), body.Name)
	} else {
		key, ok := idempotencyKey(w, r)
		if !ok {
			return
		}
		v, err = d.Characters.CreateOnboarded(r.Context(), userFrom(r.Context()), body.Name,
			character.Onboarding{IdempotencyKey: key, FactionID: body.FactionID, Path: body.Path, Look: body.Appearance})
	}
	if writeLookError(w, err) {
		return
	}
	if msg, ok := nameBlockedMessage(err); ok {
		writeError(w, 400, "name_not_allowed", msg)
		return
	}
	switch {
	case errors.Is(err, character.ErrInvalidName):
		writeError(w, 400, "invalid_name", "Tên 2–24 chữ cái, chỉ dùng chữ (có dấu) và khoảng trắng")
	case errors.Is(err, character.ErrNameTaken):
		writeError(w, 409, "name_taken", "Đạo hiệu này đã có người dùng")
	case errors.Is(err, character.ErrAlreadyExists):
		writeError(w, 409, "character_exists", "account already has a character")
	case errors.Is(err, character.ErrNoFactions), errors.Is(err, character.ErrNoLooks):
		unavailable(w)
	case err != nil:
		if !writeFactionError(w, err) {
			d.internal(w, r, "create character", err)
		}
	case v.Replayed:
		writeJSON(w, 200, v)
	default:
		writeJSON(w, 201, v)
	}
}

// ───── cultivation ─────

func (d Deps) settle(w http.ResponseWriter, r *http.Request) {
	if d.Cultivation == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	st, err := d.Cultivation.Settle(r.Context(), userFrom(r.Context()), key)
	switch {
	case errors.Is(err, cultivation.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case err != nil:
		d.internal(w, r, "settle", err)
	default:
		d.Metrics.Settle(st.Replayed)
		d.markActive(r, !st.Replayed && st.EarnedMilli > 0)
		writeJSON(w, 200, st)
	}
}

// ───── breakthrough ─────

func (d Deps) breakthroughPreview(w http.ResponseWriter, r *http.Request) {
	if d.Breakthrough == nil {
		unavailable(w)
		return
	}
	p, err := d.Breakthrough.Preview(r.Context(), userFrom(r.Context()))
	switch {
	case errors.Is(err, cultivation.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case err != nil:
		d.internal(w, r, "breakthrough preview", err)
	default:
		writeJSON(w, 200, p)
	}
}

func (d Deps) breakthroughAttempt(w http.ResponseWriter, r *http.Request) {
	if d.Breakthrough == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	// Optional body {acceptLossMilli, acceptSuccessBp}: guards only (the server never uses them as values).
	var body struct {
		AcceptLossMilli *int64 `json:"acceptLossMilli"`
		AcceptSuccessBP *int64 `json:"acceptSuccessBp"` // M3: refuse if the odds dropped below what the player saw
	}
	if r.ContentLength != 0 && r.Body != nil {
		if !decodeJSON(w, r, &body) {
			return
		}
		if body.AcceptLossMilli != nil && *body.AcceptLossMilli < 0 {
			writeError(w, 400, "invalid_body", "acceptLossMilli must be ≥ 0")
			return
		}
	}
	res, err := d.Breakthrough.AttemptGuarded(r.Context(), userFrom(r.Context()), key,
		breakthrough.Guard{AcceptLossMilli: body.AcceptLossMilli, AcceptSuccessBP: body.AcceptSuccessBP})
	var nr *breakthrough.ErrNotReady
	var qc *breakthrough.ErrQuoteChanged
	var qr *breakthrough.ErrQuoteRequired
	switch {
	case errors.As(err, &qr):
		writeErrorDetails(w, 400, "quote_required", "Lôi kiếp cần xác nhận số tu vi có thể mất (acceptLossMilli)", qr.Preview)
	case errors.As(err, &nr):
		writeErrorDetails(w, 409, "not_ready", "breakthrough requirements not met", nr.Preview)
	case errors.As(err, &qc):
		writeErrorDetails(w, 409, "quote_changed", "Tình thế đã thay đổi (tu vi có thể mất hoặc tỉ lệ vượt kiếp) — hãy xem lại", qc.Preview)
	case errors.Is(err, cultivation.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case err != nil:
		d.internal(w, r, "breakthrough attempt", err)
	default:
		d.Metrics.Breakthrough(res.Success, res.Tribulation, res.Replayed)
		d.markActive(r, !res.Replayed)
		writeJSON(w, 200, res)
	}
}

// ───── leaderboard ─────

func (d Deps) leaderboardPage(w http.ResponseWriter, r *http.Request) {
	if d.Leaderboard == nil {
		unavailable(w)
		return
	}
	offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	limit, err := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64)
	if err != nil {
		limit = 50
	}
	if offset > 100_000 {
		writeError(w, 400, "offset_too_large", "offset must be ≤ 100000")
		return
	}
	entries, total, err := d.Leaderboard.Page(r.Context(), offset, limit)
	if errors.Is(err, leaderboard.ErrUnavailable) {
		writeError(w, 503, "leaderboard_unavailable", "Thiên Bảng đang được cập nhật, thử lại sau")
		return
	} else if err != nil {
		d.internal(w, r, "leaderboard", err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=10")
	writeJSON(w, 200, map[string]any{"entries": entries, "total": total, "offset": offset})
}

func (d Deps) leaderboardMe(w http.ResponseWriter, r *http.Request) {
	if d.Leaderboard == nil || d.Characters == nil {
		unavailable(w)
		return
	}
	v, err := d.Characters.Get(r.Context(), userFrom(r.Context()))
	if errors.Is(err, character.ErrNotFound) {
		writeError(w, 404, "character_not_found", "create a character first")
		return
	} else if err != nil {
		d.internal(w, r, "leaderboard me", err)
		return
	}
	rank, score, err := d.Leaderboard.RankOf(r.Context(), v.ID)
	if errors.Is(err, leaderboard.ErrUnavailable) {
		writeError(w, 503, "leaderboard_unavailable", "Thiên Bảng đang được cập nhật, thử lại sau")
		return
	} else if err != nil {
		d.internal(w, r, "leaderboard me", err)
		return
	}
	writeJSON(w, 200, map[string]any{"rank": rank, "score": score, "characterId": v.ID})
}
