package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"tiendao/game/internal/character"
	"tiendao/game/internal/moderation"
	"tiendao/game/internal/report"
)

// nameBlockedMessage is the player-facing text per moderation category (the matched term is never revealed).
func nameBlockedMessage(err error) (string, bool) {
	var ne *moderation.NameError
	if !errors.As(err, &ne) {
		return "", false
	}
	switch ne.Category {
	case moderation.CategoryImpersonation:
		return "Đạo hiệu không được giả danh ban quản trị hoặc hệ thống", true
	case moderation.CategoryStory:
		return "Đạo hiệu trùng với nhân vật trong cốt truyện, hãy chọn tên khác", true
	case moderation.CategoryReligion:
		return "Đạo hiệu không được dùng danh xưng của nhân vật tín ngưỡng", true
	default:
		return "Đạo hiệu chứa từ ngữ không phù hợp", true
	}
}

// submitReport: POST /reports {targetCharacterId, reason, note} — idempotent per Idempotency-Key.
func (d Deps) submitReport(w http.ResponseWriter, r *http.Request) {
	if d.Reports == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		TargetCharacterID uuid.UUID `json:"targetCharacterId"`
		Reason            string    `json:"reason"`
		Note              string    `json:"note"`
		ChatMessageID     int64     `json:"chatMessageId"`
		TradeOfferID      uuid.UUID `json:"tradeOfferId"` // reason trade_abuse: the counterpart is derived from the offer
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	var rep report.Report
	var err error
	if body.TradeOfferID != uuid.Nil || body.Reason == report.ReasonTradeAbuse {
		rep, err = d.Reports.SubmitTrade(r.Context(), userFrom(r.Context()), body.TradeOfferID, body.Note, key)
	} else {
		rep, err = d.Reports.Submit(r.Context(), userFrom(r.Context()), body.TargetCharacterID, body.Reason, body.Note, key, body.ChatMessageID)
	}
	switch {
	case errors.Is(err, report.ErrInvalidReport):
		writeError(w, 400, "invalid_report", "Lý do không hợp lệ hoặc ghi chú quá dài")
	case errors.Is(err, report.ErrSelfReport):
		writeError(w, 400, "self_report", "Không thể tự báo cáo chính mình")
	case errors.Is(err, report.ErrTargetNotFound):
		writeError(w, 404, "target_not_found", "Không tìm thấy đạo hữu này")
	case errors.Is(err, report.ErrUnknownReporter):
		writeError(w, 403, "forbidden", "account not found")
	case errors.Is(err, report.ErrKeyReuse):
		writeError(w, 409, "idempotency_key_reused", err.Error())
	case errors.Is(err, report.ErrDailyLimit):
		writeError(w, 429, "report_limit", "Hôm nay bạn đã gửi đủ số báo cáo cho phép")
	case err != nil:
		d.internal(w, r, "submit report", err)
	default:
		writeJSON(w, 200, rep)
	}
}

// adminListReports: GET /admin/reports?status=open&limit=50&offset=0
func (d Deps) adminListReports(w http.ResponseWriter, r *http.Request) {
	if d.Reports == nil {
		unavailable(w)
		return
	}
	qs := r.URL.Query()
	status := qs.Get("status")
	if status == "" {
		status = report.StatusOpen
	}
	limit, offset := 50, 0
	if v := qs.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			writeError(w, 400, "invalid_query", "limit must be 1..100")
			return
		}
		limit = n
	}
	if v := qs.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 100_000 {
			writeError(w, 400, "invalid_query", "offset must be 0..100000")
			return
		}
		offset = n
	}
	list, err := d.Reports.List(r.Context(), status, int32(limit), int32(offset))
	if errors.Is(err, report.ErrInvalidResolution) {
		writeError(w, 400, "invalid_query", "status must be open|dismissed|actioned")
		return
	} else if err != nil {
		d.internal(w, r, "list reports", err)
		return
	}
	writeJSON(w, 200, map[string]any{"reports": list, "limit": limit, "offset": offset})
}

// adminResolveReport: POST /admin/reports/{reportId}/resolve {outcome, note, renameTo?} — audited.
func (d Deps) adminResolveReport(w http.ResponseWriter, r *http.Request) {
	if d.Reports == nil {
		unavailable(w)
		return
	}
	id, err := uuid.Parse(r.PathValue("reportId"))
	if err != nil {
		writeError(w, 404, "report_not_found", "report not found")
		return
	}
	var body struct {
		Outcome  string `json:"outcome"`
		Note     string `json:"note"`
		RenameTo string `json:"renameTo"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := d.Reports.Resolve(r.Context(), userFrom(r.Context()), id, body.Outcome, body.Note, body.RenameTo)
	if msg, ok := nameBlockedMessage(err); ok {
		writeError(w, 400, "name_not_allowed", msg)
		return
	}
	switch {
	case errors.Is(err, report.ErrInvalidResolution):
		writeError(w, 400, "invalid_resolution", "outcome must be dismissed|actioned; renameTo only with actioned; note ≤ 1000")
	case errors.Is(err, character.ErrInvalidName):
		writeError(w, 400, "invalid_name", "Tên 2–24 chữ cái, chỉ dùng chữ (có dấu) và khoảng trắng")
	case errors.Is(err, character.ErrNameTaken):
		writeError(w, 409, "name_taken", "Đạo hiệu này đã có người dùng")
	case errors.Is(err, character.ErrNotFound):
		writeError(w, 404, "target_not_found", "reported character no longer exists")
	case errors.Is(err, report.ErrReportNotFound):
		writeError(w, 404, "report_not_found", "report not found")
	case errors.Is(err, report.ErrAlreadyResolved):
		writeError(w, 409, "already_resolved", "report already resolved")
	case err != nil:
		d.internal(w, r, "resolve report", err)
	default:
		writeJSON(w, 200, res)
	}
}
