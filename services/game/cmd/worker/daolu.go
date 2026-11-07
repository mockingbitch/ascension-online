package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tiendao/game/internal/content"
	"tiendao/game/internal/daolu/bond"
	"tiendao/game/internal/netsig"
)

// socialSweep runs on the reconcile ticker (default every 10 min), bounded per run:
//   - netsig purge: delete network hashes unseen for 30 days (privacy policy), ≤ 10 × 1,000 rows per run, and the
//     đạo lữ net_flag marks/events older than 30 days (≤ 1,000 each);
//   - đạo lữ absence: end ≤ 100 bonds per run where a partner showed no play for dao_lu.yaml absence_days.
type socialSweep struct {
	net *netsig.Store
	dl  *bond.Service
	log *slog.Logger
}

func newSocialSweep(cb *content.Bundle, pool *pgxpool.Pool, log *slog.Logger) *socialSweep {
	return &socialSweep{net: &netsig.Store{Pool: pool, Now: time.Now},
		dl: &bond.Service{Pool: pool, Cfg: cb.DaoLu, Realms: cb.Realms, Now: time.Now, Log: log}, log: log}
}

func (s *socialSweep) run(ctx context.Context) {
	if n, err := s.net.Purge(ctx, 1000, 10); err != nil && !errors.Is(err, context.Canceled) {
		s.log.Error("netsig purge", "err", err)
	} else if n > 0 {
		s.log.Info("netsig purge", "deleted", n)
	}
	if n, err := s.dl.PurgeNetFlags(ctx, 1000); err != nil && !errors.Is(err, context.Canceled) { // flag retention = 30 days
		s.log.Error("dao lu net flag purge", "err", err)
	} else if n > 0 {
		s.log.Info("dao lu net flags purged", "rows", n)
	}
	if n, err := s.dl.EndAbsent(ctx, 100); err != nil && !errors.Is(err, context.Canceled) {
		s.log.Error("dao lu absence sweep", "err", err)
	} else if n > 0 {
		s.log.Info("dao lu bonds ended by absence", "ended", n)
	}
}
