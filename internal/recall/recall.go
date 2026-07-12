// Package recall provides a unified search tool that queries both the memory
// store and the knowledge base in a single call, returning results annotated
// with their source so the model can disambiguate and choose the right follow-up
// (memory read vs knowledge read).
package recall

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"reasonix/internal/knowledge"
	"reasonix/internal/memory"
	"reasonix/internal/tool"
)

const (
	defaultLimit = 8
	maxLimit     = 20
)

// Tool implements tool.Tool for unified memory + knowledge search.
type Tool struct {
	memStore memory.Store
	kbStore  *knowledge.Store
}

// NewTool returns a recall Tool backed by both stores.
func NewTool(memStore memory.Store, kbStore *knowledge.Store) tool.Tool {
	return &Tool{memStore: memStore, kbStore: kbStore}
}

func (t *Tool) Name() string { return "recall" }

func (t *Tool) Description() string {
	return "Search both saved memories and the knowledge base in one call. " +
		"Use this as the FIRST retrieval step when the user asks a question " +
		"that might be answered by either source — it returns results from both, " +
		"each labelled with its source (memory or knowledge), so you can decide " +
		"which to follow up on. Use operation='search' to run a combined hybrid " +
		"(BM25 + dense) search; use operation='list' to see available memories and documents."
}

func (t *Tool) ReadOnly() bool { return true }

