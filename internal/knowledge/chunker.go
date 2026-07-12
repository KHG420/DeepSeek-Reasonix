package knowledge

import (
	"strings"
	"unicode/utf8"
)

const (
	shortChunk = 200  // chars below this are merged into the preceding chunk
	longChunk  = 2000 // chars above this are re-split on sentence boundaries
)

// ChunkText splits text into paragraph-level chunks suitable for BM25 retrieval.
//
// Algorithm:
//  1. Split on "\n\n" to get paragraphs.
//  2. Merge short paragraphs (< 200 chars) into the previous chunk.
//  3. Re-split long paragraphs (> 2000 chars) on sentence boundaries
//     (。.！!？?) so no single chunk is overwhelmingly large.
//
// Empty input returns nil.
func ChunkText(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// Step 1: split into raw paragraphs.
	raw := splitParagraphs(text)

	// Step 2: merge short paragraphs backward.
	merged := mergeShort(raw)

	// Step 3: split long paragraphs on sentence boundaries.
	var out []string
	for _, p := range merged {
		if utf8.RuneCountInString(p) > longChunk {
			out = append(out, splitLong(p)...)
		} else {
			out = append(out, p)
		}
	}
	return out
}

// splitParagraphs splits text on double-newline boundaries, preserving
// single newlines within a paragraph.
func splitParagraphs(text string) []string {
	// Normalise \r\n → \n, then split on \n\n.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	parts := strings.Split(text, "\n\n")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// mergeShort merges paragraphs shorter than shortChunk chars into the
// preceding chunk. The first paragraph is never merged "upward" — if it is
// short it stays as-is (there is no predecessor).
func mergeShort(paras []string) []string {
	if len(paras) <= 1 {
		return paras
	}
	var out []string
	for _, p := range paras {
		if len(out) > 0 && utf8.RuneCountInString(p) < shortChunk {
			// Merge into the previous chunk.
			out[len(out)-1] += "\n\n" + p
		} else {
			out = append(out, p)
		}
	}
	return out
}

// splitLong splits a single long paragraph on sentence boundaries.
// It tries to cut at 。.！!？? and keeps each piece under ~longChunk chars.
func splitLong(text string) []string {
	sentences := splitSentences(text)
	if len(sentences) <= 1 {
		// Could not find sentence boundaries; return as-is.
		return []string{text}
	}

	var out []string
	var buf strings.Builder
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		// If adding this sentence would exceed the limit and we already
		// have content, flush the buffer.
		if buf.Len() > 0 && utf8.RuneCountInString(buf.String())+utf8.RuneCountInString(s) > longChunk {
			out = append(out, strings.TrimSpace(buf.String()))
			buf.Reset()
		}
		if buf.Len() > 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(s)
	}
	if buf.Len() > 0 {
		out = append(out, strings.TrimSpace(buf.String()))
	}
	return out
}

// splitSentences splits text at sentence-ending punctuation marks
// (。.！!？?) while keeping the punctuation attached to its sentence.
func splitSentences(text string) []string {
	var out []string
	start := 0
	runes := []rune(text)
	for i, r := range runes {
		if isSentenceEnd(r) {
			// Include the punctuation in this sentence.
			out = append(out, string(runes[start:i+1]))
			start = i + 1
		}
	}
	// Remainder after the last punctuation.
	if start < len(runes) {
		rem := strings.TrimSpace(string(runes[start:]))
		if rem != "" {
			out = append(out, rem)
		}
	}
	return out
}

func isSentenceEnd(r rune) bool {
	switch r {
	case '。', '.', '！', '!', '？', '?':
		return true
	}
	return false
}
