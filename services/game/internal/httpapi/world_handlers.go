package httpapi

import "net/http"

// getWorld: GET /world — public story clock (era, progress toward the next milestone, its window/estimate, active
// events, timeline). Same for everyone, so a short shared cache is fine.
func (d Deps) getWorld(w http.ResponseWriter, r *http.Request) {
	if d.World == nil {
		unavailable(w)
		return
	}
	v, err := d.World.View(r.Context())
	if err != nil {
		d.internal(w, r, "world view", err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=15")
	writeJSON(w, 200, v)
}
