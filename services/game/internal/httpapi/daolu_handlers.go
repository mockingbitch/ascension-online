package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"tiendao/game/internal/daolu/bond"
)

// Song tu calls (start/heartbeat/claim) are NOT game actions: none of them calls markActive (review H1), otherwise a
// heartbeat script alone would keep both partners "active".
const songTuHeartbeatPath = "/api/v1/dao-lu/song-tu/heartbeat"

// daoLuState: GET /dao-lu — rules, gates, my bond (+ partner presence, song tu), incoming/outgoing proposals.
func (d Deps) daoLuState(w http.ResponseWriter, r *http.Request) {
	if d.DaoLu == nil {
		unavailable(w)
		return
	}
	v, err := d.DaoLu.State(r.Context(), userFrom(r.Context()))
	if !d.writeDaoLuError(w, r, "dao lu state", err) {
		writeJSON(w, 200, v)
	}
}

type daoLuProposeRequest struct {
	TargetCharacterID    string `json:"targetCharacterId"`
	AcknowledgePermanent bool   `json:"acknowledgePermanent"`
}

// daoLuPropose: POST /dao-lu/proposals {targetCharacterId, acknowledgePermanent} + Idempotency-Key.
func (d Deps) daoLuPropose(w http.ResponseWriter, r *http.Request) {
	if d.DaoLu == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var req daoLuProposeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	target, err := uuid.Parse(req.TargetCharacterID)
	if err != nil {
		writeError(w, 404, "target_not_found", "Không tìm thấy đạo hữu này")
		return
	}
	v, err := d.DaoLu.Propose(r.Context(), userFrom(r.Context()), target, req.AcknowledgePermanent, key)
	if !d.writeDaoLuError(w, r, "dao lu propose", err) {
		writeJSON(w, 200, v)
	}
}

type daoLuAcceptRequest struct {
	AcknowledgePermanent bool `json:"acknowledgePermanent"`
}

// daoLuRespond: POST /dao-lu/proposals/{proposalId}/{accept|decline|cancel} + Idempotency-Key.
func (d Deps) daoLuRespond(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.DaoLu == nil {
			unavailable(w)
			return
		}
		key, ok := idempotencyKey(w, r)
		if !ok {
			return
		}
		id, err := uuid.Parse(r.PathValue("proposalId"))
		if err != nil {
			writeError(w, 409, "proposal_gone", "Lời cầu thân đã hết hạn hoặc đã được hồi đáp")
			return
		}
		ctx, user := r.Context(), userFrom(r.Context())
		var v bond.View
		switch action {
		case "accept":
			var req daoLuAcceptRequest
			if !decodeJSON(w, r, &req) {
				return
			}
			v, err = d.DaoLu.Accept(ctx, user, id, req.AcknowledgePermanent, key)
		case "decline":
			v, err = d.DaoLu.Decline(ctx, user, id, key)
		default:
			v, err = d.DaoLu.Cancel(ctx, user, id, key)
		}
		if !d.writeDaoLuError(w, r, "dao lu "+action, err) {
			writeJSON(w, 200, v)
		}
	}
}

// daoLuSongTu: GET /dao-lu/song-tu and POST /dao-lu/song-tu/{start|heartbeat}.
func (d Deps) daoLuSongTu(op string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.DaoLu == nil {
			unavailable(w)
			return
		}
		ctx, user := r.Context(), userFrom(r.Context())
		var v bond.SongTuView
		var err error
		switch op {
		case "start":
			v, err = d.DaoLu.StartSongTu(ctx, user)
		case "heartbeat":
			v, err = d.DaoLu.HeartbeatSongTu(ctx, user)
		default:
			v, err = d.DaoLu.SongTuState(ctx, user)
		}
		if !d.writeDaoLuError(w, r, "song tu "+op, err) {
			writeJSON(w, 200, v)
		}
	}
}

