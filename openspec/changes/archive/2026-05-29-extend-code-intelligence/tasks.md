# Extend Code Intelligence — Tasks

## Task 1: TypeScript Graph Extractor

**Files:** `internal/graph/typescript_extractor.go`, `internal/graph/typescript_extractor_test.go`

- [ ] Create `TypeScriptGraphExtractor` struct with dual grammar: `grammars.TypescriptLanguage()` for `.ts`, `grammars.TsxLanguage()` for `.tsx` (match `internal/symbol/typescript_extractor.go:28-33`)
- [ ] Implement `Supports()` for `.ts`, `.tsx`
- [ ] Implement `ExtractEdges()` following GoGraphExtractor pattern exactly
- [ ] Handle ES imports: single query `(import_statement source: (string) @path)` + `strings.Trim` to strip quotes
- [ ] Handle require(): match `call_expression` with `identifier` function, post-filter `bt.NodeText(fnNode) == "require"`, extract string arg as import edge. Do NOT use blanket query matching all calls with string args.
- [ ] Handle contains: function, class, interface, type alias, enum, const/let declarations
- [ ] Handle calls TWO-PHASE: Phase 1 finds function bodies via `(function_declaration name: (identifier) @fn_name body: (statement_block) @body)` and `(method_definition name: (property_identifier) @fn_name body: (statement_block) @body)`. Phase 2 finds call expressions within byte ranges. Note: `statement_block` not `block` for TS/JS.
- [ ] Known limitation: arrow functions attributed to file level (no enclosing function)
- [ ] Unit tests with sample TypeScript source — include: ES imports, require(), function + method calls, TSX file
- [ ] Verify: `go test -race -short ./internal/graph/...`

## Task 2: JavaScript Graph Extractor

**Files:** `internal/graph/javascript_extractor.go`, `internal/graph/javascript_extractor_test.go`

- [ ] Create `JavaScriptGraphExtractor` struct using `grammars.JavascriptLanguage()` (lowercase 's', single grammar — no dual-lang)
- [ ] Implement `Supports()` for `.js`, `.jsx`
- [ ] Handle ES imports: same single-query pattern as TS
- [ ] Handle CommonJS require(): same post-filter approach as TS
- [ ] Handle contains: function, class, const/let declarations (no interface/enum/type_alias)
- [ ] Handle calls TWO-PHASE: same `statement_block` approach as TS
- [ ] Unit tests with sample JavaScript source — include: ES imports, require(), function calls, method calls
- [ ] Verify: `go test -race -short ./internal/graph/...`

## Task 3: Python Graph Extractor

**Files:** `internal/graph/python_extractor.go`, `internal/graph/python_extractor_test.go`

- [ ] Create `PythonGraphExtractor` struct using `grammars.PythonLanguage()`
- [ ] Implement `Supports()` for `.py`
- [ ] Handle imports: `import x`, `from x import y` — uses `(import_statement name: (dotted_name) @path)` and `(import_from_statement module_name: (dotted_name) @path)`
- [ ] Handle contains: function_definition, class_definition
- [ ] Handle module-level assignments: `(assignment left: (identifier) @name) @decl` — MUST filter to only match assignments whose parent node type is `module`. Nested assignments (inside functions) SHALL NOT produce contains edges.
- [ ] Handle calls TWO-PHASE: Phase 1 uses `(function_definition name: (identifier) @fn_name body: (block) @body)` — note Python uses `block` not `statement_block`. Phase 2 uses `(call function: (identifier) @callee)` — note Python uses `call` not `call_expression`.
- [ ] Unit tests with sample Python source — include: imports, from-imports, module-level constants, nested assignments (must NOT produce edges), function calls, method calls
- [ ] Verify: `go test -race -short ./internal/graph/...`

## Task 4: Register extractors in server startup

**Files:** `cmd/nano-brain/main.go`

- [ ] Create TypeScript, JavaScript, Python graph extractors
- [ ] Add all 3 to `graph.NewRegistry(goGraph, tsGraph, jsGraph, pyGraph)`
- [ ] Handle constructor errors (log.Warn, don't crash — match existing pattern)
- [ ] Verify: `CGO_ENABLED=0 go build ./...`

## Task 5: CLI — `context` command

**Files:** `cmd/nano-brain/cmd_context.go`

- [ ] Parse args: `<symbol>`, `--workspace`, `--json`
- [ ] HTTP calls: `GET /api/v1/symbols?query=<symbol>&workspace=<hash>` → `POST /api/v1/graph/query` (direction=out) → `POST /api/v1/graph/query` (direction=in)
- [ ] Format: human-readable (default) or JSON
- [ ] Register in CLI dispatcher (follow `runStubCmd` pattern in `commands.go`)
- [ ] Verify: `go build ./... && ./nano-brain context --help`

## Task 6: CLI — `code-impact` command

**Files:** `cmd/nano-brain/cmd_code_impact.go`

- [ ] Parse args: `<symbol>`, `--workspace`, `--depth` (default 2, server clamps to [1,3]), `--edge-type`, `--json`
- [ ] HTTP call: `POST /api/v1/graph/impact` with `{"workspace":"<hash>","node":"<symbol>","max_depth":2}`
- [ ] Document max_depth [1,3] clamp in `--help` text
- [ ] Format: tree view (default) or JSON
- [ ] Register in CLI dispatcher
- [ ] Verify: `go build ./... && ./nano-brain code-impact --help`

## Task 7: CLI — `detect-changes` command

**Files:** `cmd/nano-brain/cmd_detect_changes.go`

- [ ] Parse args: `--staged` (default), `--all`, `--workspace`, `--json`
- [ ] Check `exec.LookPath("git")` first — print "git not found in PATH" + exit 1 if missing
- [ ] Run `exec.CommandContext` with 10s timeout: `git diff --name-only [--staged]` → parse changed files
- [ ] For each file: `git diff [--staged] <file>` → parse unified diff hunk headers (`@@ -start,count +start,count @@`) for changed line ranges
- [ ] Cross-reference with indexed symbols by file + line range via `GET /api/v1/symbols?file=<file>&workspace=<hash>`
- [ ] Run impact analysis per affected symbol (depth=1) via `POST /api/v1/graph/impact`
- [ ] Format: summary + per-file breakdown (text or JSON)
- [ ] Register in CLI dispatcher
- [ ] Verify: `go build ./... && ./nano-brain detect-changes --help`

## Task 8: Integration tests

**Files:** `internal/graph/integration_test.go` (or per-extractor `*_integration_test.go`)

- [ ] Test TypeScript: create temp .ts file → extract edges → verify contains + imports + calls
- [ ] Test JavaScript: create temp .js file → extract edges → verify
- [ ] Test Python: create temp .py file → extract edges → verify
- [ ] Test watcher pipeline: file change → DB has correct edges (if watcher integration test exists)
- [ ] Verify: `go test -race -tags=integration ./internal/graph/...`

## Task 9: Validate + final checks

- [ ] `CGO_ENABLED=0 go build ./...` passes
- [ ] `go test -race -short ./...` passes (all tests)
- [ ] `go test -race -tags=integration ./...` passes
- [ ] Existing Go graph extractor tests still pass
- [ ] Update README.md CLI commands table if needed
