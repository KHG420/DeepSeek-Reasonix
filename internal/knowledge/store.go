package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"reasonix/internal/retrieval"
)

const maxTermsPerChunk = 200 // top-N frequent terms retained in CHUNKS.toml

// Store manages the on-disk knowledge base under .reasonix/knowledge/.
type Store struct {
	root     string // workspace root (contains .reasonix/)
	rewriter QueryRewriter
	embedder Embedder
	reranker Reranker
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

// SectionsDir returns the path to a document's section chunks directory.
func (s *Store) SectionsDir(slug string) string {
	return filepath.Join(s.ChunksDir(slug), "sections")
}

// SectionChunkPath returns the path to a section-level chunk file (e.g. "S00" → ".../chunks/sections/S00.md").
func (s *Store) SectionChunkPath(slug, sectionID string) string {
	return filepath.Join(s.SectionsDir(slug), sectionID+".md")
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

// AppendChunks writes additional chunks to an existing document's chunks/
// directory, picking up IDs where the existing chunks leave off. It does NOT
// remove existing chunks.
func (s *Store) AppendChunks(slug string, chunks []string) error {
	dir := s.ChunksDir(slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create chunks dir: %w", err)
	}
	// Determine the starting ID from existing chunks.
	existing, err := s.ListChunks(slug)
	if err != nil {
		// Document doesn't exist yet; start from 0.
		existing = nil
	}
	startID := len(existing)
	for i, content := range chunks {
		chunkID := fmt.Sprintf("%03d", startID+i)
		path := s.ChunkPath(slug, chunkID)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write chunk %s: %w", chunkID, err)
		}
	}
	return nil
}

// AppendChunksIndex reads the existing CHUNKS.toml for a document, appends new
// index entries, and writes the result back. It creates a new index when none
// exists.
func (s *Store) AppendChunksIndex(slug string, newEntries []ChunkIndexEntry) error {
	index, err := s.ReadChunksIndex(slug)
	if err != nil {
		return fmt.Errorf("read existing chunks index: %w", err)
	}
	if index == nil {
		index = &ChunksIndex{
			Slug:       slug,
			ChunkCount: 0,
			Chunks:     nil,
		}
	}
	index.Chunks = append(index.Chunks, newEntries...)
	index.ChunkCount = len(index.Chunks)
	return s.WriteChunksIndex(slug, index)
}

// AppendDocumentText chunks new text and appends it to an existing document.
// It writes new chunk files, updates the search index, and updates meta.json.
// Returns the number of new chunks added.
func (s *Store) AppendDocumentText(slug string, newText string) (int, error) {
	// Verify the document exists.
	meta, err := s.ReadMeta(slug)
	if err != nil {
		return 0, fmt.Errorf("document %q not found: %w", slug, err)
	}

	// Chunk the new text.
	fineChunks, coarseChunks := ChunkTextHierarchical(newText)
	if len(fineChunks) == 0 {
		return 0, nil // nothing to append
	}

	// Step 1: write chunk files.
	chunks := make([]string, len(fineChunks))
	for i, c := range fineChunks {
		chunks[i] = c.Content
	}
	if err := s.AppendChunks(slug, chunks); err != nil {
		return 0, fmt.Errorf("append chunks: %w", err)
	}

	// Step 2: append section-level chunks if any.
	if len(coarseChunks) > 0 {
		_ = s.WriteSectionChunks(slug, coarseChunks)
	}

	// Step 3: build index entries for the new chunks.
	var newEntries []ChunkIndexEntry
	for i, c := range fineChunks {
		id := fmt.Sprintf("%03d", meta.ChunkCount+i)
		tokens := retrieval.Tokens(c.Content)
		tc := retrieval.Counts(tokens)
		entry := ChunkIndexEntry{
			ID:          id,
			TermCount:   len(tokens),
			Terms:       trimTopTerms(tc, maxTermsPerChunk),
			Section:     c.Section,
			Offset:      meta.TotalChars + c.Offset,
			SectionRole: c.SectionRole,
		}
		if c.SectionID != "" {
			entry.SectionChunkID = c.SectionID
		}
		newEntries = append(newEntries, entry)
	}
	if err := s.AppendChunksIndex(slug, newEntries); err != nil {
		return 0, fmt.Errorf("append chunks index: %w", err)
	}

	// Step 4: update meta.json.
	meta.ChunkCount += len(fineChunks)
	meta.TotalChars += len(newText)
	if err := s.WriteMeta(slug, meta); err != nil {
		return 0, fmt.Errorf("update meta: %w", err)
	}

	return len(fineChunks), nil
}

// WriteSectionChunks writes section-level chunks into chunks/sections/.
// Each section chunk is stored as S00.md, S01.md, etc.
func (s *Store) WriteSectionChunks(slug string, sections []ChunkWithMeta) error {
	dir := s.SectionsDir(slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create sections dir: %w", err)
	}
	// Remove old section chunks.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		os.Remove(filepath.Join(dir, e.Name()))
	}
	for i, sec := range sections {
		id := fmt.Sprintf("S%02d", i)
		path := s.SectionChunkPath(slug, id)
		if err := os.WriteFile(path, []byte(sec.Content), 0o644); err != nil {
			return fmt.Errorf("write section chunk %s: %w", id, err)
		}
	}
	return nil
}

