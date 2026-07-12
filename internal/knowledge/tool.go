package knowledge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Tool implements tool.Tool for the knowledge base.
type Tool struct {
	store *Store
}

// NewTool returns a knowledge Tool backed by the given Store.
func NewTool(store *Store) *Tool {
	return &Tool{store: store}
}

func (t *Tool) Name() string { return "knowledge" }

func (t *Tool) Description() string {
	return "Search, read, list, upload, and remove documents in the project knowledge base. " +
		"When the user asks a domain-specific or technical question — especially about " +
		"uploaded manuals, specifications, references, or standards — search the " +
		"knowledge base FIRST with operation='search' before relying on general " +
		"knowledge. Use 'read' to get the full text of a chunk that looks relevant; " +
		"'list' to see all uploaded documents; 'upload' to ingest a document file; " +
		"'remove' to delete a document and all its chunks."
}

func (t *Tool) ReadOnly() bool { return false }

// Schema returns the JSON Schema for the tool's parameters.
func (t *Tool) Schema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "operation": {
      "type": "string",
      "enum": ["search", "read", "list", "upload", "remove"],
      "description": "search = BM25 search across all chunked documents; read = read a specific chunk by docSlug/chunkID; list = list all uploaded documents; upload = ingest a document file; remove = delete a document and its chunks"
    },
    "query": {
      "type": "string",
      "description": "Search query for operation='search'. Use distinctive keywords."
    },
    "docSlug": {
      "type": "string",
      "description": "Document slug for operation='read' or 'remove'. From list/search results."
    },
    "chunkID": {
      "type": "string",
      "description": "Chunk identifier (e.g. '005') for operation='read'. From search results."
    },
    "context": {
      "type": "integer",
      "description": "Number of adjacent chunks to include before and after the target chunk for operation='read'. Default 0 (just the chunk itself). Max 5."
    },
    "filePath": {
      "type": "string",
      "description": "Absolute or workspace-relative path to the document file for operation='upload'. Mutually exclusive with 'directory'."
    },
    "directory": {
      "type": "string",
      "description": "Directory path for batch upload (operation='upload'). Files with supported extensions (.md, .txt, .pdf, .docx, .odt, .epub, .html, .xlsx, .pptx) are ingested. Mutually exclusive with 'filePath'."
    },
    "recursive": {
      "type": "boolean",
      "description": "When true with operation='upload' and 'directory', recursively walk subdirectories."
    },
    "limit": {
      "type": "integer",
      "description": "Maximum search results to return. Default 8, max 20."
    },
    "sourceType": {
      "type": "string",
      "description": "Optional source type filter for operation='search', e.g. 'pdf', 'md', 'txt'. Only chunks from documents with this source type are returned."
    },
    "section": {
      "type": "string",
      "description": "Optional section filter for operation='search'. Only chunks whose section heading contains this substring are returned."
    },
    "mode": {
      "type": "string",
      "enum": ["bm25", "hybrid"],
      "description": "Search mode for operation='search'. 'bm25' (default) uses pure BM25 keyword matching. 'hybrid' combines BM25 with dense embedding similarity for better recall. Requires documents to have been uploaded with an embedder configured."
    },
    "level": {
      "type": "string",
      "enum": ["chunk", "section"],
      "description": "Read granularity for operation='read'. 'chunk' (default) reads the individual chunk. 'section' reads the entire section containing the chunk."
    }
  },
  "required": ["operation"]
}`)
}

// Execute dispatches on the "operation" field.
func (t *Tool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p knowledgeArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	p.Operation = strings.ToLower(strings.TrimSpace(p.Operation))
	switch p.Operation {
	case "search":
		return t.search(p)
	case "read":
		return t.read(p)
	case "list":
		return t.list()
	case "upload":
		return t.upload(p)
	case "remove":
		return t.remove(p)
	default:
		return "", fmt.Errorf("unknown operation %q, must be search|read|list|upload|remove", p.Operation)
	}
}

// --- arg struct ---

type knowledgeArgs struct {
	Operation  string `json:"operation"`
	Query      string `json:"query"`
	DocSlug    string `json:"docSlug"`
	ChunkID    string `json:"chunkID"`
	Context    int    `json:"context"`
	FilePath   string `json:"filePath"`
	Directory  string `json:"directory"`
	Recursive  bool   `json:"recursive"`
	Limit      int    `json:"limit"`
	SourceType string `json:"sourceType"`
	Section    string `json:"section"`
	Mode       string `json:"mode"`
	Level      string `json:"level"`
}

// --- operation handlers ---

func (t *Tool) search(p knowledgeArgs) (string, error) {
	limit := p.Limit
	if limit <= 0 {
		limit = 8
	}
	if limit > 20 {
		limit = 20
	}

	filter := SearchFilter{
		DocSlug:    p.DocSlug,
		SourceType: p.SourceType,
		Section:    p.Section,
	}

	var hits []SearchHit
	var err error
	switch strings.ToLower(p.Mode) {
	case "hybrid":
		hits, err = t.store.HybridSearch(p.Query, limit, filter)
	default:
		hits, err = t.store.Search(p.Query, limit, filter)
	}
	if err != nil {
		return "", err
	}
	if len(hits) == 0 {
		return "No matching chunks found.", nil
	}
	data, err := json.MarshalIndent(hits, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal results: %w", err)
	}
	return string(data), nil
}

func (t *Tool) read(p knowledgeArgs) (string, error) {
	if p.DocSlug == "" {
		return "", fmt.Errorf("docSlug is required for read")
	}
	if p.ChunkID == "" {
		return "", fmt.Errorf("chunkID is required for read")
	}

	// When level is "section", resolve the chunk's section chunk and read it.
	if strings.ToLower(p.Level) == "section" {
		return t.readSection(p)
	}

	ctx := p.Context
	if ctx < 0 {
		ctx = 0
	}
	if ctx > 5 {
		ctx = 5
	}
	text, err := t.store.ReadChunkContext(p.DocSlug, p.ChunkID, ctx)
	if err != nil {
		return "", err
	}
	return text, nil
}

// readSection resolves the section chunk for a given chunk and returns its full content.
func (t *Tool) readSection(p knowledgeArgs) (string, error) {
	index, err := t.store.ReadChunksIndex(p.DocSlug)
	if err != nil || index == nil {
		return "", fmt.Errorf("no index found for document %q", p.DocSlug)
	}
	for _, entry := range index.Chunks {
		if entry.ID == p.ChunkID && entry.SectionChunkID != "" {
			text, err := t.store.ReadSectionChunk(p.DocSlug, entry.SectionChunkID)
			if err != nil {
				return "", err
			}
			return text, nil
		}
	}
	// Fallback: no section chunk found, return the chunk itself.
	text, err := t.store.ReadChunk(p.DocSlug, p.ChunkID)
	if err != nil {
		return "", err
	}
	return text, nil
}

func (t *Tool) list() (string, error) {
	docs, err := t.store.List()
	if err != nil {
		return "", err
	}
	if len(docs) == 0 {
		return "Knowledge base is empty.", nil
	}
	data, err := json.MarshalIndent(docs, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal docs: %w", err)
	}
	return string(data), nil
}

func (t *Tool) upload(p knowledgeArgs) (string, error) {
	// Directory batch upload.
	if p.Directory != "" {
		if p.FilePath != "" {
			return "", fmt.Errorf("filePath and directory are mutually exclusive")
		}
		summary, err := t.store.UploadDirectory(p.Directory, p.Recursive)
		if err != nil {
			return "", fmt.Errorf("batch upload failed: %w", err)
		}
		return summary, nil
	}
	// Single file upload.
	if p.FilePath == "" {
		return "", fmt.Errorf("filePath or directory is required for upload")
	}
	meta, err := t.store.UploadDocument(p.FilePath)
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}
	return fmt.Sprintf("Document uploaded: %s (%d chunks, %d chars)",
		meta.OriginalName, meta.ChunkCount, meta.TotalChars), nil
}

func (t *Tool) remove(p knowledgeArgs) (string, error) {
	if p.DocSlug == "" {
		return "", fmt.Errorf("docSlug is required for remove")
	}
	if err := t.store.RemoveDocument(p.DocSlug); err != nil {
		return "", fmt.Errorf("remove failed: %w", err)
	}
	return fmt.Sprintf("Document %q removed.", p.DocSlug), nil
}
