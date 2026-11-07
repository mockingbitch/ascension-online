//go:build integration

package cosmetic_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/content"
	"tiendao/game/internal/cosmetic"
	"tiendao/game/internal/platform/testdb"
)

type env struct {
	pool  *pgxpool.Pool
	svc   *cosmetic.Service
	user  uuid.UUID
	char  uuid.UUID
	clock *time.Time
	ctx   context.Context
}

func setup(t *testing.T, stones int64) *env {
	t.Helper()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 7, 3, 0, 0, 0, time.UTC) // 10:00 ICT, season dong, before launch (first era)
	e := &env{pool: pool, user: uuid.New(), char: uuid.New(), clock: &now, ctx: context.Background()}
	e.svc = &cosmetic.Service{Pool: pool, Cat: cb.Cosmetics, VIP: cb.VIP, Eras: cb.World, Now: func() time.Time { return *e.clock }}
	e.exec(t, "INSERT INTO users (id, email) VALUES ($1, $2)", e.user, e.user.String()+"@t")
	e.exec(t, "INSERT INTO characters (id, user_id, name) VALUES ($1, $2, 'Thử Trang')", e.char, e.user)
	e.exec(t, "INSERT INTO character_progressions (character_id, realm_id, stage_id, last_cultivated_at) VALUES ($1, 'luyen_khi', 'luyen_khi.tang_1', $2)", e.char, now)
	e.exec(t, "INSERT INTO currency_accounts (character_id, currency, balance) VALUES ($1, 'spirit_stone', $2)", e.char, stones)
	return e
}

