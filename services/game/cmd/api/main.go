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
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/uuid"

	"tiendao/game/internal/announce"
	"tiendao/game/internal/artifact"
	"tiendao/game/internal/auth"
	"tiendao/game/internal/breakthrough"
	"tiendao/game/internal/character"
	"tiendao/game/internal/chat"
	"tiendao/game/internal/config"
	"tiendao/game/internal/content"
	"tiendao/game/internal/cosmetic"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/dailytask"
	"tiendao/game/internal/daolu/bond"
	"tiendao/game/internal/faction"
	"tiendao/game/internal/factionwork"
	"tiendao/game/internal/httpapi"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/lecture"
	"tiendao/game/internal/lecture/hall"
	"tiendao/game/internal/netsig"
	"tiendao/game/internal/payment"
	"tiendao/game/internal/platform/metrics"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/platform/redisx"
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
	if deps.TrustedProxies, err = cfg.TrustedProxies(); err != nil { // XFF honoured only from these peers (review M1)
		return err
	}
	// Prometheus: own registry, served on an internal listener (METRICS_ADDR, default :9090; "off" disables) — never
	// on the public router, and nginx denies /metrics as well.
	reg := metrics.NewRegistry()
	deps.Metrics = metrics.NewAPI(reg)
	metricsAddr := os.Getenv("METRICS_ADDR")
	if metricsAddr == "" {
		metricsAddr = ":9090"
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
		metrics.RegisterPool(reg, pool)
		cult := &cultivation.Service{Pool: pool, Realms: cb.Realms, Contribution: cb.Contribution, VIP: cb.VIP, Tasks: cb.DailyTasks, Now: time.Now}
		deps.Pool = pool
		deps.Cultivation = cult
		deps.Auth = &auth.Service{Pool: pool, Issuer: issuer, RefreshTTL: cfg.RefreshTokenTTL, Now: time.Now}
		deps.Characters = &character.Service{Pool: pool, Cultivation: cult, Names: cb.Moderation, Cosmetics: cb.Cosmetics, VIP: cb.VIP,
			Looks: cb.Appearances}
		deps.Chat = &chat.Service{Pool: pool, Cat: cb.Chat, Realms: cb.Realms, Mod: cb.Moderation, Now: time.Now,
			NameFx: cb.Cosmetics.NameFxFilter(cb.VIP.TierForPoints), Looks: cb.Appearances}
		deps.Reports = &report.Service{Pool: pool, Cat: cb.Moderation, Now: time.Now}
		deps.Protectors = &protector.Service{Pool: pool, Cfg: cb.Protector, Realms: cb.Realms, Project: cult.ProjectedStage, Now: time.Now, Log: log}
		deps.Breakthrough = &breakthrough.Service{Pool: pool, Cultivation: cult, RNG: breakthrough.NewRNG(), Trials: cb.Trials, Items: cb.Items,
			Protectors: deps.Protectors}
		deps.Protectors.Ready = func(ctx context.Context, userID uuid.UUID) (bool, error) {
			pv, err := deps.Breakthrough.Preview(ctx, userID)
			return pv.Ready && pv.Tribulation != nil, err
		}
		deps.SecretRealms = &secretrealm.Service{Pool: pool, Cat: cb.SecretRealms, Realms: cb.Realms, Project: cult.ProjectedStage, VIP: cb.VIP, Now: time.Now}
		deps.Artifacts = &artifact.Service{Pool: pool, Items: cb.Items, Now: time.Now}
		deps.Trials = &trial.Service{Pool: pool, Cat: cb.Trials, Realms: cb.Realms, Project: cult.ProjectedStage, VIP: cb.VIP, Now: time.Now}
		deps.VIP = &vip.Service{Pool: pool, Table: cb.VIP, Now: time.Now}
		// Nạp Tiên Duyên (Phase 4): providers enabled for APP_ENV; the sandbox "pay now" endpoint only in dev.
		pc := config.LoadPayment()
		provs, err := payment.BuildProviders(cb.Payments, cfg.Env, []byte(pc.SandboxSecret), cfg.PublicBaseURL+"/tien-duyen/sandbox/", pool, time.Now)
		if err != nil {
			return err
		}
		deps.Payments = &payment.Service{Pool: pool, Cat: cb.Payments, VIP: cb.VIP, Providers: provs,
			ReturnURL: cfg.PublicBaseURL + "/tien-duyen?order=", Log: log, Now: time.Now}
		deps.PaymentDev = cfg.IsDev()
		// Đạo lữ (Phase 5.1) + salted IP-hash network signal. Dev without NETSIG_SALTS uses a fixed, NON-secret salt.
		netKeys, err := netsig.ParseKeys(cfg.NetSigSalts, netsig.MinSecretLen)
		if err != nil && cfg.NetSigSalts == "" && cfg.IsDev() {
			netKeys, err = []netsig.Key{{ID: "dev", Secret: []byte("dev-only-netsig-salt-not-a-secret-0000")}}, nil
			log.Warn("NETSIG_SALTS not set: using a fixed dev-only salt")
		}
		if err != nil {
			return err
		}
		deps.NetSig = &netsig.Store{Pool: pool, Hasher: &netsig.Hasher{Keys: netKeys}, Now: time.Now}
		deps.Protectors.Net = deps.NetSig // hộ pháp pairs on a shared network → admin flag (review M2)
		deps.DaoLu = &bond.Service{Pool: pool, Cfg: cb.DaoLu, Realms: cb.Realms, Cultivation: cult, VIP: cb.VIP, Net: deps.NetSig, Now: time.Now, Log: log}
		deps.Trades = &trade.Service{Pool: pool, Cfg: cb.Trade, Items: cb.Items, Realms: cb.Realms, Net: deps.NetSig, Now: time.Now, Log: log} // Phase 5.2
		deps.DailyTasks = &dailytask.Service{Pool: pool, Cat: cb.DailyTasks, VIP: cb.VIP, Now: time.Now}
		deps.Shop = &shop.Service{Pool: pool, Cat: cb.Shop, Items: cb.Items, VIP: cb.VIP, Now: time.Now}
		deps.Cosmetics = &cosmetic.Service{Pool: pool, Cat: cb.Cosmetics, VIP: cb.VIP, Eras: cb.World, Now: time.Now,
			Looks: cb.Appearances, Realms: cb.Realms}
		deps.FactionWorks = &factionwork.Service{Pool: pool, Cat: cb.FactionWorks, Now: time.Now}
		// Vân Khê's tutorial (content/story/tutorial; numbers from realms.yaml): display-only progress
		tut, err := tutorial.Load(filepath.Join(cfg.StoryDir, "tutorial"), tutorial.Vars(cb.Realms))
		if err != nil {
			return err
		}
		deps.Tutorial = &tutorial.Service{Pool: pool, Script: tut, Now: time.Now}
		deps.Announce = &announce.Service{Pool: pool, Realms: cb.Realms, Cfg: cb.Announce, Now: time.Now, Log: log} // history only
		deps.Rituals = &ritual.Service{Pool: pool, Tasks: cb.DailyTasks, Cultivation: cult, VIP: cb.VIP, Now: time.Now}
		deps.Fortune = &ritual.Fortune{Pool: pool, Cat: cb.Fortunes, Cultivation: cult, RNG: breakthrough.NewRNG(), Now: time.Now}
		deps.World = &world.Service{Pool: pool, Realms: cb.Realms, Cat: cb.World, Contribution: cb.Contribution, Now: time.Now}
		deps.Factions = &faction.Service{Pool: pool, Cat: cb.Factions, Realms: cb.Realms, World: cb.World, Now: time.Now,
			Settle: cult.SettleLockedRow}
		deps.Characters.Factions = deps.Factions // onboarding: POST /character {name, factionId, path} (§9.6)

		if cfg.RedisURL != "" {
			rdb, err := redisx.Connect(ctx, cfg.RedisURL)
			if err != nil {
				return err
			}
			defer rdb.Close()
			metrics.RegisterRedis(reg, rdb)
			deps.Leaderboard = &leaderboard.Service{Pool: pool, Redis: rdb, Realms: cb.Realms, Look: cb.Cosmetics.BoardFilter(cb.VIP.TierForPoints),
				Looks: cb.Appearances}
			deps.Redis = rdb
			deps.Protectors.Presence = &protector.RedisPresence{R: rdb, TTL: cb.Protector.PresenceTTL(), Activity: cb.Protector.ActivityWindow()} // hộ pháp "online"
			deps.DaoLu.Presence = deps.Protectors.Presence
			deps.DaoLu.Session = &bond.Session{R: rdb, Cfg: cb.DaoLu} // song tu overlap accrual
			deps.Factions.Ranking = &faction.Ranking{Pool: pool, Redis: rdb, Realms: cb.Realms, Cat: cb.Factions,
				Look: cb.Cosmetics.BoardFilter(cb.VIP.TierForPoints), Looks: cb.Appearances}
			deps.Chat.Publish = func(ctx context.Context, topic, typ, id string, data any) error {
				return realtime.PublishTopic(ctx, rdb, topic, realtime.Envelope{Type: typ, ID: id, TS: time.Now().UTC(), Data: data})
			}
			scripts, err := lecture.LoadScripts(filepath.Join(cfg.StoryDir, "lectures"))
			if err != nil {
				return err
			}
			deps.Lectures = &hall.Service{Pool: pool, Hall: &hall.Hall{R: rdb, Cfg: cb.Lectures}, Cfg: cb.Lectures, Scripts: scripts,
				Cultivation: cult, VIP: cb.VIP, Now: time.Now, Log: log, Publish: hall.Publisher(deps.Chat.Publish),
				Looks: cb.Appearances}
			// Hall chat is per điện (decisions 2026-11-10): stored with the sender's điện, delivered on its topic.
			deps.Chat.HallChannel, deps.Chat.HallOf, deps.Chat.HallTopic = cb.Lectures.ChatChannel, deps.Lectures.HallOf, hall.Topic
			origin, err := url.Parse(cfg.PublicBaseURL)
			if err != nil {
				return err
			}
			hub := realtime.NewHub(log, []string{origin.Host}, cfg.WSMaxClients)
			hub.TopicOK = func(t string) bool { return hall.ValidTopic(cb.Lectures, t) }
			go hub.Subscribe(ctx, rdb)
			go deps.Lectures.RunPresence(ctx, hub) // one loop per instance: sweep, merge, lecture.count
			metrics.RegisterWSClients(reg, hub.Count)
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
	metrics.Serve(ctx, metricsAddr, reg, log)
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
