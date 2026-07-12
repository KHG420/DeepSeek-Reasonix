package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadDocument_Markdown(t *testing.T) {
	s := tempStore(t)

	// Create a markdown file with multiple paragraphs.
	dir := t.TempDir()
	src := filepath.Join(dir, "test.md")
	// Each paragraph needs to be > 200 chars to avoid merging.
	longPara := strings.Repeat("This is a long paragraph with enough characters to stay independent. ", 6)
	content := "# Title\n\n" + longPara + "\n\n" + longPara
	if err := os.WriteFile(src, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	meta, err := s.UploadDocument(src)
	if err != nil {
		t.Fatal(err)
	}
	if meta.OriginalName != "test.md" {
		t.Errorf("OriginalName: got %q, want %q", meta.OriginalName, "test.md")
	}
	if meta.SourceType != "md" {
		t.Errorf("SourceType: got %q, want %q", meta.SourceType, "md")
	}
	if meta.ChunkCount < 1 {
		t.Errorf("ChunkCount: got %d, want >= 1", meta.ChunkCount)
	}

	// Verify meta.json was written.
	gotMeta, err := s.ReadMeta(meta.Slug())
	if err != nil {
		t.Fatal(err)
	}
	if gotMeta.ChunkCount != meta.ChunkCount {
		t.Errorf("meta mismatch: %d vs %d", gotMeta.ChunkCount, meta.ChunkCount)
	}

	// Verify chunks exist.
	ids, err := s.ListChunks(meta.Slug())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != meta.ChunkCount {
		t.Errorf("chunks count: got %d, want %d", len(ids), meta.ChunkCount)
	}

	// Verify INDEX.md updated.
	idx, err := s.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(idx, "test.md") {
		t.Errorf("INDEX.md should mention test.md: %s", idx)
	}

	// Verify source file was copied.
	sourcePath := filepath.Join(s.DocDir(meta.Slug()), "source.md")
	if _, err := os.Stat(sourcePath); err != nil {
		t.Errorf("source file not copied: %v", err)
	}
}

func TestUploadDocument_Txt(t *testing.T) {
	s := tempStore(t)

	dir := t.TempDir()
	src := filepath.Join(dir, "notes.txt")
	longPara := strings.Repeat("This is a long paragraph with enough characters to stay independent. ", 6)
	content := longPara + "\n\n" + longPara
	if err := os.WriteFile(src, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	meta, err := s.UploadDocument(src)
	if err != nil {
		t.Fatal(err)
	}
	if meta.SourceType != "txt" {
		t.Errorf("SourceType: got %q, want txt", meta.SourceType)
	}
}

func TestUploadDocument_Empty(t *testing.T) {
	s := tempStore(t)

	dir := t.TempDir()
	src := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(src, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := s.UploadDocument(src)
	if err == nil {
		t.Error("expected error for empty document")
	}
}

func TestUploadDocument_NotFound(t *testing.T) {
	s := tempStore(t)
	_, err := s.UploadDocument("/nonexistent/file.pdf")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// Slug returns the document slug derived from the metadata's original name and
// added-at timestamp. This mirrors SlugFromPath but works from a meta struct.
func (m DocumentMeta) Slug() string {
	// The slug is embedded in the directory path. Since we don't store it in
	// meta.json, we look it up from the store by scanning.
	// For test convenience, reconstruct it from the data we have.
	name := strings.TrimSuffix(m.OriginalName, filepath.Ext(m.OriginalName))
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	name = strings.Trim(name, "-")
	if name == "" {
		name = "document"
	}
	suffix := m.AddedAt.Format("20060102-150405")
	return name + "-" + suffix
}
