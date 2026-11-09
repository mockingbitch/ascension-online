// Package announce publishes the world's news by effect tier (decisions 2026-11-09 #7 #8 #12, GĐ-D): realm
// breakthroughs and passed tribulations (never minor stage-ups), the first character to reach each realm id, entries
// into the top of Thiên Bảng, and a digest when too many arrive at once. Path: the breakthrough transaction writes an
// outbox row (+ the race-free realm_firsts row) → the worker renders the text (content/story/announcements), applies
// the pacing (content/catalog/announcements.yaml), stores a short history (world_announcements) and publishes on
// Redis → every API instance → WebSocket. Delivery is at-most-once; clients de-duplicate by id.
//
// DISPLAY ONLY: an announcement names a character by đạo hiệu (and their public realm / rank) and nothing else; it
// never feeds progress, ranking, rewards or điểm duyên.
package announce

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/realm"
	"tiendao/game/internal/realtime"
)

// Kinds stored in world_announcements.kind.
const (
	KindRealm       = "realm"
	KindTribulation = "tribulation"
	KindFirst       = "first_to_realm"
	KindRank        = "rank_top10"
	KindDigest      = "digest"
)

// WS envelope types (packages/contracts/ws-messages.md).
const (
	TypeBreakthrough = "announcement.major_breakthrough"
	TypeFirst        = "announcement.first_to_realm"
	TypeRank         = "announcement.rank_top10"
	TypeDigest       = "announcement.digest"
)

// MaxNameRunes is the longest đạo hiệu (character.NormalizeName).
const MaxNameRunes = 24

// Settings is content/catalog/announcements.yaml.
type Settings struct {
	WindowSeconds        int64 `yaml:"window_seconds"`
	MaxSeparatePerWindow int64 `yaml:"max_separate_per_window"`
	RankTop              int64 `yaml:"rank_top"`
	RankCooldownSeconds  int64 `yaml:"rank_cooldown_seconds"`
	HistoryKeepDays      int   `yaml:"history_keep_days"`
	RecentLimit          int32 `yaml:"recent_limit"`
	MaxChars             int   `yaml:"max_chars"`
}

type file struct {
	Version  int      `yaml:"version"`
	Settings Settings `yaml:"settings"`
}

// LoadFile parses and validates announcements.yaml.
func LoadFile(path string) (*Settings, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates announcements YAML.
func Load(r io.Reader) (*Settings, error) {
	var raw file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("announce: decode: %w", err)
	}
	s := raw.Settings
	var errs []error
	check := func(ok bool, msg string) {
		if !ok {
			errs = append(errs, errors.New("announce: "+msg))
		}
	}
	check(raw.Version == 1, "unsupported version")
	check(s.WindowSeconds >= 1 && s.WindowSeconds <= 600, "window_seconds must be within 1..600")
	check(s.MaxSeparatePerWindow >= 1 && s.MaxSeparatePerWindow <= 20, "max_separate_per_window must be within 1..20")
	check(s.RankTop >= 1 && s.RankTop <= 100, "rank_top must be within 1..100")
	check(s.RankCooldownSeconds >= 60, "rank_cooldown_seconds must be ≥ 60")
	check(s.HistoryKeepDays >= 1 && s.HistoryKeepDays <= 90, "history_keep_days must be within 1..90")
	check(s.RecentLimit >= 1 && s.RecentLimit <= 200, "recent_limit must be within 1..200")
	check(s.MaxChars >= 40 && s.MaxChars <= 200, "max_chars must be within 40..200")
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return &s, nil
}

// Event is the outbox payload of a successful realm breakthrough (breakthrough.recordSuccess).
type Event struct {
	CharacterID string       `json:"characterId"`
	Name        string       `json:"name"`
	RealmID     string       `json:"realmId"`
	StageID     string       `json:"stageId"`
	Path        realm.PathID `json:"path"`
	Tribulation bool         `json:"tribulation"`
	First       bool         `json:"first"`
}

// Kind of an event.
func (e Event) Kind() string {
	switch {
	case e.First:
		return KindFirst
	case e.Tribulation:
		return KindTribulation
	default:
		return KindRealm
	}
}

// TemplateKey picks the template of an event for the effect tier of its realm (realms.yaml visual.fx_tier).
func TemplateKey(e Event, fxTier int) string {
	switch e.Kind() {
	case KindFirst:
		return "first_to_realm"
	case KindTribulation:
		return fmt.Sprintf("tribulation.%s.t%d", e.Path, min(6, max(4, fxTier)))
	default:
		return fmt.Sprintf("realm.t%d", min(3, max(2, fxTier)))
	}
}

// Render renders a key with fallback to major_breakthrough.
func Render(tpl realtime.Templates, key string, vars map[string]string) string {
	if len(tpl[key]) == 0 {
		key = "major_breakthrough"
	}
	return tpl.Render(key, vars)
}

// RequiredKeys are the template keys the worker uses (checked at start).
func RequiredKeys(realms *realm.Catalogue) []string {
	keys := []string{"realm.t2", "realm.t3", "first_to_realm", "rank_top10", "rank_first", "digest", "major_breakthrough"}
	for _, p := range realms.Paths() {
		for t := 4; t <= 6; t++ {
			keys = append(keys, fmt.Sprintf("tribulation.%s.t%d", p.ID, t))
		}
	}
	return keys
}

// CheckTemplates verifies every required key exists and every variant of every key stays within maxChars after
// substitution with the worst case: the longest đạo hiệu, the longest realm name, the largest rank and count.
func CheckTemplates(tpl realtime.Templates, realms *realm.Catalogue, s *Settings) error {
	var errs []error
	for _, k := range RequiredKeys(realms) {
		if len(tpl[k]) == 0 {
			errs = append(errs, fmt.Errorf("announce: template %q missing", k))
		}
	}
	longest := ""
	for _, r := range realms.Realms() {
		if utf8.RuneCountInString(r.Name) > utf8.RuneCountInString(longest) {
			longest = r.Name
		}
	}
	vars := map[string]string{"name": strings.Repeat("Ầ", MaxNameRunes), "realm": longest,
		"rank": strconv.FormatInt(s.RankTop, 10), "count": "999"}
	for k, list := range tpl {
		for i, t := range list {
			out := t
			for v, val := range vars {
				out = strings.ReplaceAll(out, "{"+v+"}", val)
			}
			if strings.Contains(out, "{") {
				errs = append(errs, fmt.Errorf("announce: %s[%d]: unknown placeholder in %q", k, i, t))
			}
			if n := utf8.RuneCountInString(out); n > s.MaxChars {
				errs = append(errs, fmt.Errorf("announce: %s[%d]: %d characters after substitution (max %d)", k, i, n, s.MaxChars))
			}
		}
	}
	return errors.Join(errs...)
}

// RankNews decides whether a rank move is news: a character who was already on the board entering the top, or
// reaching rank 1. Ranks are 1-based; 0 = unranked. A character's first appearance on the board is never news (at
// launch every newcomer lands in the top). Returns the template key or "".
func RankNews(old, now, top int64) string {
	if old < 1 || now < 1 || now > top {
		return ""
	}
	if now == 1 && old != 1 {
		return "rank_first"
	}
	if old > top {
		return "rank_top10"
	}
	return ""
}