// ReadSectionChunk reads a single section-level chunk and returns its content.
func (s *Store) ReadSectionChunk(slug, sectionID string) (string, error) {
	data, err := os.ReadFile(s.SectionChunkPath(slug, sectionID))
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("section chunk %q not found in document %q", sectionID, slug)
		}
		return "", fmt.Errorf("read section chunk %q in %q: %w", sectionID, slug, err)
	}
	return string(data), nil
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
// metadata, including pre-computed term frequencies, position info,
// and optionally dense vectors when an embedder is configured.
// It delegates to writeChunksIndexFromMetaWithSections without section data.
func (s *Store) writeChunksIndexFromMeta(slug string, chunks []ChunkWithMeta) error {
	return s.writeChunksIndexFromMetaWithSections(slug, chunks, nil)
}

// writeChunksIndexFromMetaWithSections builds and persists a ChunksIndex from chunk
// metadata, including pre-computed term frequencies, position info,
// and optionally dense vectors when an embedder is configured.
// When sectionChunks is provided, each entry's SectionChunkID is populated
// from the chunk's SectionID field.
func (s *Store) writeChunksIndexFromMetaWithSections(slug string, chunks []ChunkWithMeta, sectionChunks []ChunkWithMeta) error {
	index := &ChunksIndex{
		Slug:       slug,
		ChunkCount: len(chunks),
		Chunks:     make([]ChunkIndexEntry, len(chunks)),
	}

	hasEmbedder := s.embedder != nil
	if hasEmbedder {
		index.VectorDim = s.embedder.Dim()
		index.HasVectors = true
	}

	// Generate vectors in batch if embedder is available.
	var vectors [][]float32
	if hasEmbedder {
		contents := make([]string, len(chunks))
		for i, c := range chunks {
			contents[i] = c.Content
		}
		var err error
		vectors, err = s.embedder.Embed(context.Background(), contents)
		if err != nil {
			// Non-fatal: continue without vectors.
			hasEmbedder = false
			index.VectorDim = 0
			index.HasVectors = false
		}
	}

	for i, c := range chunks {
		id := fmt.Sprintf("%03d", i)
		tokens := retrieval.Tokens(c.Content)
		tc := retrieval.Counts(tokens)
		entry := ChunkIndexEntry{
			ID:          id,
			TermCount:   len(tokens),
			Terms:       trimTopTerms(tc, maxTermsPerChunk),
			Section:     c.Section,
			Offset:      c.Offset,
			SectionRole: c.SectionRole,
		}
		// Link to parent section chunk when hierarchical data is available.
		if c.SectionID != "" && sectionChunks != nil {
			entry.SectionChunkID = c.SectionID
		}
		if hasEmbedder && i < len(vectors) && vectors[i] != nil {
			vec64 := make([]float64, len(vectors[i]))
			for j, v := range vectors[i] {
				vec64[j] = float64(v)
			}
			entry.Vector = vec64
		}
		index.Chunks[i] = entry
	}
	if err := s.WriteChunksIndex(slug, index); err != nil {
		return fmt.Errorf("write CHUNKS.toml: %w", err)
	}
	return nil
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

// trimTopTerms keeps only the top n terms with the highest counts, reducing
// the size of the CHUNKS.toml index. When counts is nil or already ≤ n,
// it returns the input unchanged.
func trimTopTerms(counts map[string]int, n int) map[string]int {
	if counts == nil || len(counts) <= n {
		return counts
	}
	type kv struct {
		k string
		v int
	}
	sorted := make([]kv, 0, len(counts))
	for k, v := range counts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].v > sorted[j].v
	})
	if n > len(sorted) {
		n = len(sorted)
	}
	out := make(map[string]int, n)
	for _, p := range sorted[:n] {
		out[p.k] = p.v
	}
	return out
}
