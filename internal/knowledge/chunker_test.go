package knowledge

import (
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
	// ChunkTextContent should return just the content strings.
	longPara := strings.Repeat("Long paragraph with enough characters to avoid short-chunk merging. ", 6)
	// TrimSpace is applied per-paragraph, so trailing spaces are stripped.
	trimmed := strings.TrimSpace(longPara)
	text := longPara + "\n\n" + longPara
	got := ChunkTextContent(text)
	if len(got) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(got))
	}
	if got[0] != trimmed {
		t.Errorf("chunk 0: got %q, want %q", got[0], trimmed)
	}
	if got[1] != trimmed {
		t.Errorf("chunk 1: got %q, want %q", got[1], trimmed)
	}
}
