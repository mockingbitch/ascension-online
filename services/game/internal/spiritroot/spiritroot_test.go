package spiritroot

import (
	"path/filepath"
	"testing"
)

func TestShippedSpiritRootsAreValid(t *testing.T) {
	if _, err := LoadFile(filepath.Join("..", "..", "..", "..", "content", "catalog", "spirit_roots.yaml")); err != nil {
		t.Fatal(err)
	}
}
