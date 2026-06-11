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

	type contentEntry struct {
		result *Result
		path   string
	}
	byContentHash := make(map[string]*contentEntry)
	var out []Result

	for _, docID := range docOrder {
		r := byDocID[docID]
		hash := contentHash(r.Content)
		normPath := normalizePath(r.SourcePath)

		if existing, ok := byContentHash[hash]; ok {
			if len(normPath) < len(existing.path) {
				byContentHash[hash] = &contentEntry{result: r, path: normPath}
				for i, o := range out {
					if o.DocumentID == existing.result.DocumentID {
						out = append(out[:i], out[i+1:]...)
						break
					}
				}
				out = append(out, *r)
			}
		} else {
			byContentHash[hash] = &contentEntry{result: r, path: normPath}
			out = append(out, *r)
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
