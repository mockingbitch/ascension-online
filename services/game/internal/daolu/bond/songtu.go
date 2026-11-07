package bond

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"tiendao/game/internal/daolu"
	"tiendao/game/internal/gameday"
	"tiendao/game/internal/protector"
)

// Session is the Redis side of song tu: one HASH per bond and game day (data-model §10 style, rebuildable / safe to
// lose — a lost key costs the partners at most the overlap they had not claimed yet; the DB caps what is ever paid).
//
//	daolu:songtu:<bondId>:<yyyymmdd>  HASH
//	  started        unix s the session was opened (by either partner, while both were active)
//	  beat:<charId>  unix s of that partner's last heartbeat recorded while active
//	  last           unix s up to which overlap has been considered
//	  secs           shared overlap seconds (capped at window_seconds)
//	  taken:<charId> seconds of `secs` that partner has already claimed
//
// Accrual (server clock, in Lua): on any heartbeat/claim at `now`, the gap since `last` is added to `secs` only when
// the gap ≤ max_gap_seconds, BOTH users are active (protector.ActiveKey exists: a real game mutation within the
// activity window — a heartbeat is not one) and BOTH partners' last recorded beats are within max_gap_seconds of now.
// So each partner's share is the same shared overlap, claimed separately; a partner who is offline or idle stops the
// accrual for both (at most one max_gap of trailing time after the last beat). No ticker: only requests move it.
type Session struct {
	R   *redis.Client
	Cfg *daolu.Config
}

const sessionTTL = 26 * time.Hour

func sessionKey(bondID uuid.UUID, now time.Time) string {
	return "daolu:songtu:" + bondID.String() + ":" + gameday.Of(now).Format("20060102")
}

// Session errors.
var (
	ErrNotActive        = errors.New("daolu: you have made no game action recently (not active)")
	ErrPartnerNotActive = errors.New("daolu: your partner is not active")
	ErrNoSession        = errors.New("daolu: no song tu session today (start one)")
)

// Lua: start. KEYS hash, activeMe, activePartner. ARGV me, now, maxGap, ttl.
var startScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[2]) == 0 then return -1 end
if redis.call('EXISTS', KEYS[3]) == 0 then return -2 end
local now = tonumber(ARGV[2])
redis.call('HSETNX', KEYS[1], 'started', now)
redis.call('HSET', KEYS[1], 'beat:' .. ARGV[1], now)
local last = tonumber(redis.call('HGET', KEYS[1], 'last') or '0')
if now - last > tonumber(ARGV[3]) then redis.call('HSET', KEYS[1], 'last', now) end
redis.call('HSETNX', KEYS[1], 'secs', 0)
redis.call('EXPIRE', KEYS[1], ARGV[4])
return 1
`)

// Lua: beat (ARGV[7] = '1' also takes the caller's unclaimed seconds). KEYS hash, activeMe, activePartner.
// ARGV me, partner, now, maxGap, window, ttl, take. Returns {secs, unclaimed, meActive, partnerActive, partnerBeat}
// or {-3} when no session was started today.
var beatScript = redis.NewScript(`
if redis.call('HEXISTS', KEYS[1], 'started') == 0 then return {-3} end
local now = tonumber(ARGV[3])
local gap = tonumber(ARGV[4])
local meA = redis.call('EXISTS', KEYS[2])
local pA = redis.call('EXISTS', KEYS[3])
if meA == 1 then redis.call('HSET', KEYS[1], 'beat:' .. ARGV[1], now) end
local bMe = tonumber(redis.call('HGET', KEYS[1], 'beat:' .. ARGV[1]) or '0')
local bP = tonumber(redis.call('HGET', KEYS[1], 'beat:' .. ARGV[2]) or '0')
local last = tonumber(redis.call('HGET', KEYS[1], 'last') or '0')
local secs = tonumber(redis.call('HGET', KEYS[1], 'secs') or '0')
local d = now - last
if d > 0 then
  if d <= gap and meA == 1 and pA == 1 and now - bMe <= gap and now - bP <= gap then
    secs = math.min(tonumber(ARGV[5]), secs + d)
    redis.call('HSET', KEYS[1], 'secs', secs)
  end
  redis.call('HSET', KEYS[1], 'last', now)
