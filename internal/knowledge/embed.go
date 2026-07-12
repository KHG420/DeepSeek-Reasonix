package knowledge

import "context"

// Embedder computes vector embeddings for text chunks.
// Implementations may call an external API or use a local model.
type Embedder interface {
	// Embed returns a float32 vector for a single text.
	Embed(ctx context.Context, text string) ([]float32, error)
	// BatchEmbed returns vectors for multiple texts (for efficiency).
	BatchEmbed(ctx context.Context, texts []string) ([][]float32, error)
	// Dims returns the embedding dimension.
	Dims() int
}

// EmbeddingConfig holds optional embedding configuration.
// When Embedder is nil, the knowledge base uses BM25-only search.
type EmbeddingConfig struct {
	// Embedder is the embedding provider. If nil, BM25-only search is used.
	Embedder Embedder
}
