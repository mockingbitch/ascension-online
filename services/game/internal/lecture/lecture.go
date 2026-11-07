// Package lecture runs Tử Tiêu Cung: Hồng Quân lectures all day, online players sit in the hall, and every
// insight slot ("ngộ đạo") pays the listening time they accrued as tu vi through cultivation.CreditLocked.
//
// Config: content/catalog/lectures.yaml. Scripts (what Hồng Quân says): content/story/lectures/*.yaml.
// Presence and seats live in Redis (presence.go) and are rebuildable; only paid insights are durable.
package lecture

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"tiendao/game/internal/fixedpoint"
)

// Config is lectures.yaml, converted to server units.
type Config struct {
	Seats                int
	ChatChannel          string
	Epoch                time.Time
	Heartbeat            time.Duration
	MaxGap               time.Duration
	InsightEvery         time.Duration
	InsightWindow        time.Duration
	RateMilliPerHour     int64
	DailyCapMilli        int64
	MaxSecondsPerInsight int64
	VIPMilestonesMilli   []int64 // ascending; each pays vip.yaml lecture_attendance once per game day
	AttendMinSeconds     int64   // a ngộ đạo counts for the daily task "Nghe giảng" only with ≥ this much listening
}

type fileConfig struct {
	Version int `yaml:"version"`
	Hall    struct {
		Seats       int    `yaml:"seats"`
		ChatChannel string `yaml:"chat_channel"`
	} `yaml:"hall"`
	Cycle struct {
		Epoch string `yaml:"epoch"`
	} `yaml:"cycle"`
	Presence struct {
		HeartbeatSeconds int `yaml:"heartbeat_seconds"`
		MaxGapSeconds    int `yaml:"max_gap_seconds"`
	} `yaml:"presence"`
	Insight struct {
		EverySeconds  int `yaml:"every_seconds"`
		WindowSeconds int `yaml:"window_seconds"`
	} `yaml:"insight"`
	Reward struct {
		PointsPerHour        int64   `yaml:"points_per_hour"`
		DailyCapPoints       int64   `yaml:"daily_cap_points"`
		MaxSecondsPerInsight int64   `yaml:"max_seconds_per_insight"`
		VIPMilestonesPoints  []int64 `yaml:"vip_milestones_points"`
		AttendMinSeconds     int64   `yaml:"attend_min_seconds"`
	} `yaml:"reward"`
}

// LoadFile reads lectures.yaml.
func LoadFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("lectures: %w", err)
	}
	defer f.Close()
	c, err := Load(f)
	if err != nil {
		return nil, fmt.Errorf("lectures %s: %w", path, err)
	}
	return c, nil
}

