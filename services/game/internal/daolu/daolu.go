// Package daolu is đạo lữ song tu (Phase 5.1, cot-truyen §5.6, plan §3.11): a permanent bond between two characters
// (propose → accept with a double "vĩnh viễn" confirmation; one active bond per character, enforced by the
// dao_lu_members partial unique index) and song tu — overlap time while both partners are active, accrued in Redis on
// the server clock and claimed by each partner into the community bucket (cultivation.CreditLocked source
// dual_cultivation). No voluntary divorce: the docs only allow lượng kiếp (luân hồi / Phong Thần — bond.EndForCharacterLocked
// is the hook for those phases), an admin after a report, and absence_days without play.
//
// This package is the catalogue (dao_lu.yaml) and pure rules; the runtime service is internal/daolu/bond.
package daolu

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/fixedpoint"
)

// Bond is dao_lu.yaml bond.
type Bond struct {
	MinAccountAgeDays   int64 `yaml:"min_account_age_days"`
	MinTier             int   `yaml:"min_tier"`
	ProposalExpiryHours int64 `yaml:"proposal_expiry_hours"`
	DailyProposals      int32 `yaml:"daily_proposals"`
	IncomingListLimit   int32 `yaml:"incoming_list_limit"`
	MaxIncomingPending  int32 `yaml:"max_incoming_pending"`
	DeclineCooldownDays int64 `yaml:"decline_cooldown_days"`
	AbsenceDays         int64 `yaml:"absence_days"`
	RebondCooldownDays  int64 `yaml:"rebond_cooldown_days"`
}

// SongTu is dao_lu.yaml song_tu.
type SongTu struct {
	WindowSeconds    int64 `yaml:"window_seconds"`
	RateMilliPerHour int64 `yaml:"rate_milli_per_hour"`
	HeartbeatSeconds int64 `yaml:"heartbeat_seconds"`
	MaxGapSeconds    int64 `yaml:"max_gap_seconds"`
	DuyenMinSeconds  int64 `yaml:"duyen_min_seconds"`
}

// Config is content/catalog/dao_lu.yaml.
type Config struct {
	Version int    `yaml:"version"`
	Bond    Bond   `yaml:"bond"`
	SongTu  SongTu `yaml:"song_tu"`
}

const day = 24 * time.Hour

// MinAccountAge is the minimum Google account age of both partners.
func (c *Config) MinAccountAge() time.Duration { return time.Duration(c.Bond.MinAccountAgeDays) * day }

// ProposalExpiry is a proposal's lifetime.
func (c *Config) ProposalExpiry() time.Duration {
	return time.Duration(c.Bond.ProposalExpiryHours) * time.Hour
}

// DeclineCooldown is how long a proposer waits after the same target declined.
func (c *Config) DeclineCooldown() time.Duration {
	return time.Duration(c.Bond.DeclineCooldownDays) * day
}

// Absence is how long without play ends a bond.
func (c *Config) Absence() time.Duration { return time.Duration(c.Bond.AbsenceDays) * day }

// RebondCooldown is the wait after a bond ended.
func (c *Config) RebondCooldown() time.Duration {
	return time.Duration(c.Bond.RebondCooldownDays) * day
}

// MaxSongTuMilli is the most a character can be paid by song tu in one game day (raw, before caps).
func (c *Config) MaxSongTuMilli() int64 { return c.PayMilli(c.SongTu.WindowSeconds) }

// PayMilli is the raw tu vi for `secs` paid seconds in a day (cumulative: a claim pays PayMilli(after) − PayMilli(before),
// so splitting claims never loses or gains a milli). Pure.
func (c *Config) PayMilli(secs int64) int64 {
	secs = max(0, min(secs, c.SongTu.WindowSeconds))
	return fixedpoint.MulDiv(secs, c.SongTu.RateMilliPerHour, 3600)
}

