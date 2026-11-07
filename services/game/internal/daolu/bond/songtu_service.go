package bond

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"tiendao/game/internal/contribution"
	"tiendao/game/internal/cultivation"
	"tiendao/game/internal/db"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/platform/postgres"
	"tiendao/game/internal/vip"
)

// ClaimResult is POST /dao-lu/song-tu/claim.
type ClaimResult struct {
	Cultivation cultivation.Snapshot `json:"cultivation"`
	Seconds     int64                `json:"seconds"`
	RawMilli    int64                `json:"rawMilli"`
	EarnedMilli int64                `json:"earnedMilli"` // after bucket/ceiling/bottleneck
	VIPPoints   int64                `json:"vipPoints"`
	Replayed    bool                 `json:"replayed"`
	SongTu      SongTuView           `json:"songTu"`
}

// activePair resolves the viewer's bond and partner (read-only).
func (s *Service) activePair(ctx context.Context, q *db.Queries, userID uuid.UUID) (Pair, error) {
	me, err := s.me(ctx, q, userID)
	if err != nil {
		return Pair{}, err
	}
	bond, err := q.GetActiveDaoLuBond(ctx, me.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Pair{}, ErrNoBond
	} else if err != nil {
		return Pair{}, err
	}
	p, _, err := s.pairOf(ctx, q, userID, me.ID, bond)
	return p, err
}

// songTuView combines the Redis session (b, or a Peek when nil) with the viewer's paid seconds today (DB).
func (s *Service) songTuView(ctx context.Context, q *db.Queries, p Pair, b *Beat, now time.Time) (SongTuView, error) {
	st := s.Cfg.SongTu
	v := SongTuView{Available: s.Session != nil, GameDay: gameday.Of(now).Format(time.DateOnly), ResetAt: gameday.NextReset(now),
		WindowSeconds: st.WindowSeconds, HeartbeatSeconds: st.HeartbeatSeconds}
	paid, err := q.SumSongTuDay(ctx, db.SumSongTuDayParams{CharacterID: p.MeChar, GameDay: gameday.PG(now)})
	if err != nil {
		return v, err
	}
	v.PaidSeconds = int64(paid)
	if s.Session == nil {
		return v, nil
	}
	if b == nil {
		got, err := s.Session.Peek(ctx, p, now)
		if err != nil {
			if s.Log != nil {
				s.Log.WarnContext(ctx, "song tu peek", "err", err)
			}
			return v, nil
		}
		b = &got
	}
	v.SessionStarted, v.AccruedSeconds, v.MeActive, v.PartnerActive = b.Started, b.Secs, b.MeActive, b.PartnerActive
	if !b.PartnerBeatAt.IsZero() {
		t := b.PartnerBeatAt
		v.PartnerBeatAt = &t
	}
	v.ClaimableSeconds = max(0, min(b.Unclaimed, st.WindowSeconds-v.PaidSeconds))
	v.ClaimableMilli = s.Cfg.PayMilli(v.PaidSeconds+v.ClaimableSeconds) - s.Cfg.PayMilli(v.PaidSeconds)
	return v, nil
}

// SongTuState is GET /dao-lu/song-tu (no accrual).
func (s *Service) SongTuState(ctx context.Context, userID uuid.UUID) (SongTuView, error) {
	q := db.New(s.Pool)
	p, err := s.activePair(ctx, q, userID)
	if err != nil {
		return SongTuView{}, err
	}
	return s.songTuView(ctx, q, p, nil, s.Now().UTC())
}

// StartSongTu opens today's session (either partner, only while BOTH are active). Idempotent.
func (s *Service) StartSongTu(ctx context.Context, userID uuid.UUID) (SongTuView, error) {
	if s.Session == nil {
		return SongTuView{}, ErrSongTuOff
	}
	q := db.New(s.Pool)
	p, err := s.activePair(ctx, q, userID)
	if err != nil {
		return SongTuView{}, err
	}
	now := s.Now().UTC()
	if err := s.Session.Start(ctx, p, now); err != nil {
		return SongTuView{}, err
	}
	b, err := s.Session.Beat(ctx, p, now)
	if err != nil {
		return SongTuView{}, err
	}
	return s.songTuView(ctx, q, p, &b, now)
}

