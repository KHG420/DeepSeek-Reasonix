package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestProjectStateDirUsesModernNamespaceForNewProjects(t *testing.T) {
	setRuntimeGOOS(t, "linux")
	base := t.TempDir()
	root := filepath.Join(base, "plain", "project")
	userDir := filepath.Join(base, "state")
	if got, want := ProjectStateDir(userDir, root), filepath.Join(userDir, "projects-v2", WorkspaceSlug(root)); got != want {
		t.Fatalf("new project state = %q, want %q", got, want)
	}
}

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
	if err := os.MkdirAll(filepath.Join(userDir, "projects-v2", WorkspaceSlug(first)), 0o700); err != nil {
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
	modern := filepath.Join(userDir, "projects-v2", WorkspaceSlug(root))
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

func TestProjectStateDirSeparatesEscapedSlugFromAnotherLegacySlug(t *testing.T) {
	setRuntimeGOOS(t, "linux")
	base := t.TempDir()
	escapedRoot := filepath.Join(base, "a%b")
	legacyRoot := filepath.Join(base, "a%25b")
	if WorkspaceSlug(escapedRoot) != LegacyWorkspaceSlug(legacyRoot) {
		t.Fatal("fixture modern and legacy slugs do not collide")
	}
	userDir := filepath.Join(base, "state")
	legacy := filepath.Join(userDir, "projects", LegacyWorkspaceSlug(legacyRoot))
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(legacy, "sessions", "old.jsonl")
	if err := os.MkdirAll(filepath.Dir(sentinel), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sentinel, []byte("legacy session"), 0o600); err != nil {
		t.Fatal(err)
	}
	modern := ProjectStateDir(userDir, escapedRoot)
	if want := filepath.Join(userDir, "projects-v2", WorkspaceSlug(escapedRoot)); modern != want {
		t.Fatalf("escaped state = %q, want %q", modern, want)
	}
	if modern == legacy {
		t.Fatal("escaped workspace reused another workspace's legacy directory")
	}
	if got := ProjectStateDir(userDir, legacyRoot); got != legacy {
		t.Fatalf("legacy state = %q, want %q", got, legacy)
	}
	if data, err := os.ReadFile(sentinel); err != nil || string(data) != "legacy session" {
		t.Fatalf("legacy session was not preserved: %q, %v", data, err)
	}
}

func TestProjectStateDirUnescapedRootRespectsLegacyOwner(t *testing.T) {
	setRuntimeGOOS(t, "linux")
	base := t.TempDir()
	escapedRoot := filepath.Join(base, "a-b")
	unescapedRoot := filepath.Join(base, "a", "b")
	if LegacyWorkspaceSlug(escapedRoot) != LegacyWorkspaceSlug(unescapedRoot) {
		t.Fatal("fixture roots do not collide under the legacy slug")
	}
	if WorkspaceSlug(unescapedRoot) != LegacyWorkspaceSlug(unescapedRoot) {
		t.Fatal("fixture root unexpectedly needs escaping")
	}
	userDir := filepath.Join(base, "state")
	legacy := filepath.Join(userDir, "projects", LegacyWorkspaceSlug(escapedRoot))
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := ProjectStateDir(userDir, escapedRoot); got != legacy {
		t.Fatalf("first claimant used %q, want legacy %q", got, legacy)
	}
	if got, want := ProjectStateDir(userDir, unescapedRoot), filepath.Join(userDir, "projects-v2", WorkspaceSlug(unescapedRoot)); got != want {
		t.Fatalf("second claimant used %q, want %q", got, want)
	}
	if got := ProjectStateDir(userDir, escapedRoot); got != legacy {
		t.Fatalf("first claimant moved to %q", got)
	}
}
