// Package httpapi exposes the REST/JSON API under /api/v1 (GDD §17).
// Handlers are thin: decode → call domain service → encode authoritative state.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"tiendao/game/internal/auth"
	"tiendao/game/internal/breakthrough"
	"tiendao/game/internal/character"
	"tiendao/game/internal/content"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/realtime"
	"tiendao/game/internal/vip"
	"tiendao/game/internal/world"
)

// Deps are injected by cmd/api. Nil services → their routes answer 503 (e.g. content-only dev mode).
type Deps struct {
	Log     *slog.Logger
	Pool    *pgxpool.Pool
	Content *content.Bundle
	Now     func() time.Time

	Auth         *auth.Service
	Issuer       *auth.Issuer
	Google       auth.Provider // nil when Google is not configured
	Flow         auth.FlowSigner
	Cultivation  *cultivation.Service
	Characters   *character.Service
	Breakthrough *breakthrough.Service
	Leaderboard  *leaderboard.Service
	VIP          *vip.Service
	DailyTasks   *dailytask.Service
	World        *world.Service
	Realtime     *realtime.Hub // nil → /ws disabled
	Redis        *redis.Client // shared rate limits; nil → per-instance limits

	PublicBaseURL string
	SecureCookies bool
	AuthDevHeader bool // DEV ONLY
	TrustProxy    bool
}

// New builds the router.
func New(d Deps) http.Handler {
	if d.Now == nil {
		d.Now = time.Now
	}
	lim := func(name string, events int, per time.Duration, burst int) *limiter {
		return newLimiter(limitSpec{name: name, events: events, per: per, burst: burst}, d.Redis, d.TrustProxy, d.Log)
	}
	authLimit := lim("auth", 1, 5*time.Second, 10)     // per IP: burst 10, then 1 per 5s
	refreshLimit := lim("refresh", 1, time.Second, 30) // every open tab refreshes; own bucket
	gameLimit := lim("game", 2, time.Second, 20)       // per user for mutations
	wsLimit := lim("ws", 1, 2*time.Second, 10)         // per IP connection attempts
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", d.readyz)

	// Public catalogue
	mux.HandleFunc("GET /api/v1/realms", d.listRealms)
	mux.HandleFunc("GET /api/v1/items", d.listItems)
	mux.HandleFunc("GET /api/v1/vip/tiers", d.listVIPTiers)

	// Auth
	mux.Handle("GET /api/v1/auth/google/start", authLimit.byIP(http.HandlerFunc(d.googleStart)))
	mux.Handle("GET /api/v1/auth/google/callback", authLimit.byIP(http.HandlerFunc(d.googleCallback)))
	mux.Handle("POST /api/v1/auth/refresh", refreshLimit.byIP(http.HandlerFunc(d.refresh)))
	mux.Handle("POST /api/v1/auth/logout", http.HandlerFunc(d.logout))
	mux.Handle("GET /api/v1/me", d.authn(http.HandlerFunc(d.me)))

	// Game (authenticated)
	mux.Handle("GET /api/v1/vip/status", d.authn(http.HandlerFunc(d.vipStatus)))
	mux.Handle("GET /api/v1/character", d.authn(http.HandlerFunc(d.getCharacter)))
	mux.Handle("POST /api/v1/character", d.authn(gameLimit.byUser(http.HandlerFunc(d.createCharacter))))
	mux.Handle("POST /api/v1/cultivation/settle", d.authn(gameLimit.byUser(http.HandlerFunc(d.settle))))
	mux.Handle("GET /api/v1/breakthrough/preview", d.authn(http.HandlerFunc(d.breakthroughPreview)))
	mux.Handle("POST /api/v1/breakthrough/attempt", d.authn(gameLimit.byUser(http.HandlerFunc(d.breakthroughAttempt))))
	mux.Handle("GET /api/v1/leaderboards/global", http.HandlerFunc(d.leaderboardPage))
	mux.Handle("GET /api/v1/leaderboards/global/me", d.authn(http.HandlerFunc(d.leaderboardMe)))
	mux.Handle("GET /api/v1/inventory", d.authn(http.HandlerFunc(d.inventory)))
	mux.Handle("POST /api/v1/items/{itemId}/use", d.authn(gameLimit.byUser(http.HandlerFunc(d.useItem))))
	mux.Handle("POST /api/v1/vip/daily-gift", d.authn(gameLimit.byUser(http.HandlerFunc(d.claimVIPGift))))
	mux.Handle("GET /api/v1/tasks/daily", d.authn(http.HandlerFunc(d.dailyTasks)))
	mux.Handle("POST /api/v1/tasks/daily/{taskId}/claim", d.authn(gameLimit.byUser(http.HandlerFunc(d.claimDailyTask))))

	// Realtime (read-only broadcast: world announcements)
	if d.Realtime != nil {
		mux.Handle("GET /ws", wsLimit.byIP(d.Realtime))
	}

	// Admin
	mux.Handle("POST /api/v1/admin/vip/grant", d.authn(d.admin(http.HandlerFunc(d.adminVIPGrant))))
	mux.Handle("POST /api/v1/admin/inventory/grant", d.authn(d.admin(http.HandlerFunc(d.adminInventoryGrant))))
	mux.Handle("POST /api/v1/admin/world/start", d.authn(d.admin(http.HandlerFunc(d.adminWorldStart))))

	return withRecover(d.Log, withRequestLog(d.Log, withSecurityHeaders(mux)))
}

