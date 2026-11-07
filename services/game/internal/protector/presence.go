package protector

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Presence answers "who is online". Everything is ephemeral (TTL) and safe to lose: a lost key only means a protector
// has to ping again before the attempt counts them.
type Presence interface {
	// MarkActive records a real game action of the user (refreshes the activity window).
	MarkActive(ctx context.Context, userID uuid.UUID) error
	// Ping marks the character online for the TTL — only when the user was active within the activity window.
	Ping(ctx context.Context, charID, userID uuid.UUID, factionID string, now time.Time) (bool, error)
	Online(ctx context.Context, ids []uuid.UUID, now time.Time) (map[uuid.UUID]bool, error)
	FactionOnline(ctx context.Context, factionID string, now time.Time, limit int64) ([]uuid.UUID, error)
}

// Redis keys (data-model §10 style).
func keyChar(id uuid.UUID) string    { return "presence:char:" + id.String() }    // STRING factionId, EX ttl
func keyFaction(fid string) string   { return "presence:faction:" + fid }         // ZSET characterId → last ping (unix s)
func keyActive(uid uuid.UUID) string { return "presence:active:" + uid.String() } // STRING 1, EX activity window

// ActiveKey is the Redis key that exists while the user made a successful authenticated game mutation within the
// activity window (anti-cheat M2). Shared with đạo lữ song tu, which requires both partners to be active.
func ActiveKey(userID uuid.UUID) string { return keyActive(userID) }

// RedisPresence is the production Presence.
type RedisPresence struct {
	R        *redis.Client
	TTL      time.Duration
	Activity time.Duration // activity window (protector.yaml activity_window_seconds)
}

// MarkActive refreshes the user's activity window (called after a successful authenticated game mutation).
func (p *RedisPresence) MarkActive(ctx context.Context, userID uuid.UUID) error {
	return p.R.Set(ctx, keyActive(userID), 1, p.Activity).Err()
}

// Ping marks the character online for TTL (and in its faction's ZSET, pruned lazily on read), only when the user made
// a real game action within the activity window: a bare ping script keeps nobody online (anti-cheat M2).
func (p *RedisPresence) Ping(ctx context.Context, charID, userID uuid.UUID, factionID string, now time.Time) (bool, error) {
	n, err := p.R.Exists(ctx, keyActive(userID)).Result()
	if err != nil || n == 0 {
		return false, err
	}
	pipe := p.R.TxPipeline()
	pipe.Set(ctx, keyChar(charID), factionID, p.TTL)
	if factionID != "" {
		pipe.ZAdd(ctx, keyFaction(factionID), redis.Z{Score: float64(now.Unix()), Member: charID.String()})
		pipe.Expire(ctx, keyFaction(factionID), 10*p.TTL)
	}
	_, err = pipe.Exec(ctx)
	return err == nil, err
}

// Online reports which characters have a live presence key.
func (p *RedisPresence) Online(ctx context.Context, ids []uuid.UUID, _ time.Time) (map[uuid.UUID]bool, error) {
	out := map[uuid.UUID]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = keyChar(id)
	}
	vals, err := p.R.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	for i, v := range vals {
		if v != nil {
			out[ids[i]] = true
		}
	}
	return out, nil
}

// FactionOnline lists up to limit members pinged within TTL, newest first (prunes stale entries).
func (p *RedisPresence) FactionOnline(ctx context.Context, factionID string, now time.Time, limit int64) ([]uuid.UUID, error) {
	cutoff := now.Add(-p.TTL).Unix()
	k := keyFaction(factionID)
	if err := p.R.ZRemRangeByScore(ctx, k, "-inf", "("+strconv.FormatInt(cutoff, 10)).Err(); err != nil {
		return nil, err
	}
	ids, err := p.R.ZRevRangeByScore(ctx, k, &redis.ZRangeBy{Min: strconv.FormatInt(cutoff, 10), Max: "+inf", Count: limit}).Result()
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(ids))
	for _, s := range ids {
		if id, err := uuid.Parse(s); err == nil {
			out = append(out, id)
		}
	}
	return out, nil
}
