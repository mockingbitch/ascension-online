// Package hall runs Tử Tiêu Cung at runtime: Redis presence split into điện (presence.go), the ngộ đạo payment
// transaction and the per-điện event fan-out (service.go). Pure rules and content live in package lecture.
//
// Điện (decisions 2026-11-10): the hall is sharded into numbered điện of lectures.yaml hall.seats cushions. Everyone
// in a điện has a cushion (there is no "outside"). Assignment, moving and merging run in Lua, so each is atomic:
// concurrent enters can never put more than `seats` people in one điện.
package hall

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"tiendao/game/internal/appearance"
	"tiendao/game/internal/lecture"
)

// Redis keys (data-model §10). Everything here is rebuildable: losing it costs listeners at most the unpaid
// listening since their last ngộ đạo (decision O4, 2026-11-07) and a re-seat on the next heartbeat. Rewards and the
// daily caps live in Postgres (lecture_insights) and do not depend on the điện.
//
// Single Redis (not Cluster): the scripts touch the per-điện keys they compute from the prefix.
const (
	keyBeat   = "lecture:hall:beat"   // ZSET characterId → last heartbeat (unix s) = everyone listening, all điện
	keyOpen   = "lecture:hall:open"   // SET điện ids that have at least one listener
	keyAt     = "lecture:hall:at"     // HASH characterId → "<điện>:<seat>"
	keyWho    = "lecture:hall:who"    // HASH characterId → Listener JSON (name, stage, look)
	keyFacOf  = "lecture:hall:facof"  // HASH characterId → faction id (assignment hint)
	keyMerged = "lecture:hall:merged" // STRING the last lecture number whose boundary merged near-empty điện
	hallKeyP  = "lecture:hall:d:"     // + <điện> + ":seat" (HASH seat → characterId) | ":fac" (HASH faction → count)
	accTTL    = time.Hour
	// mergeBudget bounds one merge call: at most this many source điện (each ≤ merge_below listeners) per EVAL.
	mergeBudget = 8
)

func keyAcc(charID string) string { return "lecture:acc:" + charID } // HASH secs, last

// Listener is what the hall shows for one seat.
type Listener struct {
	CharacterID string `json:"characterId"`
	Name        string `json:"name"`
	StageID     string `json:"stageId"`
	HallID      int    `json:"hallId"`
	Seat        int    `json:"seat"`
	// FactionID of the listener (display only: phones show the names of same-faction listeners first).
	FactionID string `json:"factionId,omitempty"`
	// Look resolved for the listener's path (appearances.yaml; display only): the seat shows the chosen figure.
	Look *appearance.Look `json:"look,omitempty"`
}

// HallCount is one điện in the picker.
type HallCount struct {
	ID    int `json:"id"`
	Count int `json:"count"`
}

// Move is one listener moved by a merge.
type Move struct {
	CharacterID string
	From, To    int // điện
	FromSeat    int
	ToSeat      int
}

// Errors of the presence layer (mapped to HTTP in httpapi).
var (
	ErrNotPresent  = errors.New("hall: not in the hall")
	ErrHallsFull   = errors.New("hall: every điện is full (max_halls reached)")
	ErrHallUnknown = errors.New("hall: no such điện is open")
	ErrHallFull    = errors.New("hall: that điện is full")
)

// Hall is the Redis-backed presence of Tử Tiêu Cung.
type Hall struct {
	R   *redis.Client
	Cfg *lecture.Config
}

func (h *Hall) keys() []string { return []string{keyBeat, keyOpen, keyAt, keyWho, keyFacOf} }

