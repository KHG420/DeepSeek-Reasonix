package knowledge

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestChunk_Empty(t *testing.T) {
	if got := ChunkText(""); got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
	if got := ChunkText("   \n\n  "); got != nil {
		t.Errorf("expected nil for whitespace-only input, got %v", got)
	}
}

func TestChunk_SingleParagraph(t *testing.T) {
	got := ChunkText("hello world")
	if len(got) != 1 || got[0].Content != "hello world" {
		t.Errorf("got %v, want [hello world]", got)
	}
}

func TestChunk_MultipleParagraphs(t *testing.T) {
	// Use paragraphs longer than 200 chars so they aren't merged.
	longPara := strings.Repeat("Long paragraph with enough characters to avoid short-chunk merging. ", 6)
	text := longPara + "\n\n" + longPara + "\n\n" + longPara
	got := ChunkText(text)
	if len(got) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(got))
	}
}

func TestChunk_ShortParagraphMerged(t *testing.T) {
	// Long paragraph followed by a short one → short merged into long.
	longPara := strings.Repeat("Long paragraph with enough characters to avoid short-chunk merging. ", 6)
	text := longPara + "\n\nOK"
	got := ChunkText(text)
	if len(got) != 1 {
		t.Fatalf("expected 1 chunk after merge, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0].Content, "OK") {
		t.Errorf("short paragraph not merged: %q", got[0].Content)
	}
}

func TestChunk_ShortFirstStays(t *testing.T) {
	// The very first paragraph is short — it stays (no predecessor).
	longPara := strings.Repeat("Long paragraph with enough characters to avoid short-chunk merging. ", 6)
	text := "Hi\n\n" + longPara
	got := ChunkText(text)
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(got), got)
	}
}

func TestChunk_ChineseParagraphs(t *testing.T) {
	// Build Chinese paragraphs longer than 200 chars.
	cnLong := strings.Repeat("这是一个中文测试段落，包含足够多的字符来避免短段落合并。", 10)
	text := cnLong + "\n\n" + cnLong
	got := ChunkText(text)
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(got))
	}
}

func TestChunk_LongParagraphSplit(t *testing.T) {
	// Build a paragraph with many sentences separated by periods.
	var b strings.Builder
	sentence := "This is a sentence that will be repeated many times."
	for b.Len() < 2500 {
		b.WriteString(sentence)
		b.WriteString(" ")
	}
	text := b.String()
	got := ChunkText(text)
	if len(got) < 2 {
		t.Fatalf("expected at least 2 chunks for long paragraph, got %d", len(got))
	}
	for i, c := range got {
		if len(c.Content) > 2200 {
			t.Errorf("chunk %d is still too long: %d chars", i, len(c.Content))
		}
	}
}

func TestChunk_ChineseSentenceSplit(t *testing.T) {
	// Build a long Chinese paragraph with many sentences (> 2000 runes).
	var b strings.Builder
	sentence := "这是一个测试句子，用来验证中文分句功能。"
	for utf8.RuneCountInString(b.String()) < 2500 {
		b.WriteString(sentence)
	}
	text := b.String()
	got := ChunkText(text)
	if len(got) < 2 {
		t.Fatalf("expected at least 2 chunks for long Chinese paragraph (got %d, rune count %d)",
			len(got), utf8.RuneCountInString(text))
	}
}

func TestChunk_MixedNewlines(t *testing.T) {
	longPara := strings.Repeat("Long paragraph with enough characters to avoid short-chunk merging. ", 6)
	text := longPara + "\r\n\r\n" + longPara + "\r\n\r\n" + longPara
	got := ChunkText(text)
	if len(got) != 3 {
		t.Fatalf("expected 3 chunks with CRLF, got %d", len(got))
	}
}

func TestChunk_SingleNewlinePreserved(t *testing.T) {
	longPara := "Line one.\nLine two.\nLine three.\nLine four.\nLine five.\nLine six.\n" +
		"More lines to get past the short chunk threshold of 200 characters. " +
		"Even more text padding here to ensure we exceed the minimum chunk size."
	text := longPara + "\n\n" + longPara
	got := ChunkText(text)
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0].Content, "\n") {
		t.Error("single newlines within paragraph should be preserved")
	}
}

