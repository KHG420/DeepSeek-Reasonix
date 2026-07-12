package knowledge

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"reasonix/internal/retrieval"
)

const rrfK = 60.0 // RRF constant for hybrid search fusion

// searchEntry is a unified representation of one chunk during scoring. When the
// index path is used, text is empty until snippet generation; when the fallback
// path is used, text is populated from the chunk file and tokens are computed
// on the fly.
type searchEntry struct {
	docSlug     string
	chunkID     string
	text        string         // chunk content (only populated in fallback or for snippet)
	terms       map[string]int // term frequencies (from index or computed)
	termLen     int            // total token count
	section     string         // from CHUNKS.toml metadata
	offset      int            // from CHUNKS.toml metadata
	sourceType  string         // from meta.json
	vector      []float64      // dense embedding vector (from CHUNKS.toml, if available)
	sectionRole string         // classified section role (C2), e.g. "abstract", "introduction"
}

// collectEntries gathers all search entries from the knowledge base,
// applying the given filter. It reads from the pre-computed CHUNKS.toml
// index when available and falls back to scanning chunk files otherwise.
func (s *Store) collectEntries(filter SearchFilter) ([]searchEntry, error) {
	kd := s.knowledgeDir()
	docDirs, err := listDocDirs(kd)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}

	var entries []searchEntry

	for _, slug := range docDirs {
		// Filter by doc slug.
		if filter.DocSlug != "" && slug != filter.DocSlug {
			continue
		}

		// Read meta if we need source type for filtering.
		if filter.SourceType != "" {
			meta, metaErr := s.ReadMeta(slug)
			if metaErr != nil {
				continue
			}
			if meta.SourceType != filter.SourceType {
				continue
			}
		}

		index, idxErr := s.ReadChunksIndex(slug)
		if idxErr != nil {
			// Corrupt index — skip this document.
			continue
		}
		if index != nil {
			// Index path: use pre-computed term frequencies.
			for _, e := range index.Chunks {
				// Filter by section.
				if filter.Section != "" && !strings.Contains(e.Section, filter.Section) {
					continue
				}
				srcType := ""
				if filter.SourceType != "" {
					srcType = filter.SourceType // already resolved above
				}
				entries = append(entries, searchEntry{
					docSlug:     slug,
					chunkID:     e.ID,
					terms:       e.Terms,
					termLen:     e.TermCount,
					section:     e.Section,
					offset:      e.Offset,
					sourceType:  srcType,
					vector:      e.Vector,
					sectionRole: e.SectionRole,
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
					docSlug:    slug,
					chunkID:    id,
					text:       text,
					terms:      retrieval.Counts(tokens),
					termLen:    len(tokens),
					sourceType: resolveSourceType(s, slug, filter),
				})
			}
		}
	}

	return entries, nil
}

// Search runs a BM25 query across all chunks in the knowledge base and returns
// ranked hits. It first tries the per-document CHUNKS.toml index for
// pre-computed term frequencies; documents without an index fall back to
// reading and tokenising every chunk file.
//
// An optional SearchFilter can be passed to narrow results by doc slug, source
// type, or section. When no filter is passed, all documents are searched.
//
// The limit caps the number of results; hits below 15% of the top score are
// trimmed via retrieval.KeepTopRelativeScore. Only the top-K results (after
// trimming and capping) have their chunk files read for snippet generation.
func (s *Store) Search(query string, limit int, filters ...SearchFilter) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 8
	}

	// Query rewriting: if a QueryRewriter is configured, expand the query
	// with synonyms and merge all variants into a single set of unique terms.
	rewritten := s.rewrittenQueries(query)

	queryTerms, err := retrieval.QueryTerms(rewritten)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	// Resolve filter.
	var filter SearchFilter
	if len(filters) > 0 {
		filter = filters[0]
	}

	entries, err := s.collectEntries(filter)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
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
		score := retrieval.BM25Score(docs[i], lengths[i], queryTerms, df, len(entries), avgLen)
		if score > 0 {
			results = append(results, ranked{entry: e, score: score})
		}
	}

	// Phase 3.5: abstract score boost. Chunks from the "abstract" section
	// receive a modest boost (×1.1) because abstracts condense the core
	// contribution of a paper.
	for i := range results {
		if results[i].entry.sectionRole == "abstract" {
			results[i].score *= 1.1
		}
	}

	// Phase 4: sort descending by score.
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// Phase 5: trim low-scoring noise.
	results = retrieval.KeepTopRelativeScore(results, 0.15, func(r ranked) float64 {
		return r.score
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
			Score:       r.score,
			DocSlug:     r.entry.docSlug,
			ChunkID:     r.entry.chunkID,
			Snippet:     retrieval.MakeSnippet(r.entry.text, query, queryTerms, 200),
			Section:     r.entry.section,
			Offset:      r.entry.offset,
			SectionRole: r.entry.sectionRole,
		}
	}
	return hits, nil
}