// Schema returns the JSON Schema for the tool's parameters.
func (t *Tool) Schema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "operation": {
      "type": "string",
      "enum": ["search", "list"],
      "description": "search = combined hybrid (BM25 + dense) search across memories and knowledge base; list = list available memories and documents"
    },
    "query": {
      "type": "string",
      "description": "Search query for operation='search'. Use distinctive keywords."
    },
    "type": {
      "type": "string",
      "enum": ["user", "feedback", "project", "reference"],
      "description": "Optional memory type filter for search or list."
    },
    "limit": {
      "type": "integer",
      "description": "Maximum total results to return. Default 8, max 20."
    }
  },
  "required": ["operation"]
}`)
}

// Execute dispatches on the "operation" field.
func (t *Tool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p recallArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}

	p.Operation = strings.ToLower(strings.TrimSpace(p.Operation))
	switch p.Operation {
	case "search":
		return t.search(ctx, p)
	case "list":
		return t.listSources(p)
	case "":
		return "", fmt.Errorf("operation is required")
	default:
		return "", fmt.Errorf("unknown operation %q, must be search|list", p.Operation)
	}
}

type recallArgs struct {
	Operation string `json:"operation"`
	Query     string `json:"query"`
	Type      string `json:"type"`
	Limit     int    `json:"limit"`
}

// --- search ---

func (t *Tool) search(ctx context.Context, p recallArgs) (string, error) {
	query := strings.TrimSpace(p.Query)
	if query == "" {
		return "", fmt.Errorf("query is required for search")
	}

	limit := p.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	memType, err := recallTypeFilter(p.Type)
	if err != nil {
		return "", err
	}

	// Run both searches concurrently.
	type memResult struct {
		hits []memory.MemoryHit
		err  error
	}
	type kbResult struct {
		hits []knowledge.SearchHit
		err  error
	}

	memCh := make(chan memResult, 1)
	kbCh := make(chan kbResult, 1)

	go func() {
		hits, err := memory.SearchMemories(ctx, t.memStore, query, memType, limit)
		memCh <- memResult{hits: hits, err: err}
	}()

	go func() {
		hits, err := t.kbStore.HybridSearch(query, limit)
		kbCh <- kbResult{hits: hits, err: err}
	}()

	memRes := <-memCh
	kbRes := <-kbCh

	// Build the combined output. We present results grouped by source because
	// BM25 scores from two different collections are not directly comparable
	// (document frequency and average length differ).
	var b strings.Builder
	totalHits := 0

	// Memory results first.
	if memRes.err != nil {
		fmt.Fprintf(&b, "## Memory search error: %v\n\n", memRes.err)
	} else if len(memRes.hits) == 0 {
		b.WriteString("## Memory\nNo saved memories matched.\n\n")
	} else {
		trimmed := memRes.hits
		if len(trimmed) > limit {
			trimmed = trimmed[:limit]
		}
		fmt.Fprintf(&b, "## Memory (%d hit(s))\n", len(trimmed))
		for i, hit := range trimmed {
			m := hit.Memory
			fmt.Fprintf(&b, "%d. score=%.3f source=memory name=%s type=%s title=%s\n   description: %s\n   snippet: %s\n",
				i+1, hit.Score, m.Name, memory.NormalizeType(string(m.Type)), displayTitle(m.Title, m.Name), oneLine(m.Description), hit.Snippet)
		}
		b.WriteString("\n")
		totalHits += len(trimmed)
	}

	// Knowledge results second.
	if kbRes.err != nil {
		fmt.Fprintf(&b, "## Knowledge base search error: %v\n\n", kbRes.err)
	} else if len(kbRes.hits) == 0 {
		b.WriteString("## Knowledge base\nNo matching chunks found.\n\n")
	} else {
		trimmed := kbRes.hits
		if len(trimmed) > limit {
			trimmed = trimmed[:limit]
		}
		fmt.Fprintf(&b, "## Knowledge base (%d hit(s))\n", len(trimmed))
		for i, hit := range trimmed {
			sectionPart := ""
			if hit.Section != "" {
				sectionPart = fmt.Sprintf(" section=%s", hit.Section)
			}
			fmt.Fprintf(&b, "%d. score=%.3f source=knowledge doc=%s chunk=%s%s\n   snippet: %s\n",
				i+1, hit.Score, hit.DocSlug, hit.ChunkID, sectionPart, hit.Snippet)
		}
		b.WriteString("\n")
		totalHits += len(trimmed)
	}

	if totalHits == 0 {
		return fmt.Sprintf("No results found for %s in memory or knowledge base.\n\nTry different keywords, or use the memory tool to list all memories and the knowledge tool to list all documents.", strconvQuote(query)), nil
	}

	b.WriteString("To read a full result: use the memory tool with operation='read' for memory hits, or the knowledge tool with operation='read' for knowledge hits.")
	return strings.TrimSpace(b.String()), nil
}

// --- list ---

func (t *Tool) listSources(p recallArgs) (string, error) {
	memType, err := recallTypeFilter(p.Type)
	if err != nil {
		return "", err
	}
	limit := p.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	var b strings.Builder

	// Memory list.
	if t.memStore.Dir == "" {
		b.WriteString("## Memory\nMemory store is unavailable.\n\n")
	} else {
		memories := memory.FilterMemories(t.memStore.List(), memType)
		if len(memories) == 0 {
			b.WriteString("## Memory\nNo saved memories.\n\n")
		} else {
			if len(memories) > limit {
				memories = memories[:limit]
			}
			fmt.Fprintf(&b, "## Memory (%d of %d total)\n", len(memories), len(t.memStore.List()))
			for _, m := range memories {
				fmt.Fprintf(&b, "- source=memory [%s](%s.md) type=%s — %s\n",
					displayTitle(m.Title, m.Name), m.Name, memory.NormalizeType(string(m.Type)), oneLine(m.Description))
			}
			b.WriteString("\n")
		}
	}

	// Knowledge list.
	docs, err := t.kbStore.List()
	if err != nil {
		fmt.Fprintf(&b, "## Knowledge base\nError listing documents: %v\n", err)
	} else if len(docs) == 0 {
		b.WriteString("## Knowledge base\nKnowledge base is empty.\n")
	} else {
		fmt.Fprintf(&b, "## Knowledge base (%d document(s))\n", len(docs))
		for _, d := range docs {
			fmt.Fprintf(&b, "- source=knowledge doc=%s type=%s chunks=%d chars=%d\n",
				d.OriginalName, d.SourceType, d.ChunkCount, d.TotalChars)
		}
	}

	return strings.TrimSpace(b.String()), nil
}

// --- helpers ---

func recallTypeFilter(s string) (memory.Type, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	// memory.NormalizeType maps anything to a valid type, but we want strict validation here.
	t := memory.Type(strings.ToLower(s))
	switch t {
	case memory.TypeUser, memory.TypeFeedback, memory.TypeProject, memory.TypeReference:
		return t, nil
	default:
		return "", fmt.Errorf("type must be one of user, feedback, project, reference")
	}
}

func displayTitle(title, name string) string {
	if title != "" {
		return title
	}
	return name
}

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexAny(s, "\n\r"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// sortedByScore is a generic sorter for unified result display.
type sortedByScore[T any] struct {
	items []T
	score func(T) float64
}

func (s sortedByScore[T]) Len() int           { return len(s.items) }
func (s sortedByScore[T]) Less(i, j int) bool { return s.score(s.items[i]) > s.score(s.items[j]) }
func (s sortedByScore[T]) Swap(i, j int)      { s.items[i], s.items[j] = s.items[j], s.items[i] }

// sortByScore sorts items in descending order by score.
func sortByScore[T any](items []T, scoreFn func(T) float64) {
	sort.Sort(sortedByScore[T]{items: items, score: scoreFn})
}
