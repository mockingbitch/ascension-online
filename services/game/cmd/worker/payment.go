package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/config"
	"tiendao/game/internal/content"
	"tiendao/game/internal/payment"
)

// paymentSweep expires stale top-up orders and reconciles pending ones on a ticker (bounded batches; one query per
// pass, one provider call per candidate). Both are idempotent: concurrent workers or a webhook racing a reconcile
// settle an order exactly once (order row lock + order-anchored vip.TopupLocked).
type paymentSweep struct {
	*time.Ticker
	svc *payment.Service
	log *slog.Logger
}

func newPaymentSweep(cfg config.Config, cb *content.Bundle, pool *pgxpool.Pool, log *slog.Logger) (*paymentSweep, error) {
	pc := config.LoadPayment()
	provs, err := payment.BuildProviders(cb.Payments, cfg.Env, []byte(pc.SandboxSecret), cfg.PublicBaseURL+"/tien-duyen/sandbox/", pool, time.Now)
	if err != nil {
		return nil, err
	}
	every := pc.SweepInterval
	if every <= 0 {
		every = time.Minute
	}
	return &paymentSweep{Ticker: time.NewTicker(every), log: log,
		svc: &payment.Service{Pool: pool, Cat: cb.Payments, VIP: cb.VIP, Providers: provs, Log: log, Now: time.Now}}, nil
}

func (p *paymentSweep) run(ctx context.Context) {
	// Reconcile first: a pending order whose webhook was lost is settled before it would expire.
	res, err := p.svc.Reconcile(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		p.log.Error("payment reconcile", "err", err)
	} else if res.Checked > 0 {
		p.log.Info("payment reconcile", "checked", res.Checked, "settled", res.Settled, "failed", res.Failed)
	}
	if n, err := p.svc.Expire(ctx); err != nil && !errors.Is(err, context.Canceled) {
		p.log.Error("payment expiry", "err", err)
	} else if n > 0 {
		p.log.Info("payment orders expired", "count", n)
	}
}