func TestChunk_SectionDetection(t *testing.T) {
	// Chunks should carry the nearest preceding heading at their start offset.
	// Short headings (like "## Installation" alone) merge into the previous
	// paragraph, so the merged chunk's section reflects where the chunk starts.
	text := "# Introduction\n\n" +
		strings.Repeat("This is the intro paragraph with enough characters to avoid short-chunk merging. ", 6) +
		"\n\n## Installation\n\n" +
		strings.Repeat("Installation instructions with enough characters to avoid merging. ", 6)
	got := ChunkText(text)
	if len(got) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(got))
	}
	// Chunk 0: "# Introduction" heading (short, stands alone as first para).
	if got[0].Section != "# Introduction" {
		t.Errorf("chunk 0 section: got %q, want '# Introduction'", got[0].Section)
	}
	// Chunk 1: the intro paragraph merged with the short "## Installation"
	// heading that follows it. The section is where the chunk *starts*.
	if got[1].Section != "# Introduction" {
		t.Errorf("chunk 1 section: got %q, want '# Introduction'", got[1].Section)
	}
	// Subsequent chunks (after the heading boundary) get "## Installation".
	foundInstall := false
	for _, c := range got[2:] {
		if c.Section == "## Installation" {
			foundInstall = true
			break
		}
	}
	if !foundInstall {
		t.Error("expected a later chunk with section '## Installation'")
	}
}

func TestChunk_FragmentMerged(t *testing.T) {
	// A long paragraph ending with a short sentence that becomes a < 60 char fragment.
	// Build a paragraph > 2000 runes with many sentences, where one sentence is very short.
	var b strings.Builder
	longSentence := strings.Repeat("Long sentence filled with enough words to exceed the fragment threshold. ", 5)
	for utf8.RuneCountInString(b.String()) < 2200 {
		b.WriteString(longSentence)
	}
	// Append a tiny sentence that will be a fragment after splitLong.
	b.WriteString("Short.")
	text := b.String()
	got := ChunkText(text)
	// The last chunk should contain "Short." merged into it (not a separate fragment).
	last := got[len(got)-1]
	if !strings.Contains(last.Content, "Short") {
		t.Errorf("fragment 'Short.' should be merged into the last chunk: %q", last.Content)
	}
}

func TestChunk_FragmentFirstChunkStays(t *testing.T) {
	// A short first chunk like "Hi." should stay even if it's < fragmentThreshold
	// because there's no preceding chunk to merge it into.
	text := "Hi.\n\n" + strings.Repeat("Long paragraph with enough characters to avoid short-chunk merging. ", 6)
	got := ChunkText(text)
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(got), got)
	}
	if got[0].Content != "Hi." {
		t.Errorf("first chunk should be 'Hi.', got %q", got[0].Content)
	}
}

func TestChunk_OffsetTracking(t *testing.T) {
	// The first chunk should start near offset 0; subsequent chunks at higher offsets.
	longPara := strings.Repeat("Long paragraph with enough characters to avoid short-chunk merging. ", 6)
	text := "# Title\n\n" + longPara + "\n\n" + longPara
	got := ChunkText(text)
	if len(got) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(got))
	}
	if got[0].Offset < 0 {
		t.Errorf("chunk 0 offset should be >= 0, got %d", got[0].Offset)
	}
	if got[1].Offset <= got[0].Offset {
		t.Errorf("chunk 1 offset (%d) should be > chunk 0 offset (%d)", got[1].Offset, got[0].Offset)
	}
}

func TestChunkTextContent(t *testing.T) {
	// ChunkTextContent should return the content strings. With overlap,
	// chunks after the first include tail context from the previous chunk.
	longPara := strings.Repeat("Long paragraph with enough characters to avoid short-chunk merging. ", 6)
	text := longPara + "\n\n" + longPara
	got := ChunkTextContent(text)
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(got))
	}
	// Chunk 0 is unchanged.
	if !strings.Contains(got[0], "Long paragraph") {
		t.Errorf("chunk 0 should contain the paragraph text")
	}
	// Chunk 1 should contain the paragraph text (its own content + overlap).
	if !strings.Contains(got[1], "Long paragraph") {
		t.Errorf("chunk 1 should contain the paragraph text")
	}
	// Chunk 1 should be longer than chunk 0 (due to overlap).
	if len(got[1]) <= len(got[0]) {
		t.Errorf("chunk 1 (%d) should be longer than chunk 0 (%d) due to overlap", len(got[1]), len(got[0]))
	}
}

