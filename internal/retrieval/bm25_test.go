package retrieval

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTokensHandlesLatinAndCJK(t *testing.T) {
	got := Tokens("BM25 检索 cache-first")
	want := []string{"bm25", "检", "索", "检索", "cache", "first"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Tokens() = %#v, want %#v", got, want)
	}
}

func TestBM25ScoreRanksMatchingDocument(t *testing.T) {
	query := Unique(Tokens("prompt cache"))
	doc1 := Counts(Tokens("prompt cache cache stability"))
	doc2 := Counts(Tokens("dashboard colors"))
	df := DocumentFrequency([]map[string]int{doc1, doc2})
	score1 := BM25Score(doc1, 4, query, df, 2, 3)
	score2 := BM25Score(doc2, 2, query, df, 2, 3)
	if score1 <= score2 {
		t.Fatalf("matching score %.3f should exceed unrelated score %.3f", score1, score2)
	}
}

func TestKeepTopRelativeScoreKeepsTopAndDropsWeakTail(t *testing.T) {
	items := []struct {
		name  string
		score float64
	}{
		{name: "top", score: 10},
		{name: "near", score: 2},
		{name: "noise", score: 1.4},
		{name: "zero", score: 0},
	}
	got := KeepTopRelativeScore(items, 0.15, func(item struct {
		name  string
		score float64
	}) float64 {
		return item.score
	})
	if len(got) != 2 || got[0].name != "top" || got[1].name != "near" {
		t.Fatalf("KeepTopRelativeScore() = %#v, want top and near", got)
	}
}

func TestMakeSnippetHandlesMultibyteBoundary(t *testing.T) {
	text := strings.Repeat("前缀", 80) + "稳定结论 synthesis cache " + strings.Repeat("后缀", 80)
	out := MakeSnippet(text, "synthesis cache", QueryTermsForTest(t, "synthesis cache"), 60)
	if !strings.Contains(out, "synthesis cache") {
		t.Fatalf("snippet missing query: %q", out)
	}
	if strings.ContainsRune(out, utf8.RuneError) {
		t.Fatalf("snippet contains replacement rune: %q", out)
	}
}

func TestMakeSnippetSentenceBoundaryChinese(t *testing.T) {
	// Chinese text with sentence punctuation. "新模型" appears as a contiguous
	// substring in the second sentence.
	text := "本文介绍了深度学习在自然语言处理中的应用。我们提出了新模型架构。实验结果表明该方法效果显著。"
	out := MakeSnippet(text, "新模型", QueryTermsForTest(t, "新模型"), 30)
	if !strings.Contains(out, "新模型") {
		t.Fatalf("snippet missing query: %q", out)
	}
	// Should start at a sentence boundary, not in the middle.
	if strings.HasPrefix(out, "们提出了") {
		t.Fatalf("snippet should not start mid-sentence: %q", out)
	}
}

func TestMakeSnippetSentenceBoundaryEnglish(t *testing.T) {
	text := "This is the first sentence. The model architecture is described here. Experimental results show great performance."
	out := MakeSnippet(text, "model architecture", QueryTermsForTest(t, "model architecture"), 40)
	if !strings.Contains(out, "model architecture") {
		t.Fatalf("snippet missing query: %q", out)
	}
}

func TestMakeSnippetFallbackNoPunctuation(t *testing.T) {
	// Text with no sentence punctuation should fall back to centered window.
	text := strings.Repeat("word ", 200)
	out := MakeSnippet(text, "word", QueryTermsForTest(t, "word"), 100)
	// Content window is ≤ maxRunes; "..." suffix may add 3 extra runes.
	if utf8.RuneCountInString(out) > 106 {
		t.Fatalf("snippet too long without punctuation fallback: %d runes", utf8.RuneCountInString(out))
	}
	if !strings.Contains(out, "word") {
		t.Fatalf("snippet missing query term: %q", out)
	}
}

func TestMakeSnippetShortTextNoTruncation(t *testing.T) {
	text := "Short text with no need to truncate."
	out := MakeSnippet(text, "text", QueryTermsForTest(t, "text"), 200)
	if out != text {
		t.Fatalf("short text should not be truncated: %q, want %q", out, text)
	}
}

func QueryTermsForTest(t *testing.T, query string) []string {
	t.Helper()
	terms, err := QueryTerms(query)
	if err != nil {
		t.Fatal(err)
	}
	return terms
}