// Load parses and validates lectures.yaml.
func Load(r io.Reader) (*Config, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	var raw fileConfig
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if raw.Version != 1 {
		fail("version must be 1")
	}
	epoch, err := time.Parse(time.RFC3339, raw.Cycle.Epoch)
	if err != nil {
		fail("cycle.epoch: %v", err)
	}
	h, ins, rw := raw.Presence, raw.Insight, raw.Reward
	if raw.Hall.Seats < 1 || raw.Hall.Seats > 200 {
		fail("hall.seats must be 1..200")
	}
	if raw.Hall.ChatChannel == "" {
		fail("hall.chat_channel required")
	}
	if h.HeartbeatSeconds < 5 || h.MaxGapSeconds < h.HeartbeatSeconds*2 {
		fail("presence: heartbeat_seconds ≥ 5 and max_gap_seconds ≥ 2 × heartbeat_seconds")
	}
	if ins.EverySeconds < 60 || ins.WindowSeconds < 30 || ins.WindowSeconds >= ins.EverySeconds {
		fail("insight: every_seconds ≥ 60, 30 ≤ window_seconds < every_seconds")
	}
	if rw.PointsPerHour < 1 || rw.DailyCapPoints < 1 || rw.PointsPerHour > rw.DailyCapPoints {
		fail("reward: 1 ≤ points_per_hour ≤ daily_cap_points")
	}
	if rw.MaxSecondsPerInsight < int64(ins.EverySeconds) || rw.MaxSecondsPerInsight > int64(ins.EverySeconds+ins.WindowSeconds) {
		fail("reward.max_seconds_per_insight must be within [every_seconds, every_seconds + window_seconds]")
	}
	if rw.AttendMinSeconds < 60 || rw.AttendMinSeconds > rw.MaxSecondsPerInsight {
		fail("reward.attend_min_seconds must be within 60..max_seconds_per_insight")
	}
	ms := make([]int64, 0, len(rw.VIPMilestonesPoints))
	for i, m := range rw.VIPMilestonesPoints {
		if m < 1 || m > rw.DailyCapPoints || (i > 0 && m <= rw.VIPMilestonesPoints[i-1]) {
			fail("reward.vip_milestones_points must be ascending within 1..daily_cap_points")
		}
		ms = append(ms, m*fixedpoint.MilliScale)
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return &Config{
		Seats: raw.Hall.Seats, ChatChannel: raw.Hall.ChatChannel, Epoch: epoch,
		Heartbeat: time.Duration(h.HeartbeatSeconds) * time.Second, MaxGap: time.Duration(h.MaxGapSeconds) * time.Second,
		InsightEvery: time.Duration(ins.EverySeconds) * time.Second, InsightWindow: time.Duration(ins.WindowSeconds) * time.Second,
		RateMilliPerHour: rw.PointsPerHour * fixedpoint.MilliScale, DailyCapMilli: rw.DailyCapPoints * fixedpoint.MilliScale,
		MaxSecondsPerInsight: rw.MaxSecondsPerInsight, VIPMilestonesMilli: ms, AttendMinSeconds: rw.AttendMinSeconds,
	}, nil
}

// Slot is one ngộ đạo moment. Slots repeat every InsightEvery from Epoch; each is open for InsightWindow.
type Slot struct {
	No       int64     `json:"no"`
	OpensAt  time.Time `json:"opensAt"`
	ClosesAt time.Time `json:"closesAt"`
}

// SlotAt returns the slot that started most recently at now, and whether it is still open.
func (c *Config) SlotAt(now time.Time) (Slot, bool) {
	off := now.Sub(c.Epoch)
	if off < 0 {
		off = 0
	}
	n := int64(off / c.InsightEvery)
	opens := c.Epoch.Add(time.Duration(n) * c.InsightEvery)
	s := Slot{No: n, OpensAt: opens, ClosesAt: opens.Add(c.InsightWindow)}
	return s, now.Before(s.ClosesAt) && !now.Before(c.Epoch)
}

// NextSlot is the first slot opening after now.
func (c *Config) NextSlot(now time.Time) Slot {
	s, _ := c.SlotAt(now)
	opens := s.OpensAt.Add(c.InsightEvery)
	return Slot{No: s.No + 1, OpensAt: opens, ClosesAt: opens.Add(c.InsightWindow)}
}

// Pay converts accrued listening seconds into raw tu vi (milli), clipped by the per-insight seconds cap and by
// what is left of the lecture daily cap. Pure.
func (c *Config) Pay(seconds, todayRawMilli int64) int64 {
	seconds = max(0, min(seconds, c.MaxSecondsPerInsight))
	raw := fixedpoint.MulDiv(seconds, c.RateMilliPerHour, 3600)
	return max(0, min(raw, c.DailyCapMilli-todayRawMilli))
}

// MilestonesCrossed lists the điểm duyên milestones (milli) reached when the day's lecture tu vi goes from
// before to after (before < m ≤ after). Each milestone credits vip.ActLecture once per game day (amount and daily
// max from vip.yaml).
func (c *Config) MilestonesCrossed(before, after int64) []int64 {
	var out []int64
	for _, m := range c.VIPMilestonesMilli {
		if before < m && m <= after {
			out = append(out, m)
		}
	}
	return out
}

// Line is one popup of Hồng Quân.
type Line struct {
	Text    string `yaml:"text" json:"text"`
	Seconds int    `yaml:"seconds" json:"seconds"`
}

// Script is one lecture (content/story/lectures/NN_slug.yaml).
type Script struct {
	ID       string `yaml:"id" json:"id"`
	Order    int    `yaml:"order" json:"order"`
	Title    string `yaml:"title" json:"title"`
	Theme    string `yaml:"theme" json:"theme"`
	Lines    []Line `yaml:"lines" json:"lines"`
	Question string `yaml:"question" json:"question"`
	Takeaway string `yaml:"takeaway" json:"takeaway"`

	Seconds int `yaml:"-" json:"seconds"` // Σ line seconds
}

// Scripts is the lecture loop in order.
type Scripts struct {
	List  []Script
	Total time.Duration // one full loop
	byID  map[string]*Script
}

// Get returns a script by id.
func (s *Scripts) Get(id string) (*Script, bool) {
	if s == nil {
		return nil, false
	}
	sc, ok := s.byID[id]
	return sc, ok && sc != nil
}

// At returns the script being taught at now and the offset into it. Nil scripts → nil.
func (s *Scripts) At(epoch, now time.Time) (*Script, time.Duration) {
	if s == nil || len(s.List) == 0 {
		return nil, 0
	}
	off := now.Sub(epoch) % s.Total
	if off < 0 {
		off += s.Total
	}
	for i := range s.List {
		d := time.Duration(s.List[i].Seconds) * time.Second
		if off < d {
			return &s.List[i], off
		}
		off -= d
	}
	return &s.List[len(s.List)-1], 0 // unreachable: off < Total
}

// LoadScripts reads every *.yaml in dir (README and other files are ignored).
func LoadScripts(dir string) (*Scripts, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("lecture scripts: no *.yaml in %s", dir)
	}
	sort.Strings(paths)
	out := &Scripts{byID: map[string]*Script{}}
	var errs []error
	orders := map[int]string{}
	for _, p := range paths {
		sc, err := loadScript(p)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", filepath.Base(p), err))
			continue
		}
		if _, dup := out.byID[sc.ID]; dup {
			errs = append(errs, fmt.Errorf("%s: duplicate id %q", filepath.Base(p), sc.ID))
		}
		if prev, dup := orders[sc.Order]; dup {
			errs = append(errs, fmt.Errorf("%s: order %d already used by %q", filepath.Base(p), sc.Order, prev))
		}
		orders[sc.Order] = sc.ID
		out.List = append(out.List, sc)
		out.byID[sc.ID] = nil
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("lecture scripts: %w", errors.Join(errs...))
	}
	slices.SortFunc(out.List, func(a, b Script) int { return a.Order - b.Order })
	for i := range out.List {
		out.byID[out.List[i].ID] = &out.List[i]
		out.Total += time.Duration(out.List[i].Seconds) * time.Second
	}
	return out, nil
}

