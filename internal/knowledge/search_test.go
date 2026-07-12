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

func totalLen(chunks []string) int {
	n := 0
	for _, c := range chunks {
		n += len(c)
	}
	return n
}
