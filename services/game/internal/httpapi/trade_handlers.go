package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"tiendao/game/internal/trade"
)

// tradeState: GET /trades — rules, my limits today, incoming/outgoing pending offers.
func (d Deps) tradeState(w http.ResponseWriter, r *http.Request) {
	if d.Trades == nil {
		unavailable(w)
		return
	}
	v, err := d.Trades.State(r.Context(), userFrom(r.Context()))
	if !d.writeTradeError(w, r, "trade state", err) {
		writeJSON(w, 200, v)
	}
}

// tradeHistory: GET /trades/history — answered/lapsed offers, newest first.
func (d Deps) tradeHistory(w http.ResponseWriter, r *http.Request) {
	if d.Trades == nil {
		unavailable(w)
		return
	}
	h, err := d.Trades.History(r.Context(), userFrom(r.Context()))
	if !d.writeTradeError(w, r, "trade history", err) {
		writeJSON(w, 200, map[string]any{"offers": h})
	}
}

// tradePeer: GET /trades/peer/{characterId} — relation, the cap between us and whether they can receive.
func (d Deps) tradePeer(w http.ResponseWriter, r *http.Request) {
	if d.Trades == nil {
		unavailable(w)
		return
	}
	id, err := uuid.Parse(r.PathValue("characterId"))
	if err != nil {
		writeError(w, 404, "target_not_found", "Không tìm thấy đạo hữu này")
		return
	}
	p, err := d.Trades.Peer(r.Context(), userFrom(r.Context()), id)
	if !d.writeTradeError(w, r, "trade peer", err) {
		writeJSON(w, 200, p)
	}
}

type tradeCreateRequest struct {
	ReceiverCharacterID string       `json:"receiverCharacterId"`
	Give                []trade.Line `json:"give"`
	Take                []trade.Line `json:"take"`
}

// tradeCreate: POST /trades {receiverCharacterId, give[], take[]} + Idempotency-Key. take empty = gift.
// Lines are intent only (which asset, how many); the server prices, caps and moves everything at accept.
func (d Deps) tradeCreate(w http.ResponseWriter, r *http.Request) {
	if d.Trades == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var req tradeCreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	target, err := uuid.Parse(req.ReceiverCharacterID)
	if err != nil {
		writeError(w, 404, "target_not_found", "Không tìm thấy đạo hữu này")
		return
	}
	v, err := d.Trades.Create(r.Context(), userFrom(r.Context()), target, req.Give, req.Take, key)
	if !d.writeTradeError(w, r, "trade create", err) {
		d.markActive(r, !v.Replayed) // a real game action (activity window); never replays
		writeJSON(w, 200, v)
	}
}

type tradeAcceptRequest struct {
	AcknowledgeIrreversible bool `json:"acknowledgeIrreversible"`
}

// tradeRespond: POST /trades/{offerId}/{accept|decline|cancel} + Idempotency-Key.
func (d Deps) tradeRespond(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.Trades == nil {
			unavailable(w)
			return
		}
		key, ok := idempotencyKey(w, r)
		if !ok {
			return
		}
		id, err := uuid.Parse(r.PathValue("offerId"))
		if err != nil {
			writeError(w, 409, "offer_gone", "Lời trao đổi đã hết hạn hoặc đã được hồi đáp")
			return
		}
		ctx, user := r.Context(), userFrom(r.Context())
		var v trade.View
		switch action {
		case "accept":
			var req tradeAcceptRequest
			if !decodeJSON(w, r, &req) {
				return
			}
			v, err = d.Trades.Accept(ctx, user, id, req.AcknowledgeIrreversible, key)
		case "decline":
			v, err = d.Trades.Decline(ctx, user, id, key)
		default:
			v, err = d.Trades.Cancel(ctx, user, id, key)
		}
		if !d.writeTradeError(w, r, "trade "+action, err) {
			d.markActive(r, action == "accept" && !v.Replayed) // accept only: declines, cancels and replays don't count
			writeJSON(w, 200, v)
		}
	}
}

