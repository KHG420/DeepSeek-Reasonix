package knowledge

import (
	"fmt"
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
			// Corrupt index — skip this document.
			continue
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
