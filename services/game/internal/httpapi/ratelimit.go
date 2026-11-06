package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

// limitSpec: `events` per `per`, with bursts up to `burst`.
type limitSpec struct {
	name   string
	events int
	per    time.Duration
	burst  int
}

// limiter enforces a limitSpec per key. With Redis it is shared by every API instance (GCRA via
// redis_rate); if Redis is absent or errors, it falls back to an in-process token bucket so a Redis
// outage degrades to per-instance limits instead of disabling limits or failing requests.
type limiter struct {
	spec       limitSpec
	redis      *redis_rate.Limiter // nil → memory only
	mem        *memLimiter
	trustProxy bool
	log        *slog.Logger
}

func newLimiter(spec limitSpec, rdb *redis.Client, trustProxy bool, log *slog.Logger) *limiter {
	l := &limiter{spec: spec, mem: newMemLimiter(spec), trustProxy: trustProxy, log: log}
	if rdb != nil {
		l.redis = redis_rate.NewLimiter(rdb)
	}
	return l
}

func (l *limiter) allow(ctx context.Context, key string) bool {
	if l.redis != nil {
		ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
		defer cancel()
		res, err := l.redis.Allow(ctx, "ratelimit:"+l.spec.name+":"+key,
			redis_rate.Limit{Rate: l.spec.events, Period: l.spec.per, Burst: l.spec.burst})
		if err == nil {
			return res.Allowed > 0
		}
		if l.log != nil {
			l.log.Warn("redis rate limiter unavailable, using in-memory fallback", "err", err)
		}
	}
	return l.mem.allow(key)
}

func (l *limiter) byIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(r.Context(), "ip:"+clientIP(r, l.trustProxy)) {
			writeError(w, 429, "rate_limited", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// byUser must run after authn.
func (l *limiter) byUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(r.Context(), "user:"+userFrom(r.Context()).String()) {
			writeError(w, 429, "rate_limited", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ───── in-process fallback ─────

const maxBuckets = 100_000

type memLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	rps       rate.Limit
	burst     int
	lastSweep time.Time
}

type bucket struct {
	l    *rate.Limiter
	seen time.Time
}

func newMemLimiter(s limitSpec) *memLimiter {
	return &memLimiter{
		buckets: map[string]*bucket{}, rps: rate.Limit(float64(s.events) / s.per.Seconds()), burst: s.burst, lastSweep: time.Now(),
	}
}

func (l *memLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.Sub(l.lastSweep) > time.Minute { // evict idle buckets
		for k, b := range l.buckets {
			if now.Sub(b.seen) > 10*time.Minute {
				delete(l.buckets, k)
			}
		}
		l.lastSweep = now
	}
	b, ok := l.buckets[key]
	if !ok && len(l.buckets) >= maxBuckets {
		// Memory bound under key-spraying: drop all state (fails open briefly rather than OOM).
		l.buckets = map[string]*bucket{}
	}
	if !ok {
		b = &bucket{l: rate.NewLimiter(l.rps, l.burst)}
		l.buckets[key] = b
	}
	b.seen = now
	return b.l.Allow()
}
