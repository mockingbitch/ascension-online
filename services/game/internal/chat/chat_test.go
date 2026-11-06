package chat

import (
	"path/filepath"
	"strings"
	"testing"

	"tiendao/game/internal/realm"
)

const catalogDir = "../../../../content/catalog"

func TestShippedChatIsValid(t *testing.T) {
	c, err := LoadFile(filepath.Join(catalogDir, "chat.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	w, ok := c.Channel("world")
	if !ok || w.MinStage != "luyen_khi.tang_3" || c.MaxChars != 200 || c.Retention.Hours() != 90*24 {
		t.Fatalf("%+v %+v", w, c)
	}
}

func TestLoadRejects(t *testing.T) {
	good := "version: 1\nchannels:\n  - { id: world, name: W, min_stage: a.b }\nmax_chars: 200\nhistory_size: 50\nretention_days: 90\nduplicate_window_seconds: 30\nmask_categories: [profanity]\nrate: { messages: 1, per_seconds: 3, burst: 5 }\nmute: { max_hours: 720 }\n"
	if _, err := Load(strings.NewReader(good)); err != nil {
		t.Fatalf("good: %v", err)
	}
	for name, bad := range map[string]string{
		"bad id":        strings.Replace(good, "id: world", "id: World", 1),
		"too long":      strings.Replace(good, "max_chars: 200", "max_chars: 900", 1),
		"retention":     strings.Replace(good, "retention_days: 90", "retention_days: 1", 1),
		"burst":         strings.Replace(good, "burst: 5", "burst: 0", 1),
		"unknown field": strings.Replace(good, "max_chars: 200", "max_chars: 200\ncolour: red", 1),
		"no channel":    strings.Replace(good, "  - { id: world, name: W, min_stage: a.b }\n", "", 1),
	} {
		if _, err := Load(strings.NewReader(bad)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCleanText(t *testing.T) {
	ok := map[string]string{
		"  chào   đạo hữu  ":     "chào đạo hữu",
		"dòng 1\ndòng 2\t\tx":    "dòng 1 dòng 2 x",
		"ẩn​danh‮":               "ẩndanh",
		"Hàn":                   "Hàn",
		strings.Repeat("a", 200): strings.Repeat("a", 200),
	}
	for in, want := range ok {
		if got, err := CleanText(in, 200); err != nil || got != want {
			t.Errorf("CleanText(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "   \n ", "​​", strings.Repeat("a", 201), "\xff\xfe"} {
		if _, err := CleanText(bad, 200); err == nil {
			t.Errorf("CleanText(%q) accepted", bad)
		}
	}
}

func TestCanPost(t *testing.T) {
	realms, err := realm.LoadFile(filepath.Join(catalogDir, "realms.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	ch := Channel{ID: "world", MinStage: "luyen_khi.tang_3"}
	for stage, want := range map[string]bool{
		"luyen_khi.tang_1": false, "luyen_khi.tang_2": false, "luyen_khi.tang_3": true, "luyen_khi.tang_9": true,
		"truc_co.so_ky": true, "nope.nope": false,
	} {
		if got := CanPost(realms, ch, stage); got != want {
			t.Errorf("CanPost(%s) = %v", stage, got)
		}
	}
	// Another path compares by tier/order.
	vuFirst := realms.First("vu")
	if vuFirst != nil && CanPost(realms, ch, vuFirst.ID) {
		t.Errorf("vu first stage %s must not post", vuFirst.ID)
	}
}