// LoadFile parses and validates dao_lu.yaml.
func LoadFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates dao_lu YAML.
func Load(r io.Reader) (*Config, error) {
	var c Config
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("dao_lu: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if c.Version != 1 {
		fail("unsupported version %d", c.Version)
	}
	b, s := c.Bond, c.SongTu
	if b.MinAccountAgeDays < 0 || b.MinAccountAgeDays > 180 {
		fail("bond.min_account_age_days must be within 0..180")
	}
	if b.MinTier < 1 || b.MinTier > 20 {
		fail("bond.min_tier must be within 1..20")
	}
	if b.ProposalExpiryHours < 1 || b.ProposalExpiryHours > 24*14 {
		fail("bond.proposal_expiry_hours must be within 1..336")
	}
	if b.DailyProposals < 1 || b.DailyProposals > 20 {
		fail("bond.daily_proposals must be within 1..20")
	}
	if b.IncomingListLimit < 1 || b.IncomingListLimit > 100 {
		fail("bond.incoming_list_limit must be within 1..100")
	}
	if b.MaxIncomingPending < 1 || b.MaxIncomingPending > b.IncomingListLimit {
		fail("bond.max_incoming_pending must be within 1..incoming_list_limit")
	}
	if b.DeclineCooldownDays < 0 || b.DeclineCooldownDays > 90 {
		fail("bond.decline_cooldown_days must be within 0..90")
	}
	if b.AbsenceDays < 30 || b.AbsenceDays > 365 {
		fail("bond.absence_days must be within 30..365")
	}
	if b.RebondCooldownDays < 0 || b.RebondCooldownDays > 90 {
		fail("bond.rebond_cooldown_days must be within 0..90")
	}
	if s.WindowSeconds < 600 || s.WindowSeconds > 4*3600 {
		fail("song_tu.window_seconds must be within 600..14400 (decision: 2h/day)")
	}
	if s.RateMilliPerHour < 1 || s.RateMilliPerHour > 20_000 {
		fail("song_tu.rate_milli_per_hour must be within 1..20000")
	}
	if s.HeartbeatSeconds < 10 || s.HeartbeatSeconds > 120 {
		fail("song_tu.heartbeat_seconds must be within 10..120")
	}
	if s.MaxGapSeconds < 2*s.HeartbeatSeconds || s.MaxGapSeconds > 4*s.HeartbeatSeconds {
		fail("song_tu.max_gap_seconds must be within 2..4 × heartbeat_seconds")
	}
	if s.DuyenMinSeconds < 0 || s.DuyenMinSeconds > s.WindowSeconds {
		fail("song_tu.duyen_min_seconds must be within 0..window_seconds")
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("dao_lu: invalid config: %w", errors.Join(errs...))
	}
	return &c, nil
}

// Gate reasons (also the API's `blockers` codes).
const (
	BlockAccountAge  = "account_age"  // a Google account is younger than min_account_age_days
	BlockTier        = "tier"         // a character is below min_tier
	BlockBonded      = "bonded"       // already in an active bond
	BlockCooldown    = "cooldown"     // a bond ended less than rebond_cooldown_days ago
	BlockSelf        = "self"         // proposing to oneself
	BlockNoCharacter = "no_character" // target not found
)

// Party is what the gates need about one side.
type Party struct {
	AccountCreatedAt time.Time
	Tier             int
	Bonded           bool
	LastEndedAt      time.Time // zero/epoch = never
}

// Blockers lists why one side cannot bond now (empty = may). Pure.
func (c *Config) Blockers(p Party, now time.Time) []string {
	out := []string{}
	if now.Sub(p.AccountCreatedAt) < c.MinAccountAge() {
		out = append(out, BlockAccountAge)
	}
	if p.Tier < c.Bond.MinTier {
		out = append(out, BlockTier)
	}
	if p.Bonded {
		out = append(out, BlockBonded)
	}
	if !p.LastEndedAt.IsZero() && p.LastEndedAt.Unix() > 0 && now.Sub(p.LastEndedAt) < c.RebondCooldown() {
		out = append(out, BlockCooldown)
	}
	return out
}

// CooldownUntil is when a party may bond again after its last ending (zero when not cooling down).
func (c *Config) CooldownUntil(lastEnded, now time.Time) time.Time {
	if lastEnded.IsZero() || lastEnded.Unix() <= 0 {
		return time.Time{}
	}
	if u := lastEnded.Add(c.RebondCooldown()); now.Before(u) {
		return u
	}
	return time.Time{}
}
