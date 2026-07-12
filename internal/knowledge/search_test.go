package knowledge

import (
	"strings"
	"testing"
)

func TestSearch_NoDocs(t *testing.T) {
	s := tempStore(t)
	hits, err := s.Search("anything", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Errorf("expected 0 hits, got %d", len(hits))
	}
}

func TestSearch_English(t *testing.T) {
	s := tempStore(t)
	// Upload a document with distinctive content.
	populateDoc(t, s, "search-test", []string{
		"The quick brown fox jumps over the lazy dog near the river bank.",
		"A completely different topic about machine learning and neural networks.",
		"Another paragraph about the fox and its habitat in the forest.",
	})

	hits, err := s.Search("fox river", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit for 'fox river'")
	}
	// The first chunk (mentions fox + river) should rank highest.
	if hits[0].ChunkID != "000" {
		t.Errorf("expected chunk 000 to rank first, got %s (score=%.2f)", hits[0].ChunkID, hits[0].Score)
	}
}

func TestSearch_Chinese(t *testing.T) {
	s := tempStore(t)
	populateDoc(t, s, "cn-doc", []string{
		"深度学习是机器学习的一个分支，使用多层神经网络来学习数据的表示。",
		"强化学习是一种通过与环境交互来学习最优策略的机器学习方法。",
		"自然语言处理是人工智能的一个重要领域，涉及机器理解和生成人类语言。",
	})

	hits, err := s.Search("机器学习 神经网络", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit for CJK query")
	}
}

func TestSearch_ScoreOrdering(t *testing.T) {
	s := tempStore(t)
	populateDoc(t, s, "order-test", []string{
		"apple banana cherry date elderberry fig grape",
		"apple banana cherry date elderberry",
		"apple banana cherry",
		"zebra yak x-ray whale",
	})

	hits, err := s.Search("apple banana cherry date", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) < 3 {
		t.Fatalf("expected at least 3 hits, got %d", len(hits))
	}
	// Scores should be descending (more matching terms = higher score).
	for i := 1; i < len(hits); i++ {
		if hits[i].Score > hits[i-1].Score {
			t.Errorf("scores not descending: hits[%d]=%.4f > hits[%d]=%.4f",
				i, hits[i].Score, i-1, hits[i-1].Score)
		}
	}
}

func TestSearch_Snippet(t *testing.T) {
	s := tempStore(t)
	populateDoc(t, s, "snippet-test", []string{
		"This paragraph contains a very specific keyword xylophone that we want to find in the snippet output.",
	})

	hits, err := s.Search("xylophone", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit")
	}
	if !strings.Contains(hits[0].Snippet, "xylophone") {
		t.Errorf("snippet should contain 'xylophone': %q", hits[0].Snippet)
	}
}

func TestSearch_Limit(t *testing.T) {
	s := tempStore(t)
	var chunks []string
	for i := 0; i < 15; i++ {
		chunks = append(chunks, strings.Repeat("common topic word ", 20))
	}
	populateDoc(t, s, "limit-test", chunks)

	hits, err := s.Search("common topic", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) > 5 {
		t.Errorf("expected at most 5 hits, got %d", len(hits))
	}
}

func TestSearch_IndexPath(t *testing.T) {
	s := tempStore(t)
	populateDocWithIndex(t, s, "index-test", []string{
		"The quick brown fox jumps over the lazy dog near the river bank.",
		"A completely different topic about machine learning and neural networks.",
		"Another paragraph about the fox and its habitat in the forest.",
	}, []string{
		"## Introduction",
		"## Methods",
		"## Results",
	}, []int{0, 100, 250})

	// Search via index path — should produce same ranking as fallback but with
	// Section and Offset populated.
	hits, err := s.Search("fox river", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit for 'fox river'")
	}
	// Chunk 000 should rank first (mentions both fox and river).
	if hits[0].ChunkID != "000" {
		t.Errorf("expected chunk 000 to rank first, got %s (score=%.2f)", hits[0].ChunkID, hits[0].Score)
	}
	// Section and Offset should be populated from the index.
	if hits[0].Section != "## Introduction" {
		t.Errorf("expected section '## Introduction', got %q", hits[0].Section)
	}
	if hits[0].Offset != 0 {
		t.Errorf("expected offset 0, got %d", hits[0].Offset)
	}
	// Snippet should still be generated from the actual chunk text.
	if !strings.Contains(hits[0].Snippet, "fox") {
		t.Errorf("snippet should contain 'fox': %q", hits[0].Snippet)
	}
}

func TestSearch_IndexPathFallback(t *testing.T) {
	// A document without CHUNKS.toml should still be searchable via the
	// fallback (full-scan) path.
	s := tempStore(t)
	populateDoc(t, s, "no-index", []string{
		"alpha beta gamma delta",
		"epsilon zeta eta theta",
	})

	hits, err := s.Search("alpha beta", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit")
	}
	if hits[0].ChunkID != "000" {
		t.Errorf("expected chunk 000 to rank first, got %s (score=%.2f)", hits[0].ChunkID, hits[0].Score)
	}
	// Without index, Section/Offset should be zero values.
	if hits[0].Section != "" {
		t.Errorf("expected empty section for fallback, got %q", hits[0].Section)
	}
	if hits[0].Offset != 0 {
		t.Errorf("expected zero offset for fallback, got %d", hits[0].Offset)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	s := tempStore(t)
	populateDoc(t, s, "empty-query", []string{"some content here"})

	_, err := s.Search("   ", 10)
	if err == nil {
		t.Error("expected error for empty query")
	}
}

// populateDoc is a helper that writes chunks and metadata for a document
// directly (bypassing UploadDocument) so search tests don't need real files.
func populateDoc(t *testing.T, s *Store, slug string, chunks []string) {
	t.Helper()
	if err := s.WriteChunks(slug, chunks); err != nil {
		t.Fatal(err)
	}
	meta := DocumentMeta{
		OriginalName: slug + ".md",
		SourceType:   "md",
		ChunkCount:   len(chunks),
		TotalChars:   totalLen(chunks),
	}
	if err := s.WriteMeta(slug, meta); err != nil {
		t.Fatal(err)
	}
}

// populateDocWithIndex is like populateDoc but also writes a CHUNKS.toml
// index so Search can exercise the index fast path. Sections and offsets are
// passed per chunk; pass nil slices to leave them empty.
func populateDocWithIndex(t *testing.T, s *Store, slug string, chunks []string, sections []string, offsets []int) {
	t.Helper()
	populateDoc(t, s, slug, chunks)

	chunkMetas := make([]ChunkWithMeta, len(chunks))
	for i, c := range chunks {
		sec := ""
		if i < len(sections) {
			sec = sections[i]
		}
		off := 0
		if i < len(offsets) {
			off = offsets[i]
		}
		chunkMetas[i] = ChunkWithMeta{Content: c, Section: sec, Offset: off}
	}
	if err := s.writeChunksIndexFromMeta(slug, chunkMetas); err != nil {
		t.Fatal(err)
	}
}

func totalLen(chunks []string) int {
	n := 0
	for _, c := range chunks {
		n += len(c)
	}
	return n
}
