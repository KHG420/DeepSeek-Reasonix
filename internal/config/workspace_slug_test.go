package config

import "testing"

// WorkspaceSlug folds case on Windows so equivalent spellings of one
// workspace (drive-letter case, Explorer renames) map to a single slug —
// the same key form agent.CanonicalSessionPath uses for session paths.
func TestWorkspaceSlugFoldsCaseOnWindows(t *testing.T) {
	setRuntimeGOOS(t, "windows")
	upper := WorkspaceSlug(`C:\Users\Dev\Proj`)
	lower := WorkspaceSlug(`c:\users\dev\proj`)
	if upper != lower {
		t.Fatalf("WorkspaceSlug case-split on windows: %q vs %q", upper, lower)
	}
	if want := "c--users-dev-proj"; upper != want {
		t.Fatalf("WorkspaceSlug = %q, want %q", upper, want)
	}
}

// Unix paths are case-sensitive: two spellings that differ in case are two
// different directories and must keep distinct slugs.
func TestWorkspaceSlugPreservesCaseOffWindows(t *testing.T) {
	setRuntimeGOOS(t, "linux")
	if got, want := WorkspaceSlug("/Users/Dev/Proj"), "-Users-Dev-Proj"; got != want {
		t.Fatalf("WorkspaceSlug = %q, want %q", got, want)
	}
	if WorkspaceSlug("/users/dev/proj") == WorkspaceSlug("/Users/Dev/Proj") {
		t.Fatal("WorkspaceSlug folded case off windows; unix paths are case-sensitive")
	}
}

func TestWorkspaceSlugKeepsDifferentWorkspacePathsSeparate(t *testing.T) {
	setRuntimeGOOS(t, "linux")
	for _, pair := range [][2]string{
		{"/repo/front-end/app", "/repo/front/end-app"},
		{"/repo/a%b", "/repo/a%25b"},
		{"/repo/a:b", "/repo/a/b"},
		{"/repo/a\\b", "/repo/a/b"},
	} {
		if a, b := WorkspaceSlug(pair[0]), WorkspaceSlug(pair[1]); a == b {
			t.Errorf("distinct paths %q and %q share slug %q", pair[0], pair[1], a)
		}
	}
}

func TestWorkspaceSlugKeepsWindowsPathsSeparate(t *testing.T) {
	setRuntimeGOOS(t, "windows")
	first := WorkspaceSlug(`C:\repo\front-end\app`)
	second := WorkspaceSlug(`C:\repo\front\end-app`)
	if first == second {
		t.Fatalf("different Windows workspaces share slug %q", first)
	}
	if WorkspaceSlug(`c:\REPO\FRONT-END\APP`) != first {
		t.Fatal("equivalent Windows path spellings split state")
	}
}
