// Package knowledge implements a local, file-based knowledge base that stores
// arbitrary documents as paragraph-level chunks and retrieves them via BM25
// text search. It sits alongside the memory subsystem but is independent: memory
// stores discrete facts as frontmatter .md files indexed by MEMORY.md (which
// loads into the system-prompt prefix), while knowledge stores full documents in
// .reasonix/knowledge/<slug>/chunks/*.md and is queried at runtime via a tool —
// its content NEVER enters the system-prompt prefix, keeping the DeepSeek
// prefix-cache warm regardless of knowledge-base size.
//
// Layout:
//
//	.reasonix/knowledge/
//	├── INDEX.md                   ← document-level index (runtime, not prefix)
//	└── <document-slug>/
//	    ├── meta.json              ← {original_name, source_type, added_at, chunk_count, total_chars}
//	    ├── source.<ext>           ← original file (preserved for audit)
//	    └── chunks/
//	        ├── 000.md
//	        ├── 001.md
//	        └── ...
//
// Document parsing uses tsawler/tabula (MIT, pure Go) for PDF/DOCX/ODT/EPUB/
// HTML/XLSX/PPTX/MD/TXT; chunking splits on paragraph boundaries with
// short-chunk merging and long-chunk sentence-boundary re-splitting; search
// reuses the internal/retrieval BM25 engine already in use by history/memory.
package knowledge

import "time"

// DocumentMeta is the per-document metadata persisted in meta.json.
type DocumentMeta struct {
	OriginalName string    `json:"original_name"`
	SourceType   string    `json:"source_type"` // e.g. "pdf", "docx", "md", "txt"
	AddedAt      time.Time `json:"added_at"`
	ChunkCount   int       `json:"chunk_count"`
	TotalChars   int       `json:"total_chars"`
}

// Chunk is a single paragraph-level slice of a document, stored as 000.md etc.
type Chunk struct {
	ID      string // e.g. "005"
	Content string // raw Markdown content of the chunk file
}

// SearchHit is one ranked result from a BM25 search over chunks.
type SearchHit struct {
	Score   float64
	DocSlug string
	ChunkID string
	Snippet string // whitespace-compacted excerpt centered on the query
}
