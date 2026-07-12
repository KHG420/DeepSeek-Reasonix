package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	// Create .reasonix/ to mimic workspace layout.
	if err := os.MkdirAll(filepath.Join(dir, ".reasonix"), 0o755); err != nil {
		t.Fatal(err)
	}
	return NewStore(dir)
}

func TestEnsureDir(t *testing.T) {
	s := tempStore(t)
	if err := s.EnsureDir(); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(s.knowledgeDir())
	if err != nil {
		t.Fatal(err)
	}
	if !fi.IsDir() {
		t.Error("knowledge dir is not a directory")
	}
}

func TestWriteReadMeta(t *testing.T) {
	s := tempStore(t)
	meta := DocumentMeta{
		OriginalName: "test.pdf",
		SourceType:   "pdf",
		AddedAt:      time.Now().Truncate(time.Second),
		ChunkCount:   3,
		TotalChars:   1500,
	}
	if err := s.WriteMeta("my-doc", meta); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadMeta("my-doc")
	if err != nil {
		t.Fatal(err)
	}
	if got.OriginalName != meta.OriginalName {
		t.Errorf("OriginalName: got %q, want %q", got.OriginalName, meta.OriginalName)
	}
	if got.ChunkCount != meta.ChunkCount {
		t.Errorf("ChunkCount: got %d, want %d", got.ChunkCount, meta.ChunkCount)
	}
	if !got.AddedAt.Equal(meta.AddedAt) {
		t.Errorf("AddedAt: got %v, want %v", got.AddedAt, meta.AddedAt)
	}
}

func TestReadMetaNotFound(t *testing.T) {
	s := tempStore(t)
	_, err := s.ReadMeta("nonexistent")
	if err == nil {
		t.Error("expected error for missing doc")
	}
}

func TestWriteReadIndex(t *testing.T) {
	s := tempStore(t)
	// Empty index when no file exists.
	content, err := s.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}
	if content != "" {
		t.Errorf("expected empty index, got %q", content)
	}
	// Write and read back.
	idx := "# Knowledge Base\n\n- [doc1](doc1/meta.json)\n"
	if err := s.WriteIndex(idx); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}
	if got != idx {
		t.Errorf("index mismatch:\ngot:  %q\nwant: %q", got, idx)
	}
}

func TestWriteReadChunks(t *testing.T) {
	s := tempStore(t)
	chunks := []string{"chunk zero", "chunk one", "chunk two"}
	if err := s.WriteChunks("doc", chunks); err != nil {
		t.Fatal(err)
	}
	// Read each chunk back.
	for i, want := range chunks {
		id := chunkID(i)
		got, err := s.ReadChunk("doc", id)
		if err != nil {
			t.Fatalf("ReadChunk(%s): %v", id, err)
		}
		if got != want {
			t.Errorf("chunk %s: got %q, want %q", id, got, want)
		}
	}
	// List chunks.
	ids, err := s.ListChunks("doc")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != len(chunks) {
		t.Errorf("ListChunks: got %d ids, want %d", len(ids), len(chunks))
	}
}

func TestWriteChunksOverwrite(t *testing.T) {
	s := tempStore(t)
	// Write initial chunks.
	if err := s.WriteChunks("doc", []string{"old"}); err != nil {
		t.Fatal(err)
	}
	// Overwrite with new chunks.
	if err := s.WriteChunks("doc", []string{"new0", "new1"}); err != nil {
		t.Fatal(err)
	}
	ids, err := s.ListChunks("doc")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 chunks after overwrite, got %d", len(ids))
	}
	got, _ := s.ReadChunk("doc", "000")
	if got != "new0" {
		t.Errorf("chunk 000 after overwrite: got %q, want %q", got, "new0")
	}
}

func TestReadChunkNotFound(t *testing.T) {
	s := tempStore(t)
	if err := s.WriteChunks("doc", []string{"only"}); err != nil {
		t.Fatal(err)
	}
	_, err := s.ReadChunk("doc", "999")
	if err == nil {
		t.Error("expected error for missing chunk")
	}
	_, err = s.ReadChunk("nonexistent", "000")
	if err == nil {
		t.Error("expected error for missing doc")
	}
}

func TestListDocuments(t *testing.T) {
	s := tempStore(t)
	// No docs initially.
	docs, err := s.ListDocuments()
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(docs))
	}
	// Add two docs.
	s.WriteMeta("doc-a", DocumentMeta{OriginalName: "a.pdf", SourceType: "pdf", AddedAt: time.Now()})
	s.WriteMeta("doc-b", DocumentMeta{OriginalName: "b.txt", SourceType: "txt", AddedAt: time.Now()})
	docs, err = s.ListDocuments()
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
}

func TestRemoveDocument(t *testing.T) {
	s := tempStore(t)
	s.WriteMeta("rm-me", DocumentMeta{OriginalName: "x.pdf", SourceType: "pdf", AddedAt: time.Now()})
	s.WriteChunks("rm-me", []string{"hello"})
	if !s.Exists("rm-me") {
		t.Fatal("expected doc to exist before remove")
	}
	if err := s.RemoveDocument("rm-me"); err != nil {
		t.Fatal(err)
	}
	if s.Exists("rm-me") {
		t.Error("expected doc to be gone after remove")
	}
}

func TestSlugFromPath(t *testing.T) {
	// SlugFromPath includes a timestamp suffix, so we check prefix.
	s := SlugFromPath("/some/path/My Document (v2).pdf")
	if !strings.HasPrefix(s, "My-Document-v2-") {
		t.Errorf("unexpected slug prefix: %q", s)
	}
	s2 := SlugFromPath("simple.txt")
	if !strings.HasPrefix(s2, "simple-") {
		t.Errorf("unexpected slug prefix: %q", s2)
	}
}

func TestExists(t *testing.T) {
	s := tempStore(t)
	if s.Exists("nope") {
		t.Error("expected false for nonexistent doc")
	}
	s.WriteMeta("yep", DocumentMeta{OriginalName: "f", SourceType: "txt", AddedAt: time.Now()})
	if !s.Exists("yep") {
		t.Error("expected true for existing doc")
	}
}

func chunkID(i int) string { return fmt.Sprintf("%03d", i) }
