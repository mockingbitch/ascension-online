package httpapi

import (
	"errors"
	"net/http"

	"tiendao/game/internal/tutorial"
)

// tutorialScript: GET /tutorial/script — Vân Khê's lines (public, immutable; onboarding runs before a character exists).
func (d Deps) tutorialScript(w http.ResponseWriter, _ *http.Request) {
	if d.Tutorial == nil {
		unavailable(w)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	writeJSON(w, 200, d.Tutorial.Script)
}

func (d Deps) tutorialError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, tutorial.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case errors.Is(err, tutorial.ErrUnknownStep):
		writeError(w, 404, "unknown_step", "bước hướng dẫn không tồn tại")
	case errors.Is(err, tutorial.ErrBadStatus):
		writeError(w, 400, "invalid_body", "status must be done or skipped")
	case errors.Is(err, tutorial.ErrKeyReuse):
		writeError(w, 409, "idempotency_key_reused", "Idempotency-Key đã dùng cho một bước khác")
	default:
		d.internal(w, r, op, err)
	}
}

// tutorialState: GET /tutorial — the caller's seen/skipped steps and the "không nhắc lại" flag.
func (d Deps) tutorialState(w http.ResponseWriter, r *http.Request) {
	if d.Tutorial == nil {
		unavailable(w)
		return
	}
	st, err := d.Tutorial.Get(r.Context(), userFrom(r.Context()))
	if err != nil {
		d.tutorialError(w, r, "tutorial state", err)
		return
	}
	writeJSON(w, 200, st)
}

// tutorialMark: POST /tutorial/steps/{stepId}/done|skip + Idempotency-Key (no body). Display progress only.
func (d Deps) tutorialMark(status string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if d.Tutorial == nil {
			unavailable(w)
			return
		}
		key, ok := idempotencyKey(w, r)
		if !ok {
			return
		}
		res, err := d.Tutorial.Mark(r.Context(), userFrom(r.Context()), r.PathValue("stepId"), status, key)
		if err != nil {
			d.tutorialError(w, r, "tutorial mark", err)
			return
		}
		writeJSON(w, 200, res)
	})
}

// tutorialSkipAll: POST /tutorial/skip-all + Idempotency-Key — "không nhắc lại": the guide offers no step on her own
// any more (the "?" button still summons her). Same key as the last settings action = replay.
func (d Deps) tutorialSkipAll(w http.ResponseWriter, r *http.Request) {
	if d.Tutorial == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	st, err := d.Tutorial.SetMuted(r.Context(), userFrom(r.Context()), true, key)
	if err != nil {
		d.tutorialError(w, r, "tutorial skip-all", err)
		return
	}
	writeJSON(w, 200, st)
}

// tutorialReset: POST /tutorial/reset + Idempotency-Key — forget seen steps and un-mute (Cài đặt › Xem lại hướng dẫn).
func (d Deps) tutorialReset(w http.ResponseWriter, r *http.Request) {
	if d.Tutorial == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	st, err := d.Tutorial.Reset(r.Context(), userFrom(r.Context()), key)
	if err != nil {
		d.tutorialError(w, r, "tutorial reset", err)
		return
	}
	writeJSON(w, 200, st)
}
