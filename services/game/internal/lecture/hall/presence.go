// Package hall runs Tử Tiêu Cung at runtime: Redis presence/seats (presence.go) and the ngộ đạo payment
// transaction (service.go). Pure rules and content live in package lecture.
package hall

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"tiendao/game/internal/lecture"
)

// Redis keys (data-model §10). Everything here is rebuildable: losing it costs listeners at most the unpaid
// listening since their last ngộ đạo (decision O4, 2026-11-07) and a re-seat on the next heartbeat.
const (
	keyBeat   = "lecture:hall:beat"   // ZSET characterId → last heartbeat (unix seconds) = who is in the hall
	keySeat   = "lecture:hall:seat"   // HASH seat index → characterId
	keySeatOf = "lecture:hall:seatof" // HASH characterId → seat index (-1 = listening outside the hall)
	keyWho    = "lecture:hall:who"    // HASH characterId → Listener JSON
	accTTL    = time.Hour
)

func keyAcc(charID string) string { return "lecture:acc:" + charID } // HASH secs, last

// Listener is what the hall shows for one seat.
type Listener struct {
	CharacterID string `json:"characterId"`
	Name        string `json:"name"`
	StageID     string `json:"stageId"`
	Seat        int    `json:"seat"` // -1 = outside the hall
}

// ErrNotPresent: the character has no recent heartbeat in the hall.
var ErrNotPresent = errors.New("hall: not in the hall")

// Hall is the Redis-backed presence of Tử Tiêu Cung.
type Hall struct {
	R   *redis.Client
	Cfg *lecture.Config
}

// enter: keep or take the lowest free seat, (re)start accrual if the last beat is stale, mark present.
var enterScript = redis.NewScript(`
local seat = redis.call('HGET', KEYS[3], ARGV[1])
local new = 0
if not seat then
  seat = -1
  for i = 0, tonumber(ARGV[3]) - 1 do
    if redis.call('HSETNX', KEYS[2], i, ARGV[1]) == 1 then seat = i break end
  end
  redis.call('HSET', KEYS[3], ARGV[1], seat)
  new = 1
end
redis.call('HSET', KEYS[4], ARGV[1], ARGV[4])
local now = tonumber(ARGV[2])
local last = tonumber(redis.call('HGET', KEYS[5], 'last') or '0')
if now - last > tonumber(ARGV[5]) then redis.call('HSET', KEYS[5], 'last', now) end
redis.call('HSETNX', KEYS[5], 'secs', 0)
redis.call('EXPIRE', KEYS[5], ARGV[6])
redis.call('ZADD', KEYS[1], now, ARGV[1])
return {tonumber(seat), new}
`)

// beat: accrue the gap since the last beat if it is ≤ max gap (capped), mark present. ARGV[5]=1 also takes
// (returns and zeroes) the accrued seconds — used by ngộ đạo. Returns -1 when the character is not in the hall.
var beatScript = redis.NewScript(`
if not redis.call('ZSCORE', KEYS[1], ARGV[1]) then return -1 end
local now = tonumber(ARGV[2])
local last = tonumber(redis.call('HGET', KEYS[2], 'last') or '0')
local secs = tonumber(redis.call('HGET', KEYS[2], 'secs') or '0')
local d = now - last
if d > 0 and d <= tonumber(ARGV[3]) then secs = math.min(tonumber(ARGV[4]), secs + d) end
if d > 0 then redis.call('HSET', KEYS[2], 'last', now) end
if ARGV[5] == '1' then redis.call('HSET', KEYS[2], 'secs', 0) else redis.call('HSET', KEYS[2], 'secs', secs) end
redis.call('EXPIRE', KEYS[2], ARGV[6])
redis.call('ZADD', KEYS[1], now, ARGV[1])
return secs
`)

// restore: give seconds back (clamped to the per-insight cap) and keep the key expiring.
var restoreScript = redis.NewScript(`
local secs = tonumber(redis.call('HGET', KEYS[1], 'secs') or '0') + tonumber(ARGV[1])
redis.call('HSET', KEYS[1], 'secs', math.min(secs, tonumber(ARGV[2])))
redis.call('EXPIRE', KEYS[1], ARGV[3])
return 1
`)

// leave one character, or (ARGV[1] == ”) every character whose last beat ≤ ARGV[2] (max ARGV[3]).
// Returns a flat list {characterId, seat, ...}.
var leaveScript = redis.NewScript(`
local ids
if ARGV[1] ~= '' then ids = {ARGV[1]} else
  ids = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[2], 'LIMIT', 0, tonumber(ARGV[3]))
end
local out = {}
for _, id in ipairs(ids) do
  local seat = redis.call('HGET', KEYS[3], id)
  local was = redis.call('ZREM', KEYS[1], id)
  redis.call('HDEL', KEYS[3], id)
  redis.call('HDEL', KEYS[4], id)
  if seat and tonumber(seat) >= 0 and redis.call('HGET', KEYS[2], seat) == id then redis.call('HDEL', KEYS[2], seat) end
  if was == 1 or seat then table.insert(out, id) table.insert(out, seat or '-1') end
end
return out
`)

