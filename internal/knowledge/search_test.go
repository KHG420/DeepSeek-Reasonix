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

func TestSearch_FilterByDocSlug(t *testing.T) {
	s := tempStore(t)
	populateDoc(t, s, "doc-alpha", []string{"alpha content about machine learning"})
	populateDoc(t, s, "doc-beta", []string{"beta content about deep learning"})

	// Search all docs — both should match.
	all, err := s.Search("learning", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 hits without filter, got %d", len(all))
	}

	// Filter by doc slug — only beta.
	hits, err := s.Search("learning", 10, SearchFilter{DocSlug: "doc-beta"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit for doc-beta, got %d", len(hits))
	}
	if hits[0].DocSlug != "doc-beta" {
		t.Errorf("expected doc-beta, got %q", hits[0].DocSlug)
	}
}

func TestSearch_FilterBySourceType(t *testing.T) {
	s := tempStore(t)
	populateDocWithSourceType(t, s, "doc-pdf", "pdf", []string{"machine learning in pdf"})
	populateDocWithSourceType(t, s, "doc-txt", "txt", []string{"machine learning in txt"})

	// Filter by source type.
	hits, err := s.Search("machine", 10, SearchFilter{SourceType: "pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit for pdf, got %d", len(hits))
	}
	if hits[0].DocSlug != "doc-pdf" {
		t.Errorf("expected doc-pdf, got %q", hits[0].DocSlug)
	}
}

func TestSearch_FilterBySection(t *testing.T) {
	s := tempStore(t)
	// Use populateDocWithIndex to set sections.
	populateDocWithIndex(t, s, "sec-doc", []string{
		"Introduction text about neural networks.",
		"Methods section describing optimization techniques.",
		"Results of the experiment on neural network training.",
	}, []string{
		"## Introduction",
		"## Methods",
		"## Results",
	}, []int{0, 100, 250})

	// Filter by section substring.
	hits, err := s.Search("neural", 10, SearchFilter{Section: "Results"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit for section Results, got %d", len(hits))
	}
	if hits[0].ChunkID != "002" {
		t.Errorf("expected chunk 002 (Results), got %s", hits[0].ChunkID)
	}
}

func TestSearch_FilterCombined(t *testing.T) {
	s := tempStore(t)
	populateDocWithIndex(t, s, "doc-a", []string{
		"Alpha intro about machine learning.",
		"Alpha methods for deep learning.",
	}, []string{"## Introduction", "## Methods"}, []int{0, 100})
	populateDocWithIndex(t, s, "doc-b", []string{
		"Beta intro about machine learning.",
		"Beta methods for reinforcement learning.",
	}, []string{"## Introduction", "## Methods"}, []int{0, 100})

	// Combined filter: doc slug + section.
	hits, err := s.Search("learning", 10, SearchFilter{DocSlug: "doc-b", Section: "Methods"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].DocSlug != "doc-b" || hits[0].ChunkID != "001" {
		t.Errorf("expected doc-b chunk 001, got %s/%s", hits[0].DocSlug, hits[0].ChunkID)
	}
}

func TestSearch_EmptyFilterEquivalent(t *testing.T) {
	s := tempStore(t)
	populateDoc(t, s, "filter-equiv", []string{"content about topic"})

	// Search with empty filter should equal search without filter.
	hitsNoFilter, err := s.Search("topic", 10)
	if err != nil {
		t.Fatal(err)
	}
	hitsWithFilter, err := s.Search("topic", 10, SearchFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(hitsNoFilter) != len(hitsWithFilter) {
		t.Fatalf("mismatch: no filter=%d, empty filter=%d", len(hitsNoFilter), len(hitsWithFilter))
	}
	if len(hitsNoFilter) > 0 && hitsNoFilter[0].Score != hitsWithFilter[0].Score {
		t.Errorf("score mismatch: %.4f vs %.4f", hitsNoFilter[0].Score, hitsWithFilter[0].Score)
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

// populateDocWithSourceType is like populateDoc but sets a specific source
// type in the metadata so source-type filtering tests can rely on it.
func populateDocWithSourceType(t *testing.T, s *Store, slug string, sourceType string, chunks []string) {
	t.Helper()
	if err := s.WriteChunks(slug, chunks); err != nil {
		t.Fatal(err)
	}
	meta := DocumentMeta{
		OriginalName: slug + "." + sourceType,
		SourceType:   sourceType,
		ChunkCount:   len(chunks),
		TotalChars:   totalLen(chunks),
	}
	if err := s.WriteMeta(slug, meta); err != nil {
		t.Fatal(err)
	}
}

func TestHybridSearch_NoEmbedder(t *testing.T) {
	s := tempStore(t)
	populateDoc(t, s, "hybrid-test", []string{
		"alpha beta gamma delta",
		"epsilon zeta eta theta",
	})

	// Without an embedder, HybridSearch falls back to BM25 and should still work.
	hits, err := s.HybridSearch("alpha beta", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit from hybrid fallback")
	}
}

func TestHybridSearch_WithVectors(t *testing.T) {
	s := tempStore(t)
	s.SetEmbedder(NewMockEmbedder(4))

	populateDocWithIndex(t, s, "hybrid-vec", []string{
		"The quick brown fox jumps over the lazy dog.",
		"Machine learning and artificial intelligence topics.",
	}, []string{"## Animals", "## AI"}, []int{0, 100})

	// Both Search (BM25) and HybridSearch should return results.
	bm25Hits, err := s.Search("fox dog", 10)
	if err != nil {
		t.Fatal(err)
	}
	hybridHits, err := s.HybridSearch("fox dog", 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(bm25Hits) == 0 {
		t.Fatal("BM25 search should find hits")
	}
	if len(hybridHits) == 0 {
		t.Fatal("Hybrid search should find hits")
	}
	// The top result should be the same chunk (fox content).
	if hybridHits[0].ChunkID != "000" {
		t.Errorf("expected chunk 000 to rank first, got %s", hybridHits[0].ChunkID)
	}
}

func TestHybridSearch_WithFilter(t *testing.T) {
	s := tempStore(t)
	s.SetEmbedder(NewMockEmbedder(4))

	populateDocWithIndex(t, s, "doc-x", []string{
		"Alpha content about machine learning.",
		"Alpha content about deep learning.",
	}, []string{"## Intro", "## Methods"}, []int{0, 100})
	populateDocWithIndex(t, s, "doc-y", []string{
		"Beta content about reinforcement learning.",
	}, []string{"## Intro"}, []int{0})

	// Filter by doc slug.
	hits, err := s.HybridSearch("learning", 10, SearchFilter{DocSlug: "doc-x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits from doc-x, got %d", len(hits))
	}
	for _, h := range hits {
		if h.DocSlug != "doc-x" {
			t.Errorf("expected only doc-x hits, got %s", h.DocSlug)
		}
	}
}

func TestHybridSearch_NoResults(t *testing.T) {
	s := tempStore(t)
	s.SetEmbedder(NewMockEmbedder(4))

	populateDoc(t, s, "empty-hybrid", []string{"irrelevant content here"})

	hits, err := s.HybridSearch("quantum physics", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Errorf("expected 0 hits for unrelated query, got %d", len(hits))
	}
}

func TestHybridSearch_EmptyQuery(t *testing.T) {
	s := tempStore(t)
	populateDoc(t, s, "empty-hybrid", []string{"content"})

	_, err := s.HybridSearch("   ", 10)
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestHybridSearch_WithReranker(t *testing.T) {
	s := tempStore(t)
	s.SetEmbedder(NewMockEmbedder(4))
	s.SetReranker(MockReranker{})

	populateDocWithIndex(t, s, "rerank-doc", []string{
		"The quick brown fox jumps over the lazy dog near the river bank.",
		"Machine learning and neural networks for artificial intelligence.",
	}, []string{"## Animals", "## AI"}, []int{0, 100})

	// Hybrid search with reranker should still return results.
	hits, err := s.HybridSearch("machine learning neural networks", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit with reranker")
	}
	// The ML chunk should rank higher than the animal chunk.
	if hits[0].ChunkID != "001" {
		t.Errorf("expected chunk 001 (ML) to rank first with reranker, got %s", hits[0].ChunkID)
	}
}

func TestHybridSearch_WithReranker_NoVectors(t *testing.T) {
	s := tempStore(t)
	// Set reranker but no embedder (no vectors).
	s.SetReranker(MockReranker{})

	populateDoc(t, s, "rerank-novec", []string{
		"alpha beta gamma delta epsilon",
		"zeta eta theta iota kappa",
	})

	// Should work as BM25-only + reranker.
	hits, err := s.HybridSearch("alpha beta gamma", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit with reranker and no vectors")
	}
}

func TestSetReranker(t *testing.T) {
	s := tempStore(t)
	if s.reranker != nil {
		t.Error("expected nil reranker by default")
	}
	s.SetReranker(MockReranker{})
	if s.reranker == nil {
		t.Error("expected non-nil reranker after SetReranker")
	}
}

func totalLen(chunks []string) int {
	n := 0
	for _, c := range chunks {
		n += len(c)
	}
	return n
}
