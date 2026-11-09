package httpapi

import (
	"net/http"
	"strconv"
)

// announcementsRecent: GET /announcements/recent?limit=N — the world's recent news (public, short history kept by the
// worker; đạo hiệu + realm/rank only). Clients merge it with the live WebSocket feed by id.
func (d Deps) announcementsRecent(w http.ResponseWriter, r *http.Request) {
	if d.Announce == nil {
		unavailable(w)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := d.Announce.Recent(r.Context(), int32(limit))
	if err != nil {
		d.internal(w, r, "recent announcements", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, map[string]any{"items": items})
}
