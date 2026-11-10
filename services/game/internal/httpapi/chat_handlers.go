package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"tiendao/game/internal/chat"
)

// chatHistory: GET /chat/{channel}/messages?before=<id> — visible messages, oldest first.
func (d Deps) chatHistory(w http.ResponseWriter, r *http.Request) {
	if d.Chat == nil {
		unavailable(w)
		return
	}
	var before int64
	if v := r.URL.Query().Get("before"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			writeError(w, 400, "invalid_query", "before must be a message id")
			return
		}
		before = n
	}
	var hallID int
	if v := r.URL.Query().Get("hall"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, 400, "invalid_query", "hall must be a điện number ≥ 1")
			return
		}
		hallID = n
	}
	channel := r.PathValue("channel")
	msgs, err := d.Chat.History(r.Context(), channel, hallID, before)
	if errors.Is(err, chat.ErrUnknownChannel) {
		writeError(w, 404, "unknown_channel", "unknown chat channel")
		return
	} else if errors.Is(err, chat.ErrHallRequired) {
		writeError(w, 400, "hall_required", "Kênh Tử Tiêu Cung cần số điện (?hall=N)")
		return
	} else if err != nil {
		d.internal(w, r, "chat history", err)
		return
	}
	ch, _ := d.Chat.Cat.Channel(channel)
	writeJSON(w, 200, map[string]any{"channel": ch, "messages": msgs, "maxChars": d.Chat.Cat.MaxChars})
}

// chatSend: POST /chat/{channel}/messages {text} — idempotent per Idempotency-Key.
func (d Deps) chatSend(w http.ResponseWriter, r *http.Request) {
	if d.Chat == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	msg, err := d.Chat.Send(r.Context(), userFrom(r.Context()), r.PathValue("channel"), body.Text, key)
	var low *chat.StageTooLowError
	var muted *chat.MutedError
	switch {
	case err == nil:
		d.Metrics.ChatMessage(msg.Channel)
		d.markActive(r, true)
		writeJSON(w, 200, msg)
	case errors.Is(err, chat.ErrUnknownChannel):
		writeError(w, 404, "unknown_channel", "unknown chat channel")
	case errors.Is(err, chat.ErrInvalidText):
		writeError(w, 400, "invalid_text", "Tin nhắn trống hoặc quá dài")
	case errors.Is(err, chat.ErrNoCharacter):
		writeError(w, 403, "no_character", "Cần tạo nhân vật trước khi trò chuyện")
	case errors.Is(err, chat.ErrNotInHall):
		writeError(w, 409, "not_in_hall", "Hãy vào một điện của Tử Tiêu Cung để luận đạo")
	case errors.Is(err, chat.ErrUnknownUser):
		writeError(w, 403, "forbidden", "account not found")
	case errors.As(err, &low):
		stage := low.MinStage
		if d.Content != nil {
			if s, err := d.Content.Realms.Stage(low.MinStage); err == nil {
				stage = s.Name
			}
		}
		writeErrorDetails(w, 403, "stage_too_low", "Cần đạt "+stage+" mới được nói ở kênh này", map[string]string{"minStage": low.MinStage})
	case errors.As(err, &muted):
		writeErrorDetails(w, 403, "muted", "Bạn đang bị cấm nói", map[string]any{"mutedUntil": muted.Until})
	case errors.Is(err, chat.ErrDuplicate):
		writeError(w, 429, "duplicate_message", "Đừng lặp lại cùng một câu quá nhanh")
	case errors.Is(err, chat.ErrKeyReuse):
		writeError(w, 409, "idempotency_key_reused", err.Error())
	default:
		d.internal(w, r, "chat send", err)
	}
}

// adminChatHide: POST /admin/chat/messages/{messageId}/hide {reason} — audited, broadcasts chat.hidden.
func (d Deps) adminChatHide(w http.ResponseWriter, r *http.Request) {
	if d.Chat == nil {
		unavailable(w)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("messageId"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, 404, "message_not_found", "message not found")
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	switch err := d.Chat.Hide(r.Context(), userFrom(r.Context()), id, body.Reason); {
	case errors.Is(err, chat.ErrNotFound):
		writeError(w, 404, "message_not_found", "message not found")
	case err != nil:
		d.internal(w, r, "chat hide", err)
	default:
		writeJSON(w, 200, map[string]any{"id": id, "hidden": true})
	}
}

// adminChatMute: POST /admin/chat/mutes {userId, hours, reason} — hours 0 lifts the mute. Audited.
func (d Deps) adminChatMute(w http.ResponseWriter, r *http.Request) {
	if d.Chat == nil {
		unavailable(w)
		return
	}
	var body struct {
		UserID uuid.UUID `json:"userId"`
		Hours  int       `json:"hours"`
		Reason string    `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	st, err := d.Chat.Mute(r.Context(), userFrom(r.Context()), body.UserID, body.Hours, body.Reason)
	switch {
	case errors.Is(err, chat.ErrInvalidMute):
		writeError(w, 400, "invalid_mute", "hours out of range")
	case errors.Is(err, chat.ErrUnknownUser):
		writeError(w, 404, "user_not_found", "user not found")
	case err != nil:
		d.internal(w, r, "chat mute", err)
	default:
		writeJSON(w, 200, st)
	}
}
