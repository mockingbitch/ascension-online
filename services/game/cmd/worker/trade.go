package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/content"
	"tiendao/game/internal/trade"
)

// tradeSweep runs on the reconcile ticker (default every 10 min): pending trade offers past expires_at become
// 'expired' (≤ 10 × 500 per run, SKIP LOCKED so an offer being answered is skipped). Reads already hide lapsed offers
// and accept/decline/cancel expire them on touch, so the sweep only tidies the log; a backlog drains over runs.
type tradeSweep struct {
	svc *trade.Service
	log *slog.Logger
}

func newTradeSweep(cb *content.Bundle, pool *pgxpool.Pool, log *slog.Logger) *tradeSweep {
	return &tradeSweep{svc: &trade.Service{Pool: pool, Cfg: cb.Trade, Items: cb.Items, Realms: cb.Realms, Now: time.Now, Log: log}, log: log}
}

func (t *tradeSweep) run(ctx context.Context) {
	total := 0
	for range 10 {
		n, err := t.svc.ExpireDue(ctx, 500)
		total += n
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				t.log.Error("trade expiry sweep", "err", err)
			}
			break
		}
		if n < 500 {
			break
		}
	}
	if total > 0 {
		t.log.Info("trade offers expired", "count", total)
	}
}