func (e *env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(e.ctx, sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func (e *env) scalar(t *testing.T, sql string, args ...any) (v int64) {
	t.Helper()
	if err := e.pool.QueryRow(e.ctx, sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

func (e *env) balance(t *testing.T) int64 {
	return e.scalar(t, "SELECT balance FROM currency_accounts WHERE character_id=$1 AND currency='spirit_stone'", e.char)
}

// One purchase = 1 ownership row + 1 currency ledger row; a retry with the same key replays; a second copy is refused.
func TestBuyIdempotentLedgeredOnce(t *testing.T) {
	e := setup(t, 5000)
	r, err := e.svc.Buy(e.ctx, e.user, "title:so_tam", "cos-key-000001")
	if err != nil {
		t.Fatal(err)
	}
	if r.Replayed || r.Price != 1500 || r.State.Balance != 3500 {
		t.Fatalf("buy: %+v", r)
	}
	again, err := e.svc.Buy(e.ctx, e.user, "title:so_tam", "cos-key-000001")
	if err != nil || !again.Replayed || again.State.Balance != 3500 {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if _, err := e.svc.Buy(e.ctx, e.user, "title:tinh_tam", "cos-key-000001"); !errors.Is(err, cosmetic.ErrKeyReuse) {
		t.Fatalf("key reuse: %v", err)
	}
	if _, err := e.svc.Buy(e.ctx, e.user, "title:so_tam", "cos-key-000002"); !errors.Is(err, cosmetic.ErrAlreadyOwned) {
		t.Fatalf("second copy: %v", err)
	}
	if e.balance(t) != 3500 ||
		e.scalar(t, "SELECT COUNT(*) FROM character_cosmetics WHERE character_id=$1", e.char) != 1 ||
		e.scalar(t, "SELECT COUNT(*) FROM currency_ledger WHERE character_id=$1 AND reason='cosmetic_purchase' AND delta=-1500 AND balance_after=3500", e.char) != 1 {
		t.Fatal("double effect or missing ledger")
	}
	// No outbox, no progression or điểm duyên change from a cosmetic.
	if e.scalar(t, "SELECT COUNT(*) FROM outbox_events") != 0 || e.scalar(t, "SELECT COUNT(*) FROM vip_point_ledger") != 0 {
		t.Fatal("cosmetic purchase wrote side effects")
	}
}

func TestBuyRefusals(t *testing.T) {
	e := setup(t, 1400)
	cases := map[string]error{
		"title:so_tam":                  cosmetic.ErrInsufficient, // 1500 > 1400
		"cave_decoration:canh_dao":      cosmetic.ErrUnavailable,  // spring set in November
		"cave_decoration:co_phong_than": cosmetic.ErrUnavailable,  // Phong Thần era not reached
		"title:vip_1":                   cosmetic.ErrNotForSale,   // Tiên Duyên perk
		"title:khong_co":                cosmetic.ErrUnknown,
	}
	for id, want := range cases {
		if _, err := e.svc.Buy(e.ctx, e.user, id, "cos-ref-"+uuid.NewString()[:8]); !errors.Is(err, want) {
			t.Errorf("%s: got %v want %v", id, err, want)
		}
	}
	if e.balance(t) != 1400 || e.scalar(t, "SELECT COUNT(*) FROM character_cosmetics") != 0 {
		t.Fatal("partial write on refusal")
	}
	// The all-season set (1,200) is on sale; so is the winter set in November (checked via View).
	if v, err := e.svc.View(e.ctx, e.user); err != nil || !slices.Contains(v.Available, "cave_decoration:canh_mai") || slices.Contains(v.Available, "cave_decoration:canh_dao") {
		t.Fatalf("available: %+v %v", v.Available, err)
	}
	if _, err := e.svc.Buy(e.ctx, e.user, "cave_decoration:den_da", "cos-key-den001"); err != nil {
		t.Fatal(err)
	}
}

// Concurrency: 8 different keys race for the same cosmetic → exactly one purchase, one debit.
// 8 different cosmetics racing on a wallet that pays for 2 → exactly 2.
func TestBuyConcurrent(t *testing.T) {
	e := setup(t, 100_000)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := e.svc.Buy(e.ctx, e.user, "avatar_frame:truc_moc", "cos-race-"+string(rune('a'+i))+"0000")
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				ok++
			} else if !errors.Is(err, cosmetic.ErrAlreadyOwned) {
				t.Errorf("race: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if ok != 1 || e.balance(t) != 98_000 {
		t.Fatalf("ok %d balance %d", ok, e.balance(t))
	}

	e2 := setup(t, 3200) // so_tam 1500 + truc... pick 8 items ≤ 1600 each: only 2 fit
	ids := []string{"title:so_tam", "cave_decoration:den_da", "cave_decoration:canh_mai", "title:tinh_tam", "avatar_frame:truc_moc",
		"chat_aura:mac_huong", "avatar_frame:ngoc_bich", "cave_decoration:chau_lan"}
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			_, err := e2.svc.Buy(e2.ctx, e2.user, id, "cos-wal-"+string(rune('a'+i))+"0000")
			if err != nil && !errors.Is(err, cosmetic.ErrInsufficient) {
				t.Errorf("%s: %v", id, err)
			}
		}(i, id)
	}
	wg.Wait()
	spent := e2.scalar(t, "SELECT COALESCE(-SUM(delta),0) FROM currency_ledger WHERE character_id=$1", e2.char)
	if e2.balance(t) < 0 || spent+e2.balance(t) != 3200 ||
		spent != e2.scalar(t, "SELECT COALESCE(SUM(price),0) FROM character_cosmetics WHERE character_id=$1", e2.char) {
		t.Fatalf("inconsistent: spent %d balance %d", spent, e2.balance(t))
	}
}

func TestEquipOwnedOnlyIdempotent(t *testing.T) {
	e := setup(t, 20_000)
	// Not owned → refused, nothing written.
	if _, err := e.svc.Equip(e.ctx, e.user, cosmetic.Appearance{TitleID: "title:so_tam"}, "eq-key-000001"); !errors.Is(err, cosmetic.ErrNotOwned) {
		t.Fatalf("equip unowned: %v", err)
	}
	if _, err := e.svc.Equip(e.ctx, e.user, cosmetic.Appearance{AvatarFrameID: "avatar_frame:ngoc"}, "eq-key-000002"); !errors.Is(err, cosmetic.ErrNotOwned) {
		t.Fatalf("equip perk below tier: %v", err)
	}
	if e.scalar(t, "SELECT COUNT(*) FROM cosmetic_equips") != 0 {
		t.Fatal("refused equip logged")
	}
	for i, id := range []string{"title:so_tam", "cave_decoration:den_da", "cave_decoration:canh_mai", "chat_aura:mac_huong"} {
		if _, err := e.svc.Buy(e.ctx, e.user, id, "eq-buy-"+string(rune('a'+i))+"00000"); err != nil {
			t.Fatal(err)
		}
	}
	want := cosmetic.Appearance{TitleID: "title:so_tam", ChatAuraID: "chat_aura:mac_huong",
		CaveDecorationIDs: []string{"cave_decoration:canh_mai", "cave_decoration:den_da"}}
	r, err := e.svc.Equip(e.ctx, e.user, want, "eq-key-000003")
	if err != nil || r.Replayed || !r.State.Equipped.Equal(want) {
		t.Fatalf("equip: %+v %v", r, err)
	}
	again, err := e.svc.Equip(e.ctx, e.user, want, "eq-key-000003")
	if err != nil || !again.Replayed {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if _, err := e.svc.Equip(e.ctx, e.user, cosmetic.Appearance{}, "eq-key-000003"); !errors.Is(err, cosmetic.ErrKeyReuse) {
		t.Fatalf("key reuse: %v", err)
	}
	if e.scalar(t, "SELECT COUNT(*) FROM cosmetic_equips WHERE character_id=$1", e.char) != 1 ||
		e.scalar(t, "SELECT version FROM character_appearance WHERE character_id=$1", e.char) != 1 {
		t.Fatal("replay re-applied")
	}
	// Tiên Duyên perk: owned once the level reaches the granting tier (vip.yaml TD3 avatar_frame:ngoc, 500 points).
	e.exec(t, "INSERT INTO vip_accounts (user_id, points) VALUES ($1, 500)", e.user)
	if _, err := e.svc.Equip(e.ctx, e.user, cosmetic.Appearance{AvatarFrameID: "avatar_frame:ngoc", TitleID: "title:vip_3"}, "eq-key-000004"); err != nil {
		t.Fatalf("perk equip: %v", err)
	}
	// Concurrent equips with different keys serialize on the appearance row: final state = one of them, log = all.
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a := cosmetic.Appearance{TitleID: "title:so_tam"}
			if i%2 == 0 {
				a.TitleID = "title:vip_2"
			}
			if _, err := e.svc.Equip(e.ctx, e.user, a, "eq-race-"+string(rune('a'+i))+"0000"); err != nil {
				t.Errorf("race %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	if v := e.scalar(t, "SELECT version FROM character_appearance WHERE character_id=$1", e.char); v != 8 {
		t.Fatalf("version %d, want 8 (1 + 1 + 6)", v)
	}
	// The look is what others see: chat send snapshot reads it (covered in chat), GET /character reads it here.
	look, err := e.svc.View(e.ctx, e.user)
	if err != nil || look.Equipped.TitleID == "" {
		t.Fatalf("view: %+v %v", look, err)
	}
}
