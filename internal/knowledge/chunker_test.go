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
	if len(got) != 1 || got[0] != "hello world" {
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
	if !strings.Contains(got[0], "OK") {
		t.Errorf("short paragraph not merged: %q", got[0])
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
		if len(c) > 2200 {
			t.Errorf("chunk %d is still too long: %d chars", i, len(c))
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
	if !strings.Contains(got[0], "\n") {
		t.Error("single newlines within paragraph should be preserved")
	}
}
