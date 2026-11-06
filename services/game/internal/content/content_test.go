package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const shippedDir = "../../../../content/catalog"

func TestShippedContentIsConsistent(t *testing.T) {
	if _, err := LoadAll(shippedDir); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownItemReferenceFails(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"realms.yaml", "vip.yaml", "spirit_roots.yaml", "daily_tasks.yaml", "contribution.yaml", "world_milestones.yaml", "moderation.yaml", "chat.yaml"} {
		b, err := os.ReadFile(filepath.Join(shippedDir, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, f), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	items := "version: 1\nitems:\n  - { id: linh_thao, name: Linh Thảo, category: herb, rarity: pham }\n"
	if err := os.WriteFile(filepath.Join(dir, "items.yaml"), []byte(items), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadAll(dir)
	if err == nil || !strings.Contains(err.Error(), "unknown item") {
		t.Fatalf("expected cross-ref error, got %v", err)
	}
}