end
local taken = tonumber(redis.call('HGET', KEYS[1], 'taken:' .. ARGV[1]) or '0')
local un = math.max(0, secs - taken)
if ARGV[7] == '1' then redis.call('HSET', KEYS[1], 'taken:' .. ARGV[1], math.max(taken, secs)) end
redis.call('EXPIRE', KEYS[1], ARGV[6])
return {secs, un, meA, pA, bP}
`)

// Lua: give back taken seconds after a failed payment. KEYS hash. ARGV me, n.
var restoreScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then return 0 end
local t = tonumber(redis.call('HGET', KEYS[1], 'taken:' .. ARGV[1]) or '0') - tonumber(ARGV[2])
redis.call('HSET', KEYS[1], 'taken:' .. ARGV[1], math.max(0, t))
return 1
`)

// Pair identifies the two partners of a session from the caller's side.
type Pair struct {
	BondID      uuid.UUID
	MeChar      uuid.UUID
	MeUser      uuid.UUID
	PartnerChar uuid.UUID
	PartnerUser uuid.UUID
}

func (p Pair) keys(now time.Time) []string {
	return []string{sessionKey(p.BondID, now), protector.ActiveKey(p.MeUser), protector.ActiveKey(p.PartnerUser)}
}

// Beat is one session reading.
type Beat struct {
	Started       bool
	Secs          int64 // shared overlap today
	Unclaimed     int64 // the caller's unclaimed part of Secs
	MeActive      bool
	PartnerActive bool
	PartnerBeatAt time.Time // zero = never today
}

// Start opens today's session (idempotent) when both partners are active.
func (s *Session) Start(ctx context.Context, p Pair, now time.Time) error {
	n, err := startScript.Run(ctx, s.R, p.keys(now), p.MeChar.String(), now.Unix(), s.Cfg.SongTu.MaxGapSeconds, int64(sessionTTL/time.Second)).Int64()
	switch {
	case err != nil:
		return err
	case n == -1:
		return ErrNotActive
	case n == -2:
		return ErrPartnerNotActive
	}
	return nil
}

// Beat records the caller's heartbeat (when active) and accrues overlap up to now.
func (s *Session) Beat(ctx context.Context, p Pair, now time.Time) (Beat, error) {
	return s.run(ctx, p, now, false)
}

// Take accrues up to now, then marks the caller's unclaimed seconds as taken and returns them in Unclaimed.
func (s *Session) Take(ctx context.Context, p Pair, now time.Time) (Beat, error) {
	return s.run(ctx, p, now, true)
}

func (s *Session) run(ctx context.Context, p Pair, now time.Time, take bool) (Beat, error) {
	t := "0"
	if take {
		t = "1"
	}
	res, err := beatScript.Run(ctx, s.R, p.keys(now), p.MeChar.String(), p.PartnerChar.String(), now.Unix(),
		s.Cfg.SongTu.MaxGapSeconds, s.Cfg.SongTu.WindowSeconds, int64(sessionTTL/time.Second), t).Int64Slice()
	if err != nil {
		return Beat{}, err
	}
	if len(res) == 1 {
		return Beat{}, ErrNoSession
	}
	b := Beat{Started: true, Secs: res[0], Unclaimed: res[1], MeActive: res[2] == 1, PartnerActive: res[3] == 1}
	if res[4] > 0 {
		b.PartnerBeatAt = time.Unix(res[4], 0).UTC()
	}
	return b, nil
}

// Restore gives back n taken seconds (best effort, after a failed payment transaction).
func (s *Session) Restore(ctx context.Context, p Pair, now time.Time, n int64) error {
	if n <= 0 {
		return nil
	}
	return restoreScript.Run(ctx, s.R, []string{sessionKey(p.BondID, now)}, p.MeChar.String(), n).Err()
}

// Peek reads the session without accruing or beating (GET views).
func (s *Session) Peek(ctx context.Context, p Pair, now time.Time) (Beat, error) {
	pipe := s.R.Pipeline()
	h := pipe.HMGet(ctx, sessionKey(p.BondID, now), "started", "secs", "taken:"+p.MeChar.String(), "beat:"+p.PartnerChar.String())
	me := pipe.Exists(ctx, protector.ActiveKey(p.MeUser))
	pa := pipe.Exists(ctx, protector.ActiveKey(p.PartnerUser))
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return Beat{}, err
	}
	v := h.Val()
	num := func(i int) int64 {
		if i < len(v) {
			if str, ok := v[i].(string); ok {
				var n int64
				for _, c := range str {
					if c < '0' || c > '9' {
						return 0
					}
					n = n*10 + int64(c-'0')
				}
				return n
			}
		}
		return 0
	}
	b := Beat{Started: num(0) > 0, Secs: num(1), MeActive: me.Val() == 1, PartnerActive: pa.Val() == 1}
	b.Unclaimed = max(0, b.Secs-num(2))
	if pb := num(3); pb > 0 {
		b.PartnerBeatAt = time.Unix(pb, 0).UTC()
	}
	return b, nil
}
