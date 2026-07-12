package knowledge

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"

	"reasonix/internal/retrieval"
)

// searchEntry is a unified representation of one chunk during scoring. When the
// index path is used, text is empty until snippet generation; when the fallback
// path is used, text is populated from the chunk file and tokens are computed
// on the fly.
type searchEntry struct {
	docSlug  string
	chunkID  string
	text     string         // chunk content (only populated in fallback or for snippet)
	terms    map[string]int // term frequencies (from index or computed)
	termLen  int            // total token count
	section  string         // from CHUNKS.toml metadata
	offset   int            // from CHUNKS.toml metadata
}

// Search runs a BM25 query across all chunks in the knowledge base and returns
// ranked hits. It first tries the per-document CHUNKS.toml index for
// pre-computed term frequencies; documents without an index fall back to
// reading and tokenising every chunk file.
//
// The limit caps the number of results; hits below 15% of the top score are
// trimmed via retrieval.KeepTopRelativeScore. Only the top-K results (after
// trimming and capping) have their chunk files read for snippet generation.
func (s *Store) Search(query string, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 8
	}

	queryTerms, err := retrieval.QueryTerms(query)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	// Phase 1: collect entries from all documents (index path or fallback).
	var entries []searchEntry

	kd := s.knowledgeDir()
	docDirs, err := listDocDirs(kd)
	if err != nil {
		return nil, fmt.Errorf("search: list documents: %w", err)
	}

	for _, slug := range docDirs {
		index, idxErr := s.ReadChunksIndex(slug)
		if idxErr != nil {
			slog.Warn("knowledge: corrupt CHUNKS.toml, falling back to full scan",
				"slug", slug, "err", idxErr)
		}
		if index != nil {
			// Index path: use pre-computed term frequencies.
			for _, e := range index.Chunks {
				entries = append(entries, searchEntry{
					docSlug: slug,
					chunkID: e.ID,
					terms:   e.Terms,
					termLen: e.TermCount,
					section: e.Section,
					offset:  e.Offset,
				})
			}
		} else {
			// Fallback: read and tokenise each chunk file.
			ids, listErr := s.ListChunks(slug)
			if listErr != nil {
				continue
			}
			for _, id := range ids {
				text, readErr := s.ReadChunk(slug, id)
				if readErr != nil {
					continue
				}
				tokens := retrieval.Tokens(text)
				entries = append(entries, searchEntry{
					docSlug: slug,
					chunkID: id,
					text:    text,
					terms:   retrieval.Counts(tokens),
					termLen: len(tokens),
				})
			}
		}
	}

	if len(entries) == 0 {
		return nil, nil
	}

	// Phase 2: build document-frequency map and compute average length.
	docs := make([]map[string]int, len(entries))
	lengths := make([]int, len(entries))
	var totalLen int
	for i, e := range entries {
		docs[i] = e.terms
		lengths[i] = e.termLen
		totalLen += e.termLen
	}
	df := retrieval.DocumentFrequency(docs)
	avgLen := float64(totalLen) / float64(len(entries))

	// Phase 3: score each entry with BM25.
	type ranked struct {
		entry searchEntry
		score float64
	}
	var results []ranked
	for i, e := range entries {
		s := retrieval.BM25Score(docs[i], lengths[i], queryTerms, df, len(entries), avgLen)
		if s > 0 {
			results = append(results, ranked{entry: e, score: s})
		}
	}

	// Phase 4: sort descending by score.
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// Phase 5: trim low-scoring noise.
	results = retrieval.KeepTopRelativeScore(results, 0.15, func(s ranked) float64 {
		return s.score
	})

	// Phase 6: cap to limit.
	if len(results) > limit {
		results = results[:limit]
	}

	// Phase 7: for index-path results that lack chunk text, read the file for
	// snippet generation. Fallback entries already have text populated.
	for i := range results {
		if results[i].entry.text == "" {
			text, readErr := s.ReadChunk(results[i].entry.docSlug, results[i].entry.chunkID)
			if readErr != nil {
				text = "" // snippet will reflect the failure
			}
			results[i].entry.text = text
		}
	}

	// Phase 8: convert to SearchHit slice with section/offset metadata.
	hits := make([]SearchHit, len(results))
	for i, r := range results {
		hits[i] = SearchHit{
			Score:   r.score,
			DocSlug: r.entry.docSlug,
			ChunkID: r.entry.chunkID,
			Snippet: retrieval.MakeSnippet(r.entry.text, query, queryTerms, 200),
			Section: r.entry.section,
			Offset:  r.entry.offset,
		}
	}
	return hits, nil
}

// listDocDirs returns the names of all document subdirectories under the
// knowledge directory.
func listDocDirs(kd string) ([]string, error) {
	entries, err := readDirNames(kd)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, name := range entries {
		if name == "INDEX.md" {
			continue
		}
		dirs = append(dirs, name)
	}
	return dirs, nil
}

// readDirNames is a thin wrapper around os.ReadDir that returns entry names.
func readDirNames(dir string) ([]string, error) {
	des, err := readDirSafe(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(des))
	for i, d := range des {
		names[i] = d.Name()
	}
	return names, nil
}

