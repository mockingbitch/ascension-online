package metrics

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// API records the api process: HTTP traffic, rate limits and the game activity seen at the handler boundary
// (only fresh, committed results — idempotent replays are counted separately or not at all).
// A nil *API is valid and records nothing.
type API struct {
	requests    *prometheus.CounterVec
	duration    *prometheus.HistogramVec
	inflight    prometheus.Gauge
	rateLimited *prometheus.CounterVec

	settle       *prometheus.CounterVec
	breakthrough *prometheus.CounterVec
	trials       *prometheus.CounterVec
	secretRealms *prometheus.CounterVec
	ngoDao       prometheus.Counter
	ngoDaoMilli  prometheus.Counter
	purchases    *prometheus.CounterVec
	chat         *prometheus.CounterVec

	labels *Bounded // catalogue ids used as labels (trial kind, realm, channel)
}

// LatencyBuckets are tuned around the GDD §26 targets (p95 < 200 ms REST, < 300 ms leaderboard).
var LatencyBuckets = []float64{.005, .01, .025, .05, .1, .2, .3, .5, 1, 2.5, 5}

// NewAPI registers the api collectors on reg.
func NewAPI(reg prometheus.Registerer) *API {
	cv := func(name, help string, labels ...string) *prometheus.CounterVec {
		return prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: Namespace, Name: name, Help: help}, labels)
	}
	m := &API{
		requests: cv("http_requests_total", "HTTP requests by route pattern and status class.", "route", "code"),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: Namespace, Name: "http_request_duration_seconds",
			Help: "HTTP latency by route pattern (WebSocket upgrades excluded).", Buckets: LatencyBuckets}, []string{"route"}),
		inflight: prometheus.NewGauge(prometheus.GaugeOpts{Namespace: Namespace, Name: "http_inflight_requests",
			Help: "Requests being served."}),
		rateLimited:  cv("rate_limited_total", "Requests rejected with 429 by limiter name.", "limiter"),
		settle:       cv("game_settle_total", "Cultivation settlements (result: settled|replayed).", "result"),
		breakthrough: cv("game_breakthrough_attempts_total", "Breakthrough attempts by result and tribulation.", "result", "tribulation"),
		trials:       cv("game_trial_verdicts_total", "Mini-game trial verdicts (fresh submissions).", "kind", "verdict"),
		secretRealms: cv("game_secret_realm_verdicts_total", "Bí cảnh verdicts (fresh submissions).", "realm", "verdict"),
		ngoDao:       prometheus.NewCounter(prometheus.CounterOpts{Namespace: Namespace, Name: "game_ngo_dao_payouts_total", Help: "Ngộ đạo (lecture insight) payouts."}),
		ngoDaoMilli: prometheus.NewCounter(prometheus.CounterOpts{Namespace: Namespace, Name: "game_ngo_dao_cultivation_milli_total",
			Help: "Cultivation (milli) paid by ngộ đạo."}),
		purchases: cv("game_shop_purchases_total", "Fresh purchases by shop (shop|cosmetic).", "shop"),
		chat:      cv("game_chat_messages_total", "Chat messages accepted by channel.", "channel"),
		labels:    NewBounded(64),
	}
	reg.MustRegister(m.requests, m.duration, m.inflight, m.rateLimited, m.settle, m.breakthrough, m.trials, m.secretRealms,
		m.ngoDao, m.ngoDaoMilli, m.purchases, m.chat)
	return m
}

// RegisterWSClients exports the connected WebSocket client count.
func RegisterWSClients(reg prometheus.Registerer, count func() int) {
	reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Namespace: Namespace, Name: "ws_connected_clients",
		Help: "WebSocket clients connected to this api instance."}, func() float64 { return float64(count()) }))
}

// Instrument wraps the router: counts and times every request by its matched ServeMux pattern ("unmatched" for 404s
// outside the router). The pattern is read after the mux ran (it sets Request.Pattern on the request it received).
func (m *API) Instrument(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m.inflight.Inc()
		sw := &statusWriter{ResponseWriter: w}
		defer func() {
			m.inflight.Dec()
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			code := sw.status
			if code == 0 {
				code = http.StatusOK
			}
			m.requests.WithLabelValues(route, strconv.Itoa(code/100)+"xx").Inc()
			if !sw.hijacked {
				m.duration.WithLabelValues(route).Observe(time.Since(start).Seconds())
			}
		}()
		next.ServeHTTP(sw, r)
	})
}

// RateLimited counts a 429 from the named limiter.
func (m *API) RateLimited(limiter string) {
	if m != nil {
		m.rateLimited.WithLabelValues(limiter).Inc()
	}
}

// Settle counts a settlement.
func (m *API) Settle(replayed bool) {
	if m != nil {
		m.settle.WithLabelValues(pick(replayed, "replayed", "settled")).Inc()
	}
}

// Breakthrough counts an attempt result.
func (m *API) Breakthrough(success, tribulation, replayed bool) {
	if m == nil {
		return
	}
	res := pick(success, "success", "fail")
	if replayed {
		res = "replayed"
	}
	m.breakthrough.WithLabelValues(res, strconv.FormatBool(tribulation)).Inc()
}

// TrialVerdict counts a fresh trial verdict.
func (m *API) TrialVerdict(kind, verdict string, replayed bool) {
	if m != nil && !replayed {
		m.trials.WithLabelValues(m.labels.Value(kind), m.labels.Value(verdict)).Inc()
	}
}

// SecretRealmVerdict counts a fresh bí cảnh verdict.
func (m *API) SecretRealmVerdict(realm, verdict string, replayed bool) {
	if m != nil && !replayed {
		m.secretRealms.WithLabelValues(m.labels.Value(realm), m.labels.Value(verdict)).Inc()
	}
}

// NgoDao counts a fresh ngộ đạo payout.
func (m *API) NgoDao(earnedMilli int64, replayed bool) {
	if m != nil && !replayed {
		m.ngoDao.Inc()
		if earnedMilli > 0 {
			m.ngoDaoMilli.Add(float64(earnedMilli))
		}
	}
}

// Purchase counts a fresh purchase from shop "shop" or "cosmetic".
func (m *API) Purchase(shop string, replayed bool) {
	if m != nil && !replayed {
		m.purchases.WithLabelValues(shop).Inc()
	}
}

// ChatMessage counts an accepted chat message.
func (m *API) ChatMessage(channel string) {
	if m != nil {
		m.chat.WithLabelValues(m.labels.Value(channel)).Inc()
	}
}

func pick(c bool, a, b string) string {
	if c {
		return a
	}
	return b
}

// statusWriter captures the status code; it forwards Flush/Hijack so streaming and WebSocket upgrades keep working.
type statusWriter struct {
	http.ResponseWriter
	status   int
	hijacked bool
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("metrics: hijack not supported")
	}
	c, rw, err := h.Hijack()
	if err == nil {
		w.hijacked = true
		if w.status == 0 {
			w.status = http.StatusSwitchingProtocols
		}
	}
	return c, rw, err
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
