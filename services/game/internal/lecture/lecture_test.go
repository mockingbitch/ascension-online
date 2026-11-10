package lecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const shipped = "../../../../content/catalog/lectures.yaml"

func cfg(t *testing.T) *Config {
	t.Helper()
	c, err := LoadFile(shipped)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestShippedConfig(t *testing.T) {
	c := cfg(t)
	// Balance guard: lectures keep the 20 points/day share (balance-the-gioi-chung.md §3).
	if c.DailyCapMilli != 20_000 {
		t.Fatalf("daily cap = %d, want 20000", c.DailyCapMilli)
	}
	// About two hours of listening (12 insights of 10 minutes, plus a 13th for the truncated milli) reach the cap.
	var total int64
	for range 13 {
		total += c.Pay(600, total)
	}
	if total != c.DailyCapMilli {
		t.Fatalf("2h pays %d", total)
	}
}

func TestSlotAt(t *testing.T) {
	c := cfg(t)
	at := func(d time.Duration) time.Time { return c.Epoch.Add(d) }
	tests := []struct {
		name string
		off  time.Duration
		no   int64
		open bool
	}{
		{"epoch opens slot 0", 0, 0, true},
		{"inside window", c.InsightWindow - time.Second, 0, true},
		{"window just closed", c.InsightWindow, 0, false},
		{"next slot", c.InsightEvery, 1, true},
		{"before epoch", -time.Hour, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, open := c.SlotAt(at(tt.off))
			if s.No != tt.no || open != tt.open {
				t.Fatalf("got slot %d open %v, want %d %v", s.No, open, tt.no, tt.open)
			}
		})
	}
	if n := c.NextSlot(at(time.Minute)); n.No != 1 || !n.OpensAt.Equal(at(c.InsightEvery)) {
		t.Fatalf("next slot = %+v", n)
	}
}

func TestPay(t *testing.T) {
	c := cfg(t)
	tests := []struct {
		name         string
		secs, before int64
		want         int64
	}{
		{"nothing", 0, 0, 0},
		{"negative", -5, 0, 0},
		{"ten minutes", 600, 0, 600 * c.RateMilliPerHour / 3600},
		{"per-insight cap", 100_000, 0, c.MaxSecondsPerInsight * c.RateMilliPerHour / 3600},
		{"daily cap left", 600, c.DailyCapMilli - 100, 100},
		{"daily cap reached", 600, c.DailyCapMilli, 0},
		{"over cap (data drift)", 600, c.DailyCapMilli + 5, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.Pay(tt.secs, tt.before); got != tt.want {
				t.Fatalf("Pay(%d,%d) = %d, want %d", tt.secs, tt.before, got, tt.want)
			}
		})
	}
}

// Splitting the same listening across many insights never pays more than the daily cap.
func TestPaySplitNeverExceedsCap(t *testing.T) {
	c := cfg(t)
	var total int64
	for range 1000 {
		total += c.Pay(c.MaxSecondsPerInsight, total)
	}
	if total != c.DailyCapMilli {
		t.Fatalf("total %d, want %d", total, c.DailyCapMilli)
	}
}

func TestMilestonesCrossed(t *testing.T) {
	c := cfg(t)
	if got := c.MilestonesCrossed(0, 9_999); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	if got := c.MilestonesCrossed(9_999, 10_000); len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	if got := c.MilestonesCrossed(0, 20_000); len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	if got := c.MilestonesCrossed(10_000, 20_000); len(got) != 1 || got[0] != 20_000 {
		t.Fatalf("got %v", got)
	}
}

