package knowledge

import (
	"fmt"
	"os"
	"sort"

	"reasonix/internal/retrieval"
)

// Search runs a BM25 query across all chunks in the knowledge base and returns
// ranked hits. The limit caps the number of results; hits below 15% of the top
// score are trimmed via retrieval.KeepTopRelativeScore.
func (s *Store) Search(query string, limit int) ([]SearchHit, error) {
	if limit <= 0 {
		limit = 8
	}

	queryTerms, err := retrieval.QueryTerms(query)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	// Step 1: collect all chunks from all documents.
	type chunkEntry struct {
		docSlug string
		chunkID string
		text    string
	}
	var entries []chunkEntry

	kd := s.knowledgeDir()
	docDirs, err := listDocDirs(kd)
	if err != nil {
		return nil, fmt.Errorf("search: list documents: %w", err)
	}

	for _, slug := range docDirs {
		ids, err := s.ListChunks(slug)
		if err != nil {
			continue // skip unreadable docs
		}
		for _, id := range ids {
			text, err := s.ReadChunk(slug, id)
			if err != nil {
				continue
			}
			entries = append(entries, chunkEntry{docSlug: slug, chunkID: id, text: text})
		}
	}

	if len(entries) == 0 {
		return nil, nil
	}

	// Step 2: build document-frequency and tokenise each chunk.
	docs := make([]map[string]int, len(entries))
	lengths := make([]int, len(entries))
	var totalLen int
	for i, e := range entries {
		tokens := retrieval.Tokens(e.text)
		docs[i] = retrieval.Counts(tokens)
		lengths[i] = len(tokens)
		totalLen += lengths[i]
	}
	df := retrieval.DocumentFrequency(docs)
	avgLen := float64(totalLen) / float64(len(entries))

	// Step 3: score each chunk with BM25.
	type ranked struct {
		entry chunkEntry
		score float64
	}
	var results []ranked
	for i, e := range entries {
		s := retrieval.BM25Score(docs[i], lengths[i], queryTerms, df, len(entries), avgLen)
		if s > 0 {
			results = append(results, ranked{entry: e, score: s})
		}
	}

	// Step 4: sort descending by score.
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	// Step 5: trim low-scoring noise.
	results = retrieval.KeepTopRelativeScore(results, 0.15, func(s ranked) float64 {
		return s.score
	})

	// Step 6: cap and convert to SearchHit.
	if len(results) > limit {
		results = results[:limit]
	}

	hits := make([]SearchHit, len(results))
	for i, s := range results {
		hits[i] = SearchHit{
			Score:   s.score,
			DocSlug: s.entry.docSlug,
			ChunkID: s.entry.chunkID,
			Snippet: retrieval.MakeSnippet(s.entry.text, query, queryTerms, 200),
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
