package memory

import (
	"os"
	"path/filepath"
	"testing"

	"reasonix/internal/config"
)

func TestProjectMemoryStoresDoNotShareCollidingWorkspaceSlugs(t *testing.T) {
	base := t.TempDir()
	userDir := filepath.Join(base, "state")
	first := filepath.Join(base, "front-end", "app")
	second := filepath.Join(base, "front", "end-app")
	legacy := filepath.Join(userDir, "projects", config.LegacyWorkspaceSlug(first), "memory")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(legacy, "old.md")
	if err := os.WriteFile(sentinel, []byte("old memory"), 0o600); err != nil {
		t.Fatal(err)
	}

	a := StoreFor(userDir, first)
	b := StoreFor(userDir, second)
	if a.Dir == b.Dir {
		t.Fatalf("different projects share memory directory %q", a.Dir)
	}
	if a.Dir != legacy {
		t.Fatalf("legacy project's memory moved from %q to %q", legacy, a.Dir)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "old memory" {
		t.Fatalf("legacy memory was not preserved: %q, %v", data, err)
	}
}