// luaCommon: KEYS[1..5] = beat, open, at, who, facof; ARGV[1] = per-điện key prefix.
const luaCommon = `
local P = ARGV[1]
local K_BEAT, K_OPEN, K_AT, K_WHO, K_FAC = KEYS[1], KEYS[2], KEYS[3], KEYS[4], KEYS[5]
local function seatKey(h) return P .. h .. ':seat' end
local function facKey(h) return P .. h .. ':fac' end
local function where(id)
  local at = redis.call('HGET', K_AT, id)
  if not at then return nil end
  local h, s = string.match(at, '^(%d+):(%d+)$')
  if not h then return nil end
  return tonumber(h), tonumber(s)
end
-- frees id's cushion and faction count; closes the điện when it empties. Returns the old điện, seat (or nil).
local function unseat(id)
  local h, s = where(id)
  local f = redis.call('HGET', K_FAC, id)
  redis.call('HDEL', K_AT, id)
  redis.call('HDEL', K_FAC, id)
  if not h then return nil end
  if redis.call('HGET', seatKey(h), s) == id then redis.call('HDEL', seatKey(h), s) end
  if f and f ~= '' then
    if redis.call('HINCRBY', facKey(h), f, -1) <= 0 then redis.call('HDEL', facKey(h), f) end
  end
  if redis.call('HLEN', seatKey(h)) == 0 then
    redis.call('SREM', K_OPEN, h)
    redis.call('DEL', facKey(h))
  end
  return h, s
end
-- seats id on the lowest free cushion of điện h; nil when it is full.
local function seatIn(id, h, cap, fac)
  for i = 0, cap - 1 do
    if redis.call('HSETNX', seatKey(h), i, id) == 1 then
      redis.call('SADD', K_OPEN, h)
      redis.call('HSET', K_AT, id, h .. ':' .. i)
      if fac and fac ~= '' then
        redis.call('HSET', K_FAC, id, fac)
        redis.call('HINCRBY', facKey(h), fac, 1)
      end
      return i
    end
  end
  return nil
end
`

// enter: keep the listener's cushion, or assign one (đạo lữ's điện if not full → most same-faction listeners →
// fullest non-full → open the lowest unused number); (re)start accrual if the last beat is stale; mark present.
// ARGV: 2 id, 3 now, 4 cap, 5 who JSON, 6 max gap s, 7 acc TTL s, 8 partner id, 9 faction id, 10 max điện.
// KEYS[6] = the accrual hash. Returns {điện, seat, new}; {0, -1, 0} when every điện is full.
var enterScript = redis.NewScript(luaCommon + `
local id, cap, fac = ARGV[2], tonumber(ARGV[4]), ARGV[9]
local hall, seat = where(id)
local new = 0
if hall and redis.call('HGET', seatKey(hall), seat) ~= id then
  unseat(id) -- stale pointer (should not happen): assign again
  hall = nil
end
if hall then
  local old = redis.call('HGET', K_FAC, id) or ''
  if old ~= fac then -- faction changed while seated: keep the hint counts right
    if old ~= '' and redis.call('HINCRBY', facKey(hall), old, -1) <= 0 then redis.call('HDEL', facKey(hall), old) end
    if fac ~= '' then redis.call('HSET', K_FAC, id, fac) redis.call('HINCRBY', facKey(hall), fac, 1) else redis.call('HDEL', K_FAC, id) end
  end
else
  local best = nil
  if ARGV[8] ~= '' then
    local ph = where(ARGV[8])
    if ph and redis.call('HLEN', seatKey(ph)) < cap then best = ph end
  end
  if not best then
    local bestF, bestN = -1, -1
    for _, hs in ipairs(redis.call('SMEMBERS', K_OPEN)) do
      local n = redis.call('HLEN', seatKey(hs))
      if n < cap then
        local f = 0
        if fac ~= '' then f = tonumber(redis.call('HGET', facKey(hs), fac) or '0') end
        local hn = tonumber(hs)
        if f > bestF or (f == bestF and (n > bestN or (n == bestN and hn < best))) then best, bestF, bestN = hn, f, n end
      end
    end
  end
  if not best then
    local n = 1
    while redis.call('SISMEMBER', K_OPEN, n) == 1 do n = n + 1 end
    if n > tonumber(ARGV[10]) then return {0, -1, 0} end
    best = n
  end
  seat = seatIn(id, best, cap, fac)
  if not seat then return {0, -1, 0} end -- unreachable: best has room
  hall = best
  new = 1
end
redis.call('HSET', K_WHO, id, ARGV[5])
local now = tonumber(ARGV[3])
local last = tonumber(redis.call('HGET', KEYS[6], 'last') or '0')
if now - last > tonumber(ARGV[6]) then redis.call('HSET', KEYS[6], 'last', now) end
redis.call('HSETNX', KEYS[6], 'secs', 0)
redis.call('EXPIRE', KEYS[6], ARGV[7])
redis.call('ZADD', K_BEAT, now, id)
return {hall, seat, new}
`)

// beat: accrue the gap since the last beat if it is ≤ max gap (capped), mark present. ARGV[5]=1 also takes
// (returns and zeroes) the accrued seconds — used by ngộ đạo. Returns -1 when the character is not in the hall.
// Unchanged by the điện split: listening time is global.
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

