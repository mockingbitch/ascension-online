package httpapi

import (
	"errors"
	"net/http"

	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/db"
	"tiendao/game/internal/ritual"
	"tiendao/game/internal/shop"
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

// useItem: POST /items/{itemId}/use — offline-bonus items (Tụ Linh Phù) and speed items (Tụ Linh Đan).
func (d Deps) useItem(w http.ResponseWriter, r *http.Request) {
	if d.Cultivation == nil || d.Content == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	itemID := r.PathValue("itemId")
	if def, ok := d.Content.Items.Get(itemID); ok && def.InstantPointsMilli > 0 {
		res, err := d.Cultivation.UseSpeedItem(r.Context(), d.Content.Items, userFrom(r.Context()), itemID, key)
		switch {
		case errors.Is(err, cultivation.ErrNoItem):
			writeError(w, 409, "no_item", "không có vật phẩm trong túi")
		case errors.Is(err, cultivation.ErrItemDailyLimit):
			writeError(w, 409, "daily_limit", "hôm nay đã dùng đủ số lần cho phép")
		case errors.Is(err, cultivation.ErrNoRoom):
			writeError(w, 409, "no_room", "hạn mức tu vi hôm nay không còn đủ cho trọn một viên — đan được giữ lại")
		case errors.Is(err, cultivation.ErrBottleneckFull):
			writeError(w, 409, "bottleneck_full", "bình cảnh không còn đủ chỗ cho trọn một viên, hãy đột phá trước — đan được giữ lại")
		case errors.Is(err, cultivation.ErrNoCharacter):
			writeError(w, 404, "character_not_found", "create a character first")
		case err != nil:
			d.internal(w, r, "use speed item", err)
		default:
			writeJSON(w, 200, res)
		}
		return
	}
	st, err := d.Cultivation.UseOfflineBonusItem(r.Context(), d.Content.Items, userFrom(r.Context()), itemID, key)
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
		writeError(w, 409, "no_gift", "cấp Tiên Duyên hiện tại chưa có quà ngày")
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
	case errors.Is(err, dailytask.ErrUseIncense):
		writeError(w, 409, "use_incense", "hãy dâng hương — phần thưởng được trao ngay khi dâng")
	case errors.Is(err, dailytask.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case err != nil:
		d.internal(w, r, "claim task", err)
	default:
		d.markActive(r, true) // a repeat claim is an error (already_claimed), never a 2xx
		writeJSON(w, 200, v)
	}
}

// offerIncense: POST /tasks/daily/incense — once per game day (ritual_streaks.last_day under the progression lock);
// a repeat returns the same result with replayed=true, so no Idempotency-Key is needed.
func (d Deps) offerIncense(w http.ResponseWriter, r *http.Request) {
	if d.Rituals == nil {
		unavailable(w)
		return
	}
	res, err := d.Rituals.Incense(r.Context(), userFrom(r.Context()))
	switch {
	case errors.Is(err, ritual.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case errors.Is(err, ritual.ErrNoIncense):
		writeError(w, 404, "unknown_task", "no incense task")
	case err != nil:
		d.internal(w, r, "incense", err)
	default:
		d.markActive(r, !res.Replayed)
		writeJSON(w, 200, res)
	}
}

// fortuneView: GET /fortune — disclosed odds, today's draw, active buff.
func (d Deps) fortuneView(w http.ResponseWriter, r *http.Request) {
	if d.Fortune == nil {
		unavailable(w)
		return
	}
	v, err := d.Fortune.View(r.Context(), userFrom(r.Context()))
	d.writeFortune(w, r, v, err)
}

// fortuneDraw: POST /fortune/draw — once per game day (PK); a repeat returns today's draw with replayed=true.
func (d Deps) fortuneDraw(w http.ResponseWriter, r *http.Request) {
	if d.Fortune == nil {
		unavailable(w)
		return
	}
	v, err := d.Fortune.Draw(r.Context(), userFrom(r.Context()))
	if err == nil {
		d.markActive(r, !v.Replayed)
	}
	d.writeFortune(w, r, v, err)
}

func (d Deps) writeFortune(w http.ResponseWriter, r *http.Request, v ritual.FortuneView, err error) {
	switch {
	case errors.Is(err, ritual.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case err != nil:
		d.internal(w, r, "fortune", err)
	default:
		writeJSON(w, 200, v)
	}
}

// shopView: GET /shop — listings with today's remaining limits for the caller.
func (d Deps) shopView(w http.ResponseWriter, r *http.Request) {
	if d.Shop == nil {
		unavailable(w)
		return
	}
	v, err := d.Shop.View(r.Context(), userFrom(r.Context()))
	if errors.Is(err, shop.ErrNoCharacter) {
		writeError(w, 404, "character_not_found", "create a character first")
		return
	} else if err != nil {
		d.internal(w, r, "shop", err)
		return
	}
	writeJSON(w, 200, v)
}

// shopBuy: POST /shop/buy {listingId, lots} + Idempotency-Key. Price and item come from shop.yaml only (S5).
func (d Deps) shopBuy(w http.ResponseWriter, r *http.Request) {
	if d.Shop == nil {
		unavailable(w)
		return
	}
	key, ok := idempotencyKey(w, r)
	if !ok {
		return
	}
	var body struct {
		ListingID string `json:"listingId"`
		Lots      int32  `json:"lots"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	res, err := d.Shop.Buy(r.Context(), userFrom(r.Context()), body.ListingID, body.Lots, key)
	switch {
	case errors.Is(err, shop.ErrUnknownListing):
		writeError(w, 404, "unknown_listing", "món hàng không tồn tại")
	case errors.Is(err, shop.ErrBadLots):
		writeError(w, 400, "invalid_lots", "số lượng không hợp lệ")
	case errors.Is(err, shop.ErrDailyLimit):
		writeError(w, 409, "daily_limit", "hôm nay đã mua đủ giới hạn món này")
	case errors.Is(err, shop.ErrTierLocked):
		writeError(w, 409, "tier_locked", "cần Tiên Duyên cao hơn để mua món này")
	case errors.Is(err, shop.ErrAlreadyOwned):
		writeError(w, 409, "already_owned", "đạo hữu đã có vật phẩm này")
	case errors.Is(err, shop.ErrInsufficient):
		writeError(w, 409, "insufficient_stones", "không đủ linh thạch")
	case errors.Is(err, shop.ErrKeyReuse):
		writeError(w, 409, "idempotency_key_reused", "Idempotency-Key đã dùng cho một giao dịch khác")
	case errors.Is(err, shop.ErrNoCharacter):
		writeError(w, 404, "character_not_found", "create a character first")
	case err != nil:
		d.internal(w, r, "shop buy", err)
	default:
		d.Metrics.Purchase("shop", res.Replayed)
		d.markActive(r, !res.Replayed)
		writeJSON(w, 200, res)
	}
}
