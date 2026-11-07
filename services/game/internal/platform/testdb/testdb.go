// Package testdb gives integration tests an isolated, fully migrated PostgreSQL database.
//
// Usage (file must carry `//go:build integration`):
//
//	pool := testdb.New(t)
//
// Requires TEST_DATABASE_URL pointing at a server where the user may CREATE DATABASE
// (default: the docker-compose postgres). Each call creates a fresh database and drops it on cleanup.
package testdb

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultURL = "postgres://tiendao:tiendao@localhost:2432/tiendao?sslmode=disable"

// New creates a migrated database and returns a pool to it.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		base = defaultURL
	}
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("testdb: connect %s: %v (run `make db-up`)", base, err)
	}
	defer admin.Close(ctx)

	name := fmt.Sprintf("tiendao_test_%d_%d", time.Now().UnixNano(), rand.IntN(1e6))
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("testdb: create: %v", err)
	}
	u, _ := url.Parse(base)
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("testdb: pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		c, err := pgx.Connect(context.Background(), base)
		if err != nil {
			return
		}
		defer c.Close(context.Background())
		_, _ = c.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})

	if err := migrate(ctx, pool); err != nil {
		t.Fatalf("testdb: migrate: %v", err)
	}
	return pool
}

func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations")
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	slices.Sort(files)
	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
	}
	return nil
}
