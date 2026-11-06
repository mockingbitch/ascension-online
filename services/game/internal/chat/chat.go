// Package chat holds the chat catalogue (content/catalog/chat.yaml), pure message rules and (service.go) the
// send / history / moderation transactions. Delivery to clients goes through realtime (Redis pub/sub → WS).
package chat

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
	"gopkg.in/yaml.v3"
)

// Channel is one chat channel.
type Channel struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MinStage string `json:"minStage"`
}

// Rate is the per-player posting limit.
type Rate struct {
	Messages int
	Per      time.Duration
	Burst    int
}

// Catalogue is the validated chat config.
type Catalogue struct {
	Channels        []Channel
	byID            map[string]Channel
	MaxChars        int
	HistorySize     int
	Retention       time.Duration
	DuplicateWindow time.Duration
	MaskCategories  []string
	Rate            Rate
	MuteMax         time.Duration
}

// Channel looks up a channel by id.
func (c *Catalogue) Channel(id string) (Channel, bool) {
	ch, ok := c.byID[id]
	return ch, ok
}

var idRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type file struct {
	Version  int `yaml:"version"`
	Channels []struct {
		ID       string `yaml:"id"`
		Name     string `yaml:"name"`
		MinStage string `yaml:"min_stage"`
	} `yaml:"channels"`
	MaxChars               int      `yaml:"max_chars"`
	HistorySize            int      `yaml:"history_size"`
	RetentionDays          int      `yaml:"retention_days"`
	DuplicateWindowSeconds int      `yaml:"duplicate_window_seconds"`
	MaskCategories         []string `yaml:"mask_categories"`
	Rate                   struct {
		Messages   int `yaml:"messages"`
		PerSeconds int `yaml:"per_seconds"`
		Burst      int `yaml:"burst"`
	} `yaml:"rate"`
	Mute struct {
		MaxHours int `yaml:"max_hours"`
	} `yaml:"mute"`
}

// LoadFile parses and validates chat.yaml. Stage and mask-category references are checked in content.LoadAll.
func LoadFile(path string) (*Catalogue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Load(f)
}

// Load parses and validates chat YAML, collecting every error.
func Load(r io.Reader) (*Catalogue, error) {
	var raw file
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("chat: decode: %w", err)
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if raw.Version != 1 {
		fail("unsupported version %d", raw.Version)
	}
	c := &Catalogue{byID: map[string]Channel{}}
	if len(raw.Channels) == 0 {
		fail("channels: at least one channel required")
	}
	for i, ch := range raw.Channels {
		if !idRe.MatchString(ch.ID) || len(ch.ID) > 32 {
			fail("channels[%d]: id %q must match %s (≤ 32 chars)", i, ch.ID, idRe)
		}
		if _, dup := c.byID[ch.ID]; dup {
			fail("channels[%d]: duplicate id %q", i, ch.ID)
		}
		if strings.TrimSpace(ch.Name) == "" || ch.MinStage == "" {
			fail("channels[%d]: name and min_stage are required", i)
		}
		v := Channel{ID: ch.ID, Name: ch.Name, MinStage: ch.MinStage}
		c.byID[ch.ID] = v
		c.Channels = append(c.Channels, v)
	}
	if raw.MaxChars < 1 || raw.MaxChars > 500 {
		fail("max_chars must be within 1..500 (DB limit)")
	}
	if raw.HistorySize < 10 || raw.HistorySize > 200 {
		fail("history_size must be within 10..200")
	}
	if raw.RetentionDays < 7 || raw.RetentionDays > 365 {
		fail("retention_days must be within 7..365")
	}
	if raw.DuplicateWindowSeconds < 0 || raw.DuplicateWindowSeconds > 3600 {
		fail("duplicate_window_seconds must be within 0..3600")
	}
	if raw.Rate.Messages < 1 || raw.Rate.PerSeconds < 1 || raw.Rate.Burst < 1 || raw.Rate.Burst > 50 {
		fail("rate: messages ≥ 1, per_seconds ≥ 1, burst 1..50")
	}
	if raw.Mute.MaxHours < 1 || raw.Mute.MaxHours > 24*365 {
		fail("mute.max_hours must be within 1..8760")
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("chat: invalid catalogue: %w", errors.Join(errs...))
	}
	c.MaxChars, c.HistorySize = raw.MaxChars, raw.HistorySize
	c.Retention = time.Duration(raw.RetentionDays) * 24 * time.Hour
	c.DuplicateWindow = time.Duration(raw.DuplicateWindowSeconds) * time.Second
	c.MaskCategories = raw.MaskCategories
	c.Rate = Rate{Messages: raw.Rate.Messages, Per: time.Duration(raw.Rate.PerSeconds) * time.Second, Burst: raw.Rate.Burst}
	c.MuteMax = time.Duration(raw.Mute.MaxHours) * time.Hour
	return c, nil
}

// ErrInvalidText: empty after cleaning, too long, or invalid UTF-8.
var ErrInvalidText = errors.New("chat: message empty or too long")

// CleanText normalises a message: NFC, control/format characters removed (incl. zero-width and bidi overrides),
// all whitespace runs collapsed to one space (single-line chat), trimmed. Pure.
func CleanText(raw string, maxChars int) (string, error) {
	if !utf8.ValidString(raw) {
		return "", ErrInvalidText
	}
	var b strings.Builder
	space := false
	for _, r := range norm.NFC.String(raw) {
		switch {
		case unicode.IsSpace(r):
			space = true
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError:
			// dropped: invisible / layout-breaking characters
		default:
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		}
	}
	s := b.String()
	if n := utf8.RuneCountInString(s); n == 0 || n > maxChars {
		return "", ErrInvalidText
	}
	return s, nil
}
