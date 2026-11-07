package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"tiendao/game/internal/inventory"
	"tiendao/game/internal/vip"
	"tiendao/game/internal/world"
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

// adminVIPRefund: POST /admin/vip/refund {ledgerId, reason} — removes exactly the điểm duyên of one top-up purchase
// row (decision 10); the Tiên Duyên level may drop. Audited, idempotent (Idempotency-Key), at most once per purchase.
func (d Deps) adminVIPRefund(w http.ResponseWriter, r *http.Request) {
	if d.VIP == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		LedgerID int64  `json:"ledgerId"`
		Reason   string `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.LedgerID <= 0 || strings.TrimSpace(body.Reason) == "" || len(body.Reason) > 500 {
		writeError(w, 400, "invalid_refund", "ledgerId and a reason (≤ 500 chars) are required")
		return
	}
	res, st, err := d.VIP.Refund(r.Context(), userFrom(r.Context()), body.LedgerID, body.Reason, key)
	switch {
	case errors.Is(err, vip.ErrLedgerNotFound):
		writeError(w, 404, "ledger_not_found", err.Error())
	case errors.Is(err, vip.ErrNotRefundable):
		writeError(w, 409, "not_refundable", err.Error())
	case errors.Is(err, vip.ErrAlreadyRefunded):
		writeError(w, 409, "already_refunded", err.Error())
	case errors.Is(err, vip.ErrKeyReuse):
		writeError(w, 409, "idempotency_key_reused", err.Error())
	case errors.Is(err, vip.ErrInvalidGrant):
		writeError(w, 409, "insufficient_points", err.Error())
	case err != nil:
		d.internal(w, r, "vip refund", err)
	default:
		writeJSON(w, 200, map[string]any{"refund": res, "status": st})
	}
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

// adminWorld: GET /admin/world — progress, last 14 day totals (DAU, per-capita, median), overrides, reached, events.
func (d Deps) adminWorld(w http.ResponseWriter, r *http.Request) {
	if d.World == nil {
		unavailable(w)
		return
	}
	v, err := d.World.AdminView(r.Context())
	if err != nil {
		d.internal(w, r, "admin world", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, v)
}

// adminWorldOverride: PUT /admin/world/milestones/{milestoneId} {thresholdPointsPerActive?, targetDay?, forceReach,
// paused, reason} — replaces the override of an unreached milestone (shift / force / hold). Audited; applied by the
// next worker tick.
func (d Deps) adminWorldOverride(w http.ResponseWriter, r *http.Request) {
	if d.World == nil {
		unavailable(w)
		return
	}
	var body world.OverrideInput
	if !decodeJSON(w, r, &body) {
		return
	}
	ov, err := d.World.SetOverride(r.Context(), userFrom(r.Context()), r.PathValue("milestoneId"), body)
	switch {
	case errors.Is(err, world.ErrUnknownMilestone):
		writeError(w, 404, "unknown_milestone", err.Error())
	case errors.Is(err, world.ErrMilestoneReached):
		writeError(w, 409, "milestone_reached", err.Error())
	case errors.Is(err, world.ErrInvalidOverride):
		writeError(w, 400, "invalid_override", err.Error())
	case err != nil:
		d.internal(w, r, "world override", err)
	default:
		writeJSON(w, 200, ov)
	}
}

// adminWorldPause: POST /admin/world/pause | /resume {reason} — holds or releases milestone reaching. Audited.
func (d Deps) adminWorldPause(paused bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.World == nil {
			unavailable(w)
			return
		}
		var body struct {
			Reason string `json:"reason"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		st, err := d.World.SetPaused(r.Context(), userFrom(r.Context()), paused, body.Reason)
		if errors.Is(err, world.ErrInvalidOverride) {
			writeError(w, 400, "invalid_request", err.Error())
			return
		} else if err != nil {
			d.internal(w, r, "world pause", err)
			return
		}
		writeJSON(w, 200, st)
	}
}
