package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ProjectStateDir keeps an existing project on its historical state directory
// when that directory can be claimed by this workspace. A different workspace
// with the same lossy legacy slug uses the escaped slug, so new data cannot be
// written into the other project's state. The legacy directory is never moved
// or deleted; mixed pre-upgrade records remain on disk under its first owner.
func ProjectStateDir(userDir, workspaceRoot string) string {
	if userDir == "" || strings.TrimSpace(workspaceRoot) == "" {
		return ""
	}
	root := strings.TrimSpace(workspaceRoot)
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	if runtimeGOOS == "windows" {
		root = strings.ToLower(root)
	}
	modern := filepath.Join(userDir, "projects", WorkspaceSlug(root))
	legacy := filepath.Join(userDir, "projects", LegacyWorkspaceSlug(root))
	if modern == legacy {
		return modern
	}
	if info, err := os.Stat(legacy); err != nil || !info.IsDir() {
		return modern
	}
	ownerPath := filepath.Join(legacy, ".workspace-root")
	owner, err := os.ReadFile(ownerPath)
	if err == nil {
		if string(owner) == root+"\n" {
			return legacy
		}
		return modern
	}
	if !os.IsNotExist(err) {
		return modern
	}
	if info, err := os.Stat(modern); err == nil && info.IsDir() {
		return modern
	}
	// Publish a complete owner file without replacing a concurrent claimant.
	f, err := os.CreateTemp(legacy, ".workspace-root-*")
	if err != nil {
		return modern
	}
	defer func() { _ = os.Remove(f.Name()) }()
	_, writeErr := f.WriteString(root + "\n")
	if writeErr == nil {
		writeErr = f.Sync()
	}
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		return modern
	}
	if err := os.Link(f.Name(), ownerPath); err == nil {
		return legacy
	}
	if owner, err = os.ReadFile(ownerPath); err == nil && string(owner) == root+"\n" {
		return legacy
	}
	return modern
}
