package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Store manages the on-disk knowledge base under .reasonix/knowledge/.
type Store struct {
	root string // workspace root (contains .reasonix/)
}

// NewStore returns a Store rooted at workspaceRoot. The caller must ensure
// workspaceRoot/.reasonix/ exists (it always does in a Reasonix project).
func NewStore(workspaceRoot string) *Store {
	return &Store{root: workspaceRoot}
}

// knowledgeDir returns the path to .reasonix/knowledge/.
func (s *Store) knowledgeDir() string {
	return filepath.Join(s.root, ".reasonix", "knowledge")
}

// EnsureDir creates the knowledge directory tree if it doesn't exist.
func (s *Store) EnsureDir() error {
	return os.MkdirAll(s.knowledgeDir(), 0o755)
}

// IndexPath returns the path to INDEX.md.
func (s *Store) IndexPath() string {
	return filepath.Join(s.knowledgeDir(), "INDEX.md")
}

// ReadIndex returns the raw content of INDEX.md. It returns an empty string
// if the file doesn't exist.
func (s *Store) ReadIndex() (string, error) {
	data, err := os.ReadFile(s.IndexPath())
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read INDEX.md: %w", err)
	}
	return string(data), nil
}

// WriteIndex overwrites INDEX.md with the given content.
func (s *Store) WriteIndex(content string) error {
	if err := os.MkdirAll(s.knowledgeDir(), 0o755); err != nil {
		return fmt.Errorf("ensure knowledge dir: %w", err)
	}
	if err := os.WriteFile(s.IndexPath(), []byte(content), 0o644); err != nil {
		return fmt.Errorf("write INDEX.md: %w", err)
	}
	return nil
}

// DocDir returns the path for a document's directory.
func (s *Store) DocDir(slug string) string {
	return filepath.Join(s.knowledgeDir(), slug)
}

// MetaPath returns the path to a document's meta.json.
func (s *Store) MetaPath(slug string) string {
	return filepath.Join(s.DocDir(slug), "meta.json")
}

// ChunksDir returns the path to a document's chunks/ directory.
func (s *Store) ChunksDir(slug string) string {
	return filepath.Join(s.DocDir(slug), "chunks")
}

// ChunkPath returns the path to a chunk file (e.g. "005" → ".../chunks/005.md").
func (s *Store) ChunkPath(slug, chunkID string) string {
	return filepath.Join(s.ChunksDir(slug), chunkID+".md")
}

// WriteMeta writes a DocumentMeta as JSON to the document's meta.json.
func (s *Store) WriteMeta(slug string, meta DocumentMeta) error {
	if err := os.MkdirAll(s.DocDir(slug), 0o755); err != nil {
		return fmt.Errorf("ensure doc dir: %w", err)
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	if err := os.WriteFile(s.MetaPath(slug), data, 0o644); err != nil {
		return fmt.Errorf("write meta.json: %w", err)
	}
	return nil
}

// ReadMeta reads and unmarshals a document's meta.json.
func (s *Store) ReadMeta(slug string) (DocumentMeta, error) {
	var meta DocumentMeta
	data, err := os.ReadFile(s.MetaPath(slug))
	if err != nil {
		if os.IsNotExist(err) {
			return meta, fmt.Errorf("document %q not found", slug)
		}
		return meta, fmt.Errorf("read meta.json for %q: %w", slug, err)
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return meta, fmt.Errorf("unmarshal meta.json for %q: %w", slug, err)
	}
	return meta, nil
}

// WriteChunks creates the chunks/ directory and writes each chunk as NNN.md.
func (s *Store) WriteChunks(slug string, chunks []string) error {
	dir := s.ChunksDir(slug)
	// Start fresh: remove existing chunks dir if present.
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove old chunks: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create chunks dir: %w", err)
	}
	for i, content := range chunks {
		chunkID := fmt.Sprintf("%03d", i)
		path := s.ChunkPath(slug, chunkID)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write chunk %s: %w", chunkID, err)
		}
	}
	return nil
}

// ReadChunk reads a single chunk file and returns its content.
func (s *Store) ReadChunk(slug, chunkID string) (string, error) {
	data, err := os.ReadFile(s.ChunkPath(slug, chunkID))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("chunk %q not found in document %q", chunkID, slug)
		}
		return "", fmt.Errorf("read chunk %q in %q: %w", chunkID, slug, err)
	}
	return string(data), nil
}

// ListChunks returns all chunk IDs for a document, sorted by name.
func (s *Store) ListChunks(slug string) ([]string, error) {
	dir := s.ChunksDir(slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("document %q not found", slug)
		}
		return nil, fmt.Errorf("read chunks dir for %q: %w", slug, err)
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".md") {
			ids = append(ids, strings.TrimSuffix(name, ".md"))
		}
	}
	return ids, nil
}

// SlugFromPath derives a filesystem-safe document slug from a file path.
func SlugFromPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	// Replace problematic characters with hyphens.
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
	// Collapse consecutive hyphens and trim.
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	name = strings.Trim(name, "-")
	if name == "" {
		name = "document"
	}
	// Append timestamp suffix for uniqueness.
	suffix := time.Now().Format("20060102-150405")
	return name + "-" + suffix
}

// ListDocuments returns metadata for all documents in the knowledge base.
func (s *Store) ListDocuments() ([]DocumentMeta, error) {
	kd := s.knowledgeDir()
	entries, err := os.ReadDir(kd)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read knowledge dir: %w", err)
	}
	var docs []DocumentMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		meta, err := s.ReadMeta(e.Name())
		if err != nil {
			continue // skip invalid entries
		}
		docs = append(docs, meta)
	}
	return docs, nil
}

// Exists checks whether a document slug exists.
func (s *Store) Exists(slug string) bool {
	_, err := os.Stat(s.DocDir(slug))
	return err == nil
}