// daoLuClaim: POST /dao-lu/song-tu/claim + Idempotency-Key — pays my unclaimed overlap.
func (d Deps) daoLuClaim(w http.ResponseWriter, r *http.Request) {
	if d.DaoLu == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	res, err := d.DaoLu.ClaimSongTu(r.Context(), userFrom(r.Context()), key)
	if !d.writeDaoLuError(w, r, "song tu claim", err) {
		writeJSON(w, 200, res)
	}
}

type adminDaoLuTerminateRequest struct {
	ReportID string `json:"reportId"`
	Note     string `json:"note"`
}

// adminDaoLuTerminate: POST /admin/dao-lu/bonds/{bondId}/terminate {reportId, note} — audited.
func (d Deps) adminDaoLuTerminate(w http.ResponseWriter, r *http.Request) {
	if d.DaoLu == nil {
		unavailable(w)
		return
	}
	bondID, err := uuid.Parse(r.PathValue("bondId"))
	if err != nil {
		writeError(w, 404, "no_bond", "bond not found")
		return
	}
	var req adminDaoLuTerminateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	reportID, err := uuid.Parse(req.ReportID)
	if err != nil {
		writeError(w, 400, "report_required", "reportId of a report by or about one of the partners is required")
		return
	}
	err = d.DaoLu.AdminEnd(r.Context(), userFrom(r.Context()), bondID, reportID, req.Note)
	if !d.writeDaoLuError(w, r, "dao lu terminate", err) {
		writeJSON(w, 200, map[string]any{"bondId": bondID, "ended": true, "reason": bond.ReasonAdmin})
	}
}

// adminDaoLuFlags: GET /admin/dao-lu/flags?days=D — proposals/bonds between accounts that shared a network.
func (d Deps) adminDaoLuFlags(w http.ResponseWriter, r *http.Request) {
	if d.DaoLu == nil {
		unavailable(w)
		return
	}
	days := 30
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v > 0 && v <= 365 {
		days = v
	}
	flags, err := d.DaoLu.Flags(r.Context(), userFrom(r.Context()), days) // audited read (review L7)
	if err != nil {
		d.internal(w, r, "dao lu flags", err)
		return
	}
	writeJSON(w, 200, map[string]any{"days": days, "flags": flags})
}

// adminNetClusters: GET /admin/netsig/clusters?days=&min= — account sets seen on the same network (salted IP hash)
// within the retention window. No hash or IP is ever returned.
func (d Deps) adminNetClusters(w http.ResponseWriter, r *http.Request) {
	if d.NetSig == nil {
		unavailable(w)
		return
	}
	days, minAccounts := 30, 2
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v > 0 {
		days = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("min")); err == nil && v >= 2 && v <= 50 {
		minAccounts = v
	}
	if err := d.NetSig.AuditRead(r.Context(), userFrom(r.Context()), "netsig.clusters", map[string]any{"days": days, "min": minAccounts}); err != nil {
		d.internal(w, r, "net clusters audit", err)
		return
	}
	cs, err := d.NetSig.Clusters(r.Context(), days, minAccounts, 100)
	if err != nil {
		d.internal(w, r, "net clusters", err)
		return
	}
	writeJSON(w, 200, map[string]any{"days": min(days, 30), "min": minAccounts, "clusters": cs})
}

// recordNet stores the salted network hash of the request's client IP for the user (Google login, session refresh).
// Best effort: a failure is logged and never breaks the sign-in.
func (d Deps) recordNet(r *http.Request, userID uuid.UUID) {
	if d.NetSig == nil {
		return
	}
	if err := d.NetSig.Record(context.WithoutCancel(r.Context()), userID, clientIP(r, d.proxies())); err != nil {
		d.Log.WarnContext(r.Context(), "net signal record", "err", err)
	}
}

