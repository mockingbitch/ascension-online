// Package metrics holds the Prometheus collectors of the api and worker processes (GDD §25 "metrics and structured
// logs", decisions 2026-11-07). Each process builds its own registry in cmd/* and injects the typed recorder; there is
// no package-level state and a nil recorder records nothing (tests, content-only mode).
//
// Cardinality rules: labels are route PATTERNS (never raw paths), status classes, catalogue/code-defined enums
// (limiter name, verdict, chat channel, ledger reason/source) — never user, character, IP or free text.
// Every free-form label value goes through Bounded, which folds unexpected or overflowing values into "other".
package metrics

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

// Namespace prefixes every metric of the game services.
const Namespace = "tiendao"

// NewRegistry returns a registry with the Go runtime and process collectors.
func NewRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return reg
}

// Handler serves reg in the Prometheus text format.
func Handler(reg *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg, MaxRequestsInFlight: 4, Timeout: 10 * time.Second})
}

// Serve runs an internal HTTP listener with GET /metrics until ctx is done. The address must not be routed by the
// public reverse proxy (docker-compose publishes it on the internal network only; nginx also denies /metrics).
func Serve(ctx context.Context, addr string, reg *prometheus.Registry, log *slog.Logger) {
	if addr == "" || addr == "off" {
		return
	}
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", Handler(reg))
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 15 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	go func() {
		log.Info("metrics listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("metrics listener", "err", err)
		}
	}()
}

// ───── bounded label values ─────

var labelRe = regexp.MustCompile(`^[a-z0-9_.:-]{1,48}$`)

// Bounded keeps at most max distinct label values; anything unexpected or past the bound becomes "other".
type Bounded struct {
	mu   sync.Mutex
	seen map[string]struct{}
	max  int
}

// NewBounded allows up to max distinct values.
func NewBounded(max int) *Bounded { return &Bounded{seen: map[string]struct{}{}, max: max} }

// Value returns v when allowed, else "other".
func (b *Bounded) Value(v string) string {
	if !labelRe.MatchString(v) {
		return "other"
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.seen[v]; ok {
		return v
	}
	if len(b.seen) >= b.max {
		return "other"
	}
	b.seen[v] = struct{}{}
	return v
}

// ───── shared collectors ─────

// RegisterPool exports pgxpool statistics (acquired/idle/total conns, acquire wait) as gauges/counters.
func RegisterPool(reg prometheus.Registerer, pool *pgxpool.Pool) {
	if pool == nil {
		return
	}
	reg.MustRegister(&poolCollector{pool: pool})
}

type poolCollector struct{ pool *pgxpool.Pool }

var (
	poolAcquired = prometheus.NewDesc(Namespace+"_db_pool_acquired_conns", "Connections currently checked out.", nil, nil)
	poolIdle     = prometheus.NewDesc(Namespace+"_db_pool_idle_conns", "Idle connections.", nil, nil)
	poolTotal    = prometheus.NewDesc(Namespace+"_db_pool_total_conns", "Open connections.", nil, nil)
	poolMax      = prometheus.NewDesc(Namespace+"_db_pool_max_conns", "Pool size limit.", nil, nil)
	poolWaits    = prometheus.NewDesc(Namespace+"_db_pool_acquire_waits_total", "Acquires that had to wait for a connection.", nil, nil)
	poolWaitSec  = prometheus.NewDesc(Namespace+"_db_pool_acquire_wait_seconds_total", "Total time spent waiting for a connection.", nil, nil)
	poolUp       = prometheus.NewDesc(Namespace+"_db_up", "1 when a ping to PostgreSQL succeeds at scrape time.", nil, nil)
)

func (c *poolCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{poolAcquired, poolIdle, poolTotal, poolMax, poolWaits, poolWaitSec, poolUp} {
		ch <- d
	}
}

func (c *poolCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.pool.Stat()
	ch <- prometheus.MustNewConstMetric(poolAcquired, prometheus.GaugeValue, float64(s.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(poolIdle, prometheus.GaugeValue, float64(s.IdleConns()))
	ch <- prometheus.MustNewConstMetric(poolTotal, prometheus.GaugeValue, float64(s.TotalConns()))
	ch <- prometheus.MustNewConstMetric(poolMax, prometheus.GaugeValue, float64(s.MaxConns()))
	ch <- prometheus.MustNewConstMetric(poolWaits, prometheus.CounterValue, float64(s.EmptyAcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolWaitSec, prometheus.CounterValue, s.AcquireDuration().Seconds())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	up := 1.0
	if err := c.pool.Ping(ctx); err != nil {
		up = 0
	}
	ch <- prometheus.MustNewConstMetric(poolUp, prometheus.GaugeValue, up)
}

// RedisErrors counts failed Redis commands (redis.Nil is not an error) and reports reachability.
type RedisErrors struct {
	errs *prometheus.CounterVec
}

// RegisterRedis adds a hook to rdb counting errors by kind (dial|command|pipeline) and a tiendao_redis_up gauge.
func RegisterRedis(reg prometheus.Registerer, rdb *redis.Client) {
	if rdb == nil {
		return
	}
	e := &RedisErrors{errs: prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: Namespace, Name: "redis_errors_total",
		Help: "Redis calls that failed (redis.Nil excluded)."}, []string{"kind"})}
	reg.MustRegister(e.errs, prometheus.NewGaugeFunc(prometheus.GaugeOpts{Namespace: Namespace, Name: "redis_up",
		Help: "1 when a PING to Redis succeeds at scrape time."}, func() float64 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if rdb.Ping(ctx).Err() != nil {
			return 0
		}
		return 1
	}))
	rdb.AddHook(e)
}

func (e *RedisErrors) count(kind string, err error) {
	if err != nil && !errors.Is(err, redis.Nil) && !errors.Is(err, context.Canceled) {
		e.errs.WithLabelValues(kind).Inc()
	}
}

// DialHook implements redis.Hook.
func (e *RedisErrors) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		c, err := next(ctx, network, addr)
		e.count("dial", err)
		return c, err
	}
}

// ProcessHook implements redis.Hook.
func (e *RedisErrors) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		err := next(ctx, cmd)
		e.count("command", err)
		return err
	}
}

// ProcessPipelineHook implements redis.Hook.
func (e *RedisErrors) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		err := next(ctx, cmds)
		e.count("pipeline", err)
		return err
	}
}
