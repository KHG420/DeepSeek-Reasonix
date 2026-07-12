package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UploadDocument ingests a file into the knowledge base:
//  1. ParseFile extracts the full text.
//  2. ChunkText splits it into paragraph-level chunks.
//  3. Chunks are written as NNN.md under .reasonix/knowledge/<slug>/chunks/.
//  4. Metadata is written to meta.json.
//  5. INDEX.md is updated with a link to the new document.
//
// The original file is NOT copied into the knowledge base by this method; the
// caller is responsible for preserving source.<ext> if desired. Returns the
// generated slug and the metadata written.
func (s *Store) UploadDocument(path string) (DocumentMeta, error) {
	// Step 1: parse.
	text, err := ParseFile(path)
	if err != nil {
		return DocumentMeta{}, fmt.Errorf("upload: parse: %w", err)
	}

	// Step 2: chunk.
	chunks := ChunkText(text)
	if len(chunks) == 0 {
		return DocumentMeta{}, fmt.Errorf("upload: document produced no chunks (empty after parsing)")
	}

	// Step 3: derive slug and metadata.
	slug := SlugFromPath(path)
	sourceType := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")

	meta := DocumentMeta{
		OriginalName: filepath.Base(path),
		SourceType:   sourceType,
		AddedAt:      time.Now().Truncate(time.Second),
		ChunkCount:   len(chunks),
		TotalChars:   len(text),
	}

	// Step 4: persist chunks and metadata.
	if err := s.WriteChunks(slug, chunks); err != nil {
		return DocumentMeta{}, fmt.Errorf("upload: write chunks: %w", err)
	}
	if err := s.WriteMeta(slug, meta); err != nil {
		return DocumentMeta{}, fmt.Errorf("upload: write meta: %w", err)
	}

	// Step 5: optionally copy source file for traceability.
	if err := s.copySource(path, slug); err != nil {
		// Non-fatal: the document is already ingested.
		_ = err
	}

	// Step 6: update INDEX.md.
	if err := s.updateIndex(slug, meta); err != nil {
		// Non-fatal: re-index can be rebuilt.
		_ = err
	}

	return meta, nil
}

// copySource copies the original file into the document directory as source.<ext>.
func (s *Store) copySource(srcPath, slug string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	ext := filepath.Ext(srcPath)
	dest := filepath.Join(s.DocDir(slug), "source"+ext)
	if err := os.MkdirAll(s.DocDir(slug), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}

// updateIndex appends a line to INDEX.md for the newly uploaded document.
func (s *Store) updateIndex(slug string, meta DocumentMeta) error {
	existing, _ := s.ReadIndex()
	line := fmt.Sprintf("- [%s](%s/meta.json) — %d chunks, %s\n",
		meta.OriginalName, slug, meta.ChunkCount, meta.AddedAt.Format(time.RFC3339))
	return s.WriteIndex(existing + line)
}
