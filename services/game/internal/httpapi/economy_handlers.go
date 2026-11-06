package httpapi

import (
	"errors"
	"net/http"

	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/db"
	"tiendao/game/internal/vip"
)

// inventory: GET /inventory → wallet + items of the caller's character.
func (d Deps) inventory(w http.ResponseWriter, r *http.Request) {
	if d.Pool == nil {
		unavailable(w)
		return
	}
	q := db.New(d.Pool)
	ch, err := q.GetCharacterByUser(r.Context(), userFrom(r.Context()))
	if err != nil {
		writeError(w, 404, "character_not_found", "create a character first")
		return
	}
	cur, err := q.ListCurrencyBalances(r.Context(), ch.ID)
	if err != nil {
		d.internal(w, r, "inventory", err)
		return
	}
	items, err := q.ListInventory(r.Context(), ch.ID)
	if err != nil {
		d.internal(w, r, "inventory", err)
		return
	}
	wallet := map[string]int64{}
	for _, c := range cur {
		wallet[c.Currency] = c.Balance
	}
	list := make([]map[string]any, 0, len(items))
	for _, it := range items {
		list = append(list, map[string]any{"itemId": it.ItemID, "quantity": it.Quantity})
	}
	writeJSON(w, 200, map[string]any{"currencies": wallet, "items": list})
}

// useItem: POST /items/{itemId}/use — currently offline-bonus items (Tụ Linh Phù).
func (d Deps) useItem(w http.ResponseWriter, r *http.Request) {
	if d.Cultivation == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	st, err := d.Cultivation.UseOfflineBonusItem(r.Context(), d.Content.Items, userFrom(r.Context()), r.PathValue("itemId"), key)
	switch {
	case errors.Is(err, cultivation.ErrNotUsable):
		writeError(w, 400, "not_usable", "vật phẩm này không thể sử dụng")
	case errors.Is(err, cultivation.ErrNoItem):
		writeError(w, 409, "no_item", "không có vật phẩm trong túi")
	case errors.Is(err, cultivation.ErrBonusActive):
		writeError(w, 409, "bonus_active", "linh khí tụ linh vẫn còn hiệu lực, chưa thể dùng thêm")
	case errors.Is(err, cultivation.ErrBonusUsedToday):
		writeError(w, 409, "used_today", "hôm nay đã dùng Tụ Linh Phù")
	case errors.Is(err, cultivation.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case err != nil:
		d.internal(w, r, "use item", err)
	default:
		writeJSON(w, 200, st)
	}
}

// claimVIPGift: POST /vip/daily-gift — once per game day (DB primary key), no Idempotency-Key needed.
func (d Deps) claimVIPGift(w http.ResponseWriter, r *http.Request) {
	if d.VIP == nil {
		unavailable(w)
		return
	}
	res, err := d.VIP.ClaimDailyGift(r.Context(), userFrom(r.Context()), d.Now())
	switch {
	case errors.Is(err, vip.ErrNoGift):
		writeError(w, 409, "no_gift", "cấp VIP hiện tại chưa có quà ngày")
	case errors.Is(err, vip.ErrGiftClaimed):
		writeError(w, 409, "already_claimed", "hôm nay đã nhận quà")
	case errors.Is(err, vip.ErrGiftNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case err != nil:
		d.internal(w, r, "vip gift", err)
	default:
		writeJSON(w, 200, res)
	}
}

func (d Deps) dailyTasks(w http.ResponseWriter, r *http.Request) {
	if d.DailyTasks == nil {
		unavailable(w)
		return
	}
	b, err := d.DailyTasks.Board(r.Context(), userFrom(r.Context()))
	if errors.Is(err, dailytask.ErrNoCharacter) {
		writeError(w, 404, "character_not_found", "create a character first")
		return
	} else if err != nil {
		d.internal(w, r, "daily tasks", err)
		return
	}
	writeJSON(w, 200, b)
}

// claimDailyTask: POST /tasks/daily/{taskId}/claim — once per task per game day (row lock + claimed_at).
func (d Deps) claimDailyTask(w http.ResponseWriter, r *http.Request) {
	if d.DailyTasks == nil {
		unavailable(w)
		return
	}
	v, err := d.DailyTasks.Claim(r.Context(), userFrom(r.Context()), r.PathValue("taskId"))
	switch {
	case errors.Is(err, dailytask.ErrUnknownTask):
		writeError(w, 404, "unknown_task", "unknown task")
	case errors.Is(err, dailytask.ErrNotComplete):
		writeError(w, 409, "not_complete", "nhiệm vụ chưa hoàn thành")
	case errors.Is(err, dailytask.ErrAlreadyClaimed):
		writeError(w, 409, "already_claimed", "đã nhận thưởng hôm nay")
	case errors.Is(err, dailytask.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case err != nil:
		d.internal(w, r, "claim task", err)
	default:
		writeJSON(w, 200, v)
	}
}
