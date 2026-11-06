// Package config reads process configuration from environment variables (12-factor).
package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config for api and worker processes.
type Config struct {
	Env             string // dev | staging | prod
	HTTPAddr        string
	DatabaseURL     string
	DBMaxConns      int32
	RedisURL        string
	ContentDir      string
	StoryDir        string // content/story (announcement templates)
	WSMaxClients    int
	ShutdownTimeout time.Duration

	// Auth
	JWTSecret          string        // ≥ 32 bytes; required outside dev
	AccessTokenTTL     time.Duration // default 15m
	RefreshTokenTTL    time.Duration // default 30d
	GoogleClientID     string
	GoogleClientSecret string
	PublicBaseURL      string // web origin users see, e.g. http://localhost:5173 — OAuth redirect is built from it
	AuthDevHeader      bool   // DEV ONLY: trust X-Dev-User-Id. Refused unless Env=dev.
	TrustProxy         bool   // take client IP from X-Forwarded-For (only behind our Nginx)

	// Worker
	WorkerInterval    time.Duration
	ReconcileInterval time.Duration
	WorldInterval     time.Duration // world day job: per-day totals, day close + median
}

// Load reads env with sane dev defaults.
func Load() Config {
	return Config{
		Env:                os.Getenv("APP_ENV"), // REQUIRED, no default (a missing value must never mean dev)
		HTTPAddr:           get("HTTP_ADDR", ":8080"),
		DatabaseURL:        get("DATABASE_URL", ""),
		DBMaxConns:         int32(getInt("DB_MAX_CONNS", 20)),
		RedisURL:           get("REDIS_URL", ""),
		ContentDir:         get("CONTENT_DIR", "../../content/catalog"),
		StoryDir:           get("STORY_DIR", "../../content/story"),
		WSMaxClients:       getInt("WS_MAX_CLIENTS", 10000),
		ShutdownTimeout:    time.Duration(getInt("SHUTDOWN_TIMEOUT_SECONDS", 15)) * time.Second,
		JWTSecret:          get("JWT_SECRET", ""),
		AccessTokenTTL:     time.Duration(getInt("ACCESS_TOKEN_TTL_SECONDS", 900)) * time.Second,
		RefreshTokenTTL:    time.Duration(getInt("REFRESH_TOKEN_TTL_HOURS", 720)) * time.Hour,
		GoogleClientID:     get("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: get("GOOGLE_CLIENT_SECRET", ""),
		PublicBaseURL:      strings.TrimRight(get("PUBLIC_BASE_URL", "http://localhost:5173"), "/"),
		AuthDevHeader:      get("AUTH_DEV_HEADER", "false") == "true",
		TrustProxy:         get("TRUST_PROXY", "false") == "true",
		WorkerInterval:     time.Duration(getInt("WORKER_INTERVAL_MS", 1000)) * time.Millisecond,
		ReconcileInterval:  time.Duration(getInt("LEADERBOARD_RECONCILE_SECONDS", 600)) * time.Second,
		WorldInterval:      time.Duration(getInt("WORLD_TICK_SECONDS", 60)) * time.Second,
	}
}

// ValidateWorker checks worker settings.
func (c Config) ValidateWorker() error {
	switch c.Env {
	case "dev", "staging", "prod":
		return nil
	}
	return errors.New("APP_ENV must be set to dev, staging or prod")
}

// IsDev reports the dev environment.
func (c Config) IsDev() bool { return c.Env == "dev" }

// GoogleEnabled reports whether Google sign-in is configured.
func (c Config) GoogleEnabled() bool { return c.GoogleClientID != "" && c.GoogleClientSecret != "" }

// ValidateAPI enforces production safety rules.
func (c Config) ValidateAPI() error {
	var errs []error
	switch c.Env {
	case "dev", "staging", "prod":
	default:
		errs = append(errs, errors.New("APP_ENV must be set to dev, staging or prod"))
	}
	if c.AccessTokenTTL <= 0 || c.AccessTokenTTL > time.Hour {
		errs = append(errs, errors.New("ACCESS_TOKEN_TTL_SECONDS must be within 1..3600"))
	}
	if c.AuthDevHeader && !c.IsDev() {
		errs = append(errs, errors.New("AUTH_DEV_HEADER may only be enabled when APP_ENV=dev"))
	}
	if !c.IsDev() {
		if len(c.JWTSecret) < 32 {
			errs = append(errs, errors.New("JWT_SECRET (≥ 32 bytes) is required outside dev"))
		}
		if !c.GoogleEnabled() {
			errs = append(errs, errors.New("GOOGLE_CLIENT_ID/GOOGLE_CLIENT_SECRET are required outside dev"))
		}
		if !strings.HasPrefix(c.PublicBaseURL, "https://") {
			errs = append(errs, errors.New("PUBLIC_BASE_URL must be https outside dev"))
		}
	}
	return errors.Join(errs...)
}

func get(k, def string) string {
	if v, ok := os.LookupEnv(k); ok && v != "" {
		return v
	}
	return def
}

func getInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return def
}
