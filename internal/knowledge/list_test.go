package knowledge

import (
	"testing"
	"time"
)

func TestList(t *testing.T) {
	s := tempStore(t)

	// Empty initially.
	docs, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(docs))
	}

	// Add a doc.
	s.WriteMeta("doc-a", DocumentMeta{
		OriginalName: "a.pdf",
		SourceType:   "pdf",
		AddedAt:      time.Now(),
		ChunkCount:   3,
		TotalChars:   100,
	})
	s.WriteChunks("doc-a", []string{"a", "b", "c"})

	docs, err = s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	if docs[0].OriginalName != "a.pdf" {
		t.Errorf("unexpected doc: %+v", docs[0])
	}
}

func TestReadChunk_ListResult(t *testing.T) {
	s := tempStore(t)
	s.WriteMeta("doc", DocumentMeta{
		OriginalName: "f.txt",
		SourceType:   "txt",
		AddedAt:      time.Now(),
		ChunkCount:   2,
		TotalChars:   20,
	})
	s.WriteChunks("doc", []string{"chunk zero", "chunk one"})

	// List then read.
	docs, err := s.List()
	if err != nil || len(docs) != 1 {
		t.Fatal("list failed")
	}

	got, err := s.ReadChunk("doc", "000")
	if err != nil {
		t.Fatal(err)
	}
	if got != "chunk zero" {
		t.Errorf("got %q, want 'chunk zero'", got)
	}
}
