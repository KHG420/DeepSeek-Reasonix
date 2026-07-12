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
//  1. Scan for markdown headings to track section boundaries.
//  2. Split on "\n\n" to get paragraphs, tracking character offsets.
//  3. Merge short paragraphs (< 200 chars) into the previous chunk.
//  4. Re-split long paragraphs (> 2000 chars) on sentence boundaries
//     (。.！!？?) so no single chunk is overwhelmingly large.
//
// Empty input returns nil.
func ChunkText(text string) []ChunkWithMeta {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// Build section map: for each character offset, what section are we in.
	sections := buildSectionMap(text)

	// Step 1: split into raw paragraphs with offsets.
	raw := splitParagraphsWithOffset(text)

	// Step 2: merge short paragraphs backward.
	merged := mergeShortWithOffset(raw)

	// Step 3: split long paragraphs on sentence boundaries.
	var out []ChunkWithMeta
	for _, p := range merged {
		sec := sectionAt(sections, p.offset)
		if utf8.RuneCountInString(p.content) > longChunk {
			for _, sub := range splitLong(p.content) {
				out = append(out, ChunkWithMeta{
					Content: sub,
					Section: sec,
					Offset:  p.offset,
				})
			}
		} else {
			out = append(out, ChunkWithMeta{
				Content: p.content,
				Section: sec,
				Offset:  p.offset,
			})
		}
	}
	return out
}

// paraWithOffset is a paragraph with its character offset in the original text.
type paraWithOffset struct {
	content string
	offset  int
}

// buildSectionMap scans text for markdown headings (lines starting with #)
// and returns a map: character offset → section heading text.
// The section for any position is the nearest preceding heading.
func buildSectionMap(text string) []sectionBoundary {
	normalised := strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(normalised, "\n")
	var boundaries []sectionBoundary
	offset := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if isHeading(trimmed) {
			boundaries = append(boundaries, sectionBoundary{offset: offset, heading: trimmed})
		}
		offset += len(line) + 1 // +1 for the newline
	}
	return boundaries
}

type sectionBoundary struct {
	offset  int
	heading string
}

// sectionAt returns the section heading for a given character offset.
func sectionAt(boundaries []sectionBoundary, offset int) string {
	sec := ""
	for _, b := range boundaries {
		if b.offset <= offset {
			sec = b.heading
		} else {
			break
		}
	}
	return sec
}

// isHeading reports whether line is a markdown heading (e.g. "# Title", "## Section").
func isHeading(line string) bool {
	if !strings.HasPrefix(line, "#") {
		return false
	}
	// Must have a space after the #s and not be something like "###"
	hashEnd := 0
	for hashEnd < len(line) && line[hashEnd] == '#' {
		hashEnd++
	}
	if hashEnd > 6 {
		return false // too many #s
	}
	return hashEnd < len(line) && line[hashEnd] == ' '
}

// splitParagraphsWithOffset splits text on double-newline boundaries and tracks
// the character offset of each paragraph in the normalised text.
func splitParagraphsWithOffset(text string) []paraWithOffset {
	// Normalise \r\n → \n, then split on \n\n.
	normalised := strings.ReplaceAll(text, "\r\n", "\n")
	parts := strings.Split(normalised, "\n\n")
	var out []paraWithOffset
	offset := 0
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			// Find the actual offset of this paragraph in the normalised text.
			idx := strings.Index(normalised[offset:], trimmed)
			if idx >= 0 {
				out = append(out, paraWithOffset{content: trimmed, offset: offset + idx})
			} else {
				out = append(out, paraWithOffset{content: trimmed, offset: offset})
			}
		}
		offset += len(p) + 2 // +2 for the "\n\n" separator
	}
	return out
}

// mergeShortWithOffset merges paragraphs shorter than shortChunk chars into the
// preceding chunk. The first paragraph is never merged "upward".
func mergeShortWithOffset(paras []paraWithOffset) []paraWithOffset {
	if len(paras) <= 1 {
		return paras
	}
	var out []paraWithOffset
	for _, p := range paras {
		if len(out) > 0 && utf8.RuneCountInString(p.content) < shortChunk {
			// Merge into the previous chunk; keep the offset of the first chunk.
			out[len(out)-1].content += "\n\n" + p.content
		} else {
			out = append(out, p)
		}
	}
	return out
}

// ChunkTextContent is a convenience wrapper that returns just the content strings,
// for callers that don't need position metadata.
func ChunkTextContent(text string) []string {
	chunks := ChunkText(text)
	out := make([]string, len(chunks))
	for i, c := range chunks {
		out[i] = c.Content
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
