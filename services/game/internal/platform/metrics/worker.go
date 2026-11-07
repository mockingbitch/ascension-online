package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Worker records the worker process: outbox health, scheduled job timings and the economy flows read from the
// ledgers. A nil *Worker is valid and records nothing.
type Worker struct {
	outboxPending   prometheus.Gauge
	outboxOldestAge prometheus.Gauge
	outboxHandled   *prometheus.CounterVec
	deadLetters     *prometheus.CounterVec
	jobDuration     *prometheus.HistogramVec
	jobErrors       *prometheus.CounterVec
	jobLastSuccess  *prometheus.GaugeVec

	stoneCreated   *prometheus.CounterVec
	stoneDestroyed *prometheus.CounterVec
	duyenEarned    *prometheus.CounterVec
	duyenRemoved   *prometheus.CounterVec

	topics  *Bounded
	reasons *Bounded
}

// NewWorker registers the worker collectors on reg.
func NewWorker(reg prometheus.Registerer) *Worker {
	cv := func(name, help string, labels ...string) *prometheus.CounterVec {
		return prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: Namespace, Name: name, Help: help}, labels)
	}
	m := &Worker{
		outboxPending: prometheus.NewGauge(prometheus.GaugeOpts{Namespace: Namespace, Name: "outbox_pending_events",
			Help: "Outbox events not yet published."}),
		outboxOldestAge: prometheus.NewGauge(prometheus.GaugeOpts{Namespace: Namespace, Name: "outbox_oldest_pending_age_seconds",
			Help: "Age of the oldest unpublished outbox event (0 when empty)."}),
		outboxHandled: cv("outbox_events_handled_total", "Outbox events marked published, by topic.", "topic"),
		deadLetters:   cv("outbox_dead_letters_total", "Outbox events dropped as permanent failures (bad payload, unknown topic, rejected reward).", "topic"),
		jobDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: Namespace, Name: "worker_job_duration_seconds",
			Help: "Duration of scheduled worker jobs.", Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60}}, []string{"job_name"}),
		jobErrors: cv("worker_job_errors_total", "Failed runs of scheduled worker jobs.", "job_name"),
		jobLastSuccess: prometheus.NewGaugeVec(prometheus.GaugeOpts{Namespace: Namespace, Name: "worker_job_last_success_timestamp_seconds",
			Help: "Unix time of the last successful run of a job."}, []string{"job_name"}),
		stoneCreated:   cv("economy_spirit_stone_created_total", "Linh thạch credited (currency_ledger delta > 0) by reason.", "reason"),
		stoneDestroyed: cv("economy_spirit_stone_destroyed_total", "Linh thạch debited (currency_ledger delta < 0) by reason.", "reason"),
		duyenEarned:    cv("economy_duyen_points_earned_total", "Điểm duyên credited (vip_point_ledger delta > 0) by source.", "source"),
		duyenRemoved:   cv("economy_duyen_points_removed_total", "Điểm duyên removed (refunds, revokes) by source.", "source"),
		topics:         NewBounded(48),
		reasons:        NewBounded(96),
	}
	reg.MustRegister(m.outboxPending, m.outboxOldestAge, m.outboxHandled, m.deadLetters, m.jobDuration, m.jobErrors,
		m.jobLastSuccess, m.stoneCreated, m.stoneDestroyed, m.duyenEarned, m.duyenRemoved)
	return m
}

// Job records one run of a scheduled job (world_tick, faction_effects, faction_ranking, reconcile, outbox_drain,
// chat_sweep, ledger_scan).
func (m *Worker) Job(job string, start time.Time, err error) {
	if m == nil {
		return
	}
	m.jobDuration.WithLabelValues(job).Observe(time.Since(start).Seconds())
	if err != nil {
		m.jobErrors.WithLabelValues(job).Inc()
		return
	}
	m.jobLastSuccess.WithLabelValues(job).SetToCurrentTime()
}

// Outbox sets the backlog gauges.
func (m *Worker) Outbox(pending int64, oldest time.Duration) {
	if m == nil {
		return
	}
	m.outboxPending.Set(float64(pending))
	m.outboxOldestAge.Set(oldest.Seconds())
}

// Handled counts an event marked published.
func (m *Worker) Handled(topic string) {
	if m != nil {
		m.outboxHandled.WithLabelValues(m.topics.Value(topic)).Inc()
	}
}

// DeadLetter counts an event dropped as a permanent failure.
func (m *Worker) DeadLetter(topic string) {
	if m != nil {
		m.deadLetters.WithLabelValues(m.topics.Value(topic)).Inc()
	}
}

// Stone adds one ledger aggregate (sum of positive or negative deltas for a reason).
func (m *Worker) Stone(reason string, created, destroyed int64) {
	if m == nil {
		return
	}
	r := m.reasons.Value(reason)
	if created > 0 {
		m.stoneCreated.WithLabelValues(r).Add(float64(created))
	}
	if destroyed > 0 {
		m.stoneDestroyed.WithLabelValues(r).Add(float64(destroyed))
	}
}

// Duyen adds one điểm duyên ledger aggregate by source.
func (m *Worker) Duyen(source string, earned, removed int64) {
	if m == nil {
		return
	}
	s := m.reasons.Value(source)
	if earned > 0 {
		m.duyenEarned.WithLabelValues(s).Add(float64(earned))
	}
	if removed > 0 {
		m.duyenRemoved.WithLabelValues(s).Add(float64(removed))
	}
}
