//go:build integration

package report_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/character"
	"tiendao/game/internal/content"
	"tiendao/game/internal/moderation"
	"tiendao/game/internal/platform/testdb"
	"tiendao/game/internal/report"
)

func TestSubmitAndResolve(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	cb, err := content.LoadAll("../../../../content/catalog")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 6, 5, 0, 0, 0, time.UTC) // 12:00 ICT
	svc := &report.Service{Pool: pool, Cat: cb.Moderation, Now: func() time.Time { return now }}

	mk := func(name string) (uuid.UUID, uuid.UUID) {
		t.Helper()
		u, ch := uuid.New(), uuid.New()
		if _, err := pool.Exec(ctx, "INSERT INTO users (id, email, role) VALUES ($1, $2, $3)", u, u.String()+"@t", "player"); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "INSERT INTO characters (id, user_id, name) VALUES ($1, $2, $3)", ch, u, name); err != nil {
			t.Fatal(err)
		}
		return u, ch
	}
	reporterA, _ := mk("Lâm Thanh Vân")
	reporterB, _ := mk("Hàn Lập")
	targetUser, target := mk("Tên Xấu Cũ")
	admin, _ := mk("Quản Sự")
	if _, err := pool.Exec(ctx, "UPDATE users SET role='admin' WHERE id=$1", admin); err != nil {
		t.Fatal(err)
	}

	// Validation + self report + unknown target.
	if _, err := svc.Submit(ctx, reporterA, target, "spam", "", "key-aaaa-1", 0); !errors.Is(err, report.ErrInvalidReport) {
		t.Fatalf("bad reason: %v", err)
	}
	if _, err := svc.Submit(ctx, targetUser, target, report.ReasonOffensiveName, "", "key-self-1", 0); !errors.Is(err, report.ErrSelfReport) {
		t.Fatalf("self: %v", err)
	}
	if _, err := svc.Submit(ctx, reporterA, uuid.New(), report.ReasonOffensiveName, "", "key-none-1", 0); !errors.Is(err, report.ErrTargetNotFound) {
		t.Fatalf("missing target: %v", err)
	}

	// Idempotent replay, key reuse, open-report dedupe.
	r1, err := svc.Submit(ctx, reporterA, target, report.ReasonOffensiveName, "  tên tục  ", "key-aaaa-2", 0)
	if err != nil || r1.Note != "tên tục" || r1.TargetName != "Tên Xấu Cũ" || r1.Status != report.StatusOpen {
		t.Fatalf("submit %+v %v", r1, err)
	}
	if again, err := svc.Submit(ctx, reporterA, target, report.ReasonOffensiveName, "", "key-aaaa-2", 0); err != nil || again.ID != r1.ID {
		t.Fatalf("replay %+v %v", again, err)
	}
	if _, err := svc.Submit(ctx, reporterA, target, report.ReasonHarassment, "", "key-aaaa-2", 0); !errors.Is(err, report.ErrKeyReuse) {
		t.Fatalf("key reuse: %v", err)
	}
	if dup, err := svc.Submit(ctx, reporterA, target, report.ReasonOffensiveName, "", "key-aaaa-3", 0); err != nil || dup.ID != r1.ID {
		t.Fatalf("dedupe %+v %v", dup, err)
	}
	r2, err := svc.Submit(ctx, reporterB, target, report.ReasonOffensiveName, "", "key-bbbb-1", 0)
	if err != nil {
		t.Fatal(err)
	}
	harass, err := svc.Submit(ctx, reporterB, target, report.ReasonHarassment, "", "key-bbbb-2", 0)
	if err != nil {
		t.Fatal(err)
	}

	list, err := svc.List(ctx, report.StatusOpen, 50, 0)
	if err != nil || len(list) != 3 {
		t.Fatalf("list %+v %v", list, err)
	}
	for _, e := range list { // same created_at (fixed clock): order among them is by id, so look up by id
		if e.OpenAgainstTarget != 3 || e.TargetCurrentName != "Tên Xấu Cũ" || (e.ID == r1.ID && e.ReporterName != "Lâm Thanh Vân") {
			t.Fatalf("entry %+v", e)
		}
	}

	// Rename to a blocked / taken name fails and changes nothing.
	if _, err := svc.Resolve(ctx, admin, r1.ID, report.StatusActioned, "", "Hồng Quân"); !errors.Is(err, moderation.ErrNameNotAllowed) {
		t.Fatalf("blocked rename: %v", err)
	}
	if _, err := svc.Resolve(ctx, admin, r1.ID, report.StatusActioned, "", "hàn lập"); !errors.Is(err, character.ErrNameTaken) {
		t.Fatalf("taken rename: %v", err)
	}
	if _, err := svc.Resolve(ctx, admin, r1.ID, report.StatusDismissed, "", "Tên Mới"); !errors.Is(err, report.ErrInvalidResolution) {
		t.Fatalf("rename with dismiss: %v", err)
	}

	// Rename resolves every open name report against the target; harassment report stays open.
	res, err := svc.Resolve(ctx, admin, r1.ID, report.StatusActioned, "đổi tên", "Vô Danh Khách")
	if err != nil || res.RenamedFrom != "Tên Xấu Cũ" || res.RenamedTo != "Vô Danh Khách" || len(res.AlsoResolved) != 1 || res.AlsoResolved[0] != r2.ID {
		t.Fatalf("resolve %+v %v", res, err)
	}
	var name string
	if err := pool.QueryRow(ctx, "SELECT name FROM characters WHERE id=$1", target).Scan(&name); err != nil || name != "Vô Danh Khách" {
		t.Fatalf("name %q %v", name, err)
	}
	if _, err := svc.Resolve(ctx, admin, r2.ID, report.StatusDismissed, "", ""); !errors.Is(err, report.ErrAlreadyResolved) {
		t.Fatalf("already resolved: %v", err)
	}
	if _, err := svc.Resolve(ctx, admin, harass.ID, report.StatusDismissed, "không đủ chứng cứ", ""); err != nil {
		t.Fatal(err)
	}
	var audits int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM admin_audit_logs WHERE admin_user_id=$1", admin).Scan(&audits); err != nil || audits != 4 {
		t.Fatalf("audits %d %v (want rename + 3 report.resolve)", audits, err)
	}
	if open, _ := svc.List(ctx, report.StatusOpen, 50, 0); len(open) != 0 {
		t.Fatalf("still open: %+v", open)
	}

	// Daily limit (10): reporterA already filed 1 today.
	for i := 0; i < 9; i++ {
		_, ch := mk("Mục Tiêu " + string(rune('A'+i)) + "a")
		if _, err := svc.Submit(ctx, reporterA, ch, report.ReasonHarassment, "", "lim-"+uuid.NewString()[:8], 0); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
	}
	_, last := mk("Mục Tiêu Cuối")
	if _, err := svc.Submit(ctx, reporterA, last, report.ReasonHarassment, "", "lim-overflow", 0); !errors.Is(err, report.ErrDailyLimit) {
		t.Fatalf("limit: %v", err)
	}
	// Next game day the limit resets.
	now = now.Add(24 * time.Hour)
	if _, err := svc.Submit(ctx, reporterA, last, report.ReasonHarassment, "", "lim-nextday", 0); err != nil {
		t.Fatalf("next day: %v", err)
	}
}
