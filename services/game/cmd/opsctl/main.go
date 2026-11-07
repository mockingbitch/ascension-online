// Command opsctl is the operator CLI for launch/reset (lo-trinh-checklist Phase 4). Wiring only; logic in internal/ops.
//
//	opsctl status
//	opsctl reset-beta  --admin <email|uuid> [--confirm "reset <db>"] [--keep-duyen] [--wipe-purchases] [--keep-redis] [--reason ...] [--allow-prod]
//	opsctl start-world --admin <email|uuid> [--confirm "start <db>"] [--allow-prod]
//
// Reads DATABASE_URL, REDIS_URL, APP_ENV, CONTENT_DIR like the api. Refuses APP_ENV=prod unless --allow-prod. Prints a
// summary and asks for the typed confirmation phrase on stdin unless --confirm carries it already. Never prints the
// connection string (it carries the password).
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"tiendao/game/internal/config"
	"tiendao/game/internal/content"
	"tiendao/game/internal/db"
	"tiendao/game/internal/faction"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/ops"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/platform/redisx"
	"tiendao/game/internal/world"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "opsctl:", err)
		os.Exit(1)
	}
}

const usage = `usage: opsctl <status|reset-beta|start-world> [flags]   (opsctl <cmd> -h for flags)`

func run(ctx context.Context, args []string, in io.Reader, out io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	cfg := config.Load()
	if err := cfg.ValidateWorker(); err != nil { // APP_ENV must be explicit
		return err
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(out)
	adminRef := fs.String("admin", "", "admin user (e-mail or UUID) recorded in admin_audit_logs")
	confirm := fs.String("confirm", "", "confirmation phrase (skips the interactive prompt)")
	allowProd := fs.Bool("allow-prod", false, "allow running against APP_ENV=prod")
	keepDuyen := fs.Bool("keep-duyen", false, "reset-beta: keep Tiên Duyên tables (vip_accounts, vip_point_ledger, vip_daily_gift_claims)")
	wipePurchases := fs.Bool("wipe-purchases", false, "reset-beta: allow wiping real-money purchase rows of vip_point_ledger")
	keepRedis := fs.Bool("keep-redis", false, "reset-beta: do not FLUSHDB + rebuild Redis afterwards")
	reason := fs.String("reason", "", "reason stored in the audit row")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if cmd != "status" && cfg.Env == "prod" && !*allowProd {
		return errors.New("APP_ENV=prod: refusing without --allow-prod")
	}
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	pool, err := postgres.Connect(ctx, cfg.DatabaseURL, 4)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch cmd {
	case "status":
		return status(ctx, pool, cfg.Env, out)
	case "reset-beta":
		admin, err := ops.ResolveAdmin(ctx, pool, *adminRef)
		if err != nil {
			return err
		}
		o := ops.ResetOptions{Admin: admin, KeepDuyen: *keepDuyen, WipePurchases: *wipePurchases, Reason: *reason}
		s, err := ops.Plan(ctx, pool, o)
		if err != nil {
			return err
		}
		printSummary(out, cfg.Env, s, o)
		if len(s.Unclassified) > 0 {
			return fmt.Errorf("%w: %s", ops.ErrUnclassified, strings.Join(s.Unclassified, ", "))
		}
		o.ConfirmPhrase, err = askConfirm(in, out, *confirm, ops.ResetConfirmPhrase(s.Database))
		if err != nil {
			return err
		}
		start := time.Now()
		if s, err = ops.ResetBeta(ctx, pool, o); err != nil {
			return err
		}
		fmt.Fprintf(out, "reset done: %d rows removed from %d tables in %s (audit: ops.reset_beta)\n", s.Total(), len(s.Counts),
			time.Since(start).Round(time.Millisecond))
		if *keepRedis {
			fmt.Fprintln(out, "redis untouched (--keep-redis): the worker reconcile rebuilds rankings within LEADERBOARD_RECONCILE_SECONDS")
			return nil
		}
		return rebuildRedis(ctx, cfg, pool, out)
	case "start-world":
		admin, err := ops.ResolveAdmin(ctx, pool, *adminRef)
		if err != nil {
			return err
		}
		var dbName string
		if err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName); err != nil {
			return err
		}
		svc := &world.Service{Pool: pool, Now: time.Now}
		ws, err := db.New(pool).GetWorldState(ctx)
		if err != nil {
			return err
		}
		if ws.StartedAt != nil {
			fmt.Fprintf(out, "world already started at %s (server day %d): nothing to do\n", ws.StartedAt.Format(time.RFC3339),
				world.ServerDay(ws.StartedAt, time.Now()))
			return nil
		}
		fmt.Fprintf(out, "env=%s database=%s\nThis sets world_state.started_at = now (server day 0). It cannot be undone except by reset-beta.\n", cfg.Env, dbName)
		if _, err := askConfirm(in, out, *confirm, "start "+dbName); err != nil {
			return err
		}
		st, err := svc.Start(ctx, admin)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "world started at %s (audit: world.start)\n", st.StartedAt.Format(time.RFC3339))
		return nil
	default:
		return errors.New(usage)
	}
}

