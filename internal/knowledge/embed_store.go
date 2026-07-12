package knowledge

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

// EmbeddingsDir returns the path to a document's embeddings directory.
func (s *Store) EmbeddingsDir(slug string) string {
	return filepath.Join(s.DocDir(slug), "embeddings")
}

// EmbeddingPath returns the path to a single embedding file (e.g. "000.f32").
func (s *Store) EmbeddingPath(slug, chunkID string) string {
	return filepath.Join(s.EmbeddingsDir(slug), chunkID+".f32")
}

// WriteEmbeddings writes a batch of float32 vectors to the embeddings directory.
// Each vector is written as a separate .f32 file in little-endian format,
// named after the chunk ID (000.f32, 001.f32, ...).
func (s *Store) WriteEmbeddings(slug string, vectors [][]float32) error {
	dir := s.EmbeddingsDir(slug)
	// Start fresh.
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove old embeddings: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create embeddings dir: %w", err)
	}
	for i, vec := range vectors {
		chunkID := fmt.Sprintf("%03d", i)
		if err := writeFloat32Vec(s.EmbeddingPath(slug, chunkID), vec); err != nil {
			return fmt.Errorf("write embedding %s: %w", chunkID, err)
		}
	}
	return nil
}

// ReadEmbedding reads a single embedding vector file.
func (s *Store) ReadEmbedding(slug, chunkID string) ([]float32, error) {
	return readFloat32Vec(s.EmbeddingPath(slug, chunkID))
}

// ListEmbeddingIDs returns all chunk IDs that have embedding files, sorted.
func (s *Store) ListEmbeddingIDs(slug string) ([]string, error) {
	dir := s.EmbeddingsDir(slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list embeddings for %q: %w", slug, err)
	}
	var ids []string
	for _, e := range entries {
		name := e.Name()
		if len(name) == 7 && name[3:] == ".f32" {
			ids = append(ids, name[:3])
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// cosineSimilarity computes the cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		fa := float64(a[i])
		fb := float64(b[i])
		dot += fa * fb
		normA += fa * fa
		normB += fb * fb
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// writeFloat32Vec writes a float32 slice as little-endian binary.
func writeFloat32Vec(path string, vec []float32) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return binary.Write(f, binary.LittleEndian, vec)
}

// readFloat32Vec reads a float32 slice from little-endian binary.
func readFloat32Vec(path string) ([]float32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	vec := make([]float32, len(data)/4)
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &vec); err != nil {
		return nil, fmt.Errorf("read embedding: %w", err)
	}
	return vec, nil
}
