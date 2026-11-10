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
	"net/netip"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"tiendao/game/internal/announce"
	"tiendao/game/internal/artifact"
	"tiendao/game/internal/auth"
	"tiendao/game/internal/breakthrough"
	"tiendao/game/internal/character"
	"tiendao/game/internal/chat"
	"tiendao/game/internal/content"
	"tiendao/game/internal/cosmetic"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/daolu/bond"
	"tiendao/game/internal/faction"
	"tiendao/game/internal/factionwork"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/lecture/hall"
	"tiendao/game/internal/netsig"
	"tiendao/game/internal/payment"
	"tiendao/game/internal/platform/metrics"
	"tiendao/game/internal/protector"
	"tiendao/game/internal/realtime"
	"tiendao/game/internal/report"
	"tiendao/game/internal/ritual"
	"tiendao/game/internal/secretrealm"
	"tiendao/game/internal/shop"
	"tiendao/game/internal/trade"
	"tiendao/game/internal/trial"
	"tiendao/game/internal/tutorial"
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
	Factions     *faction.Service
	Reports      *report.Service
	Chat         *chat.Service
	Shop         *shop.Service
	Rituals      *ritual.Service      // morning incense
	Fortune      *ritual.Fortune      // gieo quẻ
	Trials       *trial.Service       // mini-game trials (Phase 3)
	Cosmetics    *cosmetic.Service    // non-progress sinks: titles, frames, chat auras, cave decorations
	Tutorial     *tutorial.Service    // Vân Khê's "lần đầu" progress (display only); nil → tutorial routes answer 503
	Announce     *announce.Service    // world announcement history (GET /announcements/recent); nil → 503
	FactionWorks *factionwork.Service // cúng góp thế lực (visual faction works)
	SecretRealms *secretrealm.Service // bí cảnh thử luyện (Phase 3.4)
	Artifacts    *artifact.Service    // pháp bảo slot (Phase 3.4)
	Protectors   *protector.Service   // hộ pháp + presence (Phase 3.4; presence needs Redis)
	Payments     *payment.Service     // nạp Tiên Duyên (Phase 4); nil → payment routes answer 503
	DaoLu        *bond.Service        // đạo lữ song tu (Phase 5.1); song tu needs Redis
	Trades       *trade.Service       // trao đổi vật phẩm (Phase 5.2)
	NetSig       *netsig.Store        // salted IP-hash network signal (login/refresh); nil → not recorded
	Lectures     *hall.Service        // nil → lecture routes answer 503 (needs Redis)
	Realtime     *realtime.Hub        // nil → /ws disabled
	Redis        *redis.Client        // shared rate limits; nil → per-instance limits
	Metrics      *metrics.API         // Prometheus recorder; nil → no metrics (served on METRICS_ADDR by cmd/api)

	PublicBaseURL string
	SecureCookies bool
	AuthDevHeader bool // DEV ONLY
	PaymentDev    bool // DEV ONLY: register the sandbox "pay now / fail" endpoint (APP_ENV=dev)
	TrustProxy    bool
	// TrustedProxies are the peers whose X-Forwarded-For is honoured when TrustProxy (TRUSTED_PROXY_CIDRS, review M1).
	TrustedProxies []netip.Prefix
}

