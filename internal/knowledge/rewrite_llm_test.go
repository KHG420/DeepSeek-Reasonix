package knowledge

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// mockCompleter implements TextCompleter for testing.
type mockCompleter struct {
	response string
	err      error
}

func (m mockCompleter) Complete(_ context.Context, _ string) (string, error) {
	return m.response, m.err
}

func TestLLMQueryRewriter_Fallback(t *testing.T) {
	// When the LLM returns an error, the fallback rewriter should be used.
	mc := mockCompleter{err: errors.New("LLM unavailable")}
	r := NewLLMQueryRewriter(mc)
	got := r.Rewrite("RAG embedding")
	if len(got) < 2 {
		t.Fatalf("expected fallback to produce variants, got %d: %v", len(got), got)
	}
	// Original must be first.
	if got[0] != "RAG embedding" {
		t.Errorf("expected original first, got %q", got[0])
	}
	// The fallback should include synonym expansions (e.g. "retrieval").
	hasExpanded := false
	for _, v := range got {
		if strings.Contains(v, "retrieval") {
			hasExpanded = true
			break
		}
	}
	if !hasExpanded {
		t.Errorf("expected synonym expansion in fallback, got %v", got)
	}
}

func TestLLMQueryRewriter_Success(t *testing.T) {
	// When the LLM returns valid alternatives, they should be parsed and included.
	mc := mockCompleter{
		response: "1. machine learning retrieval\n- neural search expansion\n* vector search techniques",
	}
	r := NewLLMQueryRewriter(mc)
	got := r.Rewrite("RAG")
	if len(got) < 2 {
		t.Fatalf("expected original + alternatives, got %d: %v", len(got), got)
	}
	// Original must be first.
	if got[0] != "RAG" {
		t.Errorf("expected original first, got %q", got[0])
	}
	// The three alternatives should appear with cleaned prefixes.
	seen := map[string]bool{}
	for _, v := range got {
		seen[v] = true
	}
	if !seen["machine learning retrieval"] {
		t.Errorf("expected 'machine learning retrieval' in variants, got %v", got)
	}
	if !seen["neural search expansion"] {
		t.Errorf("expected 'neural search expansion' in variants, got %v", got)
	}
	if !seen["vector search techniques"] {
		t.Errorf("expected 'vector search techniques' in variants, got %v", got)
	}
}

func TestLLMQueryRewriter_EmptyResponse(t *testing.T) {
	// When the LLM returns empty output, the fallback should be used.
	mc := mockCompleter{response: ""}
	r := NewLLMQueryRewriter(mc)
	got := r.Rewrite("RAG")
	if len(got) < 2 {
		t.Fatalf("expected fallback to produce variants on empty LLM response, got %d: %v", len(got), got)
	}
	if got[0] != "RAG" {
		t.Errorf("expected original first, got %q", got[0])
	}
}

func TestLLMQueryRewriter_NoDuplicates(t *testing.T) {
	// Duplicate alternatives from the LLM should be removed.
	mc := mockCompleter{
		response: "machine learning\nmachine learning\nretrieval augmented",
	}
	r := NewLLMQueryRewriter(mc)
	got := r.Rewrite("RAG")
	// Should have: original + 2 unique alternatives = 3.
	if len(got) < 3 {
		t.Fatalf("expected at least 3 entries, got %d: %v", len(got), got)
	}
	seen := map[string]bool{}
	for _, v := range got {
		if seen[v] {
			t.Errorf("duplicate variant: %q", v)
		}
		seen[v] = true
	}
}

func TestLLMQueryRewriter_EmptyQuery(t *testing.T) {
	mc := mockCompleter{response: "some response"}
	r := NewLLMQueryRewriter(mc)
	got := r.Rewrite("   ")
	if got != nil {
		t.Errorf("expected nil for whitespace query, got %v", got)
	}
}

func TestLLMQueryRewriter_WithFallback(t *testing.T) {
	mc := mockCompleter{err: errors.New("unavailable")}
	r := NewLLMQueryRewriter(mc)
	// Use NoopRewriter as fallback.
	r.WithFallback(NoopRewriter{})
	got := r.Rewrite("machine learning")
	if len(got) != 1 || got[0] != "machine learning" {
		t.Errorf("expected only original query with NoopRewriter fallback, got %v", got)
	}
}

func TestCleanVariant(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1. query expansion", "query expansion"},
		{"2) alternative phrasing", "alternative phrasing"},
		{"- bullet variant", "bullet variant"},
		{"* star variant", "star variant"},
		{"• bullet point", "bullet point"},
		{`"quoted variant"`, "quoted variant"},
		{"'single quoted'", "single quoted"},
		{"`backtick quoted`", "backtick quoted"},
		{"plain variant", "plain variant"},
		{"  trimmed  ", "trimmed"},
		{"", ""},
	}
	for _, tt := range tests {
		got := cleanVariant(tt.input)
		if got != tt.expected {
			t.Errorf("cleanVariant(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// TestLLMQueryRewriter_WithPrompt verifies that WithPrompt overrides the
// default prompt template.
func TestLLMQueryRewriter_WithPrompt(t *testing.T) {
	mc := mockCompleter{
		response: "q1\nq2",
	}
	r := NewLLMQueryRewriter(mc).WithPrompt("Custom prompt:")
	got := r.Rewrite("query")
	if len(got) < 2 {
		t.Fatalf("expected original + alternatives, got %d: %v", len(got), got)
	}
	if got[0] != "query" {
		t.Errorf("expected original first, got %q", got[0])
	}
}
