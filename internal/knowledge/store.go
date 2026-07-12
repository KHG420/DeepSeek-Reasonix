package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"reasonix/internal/retrieval"
)

// Store manages the on-disk knowledge base under .reasonix/knowledge/.
type Store struct {
	root     string   // workspace root (contains .reasonix/)
	embedder Embedder // optional embedding provider for hybrid search
}

// NewStore returns a Store rooted at workspaceRoot. The caller must ensure
// workspaceRoot/.reasonix/ exists (it always does in a Reasonix project).
func NewStore(workspaceRoot string) *Store {
	return &Store{root: workspaceRoot}
}

// SetEmbedder configures an optional embedding provider for hybrid search.
// When set, Search will attempt embedding-based retrieval followed by BM25
// reranking. Setting to nil reverts to BM25-only search.
func (s *Store) SetEmbedder(e Embedder) {
	s.embedder = e
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

// ChunksIndexPath returns the path to a document's CHUNKS.toml.
func (s *Store) ChunksIndexPath(slug string) string {
	return filepath.Join(s.DocDir(slug), "CHUNKS.toml")
}

// WriteChunksIndex persists a ChunksIndex as TOML. It ensures the document
// directory exists before writing.
func (s *Store) WriteChunksIndex(slug string, index *ChunksIndex) error {
	if err := os.MkdirAll(s.DocDir(slug), 0o755); err != nil {
		return fmt.Errorf("ensure doc dir: %w", err)
	}
	f, err := os.Create(s.ChunksIndexPath(slug))
	if err != nil {
		return fmt.Errorf("create CHUNKS.toml: %w", err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(index); err != nil {
		return fmt.Errorf("encode CHUNKS.toml: %w", err)
	}
	return nil
}

// ReadChunksIndex reads and decodes a document's CHUNKS.toml. It returns nil
// and no error when the file does not exist, so callers can fall back to a
// full scan of chunk files.
func (s *Store) ReadChunksIndex(slug string) (*ChunksIndex, error) {
	f, err := os.Open(s.ChunksIndexPath(slug))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open CHUNKS.toml: %w", err)
	}
	defer f.Close()
	var index ChunksIndex
	if _, err := toml.NewDecoder(f).Decode(&index); err != nil {
		return nil, fmt.Errorf("decode CHUNKS.toml: %w", err)
	}
	return &index, nil
}

// writeChunksIndexFromMeta builds and persists a ChunksIndex from chunk
// metadata, including pre-computed term frequencies and position info.
func (s *Store) writeChunksIndexFromMeta(slug string, chunks []ChunkWithMeta) error {
	index := &ChunksIndex{
		Slug:       slug,
		ChunkCount: len(chunks),
		Chunks:     make([]ChunkIndexEntry, len(chunks)),
	}
	for i, c := range chunks {
		id := fmt.Sprintf("%03d", i)
		tokens := retrieval.Tokens(c.Content)
		tc := retrieval.Counts(tokens)
		index.Chunks[i] = ChunkIndexEntry{
			ID:        id,
			TermCount: len(tokens),
			Terms:     tc,
			Section:   c.Section,
			Offset:    c.Offset,
		}
	}
	if err := s.WriteChunksIndex(slug, index); err != nil {
		return fmt.Errorf("write CHUNKS.toml: %w", err)
	}
	return nil
}

// RebuildIndex rebuilds a document's CHUNKS.toml from its chunk files.
// It reads every chunk .md file, tokenises the content, and writes a fresh
// index. Section and Offset metadata are lost (set to zero values) since the
// original position information is not recoverable from chunk files alone.
func (s *Store) RebuildIndex(slug string) error {
	ids, err := s.ListChunks(slug)
	if err != nil {
		return fmt.Errorf("rebuild index for %q: %w", slug, err)
	}
	if len(ids) == 0 {
		return fmt.Errorf("rebuild index for %q: no chunk files found", slug)
	}

	chunkMetas := make([]ChunkWithMeta, len(ids))
	for i, id := range ids {
		text, readErr := s.ReadChunk(slug, id)
		if readErr != nil {
			return fmt.Errorf("rebuild index for %q: read chunk %s: %w", slug, id, readErr)
		}
		chunkMetas[i] = ChunkWithMeta{
			Content: text,
			Section: "",  // lost — not recoverable
			Offset:  0,   // lost — not recoverable
		}
	}

	if err := s.writeChunksIndexFromMeta(slug, chunkMetas); err != nil {
		return fmt.Errorf("rebuild index for %q: %w", slug, err)
	}
	return nil
}

// Diagnose returns a diagnostic report for a document, checking the health of
// meta.json, CHUNKS.toml, and chunk files. Returns a human-readable summary.
func (s *Store) Diagnose(slug string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Document: %s\n", slug)

	// Check meta.json.
	metaPath := s.MetaPath(slug)
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		b.WriteString("  meta.json: MISSING\n")
	} else if err != nil {
		fmt.Fprintf(&b, "  meta.json: error: %v\n", err)
	} else {
		meta, err := s.ReadMeta(slug)
		if err != nil {
			fmt.Fprintf(&b, "  meta.json: corrupt: %v\n", err)
		} else {
			fmt.Fprintf(&b, "  meta.json: OK (%d chunks, %d chars)\n", meta.ChunkCount, meta.TotalChars)
		}
	}

	// Check CHUNKS.toml.
	idxPath := s.ChunksIndexPath(slug)
	if _, err := os.Stat(idxPath); os.IsNotExist(err) {
		b.WriteString("  CHUNKS.toml: MISSING (will fall back to full scan)\n")
	} else if err != nil {
		fmt.Fprintf(&b, "  CHUNKS.toml: stat error: %v\n", err)
	} else {
		index, err := s.ReadChunksIndex(slug)
		if err != nil {
			fmt.Fprintf(&b, "  CHUNKS.toml: CORRUPT: %v (run rebuild_index to fix)\n", err)
		} else {
			fmt.Fprintf(&b, "  CHUNKS.toml: OK (%d entries)\n", index.ChunkCount)
		}
	}

	// Check chunk files.
	ids, err := s.ListChunks(slug)
	if err != nil {
		// Check if chunks dir actually exists.
		if _, statErr := os.Stat(s.ChunksDir(slug)); os.IsNotExist(statErr) {
			b.WriteString("  chunks/: MISSING\n")
		} else {
			fmt.Fprintf(&b, "  chunks/: error: %v\n", err)
		}
	} else {
		b.WriteString(fmt.Sprintf("  chunks/: %d file(s)\n", len(ids)))
		// Verify each chunk is readable.
		var missing int
		for _, id := range ids {
			if _, err := s.ReadChunk(slug, id); err != nil {
				missing++
			}
		}
		if missing > 0 {
			fmt.Fprintf(&b, "  chunks/: %d file(s) unreadable\n", missing)
		}
	}

	return b.String(), nil
}

// ReadChunkContext reads a chunk identified by docSlug and chunkID, optionally
// including up to context adjacent chunks before and after. When context is 0
// it behaves like ReadChunk.
//
// If the document has a CHUNKS.toml with section metadata, adjacent chunks
// under the same section are merged into continuous text with section headers
// (## Section). Otherwise the result is formatted with chunk ID markers as a
// fallback.
func (s *Store) ReadChunkContext(slug, chunkID string, context int) (string, error) {
	if context <= 0 {
		return s.ReadChunk(slug, chunkID)
	}

	// Parse chunk ID to integer.
	id := chunkIDToInt(chunkID)

	// Collect all chunk IDs.
	allIDs, err := s.ListChunks(slug)
	if err != nil {
		return "", err
	}

	// Determine the window.
	start := id - context
	if start < 0 {
		start = 0
	}
	end := id + context + 1 // +1 to include the target
	maxID := len(allIDs)
	if end > maxID {
		end = maxID
	}

	// Try to load section metadata from CHUNKS.toml for richer output.
	sectionByID := map[string]string{}
	hasSections := false
	if index, err := s.ReadChunksIndex(slug); err == nil && index != nil {
		for _, entry := range index.Chunks {
			sectionByID[entry.ID] = entry.Section
			if entry.Section != "" {
				hasSections = true
			}
		}
	}

	var b strings.Builder
	if hasSections {
		// Rich output: merge adjacent chunks under the same section header.
		var lastSection string
		for i := start; i < end; i++ {
			cid := fmt.Sprintf("%03d", i)
			text, err := s.ReadChunk(slug, cid)
			if err != nil {
				continue
			}
			section := sectionByID[cid]
			if section != lastSection {
				if b.Len() > 0 {
					b.WriteString("\n\n")
				}
				if section != "" {
					b.WriteString("## " + section + "\n")
				}
				lastSection = section
			} else if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString(text)
		}
	} else {
		// Fallback: chunk ID markers for documents without section metadata.
		for i := start; i < end; i++ {
			cid := fmt.Sprintf("%03d", i)
			text, err := s.ReadChunk(slug, cid)
			if err != nil {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n\n---\n\n")
			}
			b.WriteString(fmt.Sprintf("[%s]\n%s", cid, text))
		}
	}

	if b.Len() == 0 {
		return "", fmt.Errorf("chunk %q not found in document %q", chunkID, slug)
	}
	return b.String(), nil
}

// AppendChunks writes new chunk files to an existing document, numbering them
// sequentially after the last existing chunk. It does not remove any existing
// chunks.
func (s *Store) AppendChunks(slug string, chunks []string) error {
	dir := s.ChunksDir(slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create chunks dir: %w", err)
	}

	// Find the next available ID — if the chunks dir doesn't exist yet, start at 0.
	existing, listErr := s.ListChunks(slug)
	var startID int
	if listErr == nil {
		startID = len(existing)
	} else if _, statErr := os.Stat(s.ChunksDir(slug)); os.IsNotExist(statErr) {
		startID = 0 // fresh doc, no chunks yet
	} else {
		return fmt.Errorf("append chunks to %q: list existing: %w", slug, listErr)
	}

	for i, content := range chunks {
		chunkID := fmt.Sprintf("%03d", startID+i)
		path := s.ChunkPath(slug, chunkID)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write chunk %s: %w", chunkID, err)
		}
	}
	return nil
}

