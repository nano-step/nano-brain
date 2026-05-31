# Extend Code Intelligence — Design

## Architecture

No new packages. All work extends existing `internal/graph/` and `cmd/nano-brain/`.

```
internal/graph/
├── edge.go                    # Edge, Extractor interface (unchanged)
├── registry.go                # Registry (unchanged)
├── go_extractor.go            # existing
├── go_extractor_test.go       # existing
├── typescript_extractor.go    # NEW
├── typescript_extractor_test.go # NEW
├── javascript_extractor.go    # NEW
├── javascript_extractor_test.go # NEW
├── python_extractor.go        # NEW
└── python_extractor_test.go   # NEW

cmd/nano-brain/
├── cmd_context.go             # NEW — `nano-brain context <symbol>`
├── cmd_code_impact.go         # NEW — `nano-brain code-impact <symbol>`
└── cmd_detect_changes.go      # NEW — `nano-brain detect-changes`
```

## Part 1: Graph Extractors

### Interface Contract (existing, unchanged)

```go
type Extractor interface {
    ExtractEdges(filePath string, content []byte) ([]Edge, error)
    Supports(ext string) bool
}
```

### TypeScript Extractor

**Supports**: `.ts`, `.tsx`

**Grammar**: `grammars.TypescriptLanguage()` for `.ts`, `grammars.TsxLanguage()` for `.tsx` (dual grammar, matching `internal/symbol/typescript_extractor.go:28-33` pattern).

**Tree-sitter queries:**

```
# imports — ES imports only (single pattern, strip quotes in Go code via strings.Trim)
(import_statement source: (string) @path)

# contains — exported/top-level declarations
(function_declaration name: (identifier) @name) @decl
(class_declaration name: (type_identifier) @name) @decl
(interface_declaration name: (type_identifier) @name) @decl
(type_alias_declaration name: (type_identifier) @name) @decl
(enum_declaration name: (identifier) @name) @decl
(lexical_declaration (variable_declarator name: (identifier) @name)) @decl

# calls — TWO-PHASE (matching go_extractor.go pattern)
# Phase 1: find function/method bodies to get enclosing function name + byte range
(function_declaration name: (identifier) @fn_name body: (statement_block) @body) @fn_decl
(method_definition name: (property_identifier) @fn_name body: (statement_block) @body) @fn_decl
# Phase 2: find call expressions, match to enclosing function via byte range
(call_expression function: (identifier) @callee)
(call_expression function: (member_expression property: (property_identifier) @callee))
```

**require() handling**: Detected separately — match `(call_expression function: (identifier) @fn ...)`, then post-filter in Go code: `if bt.NodeText(fnNode) == "require"`, extract the string argument as an import edge. Do NOT use a blanket query matching all calls with string args.

**Known limitation**: Arrow functions (`const foo = () => {}`) won't have enclosing-function attribution in two-phase calls. Accepted — call edges still emit, attributed to file level.

### JavaScript Extractor

**Supports**: `.js`, `.jsx`

**Grammar**: `grammars.JavascriptLanguage()` (lowercase 's'). Single grammar — no dual-lang needed (unlike TS/TSX).

Key difference from TS: no type annotations, no interface/enum/type_alias nodes.

```
# imports — ES only (strip quotes in Go code)
(import_statement source: (string) @path)

# contains
(function_declaration name: (identifier) @name) @decl
(class_declaration name: (identifier) @name) @decl
(lexical_declaration (variable_declarator name: (identifier) @name)) @decl
(export_statement declaration: (function_declaration name: (identifier) @name)) @decl

# calls — TWO-PHASE (same as TypeScript)
# Phase 1: function bodies
(function_declaration name: (identifier) @fn_name body: (statement_block) @body) @fn_decl
(method_definition name: (property_identifier) @fn_name body: (statement_block) @body) @fn_decl
# Phase 2: call expressions
(call_expression function: (identifier) @callee)
(call_expression function: (member_expression property: (property_identifier) @callee))
```

