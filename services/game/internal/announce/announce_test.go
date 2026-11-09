package announce

import (
	"path/filepath"
	"strings"
	"testing"

	"tiendao/game/internal/realm"
	"tiendao/game/internal/realtime"
)

const repo = "../../../.."

func TestShippedAnnouncements(t *testing.T) {
	s, err := LoadFile(filepath.Join(repo, "content/catalog/announcements.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	// decisions 2026-11-09 #8: ≤ 3 separate per 10 s; top-10 at most once per 6 h per player; 90-char lines
	if s.WindowSeconds != 10 || s.MaxSeparatePerWindow != 3 || s.RankTop != 10 || s.RankCooldownSeconds != 6*3600 || s.MaxChars != 90 {
		t.Fatalf("settings = %+v", s)
	}
	realms, err := realm.LoadFile(filepath.Join(repo, "content/catalog/realms.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := realtime.LoadTemplates(filepath.Join(repo, "content/story/announcements/major-breakthrough.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckTemplates(tpl, realms, s); err != nil {
		t.Fatal(err)
	}
}

func TestCheckTemplatesRejects(t *testing.T) {
	realms, err := realm.LoadFile(filepath.Join(repo, "content/catalog/realms.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := realtime.LoadTemplates(filepath.Join(repo, "content/story/announcements/major-breakthrough.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := &Settings{RankTop: 10, MaxChars: 90}
	long := realtime.Templates{}
	for k, v := range tpl {
		long[k] = v
	}
	long["realm.t2"] = []string{"{name} " + strings.Repeat("rất dài ", 10) + "{realm}"}
	if err := CheckTemplates(long, realms, s); err == nil || !strings.Contains(err.Error(), "characters after substitution") {
		t.Fatalf("long: %v", err)
	}
	delete(long, "digest")
	long["realm.t2"] = []string{"{name} {odds}"}
	err = CheckTemplates(long, realms, s)
	if err == nil || !strings.Contains(err.Error(), `"digest" missing`) || !strings.Contains(err.Error(), "unknown placeholder") {
		t.Fatalf("missing/placeholder: %v", err)
	}
}

func TestTemplateKeyByTier(t *testing.T) {
	for _, c := range []struct {
		e    Event
		tier int
		want string
	}{
		{Event{}, 2, "realm.t2"},
		{Event{}, 3, "realm.t3"},
		{Event{}, 1, "realm.t2"},
		{Event{Tribulation: true, Path: "tien"}, 4, "tribulation.tien.t4"},
		{Event{Tribulation: true, Path: "vu"}, 6, "tribulation.vu.t6"},
		{Event{Tribulation: true, Path: "tien", First: true}, 6, "first_to_realm"},
	} {
		if got := TemplateKey(c.e, c.tier); got != c.want {
			t.Errorf("%+v tier %d = %s, want %s", c.e, c.tier, got, c.want)
		}
	}
}

func TestRankNews(t *testing.T) {
	for _, c := range []struct {
		old, now int64
		want     string
	}{
		{11, 10, "rank_top10"}, {40, 3, "rank_top10"}, {2, 1, "rank_first"}, {15, 1, "rank_first"},
		{0, 4, ""}, // first appearance on the board: never news (launch spam)
		{0, 1, ""}, {3, 2, ""}, {1, 1, ""}, {9, 12, ""}, {5, 0, ""},
	} {
		if got := RankNews(c.old, c.now, 10); got != c.want {
			t.Errorf("%d→%d = %q, want %q", c.old, c.now, got, c.want)
		}
	}
}

func TestLoadRejects(t *testing.T) {
	for _, body := range []string{
		"version: 1\nsettings: { window_seconds: 0, max_separate_per_window: 3, rank_top: 10, rank_cooldown_seconds: 60, history_keep_days: 7, recent_limit: 50, max_chars: 90 }\n",
		"version: 1\nsettings: { window_seconds: 10, max_separate_per_window: 3, rank_top: 10, rank_cooldown_seconds: 60, history_keep_days: 7, recent_limit: 50, max_chars: 300 }\n",
		"version: 1\nsettings: { window_seconds: 10, max_separate_per_window: 3, rank_top: 10, rank_cooldown_seconds: 60, history_keep_days: 7, recent_limit: 50, max_chars: 90, bonus_bp: 1 }\n",
	} {
		if _, err := Load(strings.NewReader(body)); err == nil {
			t.Errorf("accepted: %s", body)
		}
	}
}