// adminTradeFlows: GET /admin/trades/flows?days=D — top net receivers, lopsided pairs, many-to-one funnels and offers
// between accounts on the same network. Warnings only; nothing is blocked automatically.
func (d Deps) adminTradeFlows(w http.ResponseWriter, r *http.Request) {
	if d.Trades == nil {
		unavailable(w)
		return
	}
	days := 0
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v > 0 && v <= 90 {
		days = v
	}
	if d.NetSig != nil { // the report carries netsig shared-network flags: audit the read (admin_audit_logs, review L7)
		if err := d.NetSig.AuditRead(r.Context(), userFrom(r.Context()), "trade.flows", map[string]any{"days": days}); err != nil {
			d.internal(w, r, "trade flows audit", err)
			return
		}
	}
	f, err := d.Trades.Flows(r.Context(), days)
	if err != nil {
		d.internal(w, r, "trade flows", err)
		return
	}
	writeJSON(w, 200, f)
}

var tradeLimitMessages = map[string]string{
	trade.LimitOffersPerDay: "Hôm nay đã gửi đủ số lời trao đổi",
	trade.LimitCompleted:    "Đã đạt số lần trao đổi trong ngày",
	trade.LimitReceived:     "Vượt hạn mức giá trị được nhận trong ngày",
	trade.LimitSent:         "Vượt hạn mức giá trị được tặng đi trong ngày",
}

func (d Deps) writeTradeError(w http.ResponseWriter, r *http.Request, op string, err error) bool {
	var (
		gate  *trade.GateError
		line  *trade.LineError
		limit *trade.LimitError
		lack  *trade.InsufficientError
	)
	switch {
	case err == nil:
		return false
	case errors.As(err, &line):
		msg := "Lời trao đổi không hợp lệ"
		if line.Code == "not_tradable" {
			msg = "Vật phẩm này không thể trao đổi"
		}
		writeErrorDetails(w, 400, "invalid_lines", msg, map[string]any{"reason": line.Code, "assetId": line.AssetID})
	case errors.As(err, &gate):
		msg := "Đạo hữu chưa đủ điều kiện trao đổi"
		if gate.Side == "other" {
			msg = "Đối phương chưa đủ điều kiện trao đổi"
		}
		writeErrorDetails(w, 409, "not_eligible", msg, map[string]any{"side": gate.Side, "action": gate.Action, "blockers": gate.Blockers})
	case errors.As(err, &limit):
		msg := tradeLimitMessages[limit.Code]
		if limit.Side == "other" {
			msg += " (phía đối phương)"
		}
		status := 409
		if limit.Code == trade.LimitOffersPerDay {
			status = 429
		}
		writeErrorDetails(w, status, "limit_reached", msg, map[string]any{"side": limit.Side, "limit": limit.Code, "cap": limit.Cap,
			"used": limit.Used, "requested": limit.Requested})
	case errors.As(err, &lack):
		msg := "Đạo hữu không còn đủ vật phẩm/linh thạch (kể cả phí)"
		if lack.Side == "other" {
			msg = "Người gửi không còn đủ vật phẩm/linh thạch — lời trao đổi chưa thể hoàn tất"
		}
		writeErrorDetails(w, 409, "insufficient", msg, map[string]any{"side": lack.Side, "assetId": lack.AssetID})
	case errors.Is(err, trade.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case errors.Is(err, trade.ErrTargetNotFound):
		writeError(w, 404, "target_not_found", "Không tìm thấy đạo hữu này")
	case errors.Is(err, trade.ErrSelf):
		writeError(w, 409, "self", "Không thể tự trao đổi với chính mình")
	case errors.Is(err, trade.ErrNotAcknowledged):
		writeError(w, 400, "irreversible_not_acknowledged", "Cần xác nhận: trao đổi hoàn tất thì không hoàn tác")
	case errors.Is(err, trade.ErrOfferGone):
		writeError(w, 409, "offer_gone", "Lời trao đổi đã hết hạn hoặc đã được hồi đáp")
	case errors.Is(err, trade.ErrKeyReused):
		writeError(w, 409, "idempotency_key_reused", "Idempotency-Key was used for another request")
	case errors.Is(err, trade.ErrTooManyOpen):
		writeError(w, 409, "too_many_open", "Đạo hữu đang có quá nhiều lời trao đổi chờ hồi đáp")
	default:
		d.internal(w, r, op, err)
	}
	return true
}
