package retrieval

import (
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Tokens lowercases Latin words and tokenises CJK and similar continuous-script
// text as unigrams + bigrams.
// For example, "知识库" becomes ["知", "知识", "识库", "库"]. Latin words and
// digits are emitted as single tokens; CJK bigrams improve phrase-level search
// accuracy (e.g. "船舶" matches as a unit, not just "船" + "舶").
// Arabic, Thai, Devanagari, and other scripts without inter-word spaces are
// treated the same as CJK to avoid single-character fragmentation.
func Tokens(s string) []string {
	var out []string
	var b strings.Builder
	var prevCJK rune // previous continuous-script character for bigram generation

	flush := func() {
		if b.Len() == 0 {
			return
		}
		out = append(out, b.String())
		b.Reset()
	}
	for _, r := range s {
		switch {
		case isContinuousScript(r):
			flush()
			// Unigram: the single character itself.
			out = append(out, string(r))
			// Bigram: combine with previous character of the same script type.
			if prevCJK != 0 {
				out = append(out, string(prevCJK)+string(r))
			}
			prevCJK = r
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			prevCJK = 0
			b.WriteRune(unicode.ToLower(r))
		default:
			prevCJK = 0
			flush()
		}
	}
	flush()
	return out
}

// isContinuousScript reports whether r belongs to a script that typically does
// not use spaces between words, such as CJK, Arabic, Thai, and Indic scripts.
// These benefit from unigram+bigram tokenisation rather than space-delimited
// word splitting.
func isContinuousScript(r rune) bool {
	return unicode.In(r,
		// CJK
		unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul,
		// Arabic script (Arabic, Persian, Urdu, etc.)
		unicode.Arabic,
		// Southeast Asian scripts (typically spacing-free)
		unicode.Thai, unicode.Lao, unicode.Khmer, unicode.Myanmar,
		// Indic / Brahmic scripts
		unicode.Devanagari, unicode.Bengali, unicode.Gurmukhi, unicode.Gujarati,
		unicode.Oriya, unicode.Tamil, unicode.Telugu, unicode.Kannada,
		unicode.Malayalam, unicode.Sinhala, unicode.Limbu,
		// Other continuous scripts
		unicode.Tibetan, unicode.Georgian, unicode.Ethiopic, unicode.Syriac,
		unicode.Thaana, unicode.Mongolian,
	)
}

// Unique returns terms in first-seen order.
func Unique(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// Counts returns a term-frequency map.
func Counts(terms []string) map[string]int {
	counts := map[string]int{}
	for _, term := range terms {
		counts[term]++
	}
	return counts
}

// BM25Score scores a document against query terms.
func BM25Score(counts map[string]int, length int, queryTerms []string, df map[string]int, totalDocs int, avgLen float64) float64 {
	const (
		k1 = 1.2
		b  = 0.75
	)
	if length <= 0 || totalDocs <= 0 {
		return 0
	}
	if avgLen <= 0 {
		avgLen = 1
	}
	var score float64
	docLen := float64(length)
	for _, term := range queryTerms {
		tf := counts[term]
		if tf == 0 {
			continue
		}
		termDF := df[term]
		if termDF == 0 {
			continue
		}
		idf := math.Log(1 + (float64(totalDocs)-float64(termDF)+0.5)/(float64(termDF)+0.5))
		freq := float64(tf)
		score += idf * (freq * (k1 + 1)) / (freq + k1*(1-b+b*docLen/avgLen))
	}
	return score
}

// DocumentFrequency counts how many documents contain each term.
func DocumentFrequency(docs []map[string]int) map[string]int {
	df := map[string]int{}
	for _, counts := range docs {
		for term := range counts {
			df[term]++
		}
	}
	return df
}

// KeepTopRelativeScore keeps the best item and drops trailing items whose score
// falls below ratio * topScore. Callers must pass items already sorted best
// first. This mirrors SQLite FTS/BM25 search UIs that over-fetch, then trim
// common-word-only noise without imposing an absolute score threshold.
func KeepTopRelativeScore[T any](items []T, ratio float64, score func(T) float64) []T {
	if len(items) == 0 || ratio <= 0 {
		return items
	}
	top := score(items[0])
	if top <= 0 {
		return items
	}
	cutoff := top * ratio
	out := items[:0]
	for i, item := range items {
		if i == 0 || score(item) >= cutoff {
			out = append(out, item)
		}
	}
	return out
}

// stopWords are high-frequency terms that add little search value and
// introduce noise. They are filtered from queries to improve result quality.
// Only the most obviously noisy words are included to avoid false negatives.
var stopWords = map[string]bool{
	// Chinese
	"的": true, "了": true, "和": true, "是": true, "在": true, "也": true,
	"就": true, "都": true, "不": true, "这": true, "那": true, "有": true,
	"人": true, "我": true, "他": true, "她": true, "它": true, "们": true,
	"与": true, "或": true, "但": true, "对": true, "从": true, "到": true,
	"以": true, "上": true, "下": true, "能": true, "会": true, "要": true,
	// English
	"the": true, "a": true, "an": true, "of": true, "in": true, "to": true,
	"is": true, "and": true, "or": true, "for": true, "on": true, "with": true,
	"at": true, "by": true, "it": true, "as": true, "be": true, "this": true,
	"that": true, "are": true, "was": true, "were": true, "been": true,
	"have": true, "has": true, "had": true, "not": true, "no": true,
	"from": true, "but": true, "so": true, "if": true,
	// Japanese (hiragana particles are common)
	"は": true, "が": true, "を": true, "に": true, "で": true, "と": true,
	"へ": true, "か": true, "も": true, "の": true, "て": true, "た": true,
}

// QueryTerms normalizes a search string and reports an error when nothing
// searchable remains. Stop words are filtered from the result.
func QueryTerms(query string) ([]string, error) {
	allTerms := Unique(Tokens(strings.TrimSpace(query)))
	// Filter stop words.
	terms := allTerms[:0]
	for _, t := range allTerms {
		if !stopWords[t] {
			terms = append(terms, t)
		}
	}
	if len(terms) == 0 {
		if len(allTerms) == 0 {
			return nil, fmt.Errorf("query must contain at least one letter or number")
		}
		return nil, fmt.Errorf("query contains only stop words")
	}
	return terms, nil
}

// MakeSnippet returns a whitespace-compacted excerpt centered near the query.
func MakeSnippet(text, query string, terms []string, maxRunes int) string {
	text = CompactWhitespace(text)
	if maxRunes <= 0 || utf8.RuneCountInString(text) <= maxRunes {
		return text
	}
	lower := strings.ToLower(text)
	query = strings.ToLower(strings.TrimSpace(query))
	idx := -1
	if query != "" {
		idx = strings.Index(lower, query)
	}
	if idx < 0 {
		for _, term := range terms {
			runes := []rune(term)
			if len(runes) == 1 && !isContinuousScript(runes[0]) {
				continue
			}
			if i := strings.Index(lower, term); i >= 0 {
				idx = i
				break
			}
		}
	}
	if idx < 0 {
		idx = 0
	}
	return snippetAround(text, idx, maxRunes)
}

func snippetAround(text string, byteIdx, maxRunes int) string {
	if byteIdx < 0 {
		byteIdx = 0
	}
	if byteIdx > len(text) {
		byteIdx = len(text)
	}
	for byteIdx > 0 && byteIdx < len(text) && !utf8.RuneStart(text[byteIdx]) {
		byteIdx--
	}
	runes := []rune(text)
	pos := utf8.RuneCountInString(text[:byteIdx])
	start := pos - maxRunes/2
	if start < 0 {
		start = 0
	}
	end := start + maxRunes
	if end > len(runes) {
		end = len(runes)
		start = end - maxRunes
		if start < 0 {
			start = 0
		}
	}
	prefix := ""
	suffix := ""
	if start > 0 {
		prefix = "..."
	}
	if end < len(runes) {
		suffix = "..."
	}
	return prefix + string(runes[start:end]) + suffix
}

// CompactWhitespace collapses runs of whitespace into one ASCII space.
func CompactWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
