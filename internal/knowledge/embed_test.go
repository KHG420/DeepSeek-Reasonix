package knowledge

import (
	"context"
	"math"
	"testing"
)

func TestMockEmbedder_Basic(t *testing.T) {
	m := NewMockEmbedder(4)
	if m.Dim() != 4 {
		t.Errorf("expected dim 4, got %d", m.Dim())
	}

	vecs, err := m.Embed(context.Background(), []string{"hello world", "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}
	for i, v := range vecs {
		if len(v) != 4 {
			t.Errorf("vector %d: expected len 4, got %d", i, len(v))
		}
	}
}

func TestMockEmbedder_Deterministic(t *testing.T) {
	m := NewMockEmbedder(8)
	ctx := context.Background()

	v1, _ := m.Embed(ctx, []string{"same text"})
	v2, _ := m.Embed(ctx, []string{"same text"})

	if len(v1) != 1 || len(v2) != 1 {
		t.Fatal("expected one vector each")
	}
	for i := range v1[0] {
		if v1[0][i] != v2[0][i] {
			t.Errorf("deterministic vector mismatch at %d: %f vs %f", i, v1[0][i], v2[0][i])
			break
		}
	}
}

func TestMockEmbedder_DifferentTexts(t *testing.T) {
	m := NewMockEmbedder(4)
	ctx := context.Background()

	// Different texts should produce different vectors.
	v1, _ := m.Embed(ctx, []string{"alpha"})
	v2, _ := m.Embed(ctx, []string{"beta"})

	if len(v1) != 1 || len(v2) != 1 {
		t.Fatal("expected one vector each")
	}
	same := true
	for i := range v1[0] {
		if v1[0][i] != v2[0][i] {
			same = false
			break
		}
	}
	if same {
		t.Error("different texts produced identical vectors")
	}
}

func TestMockEmbedder_EmptyTexts(t *testing.T) {
	m := NewMockEmbedder(4)
	vecs, err := m.Embed(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 0 {
		t.Errorf("expected 0 vectors for nil, got %d", len(vecs))
	}

	vecs, err = m.Embed(context.Background(), []string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 0 {
		t.Errorf("expected 0 vectors for empty, got %d", len(vecs))
	}
}

func TestCosineSimilarity_Identical(t *testing.T) {
	a := []float64{1, 2, 3}
	b := []float64{1, 2, 3}
	sim := cosineSimilarity(a, b)
	if math.Abs(sim-1.0) > 1e-9 {
		t.Errorf("identical vectors should have similarity 1, got %f", sim)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{0, 1, 0}
	sim := cosineSimilarity(a, b)
	if math.Abs(sim) > 1e-9 {
		t.Errorf("orthogonal vectors should have similarity 0, got %f", sim)
	}
}

func TestCosineSimilarity_Opposite(t *testing.T) {
	a := []float64{1, 2, 3}
	b := []float64{-1, -2, -3}
	sim := cosineSimilarity(a, b)
	if math.Abs(sim+1.0) > 1e-9 {
		t.Errorf("opposite vectors should have similarity -1, got %f", sim)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	a := []float64{1, 2, 3}
	b := []float64{1, 2}
	sim := cosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("different length vectors should return 0, got %f", sim)
	}
}

func TestCosineSimilarity_ZeroVector(t *testing.T) {
	a := []float64{0, 0, 0}
	b := []float64{1, 2, 3}
	sim := cosineSimilarity(a, b)
	if sim != 0 {
		t.Errorf("zero vector should return 0, got %f", sim)
	}
}

func TestSetEmbedder(t *testing.T) {
	s := tempStore(t)
	if s.embedder != nil {
		t.Error("expected nil embedder by default")
	}

	m := NewMockEmbedder(4)
	s.SetEmbedder(m)
	if s.embedder == nil {
		t.Fatal("expected non-nil embedder after SetEmbedder")
	}
	if s.embedder.Dim() != 4 {
		t.Errorf("expected dim 4, got %d", s.embedder.Dim())
	}
}
