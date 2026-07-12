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
		"Use operation='search' to find relevant chunks by BM25 text retrieval; " +
		"'read' to read a specific chunk; 'list' to see all uploaded documents; " +
		"'upload' to ingest a document file; 'remove' to delete a document and all its chunks."
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
    "filePath": {
      "type": "string",
      "description": "Absolute or workspace-relative path to the document file for operation='upload'."
    },
    "limit": {
      "type": "integer",
      "description": "Maximum search results to return. Default 8, max 20."
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
	Operation string `json:"operation"`
	Query     string `json:"query"`
	DocSlug   string `json:"docSlug"`
	ChunkID   string `json:"chunkID"`
	FilePath  string `json:"filePath"`
	Limit     int    `json:"limit"`
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
	hits, err := t.store.Search(p.Query, limit)
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
	if p.FilePath == "" {
		return "", fmt.Errorf("filePath is required for upload")
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
