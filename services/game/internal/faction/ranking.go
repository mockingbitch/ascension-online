package faction

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"tiendao/game/internal/db"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/leaderboard"
	"tiendao/game/internal/realm"
)

// Redis keys (data model §10). All rebuildable from Postgres; the worker is the only writer.
//
//	<prefix>:merit          ZSET faction_id → merit milli (Σ daily_contribution.points_milli in the current era)
//	<prefix>:members        HASH faction_id → member count
//	<prefix>:<id>:realm     ZSET character_id → global realm score (leaderboard.Score)
const DefaultKeyPrefix = "leaderboard:faction"

// ErrRankingUnavailable means Redis is down; Postgres stays correct and the worker rebuilds later.
var ErrRankingUnavailable = errors.New("faction: ranking temporarily unavailable")

const rebuildPage = 1000

// Ranking maintains the faction ZSETs/HASH from Postgres (worker) and reads them (API).
type Ranking struct {
	Pool      *pgxpool.Pool
	Redis     *redis.Client
	Realms    *realm.Catalogue
	Cat       *Catalogue
	CapMilli  int64  // world_credit_cap_points × 1000: merit uses world.Countable (needed by RebuildMerit only)
	KeyPrefix string // "" → DefaultKeyPrefix
	// Look filters members' frame/title by điểm duyên (leaderboard.Service.Look). nil → shown as equipped.
	Look func(frame, title string, vipPoints int64) (string, string)
}

func (r *Ranking) prefix() string {
	if r.KeyPrefix != "" {
		return r.KeyPrefix
	}
	return DefaultKeyPrefix
}

func (r *Ranking) meritKey() string   { return r.prefix() + ":merit" }
func (r *Ranking) membersKey() string { return r.prefix() + ":members" }
func (r *Ranking) dirtyKey(full bool) string {
	if full {
		return r.prefix() + ":dirty:full"
	}
	return r.prefix() + ":dirty:counts"
}

// RealmKey is the per-faction member ZSET.
func (r *Ranking) RealmKey(factionID string) string { return r.prefix() + ":" + factionID + ":realm" }

// MeritSince is the first game day counted toward faction merit (decision H). Merit is summed per game day, so a
// day is counted only if it lies wholly inside the era: the game day AFTER era_started_at — except an era that
// started on the launch day, which counts from the launch day (nothing before launch is real play). Zero time before
// launch (everything counts). Pure.
func MeritSince(ws db.WorldState) time.Time {
	switch {
	case ws.EraStartedAt != nil && ws.StartedAt != nil && gameday.Of(*ws.EraStartedAt).Equal(gameday.Of(*ws.StartedAt)):
		return gameday.Of(*ws.StartedAt)
	case ws.EraStartedAt != nil:
		return gameday.Of(*ws.EraStartedAt).AddDate(0, 0, 1)
	case ws.StartedAt != nil:
		return gameday.Of(*ws.StartedAt)
	default:
		return time.Time{}
	}
}

// RefreshMembers re-reads the given characters' progression + faction from Postgres and moves them into the right
// per-faction ZSET (removing them from every other one). Replay-safe: it only ever writes current values.
func (r *Ranking) RefreshMembers(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := db.New(r.Pool).GetFactionProgressionsByIDs(ctx, ids)
	if err != nil {
		return err
	}
	pipe := r.Redis.TxPipeline()
	for _, row := range rows {
		member := row.CharacterID.String()
		for _, id := range r.Cat.IDs() {
			if id != row.FactionID.String {
				pipe.ZRem(ctx, r.RealmKey(id), member)
			}
		}
		if row.FactionID.Valid {
			st, err := r.Realms.Stage(row.StageID)
			if err != nil {
				return err
			}
			pipe.ZAdd(ctx, r.RealmKey(row.FactionID.String), redis.Z{Score: leaderboard.Score(st, row.CultivationMilli), Member: member})
		}
	}
	_, err = pipe.Exec(ctx)
	return err
}

// RebuildCounts replaces the member-count HASH from Postgres.
func (r *Ranking) RebuildCounts(ctx context.Context) error {
	rows, err := db.New(r.Pool).CountFactionMembers(ctx)
	if err != nil {
		return err
	}
	pipe := r.Redis.TxPipeline()
	pipe.Del(ctx, r.membersKey())
	for _, row := range rows {
		pipe.HSet(ctx, r.membersKey(), row.FactionID, row.Members)
	}
	_, err = pipe.Exec(ctx)
	return err
}

// RebuildMerit replaces the merit ZSET from Postgres (current era only).
func (r *Ranking) RebuildMerit(ctx context.Context) error {
	q := db.New(r.Pool)
	ws, err := q.GetWorldState(ctx)
	if err != nil {
		return err
	}
	if r.CapMilli <= 0 {
		return errors.New("faction: Ranking.CapMilli (world_credit_cap) not set")
	}
	rows, err := q.SumFactionMerit(ctx, db.SumFactionMeritParams{CapMilli: r.CapMilli, FromDay: date(MeritSince(ws))})
	if err != nil {
		return err
	}
	pipe := r.Redis.TxPipeline()
	pipe.Del(ctx, r.meritKey())
	for _, row := range rows {
		pipe.ZAdd(ctx, r.meritKey(), redis.Z{Score: float64(row.MeritMilli), Member: row.FactionID})
	}
	_, err = pipe.Exec(ctx)
	return err
}

