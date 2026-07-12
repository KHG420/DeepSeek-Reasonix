package knowledge

import (
	"fmt"
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

// --------------- G6: SearchDocuments ---------------

func TestSearchDocuments_NoDocs(t *testing.T) {
	s := tempStore(t)
	docs, err := s.SearchDocuments("anything", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(docs))
	}
}

func TestSearchDocuments_SingleDoc(t *testing.T) {
	s := tempStore(t)
	populateDoc(t, s, "single-doc", []string{
		"The quick brown fox jumps over the lazy dog near the river bank.",
		"A completely different topic about machine learning and neural networks.",
		"Another paragraph about the fox and its habitat in the forest.",
	})

	docs, err := s.SearchDocuments("fox river", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	if docs[0].DocSlug != "single-doc" {
		t.Errorf("expected slug single-doc, got %s", docs[0].DocSlug)
	}
	if docs[0].Score <= 0 {
		t.Errorf("expected positive MaxP score, got %.4f", docs[0].Score)
	}
	if len(docs[0].TopChunks) == 0 || len(docs[0].TopChunks) > 3 {
		t.Errorf("expected 1-3 top chunks, got %d", len(docs[0].TopChunks))
	}
	// TopChunks should be sorted by score descending.
	for i := 1; i < len(docs[0].TopChunks); i++ {
		if docs[0].TopChunks[i].Score > docs[0].TopChunks[i-1].Score {
			t.Errorf("TopChunks not sorted descending: idx %d > idx %d", i, i-1)
		}
	}
}

func TestSearchDocuments_MultiDoc(t *testing.T) {
	s := tempStore(t)
	populateDoc(t, s, "doc-alpha", []string{
		"alpha specific content about quantum physics and entanglement",
	})
	populateDoc(t, s, "doc-beta", []string{
		"beta content about classical mechanics and newtonian physics",
		"beta second paragraph about physics laws",
	})
	// Third document unrelated.
	populateDoc(t, s, "doc-gamma", []string{
		"cooking recipes and kitchen tips",
	})

	docs, err := s.SearchDocuments("physics quantum", 10)
	if err != nil {
		t.Fatal(err)
	}
	// Should find doc-alpha (direct match on quantum) and doc-beta (physics match).
	if len(docs) < 2 {
		t.Fatalf("expected at least 2 docs, got %d", len(docs))
	}
	// doc-alpha should rank first (higher score).
	if docs[0].DocSlug != "doc-alpha" {
		t.Errorf("expected doc-alpha first, got %s", docs[0].DocSlug)
	}
	// doc-gamma should not appear.
	for _, d := range docs {
		if d.DocSlug == "doc-gamma" {
			t.Errorf("unexpected doc-gamma in results")
		}
	}
}

func TestSearchDocuments_Limit(t *testing.T) {
	s := tempStore(t)
	for i := 0; i < 5; i++ {
		slug := fmt.Sprintf("doc-%d", i)
		populateDoc(t, s, slug, []string{
			strings.Repeat("common topic word ", 15),
		})
	}

	docs, err := s.SearchDocuments("common topic", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) > 3 {
		t.Errorf("expected at most 3 docs, got %d", len(docs))
	}
}

func TestSearchDocuments_EmptyQuery(t *testing.T) {
	s := tempStore(t)
	populateDoc(t, s, "empty-query", []string{"some content here"})

	_, err := s.SearchDocuments("   ", 10)
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestSearchDocuments_WithFilter(t *testing.T) {
	s := tempStore(t)
	populateDoc(t, s, "doc-foo", []string{"machine learning content"})
	populateDoc(t, s, "doc-bar", []string{"deep learning content"})

	docs, err := s.SearchDocuments("learning", 10, SearchFilter{DocSlug: "doc-foo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	if docs[0].DocSlug != "doc-foo" {
		t.Errorf("expected doc-foo, got %s", docs[0].DocSlug)
	}
}

// --------------- G5: Adaptive RRF ---------------

func TestDetectQueryType_Question(t *testing.T) {
	qtype := detectQueryType("what is a neural network")
	if qtype != "conceptual" {
		t.Errorf("expected conceptual for question, got %q", qtype)
	}
}

func TestDetectQueryType_QuestionMark(t *testing.T) {
	qtype := detectQueryType("How does RAG work?")
	if qtype != "conceptual" {
		t.Errorf("expected conceptual for '?', got %q", qtype)
	}
}

func TestDetectQueryType_VerbHeavy(t *testing.T) {
	qtype := detectQueryType("explain how to implement RAG retrieval")
	if qtype != "conceptual" {
		t.Errorf("expected conceptual for verb-heavy, got %q", qtype)
	}
}

func TestDetectQueryType_NounHeavy(t *testing.T) {
	qtype := detectQueryType("BM25 neural network Transformer embedding")
	if qtype != "factual" {
		t.Errorf("expected factual for noun-heavy, got %q", qtype)
	}
}

func TestDetectQueryType_ShortQuery(t *testing.T) {
	qtype := detectQueryType("RAG")
	if qtype != "factual" {
		t.Errorf("expected factual for short query, got %q", qtype)
	}
}

func TestDetectQueryType_Balanced(t *testing.T) {
	qtype := detectQueryType("the team has the data")
	if qtype != "balanced" {
		t.Errorf("expected balanced, got %q", qtype)
	}
}

func TestAdaptiveRRFWeight_Conceptual(t *testing.T) {
	alpha := adaptiveRRFWeight("explain how RAG works")
	if alpha != 0.4 {
		t.Errorf("expected 0.4 for conceptual, got %.1f", alpha)
	}
}

func TestAdaptiveRRFWeight_Factual(t *testing.T) {
	alpha := adaptiveRRFWeight("neural network Transformer embedding")
	if alpha != 0.6 {
		t.Errorf("expected 0.6 for factual, got %.1f", alpha)
	}
}

func TestAdaptiveRRFWeight_Balanced(t *testing.T) {
	alpha := adaptiveRRFWeight("the team has the data")
	if alpha != 0.5 {
		t.Errorf("expected 0.5 for balanced, got %.1f", alpha)
	}
}

// --------------- G9: Snippet dedup ---------------

func TestSnippetJaccard_Identical(t *testing.T) {
	a := "the quick brown fox"
	b := "the quick brown fox"
	j := snippetJaccard(a, b)
	if j != 1.0 {
		t.Errorf("expected 1.0 for identical snippets, got %.2f", j)
	}
}

func TestSnippetJaccard_Disjoint(t *testing.T) {
	a := "the quick brown fox"
	b := "machine learning neural networks"
	j := snippetJaccard(a, b)
	if j != 0.0 {
		t.Errorf("expected 0.0 for disjoint snippets, got %.2f", j)
	}
}

func TestSnippetJaccard_Partial(t *testing.T) {
	a := "the quick brown fox"
	b := "the quick brown dog"
	j := snippetJaccard(a, b)
	// 3 common words (the, quick, brown), total union of 5 words.
	if j < 0.5 || j > 0.7 {
		t.Errorf("expected ~0.6 for partial overlap, got %.2f", j)
	}
}

func TestSnippetJaccard_Empty(t *testing.T) {
	if j := snippetJaccard("", "something"); j != 0.0 {
		t.Errorf("expected 0.0 for empty input, got %.2f", j)
	}
	if j := snippetJaccard("something", ""); j != 0.0 {
		t.Errorf("expected 0.0 for empty input, got %.2f", j)
	}
	if j := snippetJaccard("", ""); j != 0.0 {
		t.Errorf("expected 0.0 for both empty, got %.2f", j)
	}
}

func TestDeduplicateSnippets_SameDocOverlap(t *testing.T) {
	// Two hits from the same document with nearly identical snippets.
	snippet := "the quick brown fox jumps over the lazy dog"
	hits := []SearchHit{
		{Score: 5.0, DocSlug: "doc", ChunkID: "000", Snippet: snippet},
		{Score: 3.0, DocSlug: "doc", ChunkID: "001", Snippet: snippet},
	}
	result := deduplicateSnippets(hits)
	// The lower score (index 1) should be marked as duplicate.
	if result[1].DuplicateOf != "000" {
		t.Errorf("expected DuplicateOf=000 for lower-scored hit, got %q", result[1].DuplicateOf)
	}
	// The higher score (index 0) should NOT be marked.
	if result[0].DuplicateOf != "" {
		t.Errorf("expected no DuplicateOf on higher-scored hit, got %q", result[0].DuplicateOf)
	}
}

func TestDeduplicateSnippets_DifferentDocs(t *testing.T) {
	// Hits from different documents should not be marked as duplicates.
	snippet := "the quick brown fox"
	hits := []SearchHit{
		{Score: 5.0, DocSlug: "doc-a", ChunkID: "000", Snippet: snippet},
		{Score: 4.0, DocSlug: "doc-b", ChunkID: "001", Snippet: snippet},
	}
	result := deduplicateSnippets(hits)
	for i, h := range result {
		if h.DuplicateOf != "" {
			t.Errorf("hit %d from different doc should not be duplicate, got DuplicateOf=%q", i, h.DuplicateOf)
		}
	}
}

func TestDeduplicateSnippets_AlreadyMarked(t *testing.T) {
	// Already-marked hits should be skipped.
	hits := []SearchHit{
		{Score: 5.0, DocSlug: "doc", ChunkID: "000", Snippet: "fox jumps over"},
		{Score: 4.0, DocSlug: "doc", ChunkID: "001", Snippet: "fox jumps over", DuplicateOf: "000"},
	}
	result := deduplicateSnippets(hits)
	// Should not re-process the already-marked hit.
	if result[1].DuplicateOf != "000" {
		t.Errorf("expected DuplicateOf=000 to be preserved, got %q", result[1].DuplicateOf)
	}
}

func TestDeduplicateSnippets_EdgeCase(t *testing.T) {
	// Empty hits slice should not panic.
	result := deduplicateSnippets(nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestDeduplicateSnippets_DifferentSnippets(t *testing.T) {
	// Different snippets from the same doc should NOT be marked.
	hits := []SearchHit{
		{Score: 5.0, DocSlug: "doc", ChunkID: "000", Snippet: "the quick brown fox"},
		{Score: 4.0, DocSlug: "doc", ChunkID: "001", Snippet: "machine learning neural networks"},
	}
	result := deduplicateSnippets(hits)
	for i, h := range result {
		if h.DuplicateOf != "" {
			t.Errorf("hit %d with different snippet should not be duplicate, got DuplicateOf=%q", i, h.DuplicateOf)
		}
	}
}

// --------------- G14: Coarse-to-fine search ---------------

func TestCoarseToFineSearch_NoSections(t *testing.T) {
	// Documents without section info should return all entries unchanged.
	s := tempStore(t)
	populateDoc(t, s, "no-sec", []string{
		"the quick brown fox jumps over the lazy dog",
		"machine learning and neural networks",
	})

	hits, err := s.Search("fox", 10, SearchFilter{Coarse: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit for coarse search without sections")
	}
	if hits[0].DocSlug != "no-sec" {
		t.Errorf("expected hit from 'no-sec', got %q", hits[0].DocSlug)
	}
}

func TestCoarseToFineSearch_WithSections(t *testing.T) {
	s := tempStore(t)
	populateDocWithIndex(t, s, "sec-doc", []string{
		"Introduction to artificial intelligence machine learning.",
		"Methods for machine learning with deep neural networks.",
		"Results of machine learning experiments show improvement.",
		"Conclusion: machine learning advances rapidly.",
	}, []string{
		"## Introduction",
		"## Methods",
		"## Results",
		"## Conclusion",
	}, []int{0, 100, 250, 400})

	// Create section chunks so coarse-to-fine can read them.
	secChunks := []ChunkWithMeta{
		{Content: "Introduction to artificial intelligence machine learning.", Section: "## Introduction", Offset: 0, SectionID: "## Introduction"},
		{Content: "Methods for machine learning with deep neural networks.", Section: "## Methods", Offset: 100, SectionID: "## Methods"},
		{Content: "Results of machine learning experiments show improvement.", Section: "## Results", Offset: 250, SectionID: "## Results"},
		{Content: "Conclusion: machine learning advances rapidly.", Section: "## Conclusion", Offset: 400, SectionID: "## Conclusion"},
	}
	if err := s.WriteSectionChunks("sec-doc", secChunks); err != nil {
		t.Fatal(err)
	}

	// Update CHUNKS.toml to include SectionChunkID references.
	index, idxErr := s.ReadChunksIndex("sec-doc")
	if idxErr != nil || index == nil {
		t.Fatal("expected chunks index")
	}
	for i := range index.Chunks {
		secID := fmt.Sprintf("S%02d", i)
		index.Chunks[i].SectionChunkID = secID
	}
	if err := s.WriteChunksIndex("sec-doc", index); err != nil {
		t.Fatal(err)
	}

	// Search without coarse filter — should return all chunks that match.
	allHits, err := s.Search("machine learning", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(allHits) < 2 {
		t.Fatalf("expected at least 2 hits without coarse filter for 'machine learning', got %d", len(allHits))
	}

	// Search with coarse filter — should still return results.
	coarseHits, err := s.Search("machine learning", 10, SearchFilter{Coarse: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(coarseHits) == 0 {
		t.Fatal("expected at least one hit with coarse filter")
	}
	// All hits should be from the same document.
	for _, h := range coarseHits {
		if h.DocSlug != "sec-doc" {
			t.Errorf("expected all hits from 'sec-doc', got %q", h.DocSlug)
		}
	}
}

func TestCoarseToFineSearch_Hybrid(t *testing.T) {
	s := tempStore(t)
	s.SetEmbedder(NewMockEmbedder(4))

	populateDocWithIndex(t, s, "hybrid-sec", []string{
		"The quick brown fox jumps over the lazy dog near the river bank.",
		"Machine learning and neural networks for artificial intelligence.",
		"Results of experiments on reinforcement learning algorithms.",
	}, []string{
		"## Animals",
		"## AI Methods",
		"## Experiments",
	}, []int{0, 100, 250})

	// Write section chunks with SectionChunkID.
	secChunks := []ChunkWithMeta{
		{Content: "The quick brown fox jumps over the lazy dog near the river bank.", Section: "## Animals", Offset: 0, SectionID: "## Animals"},
		{Content: "Machine learning and neural networks for artificial intelligence.", Section: "## AI Methods", Offset: 100, SectionID: "## AI Methods"},
		{Content: "Results of experiments on reinforcement learning algorithms.", Section: "## Experiments", Offset: 250, SectionID: "## Experiments"},
	}
	if err := s.WriteSectionChunks("hybrid-sec", secChunks); err != nil {
		t.Fatal(err)
	}

	// Link fine chunks to sections.
	index, idxErr := s.ReadChunksIndex("hybrid-sec")
	if idxErr != nil || index == nil {
		t.Fatal("expected chunks index")
	}
	for i := range index.Chunks {
		secID := fmt.Sprintf("S%02d", i)
		index.Chunks[i].SectionChunkID = secID
	}
	if err := s.WriteChunksIndex("hybrid-sec", index); err != nil {
		t.Fatal(err)
	}

	// HybridSearch with coarse filter should work.
	hits, err := s.HybridSearch("machine learning neural", 10, SearchFilter{Coarse: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit from hybrid coarse search")
	}
}

func TestCoarseToFineSearch_EmptyQuery(t *testing.T) {
	s := tempStore(t)
	populateDoc(t, s, "empty-coarse", []string{"some content"})

	_, err := s.Search("   ", 10, SearchFilter{Coarse: true})
	if err == nil {
		t.Error("expected error for empty query with coarse filter")
	}
}
