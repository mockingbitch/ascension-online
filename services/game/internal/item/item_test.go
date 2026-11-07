package item

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestShippedItemsAreValid(t *testing.T) {
	c, err := LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "items.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Get("truc_co_dan"); !ok {
		t.Fatal("truc_co_dan missing")
	}
}

func TestRejectsBadItems(t *testing.T) {
	_, err := Load(strings.NewReader(`
version: 1
items:
  - { id: Bad, name: "", category: weapon, rarity: epic }
  - { id: a, name: A, category: pill, rarity: linh }
  - { id: a, name: A, category: pill, rarity: linh }
`))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"invalid id", "empty name", "category", "rarity", "duplicate"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %v", want, err)
		}
	}
}

// Pháp bảo (Phase 3.4): artifacts are never tradable, carry exactly tribulation_bonus_bp, and nothing else may raise odds.
func TestArtifactRules(t *testing.T) {
	ok := "version: 1\nitems:\n  - { id: kinh, name: Kính, category: artifact, rarity: linh, tribulation_bonus_bp: 100 }\n"
	if _, err := Load(strings.NewReader(ok)); err != nil {
		t.Fatalf("valid artifact rejected: %v", err)
	}
	for name, tc := range map[string]struct{ yaml, want string }{
		"tradable":    {"  - { id: a, name: A, category: artifact, rarity: linh, tradable: true, tribulation_bonus_bp: 100 }", "may not be tradable"},
		"no bonus":    {"  - { id: a, name: A, category: artifact, rarity: linh }", "need tribulation_bonus_bp"},
		"extra":       {"  - { id: a, name: A, category: artifact, rarity: linh, tribulation_bonus_bp: 100, breakthrough_bonus_bp: 100 }", "carry only"},
		"pill odds":   {"  - { id: a, name: A, category: pill, rarity: linh, tribulation_bonus_bp: 100 }", "only artifacts"},
		"herb bt odd": {"  - { id: a, name: A, category: herb, rarity: linh, breakthrough_bonus_bp: 100 }", "only artifacts"},
	} {
		_, err := Load(strings.NewReader("version: 1\nitems:\n" + tc.yaml + "\n"))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want %q, got %v", name, tc.want, err)
		}
	}
	c, err := LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "items.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, d := range c.All() {
		if d.Category == CategoryArtifact {
			n++
		}
	}
	if n == 0 {
		t.Fatal("shipped items.yaml has no pháp bảo")
	}
}