func status(ctx context.Context, pool *pgxpool.Pool, env string, out io.Writer) error {
	s, err := ops.Plan(ctx, pool, ops.ResetOptions{})
	if err != nil {
		return err
	}
	ws, err := db.New(pool).GetWorldState(ctx)
	if err != nil {
		return err
	}
	st := map[string]any{"startedAt": ws.StartedAt, "serverDay": world.ServerDay(ws.StartedAt, time.Now()),
		"progressMilli": ws.ProgressMilli, "pausedAt": ws.PausedAt}
	pending, age, err := ops.OutboxBacklog(ctx, pool)
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(map[string]any{"env": env, "database": s.Database, "world": st, "characters": s.Counts["characters"],
		"outboxPending": pending, "outboxOldestAge": age.Round(time.Second).String(), "unclassifiedTables": s.Unclassified}, "", "  ")
	fmt.Fprintln(out, string(b))
	return nil
}

func printSummary(out io.Writer, env string, s ops.Summary, o ops.ResetOptions) {
	fmt.Fprintf(out, "RESET BETA — env=%s database=%s worldStarted=%v\n", env, s.Database, s.WorldStarted)
	names := make([]string, 0, len(s.Counts))
	for t := range s.Counts {
		names = append(names, t)
	}
	slices.Sort(names)
	fmt.Fprintln(out, "Tables truncated (rows):")
	for _, t := range names {
		fmt.Fprintf(out, "  %-28s %d\n", t, s.Counts[t])
	}
	fmt.Fprintf(out, "Total rows: %d\nKept: %s\n", s.Total(), strings.Join(s.Kept, ", "))
	if s.Purchases > 0 {
		fmt.Fprintf(out, "WARNING: %d real-money purchase rows in vip_point_ledger (keepDuyen=%v wipePurchases=%v)\n", s.Purchases,
			o.KeepDuyen, o.WipePurchases)
	}
	if len(s.Unclassified) > 0 {
		fmt.Fprintf(out, "UNCLASSIFIED tables (refusing): %s\n", strings.Join(s.Unclassified, ", "))
	}
}

// askConfirm returns the phrase given by flag, or reads one line from in after showing the expected phrase.
func askConfirm(in io.Reader, out io.Writer, given, want string) (string, error) {
	if given == "" {
		fmt.Fprintf(out, "Type %q to continue: ", want)
		line, err := bufio.NewReader(in).ReadString('\n')
		if err != nil && line == "" {
			return "", ops.ErrConfirm
		}
		given = strings.TrimSpace(line)
	}
	if given != want {
		return "", ops.ErrConfirm
	}
	return given, nil
}

// rebuildRedis drops the cache (Redis holds only rebuildable data) and rebuilds the rankings from Postgres.
func rebuildRedis(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, out io.Writer) error {
	if cfg.RedisURL == "" {
		fmt.Fprintln(out, "REDIS_URL empty: skip Redis rebuild (flush it before starting the api)")
		return nil
	}
	cb, err := content.LoadAll(cfg.ContentDir)
	if err != nil {
		return err
	}
	rdb, err := redisx.Connect(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer rdb.Close()
	return flushAndRebuild(ctx, rdb, pool, cb, out)
}

func flushAndRebuild(ctx context.Context, rdb *redis.Client, pool *pgxpool.Pool, cb *content.Bundle, out io.Writer) error {
	if err := rdb.FlushDB(ctx).Err(); err != nil {
		return fmt.Errorf("redis flushdb: %w", err)
	}
	n, err := (&leaderboard.Service{Pool: pool, Redis: rdb, Realms: cb.Realms}).Rebuild(ctx)
	if err != nil {
		return fmt.Errorf("leaderboard rebuild: %w", err)
	}
	f, err := (&faction.Ranking{Pool: pool, Redis: rdb, Realms: cb.Realms, Cat: cb.Factions,
		CapMilli: cb.World.Settings.WorldCreditCapMilli}).Rebuild(ctx)
	if err != nil {
		return fmt.Errorf("faction ranking rebuild: %w", err)
	}
	fmt.Fprintf(out, "redis flushed and rebuilt: leaderboard=%d factionMembers=%d\n", n, f)
	return nil
}