func TestChunk_OverlapBetweenChunks(t *testing.T) {
	// Three paragraphs, each > 200 chars. Chunks 1 and 2 should include overlap
	// from the end of their respective previous chunks.
	longPara := strings.Repeat("Long paragraph with enough characters to avoid short-chunk merging. ", 6)
	text := longPara + "\n\n" + longPara + "\n\n" + longPara
	got := ChunkText(text)
	if len(got) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(got))
	}
	// Chunk 1 should contain overlap from the tail of chunk 0.
	if !strings.Contains(got[1].Content, "Long paragraph") {
		t.Errorf("chunk 1 should contain overlap from chunk 0: %q", got[1].Content)
	}
	// Chunk 2 should contain overlap from the tail of chunk 1.
	if !strings.Contains(got[2].Content, "Long paragraph") {
		t.Errorf("chunk 2 should contain overlap from chunk 1: %q", got[2].Content)
	}
}

func TestChunk_OverlapSentenceBoundary(t *testing.T) {
	// Build two long paragraphs where the first paragraph ends with a distinctive
	// final sentence. The overlap should include that final sentence.
	p1 := strings.Repeat("Sentence one in paragraph one. ", 15) +
		"Final distinctive sentence of paragraph one."
	p2 := strings.Repeat("Another paragraph sentence. ", 15)
	text := p1 + "\n\n" + p2
	got := ChunkText(text)
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(got))
	}
	// Chunk 1 should include "Final distinctive" from p1's tail.
	if !strings.Contains(got[1].Content, "Final distinctive") {
		t.Errorf("overlap should include the final sentence: %q", got[1].Content)
	}
}

func TestChunk_OverlapOffsetUnchanged(t *testing.T) {
	// Offset values must NOT change when overlap is added.
	longPara := strings.Repeat("Long paragraph with enough characters to avoid short-chunk merging. ", 6)
	text := "# Title\n\n" + longPara + "\n\n" + longPara
	got := ChunkText(text)
	if len(got) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(got))
	}
	if got[0].Offset < 0 {
		t.Errorf("chunk 0 offset should be >= 0, got %d", got[0].Offset)
	}
	if got[1].Offset <= got[0].Offset {
		t.Errorf("chunk 1 offset (%d) should be > chunk 0 offset (%d)", got[1].Offset, got[0].Offset)
	}
	if got[0].Section != "# Title" {
		t.Errorf("chunk 0 section should be '# Title', got %q", got[0].Section)
	}
}

func TestMergeSemanticNeighbors_SimilarMerged(t *testing.T) {
	// Two chunks with identical content produce identical MockEmbedder vectors → high similarity → merged.
	embedder := NewMockEmbedder(4)
	chunks := []ChunkWithMeta{
		{Content: "The quick brown fox jumps over the lazy dog.", Section: "## Intro", Offset: 0},
		{Content: "The quick brown fox jumps over the lazy dog.", Section: "## Intro", Offset: 50},
	}
	result, err := MergeSemanticNeighbors(context.Background(), chunks, embedder, 0.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 merged chunk, got %d", len(result))
	}
	if !strings.Contains(result[0].Content, "quick brown fox") {
		t.Errorf("merged content should contain both texts: %q", result[0].Content)
	}
	if result[0].Section != "## Intro" {
		t.Errorf("section should be from first chunk, got %q", result[0].Section)
	}
	if result[0].Offset != 0 {
		t.Errorf("offset should be from first chunk, got %d", result[0].Offset)
	}
}

