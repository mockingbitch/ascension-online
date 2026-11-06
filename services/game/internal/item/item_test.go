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