// leave one character (ARGV[2]), or (ARGV[2] == ”) every character whose last beat ≤ ARGV[3] (max ARGV[4]).
// Returns a flat list {characterId, điện, seat, ...} (điện 0 / seat -1 when they had no cushion).
var leaveScript = redis.NewScript(luaCommon + `
local ids
if ARGV[2] ~= '' then ids = {ARGV[2]} else
  ids = redis.call('ZRANGEBYSCORE', K_BEAT, '-inf', ARGV[3], 'LIMIT', 0, tonumber(ARGV[4]))
end
local out = {}
for _, id in ipairs(ids) do
  local h, s = unseat(id)
  local was = redis.call('ZREM', K_BEAT, id)
  redis.call('HDEL', K_WHO, id)
  if was == 1 or h then
    table.insert(out, id) table.insert(out, tostring(h or 0)) table.insert(out, tostring(s or -1))
  end
end
return out
`)

// switch: move a present listener to an open, non-full điện. ARGV: 2 id, 3 target, 4 cap.
// Returns {code, from, fromSeat, toSeat}: 1 moved, 0 already there, -1 not present, -2 not open, -3 full.
var switchScript = redis.NewScript(luaCommon + `
local id, target, cap = ARGV[2], tonumber(ARGV[3]), tonumber(ARGV[4])
local h, s = where(id)
if not h or not redis.call('ZSCORE', K_BEAT, id) then return {-1, 0, -1, -1} end
if h == target then return {0, h, s, s} end
if redis.call('SISMEMBER', K_OPEN, target) == 0 then return {-2, h, s, -1} end
if redis.call('HLEN', seatKey(target)) >= cap then return {-3, h, s, -1} end
-- take the new cushion BEFORE freeing the old one: if none is free (e.g. seats lowered under a live Redis),
-- nothing changes and the player keeps their cushion
local ns = nil
for i = 0, cap - 1 do
  if redis.call('HSETNX', seatKey(target), i, id) == 1 then ns = i break end
end
if not ns then return {-3, h, s, -1} end
local f = redis.call('HGET', K_FAC, id)
redis.call('HDEL', seatKey(target), ns)
unseat(id)
seatIn(id, target, cap, f) -- the cushion just released is the lowest free one: ns again
return {1, h, s, ns}
`)

// merge: after a lecture boundary (ARGV[2] = the lecture number; KEYS[6] = the marker of the last number fully
// merged), repeatedly take the smallest điện with fewer than ARGV[4] listeners and move all of it into the other
// điện — preferably one điện that fits the whole group (the fullest such), else the fullest ones with room — as
// long as everyone fits. At most ARGV[5] source điện per call (bounds the time Redis is busy); the marker is set
// only when nothing qualifies anymore, so the next calls (presence loop, every few seconds) continue the work.
// ARGV[3] = cap. Returns a flat list {characterId, from, fromSeat, to, toSeat, ...}.
var mergeScript = redis.NewScript(luaCommon + `
local no, cap, below, budget = tonumber(ARGV[2]), tonumber(ARGV[3]), tonumber(ARGV[4]), tonumber(ARGV[5])
local last = redis.call('GET', KEYS[6])
if last and tonumber(last) >= no then return {} end
local out = {}
local done = false
for _ = 1, budget do
  local halls, free = {}, 0
  for _, hs in ipairs(redis.call('SMEMBERS', K_OPEN)) do
    local n = redis.call('HLEN', seatKey(hs))
    table.insert(halls, {id = tonumber(hs), n = n})
    free = free + (cap - n)
  end
  if #halls < 2 then done = true break end
  -- source: the smallest under the threshold whose listeners fit in the others (highest number first on ties)
  table.sort(halls, function(a, b) if a.n ~= b.n then return a.n < b.n end return a.id > b.id end)
  local src = nil
  for _, x in ipairs(halls) do
    if x.n >= below then break end
    if x.n > 0 and free - (cap - x.n) >= x.n then src = x break end
  end
  if not src then done = true break end
  -- targets: fullest first; a single điện that takes the whole group goes first of all
  local targets = {}
  for _, x in ipairs(halls) do if x.id ~= src.id and x.n < cap then table.insert(targets, x) end end
  table.sort(targets, function(a, b) if a.n ~= b.n then return a.n > b.n end return a.id < b.id end)
  for i, x in ipairs(targets) do
    if cap - x.n >= src.n then table.remove(targets, i) table.insert(targets, 1, x) break end
  end
  local seats = redis.call('HGETALL', seatKey(src.id))
  local members = {}
  for i = 1, #seats, 2 do table.insert(members, {seat = tonumber(seats[i]), id = seats[i + 1]}) end
  table.sort(members, function(a, b) return a.seat < b.seat end)
  local ti = 1
  for _, m in ipairs(members) do
    local f = redis.call('HGET', K_FAC, m.id)
    local fromH, fromS = unseat(m.id)
    local ns = nil
    while not ns and ti <= #targets do
      ns = seatIn(m.id, targets[ti].id, cap, f)
      if not ns then ti = ti + 1 end
    end
    if not ns then -- cannot happen (free was checked); put them back so nobody is left without a cushion
      ns = seatIn(m.id, src.id, cap, f)
      table.insert(out, m.id) table.insert(out, tostring(fromH or 0)) table.insert(out, tostring(fromS or -1)) table.insert(out, tostring(src.id)) table.insert(out, tostring(ns))
    else
      table.insert(out, m.id) table.insert(out, tostring(fromH or 0)) table.insert(out, tostring(fromS or -1)) table.insert(out, tostring(targets[ti].id)) table.insert(out, tostring(ns))
    end
  end
end
if done then redis.call('SET', KEYS[6], no, 'EX', 86400) end
return out
`)