func loadScript(path string) (Script, error) {
	f, err := os.Open(path)
	if err != nil {
		return Script{}, err
	}
	defer f.Close()
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	var sc Script
	if err := dec.Decode(&sc); err != nil {
		return Script{}, err
	}
	var errs []error
	fail := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }
	if !matchSlug(sc.ID) {
		fail("id %q must be ASCII snake_case", sc.ID)
	}
	if sc.Order < 1 {
		fail("order must be ≥ 1")
	}
	if sc.Title == "" || sc.Theme == "" {
		fail("title and theme required")
	}
	if n := len(sc.Lines); n < 5 || n > 60 {
		fail("lines: need 5..60, got %d", n)
	}
	for i, l := range sc.Lines {
		if n := utf8.RuneCountInString(l.Text); n < 1 || n > 160 {
			fail("line %d: text must be 1..160 characters", i+1)
		}
		if l.Seconds < 5 || l.Seconds > 30 {
			fail("line %d: seconds must be 5..30", i+1)
		}
		sc.Seconds += l.Seconds
	}
	for name, v := range map[string]string{"question": sc.Question, "takeaway": sc.Takeaway} {
		if n := utf8.RuneCountInString(v); n < 1 || n > 160 {
			fail("%s must be 1..160 characters", name)
		}
	}
	return sc, errors.Join(errs...)
}

func matchSlug(s string) bool {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return true
}