**require() handling**: Same post-filter approach as TypeScript — check `bt.NodeText(fnNode) == "require"` before emitting import edge.

### Python Extractor

**Supports**: `.py`

**Grammar**: `grammars.PythonLanguage()`.

```
# imports
(import_statement name: (dotted_name) @path)
(import_from_statement module_name: (dotted_name) @path)

# contains
(function_definition name: (identifier) @name) @decl
(class_definition name: (identifier) @name) @decl

# calls — TWO-PHASE
# Phase 1: function bodies (Python uses `block` not `statement_block`)
(function_definition name: (identifier) @fn_name body: (block) @body) @fn_decl
# Phase 2: call expressions (Python uses `call` not `call_expression`)
(call function: (identifier) @callee)
(call function: (attribute attribute: (identifier) @callee))
```

**Module-level assignments**: `(assignment left: (identifier) @name) @decl` — MUST filter in Go code to only match assignments whose parent node type is `module`. Do NOT emit contains edges for nested assignments (e.g., `x = 5` inside a function).

### Registration (cmd/nano-brain/main.go)

```go
// Existing:
goGraph, _ := graph.NewGoGraphExtractor()

// Add (note: lowercase 's' in function names, matching grammar naming):
tsGraph, err := graph.NewTypeScriptGraphExtractor()
if err != nil {
    log.Warn().Err(err).Msg("TypeScript graph extractor unavailable")
}
jsGraph, err := graph.NewJavaScriptGraphExtractor()
if err != nil {
    log.Warn().Err(err).Msg("JavaScript graph extractor unavailable")
}
pyGraph, err := graph.NewPythonGraphExtractor()
if err != nil {
    log.Warn().Err(err).Msg("Python graph extractor unavailable")
}

graphRegistry := graph.NewRegistry(goGraph, tsGraph, jsGraph, pyGraph)
```

### Storage

No changes — all extractors write to existing `graph_edges` table via `UpsertGraphEdge`.
Edge format is language-agnostic: `(workspace_hash, source_node, target_node, edge_type, source_file, metadata)`.

## Part 2: CLI Commands

### `nano-brain context <symbol>`

**Flow:**
1. Parse args: symbol name, optional `--workspace`, `--depth`
2. Query symbols: `GET /api/v1/symbols?query=<symbol>&workspace=<hash>` → find exact match
3. Query outgoing edges: `POST /api/v1/graph/query` `{"workspace":"<hash>","node":"<symbol>","direction":"out"}`
4. Query incoming edges: `POST /api/v1/graph/query` `{"workspace":"<hash>","node":"<symbol>","direction":"in"}`
5. Format output:

```
Symbol: processFile
  File: internal/watcher/watcher.go:142
  Kind: function
  Language: go

Calls (outgoing):
  → extractSymbols (internal/symbol/registry.go:28)
  → extractEdges (internal/graph/registry.go:16)
  → upsertDocument (internal/storage/sqlc/documents.sql.go:45)

Called by (incoming):
  ← handleFileEvent (internal/watcher/watcher.go:89)
  ← processQueue (internal/watcher/watcher.go:67)

Imports (file):
  → internal/symbol
  → internal/graph
  → internal/storage/sqlc
```

### `nano-brain code-impact <symbol>`

**Flow:**
1. Parse args: symbol, optional `--depth` (default 2), `--edge-type`
2. Call `POST /api/v1/graph/impact` with `{"workspace":"<hash>","node":"<symbol>","max_depth":2}`
   Note: max_depth is clamped to [1,3] by the impact handler.
3. Format output as tree:

```
Impact analysis: processFile (depth=2)

Level 1 (direct dependents):
  ← handleFileEvent (calls)
  ← processQueue (calls)

Level 2 (transitive):
  ← watcher.Start (calls handleFileEvent)
  ← main (calls watcher.Start)

Total: 4 symbols potentially affected
```

### `nano-brain detect-changes`

