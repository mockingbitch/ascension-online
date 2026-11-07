package httpapi

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"tiendao/game/internal/trial"
)

// ───── trials (mini games, Phase 3.1) ─────

func (d Deps) trialStatus(w http.ResponseWriter, r *http.Request) {
	if d.Trials == nil {
		unavailable(w)
		return
	}
	st, err := d.Trials.Status(r.Context(), userFrom(r.Context()))
	if !d.writeTrialError(w, r, "trial status", err) {
		writeJSON(w, 200, st)
	}
}

func (d Deps) trialStart(w http.ResponseWriter, r *http.Request) {
	if d.Trials == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	v, err := d.Trials.Start(r.Context(), userFrom(r.Context()), r.PathValue("kind"), key)
	if d.writeTrialError(w, r, "trial start", err) {
		return
	}
	status := 201
	if v.Resumed {
		status = 200
	}
	writeJSON(w, status, v)
}

func (d Deps) trialSubmit(w http.ResponseWriter, r *http.Request) {
	if d.Trials == nil {
		unavailable(w)
		return
	}
	runID, err := uuid.Parse(r.PathValue("runId"))
	if err != nil {
		writeError(w, 404, "run_not_found", "Không tìm thấy phiên thử luyện")
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	// Only the input log decides; endTick/lines/score/stateHash are the client's claim, checked against the replay.
	var body trial.Claim
	if !decodeJSONLimit(w, r, &body, trial.MaxClaimBytes) { // the loader checks max_inputs fits this limit
		return
	}
	v, err := d.Trials.Submit(r.Context(), userFrom(r.Context()), runID, key, body)
	if !d.writeTrialError(w, r, "trial submit", err) {
		d.Metrics.TrialVerdict(v.Kind, v.Verdict, v.Replayed)
		d.markActive(r, !v.Replayed)
		writeJSON(w, 200, v)
	}
}

// writeTrialError maps trial errors; false when err is nil.
func (d Deps) writeTrialError(w http.ResponseWriter, r *http.Request, op string, err error) bool {
	if err == nil {
		return false
	}
	var blocked *trial.ErrBlocked
	switch {
	case errors.As(err, &blocked):
		msg := map[string]string{
			trial.BlockCooldown:    "Tâm cảnh còn dao động, hãy tĩnh dưỡng rồi thử lại",
			trial.BlockPassActive:  "Đã vượt thử luyện này, ấn chứng vẫn còn hiệu lực",
			trial.BlockNotEligible: "Chưa tới lúc đối diện thử luyện này",
		}[blocked.Code]
		writeErrorDetails(w, 409, blocked.Code, msg, blocked.Status)
	case errors.Is(err, trial.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case errors.Is(err, trial.ErrUnknownKind):
		writeError(w, 404, "trial_not_found", "Không có thử luyện này")
	case errors.Is(err, trial.ErrRunNotFound):
		writeError(w, 404, "run_not_found", "Không tìm thấy phiên thử luyện")
	case errors.Is(err, trial.ErrAlreadySubmitted):
		writeError(w, 409, "already_submitted", "Phiên thử luyện này đã được phán định")
	case errors.Is(err, trial.ErrKeyReused):
		writeError(w, 409, "idempotency_key_reused", "Idempotency-Key was used for another trial")
	default:
		d.internal(w, r, op, err)
	}
	return true
}
