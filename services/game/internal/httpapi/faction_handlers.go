package httpapi

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"tiendao/game/internal/faction"
	"tiendao/game/internal/realm"
)

// writeFactionError maps faction domain errors to HTTP (false = not a faction error).
func writeFactionError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, faction.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case errors.Is(err, faction.ErrUnknownFaction):
		writeError(w, 404, "unknown_faction", "thế lực không tồn tại")
	case errors.Is(err, faction.ErrFactionClosed):
		writeError(w, 409, "faction_closed", "thế lực chưa mở hoặc đã đóng cửa trong thời kỳ này")
	case errors.Is(err, faction.ErrPathNotAllowed):
		writeError(w, 400, "path_not_allowed", "thế lực không nhận đạo đồ này")
	case errors.Is(err, faction.ErrAlreadyMember):
		writeError(w, 409, "already_member", "đạo hữu đã có thế lực; hãy dùng đổi phe")
	case errors.Is(err, faction.ErrNotMember):
		writeError(w, 409, "not_member", "đạo hữu chưa gia nhập thế lực nào")
	case errors.Is(err, faction.ErrSameFaction):
		writeError(w, 409, "same_faction", "đạo hữu đã ở thế lực này")
	case errors.Is(err, faction.ErrCooldown):
		writeError(w, 409, "switch_cooldown", "chưa hết thời gian chờ đổi phe")
	case errors.Is(err, faction.ErrLocked):
		writeError(w, 409, "faction_locked", "thế lực của đạo hữu đang bị khóa đổi phe")
	case errors.Is(err, faction.ErrNoPendingSplit):
		writeError(w, 409, "no_pending_split", "không có lựa chọn phân nhánh nào đang chờ")
	case errors.Is(err, faction.ErrNotABranch):
		writeError(w, 400, "not_a_branch", "đây không phải một nhánh của lần phân chia")
	case errors.Is(err, faction.ErrChoiceClosed):
		writeError(w, 409, "choice_closed", "đã hết hạn chọn nhánh")
	case errors.Is(err, faction.ErrIdempotencyReused):
		writeError(w, 409, "idempotency_key_reused", "Idempotency-Key already used for a different faction change")
	case errors.Is(err, faction.ErrBadCursor):
		writeError(w, 400, "invalid_cursor", "invalid cursor")
	case errors.Is(err, faction.ErrRankingUnavailable):
		writeError(w, 503, "ranking_unavailable", "bảng xếp hạng thế lực tạm thời không khả dụng")
	default:
		return false
	}
	return true
}

// listFactions: GET /factions — catalogue, open flags, member counts and the era's merit ranking. Public.
func (d Deps) listFactions(w http.ResponseWriter, r *http.Request) {
	if d.Factions == nil {
		unavailable(w)
		return
	}
	v, err := d.Factions.List(r.Context())
	if err != nil {
		d.internal(w, r, "list factions", err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=15")
	writeJSON(w, 200, v)
}

func (d Deps) factionMe(w http.ResponseWriter, r *http.Request) {
	if d.Factions == nil {
		unavailable(w)
		return
	}
	m, err := d.Factions.Me(r.Context(), userFrom(r.Context()))
	if writeFactionError(w, err) {
		return
	} else if err != nil {
		d.internal(w, r, "faction me", err)
		return
	}
	writeJSON(w, 200, m)
}

func (d Deps) factionMembers(w http.ResponseWriter, r *http.Request) {
	if d.Factions == nil {
		unavailable(w)
		return
	}
	p, err := d.Factions.Members(r.Context(), r.PathValue("factionId"), r.URL.Query().Get("cursor"))
	if writeFactionError(w, err) {
		return
	} else if err != nil {
		d.internal(w, r, "faction members", err)
		return
	}
	writeJSON(w, 200, p)
}

// factionChange handles join / switch (body { factionId, path? }) and split-choice (body { factionId }).
func (d Deps) factionChange(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Factions == nil {
			unavailable(w)
			return
		}
		key, ok := idempotencyKey(w, r)
		if !ok {
			return
		}
		var body struct {
			FactionID string       `json:"factionId"`
			Path      realm.PathID `json:"path"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		ctx, user := r.Context(), userFrom(r.Context())
		var res faction.ChangeResult
		var err error
		switch kind {
		case "join":
			res, err = d.Factions.Join(ctx, user, key, body.FactionID, body.Path)
		case "switch":
			res, err = d.Factions.Switch(ctx, user, key, body.FactionID, body.Path)
		default:
			if body.Path != "" {
				writeError(w, 400, "invalid_body", "path is not accepted for a split choice")
				return
			}
			res, err = d.Factions.ChooseSplit(ctx, user, key, body.FactionID)
		}
		if writeFactionError(w, err) {
			return
		} else if err != nil {
			d.internal(w, r, "faction "+kind, err)
			return
		}
		d.markActive(r, !res.Replayed && kind != "split_choice") // join / switch only
		writeJSON(w, 200, res)
	}
}

// adminFactionLock: POST /admin/factions/lock { characterId, reason, hours, note } and /unlock { characterId, note }.
func (d Deps) adminFactionLock(lock bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Factions == nil {
			unavailable(w)
			return
		}
		var body struct {
			CharacterID uuid.UUID `json:"characterId"`
			Reason      string    `json:"reason"`
			Hours       int       `json:"hours"`
			Note        string    `json:"note"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if !lock && (body.Reason != "" || body.Hours != 0) {
			writeError(w, 400, "invalid_lock", "unlock takes only characterId and note")
			return
		}
		if lock && body.Reason == "" {
			writeError(w, 400, "invalid_lock", "reason is required (fengshen | admin | event)")
			return
		}
		v, err := d.Factions.SetLock(r.Context(), userFrom(r.Context()), body.CharacterID, body.Reason, body.Hours, body.Note)
		switch {
		case errors.Is(err, faction.ErrInvalidLock):
			writeError(w, 400, "invalid_lock", err.Error())
		case errors.Is(err, faction.ErrNotMember):
			writeError(w, 404, "not_member", "character has no faction")
		case err != nil:
			d.internal(w, r, "faction lock", err)
		default:
			writeJSON(w, 200, v)
		}
	}
}