// Enter seats the listener (idempotent: keeps an existing seat). isNew reports a new arrival.
func (h *Hall) Enter(ctx context.Context, l Listener, now time.Time) (seat int, isNew bool, err error) {
	who, _ := json.Marshal(Listener{CharacterID: l.CharacterID, Name: l.Name, StageID: l.StageID})
	res, err := enterScript.Run(ctx, h.R, []string{keyBeat, keySeat, keySeatOf, keyWho, keyAcc(l.CharacterID)},
		l.CharacterID, now.Unix(), h.Cfg.Seats, string(who), int64(h.Cfg.MaxGap/time.Second), int64(accTTL/time.Second)).Int64Slice()
	if err != nil {
		return 0, false, err
	}
	return int(res[0]), res[1] == 1, nil
}

// Beat records a heartbeat and returns the accrued unpaid seconds (ErrNotPresent if not in the hall).
func (h *Hall) Beat(ctx context.Context, charID string, now time.Time) (int64, error) {
	return h.beat(ctx, charID, now, false)
}

// Take accrues up to now, then removes and returns the accrued seconds (for a ngộ đạo payment).
func (h *Hall) Take(ctx context.Context, charID string, now time.Time) (int64, error) {
	return h.beat(ctx, charID, now, true)
}

func (h *Hall) beat(ctx context.Context, charID string, now time.Time, take bool) (int64, error) {
	t := "0"
	if take {
		t = "1"
	}
	n, err := beatScript.Run(ctx, h.R, []string{keyBeat, keyAcc(charID)},
		charID, now.Unix(), int64(h.Cfg.MaxGap/time.Second), h.Cfg.MaxSecondsPerInsight, t, int64(accTTL/time.Second)).Int64()
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, ErrNotPresent
	}
	return n, nil
}

// Restore gives back seconds taken by Take when the payment transaction failed (best effort).
func (h *Hall) Restore(ctx context.Context, charID string, secs int64) error {
	if secs <= 0 {
		return nil
	}
	return restoreScript.Run(ctx, h.R, []string{keyAcc(charID)}, secs, h.Cfg.MaxSecondsPerInsight, int64(accTTL/time.Second)).Err()
}

// Leave removes one listener. Returns their former seat, or ok=false if they were not in the hall.
func (h *Hall) Leave(ctx context.Context, charID string) (seat int, ok bool, err error) {
	gone, err := h.leave(ctx, charID, 0, 0)
	if err != nil || len(gone) == 0 {
		return 0, false, err
	}
	return gone[0].Seat, true, nil
}

// Sweep removes listeners whose last heartbeat is older than the max gap (lazy: called by hall requests, no ticker).
func (h *Hall) Sweep(ctx context.Context, now time.Time) ([]Listener, error) {
	return h.leave(ctx, "", now.Add(-h.Cfg.MaxGap).Unix(), 200)
}

func (h *Hall) leave(ctx context.Context, charID string, cutoff int64, limit int) ([]Listener, error) {
	res, err := leaveScript.Run(ctx, h.R, []string{keyBeat, keySeat, keySeatOf, keyWho}, charID, cutoff, limit).StringSlice()
	if err != nil {
		return nil, err
	}
	out := make([]Listener, 0, len(res)/2)
	for i := 0; i+1 < len(res); i += 2 {
		seat, _ := strconv.Atoi(res[i+1])
		out = append(out, Listener{CharacterID: res[i], Seat: seat})
	}
	return out, nil
}

// Snapshot returns the seated listeners (by seat) and the total number of people in the hall.
func (h *Hall) Snapshot(ctx context.Context) (seated []Listener, total int64, err error) {
	pipe := h.R.Pipeline()
	seats := pipe.HGetAll(ctx, keySeat)
	count := pipe.ZCard(ctx, keyBeat)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, 0, err
	}
	ids := make([]string, 0, len(seats.Val()))
	seatOf := map[string]int{}
	for s, id := range seats.Val() {
		n, _ := strconv.Atoi(s)
		ids = append(ids, id)
		seatOf[id] = n
	}
	seated = []Listener{}
	if len(ids) > 0 {
		vals, err := h.R.HMGet(ctx, keyWho, ids...).Result()
		if err != nil {
			return nil, 0, err
		}
		for i, v := range vals {
			var l Listener
			if s, ok := v.(string); ok {
				_ = json.Unmarshal([]byte(s), &l)
			}
			l.CharacterID, l.Seat = ids[i], seatOf[ids[i]]
			seated = append(seated, l)
		}
	}
	return seated, count.Val(), nil
}

// SeatOf returns the character's seat and whether they are in the hall.
func (h *Hall) SeatOf(ctx context.Context, charID string) (int, bool, error) {
	v, err := h.R.HGet(ctx, keySeatOf, charID).Int()
	if errors.Is(err, redis.Nil) {
		return 0, false, nil
	}
	return v, err == nil, err
}