// New builds the router.
func New(d Deps) http.Handler {
	if d.Now == nil {
		d.Now = time.Now
	}
	lim := func(name string, events int, per time.Duration, burst int) *limiter {
		l := newLimiter(limitSpec{name: name, events: events, per: per, burst: burst}, d.Redis, d.proxies(), d.Log)
		l.metrics = d.Metrics
		return l
	}
	authLimit := lim("auth", 1, 5*time.Second, 10)     // per IP: burst 10, then 1 per 5s
	refreshLimit := lim("refresh", 1, time.Second, 30) // every open tab refreshes; own bucket
	gameLimit := lim("game", 2, time.Second, 20)       // per user for mutations
	wsLimit := lim("ws", 1, 2*time.Second, 10)         // per IP connection attempts
	worldLimit := lim("world", 1, 2*time.Second, 20)   // per IP: public GET /world (a few small queries)
	adminLimit := lim("admin", 1, time.Second, 10)     // per admin: a stolen token or a script bug can't sweep users at full speed
	switchLimit := gameLimit
	if d.Content != nil && d.Content.Lectures != nil { // per user, from lectures.yaml hall.switch
		lc := d.Content.Lectures
		switchLimit = lim("hallswitch", 1, lc.SwitchEvery, lc.SwitchBurst)
	}
	chatLimit := gameLimit
	if d.Content != nil && d.Content.Chat != nil { // per user, from chat.yaml
		cr := d.Content.Chat.Rate
		chatLimit = lim("chat", cr.Messages, cr.Per, cr.Burst)
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", d.readyz)

	// Public catalogue
	mux.HandleFunc("GET /api/v1/realms", d.listRealms)
	mux.HandleFunc("GET /api/v1/items", d.listItems)
	mux.HandleFunc("GET /api/v1/vip/tiers", d.listVIPTiers)
	mux.Handle("GET /api/v1/world", worldLimit.byIP(http.HandlerFunc(d.getWorld)))
	mux.Handle("GET /api/v1/factions", worldLimit.byIP(http.HandlerFunc(d.listFactions)))
	mux.Handle("GET /api/v1/cosmetics/catalog", worldLimit.byIP(http.HandlerFunc(d.cosmeticCatalog)))
	mux.Handle("GET /api/v1/announcements/recent", worldLimit.byIP(http.HandlerFunc(d.announcementsRecent)))

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
	mux.Handle("POST /api/v1/tasks/daily/incense", d.authn(gameLimit.byUser(http.HandlerFunc(d.offerIncense))))
	mux.Handle("POST /api/v1/tasks/daily/{taskId}/claim", d.authn(gameLimit.byUser(http.HandlerFunc(d.claimDailyTask))))
	mux.Handle("GET /api/v1/shop", d.authn(gameLimit.byUser(http.HandlerFunc(d.shopView))))
	mux.Handle("POST /api/v1/shop/buy", d.authn(gameLimit.byUser(http.HandlerFunc(d.shopBuy))))
	mux.Handle("GET /api/v1/fortune", d.authn(gameLimit.byUser(http.HandlerFunc(d.fortuneView))))
	mux.Handle("POST /api/v1/fortune/draw", d.authn(gameLimit.byUser(http.HandlerFunc(d.fortuneDraw))))
	mux.Handle("GET /api/v1/trials", d.authn(gameLimit.byUser(http.HandlerFunc(d.trialStatus))))
	mux.Handle("POST /api/v1/trials/{kind}/start", d.authn(gameLimit.byUser(http.HandlerFunc(d.trialStart))))
	mux.Handle("POST /api/v1/trials/{runId}/submit", d.authn(gameLimit.byUser(http.HandlerFunc(d.trialSubmit))))
	mux.Handle("GET /api/v1/cosmetics", d.authn(gameLimit.byUser(http.HandlerFunc(d.cosmeticsView))))
	mux.Handle("POST /api/v1/cosmetics/buy", d.authn(gameLimit.byUser(http.HandlerFunc(d.cosmeticsBuy))))
	mux.Handle("POST /api/v1/cosmetics/equip", d.authn(gameLimit.byUser(http.HandlerFunc(d.cosmeticsEquip))))
	mux.Handle("GET /api/v1/factions/works", d.authn(gameLimit.byUser(http.HandlerFunc(d.factionWorks))))
	mux.Handle("POST /api/v1/factions/donate", d.authn(gameLimit.byUser(http.HandlerFunc(d.factionDonate))))
	mux.Handle("GET /api/v1/secret-realms", d.authn(gameLimit.byUser(http.HandlerFunc(d.secretRealmStatus))))
	mux.Handle("POST /api/v1/secret-realms/{realmId}/start", d.authn(gameLimit.byUser(http.HandlerFunc(d.secretRealmStart))))
	mux.Handle("POST /api/v1/secret-realms/runs/{runId}/submit", d.authn(gameLimit.byUser(http.HandlerFunc(d.secretRealmSubmit))))
	mux.Handle("GET /api/v1/artifacts", d.authn(gameLimit.byUser(http.HandlerFunc(d.artifactView))))
	mux.Handle("POST /api/v1/artifacts/equip", d.authn(gameLimit.byUser(http.HandlerFunc(d.artifactEquip))))
	mux.Handle("GET /api/v1/protector", d.authn(gameLimit.byUser(http.HandlerFunc(d.protectorState))))
	mux.Handle("POST /api/v1/protector/invite", d.authn(gameLimit.byUser(http.HandlerFunc(d.protectorInvite))))
	mux.Handle("POST /api/v1/protector/invites/{inviteId}/accept", d.authn(gameLimit.byUser(http.HandlerFunc(d.protectorAccept))))
	mux.Handle("POST /api/v1/presence/ping", d.authn(gameLimit.byUser(http.HandlerFunc(d.presencePing))))
	mux.Handle("GET /api/v1/admin/protector/pairs", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminProtectorPairs)))))
	mux.Handle("POST /api/v1/reports", d.authn(gameLimit.byUser(http.HandlerFunc(d.submitReport))))
	mux.Handle("GET /api/v1/chat/{channel}/messages", d.authn(gameLimit.byUser(http.HandlerFunc(d.chatHistory))))
	mux.Handle("POST /api/v1/chat/{channel}/messages", d.authn(chatLimit.byUser(http.HandlerFunc(d.chatSend))))

	mux.Handle("GET /api/v1/factions/me", d.authn(gameLimit.byUser(http.HandlerFunc(d.factionMe))))
	mux.Handle("GET /api/v1/factions/{factionId}/members", d.authn(gameLimit.byUser(http.HandlerFunc(d.factionMembers))))
	mux.Handle("POST /api/v1/factions/join", d.authn(gameLimit.byUser(d.factionChange("join"))))
	mux.Handle("POST /api/v1/factions/switch", d.authn(gameLimit.byUser(d.factionChange("switch"))))
	mux.Handle("POST /api/v1/factions/split-choice", d.authn(gameLimit.byUser(d.factionChange("split_choice"))))

	mux.Handle("GET /api/v1/tutorial/script", worldLimit.byIP(http.HandlerFunc(d.tutorialScript)))
	mux.Handle("GET /api/v1/tutorial", d.authn(gameLimit.byUser(http.HandlerFunc(d.tutorialState))))
	mux.Handle("POST /api/v1/tutorial/steps/{stepId}/done", d.authn(gameLimit.byUser(d.tutorialMark("done"))))
	mux.Handle("POST /api/v1/tutorial/steps/{stepId}/skip", d.authn(gameLimit.byUser(d.tutorialMark("skipped"))))
	mux.Handle("POST /api/v1/tutorial/skip-all", d.authn(gameLimit.byUser(http.HandlerFunc(d.tutorialSkipAll))))
	mux.Handle("POST /api/v1/tutorial/reset", d.authn(gameLimit.byUser(http.HandlerFunc(d.tutorialReset))))

	mux.Handle("GET /api/v1/lecture/scripts", http.HandlerFunc(d.lectureScripts))
	mux.Handle("GET /api/v1/lecture/state", d.authn(gameLimit.byUser(http.HandlerFunc(d.lectureState))))
	mux.Handle("GET /api/v1/lecture/notebook", d.authn(gameLimit.byUser(http.HandlerFunc(d.lectureNotebook))))
	mux.Handle("POST /api/v1/lecture/enter", d.authn(gameLimit.byUser(http.HandlerFunc(d.lectureEnter))))
	mux.Handle("POST /api/v1/lecture/hall", d.authn(gameLimit.byUser(switchLimit.byUser(http.HandlerFunc(d.lectureSwitch)))))
	mux.Handle("POST /api/v1/lecture/leave", d.authn(gameLimit.byUser(http.HandlerFunc(d.lectureLeave))))
	mux.Handle("POST /api/v1/lecture/heartbeat", d.authn(gameLimit.byUser(http.HandlerFunc(d.lectureBeat))))
	mux.Handle("POST /api/v1/lecture/insight", d.authn(gameLimit.byUser(http.HandlerFunc(d.lectureInsight))))

	// Realtime (read-only broadcast: world announcements)
	if d.Realtime != nil {
		mux.Handle("GET /ws", wsLimit.byIP(d.Realtime))
	}

	// Admin
	mux.Handle("POST /api/v1/admin/vip/grant", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminVIPGrant)))))
	mux.Handle("POST /api/v1/admin/vip/refund", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminVIPRefund)))))
	mux.Handle("POST /api/v1/admin/inventory/grant", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminInventoryGrant)))))
	mux.Handle("POST /api/v1/admin/world/start", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminWorldStart)))))
	mux.Handle("GET /api/v1/admin/world", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminWorld)))))
	mux.Handle("PUT /api/v1/admin/world/milestones/{milestoneId}", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminWorldOverride)))))
	mux.Handle("POST /api/v1/admin/world/pause", d.authn(d.admin(adminLimit.byUser(d.adminWorldPause(true)))))
	mux.Handle("POST /api/v1/admin/world/resume", d.authn(d.admin(adminLimit.byUser(d.adminWorldPause(false)))))
	mux.Handle("POST /api/v1/admin/factions/lock", d.authn(d.admin(adminLimit.byUser(d.adminFactionLock(true)))))
	mux.Handle("POST /api/v1/admin/factions/unlock", d.authn(d.admin(adminLimit.byUser(d.adminFactionLock(false)))))
	mux.Handle("GET /api/v1/admin/reports", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminListReports)))))
	mux.Handle("POST /api/v1/admin/reports/{reportId}/resolve", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminResolveReport)))))
	mux.Handle("POST /api/v1/admin/chat/messages/{messageId}/hide", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminChatHide)))))
	mux.Handle("POST /api/v1/admin/chat/mutes", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminChatMute)))))

	// Payments (nạp Tiên Duyên, Phase 4) — payment_handlers.go
	payLimit := lim("payment", 1, 10*time.Second, 5)  // per user: order creation
	hookLimit := lim("webhook", 20, time.Second, 200) // per IP: providers send from a few addresses
	mux.Handle("GET /api/v1/payments/packages", worldLimit.byIP(http.HandlerFunc(d.paymentPackages)))
	mux.Handle("POST /api/v1/payments/orders", d.authn(payLimit.byUser(http.HandlerFunc(d.paymentCreateOrder))))
	mux.Handle("GET /api/v1/payments/orders", d.authn(gameLimit.byUser(http.HandlerFunc(d.paymentListOrders))))
	mux.Handle("GET /api/v1/payments/orders/{orderId}", d.authn(gameLimit.byUser(http.HandlerFunc(d.paymentGetOrder))))
	mux.Handle("POST /api/v1/payments/webhook/{provider}", hookLimit.byIP(http.HandlerFunc(d.paymentWebhook)))
	mux.Handle("GET /api/v1/admin/payments/orders", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminPaymentOrders)))))
	mux.Handle("GET /api/v1/admin/payments/orders/{orderId}", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminPaymentOrder)))))
	mux.Handle("POST /api/v1/admin/payments/orders/{orderId}/refund", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminPaymentRefund)))))
	mux.Handle("GET /api/v1/admin/payments/alerts", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminPaymentAlerts)))))
	if d.PaymentDev {
		mux.Handle("POST /api/v1/dev/payments/sandbox/{orderId}", d.authn(gameLimit.byUser(http.HandlerFunc(d.devSandboxComplete))))
	}

	// Đạo lữ song tu (Phase 5.1) + network signal admin — daolu_handlers.go
	bondLimit := lim("daolu", 1, 3*time.Second, 6) // per user: propose/accept/decline/cancel
	mux.Handle("GET /api/v1/dao-lu", d.authn(gameLimit.byUser(http.HandlerFunc(d.daoLuState))))
	mux.Handle("POST /api/v1/dao-lu/proposals", d.authn(bondLimit.byUser(http.HandlerFunc(d.daoLuPropose))))
	mux.Handle("POST /api/v1/dao-lu/proposals/{proposalId}/accept", d.authn(bondLimit.byUser(d.daoLuRespond("accept"))))
	mux.Handle("POST /api/v1/dao-lu/proposals/{proposalId}/decline", d.authn(bondLimit.byUser(d.daoLuRespond("decline"))))
	mux.Handle("POST /api/v1/dao-lu/proposals/{proposalId}/cancel", d.authn(bondLimit.byUser(d.daoLuRespond("cancel"))))
	mux.Handle("GET /api/v1/dao-lu/song-tu", d.authn(gameLimit.byUser(d.daoLuSongTu("state"))))
	mux.Handle("POST /api/v1/dao-lu/song-tu/start", d.authn(gameLimit.byUser(d.daoLuSongTu("start"))))
	mux.Handle("POST "+songTuHeartbeatPath, d.authn(gameLimit.byUser(d.daoLuSongTu("heartbeat"))))
	mux.Handle("POST /api/v1/dao-lu/song-tu/claim", d.authn(gameLimit.byUser(http.HandlerFunc(d.daoLuClaim))))
	mux.Handle("POST /api/v1/admin/dao-lu/bonds/{bondId}/terminate", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminDaoLuTerminate)))))
	mux.Handle("GET /api/v1/admin/dao-lu/flags", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminDaoLuFlags)))))
	mux.Handle("GET /api/v1/admin/netsig/clusters", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminNetClusters)))))

	// Trao đổi vật phẩm (Phase 5.2) — trade_handlers.go
	tradeLimit := lim("trade", 1, 3*time.Second, 6) // per user: create/accept/decline/cancel
	mux.Handle("GET /api/v1/trades", d.authn(gameLimit.byUser(http.HandlerFunc(d.tradeState))))
	mux.Handle("GET /api/v1/trades/history", d.authn(gameLimit.byUser(http.HandlerFunc(d.tradeHistory))))
	mux.Handle("GET /api/v1/trades/peer/{characterId}", d.authn(gameLimit.byUser(http.HandlerFunc(d.tradePeer))))
	mux.Handle("POST /api/v1/trades", d.authn(tradeLimit.byUser(http.HandlerFunc(d.tradeCreate))))
	mux.Handle("POST /api/v1/trades/{offerId}/accept", d.authn(tradeLimit.byUser(d.tradeRespond("accept"))))
	mux.Handle("POST /api/v1/trades/{offerId}/decline", d.authn(tradeLimit.byUser(d.tradeRespond("decline"))))
	mux.Handle("POST /api/v1/trades/{offerId}/cancel", d.authn(tradeLimit.byUser(d.tradeRespond("cancel"))))
	mux.Handle("GET /api/v1/admin/trades/flows", d.authn(d.admin(adminLimit.byUser(http.HandlerFunc(d.adminTradeFlows)))))

	return d.Metrics.Instrument(withRecover(d.Log, withRequestLog(d.Log, withSecurityHeaders(mux))))
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
	return decodeJSONLimit(w, r, v, maxBody)
}