func TestLoadRejects(t *testing.T) {
	b, err := os.ReadFile(shipped)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(b), "window_seconds: 120", "window_seconds: 900", 1)
	if _, err := Load(strings.NewReader(bad)); err == nil {
		t.Fatal("window ≥ every accepted")
	}
	for _, v := range []string{"attend_min_seconds: 30", "attend_min_seconds: 721"} {
		bad = strings.Replace(string(b), "attend_min_seconds: 300", v, 1)
		if _, err := Load(strings.NewReader(bad)); err == nil || !strings.Contains(err.Error(), "attend_min_seconds") {
			t.Fatalf("%s accepted: %v", v, err)
		}
	}
	bad = strings.Replace(string(b), "seats: 60", "seats: 60\n  extra: 1", 1)
	if _, err := Load(strings.NewReader(bad)); err == nil {
		t.Fatal("unknown key accepted")
	}
	for _, tc := range []struct{ from, to, want string }{
		{"merge_below: 20", "merge_below: 60", "merge_below"},
		{"count_push_seconds: 5", "count_push_seconds: 0", "count_push_seconds"},
		{"max_halls: 500", "max_halls: 0", "max_halls"},
		{"burst: 3 }", "burst: 0 }", "hall.switch"},
	} {
		bad = strings.Replace(string(b), tc.from, tc.to, 1)
		if bad == string(b) {
			t.Fatalf("shipped file has no %q", tc.from)
		}
		if _, err := Load(strings.NewReader(bad)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s accepted: %v", tc.to, err)
		}
	}
}

func TestShippedHallConfig(t *testing.T) {
	c := cfg(t)
	if c.Seats != 60 || c.MergeBelow != 20 || c.CountPush != 5*time.Second || c.MaxHalls < 1 || c.SwitchBurst < 1 {
		t.Fatalf("hall config %+v", c)
	}
}

func writeScript(t *testing.T, dir, name, id string, order int, lines int) {
	t.Helper()
	var b strings.Builder
	b.WriteString("id: " + id + "\norder: " + string(rune('0'+order)) + "\ntitle: T\ntheme: Th\nlines:\n")
	for range lines {
		b.WriteString("  - { text: \"Câu giảng.\", seconds: 10 }\n")
	}
	b.WriteString("question: \"Hỏi?\"\ntakeaway: \"Đạo.\"\n")
	if err := os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScriptsAt(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "02_b.yaml", "b", 2, 6) // 60 s
	writeScript(t, dir, "01_a.yaml", "a", 1, 5) // 50 s
	sc, err := LoadScripts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if sc.Total != 110*time.Second || sc.List[0].ID != "a" {
		t.Fatalf("total %v first %s", sc.Total, sc.List[0].ID)
	}
	epoch := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		off  time.Duration
		id   string
		into time.Duration
	}{
		{0, "a", 0}, {49 * time.Second, "a", 49 * time.Second}, {50 * time.Second, "b", 0},
		{110 * time.Second, "a", 0}, {-10 * time.Second, "b", 50 * time.Second},
	} {
		s, into := sc.At(epoch, epoch.Add(tt.off))
		if s.ID != tt.id || into != tt.into {
			t.Fatalf("off %v: got %s+%v, want %s+%v", tt.off, s.ID, into, tt.id, tt.into)
		}
	}
	if _, ok := sc.Get("b"); !ok {
		t.Fatal("get b")
	}
	// Index: the running lecture number changes exactly at each lecture boundary, monotonic across loops.
	for _, tt := range []struct {
		off time.Duration
		n   int64
	}{{0, 0}, {49 * time.Second, 0}, {50 * time.Second, 1}, {109 * time.Second, 1}, {110 * time.Second, 2}, {-10 * time.Second, -1}, {-70 * time.Second, -2}} {
		if got := sc.Index(epoch, epoch.Add(tt.off)); got != tt.n {
			t.Fatalf("index at %v = %d, want %d", tt.off, got, tt.n)
		}
	}
	var nilScripts *Scripts
	if s, _ := nilScripts.At(epoch, epoch); s != nil {
		t.Fatal("nil scripts")
	}
	if _, ok := nilScripts.Get("a"); ok {
		t.Fatal("nil get")
	}
}

func TestScriptsReject(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "01_a.yaml", "a", 1, 5)
	writeScript(t, dir, "02_b.yaml", "a", 1, 5) // dup id and order
	_, err := LoadScripts(dir)
	if err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Fatalf("got %v", err)
	}
}

func TestShippedScripts(t *testing.T) {
	dir := "../../../../content/story/lectures"
	if _, err := os.Stat(dir); err != nil {
		t.Skip("no lecture scripts yet")
	}
	sc, err := LoadScripts(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sc.List {
		if s.Seconds < 240 || s.Seconds > 360 {
			t.Errorf("%s: %ds, want 240..360", s.ID, s.Seconds)
		}
	}
}
