//go:build integration

package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"tiendao/game/internal/auth"
	"tiendao/game/internal/platform/testdb"
)

func newSvc(t *testing.T) *auth.Service {
	iss, err := auth.NewIssuer([]byte(strings.Repeat("k", 32)), 15*time.Minute, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return &auth.Service{Pool: testdb.New(t), Issuer: iss, RefreshTTL: 24 * time.Hour, Now: time.Now}
}

var gid = auth.Identity{Provider: "google", Subject: "sub-1", Email: "a@example.com", EmailVerified: true, Name: "Đạo Hữu"}

func TestLoginCreatesThenReusesUser(t *testing.T) {
	svc, ctx := newSvc(t), context.Background()
	a, err := svc.LoginWithIdentity(ctx, gid, "ua")
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.LoginWithIdentity(ctx, gid, "ua")
	if err != nil {
		t.Fatal(err)
	}
	if a.User.ID != b.User.ID || a.RefreshToken == b.RefreshToken {
		t.Fatalf("same identity must map to same user with a new session")
	}
	// Same email, different Google subject → refuse (no auto-link).
	other := gid
	other.Subject = "sub-2"
	if _, err := svc.LoginWithIdentity(ctx, other, "ua"); !errors.Is(err, auth.ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
	unverified := gid
	unverified.Subject, unverified.Email, unverified.EmailVerified = "sub-3", "b@example.com", false
	if _, err := svc.LoginWithIdentity(ctx, unverified, "ua"); !errors.Is(err, auth.ErrEmailUnverified) {
		t.Fatalf("want ErrEmailUnverified, got %v", err)
	}
}

func TestRefreshRotationAndReuseDetection(t *testing.T) {
	svc, ctx := newSvc(t), context.Background()
	now := time.Now()
	svc.Now = func() time.Time { return now }
	s0, err := svc.LoginWithIdentity(ctx, gid, "ua")
	if err != nil {
		t.Fatal(err)
	}
	s1, err := svc.Refresh(ctx, s0.RefreshToken, "ua")
	if err != nil {
		t.Fatal(err)
	}
	// Within the grace window (second tab / retried request) the old token yields a sibling session.
	if _, err := svc.Refresh(ctx, s0.RefreshToken, "ua"); err != nil {
		t.Fatalf("grace-window refresh should succeed: %v", err)
	}
	// After the grace window, replaying the rotated token revokes the whole family…
	now = now.Add(auth.RefreshGrace + time.Second)
	if _, err := svc.Refresh(ctx, s0.RefreshToken, "ua"); !errors.Is(err, auth.ErrRefreshReused) {
		t.Fatalf("want ErrRefreshReused, got %v", err)
	}
	// …so the legitimate newer token is dead too.
	if _, err := svc.Refresh(ctx, s1.RefreshToken, "ua"); !errors.Is(err, auth.ErrInvalidRefresh) {
		t.Fatalf("want ErrInvalidRefresh after family revocation, got %v", err)
	}
}

func TestLogoutRevokes(t *testing.T) {
	svc, ctx := newSvc(t), context.Background()
	s0, _ := svc.LoginWithIdentity(ctx, gid, "ua")
	if err := svc.Logout(ctx, s0.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Refresh(ctx, s0.RefreshToken, "ua"); !errors.Is(err, auth.ErrInvalidRefresh) {
		t.Fatalf("want ErrInvalidRefresh, got %v", err)
	}
}