// HybridSearch runs a combined BM25 + dense embedding search using Reciprocal
// Rank Fusion (RRF). It searches all chunks, computes both BM25 scores and
// cosine similarity with the query embedding, then fuses the rankings.
//
// Documents without vectors (no embedder configured at upload time) are scored
// with BM25 only. The method falls back to pure BM25 when the store has no
// embedder set.
//
// An optional SearchFilter can be passed to narrow results by doc slug, source
// type, or section.
//
// The limit caps the number of results; hits below 15% of the top BM25 score
// are trimmed before fusion.
func (s *Store) HybridSearch(query string, limit int, filters ...SearchFilter) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 8
	}

	// Query rewriting.
	rewritten := s.rewrittenQueries(query)

	queryTerms, err := retrieval.QueryTerms(rewritten)
	if err != nil {
		return nil, fmt.Errorf("hybrid search: %w", err)
	}

	var filter SearchFilter
	if len(filters) > 0 {
		filter = filters[0]
	}

	entries, err := s.collectEntries(filter)
	if err != nil {
		return nil, fmt.Errorf("hybrid search: %w", err)
	}
	if len(entries) == 0 {
		return nil, nil
	}

	// Check if any entry has a vector.
	hasVectors := false
	for _, e := range entries {
		if len(e.vector) > 0 {
			hasVectors = true
			break
		}
	}

	// Phase 2: BM25 scoring.
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

	type hybridRanked struct {
		entry     searchEntry
		bm25Score float64
		cosScore  float64
		rrfScore  float64
	}
	scored := make([]hybridRanked, len(entries))
	for i, e := range entries {
		scored[i] = hybridRanked{
			entry:     e,
			bm25Score: retrieval.BM25Score(docs[i], lengths[i], queryTerms, df, len(entries), avgLen),
		}
	}

	// Phase 3: embedding scoring (if vectors are available).
	if hasVectors && s.embedder != nil {
		queryVec, embedErr := s.embedder.Embed(nil, []string{query})
		if embedErr == nil && len(queryVec) > 0 && len(queryVec[0]) > 0 {
			qVec64 := make([]float64, len(queryVec[0]))
			for j, v := range queryVec[0] {
				qVec64[j] = float64(v)
			}
			for i := range scored {
				if len(scored[i].entry.vector) > 0 {
					scored[i].cosScore = cosineSimilarity(scored[i].entry.vector, qVec64)
				}
			}
		}
	}

	// Phase 3.5: abstract score boost. Chunks from the "abstract" section
	// receive a modest BM25 boost (×1.1) so the boost is factored into RRF fusion.
	for i := range scored {
		if scored[i].entry.sectionRole == "abstract" {
			scored[i].bm25Score *= 1.1
		}
	}

	// Phase 4: RRF fusion.
	// Sort by BM25 score descending for BM25 rank.
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].bm25Score > scored[j].bm25Score
	})
	for i := range scored {
		if scored[i].bm25Score > 0 {
			scored[i].rrfScore += 1.0 / (rrfK + float64(i))
		}
	}

	// Sort by cosine score descending for dense rank.
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].cosScore > scored[j].cosScore
	})
	for i := range scored {
		if scored[i].cosScore > 0 {
			scored[i].rrfScore += 1.0 / (rrfK + float64(i))
		}
	}

	// Phase 5: sort by final RRF score descending.
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].rrfScore > scored[j].rrfScore
	})

	// Phase 6: keep only entries with non-zero RRF score.
	var results []hybridRanked
	for _, r := range scored {
		if r.rrfScore > 0 {
			results = append(results, r)
		}
	}

	// Phase 7: trim low-scoring noise.
	if len(results) > 0 {
		top := results[0].rrfScore
		cutoff := 0.15
		trimmed := results[:0]
		for i, r := range results {
			if i == 0 || r.rrfScore >= top*cutoff {
				trimmed = append(trimmed, r)
			}
		}
		results = trimmed
	}

	// Phase 8: cap to limit.
	if len(results) > limit {
		results = results[:limit]
	}

	// Phase 8a: reranker (optional). If a Reranker is configured, re-score the
	// top results using a cross-encoder for improved precision. The reranker
	// scores replace the RRF scores.
	if s.reranker != nil && len(results) > 0 {
		rerankCandidates := results
		// Use up to 2× limit, but at least 20 candidates for a meaningful rerank.
		candLimit := limit * 2
		if candLimit < 20 {
			candLimit = 20
		}
		if len(rerankCandidates) > candLimit {
			rerankCandidates = rerankCandidates[:candLimit]
		}

		texts := make([]string, len(rerankCandidates))
		for i, r := range rerankCandidates {
			if r.entry.text == "" {
				text, readErr := s.ReadChunk(r.entry.docSlug, r.entry.chunkID)
				if readErr != nil {
					text = ""
				}
				rerankCandidates[i].entry.text = text
			}
			texts[i] = rerankCandidates[i].entry.text
		}

		rerankScores, rerankErr := s.reranker.Rerank(nil, query, texts)
		if rerankErr == nil && len(rerankScores) == len(rerankCandidates) {
			for i := range rerankCandidates {
				rerankCandidates[i].rrfScore = rerankScores[i]
			}
			// Re-sort by reranker score.
			sort.Slice(rerankCandidates, func(i, j int) bool {
				return rerankCandidates[i].rrfScore > rerankCandidates[j].rrfScore
			})
			// Replace results with reranked order.
			results = rerankCandidates
		}
	}

	// Phase 9: read chunk text for index-path results.
	for i := range results {
		if results[i].entry.text == "" {
			text, readErr := s.ReadChunk(results[i].entry.docSlug, results[i].entry.chunkID)
			if readErr != nil {
				text = ""
			}
			results[i].entry.text = text
		}
	}

	// Phase 10: convert to SearchHit slice.
	hits := make([]SearchHit, len(results))
	for i, r := range results {
		hits[i] = SearchHit{
			Score:       r.rrfScore,
			DocSlug:     r.entry.docSlug,
			ChunkID:     r.entry.chunkID,
			Snippet:     retrieval.MakeSnippet(r.entry.text, query, queryTerms, 200),
			Section:     r.entry.section,
			Offset:      r.entry.offset,
			SectionRole: r.entry.sectionRole,
		}
	}
	return hits, nil
}

// resolveSourceType returns the source type for a slug, but only reads meta
// when the filter actually needs it (to avoid unnecessary I/O in the common
// no-filter path). Returns empty string when not needed.
func resolveSourceType(s *Store, slug string, filter SearchFilter) string {
	if filter.SourceType == "" {
		return "" // not needed by any filter
	}
	meta, err := s.ReadMeta(slug)
	if err != nil {
		return ""
	}
	return meta.SourceType
}

// rewrittenQueries applies the configured QueryRewriter and merges all
// rewritten query variants into a single query string for tokenisation.
// When no rewriter is configured, the original query is returned as-is.
func (s *Store) rewrittenQueries(query string) string {
	if s.rewriter == nil {
		return query
	}
	variants := s.rewriter.Rewrite(query)
	if len(variants) == 0 {
		return query
	}
	// Merge all variants into a single query string. Duplicate terms are
	// removed during tokenisation by retrieval.QueryTerms → retrieval.Unique.
	return strings.Join(variants, " ")
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