// decodeJSONLimit is decodeJSON with a route-specific body limit (e.g. trial submissions carry an input log).
func decodeJSONLimit(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	dec := json.NewDecoder(io.LimitReader(r.Body, limit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, 400, "invalid_body", "request body must be valid JSON")
		return false
	}
	return true
}

// clientIP is the address used for per-IP limits and the netsig hash. X-Forwarded-For is honoured only when the
// direct peer (RemoteAddr) is one of the trusted proxies (TRUST_PROXY + TRUSTED_PROXY_CIDRS, review M1) — anyone else
// could spoof it. Our Nginx sets XFF to the real client ($remote_addr after Cloudflare real-IP); only the RIGHTMOST
// entry is trustworthy.
func clientIP(r *http.Request, proxies []netip.Prefix) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if len(proxies) > 0 {
		if peer, err := netip.ParseAddr(host); err == nil && inPrefixes(peer.Unmap(), proxies) {
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				parts := strings.Split(xff, ",")
				return strings.TrimSpace(parts[len(parts)-1])
			}
		}
	}
	return host
}

func inPrefixes(a netip.Addr, ps []netip.Prefix) bool {
	for _, p := range ps {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// proxies is the trusted-proxy list in effect (nil = XFF never trusted).
func (d Deps) proxies() []netip.Prefix {
	if !d.TrustProxy {
		return nil
	}
	return d.TrustedProxies
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
