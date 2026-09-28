package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDistinctNestedProjectRootsDoNotShareSessionDirectory(t *testing.T) {
	repo := t.TempDir()
	first := filepath.Join(repo, "front-end", "app")
	second := filepath.Join(repo, "front", "end-app")
	for _, root := range []string{first, second} {
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, directory := range map[string]func(string) string{
		"legacy sessions": ProjectSessionDir,
		"session store":   ProjectSessionStoreDir,
		"topic state":     DesktopTopicStatePath,
	} {
		if got, wantDifferent := directory(first), directory(second); got == wantDifferent {
			t.Errorf("distinct project roots %q and %q share %s path %q", first, second, name, got)
		}
	}
}
