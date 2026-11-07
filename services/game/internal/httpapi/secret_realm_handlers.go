package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"tiendao/game/internal/artifact"
	"tiendao/game/internal/protector"
	"tiendao/game/internal/secretrealm"
)

// ───── bí cảnh (Phase 3.4) ─────

func (d Deps) secretRealmStatus(w http.ResponseWriter, r *http.Request) {
	if d.SecretRealms == nil {
		unavailable(w)
		return
	}
	st, err := d.SecretRealms.Status(r.Context(), userFrom(r.Context()))
	if !d.writeSecretRealmError(w, r, "secret realm status", err) {
		writeJSON(w, 200, st)
	}
}

func (d Deps) secretRealmStart(w http.ResponseWriter, r *http.Request) {
	if d.SecretRealms == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	v, err := d.SecretRealms.Start(r.Context(), userFrom(r.Context()), r.PathValue("realmId"), key)
	if d.writeSecretRealmError(w, r, "secret realm start", err) {
		return
	}
	status := 201
	if v.Resumed {
		status = 200
	}
	writeJSON(w, status, v)
}

func (d Deps) secretRealmSubmit(w http.ResponseWriter, r *http.Request) {
	if d.SecretRealms == nil {
		unavailable(w)
		return
	}
	runID, err := uuid.Parse(r.PathValue("runId"))
	if err != nil {
		writeError(w, 404, "run_not_found", "Không tìm thấy phiên bí cảnh")
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	// Only the movement log decides; endTick/kills/score/stateHash are the client's claim, checked against the replay.
	var body secretrealm.Claim
	if !decodeJSONLimit(w, r, &body, secretrealm.MaxClaimBytes) {
		return
	}
	v, err := d.SecretRealms.Submit(r.Context(), userFrom(r.Context()), runID, key, body)
	if !d.writeSecretRealmError(w, r, "secret realm submit", err) {
		d.Metrics.SecretRealmVerdict(v.RealmID, v.Verdict, v.Replayed)
		d.markActive(r, !v.Replayed)
		writeJSON(w, 200, v)
	}
}

func (d Deps) writeSecretRealmError(w http.ResponseWriter, r *http.Request, op string, err error) bool {
	if err == nil {
		return false
	}
	var blocked *secretrealm.ErrBlocked
	switch {
	case errors.As(err, &blocked):
		msg := map[string]string{
			secretrealm.BlockDailyLimit: "Hôm nay đã dùng hết lượt vào bí cảnh",
			secretrealm.BlockLocked:     "Cảnh giới chưa đủ để vào bí cảnh này",
		}[blocked.Code]
		writeErrorDetails(w, 409, blocked.Code, msg, blocked.Status)
	case errors.Is(err, secretrealm.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case errors.Is(err, secretrealm.ErrUnknownRealm):
		writeError(w, 404, "realm_not_found", "Không có bí cảnh này")
	case errors.Is(err, secretrealm.ErrRunNotFound):
		writeError(w, 404, "run_not_found", "Không tìm thấy phiên bí cảnh")
	case errors.Is(err, secretrealm.ErrAlreadySubmitted):
		writeError(w, 409, "already_submitted", "Phiên bí cảnh này đã được phán định")
	case errors.Is(err, secretrealm.ErrKeyReused):
		writeError(w, 409, "idempotency_key_reused", "Idempotency-Key was used for another realm")
	default:
		d.internal(w, r, op, err)
	}
	return true
}

// ───── pháp bảo (Phase 3.4) ─────

func (d Deps) artifactView(w http.ResponseWriter, r *http.Request) {
	if d.Artifacts == nil {
		unavailable(w)
		return
	}
	v, err := d.Artifacts.Get(r.Context(), userFrom(r.Context()))
	if !d.writeArtifactError(w, r, "artifact view", err) {
		writeJSON(w, 200, v)
	}
}

func (d Deps) artifactEquip(w http.ResponseWriter, r *http.Request) {
	if d.Artifacts == nil {
		unavailable(w)
		return
	}
	if _, ok := idempotencyKey(w, r); !ok { // naturally idempotent (sets the slot); the key is required by convention
		return
	}
	var body struct {
		ItemID string `json:"itemId"` // "" = unequip
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	v, err := d.Artifacts.Equip(r.Context(), userFrom(r.Context()), body.ItemID)
	if !d.writeArtifactError(w, r, "artifact equip", err) {
		writeJSON(w, 200, v)
	}
}

func (d Deps) writeArtifactError(w http.ResponseWriter, r *http.Request, op string, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, artifact.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case errors.Is(err, artifact.ErrNotArtifact):
		writeError(w, 400, "not_artifact", "Vật phẩm này không phải pháp bảo")
	case errors.Is(err, artifact.ErrNotOwned):
		writeError(w, 409, "not_owned", "Chưa sở hữu pháp bảo này")
	default:
		d.internal(w, r, op, err)
	}
	return true
}

// ───── hộ pháp (Phase 3.4) ─────

func (d Deps) protectorState(w http.ResponseWriter, r *http.Request) {
	if d.Protectors == nil {
		unavailable(w)
		return
	}
	v, err := d.Protectors.State(r.Context(), userFrom(r.Context()))
	if !d.writeProtectorError(w, r, "protector state", err) {
		writeJSON(w, 200, v)
	}
}

func (d Deps) protectorInvite(w http.ResponseWriter, r *http.Request) {
	if d.Protectors == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	v, err := d.Protectors.Invite(r.Context(), userFrom(r.Context()), key)
	if !d.writeProtectorError(w, r, "protector invite", err) {
		writeJSON(w, 200, v)
	}
}

func (d Deps) protectorAccept(w http.ResponseWriter, r *http.Request) {
	if d.Protectors == nil {
		unavailable(w)
		return
	}
	id, err := uuid.Parse(r.PathValue("inviteId"))
	if err != nil {
		writeError(w, 404, "invite_gone", "Lời mời hộ pháp đã hết hạn")
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	v, err := d.Protectors.Accept(r.Context(), userFrom(r.Context()), id, key)
	if !d.writeProtectorError(w, r, "protector accept", err) {
		writeJSON(w, 200, v)
	}
}

// presencePing: POST /presence/ping — "I am online" (Redis TTL; no body, no DB write).
func (d Deps) presencePing(w http.ResponseWriter, r *http.Request) {
	if d.Protectors == nil {
		unavailable(w)
		return
	}
	online, err := d.Protectors.Ping(r.Context(), userFrom(r.Context()))
	if errors.Is(err, protector.ErrNoPresence) {
		unavailable(w)
		return
	}
	if !d.writeProtectorError(w, r, "presence ping", err) {
		writeJSON(w, 200, map[string]any{"online": online, "pingEverySeconds": d.Content.Protector.PingEverySeconds})
	}
}

func (d Deps) writeProtectorError(w http.ResponseWriter, r *http.Request, op string, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, protector.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case errors.Is(err, protector.ErrNoFaction):
		writeError(w, 409, "no_faction", "Cần gia nhập thế lực để mời hộ pháp")
	case errors.Is(err, protector.ErrNotEligible):
		writeError(w, 409, "not_eligible", "Chỉ mời hộ pháp khi đứng trước lôi kiếp")
	case errors.Is(err, protector.ErrInviteGone):
		writeError(w, 409, "invite_gone", "Lời mời hộ pháp đã hết hạn hoặc đã dùng")
	case errors.Is(err, protector.ErrOtherFaction):
		writeError(w, 403, "other_faction", "Chỉ đồng môn cùng thế lực mới hộ pháp được")
	case errors.Is(err, protector.ErrSelf):
		writeError(w, 409, "self", "Không thể tự hộ pháp cho mình")
	case errors.Is(err, protector.ErrNotReady):
		writeError(w, 409, "not_ready", "Chỉ mời hộ pháp khi lôi kiếp đã sẵn sàng (đủ tu vi, không trong thời gian hồi)")
	case errors.Is(err, protector.ErrTooLowRealm):
		writeError(w, 403, "realm_too_low", "Cần tu vi Trúc Cơ trở lên mới hộ pháp được")
	case errors.Is(err, protector.ErrTooNew):
		writeError(w, 403, "character_too_new", "Đạo hữu mới nhập thế chưa đủ ngày để hộ pháp")
	case errors.Is(err, protector.ErrDailyLimit):
		writeError(w, 409, "daily_limit", "Hôm nay đã hộ pháp đủ số lần")
	case errors.Is(err, protector.ErrKeyReused):
		writeError(w, 409, "idempotency_key_reused", "Idempotency-Key was used for another invite")
	default:
		d.internal(w, r, op, err)
	}
	return true
}

// ───── activity (hộ pháp presence, anti-cheat M2) ─────

// markActive records a REAL game action of the caller (anti-cheat M2 + đạo lữ review H1): it refreshes the activity
// window that hộ pháp presence pings and song tu require. Explicit allowlist — only these handlers call it, and only for
// a non-replayed success: settle that credited > 0, paid ngộ đạo, judged trial / bí cảnh submit, breakthrough attempt,
// shop / cosmetic buy, daily-task claim, incense, fortune draw, chat send, faction join/switch (and, once it exists,
// Phase 5.2 trade create/accept — call d.markActive there). Never: the presence ping, lecture enter/heartbeat, any
// /dao-lu/song-tu/* call, GETs, replays. TestMarkActiveAllowlist pins the list. Best effort (a Redis error is ignored).
func (d Deps) markActive(r *http.Request, real bool) {
	if !real || d.Protectors == nil || d.Protectors.Presence == nil {
		return
	}
	_ = d.Protectors.Presence.MarkActive(r.Context(), userFrom(r.Context()))
}

// adminProtectorPairs: GET /admin/protector/pairs?min=N&days=D — inviter → protector pairs for review (alts farming).
func (d Deps) adminProtectorPairs(w http.ResponseWriter, r *http.Request) {
	if d.Protectors == nil {
		unavailable(w)
		return
	}
	minAssists, days := 3, 14
	if v, err := strconv.Atoi(r.URL.Query().Get("min")); err == nil && v > 0 {
		minAssists = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v > 0 && v <= 365 {
		days = v
	}
	if d.NetSig != nil { // pairs carry the netsig sharedNetwork flag: audit the read (review L7)
		if err := d.NetSig.AuditRead(r.Context(), userFrom(r.Context()), "protector.pairs", map[string]any{"min": minAssists, "days": days}); err != nil {
			d.internal(w, r, "protector pairs audit", err)
			return
		}
	}
	pairs, err := d.Protectors.Pairs(r.Context(), minAssists, days)
	if err != nil {
		d.internal(w, r, "protector pairs", err)
		return
	}
	writeJSON(w, 200, map[string]any{"min": minAssists, "days": days, "pairs": pairs})
}