// readDirSafe is os.ReadDir but returns nil slice for non-existent dirs.
func readDirSafe(dir string) ([]os.DirEntry, error) {
	des, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return des, err
}

// HybridSearch runs an embedding-based search over the knowledge base, then
// reranks the top candidates with BM25. It returns results only if an embedder
// is configured; otherwise it falls back to BM25-only Search.
//
// The embedding stage retrieves limit×5 candidates (max 100) by cosine
// similarity, then the BM25 stage reranks and returns the top limit results.
func (s *Store) HybridSearch(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	if s.embedder == nil {
		return s.Search(query, limit)
	}
	if limit <= 0 {
		limit = 8
	}

	queryTerms, err := retrieval.QueryTerms(query)
	if err != nil {
		return nil, fmt.Errorf("hybrid search: %w", err)
	}

	// Phase 1: embed the query.
	queryVec, err := s.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("hybrid search: embed query: %w", err)
	}

	// Phase 2: scan all documents, collect embedding candidates.
	type cand struct {
		docSlug string
		chunkID string
		sim     float64 // cosine similarity
	}
	var candidates []cand

	kd := s.knowledgeDir()
	docDirs, err := listDocDirs(kd)
	if err != nil {
		return nil, fmt.Errorf("hybrid search: list documents: %w", err)
	}

	for _, slug := range docDirs {
		ids, listErr := s.ListEmbeddingIDs(slug)
		if listErr != nil || len(ids) == 0 {
			continue
		}
		for _, id := range ids {
			vec, readErr := s.ReadEmbedding(slug, id)
			if readErr != nil {
				continue
			}
			sim := cosineSimilarity(queryVec, vec)
			if sim > 0 {
				candidates = append(candidates, cand{
					docSlug: slug,
					chunkID: id,
					sim:     sim,
				})
			}
		}
	}

	if len(candidates) == 0 {
		// No embedding candidates — fall back to BM25.
		return s.Search(query, limit)
	}

	// Phase 3: sort by similarity descending and take top-K.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].sim > candidates[j].sim
	})
	k := limit * 5
	if k > 100 {
		k = 100
	}
	if k > len(candidates) {
		k = len(candidates)
	}
	candidates = candidates[:k]

	// Phase 4: build BM25 entries for candidates only.
	entries := make([]searchEntry, len(candidates))
	for i, c := range candidates {
		entries[i] = searchEntry{
			docSlug: c.docSlug,
			chunkID: c.chunkID,
		}
		// Load term frequencies from index (preferred).
		index, idxErr := s.ReadChunksIndex(c.docSlug)
		if idxErr == nil && index != nil {
			for _, e := range index.Chunks {
				if e.ID == c.chunkID {
					entries[i].terms = e.Terms
					entries[i].termLen = e.TermCount
					entries[i].section = e.Section
					entries[i].offset = e.Offset
					break
				}
			}
		}
		if entries[i].terms == nil {
			// Fallback: read and tokenise.
			text, readErr := s.ReadChunk(c.docSlug, c.chunkID)
			if readErr != nil {
				continue
			}
			tokens := retrieval.Tokens(text)
			entries[i].text = text
			entries[i].terms = retrieval.Counts(tokens)
			entries[i].termLen = len(tokens)
		}
	}

	// Phase 5: BM25 rerank.
	docs := make([]map[string]int, len(entries))
	lengths := make([]int, len(entries))
	var totalLen int
	for i, e := range entries {
		if e.terms == nil {
			continue
		}
		docs[i] = e.terms
		lengths[i] = e.termLen
		totalLen += e.termLen
	}
	if totalLen == 0 {
		return s.Search(query, limit)
	}
	df := retrieval.DocumentFrequency(docs)
	avgLen := float64(totalLen) / float64(len(entries))

	type ranked struct {
		entry searchEntry
		score float64
	}
	var results []ranked
	for i, e := range entries {
		if e.terms == nil {
			continue
		}
		s := retrieval.BM25Score(docs[i], lengths[i], queryTerms, df, len(entries), avgLen)
		if s > 0 {
			results = append(results, ranked{entry: e, score: s})
		}
	}

	// Phase 6: sort by BM25 score.
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})
	if len(results) > limit {
		results = results[:limit]
	}

	// Phase 7: fill in text for snippet.
	for i := range results {
		if results[i].entry.text == "" {
			text, readErr := s.ReadChunk(results[i].entry.docSlug, results[i].entry.chunkID)
			if readErr != nil {
				text = ""
			}
			results[i].entry.text = text
		}
	}

	// Phase 8: build SearchHit slice.
	hits := make([]SearchHit, len(results))
	for i, r := range results {
		hits[i] = SearchHit{
			Score:   r.score,
			DocSlug: r.entry.docSlug,
			ChunkID: r.entry.chunkID,
			Snippet: retrieval.MakeSnippet(r.entry.text, query, queryTerms, 200),
			Section: r.entry.section,
			Offset:  r.entry.offset,
		}
	}
	if len(hits) == 0 {
		return s.Search(query, limit)
	}
	return hits, nil
}