// HeartbeatSongTu records the viewer's beat and accrues overlap (server clock). The heartbeat itself is NOT a game
// action (httpapi excludes it from the activity window, like the presence ping).
func (s *Service) HeartbeatSongTu(ctx context.Context, userID uuid.UUID) (SongTuView, error) {
	if s.Session == nil {
		return SongTuView{}, ErrSongTuOff
	}
	q := db.New(s.Pool)
	p, err := s.activePair(ctx, q, userID)
	if err != nil {
		return SongTuView{}, err
	}
	now := s.Now().UTC()
	b, err := s.Session.Beat(ctx, p, now)
	if err != nil {
		return SongTuView{}, err
	}
	return s.songTuView(ctx, q, p, &b, now)
}

// ClaimSongTu pays the viewer's unclaimed overlap. Order (lecture pattern): idempotency replay → Redis take → tx
// (lock progression → key re-check → window left today (DB) → CreditLocked dual_cultivation → điểm duyên when the day's
// paid seconds cross duyen_min_seconds → claim row); on failure the taken seconds go back to Redis unless the claim
// row committed. Each partner claims separately from the same shared overlap.
func (s *Service) ClaimSongTu(ctx context.Context, userID uuid.UUID, idemKey string) (ClaimResult, error) {
	if s.Session == nil {
		return ClaimResult{}, ErrSongTuOff
	}
	q0 := db.New(s.Pool)
	p, err := s.activePair(ctx, q0, userID)
	if err != nil {
		return ClaimResult{}, err
	}
	if prev, err := q0.GetSongTuClaimByKey(ctx, db.GetSongTuClaimByKeyParams{CharacterID: p.MeChar, IdempotencyKey: idemKey}); err == nil {
		return s.replayClaim(ctx, userID, p, prev)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return ClaimResult{}, err
	}
	now := s.Now().UTC()
	b, err := s.Session.Take(ctx, p, now)
	if err != nil {
		return ClaimResult{}, err
	}
	taken := b.Unclaimed
	if taken <= 0 {
		return ClaimResult{}, ErrNothingToClaim
	}
	st := s.Cfg.SongTu
	var out ClaimResult
	var replay *db.DaoLuSongTuClaim
	err = postgres.InTx(ctx, s.Pool, func(tx pgx.Tx) error {
		q := db.New(tx)
		prog, err := q.GetProgressionForUpdate(ctx, p.MeChar)
		if err != nil {
			return fmt.Errorf("lock progression: %w", err)
		}
		// Review L4: the bond must still be active (an admin/absence ending may have committed since activePair).
		if b, err := q.GetActiveDaoLuBond(ctx, p.MeChar); errors.Is(err, pgx.ErrNoRows) || (err == nil && b.ID != p.BondID) {
			return ErrNoBond
		} else if err != nil {
			return err
		}
		if prev, err := q.GetSongTuClaimByKey(ctx, db.GetSongTuClaimByKeyParams{CharacterID: p.MeChar, IdempotencyKey: idemKey}); err == nil {
			replay = &prev
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		gd := gameday.PG(now)
		paid32, err := q.SumSongTuDay(ctx, db.SumSongTuDayParams{CharacterID: p.MeChar, GameDay: gd})
		if err != nil {
			return err
		}
		paid := int64(paid32)
		allow := min(taken, st.WindowSeconds-paid)
		if allow <= 0 {
			return ErrSongTuDayCapped
		}
		raw := s.Cfg.PayMilli(paid+allow) - s.Cfg.PayMilli(paid)
		var credit cultivation.CreditOutcome
		if raw > 0 {
			prog, credit, err = s.Cultivation.CreditLocked(ctx, q, prog, contribution.SourceDualCultivation, raw,
				cultivation.Ref{Type: "song_tu", ID: p.BondID.String(), IdemKey: idemKey}, now)
			if err != nil {
				return err
			}
		}
		var vipPts int64
		if paid < st.DuyenMinSeconds && paid+allow >= st.DuyenMinSeconds && s.VIP != nil {
			key := "song_tu:" + gameday.Of(now).Format("20060102")
			got, err := s.VIP.EarnLocked(ctx, q, userID, vip.ActDualCultivation, vip.Ref{Type: "song_tu", ID: p.BondID.String(), IdemKey: key}, now)
			if err != nil {
				return fmt.Errorf("earn điểm duyên: %w", err)
			}
			if !got.Replayed {
				vipPts = got.Points
			}
		}
		if _, err := q.InsertSongTuClaim(ctx, db.InsertSongTuClaimParams{CharacterID: p.MeChar, BondID: p.BondID, GameDay: gd, Seconds: int32(allow),
			SecondsAfter: int32(paid + allow), RawMilli: raw, CreditedMilli: credit.Applied.CreditedMilli, VipPoints: int32(vipPts),
			IdempotencyKey: idemKey, CreatedAt: now}); err != nil {
			return fmt.Errorf("insert claim: %w", err)
		}
		snap, err := s.Cultivation.SnapshotLocked(ctx, q, userID, prog, now)
		if err != nil {
			return err
		}
		out = ClaimResult{Cultivation: snap, Seconds: allow, RawMilli: raw, EarnedMilli: credit.Applied.CreditedMilli, VIPPoints: vipPts}
		return nil
	})
	restore := taken
	if err != nil && replay == nil {
		// The COMMIT may have succeeded although we saw an error: give the seconds back only when the claim row is
		// provably absent (no rows). Found, or the check itself failed → keep them taken (review L6: never double-pay).
		if _, gerr := db.New(s.Pool).GetSongTuClaimByKey(context.WithoutCancel(ctx), db.GetSongTuClaimByKeyParams{CharacterID: p.MeChar,
			IdempotencyKey: idemKey}); !errors.Is(gerr, pgx.ErrNoRows) {
			restore = 0
		}
	}
	if (err != nil && !errors.Is(err, ErrSongTuDayCapped)) || replay != nil {
		if rerr := s.Session.Restore(context.WithoutCancel(ctx), p, now, restore); rerr != nil && s.Log != nil {
			s.Log.Warn("song tu: restore accrual", "characterId", p.MeChar, "err", rerr)
		}
	}
	if err != nil {
		return ClaimResult{}, err
	}
	if replay != nil {
		return s.replayClaim(ctx, userID, p, *replay)
	}
	sv, err := s.songTuView(ctx, db.New(s.Pool), p, nil, now)
	if err != nil {
		return ClaimResult{}, err
	}
	out.SongTu = sv
	return out, nil
}

func (s *Service) replayClaim(ctx context.Context, userID uuid.UUID, p Pair, row db.DaoLuSongTuClaim) (ClaimResult, error) {
	now := s.Now().UTC()
	q := db.New(s.Pool)
	prog, err := q.GetProgression(ctx, p.MeChar)
	if err != nil {
		return ClaimResult{}, err
	}
	snap, err := s.Cultivation.SnapshotLocked(ctx, q, userID, prog, now)
	if err != nil {
		return ClaimResult{}, err
	}
	sv, err := s.songTuView(ctx, q, p, nil, now)
	if err != nil {
		return ClaimResult{}, err
	}
	return ClaimResult{Cultivation: snap, Seconds: int64(row.Seconds), RawMilli: row.RawMilli, EarnedMilli: row.CreditedMilli,
		VIPPoints: int64(row.VipPoints), Replayed: true, SongTu: sv}, nil
}