// counts: {global listeners, điện, count, điện, count, ...}.
var countsScript = redis.NewScript(luaCommon + `
local out = {redis.call('ZCARD', K_BEAT)}
for _, hs in ipairs(redis.call('SMEMBERS', K_OPEN)) do
  table.insert(out, tonumber(hs)) table.insert(out, redis.call('HLEN', seatKey(hs)))
end
return out
`)

// Affinity is the assignment hint for Enter.
type Affinity struct {
	PartnerID string // active đạo lữ character id ("" = none)
	FactionID string // "" = none
}

// Enter seats the listener (idempotent: keeps an existing cushion). isNew reports a new arrival.
func (h *Hall) Enter(ctx context.Context, l Listener, aff Affinity, now time.Time) (hallID, seat int, isNew bool, err error) {
	who, _ := json.Marshal(Listener{CharacterID: l.CharacterID, Name: l.Name, StageID: l.StageID, Look: l.Look, FactionID: aff.FactionID})
	res, err := enterScript.Run(ctx, h.R, append(h.keys(), keyAcc(l.CharacterID)),
		hallKeyP, l.CharacterID, now.Unix(), h.Cfg.Seats, string(who), int64(h.Cfg.MaxGap/time.Second), int64(accTTL/time.Second),
		aff.PartnerID, aff.FactionID, h.Cfg.MaxHalls).Int64Slice()
	if err != nil {
		return 0, 0, false, err
	}
	if len(res) != 3 || res[0] == 0 {
		return 0, -1, false, ErrHallsFull
	}
	return int(res[0]), int(res[1]), res[2] == 1, nil
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

// Leave removes one listener. Returns their former điện and seat, or ok=false if they were not in the hall.
func (h *Hall) Leave(ctx context.Context, charID string) (gone Listener, ok bool, err error) {
	out, err := h.leave(ctx, charID, 0, 0)
	if err != nil || len(out) == 0 {
		return Listener{}, false, err
	}
	return out[0], true, nil
}

// Sweep removes listeners whose last heartbeat is older than the max gap (lazy: hall requests and the presence
// loop call it; no per-player timer).
func (h *Hall) Sweep(ctx context.Context, now time.Time) ([]Listener, error) {
	return h.leave(ctx, "", now.Add(-h.Cfg.MaxGap).Unix(), 200)
}

func (h *Hall) leave(ctx context.Context, charID string, cutoff int64, limit int) ([]Listener, error) {
	res, err := leaveScript.Run(ctx, h.R, h.keys(), hallKeyP, charID, cutoff, limit).StringSlice()
	if err != nil {
		return nil, err
	}
	out := make([]Listener, 0, len(res)/3)
	for i := 0; i+2 < len(res); i += 3 {
		hid, _ := strconv.Atoi(res[i+1])
		seat, _ := strconv.Atoi(res[i+2])
		out = append(out, Listener{CharacterID: res[i], HallID: hid, Seat: seat})
	}
	return out, nil
}

// Switch moves a present listener to another open điện that has room. Moving to the điện they are in is a no-op
// (moved=false) — the request is idempotent by its target.
func (h *Hall) Switch(ctx context.Context, charID string, target int) (from Listener, seat int, moved bool, err error) {
	res, err := switchScript.Run(ctx, h.R, h.keys(), hallKeyP, charID, target, h.Cfg.Seats).Int64Slice()
	if err != nil {
		return Listener{}, 0, false, err
	}
	if len(res) != 4 {
		return Listener{}, 0, false, errors.New("hall: switch: unexpected reply")
	}
	from = Listener{CharacterID: charID, HallID: int(res[1]), Seat: int(res[2])}
	switch res[0] {
	case -1:
		return from, 0, false, ErrNotPresent
	case -2:
		return from, 0, false, ErrHallUnknown
	case -3:
		return from, 0, false, ErrHallFull
	}
	return from, int(res[3]), res[0] == 1, nil
}

// Merge folds near-empty điện into others after a lecture boundary (no-op once that number is fully merged, so any
// number of API instances may call it; a large backlog is spread over several calls). Returns who moved where.
func (h *Hall) Merge(ctx context.Context, lectureNo int64) ([]Move, error) {
	if h.Cfg.MergeBelow <= 0 {
		return nil, nil
	}
	res, err := mergeScript.Run(ctx, h.R, append(h.keys(), keyMerged), hallKeyP, lectureNo, h.Cfg.Seats, h.Cfg.MergeBelow, mergeBudget).StringSlice()
	if err != nil {
		return nil, err
	}
	out := make([]Move, 0, len(res)/5)
	for i := 0; i+4 < len(res); i += 5 {
		n := make([]int, 4)
		for j := range n {
			n[j], _ = strconv.Atoi(res[i+1+j])
		}
		out = append(out, Move{CharacterID: res[i], From: n[0], FromSeat: n[1], To: n[2], ToSeat: n[3]})
	}
	return out, nil
}

// Counts returns the global listener count and every open điện with its listeners, by number.
func (h *Hall) Counts(ctx context.Context) (total int64, halls []HallCount, err error) {
	res, err := countsScript.Run(ctx, h.R, h.keys(), hallKeyP).Int64Slice()
	if err != nil {
		return 0, nil, err
	}
	halls = make([]HallCount, 0, len(res)/2)
	for i := 1; i+1 < len(res); i += 2 {
		halls = append(halls, HallCount{ID: int(res[i]), Count: int(res[i+1])})
	}
	sort.Slice(halls, func(a, b int) bool { return halls[a].ID < halls[b].ID })
	return res[0], halls, nil
}

// Snapshot returns the seated listeners of one điện (by seat).
func (h *Hall) Snapshot(ctx context.Context, hallID int) ([]Listener, error) {
	seats, err := h.R.HGetAll(ctx, hallKeyP+strconv.Itoa(hallID)+":seat").Result()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(seats))
	seatOf := map[string]int{}
	for s, id := range seats {
		n, _ := strconv.Atoi(s)
		ids = append(ids, id)
		seatOf[id] = n
	}
	out := []Listener{}
	if len(ids) == 0 {
		return out, nil
	}
	vals, err := h.R.HMGet(ctx, keyWho, ids...).Result()
	if err != nil {
		return nil, err
	}
	for i, v := range vals {
		var l Listener
		if s, ok := v.(string); ok {
			_ = json.Unmarshal([]byte(s), &l)
		}
		l.CharacterID, l.HallID, l.Seat = ids[i], hallID, seatOf[ids[i]]
		out = append(out, l)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Seat < out[b].Seat })
	return out, nil
}

// Where returns the character's điện and seat, and whether they are in the hall.
func (h *Hall) Where(ctx context.Context, charID string) (hallID, seat int, ok bool, err error) {
	v, err := h.R.HGet(ctx, keyAt, charID).Result()
	if errors.Is(err, redis.Nil) {
		return 0, -1, false, nil
	} else if err != nil {
		return 0, -1, false, err
	}
	var a, b int
	for i := range len(v) {
		if v[i] == ':' {
			a, _ = strconv.Atoi(v[:i])
			b, _ = strconv.Atoi(v[i+1:])
			return a, b, a > 0, nil
		}
	}
	return 0, -1, false, nil
}

// Listener returns one listener's stored card (name, stage, look) at their current cushion.
func (h *Hall) Listener(ctx context.Context, charID string) (Listener, bool, error) {
	hid, seat, ok, err := h.Where(ctx, charID)
	if err != nil || !ok {
		return Listener{}, false, err
	}
	var l Listener
	if s, err := h.R.HGet(ctx, keyWho, charID).Result(); err == nil {
		_ = json.Unmarshal([]byte(s), &l)
	}
	l.CharacterID, l.HallID, l.Seat = charID, hid, seat
	return l, true, nil
}