**Flow:**
1. Parse args: `--staged` (default), `--all`, `--workspace`, `--json`
2. Run `exec.CommandContext` with 10s timeout: `git diff --name-only [--staged]` to get changed files
3. For each changed file, run `git diff [--staged] <file>` → parse unified diff hunk headers (`@@ -start,count +start,count @@`) to extract changed line ranges
4. Query symbols at those lines: match against indexed symbols by file + line range via `GET /api/v1/symbols?file=<file>&workspace=<hash>`
5. For each affected symbol, call `POST /api/v1/graph/impact` with depth=1
6. Format output (text or JSON via `--json`):

**Prerequisites**: `git` must be in PATH, CWD must be inside a git repo. This is the only nano-brain command that shells out to an external process.

```
Changed files: 3
Affected symbols: 7
Impact radius: 12

src/auth/login.ts (modified):
  Δ validateCredentials (line 42-58)
    ← loginHandler (calls)
    ← authMiddleware (calls)
  Δ hashPassword (line 89-95)
    ← createUser (calls)

src/models/user.py (modified):
  Δ User.save (line 120-135)
    ← register_view (calls)
```

## Testing Strategy

### Unit Tests (per extractor)
- Parse known source file → verify expected edges
- Test import extraction (ES import, require, Python import/from)
- Test contains extraction (functions, classes, types)
- Test call extraction (direct calls, method calls)
- Edge cases: empty files, syntax errors (tree-sitter handles gracefully), mixed content

### Integration Tests
- Watcher pipeline: create temp files → trigger watcher → verify graph_edges in DB
- CLI commands: start server → index test files → run CLI → verify output format

## Risks & Mitigations

| Risk | Mitigation |
|---|---|
| Tree-sitter query syntax differs per grammar | Test each query against real-world sample files; all `NewQuery` calls in constructors fail-fast at startup |
| Import path formats vary (relative, absolute, aliased) | Store raw import strings, no resolution (matches Go extractor) |
| Call detection is best-effort | Document limitation — no type-aware resolution. Arrow functions attributed to file level. |
| Binary size increase | Grammars already embedded (TS, JS, Python loaded for symbols) — no additional size |
| require() query matches all calls with string args | Post-filter: check `bt.NodeText(fnNode) == "require"` before emitting import edge |
| Python assignments at any depth create spurious contains edges | Filter: only emit when parent node type is `module` |
| TSX grammar differs from TS | Dual grammar pattern: `TypescriptLanguage()` for `.ts`, `TsxLanguage()` for `.tsx` |
| Graph endpoint doesn't return line numbers | `context` CLI shows file paths from symbol lookup, not graph edges; line numbers come from symbols, not edges |
| Watcher `allowedExtensions` can block new languages | Document in README; log warning at startup if graph extractors exist for unlisted extensions |
| detect-changes: stale symbol index vs current file | Print warning if symbol timestamps predate file mtime; recommend `nano-brain reindex` first |
| detect-changes: git not in PATH | Check `exec.LookPath("git")` first, print actionable error |
| Impact max_depth clamped to [1,3] | Document in `--help` text for code-impact command |

## Deep-Design Corrections Applied

Summary of corrections from Metis + Oracle review (2026-05-29):

1. **Fixed endpoint paths**: `/api/v1/graph` → `/api/v1/graph/query`, `/api/v1/impact` → `/api/v1/graph/impact`
2. **Fixed grammar function names**: `JavaScriptLanguage` → `JavascriptLanguage`, `TypeScriptLanguage` → `TypescriptLanguage`
3. **Added two-phase call detection**: All languages now specify both function-body-range queries AND call-expression queries
4. **Fixed TS function body node**: `block` → `statement_block` for TypeScript/JavaScript
5. **Fixed require() detection**: Removed blanket query matching all calls with string args; use post-filter
6. **Fixed Python assignment scope**: Must filter to module-level parent only
7. **Added TSX dual-grammar requirement**: `.tsx` needs separate `TsxLanguage()` instance
8. **Added detect-changes prerequisites**: git exec, timeout, stale-index handling
9. **Removed duplicate import query**: Single pattern + `strings.Trim` (like Go extractor)