func (d Deps) writeDaoLuError(w http.ResponseWriter, r *http.Request, op string, err error) bool {
	var gate *bond.GateError
	var pg *pgconn.PgError
	switch {
	case err == nil:
		return false
	case errors.As(err, &gate) && gate.Side == "target":
		// Review L5: never reveal WHY another player can't bond (account age, realm, cooldown, inbox…).
		writeErrorDetails(w, 409, "not_eligible", "Đối phương hiện chưa thể nhận lời cầu thân", map[string]any{"side": "target"})
	case errors.As(err, &gate):
		writeErrorDetails(w, 409, "not_eligible", "Đạo hữu chưa đủ điều kiện kết đạo lữ", map[string]any{"side": gate.Side, "blockers": gate.Blockers})
	case errors.As(err, &pg) && (pg.Code == "40P01" || pg.Code == "40001"):
		writeError(w, 409, "try_again", "Hệ thống đang bận, thử lại sau giây lát") // deadlock / serialization (review L1)
	case errors.Is(err, bond.ErrDeclinedRecently):
		writeError(w, 409, "declined_recently", "Đối phương vừa từ chối — hãy để thời gian trôi qua")
	case errors.Is(err, bond.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case errors.Is(err, bond.ErrTargetNotFound):
		writeError(w, 404, "target_not_found", "Không tìm thấy đạo hữu này")
	case errors.Is(err, bond.ErrSelf):
		writeError(w, 409, "self", "Không thể tự cầu thân chính mình")
	case errors.Is(err, bond.ErrNotAcknowledged):
		writeError(w, 400, "permanent_not_acknowledged", "Cần xác nhận đã đọc cảnh báo: đạo lữ là ràng buộc vĩnh viễn")
	case errors.Is(err, bond.ErrOpenProposal):
		writeError(w, 409, "open_proposal", "Đạo hữu đang có một lời cầu thân chờ hồi đáp")
	case errors.Is(err, bond.ErrDailyLimit):
		writeError(w, 429, "daily_limit", "Hôm nay đã gửi đủ số lời cầu thân")
	case errors.Is(err, bond.ErrProposalGone):
		writeError(w, 409, "proposal_gone", "Lời cầu thân đã hết hạn hoặc đã được hồi đáp")
	case errors.Is(err, bond.ErrNotYourProposal):
		writeError(w, 404, "proposal_gone", "Lời cầu thân đã hết hạn hoặc đã được hồi đáp")
	case errors.Is(err, bond.ErrKeyReused):
		writeError(w, 409, "idempotency_key_reused", "Idempotency-Key was used for another request")
	case errors.Is(err, bond.ErrAlreadyBonded):
		writeError(w, 409, "already_bonded", "Một trong hai người vừa kết đạo lữ với người khác")
	case errors.Is(err, bond.ErrNoBond):
		writeError(w, 404, "no_bond", "Chưa có đạo lữ")
	case errors.Is(err, bond.ErrBondEnded):
		writeError(w, 409, "bond_ended", "Ràng buộc đạo lữ đã chấm dứt")
	case errors.Is(err, bond.ErrReportMismatch):
		writeError(w, 400, "report_mismatch", "the report must be by or about one of the partners")
	case errors.Is(err, bond.ErrNoteRequired):
		writeError(w, 400, "note_required", "a note (≤ 1000 chars) is required")
	case errors.Is(err, bond.ErrSongTuOff):
		unavailable(w)
	case errors.Is(err, bond.ErrNotActive):
		writeError(w, 409, "not_active", "Hãy tu luyện (thao tác trong game) trước khi song tu")
	case errors.Is(err, bond.ErrPartnerNotActive):
		writeError(w, 409, "partner_not_active", "Đạo lữ chưa có mặt — cả hai cần cùng đang tu luyện")
	case errors.Is(err, bond.ErrNoSession):
		writeError(w, 409, "no_session", "Chưa khai mở song tu hôm nay")
	case errors.Is(err, bond.ErrNothingToClaim):
		writeError(w, 409, "nothing_to_claim", "Chưa có thời gian song tu nào để nhận")
	case errors.Is(err, bond.ErrSongTuDayCapped):
		writeError(w, 409, "day_capped", "Hôm nay đã nhận đủ thời gian song tu")
	default:
		d.internal(w, r, op, err)
	}
	return true
}
