package symbol

import "strings"

// ExpandImpactFrontier expands a memory_impact "in"/impactors frontier with the bare
// symbol suffix of any qualified "file::symbol" entries, so GetImpactorsByTargets's
// target_node = ANY($2) also matches bare-stored calls-edge targets. calls-edge targets
// are stored as bare identifiers (e.g. "checkAccess"), not "file::symbol" (see
// internal/graph/*_extractor.go extractCalls), so a qualified-only frontier never matches
// a bare-stored calls target without this expansion. Dedups the result. Shared by
// internal/mcp (memory_impact tool) and internal/server/handlers (POST
// /api/v1/graph/impact) so both surfaces resolve calls-edge callers identically.
//
// NOTE: this intentionally matches the bare symbol name workspace-wide, not scoped to the
// file of the qualified node — same-named symbols in other files/repos can produce
// false-positive callers here. This mirrors the existing ambiguity in memory_trace's
// ResolveSymbolByName round trip (both mark/allow ambiguous matches); root-cause C
// (bare-name collision, #542 F2) fixes this centrally for both call sites in a later
// phase. Deliberately not scoped per-repo here (Phase 2 Gate 1.7 decision G1).
func ExpandImpactFrontier(nodes []string) []string {
	expanded := make([]string, 0, len(nodes))
	seen := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		if !seen[n] {
			seen[n] = true
			expanded = append(expanded, n)
		}
		idx := strings.Index(n, "::")
		if idx < 0 {
			continue
		}
		bare := n[idx+2:]
		if bare == "" || seen[bare] {
			continue
		}
		seen[bare] = true
		expanded = append(expanded, bare)
	}
	return expanded
}