func (d Deps) readyz(w http.ResponseWriter, r *http.Request) {
	if d.Pool == nil {
		writeJSON(w, 200, map[string]string{"status": "ok", "db": "disabled"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := d.Pool.Ping(ctx); err != nil {
		writeError(w, 503, "db_unavailable", "database not reachable")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok", "db": "ok"})
}

// ───── authentication ─────

type ctxKey int

const principalKey ctxKey = 1

func principalFrom(ctx context.Context) auth.Principal {
	p, _ := ctx.Value(principalKey).(auth.Principal)
	return p
}

func userFrom(ctx context.Context) uuid.UUID { return principalFrom(ctx).UserID }

// authn accepts `Authorization: Bearer <access JWT>`. In dev only, X-Dev-User-Id is also accepted.
func (d Deps) authn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && d.Issuer != nil {
			p, err := d.Issuer.Verify(tok)
			if err != nil {
				writeError(w, 401, "invalid_token", "access token invalid or expired")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey, p)))
			return
		}
		if d.AuthDevHeader {
			if id, err := uuid.Parse(r.Header.Get("X-Dev-User-Id")); err == nil {
				role := "player"
				if r.Header.Get("X-Dev-Role") == "admin" {
					role = "admin"
				}
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey, auth.Principal{UserID: id, Role: role})))
				return
			}
		}
		writeError(w, 401, "unauthorized", "authentication required")
	})
}

func (d Deps) admin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := principalFrom(r.Context())
		if !p.IsAdmin() {
			writeError(w, 403, "forbidden", "admin only")
			return
		}
		// The JWT role may be stale (demoted admin): re-check the current role in the DB.
		if d.Auth != nil {
			u, err := d.Auth.Me(r.Context(), p.UserID)
			if err != nil || u.Role != "admin" {
				writeError(w, 403, "forbidden", "admin only")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ───── request helpers ─────

var idemRe = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

func idempotencyKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := r.Header.Get("Idempotency-Key")
	if !idemRe.MatchString(key) {
		writeError(w, 400, "invalid_idempotency_key", "Idempotency-Key header (8-64 chars [A-Za-z0-9_-]) is required")
		return "", false
	}
	return key, true
}

const maxBody = 64 << 10

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, 400, "invalid_body", "request body must be valid JSON")
		return false
	}
	return true
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		// Our Nginx sets/appends the real peer address; only the RIGHTMOST entry is trustworthy.
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ───── response helpers ─────

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]apiError{"error": {Code: code, Message: msg}})
}

func writeErrorDetails(w http.ResponseWriter, status int, code, msg string, details any) {
	writeJSON(w, status, map[string]apiError{"error": {Code: code, Message: msg, Details: details}})
}

// internal logs the cause and returns a generic 500.
func (d Deps) internal(w http.ResponseWriter, r *http.Request, op string, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	d.Log.ErrorContext(r.Context(), op, "err", err, "path", r.URL.Path)
	writeError(w, 500, "internal", op+" failed")
}

func unavailable(w http.ResponseWriter) {
	writeError(w, 503, "unavailable", "service not configured on this server")
}

// ───── middleware ─────

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		if strings.HasPrefix(r.URL.Path, "/api/") && h.Get("Cache-Control") == "" {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func withRequestLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Debug("http", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start))
	})
}

func withRecover(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic", "recover", rec, "path", r.URL.Path)
				writeError(w, 500, "internal", "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
