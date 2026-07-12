package knowledge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestE2E exercises the full knowledge-base lifecycle:
// upload → search → read → list → remove → verify INDEX.md cleanup.
func TestE2E(t *testing.T) {
	s := tempStore(t)

	// 1. Create a sample markdown file with searchable content.
	dir := t.TempDir()
	src := filepath.Join(dir, "e2e-test.md")
	longPara := strings.Repeat("The end-to-end test validates the complete knowledge base lifecycle. ", 6)
	content := "# E2E Test\n\n" +
		longPara + "\n\n" +
		"This paragraph mentions a unique keyword: xylophone-giraffe-42.\n\n" +
		longPara
	if err := os.WriteFile(src, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Upload the document.
	meta, err := s.UploadDocument(src)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if meta.ChunkCount < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", meta.ChunkCount)
	}
	slug := meta.Slug()

	// 3. List documents — should include ours.
	docs, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, d := range docs {
		if d.OriginalName == "e2e-test.md" {
			found = true
			break
		}
	}
	if !found {
		t.Error("uploaded document not found in list")
	}

	// 4. Search for the unique keyword.
	hits, err := s.Search("xylophone-giraffe-42", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("search found no hits for unique keyword")
	}
	if hits[0].DocSlug != slug {
		t.Errorf("search hit slug mismatch: got %q, want %q", hits[0].DocSlug, slug)
	}

	// 5. Read the chunk that contained the keyword.
	chunkID := hits[0].ChunkID
	text, err := s.ReadChunk(slug, chunkID)
	if err != nil {
		t.Fatalf("read chunk: %v", err)
	}
	if !strings.Contains(text, "xylophone-giraffe-42") {
		t.Errorf("read chunk does not contain keyword: %q", text)
	}

	// 6. Remove the document.
	if err := s.RemoveDocument(slug); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if s.Exists(slug) {
		t.Error("document should not exist after remove")
	}

	// 7. Verify INDEX.md no longer references the document.
	idx, err := s.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(idx, slug) {
		t.Errorf("INDEX.md still references removed document: %s", idx)
	}

	// 8. List should be empty (or not contain the removed doc).
	docs, err = s.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		if d.OriginalName == "e2e-test.md" {
			t.Error("removed document still appears in list")
		}
	}
}