// AppendChunksIndex appends new chunk entries to an existing CHUNKS.toml index
// without rewriting the entire index from scratch. If no index exists yet, it
// writes a full index from the given chunks.
func (s *Store) AppendChunksIndex(slug string, newChunks []ChunkWithMeta) error {
	if len(newChunks) == 0 {
		return nil
	}

	existing, err := s.ReadChunksIndex(slug)
	if err != nil {
		// Corrupt index — fall back to full rebuild.
		return s.writeChunksIndexFromMeta(slug, newChunks)
	}

	if existing == nil {
		// No index exists — write a fresh one.
		return s.writeChunksIndexFromMeta(slug, newChunks)
	}

	// Append to existing index.
	startID := len(existing.Chunks)
	for i, c := range newChunks {
		id := fmt.Sprintf("%03d", startID+i)
		tokens := retrieval.Tokens(c.Content)
		tc := retrieval.Counts(tokens)
		existing.Chunks = append(existing.Chunks, ChunkIndexEntry{
			ID:        id,
			TermCount: len(tokens),
			Terms:     tc,
			Section:   c.Section,
			Offset:    c.Offset,
		})
	}
	existing.ChunkCount = len(existing.Chunks)

	return s.WriteChunksIndex(slug, existing)
}

// chunkIDToInt parses a zero-padded chunk ID like "005" to its integer value.
func chunkIDToInt(chunkID string) int {
	id := 0
	for _, r := range chunkID {
		if r >= '0' && r <= '9' {
			id = id*10 + int(r-'0')
		}
	}
	return id
}
