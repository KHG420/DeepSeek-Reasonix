package knowledge

import (
	"strings"
	"testing"
	"time"
)

func TestRemoveDocument_Exists(t *testing.T) {
	s := tempStore(t)
	s.WriteMeta("rm-me", DocumentMeta{
		OriginalName: "x.pdf",
		SourceType:   "pdf",
		AddedAt:      time.Now(),
		ChunkCount:   1,
	})
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

func TestRemoveDocument_UpdatesIndex(t *testing.T) {
	s := tempStore(t)
	// Set up INDEX.md with an entry.
	s.WriteIndex("# KB\n\n- [doc.pdf](my-doc/meta.json) — 3 chunks\n")
	s.WriteMeta("my-doc", DocumentMeta{
		OriginalName: "doc.pdf",
		SourceType:   "pdf",
		AddedAt:      time.Now(),
		ChunkCount:   3,
	})

	if err := s.RemoveDocument("my-doc"); err != nil {
		t.Fatal(err)
	}

	idx, err := s.ReadIndex()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(idx, "my-doc") {
		t.Errorf("INDEX.md should not contain removed doc: %s", idx)
	}
}

func TestRemoveDocument_NonExistent(t *testing.T) {
	s := tempStore(t)
	// Removing a non-existent document should not error.
	if err := s.RemoveDocument("no-such-doc"); err != nil {
		t.Errorf("expected no error for non-existent doc, got: %v", err)
	}
}
