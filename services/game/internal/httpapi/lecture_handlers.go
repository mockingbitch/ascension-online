package httpapi

import (
	"errors"
	"net/http"

	"tiendao/game/internal/lecture/hall"
)

// lectureScripts: GET /lecture/scripts — the lecture loop (public content; the client plays lines on the server clock).
func (d Deps) lectureScripts(w http.ResponseWriter, _ *http.Request) {
	if d.Lectures == nil || d.Lectures.Scripts == nil {
		unavailable(w)
		return
	}
	sc := d.Lectures.Scripts
	writeJSON(w, 200, map[string]any{"lectures": sc.List, "cycleSeconds": int64(sc.Total.Seconds()), "cycleEpoch": d.Lectures.Cfg.Epoch.UTC()})
}

// lectureState: GET /lecture/state — hall, ngộ đạo timing and the viewer's own accrual.
func (d Deps) lectureState(w http.ResponseWriter, r *http.Request) {
	if d.Lectures == nil {
		unavailable(w)
		return
	}
	st, err := d.Lectures.State(r.Context(), userFrom(r.Context()))
	d.lectureReply(w, r, "lecture state", st, err)
}

// lectureEnter: POST /lecture/enter — take (or keep) a seat. Idempotent by nature (no amounts involved).
func (d Deps) lectureEnter(w http.ResponseWriter, r *http.Request) {
	if d.Lectures == nil {
		unavailable(w)
		return
	}
	st, err := d.Lectures.Enter(r.Context(), userFrom(r.Context()))
	d.lectureReply(w, r, "lecture enter", st, err)
}

// lectureLeave: POST /lecture/leave.
func (d Deps) lectureLeave(w http.ResponseWriter, r *http.Request) {
	if d.Lectures == nil {
		unavailable(w)
		return
	}
	err := d.Lectures.Leave(r.Context(), userFrom(r.Context()))
	d.lectureReply(w, r, "lecture leave", map[string]bool{"inHall": false}, err)
}

// lectureBeat: POST /lecture/heartbeat — presence; listening time is measured on the server clock.
func (d Deps) lectureBeat(w http.ResponseWriter, r *http.Request) {
	if d.Lectures == nil {
		unavailable(w)
		return
	}
	me, err := d.Lectures.Beat(r.Context(), userFrom(r.Context()))
	d.lectureReply(w, r, "lecture heartbeat", me, err)
}

// lectureInsight: POST /lecture/insight — ngộ đạo: pays the accrued listening once per slot (the slot is the
// idempotency guard; the Idempotency-Key header is still required for client retries).
func (d Deps) lectureInsight(w http.ResponseWriter, r *http.Request) {
	if d.Lectures == nil {
		unavailable(w)
		return
	}
	if _, ok := idempotencyKey(w, r); !ok {
		return
	}
	res, err := d.Lectures.Insight(r.Context(), userFrom(r.Context()))
	if err == nil {
		d.Metrics.NgoDao(res.EarnedMilli, res.Replayed)
		d.markActive(r, !res.Replayed) // a paid ngộ đạo (enter/heartbeat never count)
	}
	d.lectureReply(w, r, "lecture insight", res, err)
}

// lectureNotebook: GET /lecture/notebook — the Đạo ngộ notebook.
func (d Deps) lectureNotebook(w http.ResponseWriter, r *http.Request) {
	if d.Lectures == nil {
		unavailable(w)
		return
	}
	notes, err := d.Lectures.Notebook(r.Context(), userFrom(r.Context()))
	d.lectureReply(w, r, "lecture notebook", map[string]any{"entries": notes}, err)
}

func (d Deps) lectureReply(w http.ResponseWriter, r *http.Request, op string, v any, err error) {
	switch {
	case err == nil:
		writeJSON(w, 200, v)
	case errors.Is(err, hall.ErrNoCharacter):
		writeError(w, 404, "no_character", "Cần tạo nhân vật trước")
	case errors.Is(err, hall.ErrNotPresent):
		writeError(w, 409, "not_in_hall", "Bạn chưa ngồi trong Tử Tiêu Cung")
	case errors.Is(err, hall.ErrInsightClosed):
		writeError(w, 409, "insight_closed", "Chưa tới lúc ngộ đạo")
	case errors.Is(err, hall.ErrNothingToPay):
		writeError(w, 409, "nothing_to_pay", "Hãy nghe giảng thêm rồi ngộ đạo")
	default:
		d.internal(w, r, op, err)
	}
}