// RebuildMembers rebuilds every per-faction realm ZSET into temp keys and swaps them in.
func (r *Ranking) RebuildMembers(ctx context.Context) (int, error) {
	suffix := ":rebuild:" + strconv.FormatInt(time.Now().UnixNano(), 36)
	q := db.New(r.Pool)
	after, total := uuid.Nil, 0
	filled := map[string]bool{}
	for {
		rows, err := q.ListFactionProgressionsPage(ctx, db.ListFactionProgressionsPageParams{After: after, Lim: rebuildPage})
		if err != nil {
			return total, err
		}
		if len(rows) == 0 {
			break
		}
		pipe := r.Redis.TxPipeline()
		for _, row := range rows {
			st, err := r.Realms.Stage(row.StageID)
			if err != nil {
				return total, err
			}
			tmp := r.RealmKey(row.FactionID) + suffix
			pipe.ZAdd(ctx, tmp, redis.Z{Score: leaderboard.Score(st, row.CultivationMilli), Member: row.CharacterID.String()})
			pipe.Expire(ctx, tmp, 10*time.Minute) // a crashed rebuild must not leak keys
			filled[row.FactionID] = true
		}
		if _, err := pipe.Exec(ctx); err != nil {
			return total, err
		}
		total += len(rows)
		after = rows[len(rows)-1].CharacterID
	}
	ids := r.Cat.IDs()
	for id := range filled {
		if !slices.Contains(ids, id) {
			ids = append(ids, id) // a member of a faction no longer in the catalogue still gets a key
		}
	}
	pipe := r.Redis.TxPipeline()
	for _, id := range ids {
		if filled[id] {
			pipe.Rename(ctx, r.RealmKey(id)+suffix, r.RealmKey(id))
			pipe.Persist(ctx, r.RealmKey(id))
		} else {
			pipe.Del(ctx, r.RealmKey(id))
		}
	}
	_, err := pipe.Exec(ctx)
	return total, err
}

// Rebuild rebuilds everything (worker reconcile).
func (r *Ranking) Rebuild(ctx context.Context) (int, error) {
	if err := r.RebuildCounts(ctx); err != nil {
		return 0, fmt.Errorf("faction counts: %w", err)
	}
	if err := r.RebuildMerit(ctx); err != nil {
		return 0, fmt.Errorf("faction merit: %w", err)
	}
	n, err := r.RebuildMembers(ctx)
	if err != nil {
		return n, fmt.Errorf("faction members: %w", err)
	}
	return n, nil
}

// RequestRebuild marks the ranking for a rebuild (full) or a member-count refresh, done later by RunRequested
// outside the outbox drain (M2). Losing the marker is harmless: the periodic reconcile rebuilds everything.
func (r *Ranking) RequestRebuild(ctx context.Context, full bool) error {
	return r.Redis.Set(ctx, r.dirtyKey(full), "1", time.Hour).Err()
}

// RunRequested performs a requested rebuild, if any. The caller holds the reconcile advisory lock.
func (r *Ranking) RunRequested(ctx context.Context) (string, error) {
	full, err := r.Redis.GetDel(ctx, r.dirtyKey(true)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return "", err
	}
	counts, err := r.Redis.GetDel(ctx, r.dirtyKey(false)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return "", err
	}
	switch {
	case full != "":
		_, err := r.Rebuild(ctx)
		return "full", err
	case counts != "":
		return "counts", r.RebuildCounts(ctx)
	}
	return "", nil
}

// Board reads member counts and merit from Redis.
func (r *Ranking) Board(ctx context.Context) (members map[string]int64, merit map[string]int64, err error) {
	h, err := r.Redis.HGetAll(ctx, r.membersKey()).Result()
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrRankingUnavailable, err)
	}
	zs, err := r.Redis.ZRangeWithScores(ctx, r.meritKey(), 0, -1).Result()
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrRankingUnavailable, err)
	}
	members, merit = map[string]int64{}, map[string]int64{}
	for k, v := range h {
		n, _ := strconv.ParseInt(v, 10, 64)
		members[k] = n
	}
	for _, z := range zs {
		merit[fmt.Sprint(z.Member)] = int64(z.Score)
	}
	return members, merit, nil
}

// Members pages a faction's members by global realm score (ties: who reached the stage first, then id).
func (r *Ranking) Members(ctx context.Context, factionID string, offset, limit int64) ([]leaderboard.Entry, int64, error) {
	lb := &leaderboard.Service{Pool: r.Pool, Redis: r.Redis, Realms: r.Realms, Key: r.RealmKey(factionID), Look: r.Look}
	entries, total, err := lb.Page(ctx, offset, limit)
	if errors.Is(err, leaderboard.ErrUnavailable) {
		return nil, 0, fmt.Errorf("%w: %v", ErrRankingUnavailable, err)
	}
	return entries, total, err
}
