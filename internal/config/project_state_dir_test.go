package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestProjectStateDirKeepsLegacyDataAndSeparatesCollidingRoots(t *testing.T) {
	setRuntimeGOOS(t, "linux")
	base := t.TempDir()
	first := filepath.Join(base, "front-end", "app")
	second := filepath.Join(base, "front", "end-app")
	if LegacyWorkspaceSlug(first) != LegacyWorkspaceSlug(second) {
		t.Fatal("fixture roots do not collide under the legacy slug")
	}
	userDir := filepath.Join(base, "state")
	legacy := filepath.Join(userDir, "projects", LegacyWorkspaceSlug(first))
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(legacy, "sessions", "old.jsonl")
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte("old session"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := ProjectStateDir(userDir, first); got != legacy {
		t.Fatalf("existing project state = %q, want legacy path %q", got, legacy)
	}
	if err := os.MkdirAll(filepath.Join(userDir, "projects", WorkspaceSlug(first)), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := ProjectStateDir(userDir, second); got == legacy {
		t.Fatalf("colliding project reused the first project's legacy state %q", got)
	}
	if got := ProjectStateDir(userDir, first); got != legacy {
		t.Fatalf("legacy owner changed after collision: %q", got)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "old session" {
		t.Fatalf("legacy session was not preserved: %q, %v", data, err)
	}
}

func TestProjectStateDirConcurrentLegacyClaimsHaveOneOwner(t *testing.T) {
	setRuntimeGOOS(t, "linux")
	base := t.TempDir()
	first := filepath.Join(base, "front-end", "app")
	second := filepath.Join(base, "front", "end-app")
	userDir := filepath.Join(base, "state")
	legacy := filepath.Join(userDir, "projects", LegacyWorkspaceSlug(first))
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	roots := []string{first, second}
	got := make([]string, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range roots {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			got[i] = ProjectStateDir(userDir, roots[i])
		}(i)
	}
	close(start)
	wg.Wait()
	if (got[0] == legacy) == (got[1] == legacy) || got[0] == got[1] {
		t.Fatalf("concurrent roots used overlapping state: %q, %q", got[0], got[1])
	}
	for i, root := range roots {
		if again := ProjectStateDir(userDir, root); again != got[i] {
			t.Fatalf("project %q changed state directory from %q to %q", root, got[i], again)
		}
	}
}

func TestProjectStateDirConcurrentSameRootKeepsLegacyData(t *testing.T) {
	setRuntimeGOOS(t, "linux")
	base := t.TempDir()
	root := filepath.Join(base, "hyphen-project")
	userDir := filepath.Join(base, "state")
	legacy := filepath.Join(userDir, "projects", LegacyWorkspaceSlug(root))
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	const callers = 12
	got := make([]string, callers)
	var wg sync.WaitGroup
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got[i] = ProjectStateDir(userDir, root)
		}(i)
	}
	wg.Wait()
	for i, dir := range got {
		if dir != legacy {
			t.Fatalf("caller %d used %q rather than legacy state %q", i, dir, legacy)
		}
	}
}

func TestProjectStateDirUsesExistingEscapedState(t *testing.T) {
	setRuntimeGOOS(t, "linux")
	base := t.TempDir()
	root := filepath.Join(base, "hyphen-project")
	userDir := filepath.Join(base, "state")
	modern := filepath.Join(userDir, "projects", WorkspaceSlug(root))
	legacy := filepath.Join(userDir, "projects", LegacyWorkspaceSlug(root))
	for _, dir := range []string{modern, legacy} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if got := ProjectStateDir(userDir, root); got != modern {
		t.Fatalf("existing escaped state = %q, want %q", got, modern)
	}
}
