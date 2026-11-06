package httpapi

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"tiendao/game/internal/inventory"
	"tiendao/game/internal/vip"
)

// adminVIPGrant: POST /admin/vip/grant {userId, delta, reason} — audited, idempotent.
func (d Deps) adminVIPGrant(w http.ResponseWriter, r *http.Request) {
	if d.VIP == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		UserID uuid.UUID `json:"userId"`
		Delta  int64     `json:"delta"`
		Note   string    `json:"note"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	reason := "admin_grant"
	if body.Delta < 0 {
		reason = "admin_revoke"
	}
	admin := userFrom(r.Context())
	st, err := d.VIP.Grant(r.Context(), &admin, body.UserID, body.Delta, reason, "admin_note", body.Note, key)
	if errors.Is(err, vip.ErrInvalidGrant) {
		writeError(w, 400, "invalid_grant", err.Error())
		return
	} else if errors.Is(err, vip.ErrKeyReuse) {
		writeError(w, 409, "idempotency_key_reused", err.Error())
		return
	} else if err != nil {
		d.internal(w, r, "vip grant", err)
		return
	}
	writeJSON(w, 200, st)
}

// adminInventoryGrant: POST /admin/inventory/grant {characterId, itemId, quantity, note} — audited, idempotent.
func (d Deps) adminInventoryGrant(w http.ResponseWriter, r *http.Request) {
	if d.Pool == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		CharacterID uuid.UUID `json:"characterId"`
		ItemID      string    `json:"itemId"`
		Quantity    int64     `json:"quantity"`
		Note        string    `json:"note"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if _, ok := d.Content.Items.Get(body.ItemID); !ok || body.Quantity <= 0 || body.Quantity > 1_000_000 {
		writeError(w, 400, "invalid_grant", "unknown item or quantity out of range")
		return
	}
	admin := userFrom(r.Context())
	qty, err := inventory.AdminGrant(r.Context(), d.Pool, admin, body.CharacterID, body.ItemID, body.Quantity, body.Note, "admin:"+key)
	if err != nil {
		d.internal(w, r, "inventory grant", err)
		return
	}
	writeJSON(w, 200, map[string]any{"itemId": body.ItemID, "quantity": qty})
}

// adminWorldStart: POST /admin/world/start — sets server day 0 once (later calls return the same state). Audited.
func (d Deps) adminWorldStart(w http.ResponseWriter, r *http.Request) {
	if d.World == nil {
		unavailable(w)
		return
	}
	st, err := d.World.Start(r.Context(), userFrom(r.Context()))
	if err != nil {
		d.internal(w, r, "world start", err)
		return
	}
	writeJSON(w, 200, st)
}
