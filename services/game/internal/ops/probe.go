// Package ops holds operator tooling that is not gameplay: worker health probes (outbox backlog, ledger flow
// aggregation for metrics) and the closed-beta reset used by cmd/opsctl.
//
// Queries here are raw SQL on purpose: they are operational (TRUNCATE, catalog lookups, aggregate scans over the
// ledgers) rather than game rules, and keeping them out of sqlc keeps internal/db purely gameplay.
package ops

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OutboxBacklog returns the number of unpublished outbox events and the age of the oldest one (partial index
// outbox_events_pending_idx keeps this cheap).
func OutboxBacklog(ctx context.Context, pool *pgxpool.Pool) (int64, time.Duration, error) {
	var n int64
	var ageSec float64
	err := pool.QueryRow(ctx, `SELECT count(*), COALESCE(EXTRACT(EPOCH FROM now() - min(created_at)), 0)::float8
		FROM outbox_events WHERE published_at IS NULL`).Scan(&n, &ageSec)
	return n, time.Duration(ageSec * float64(time.Second)), err
}

// FlowSink receives ledger aggregates (metrics.Worker implements it).
type FlowSink interface {
	Stone(reason string, created, destroyed int64)
	Duyen(source string, earned, removed int64)
}

// LedgerScanner turns new ledger rows into flow counters incrementally (id cursor per ledger). The first Scan only
// positions the cursors at the current end (counters start at worker start, like any Prometheus counter).
//
// Rows younger than Lag are left for the next scan: BIGSERIAL ids are assigned at INSERT but become visible at
// COMMIT, so a lower id can appear after a higher one; game transactions are short, so a lag of a few seconds
// past their start makes a skipped row practically impossible. Run it on ONE worker: with several workers each
// keeps its own cursor and the flows are counted once per worker.
type LedgerScanner struct {
	Pool  *pgxpool.Pool
	Lag   time.Duration // default 30s
	Batch int           // rows per ledger per scan; default 50,000

	started        bool
	currencyCursor int64
	duyenCursor    int64
}

// Scan aggregates rows past the cursors into sink.
func (s *LedgerScanner) Scan(ctx context.Context, sink FlowSink) error {
	lag, batch := s.Lag, s.Batch
	if lag <= 0 {
		lag = 30 * time.Second
	}
	if batch <= 0 {
		batch = 50_000
	}
	if !s.started {
		if err := s.Pool.QueryRow(ctx, `SELECT COALESCE((SELECT max(id) FROM currency_ledger), 0),
			COALESCE((SELECT max(id) FROM vip_point_ledger), 0)`).Scan(&s.currencyCursor, &s.duyenCursor); err != nil {
			return err
		}
		s.started = true
		return nil
	}
	lagSec := lag.Seconds()
	next, err := s.aggregate(ctx, `WITH r AS (
			SELECT id, currency, reason, delta FROM currency_ledger
			WHERE id > $1 AND created_at < now() - make_interval(secs => $2) ORDER BY id LIMIT $3)
		SELECT currency || ':' || reason,
		       COALESCE(sum(delta) FILTER (WHERE delta > 0), 0)::bigint,
		       COALESCE(-sum(delta) FILTER (WHERE delta < 0), 0)::bigint,
		       max(id)
		FROM r GROUP BY currency, reason`, s.currencyCursor, lagSec, batch, func(key string, pos, neg int64) {
		if reason, ok := strings.CutPrefix(key, "spirit_stone:"); ok {
			sink.Stone(reason, pos, neg)
		}
	})
	if err != nil {
		return err
	}
	s.currencyCursor = next
	next, err = s.aggregate(ctx, `WITH r AS (
			SELECT id, source, delta FROM vip_point_ledger
			WHERE id > $1 AND created_at < now() - make_interval(secs => $2) ORDER BY id LIMIT $3)
		SELECT source,
		       COALESCE(sum(delta) FILTER (WHERE delta > 0), 0)::bigint,
		       COALESCE(-sum(delta) FILTER (WHERE delta < 0), 0)::bigint,
		       max(id)
		FROM r GROUP BY source`, s.duyenCursor, lagSec, batch, sink.Duyen)
	if err != nil {
		return err
	}
	s.duyenCursor = next
	return nil
}

func (s *LedgerScanner) aggregate(ctx context.Context, sql string, cursor int64, lagSec float64, batch int,
	emit func(key string, pos, neg int64)) (int64, error) {
	rows, err := s.Pool.Query(ctx, sql, cursor, lagSec, batch)
	if err != nil {
		return cursor, err
	}
	defer rows.Close()
	next := cursor
	type agg struct {
		key      string
		pos, neg int64
	}
	var out []agg
	for rows.Next() {
		var a agg
		var maxID int64
		if err := rows.Scan(&a.key, &a.pos, &a.neg, &maxID); err != nil {
			return cursor, err
		}
		out = append(out, a)
		next = max(next, maxID)
	}
	if err := rows.Err(); err != nil {
		return cursor, err
	}
	for _, a := range out { // only after the whole result was read: a failed scan emits nothing and keeps the cursor
		emit(a.key, a.pos, a.neg)
	}
	return next, nil
}
