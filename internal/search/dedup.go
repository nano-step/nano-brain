package search

import (
	"crypto/sha256"
	"path"
	"strings"
)

// DeduplicateResults removes duplicate search results.
// Two levels: (1) same DocumentID → keep highest-scored chunk, (2) same content hash
// after path normalization → keep shorter path. Returns a new slice.
func DeduplicateResults(results []Result) []Result {
	if len(results) <= 1 {
		return results
	}

	// Level 1: Dedup by DocumentID — keep highest-scored chunk per doc.
	byDocID := make(map[string]*Result, len(results))
	var docOrder []string
	for i := range results {
		r := &results[i]
		if existing, ok := byDocID[r.DocumentID]; ok {
			if r.Score > existing.Score {
				byDocID[r.DocumentID] = r
			}
		} else {
			byDocID[r.DocumentID] = r
			docOrder = append(docOrder, r.DocumentID)
		}
	}

	// Level 2: Dedup by content hash — keep shorter path per content group.
	type contentEntry struct {
		docID    string
		normPath string
	}
	bestContent := make(map[string]contentEntry, len(docOrder))
	for _, docID := range docOrder {
		r := byDocID[docID]
		hash := contentHash(r.Content)
		normPath := normalizePath(r.SourcePath)

		if existing, ok := bestContent[hash]; ok {
			if len(normPath) < len(existing.normPath) {
				bestContent[hash] = contentEntry{docID: docID, normPath: normPath}
			}
		} else {
			bestContent[hash] = contentEntry{docID: docID, normPath: normPath}
		}
	}

	keepDocIDs := make(map[string]bool, len(bestContent))
	for _, entry := range bestContent {
		keepDocIDs[entry.docID] = true
	}

	// Single pass to build output preserving original order.
	out := make([]Result, 0, len(keepDocIDs))
	for _, docID := range docOrder {
		if keepDocIDs[docID] {
			out = append(out, *byDocID[docID])
		}
	}

	return out
}

func contentHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return string(h[:16])
}

func normalizePath(p string) string {
	p = strings.ToLower(p)
	p = path.Clean(p)
	p = strings.ReplaceAll(p, ".agents/", ".agent/")
	return p
}
