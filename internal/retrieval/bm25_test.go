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

func TestTokensHandlesArabic(t *testing.T) {
	// Arabic text: "السلام عليكم" (hello)
	got := Tokens("السلام عليكم")
	// Each Arabic character should produce unigrams and bigrams, not be dropped.
	if len(got) == 0 {
		t.Fatal("Tokens() produced no output for Arabic text")
	}
	// Should not be split into just the Latin/digit path — Arabic characters
	// are not letters in the Latin sense, they go through the continuous script path.
	for _, tok := range got {
		if len(tok) == 0 {
			t.Errorf("empty token in output")
		}
	}
	// Ensure single characters appear (unigram).
	hasUnigram := false
	for _, tok := range got {
		if len([]rune(tok)) == 1 {
			hasUnigram = true
			break
		}
	}
	if !hasUnigram {
		t.Errorf("expected unigrams for Arabic, got: %#v", got)
	}
}

func TestTokensHandlesThai(t *testing.T) {
	// Thai text: "สวัสดี" (hello)
	got := Tokens("สวัสดี")
	if len(got) == 0 {
		t.Fatal("Tokens() produced no output for Thai text")
	}
	// Bigrams should be present for consecutive Thai characters.
	hasBigram := false
	for _, tok := range got {
		if len([]rune(tok)) == 2 {
			hasBigram = true
			break
		}
	}
	if !hasBigram {
		t.Errorf("expected bigrams for Thai, got: %#v", got)
	}
}

func TestTokensHandlesDevanagari(t *testing.T) {
	// Hindi text: "नमस्ते" (hello)
	got := Tokens("नमस्ते")
	if len(got) == 0 {
		t.Fatal("Tokens() produced no output for Devanagari text")
	}
}

func TestTokensHandlesMixedScripts(t *testing.T) {
	// Mixed: Arabic + Latin + Arabic
	got := Tokens("مرحبا world السلام")
	// Latin word should be lowercased.
	hasLatin := false
	for _, tok := range got {
		if tok == "world" {
			hasLatin = true
			break
		}
	}
	if !hasLatin {
		t.Errorf("expected Latin token 'world' in mixed-script output, got: %#v", got)
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

func QueryTermsForTest(t *testing.T, query string) []string {
	t.Helper()
	terms, err := QueryTerms(query)
	if err != nil {
		t.Fatal(err)
	}
	return terms
}

func TestQueryTermsFiltersStopWords(t *testing.T) {
	// Pure stop words should error.
	_, err := QueryTerms("the a an of")
	if err == nil {
		t.Error("expected error for query with only stop words")
	}
	if !strings.Contains(err.Error(), "stop words") {
		t.Errorf("expected 'stop words' in error, got %v", err)
	}

	// Mixed: stop words should be removed, meaningful words retained.
	terms, err := QueryTerms("the machine learning and AI")
	if err != nil {
		t.Fatal(err)
	}
	// "the", "and" should be removed; "machine", "learning", "ai" kept.
	for _, sw := range []string{"the", "and"} {
		for _, t2 := range terms {
			if t2 == sw {
				t.Errorf("stop word %q should have been filtered, got %#v", sw, terms)
			}
		}
	}
	hasML := false
	hasAI := false
	for _, t2 := range terms {
		if t2 == "machine" {
			hasML = true
		}
		if t2 == "ai" {
			hasAI = true
		}
	}
	if !hasML || !hasAI {
		t.Errorf("expected 'machine' and 'ai' in terms, got %#v", terms)
	}
}

func TestQueryTermsFiltersChineseStopWords(t *testing.T) {
	// Chinese stop words should be filtered.
	terms, err := QueryTerms("\u7684 \u673a\u5668\u5b66\u4e60 \u548c \u7684")
	if err != nil {
		t.Fatal(err)
	}
	for _, sw := range []string{"\u7684", "\u548c"} {
		for _, t2 := range terms {
			if t2 == sw {
				t.Errorf("Chinese stop word %q should have been filtered, got %#v", sw, terms)
			}
		}
	}
}

func TestQueryTermsPureStopWordsError(t *testing.T) {
	_, err := QueryTerms("\u7684 \u4e86 \u662f")
	if err == nil {
		t.Error("expected error for purely Chinese stop word query")
	}
}
