// Command api serves the REST API.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tiendao/game/internal/auth"
	"tiendao/game/internal/breakthrough"
	"tiendao/game/internal/character"
	"tiendao/game/internal/chat"
	"tiendao/game/internal/config"
	"tiendao/game/internal/content"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/httpapi"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/platform/redisx"
	"tiendao/game/internal/realtime"
	"tiendao/game/internal/report"
	"tiendao/game/internal/vip"
	"tiendao/game/internal/world"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log); err != nil {
		log.Error("api exited", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()
	if err := cfg.ValidateAPI(); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cb, err := content.LoadAll(cfg.ContentDir)
	if err != nil {
		return err
	}

	secret := []byte(cfg.JWTSecret)
	if len(secret) < 32 { // dev only (ValidateAPI enforces it elsewhere): sessions reset on restart
		secret = make([]byte, 32)
		_, _ = rand.Read(secret)
		log.Warn("JWT_SECRET not set: using a random per-process secret (dev only)")
	}
	issuer, err := auth.NewIssuer(secret, cfg.AccessTokenTTL, time.Now)
	if err != nil {
		return err
	}

	deps := httpapi.Deps{
		Log: log, Content: cb, Now: time.Now, Issuer: issuer, Flow: auth.NewFlowSigner(secret),
		PublicBaseURL: cfg.PublicBaseURL, SecureCookies: !cfg.IsDev(), AuthDevHeader: cfg.AuthDevHeader, TrustProxy: cfg.TrustProxy,
	}

	if cfg.GoogleEnabled() {
		g, err := auth.NewGoogle(ctx, cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.PublicBaseURL+"/api/v1/auth/google/callback")
		if err != nil {
			return err
		}
		deps.Google = g
	} else {
		log.Warn("Google sign-in not configured (GOOGLE_CLIENT_ID/SECRET empty)")
	}

	if cfg.DatabaseURL != "" {
		pool, err := postgres.Connect(ctx, cfg.DatabaseURL, cfg.DBMaxConns)
		if err != nil {
			return err
		}
		defer pool.Close()
		cult := &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Tasks: cb.DailyTasks, Now: time.Now}
		deps.Pool = pool
		deps.Cultivation = cult
		deps.Auth = &auth.Service{Pool: pool, Issuer: issuer, RefreshTTL: cfg.RefreshTokenTTL, Now: time.Now}
		deps.Characters = &character.Service{Pool: pool, Cultivation: cult, Names: cb.Moderation}
		deps.Chat = &chat.Service{Pool: pool, Cat: cb.Chat, Realms: cb.Realms, Mod: cb.Moderation, Now: time.Now}
		deps.Reports = &report.Service{Pool: pool, Cat: cb.Moderation, Now: time.Now}
		deps.Breakthrough = &breakthrough.Service{Pool: pool, Cultivation: cult, RNG: breakthrough.NewRNG()}
		deps.VIP = &vip.Service{Pool: pool, Table: cb.VIP, Now: time.Now}
		deps.DailyTasks = &dailytask.Service{Pool: pool, Cat: cb.DailyTasks, Now: time.Now}
		deps.World = &world.Service{Pool: pool, Realms: cb.Realms, Cat: cb.World, Contribution: cb.Contribution, Now: time.Now}

		if cfg.RedisURL != "" {
			rdb, err := redisx.Connect(ctx, cfg.RedisURL)
			if err != nil {
				return err
			}
			defer rdb.Close()
			deps.Leaderboard = &leaderboard.Service{Pool: pool, Redis: rdb, Realms: cb.Realms}
			deps.Redis = rdb
			deps.Chat.Publish = func(ctx context.Context, typ, id string, data any) error {
				return realtime.Publish(ctx, rdb, realtime.Envelope{Type: typ, ID: id, TS: time.Now().UTC(), Data: data})
			}
			origin, err := url.Parse(cfg.PublicBaseURL)
			if err != nil {
				return err
			}
			hub := realtime.NewHub(log, []string{origin.Host}, cfg.WSMaxClients)
			go hub.Subscribe(ctx, rdb)
			deps.Realtime = hub
		} else {
			log.Warn("REDIS_URL empty: leaderboard, realtime and shared rate limits disabled")
		}
	} else {
		log.Warn("DATABASE_URL empty: running in content-only mode")
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.New(deps),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr, "env", cfg.Env, "stages", len(cb.Realms.Stages()))
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	log.Info("shutting down")
	return srv.Shutdown(shutdownCtx)
}
