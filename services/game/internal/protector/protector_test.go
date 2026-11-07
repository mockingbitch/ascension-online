package protector

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestShippedProtectorConfigIsValid(t *testing.T) {
	c, err := LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "protector.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if c.PresenceTTL() < 2*c.InviteWindow()/c.InviteWindow() {
		t.Fatal("ttl")
	}
}

func TestLoaderRejects(t *testing.T) {
	for name, y := range map[string]string{
		"ttl < 2 pings": "version: 1\ninvite_window_seconds: 600\ndaily_assists: 2\npresence_ttl_seconds: 60\nping_every_seconds: 45\nlist_limit: 10\n",
		"no assists":    "version: 1\ninvite_window_seconds: 600\ndaily_assists: 0\npresence_ttl_seconds: 120\nping_every_seconds: 45\nlist_limit: 10\n",
		"unknown key":   "version: 1\ninvite_window_seconds: 600\ndaily_assists: 2\npresence_ttl_seconds: 120\nping_every_seconds: 45\nlist_limit: 10\nbonus_bp: 100\n",
	} {
		if _, err := Load(strings.NewReader(y)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestMaxCounted(t *testing.T) {
	if MaxCounted(100, 300) != 3 || MaxCounted(0, 300) != 0 || MaxCounted(100, 250) != 2 {
		t.Fatal("max counted")
	}
}

// Offline, other-faction and duplicate protectors never count; at most max, in acceptance order.
func TestCount(t *testing.T) {
	c := []Candidate{
		{AssistID: 1, CharacterID: "a", FactionID: "f", Online: true},
		{AssistID: 2, CharacterID: "b", FactionID: "f", Online: false}, // offline
		{AssistID: 3, CharacterID: "c", FactionID: "g", Online: true},  // switched faction
		{AssistID: 4, CharacterID: "a", FactionID: "f", Online: true},  // same protector again
		{AssistID: 5, CharacterID: "d", FactionID: "f", Online: true},
		{AssistID: 6, CharacterID: "e", FactionID: "f", Online: true},
		{AssistID: 7, CharacterID: "h", FactionID: "f", Online: true}, // beyond max
	}
	got := Count(c, "f", 3)
	ids := []int64{}
	for _, g := range got {
		ids = append(ids, g.AssistID)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 5 || ids[2] != 6 {
		t.Fatalf("counted %v", ids)
	}
	if len(Count(c, "", 3)) != 0 {
		t.Fatal("no faction counts nobody")
	}
}
