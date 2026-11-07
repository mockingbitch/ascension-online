// Package protector is hộ pháp (Phase 3.4): online fellow faction members guard a tribulation and add the
// "protector" part of the tribulation odds (decision F: +protector_bonus_per_member_bp per counted member, capped by
// protector_bonus_max_bp, inside the 12% total and the 90% final cap — all in realms.yaml). This package owns the
// invite/accept flow, presence (Redis, TTL) and the counting the breakthrough uses; it never computes odds itself.
package protector

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is content/catalog/protector.yaml.
type Config struct {
	Version             int   `yaml:"version"`
	InviteWindowSeconds int64 `yaml:"invite_window_seconds"`
	DailyAssists        int32 `yaml:"daily_assists"`
	PresenceTTLSeconds  int64 `yaml:"presence_ttl_seconds"`
	PingEverySeconds    int64 `yaml:"ping_every_seconds"`
	ListLimit           int32 `yaml:"list_limit"`
	// Anti-cheat M2: presence must be backed by a recent game action; protectors need a minimum realm and account age.
	ActivityWindowSeconds int64 `yaml:"activity_window_seconds"`
	MinTier               int   `yaml:"min_tier"`
	MinCharacterAgeDays   int64 `yaml:"min_character_age_days"`
}

// ActivityWindow is how long a game action keeps pings valid.
func (c *Config) ActivityWindow() time.Duration {
	return time.Duration(c.ActivityWindowSeconds) * time.Second
}

// MinCharacterAge is the minimum age of a protector's character.
func (c *Config) MinCharacterAge() time.Duration {
	return time.Duration(c.MinCharacterAgeDays) * 24 * time.Hour
}

// InviteWindow is the invite lifetime.
func (c *Config) InviteWindow() time.Duration {
	return time.Duration(c.InviteWindowSeconds) * time.Second
}

// PresenceTTL is how long a ping keeps a character online.
func (c *Config) PresenceTTL() time.Duration {
	return time.Duration(c.PresenceTTLSeconds) * time.Second
}

// LoadFile parses and validates protector.yaml.
func LoadFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates protector YAML.
func Load(r io.Reader) (*Config, error) {
	var c Config
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("protector: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if c.Version != 1 {
		fail("unsupported version %d", c.Version)
	}
	if c.InviteWindowSeconds < 60 || c.InviteWindowSeconds > 6*3600 {
		fail("invite_window_seconds must be within 60..21600")
	}
	if c.DailyAssists < 1 || c.DailyAssists > 20 {
		fail("daily_assists must be within 1..20")
	}
	if c.PingEverySeconds < 10 || c.PresenceTTLSeconds < 2*c.PingEverySeconds || c.PresenceTTLSeconds > 900 {
		fail("need ping_every_seconds ≥ 10 and 2 × ping_every_seconds ≤ presence_ttl_seconds ≤ 900")
	}
	if c.ListLimit < 1 || c.ListLimit > 100 {
		fail("list_limit must be within 1..100")
	}
	if c.ActivityWindowSeconds < c.PresenceTTLSeconds || c.ActivityWindowSeconds > 3600 {
		fail("activity_window_seconds must be within presence_ttl_seconds..3600")
	}
	if c.MinTier < 1 || c.MinTier > 20 {
		fail("min_tier must be within 1..20")
	}
	if c.MinCharacterAgeDays < 0 || c.MinCharacterAgeDays > 90 {
		fail("min_character_age_days must be within 0..90")
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("protector: invalid config: %w", errors.Join(errs...))
	}
	return &c, nil
}

// MaxCounted is how many protectors can add odds: protector_bonus_max_bp / protector_bonus_per_member_bp (realms.yaml).
func MaxCounted(perMemberBP, maxBP int64) int {
	if perMemberBP <= 0 {
		return 0
	}
	return int(maxBP / perMemberBP)
}

// Candidate is one accepted protector of an invite, as counting needs it.
type Candidate struct {
	AssistID    int64
	CharacterID string
	FactionID   string // current faction ("" = none)
	Online      bool
}

// Count picks the protectors that count, pure: same faction as the invite (now), online, in acceptance order, at most
// max. Each protector appears at most once per invite (DB unique), so nobody is counted twice.
func Count(cands []Candidate, factionID string, maxN int) []Candidate {
	out := []Candidate{}
	seen := map[string]bool{}
	for _, c := range cands {
		if len(out) >= maxN {
			break
		}
		if seen[c.CharacterID] || c.FactionID != factionID || !c.Online {
			continue
		}
		seen[c.CharacterID] = true
		out = append(out, c)
	}
	return out
}
