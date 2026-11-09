//go:build integration

package tutorial_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/realm"
	"tiendao/game/internal/tutorial"
)

// Progress is display-only and idempotent: first mark wins, same key replays, a key for another step is refused,
// onboarding/unknown steps are not stored, skip-all mutes, reset forgets. Nothing else in the DB moves.
func TestTutorialProgress(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	realms, err := realm.LoadFile("../../../../content/catalog/realms.yaml")
	if err != nil {
		t.Fatal(err)
	}
	sc, err := tutorial.Load("../../../../content/story/tutorial", tutorial.Vars(realms))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 9, 3, 0, 0, 0, time.UTC)
	svc := &tutorial.Service{Pool: pool, Script: sc, Now: func() time.Time { return now }}
	user, char := uuid.New(), uuid.New()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	if _, err := svc.Get(ctx, user); !errors.Is(err, tutorial.ErrNoCharacter) {
		t.Fatalf("no character: %v", err)
	}
	exec("INSERT INTO users (id, email) VALUES ($1, $2)", user, user.String()+"@t")
	exec("INSERT INTO characters (id, user_id, name) VALUES ($1, $2, 'Thử Dẫn')", char, user)

	st, err := svc.Get(ctx, user)
	if err != nil || len(st.Done)+len(st.Skipped) != 0 || st.Muted {
		t.Fatalf("fresh: %+v %v", st, err)
	}
	r, err := svc.Mark(ctx, user, "first_settle", "done", "tut-key-0001")
	if err != nil || r.Replayed || !slices.Equal(r.State.Done, []string{"first_settle"}) {
		t.Fatalf("mark: %+v %v", r, err)
	}
	if r, err := svc.Mark(ctx, user, "first_settle", "done", "tut-key-0001"); err != nil || !r.Replayed {
		t.Fatalf("replay: %+v %v", r, err)
	}
	if _, err := svc.Mark(ctx, user, "first_shop", "done", "tut-key-0001"); !errors.Is(err, tutorial.ErrKeyReuse) {
		t.Fatalf("key reuse: %v", err)
	}
	// first mark wins (another device skips it later: still done)
	if r, err := svc.Mark(ctx, user, "first_settle", "skipped", "tut-key-0002"); err != nil || !slices.Contains(r.State.Done, "first_settle") || len(r.State.Skipped) != 0 {
		t.Fatalf("second mark: %+v %v", r, err)
	}
	if r, err := svc.Mark(ctx, user, "first_shop", "skipped", "tut-key-0003"); err != nil || !slices.Equal(r.State.Skipped, []string{"first_shop"}) {
		t.Fatalf("skip: %+v %v", r, err)
	}
	for _, id := range []string{"ob_the_luc", "nope", "First"} {
		if _, err := svc.Mark(ctx, user, id, "done", "tut-key-x-"+id); !errors.Is(err, tutorial.ErrUnknownStep) {
			t.Fatalf("%s: %v", id, err)
		}
	}
	if _, err := svc.Mark(ctx, user, "first_realm", "won", "tut-key-0004"); !errors.Is(err, tutorial.ErrBadStatus) {
		t.Fatalf("bad status: %v", err)
	}
	if st, err := svc.SetMuted(ctx, user, true, "tut-set-0001"); err != nil || !st.Muted {
		t.Fatalf("mute: %+v %v", st, err)
	}
	st, err = svc.Reset(ctx, user, "tut-set-0002")
	if err != nil || st.Muted || len(st.Done)+len(st.Skipped) != 0 {
		t.Fatalf("reset: %+v %v", st, err)
	}
	// a lost reset answer retried after a new mark: replay, the new mark survives
	if _, err := svc.Mark(ctx, user, "first_realm", "done", "tut-key-0005"); err != nil {
		t.Fatal(err)
	}
	if st, err := svc.Reset(ctx, user, "tut-set-0002"); err != nil || !slices.Equal(st.Done, []string{"first_realm"}) {
		t.Fatalf("reset replay: %+v %v", st, err)
	}
	// two devices / a double tap with the same key for different steps: one wins, the other is a clean 409 (no 500)
	errs := make(chan error, 2)
	for _, id := range []string{"first_lecture", "first_faction"} {
		go func() { _, err := svc.Mark(ctx, user, id, "done", "tut-key-race"); errs <- err }()
	}
	ok, reused := 0, 0
	for range 2 {
		switch err := <-errs; {
		case err == nil:
			ok++
		case errors.Is(err, tutorial.ErrKeyReuse):
			reused++
		default:
			t.Fatalf("race: %v", err)
		}
	}
	if ok != 1 || reused != 1 {
		t.Fatalf("race: ok=%d reused=%d", ok, reused)
	}
	// display only: no currency, ledger, inventory, progression or outbox row was written
	var n int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM currency_ledger) + (SELECT count(*) FROM inventory_transactions)
		+ (SELECT count(*) FROM character_progressions) + (SELECT count(*) FROM outbox_events)`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("side effects: %d %v", n, err)
	}
}