func TestMergeSemanticNeighbors_DissimilarKept(t *testing.T) {
	// Two chunks with sufficiently different content. MockEmbedder produces
	// different vectors for different text; use a high threshold to split.
	embedder := NewMockEmbedder(4)
	chunks := []ChunkWithMeta{
		{Content: "AAAAA", Section: "## One", Offset: 0},
		{Content: "BBBBB", Section: "## Two", Offset: 100},
	}
	result, err := MergeSemanticNeighbors(context.Background(), chunks, embedder, 0.99)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 separate chunks, got %d", len(result))
	}
}

func TestMergeSemanticNeighbors_Empty(t *testing.T) {
	embedder := NewMockEmbedder(4)
	result, err := MergeSemanticNeighbors(context.Background(), nil, embedder, 0.75)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestMergeSemanticNeighbors_SingleChunk(t *testing.T) {
	embedder := NewMockEmbedder(4)
	chunks := []ChunkWithMeta{
		{Content: "Just one chunk.", Section: "## Intro", Offset: 0},
	}
	result, err := MergeSemanticNeighbors(context.Background(), chunks, embedder, 0.75)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(result))
	}
}

func TestMergeSemanticNeighbors_NilEmbedder(t *testing.T) {
	// When embedder is nil, chunks should be returned unchanged.
	chunks := []ChunkWithMeta{
		{Content: "First.", Section: "## A", Offset: 0},
		{Content: "Second.", Section: "## B", Offset: 10},
	}
	result, err := MergeSemanticNeighbors(context.Background(), chunks, nil, 0.75)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 chunks unchanged, got %d", len(result))
	}
}

func TestMergeSemanticNeighbors_ChainMerge(t *testing.T) {
	// Three chunks with identical content: first and second merge, then
	// third compares against the first (original index kept).
	embedder := NewMockEmbedder(4)
	chunks := []ChunkWithMeta{
		{Content: "Alpha content here.", Section: "## X", Offset: 0},
		{Content: "Alpha content here.", Section: "## X", Offset: 25},
		{Content: "Alpha content here.", Section: "## X", Offset: 50},
	}
	result, err := MergeSemanticNeighbors(context.Background(), chunks, embedder, 0.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected all 3 merged into 1, got %d", len(result))
	}
}

func TestChunkTextHierarchical_Sections(t *testing.T) {
	longPara := strings.Repeat("Long paragraph with enough characters to avoid short-chunk merging. ", 6)
	text := "# Title\n\n" + longPara + "\n\n" + longPara +
		"\n\n## Body\n\n" + longPara + "\n\n" + longPara
	fine, coarse := ChunkTextHierarchical(text)
	if len(fine) < 2 {
		t.Fatalf("expected at least 2 fine chunks, got %d", len(fine))
	}
	if len(coarse) != 2 {
		t.Fatalf("expected 2 coarse chunks (Title + Body), got %d", len(coarse))
	}
	if coarse[0].Section != "# Title" {
		t.Errorf("coarse[0] section: got %q, want '# Title'", coarse[0].Section)
	}
	if coarse[1].Section != "## Body" {
		t.Errorf("coarse[1] section: got %q, want '## Body'", coarse[1].Section)
	}
	for _, c := range fine {
		if c.SectionID == "" {
			t.Errorf("fine chunk with section %q has empty SectionID", c.Section)
		}
	}
}

func TestChunkTextHierarchical_Empty(t *testing.T) {
	fine, coarse := ChunkTextHierarchical("")
	if fine != nil || coarse != nil {
		t.Errorf("expected nil for empty input, got fine=%v, coarse=%v", fine, coarse)
	}
}

func TestChunkTextHierarchical_NoHeadings(t *testing.T) {
	longPara := strings.Repeat("Long paragraph with enough characters to avoid short-chunk merging. ", 6)
	text := longPara + "\n\n" + longPara
	fine, coarse := ChunkTextHierarchical(text)
	if len(fine) < 2 {
		t.Fatalf("expected at least 2 fine chunks, got %d", len(fine))
	}
	if len(coarse) != 1 {
		t.Fatalf("expected 1 coarse chunk (no headings), got %d", len(coarse))
	}
	if coarse[0].Section != "" {
		t.Errorf("coarse chunk section should be empty for headingless text, got %q", coarse[0].Section)
	}
}
