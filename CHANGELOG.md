# Changelog

All notable changes to nano-brain are documented here.


## [Unreleased]

### Changed (MCP only — breaking for agents parsing `content` from search results)

- **`memory_query` / `memory_search` / `memory_vsearch`** now return a 500-char `snippet` by default and OMIT the full `content` field. Agents needing full text must either pass `include_content: true` or call `memory_get` for one full document. HTTP API (`/api/v1/search`, `/api/v1/query`, `/api/v1/vsearch`) is unchanged. (#358)

### Added

- Stateless cursor-based pagination for all three MCP search tools. Each response includes a `next_cursor` field when more results exist; pass it back in the next call's `cursor` parameter. Cursors are opaque base64url(JSON) and bound to the query text (server returns `"cursor query mismatch"` if the query changes mid-pagination). (#358)
- New `include_content` boolean parameter on all three search tools (default `false`). Set to `true` to include full chunk content alongside the snippet. (#358)
- New top-level response fields `total` (count of fused results) and `query_ms` (server-measured latency) on all three search tools. (#358)
- Stable result-ordering tiebreaker (`id ASC`) in RRF fusion and recency boost. Equal-score results now have deterministic order across paginated calls. (#358)

### Internal

- Extracted `TruncateSnippet` helper from `internal/server/handlers/search.go` to `internal/search/snippet.go` so HTTP and MCP layers share the same rune-aware truncation.

## [2026.6.0] — 2026-05-30

### Added
- feat(cli): add cleanup-stale-raw command (closes #190)
- feat(config): bump summarization.max_tokens default 4096 → 8000 (closes #191)
- feat(harvest): multi-DB OpenCode discovery via db_root (#199, #200)
- feat(openspec): archive completed changes and lock new specs
- feat(graph): add TypeScript, JavaScript, Python graph extractors and CLI commands (#197)
- feat(harvest): filter OpenCode sessions by registered workspaces (#195)
- feat(harvest): summary-only ingestion with unified-path fallback (#189) (#192)
- feat: real migration_version in /api/status + init --force prints harvest result
- feat(init): add --force flag to reset workspace + remove CWD auto-detect
- feat(summarize): add POST /api/v1/summarize endpoint
- feat(init): full onboarding for all config sections
- feat(summarize,init): add debug logs + summarization onboarding
- feat(summarize): add token-bucket rate limiter for LLM calls
- feat(summarize): session summarization pipeline v2026.5.267
- feat(watcher): config-based filter - global + per-workspace exclude/extensions
- feat(watcher): file filtering - gitignore, exclude patterns, allowed extensions
- feat(memory): auto-extract decisions/lessons from sessions (Pillar 3)
- feat(graph): call chain tracing - memory_trace tool + POST /graph/trace (Pillar 1d)
- feat(graph): impact analytics - memory_impact tool + POST /graph/impact (Pillar 1c)
- feat(harvester): add OpenCode SQLite harvester (Pillar 2a)
- feat(graph): add knowledge graph (Pillar 1b)
- feat(cli): add doctor command tests + mark tasks complete
- feat: nano-brain v2 — complete greenfield rewrite (Go + PostgreSQL + pgvector)
- feat(code-intel): symbol extraction with gotreesitter (Pillar 1, issue #174)
- feat(ux): auto-trigger reindex + harvest after init --root
- feat(cli): add reindex command — wire CLI to POST /api/v1/reindex (#159)
- feat(cli+server): comprehensive structured logging (#144) (#148)
- feat(cli): auto-detect opencode session_dir at startup (#147)
- feat(cli): add workspaces list command + real health workspace count (#146)
- feat(npm): migrate to @nano-step/nano-brain + CI auto-publish (#139) (#140)
- feat(cli): add container server guard with port check and auto-config (#137) (#138)
- feat(cli): add daemon management (serve -d, stop, restart) (#135) (#136)
- feat: enhance interactive init wizard (#134)
- feat: config show/check commands
- feat: add version command
- feat: interactive init wizard for first-time setup
- feat: add doctor command for prerequisite checking
- feat: add npx nano-brain support via npm package wrapper
- feat: search telemetry with 90-day retention (Story 8.6)
- feat: tags endpoint, reindex API, enhanced workspaces (Story 8.5)
- feat: CLI operations commands — logs, docker, status, db:migrate goose (Story 8.4)
- feat: config hot-reload endpoint POST /api/reload-config (Story 8.3)
- feat: v1 SQLite to PostgreSQL migration command (Story 8.1)
- feat: bench --help and JSON round-trip validation (Story 7.5) (#115)
- feat: bench stress command for concurrent write testing (Story 7.4) (#113)
- feat: bench compare command for regression detection (Story 7.3) (#111)
- feat: bench run command with quality metrics and latency (Story 7.2) (#109)
- feat: bench generate command and dataset generator (Story 7.1) (#107)
- feat: CLI harvest command and POST /api/harvest (Story 6.5) (#105)
- feat: add harvester config to GET /api/status (Story 6.4) (#103)
- feat: implement Claude Code session harvester (Story 6.2) (#99)
- feat: implement OpenCode session harvester (Story 6.1) (#97)
- feat: implement memory_get and supersedes for memory_write (Story 5.6) (#95)
- feat: configure 30s KeepAlive on MCP server (Story 5.5) (#93)
- feat: enforce workspace on all MCP tools + cross-workspace 'all' (Story 5.3) (#89)
- feat: register all 9 MCP tools on both transports (Story 5.2) (#87)
- feat: mount MCP SSE and Streamable HTTP transports (Story 5.1) (#85)
- feat: add wake-up briefing endpoint (Story 4.6) (#83)
- feat(search): thread-safe config updates + validation tests (#80)
- feat(search): POST /api/query hybrid search pipeline (#75)
- feat(search): POST /api/v1/search BM25 full-text search (#71)
- feat(embed): POST /api/v1/embed trigger + status observability (#69)
- feat(search): POST /api/v1/vsearch vector search endpoint (#67)
- feat(embed): backpressure + retry/failure handling (#65)
- feat(embed): async embedding queue with backoff + concurrency (#63)
- feat(embed): embedding provider interface + Ollama/VoyageAI implementations (#61)
- feat(embed): HNSW migration, embed_status column, embedding sqlc queries (#59)
- feat(cli): init, write, query/search/vsearch commands + env/JSON flags (#57)
- feat(collections): collection management CLI + live watcher attach (#55)
- feat(watcher): implement file watcher with debounced reindex (#53)
- feat(ingestion): integrate chunker into WriteDocument handler (#51)
- feat: Story 2.3 — document write endpoint (POST /api/write)
- feat: Story 2.2 — API middleware (workspace validation, content-type enforcement)
- feat(harness): add b-main base branch rule + PR target check
- feat: nano-brain v2 greenfield — Epic 1 foundation + Epic 2 progress
- feat: interactive first-run setup wizard (closes #43)
- feat(graph): auto-zoom to selected node + neighbors on focus mode
- feat(graph): focus mode on node click + fix node overlap (#37)
- feat(obsidian,db): excludeFolders, frontmatter tags, db:clean --list-only (closes #34)
- feat(obsidian): Obsidian vault integration + fix entity API limit 100→2000 (closes #34)
- feat(kg): scheduled entity extraction job for memory documents (closes #30)
- feat(docs): extend Three.js neural graph to full-page background on all docs pages (closes #28)
- feat(docs): Three.js neural graph animation in hero — nodes + edges + pulse (closes #27)
- feat(docs): custom hover tooltips on MCP tools grid (closes #26)
- feat(docs): CHANGELOG.md + changelog page renders it via marked.js (closes #24)
- feat(docs): add setup guide page — Docker, config, Ollama, MCP connect, verify steps
- feat: add GitHub Pages site — landing, features, changelog, docs (closes #23)

### Fixed
- fix(embed): scope queue scan to registered workspaces only (#187) (#188)
- fix: drop incorrect content_hash unique constraint on documents
- fix: recursive watcher scan + tilde expansion for memory/sessions paths
- fix: register watcher immediately on POST /api/v1/init
- fix(harvest): scope OpenCodeSQLiteHarvester sessions to per-project workspace
- fix(summarize): expand tilde in output_dir and fail-fast on mkdir
- fix(init): default log file path in onboarding prompt
- fix(status): expose actual embedding provider in active_provider field
- fix(embed): reduce maxEmbedChars to 4000 for nomic-embed-text 2048-token limit
- fix(embed,harvest): truncate oversized chunks + strip null bytes
- fix(ci): reset package.json version to 0.0.0-dev for CI version bump
- fix(harvest): enqueue chunk IDs after tx.Commit, not before
- fix(harvest): align SQLite queries to actual OpenCode schema
- fix(harvest): auto-detect opencode.db at ~/.local/share/opencode on macOS
- fix(watcher): move processing file log after IsDir check to reduce noise
- fix(embed): wire watcher+reindex to embed queue, add file-level logs
- fix(mcp): populate kind/language/signature in memory_symbols tool response
- fix(review): post-commit self-review fixes + harness lessons
- fix(watcher): skip directories in processFile
- fix(init): always use factory defaults on overwrite, drop stale config loading
- fix(init): always prompt for harvester session_dir, add Claude Code detection
- fix(reindex): resolve collection by path, not treat root as collection name
- fix(ux): treat harvest 503 as info, not warn on init
- fix(watcher): skip non-existent collection paths silently
- fix(embed): recover embed_failed chunks in periodic scan
- fix(cli): reindex auto-derives workspace hash from --root path
- fix(server): implement POST /api/v1/reindex — replace no-op stub (#162)
- fix(init): register default code collection at workspace root (#161)
- fix(cli): smart recovery on 'cannot connect to nano-brain server' (#141) (#145)
- fix(test): fix integration test build + harness 3.10 branch slug extraction
- fix(lint): resolve all golangci-lint errcheck/unused/ineffassign issues
- fix: use version --json for binary cache check in postinstall
- fix: skip watcher for non-existent collection paths
- fix: help command and unknown command handling
- fix(ci): use nanobrain_dev DB to match default config
- fix: prevent nil pointer panic in WriteDocument when embedding disabled (#82)
- fix: address all PR #45 review findings + add self-review harness gate
- fix(graph): allow dragging focused nodes — add pointerEvents:none to dimmed dots
- fix(graph): hide edge labels ('call') when edges dimmed in focus mode
- fix(graph): hide edge labels when edges are dimmed in focus mode
- fix(graph): Symbol Call Graph — default individual mode + fix cluster edge filter (closes #38)
- fix(graph): collapse unrelated nodes to dots on focus — no label overlap
- fix(watcher): address Gemini review — streaming body, yaml parser, async fs, ready event
- fix(db): eliminate SQLite corruption root causes — readonly check + RESTART checkpoint
- fix(db): add db:clean command and bootstrap orphan/corruption guards (closes #32, #33)
- fix(kg): raise entity API limit 100→2000 so all entities appear in graph
- fix(kg): start extraction cycle 30s after startup instead of 30min
- fix(kg): startup reindex + fast drain for entity extraction queue
- fix(ci,web): web deps install in CI + fix graph node overlap
- fix(web): favicon 404, search mark rendering, missing CI web build (closes #29)
- fix(docs): Three.js — use r128 UMD CDN, fix overflow, fix init timing
- fix(docs): npx nano-brain mcp + add ai-sandbox-wrapper container setup section (closes #25)
- fix(pages): redirect root index.html → docs/index.html
- fix(docs): correct codebase facts — 30 tools, 32 CLI, 11-stage pipeline, 5 languages, accurate MCP config
- fix(ci): skip GitHub release creation if tag already exists (closes #22)
- fix(ci): skip npm publish if version already exists on registry
- fix(ci): use --legacy-peer-deps for npm ci (tree-sitter peer conflict)

### Changed
- refactor(search): normalize SearchResult schema across all endpoints (#73)
- refactor(graph): consolidate 3 useEffects into 1 — address Gemini review
- refactor(graph): shared GraphShell + node cap + truncation banner on all graphs (closes #41)

### Documentation
- docs(harness): add 3 anti-footgun rules to prevent recent merge mistakes (#202)
- docs(agents): origin now HTTPS, simplify git push workflow
- docs(evidence): cleanup-stale-raw end-to-end test (#190)
- docs: clarify summarization no longer writes .md to disk (closes #198)
- docs(agents): refine AGENTS.md/CLAUDE.md/ROADMAP and ignore .sisyphus runtime
- docs(agents): add per-package AGENTS.md files for AI context
- docs: fix container API URL in AGENTS.md — use host.docker.internal:3100
- docs(agents): add git push workflow for container environment (SSH blocked, use HTTPS kokorolx token)
- docs(agents): update harness block with full validation ladder and rules
- docs(roadmap): update status - Pillar 1 complete, Pillar 2a/2b done
- docs(harness): add response-shape + staged-files gates, forbidden practice #14
- docs: add ROADMAP.md with Pillar 1-4 vision
- docs: add per-branch self-review stubs for harness 3.10 gate (#146 #147 #148)
- docs: add self-review evidence for PRs #146 #147 #148
- docs: add npx quick start, doctor command, and npx caveats
- docs: add README.md and landing page for v2
- docs: add npm, CI, license badges to README (closes #21)
- docs(claude): add rule to create GitHub issue before starting any task

### Other
- chore(npm): manual publish 2026.5.2903 (#206)
- chore(npm): commit package-lock.json for shared-workflows compat (#204)
- chore(gitignore): exclude .sisyphus/ orchestrator state
- chore(openspec): archive opencode-multi-db-discovery (#199)
- chore(openspec): mark extend-code-intelligence tasks as complete
- chore(config): add project opencode.json to disable unused MCPs
- chore(skills): remove unused BMad skill bundles
- chore(openspec): archive harvest-summary-only
- chore(openspec): archive 7 completed changes
- chore(openspec): archive embed-queue-workspace-isolation
- chore(openspec): archive fix-harvester-per-project-workspace change
- chore(openspec): archive opencode-sqlite-harvester
- chore(openspec): archive knowledge-graph (implemented + tested)
- chore(openspec): archive reindex-real-impl (already implemented + tested)
- chore(openspec): archive init-default-collections (already implemented + tested)
- chore(openspec): archive doctor-command (complete)
- chore(openspec): archive 4 completed changes (logging, connect-error-ux, harvester-autodetect, workspaces-list-cli)
- chore(ci): upgrade Node.js 20 → 24 in release workflow
- chore: archive npm-scoped-publish OpenSpec
- chore: archive container-server-guard OpenSpec
- chore: archive daemon-management OpenSpec
- chore: bump npm version to 2.0.0-beta.5
- chore: archive OpenSpec enhanced-init
- chore: archive OpenSpec config-commands
- chore: bump npm version to 2.0.0-beta.4
- chore: archive OpenSpec interactive-init
- chore: bump npm version to 2.0.0-beta.3
- chore: bump npm version to 2.0.0-beta.2
- chore: update harness state — ALL 8 EPICS COMPLETE
- chore: update harness state — Epic 7 complete, position 8.1
- chore: update harness state — Story 7.4 done, position 7.5
- chore: update harness state — Story 7.3 done, position 7.4
- chore: update harness state — Story 7.2 done, position 7.3
- chore: update harness state — Story 7.1 done, position 7.2
- chore: update harness state - Epic 6 complete, position Epic 7
- chore: update harness state — Epic 5 complete, advance to Epic 6
- chore: update harness state — Story 5.5 done, advance to 5.6
- chore: update harness state — Story 5.4 done, position 5.5
- chore: update harness state — Story 5.3 done, position 5.4
- chore: update harness state — Story 5.2 done, position 5.3
- chore: update harness state — Story 5.1 done, position 5.2
- chore: Epic 4 complete; advance to Epic 5, Story 5.1
- chore: add smoke:e2e to harness rules and harness-check skill
- chore: update harness state - Story 4.3 done, position 4.4
- chore: update harness-state for Story 4.2 completion
- chore: update harness-state for Story 4.1 completion
- chore: update harness-state for Epic 3 completion
- chore: update harness state — Epic 2 complete, position → 3.1
- chore(bmad): install BMad Builder module
- chore(b-main): greenfield baseline
- chore: migrate workflows to kokorolx/shared-workflows@v1
- chore: fix homepage URL to GitHub Pages docs site
- chore: add repository, homepage, and bugs URLs to package.json

**Install:** `npm install @nano-step/nano-brain@2026.6.0`

---

## [0.0.0] — 2026-05-30

### Added
- feat(cli): add cleanup-stale-raw command (closes #190)
- feat(config): bump summarization.max_tokens default 4096 → 8000 (closes #191)
- feat(harvest): multi-DB OpenCode discovery via db_root (#199, #200)
- feat(openspec): archive completed changes and lock new specs
- feat(graph): add TypeScript, JavaScript, Python graph extractors and CLI commands (#197)
- feat(harvest): filter OpenCode sessions by registered workspaces (#195)
- feat(harvest): summary-only ingestion with unified-path fallback (#189) (#192)
- feat: real migration_version in /api/status + init --force prints harvest result
- feat(init): add --force flag to reset workspace + remove CWD auto-detect
- feat(summarize): add POST /api/v1/summarize endpoint
- feat(init): full onboarding for all config sections
- feat(summarize,init): add debug logs + summarization onboarding
- feat(summarize): add token-bucket rate limiter for LLM calls
- feat(summarize): session summarization pipeline v2026.5.267
- feat(watcher): config-based filter - global + per-workspace exclude/extensions
- feat(watcher): file filtering - gitignore, exclude patterns, allowed extensions
- feat(memory): auto-extract decisions/lessons from sessions (Pillar 3)
- feat(graph): call chain tracing - memory_trace tool + POST /graph/trace (Pillar 1d)
- feat(graph): impact analytics - memory_impact tool + POST /graph/impact (Pillar 1c)
- feat(harvester): add OpenCode SQLite harvester (Pillar 2a)
- feat(graph): add knowledge graph (Pillar 1b)
- feat(cli): add doctor command tests + mark tasks complete
- feat: nano-brain v2 — complete greenfield rewrite (Go + PostgreSQL + pgvector)
- feat(code-intel): symbol extraction with gotreesitter (Pillar 1, issue #174)
- feat(ux): auto-trigger reindex + harvest after init --root
- feat(cli): add reindex command — wire CLI to POST /api/v1/reindex (#159)
- feat(cli+server): comprehensive structured logging (#144) (#148)
- feat(cli): auto-detect opencode session_dir at startup (#147)
- feat(cli): add workspaces list command + real health workspace count (#146)
- feat(npm): migrate to @nano-step/nano-brain + CI auto-publish (#139) (#140)
- feat(cli): add container server guard with port check and auto-config (#137) (#138)
- feat(cli): add daemon management (serve -d, stop, restart) (#135) (#136)
- feat: enhance interactive init wizard (#134)
- feat: config show/check commands
- feat: add version command
- feat: interactive init wizard for first-time setup
- feat: add doctor command for prerequisite checking
- feat: add npx nano-brain support via npm package wrapper
- feat: search telemetry with 90-day retention (Story 8.6)
- feat: tags endpoint, reindex API, enhanced workspaces (Story 8.5)
- feat: CLI operations commands — logs, docker, status, db:migrate goose (Story 8.4)
- feat: config hot-reload endpoint POST /api/reload-config (Story 8.3)
- feat: v1 SQLite to PostgreSQL migration command (Story 8.1)
- feat: bench --help and JSON round-trip validation (Story 7.5) (#115)
- feat: bench stress command for concurrent write testing (Story 7.4) (#113)
- feat: bench compare command for regression detection (Story 7.3) (#111)
- feat: bench run command with quality metrics and latency (Story 7.2) (#109)
- feat: bench generate command and dataset generator (Story 7.1) (#107)
- feat: CLI harvest command and POST /api/harvest (Story 6.5) (#105)
- feat: add harvester config to GET /api/status (Story 6.4) (#103)
- feat: implement Claude Code session harvester (Story 6.2) (#99)
- feat: implement OpenCode session harvester (Story 6.1) (#97)
- feat: implement memory_get and supersedes for memory_write (Story 5.6) (#95)
- feat: configure 30s KeepAlive on MCP server (Story 5.5) (#93)
- feat: enforce workspace on all MCP tools + cross-workspace 'all' (Story 5.3) (#89)
- feat: register all 9 MCP tools on both transports (Story 5.2) (#87)
- feat: mount MCP SSE and Streamable HTTP transports (Story 5.1) (#85)
- feat: add wake-up briefing endpoint (Story 4.6) (#83)
- feat(search): thread-safe config updates + validation tests (#80)
- feat(search): POST /api/query hybrid search pipeline (#75)
- feat(search): POST /api/v1/search BM25 full-text search (#71)
- feat(embed): POST /api/v1/embed trigger + status observability (#69)
- feat(search): POST /api/v1/vsearch vector search endpoint (#67)
- feat(embed): backpressure + retry/failure handling (#65)
- feat(embed): async embedding queue with backoff + concurrency (#63)
- feat(embed): embedding provider interface + Ollama/VoyageAI implementations (#61)
- feat(embed): HNSW migration, embed_status column, embedding sqlc queries (#59)
- feat(cli): init, write, query/search/vsearch commands + env/JSON flags (#57)
- feat(collections): collection management CLI + live watcher attach (#55)
- feat(watcher): implement file watcher with debounced reindex (#53)
- feat(ingestion): integrate chunker into WriteDocument handler (#51)
- feat: Story 2.3 — document write endpoint (POST /api/write)
- feat: Story 2.2 — API middleware (workspace validation, content-type enforcement)
- feat(harness): add b-main base branch rule + PR target check
- feat: nano-brain v2 greenfield — Epic 1 foundation + Epic 2 progress
- feat: interactive first-run setup wizard (closes #43)
- feat(graph): auto-zoom to selected node + neighbors on focus mode
- feat(graph): focus mode on node click + fix node overlap (#37)
- feat(obsidian,db): excludeFolders, frontmatter tags, db:clean --list-only (closes #34)
- feat(obsidian): Obsidian vault integration + fix entity API limit 100→2000 (closes #34)
- feat(kg): scheduled entity extraction job for memory documents (closes #30)
- feat(docs): extend Three.js neural graph to full-page background on all docs pages (closes #28)
- feat(docs): Three.js neural graph animation in hero — nodes + edges + pulse (closes #27)
- feat(docs): custom hover tooltips on MCP tools grid (closes #26)
- feat(docs): CHANGELOG.md + changelog page renders it via marked.js (closes #24)
- feat(docs): add setup guide page — Docker, config, Ollama, MCP connect, verify steps
- feat: add GitHub Pages site — landing, features, changelog, docs (closes #23)

### Fixed
- fix(embed): scope queue scan to registered workspaces only (#187) (#188)
- fix: drop incorrect content_hash unique constraint on documents
- fix: recursive watcher scan + tilde expansion for memory/sessions paths
- fix: register watcher immediately on POST /api/v1/init
- fix(harvest): scope OpenCodeSQLiteHarvester sessions to per-project workspace
- fix(summarize): expand tilde in output_dir and fail-fast on mkdir
- fix(init): default log file path in onboarding prompt
- fix(status): expose actual embedding provider in active_provider field
- fix(embed): reduce maxEmbedChars to 4000 for nomic-embed-text 2048-token limit
- fix(embed,harvest): truncate oversized chunks + strip null bytes
- fix(ci): reset package.json version to 0.0.0-dev for CI version bump
- fix(harvest): enqueue chunk IDs after tx.Commit, not before
- fix(harvest): align SQLite queries to actual OpenCode schema
- fix(harvest): auto-detect opencode.db at ~/.local/share/opencode on macOS
- fix(watcher): move processing file log after IsDir check to reduce noise
- fix(embed): wire watcher+reindex to embed queue, add file-level logs
- fix(mcp): populate kind/language/signature in memory_symbols tool response
- fix(review): post-commit self-review fixes + harness lessons
- fix(watcher): skip directories in processFile
- fix(init): always use factory defaults on overwrite, drop stale config loading
- fix(init): always prompt for harvester session_dir, add Claude Code detection
- fix(reindex): resolve collection by path, not treat root as collection name
- fix(ux): treat harvest 503 as info, not warn on init
- fix(watcher): skip non-existent collection paths silently
- fix(embed): recover embed_failed chunks in periodic scan
- fix(cli): reindex auto-derives workspace hash from --root path
- fix(server): implement POST /api/v1/reindex — replace no-op stub (#162)
- fix(init): register default code collection at workspace root (#161)
- fix(cli): smart recovery on 'cannot connect to nano-brain server' (#141) (#145)
- fix(test): fix integration test build + harness 3.10 branch slug extraction
- fix(lint): resolve all golangci-lint errcheck/unused/ineffassign issues
- fix: use version --json for binary cache check in postinstall
- fix: skip watcher for non-existent collection paths
- fix: help command and unknown command handling
- fix(ci): use nanobrain_dev DB to match default config
- fix: prevent nil pointer panic in WriteDocument when embedding disabled (#82)
- fix: address all PR #45 review findings + add self-review harness gate
- fix(graph): allow dragging focused nodes — add pointerEvents:none to dimmed dots
- fix(graph): hide edge labels ('call') when edges dimmed in focus mode
- fix(graph): hide edge labels when edges are dimmed in focus mode
- fix(graph): Symbol Call Graph — default individual mode + fix cluster edge filter (closes #38)
- fix(graph): collapse unrelated nodes to dots on focus — no label overlap
- fix(watcher): address Gemini review — streaming body, yaml parser, async fs, ready event
- fix(db): eliminate SQLite corruption root causes — readonly check + RESTART checkpoint
- fix(db): add db:clean command and bootstrap orphan/corruption guards (closes #32, #33)
- fix(kg): raise entity API limit 100→2000 so all entities appear in graph
- fix(kg): start extraction cycle 30s after startup instead of 30min
- fix(kg): startup reindex + fast drain for entity extraction queue
- fix(ci,web): web deps install in CI + fix graph node overlap
- fix(web): favicon 404, search mark rendering, missing CI web build (closes #29)
- fix(docs): Three.js — use r128 UMD CDN, fix overflow, fix init timing
- fix(docs): npx nano-brain mcp + add ai-sandbox-wrapper container setup section (closes #25)
- fix(pages): redirect root index.html → docs/index.html
- fix(docs): correct codebase facts — 30 tools, 32 CLI, 11-stage pipeline, 5 languages, accurate MCP config
- fix(ci): skip GitHub release creation if tag already exists (closes #22)
- fix(ci): skip npm publish if version already exists on registry
- fix(ci): use --legacy-peer-deps for npm ci (tree-sitter peer conflict)

### Changed
- refactor(search): normalize SearchResult schema across all endpoints (#73)
- refactor(graph): consolidate 3 useEffects into 1 — address Gemini review
- refactor(graph): shared GraphShell + node cap + truncation banner on all graphs (closes #41)

### Documentation
- docs(harness): add 3 anti-footgun rules to prevent recent merge mistakes (#202)
- docs(agents): origin now HTTPS, simplify git push workflow
- docs(evidence): cleanup-stale-raw end-to-end test (#190)
- docs: clarify summarization no longer writes .md to disk (closes #198)
- docs(agents): refine AGENTS.md/CLAUDE.md/ROADMAP and ignore .sisyphus runtime
- docs(agents): add per-package AGENTS.md files for AI context
- docs: fix container API URL in AGENTS.md — use host.docker.internal:3100
- docs(agents): add git push workflow for container environment (SSH blocked, use HTTPS kokorolx token)
- docs(agents): update harness block with full validation ladder and rules
- docs(roadmap): update status - Pillar 1 complete, Pillar 2a/2b done
- docs(harness): add response-shape + staged-files gates, forbidden practice #14
- docs: add ROADMAP.md with Pillar 1-4 vision
- docs: add per-branch self-review stubs for harness 3.10 gate (#146 #147 #148)
- docs: add self-review evidence for PRs #146 #147 #148
- docs: add npx quick start, doctor command, and npx caveats
- docs: add README.md and landing page for v2
- docs: add npm, CI, license badges to README (closes #21)
- docs(claude): add rule to create GitHub issue before starting any task

### Other
- chore(npm): commit package-lock.json for shared-workflows compat (#204)
- chore(gitignore): exclude .sisyphus/ orchestrator state
- chore(openspec): archive opencode-multi-db-discovery (#199)
- chore(openspec): mark extend-code-intelligence tasks as complete
- chore(config): add project opencode.json to disable unused MCPs
- chore(skills): remove unused BMad skill bundles
- chore(openspec): archive harvest-summary-only
- chore(openspec): archive 7 completed changes
- chore(openspec): archive embed-queue-workspace-isolation
- chore(openspec): archive fix-harvester-per-project-workspace change
- chore(openspec): archive opencode-sqlite-harvester
- chore(openspec): archive knowledge-graph (implemented + tested)
- chore(openspec): archive reindex-real-impl (already implemented + tested)
- chore(openspec): archive init-default-collections (already implemented + tested)
- chore(openspec): archive doctor-command (complete)
- chore(openspec): archive 4 completed changes (logging, connect-error-ux, harvester-autodetect, workspaces-list-cli)
- chore(ci): upgrade Node.js 20 → 24 in release workflow
- chore: archive npm-scoped-publish OpenSpec
- chore: archive container-server-guard OpenSpec
- chore: archive daemon-management OpenSpec
- chore: bump npm version to 2.0.0-beta.5
- chore: archive OpenSpec enhanced-init
- chore: archive OpenSpec config-commands
- chore: bump npm version to 2.0.0-beta.4
- chore: archive OpenSpec interactive-init
- chore: bump npm version to 2.0.0-beta.3
- chore: bump npm version to 2.0.0-beta.2
- chore: update harness state — ALL 8 EPICS COMPLETE
- chore: update harness state — Epic 7 complete, position 8.1
- chore: update harness state — Story 7.4 done, position 7.5
- chore: update harness state — Story 7.3 done, position 7.4
- chore: update harness state — Story 7.2 done, position 7.3
- chore: update harness state — Story 7.1 done, position 7.2
- chore: update harness state - Epic 6 complete, position Epic 7
- chore: update harness state — Epic 5 complete, advance to Epic 6
- chore: update harness state — Story 5.5 done, advance to 5.6
- chore: update harness state — Story 5.4 done, position 5.5
- chore: update harness state — Story 5.3 done, position 5.4
- chore: update harness state — Story 5.2 done, position 5.3
- chore: update harness state — Story 5.1 done, position 5.2
- chore: Epic 4 complete; advance to Epic 5, Story 5.1
- chore: add smoke:e2e to harness rules and harness-check skill
- chore: update harness state - Story 4.3 done, position 4.4
- chore: update harness-state for Story 4.2 completion
- chore: update harness-state for Story 4.1 completion
- chore: update harness-state for Epic 3 completion
- chore: update harness state — Epic 2 complete, position → 3.1
- chore(bmad): install BMad Builder module
- chore(b-main): greenfield baseline
- chore: migrate workflows to kokorolx/shared-workflows@v1
- chore: fix homepage URL to GitHub Pages docs site
- chore: add repository, homepage, and bugs URLs to package.json

**Install:** `npm install @nano-step/nano-brain@0.0.0`

---

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

---

## [Unreleased]

### Fixed
- fix(mcp): `memory_wake_up` MCP tool was returning empty `recent_memories` because the MCP handler passed `Collections: nil` to `RecentDocuments`, making the post-#338 collection filter (`collection = ANY('{}')`) always false. Now matches the HTTP `/api/v1/wake-up` behaviour. (#356)

### Performance
- Add partial index on chunks.embed_status (pending/embed_failed) for embed queue worker hot path (#322)
- Add in-flight dedup set in embed queue to prevent re-embedding of chunks already being processed by a worker (#322)

### Breaking changes (operator action required)

- **fix(security): close 7 workspace-registration leak points in summary + write paths (#238).** Every write path now enforces that the target `workspace_hash` is registered in the `workspaces` table. Five layers of defense-in-depth: HTTP middleware, MCP tool handlers, harvester init, `Persister.Save`, and a new PostgreSQL FK constraint with `ON DELETE CASCADE` (migration 00011).

  **Operator-facing changes:**

  1. **The OpenCode harvester no longer auto-registers workspaces.** Previously, `internal/harvest/opencode_sqlite.go` called `UpsertWorkspace` for every worktree it discovered, silently extending the trust boundary. Now, sessions whose worktree is not pre-registered via `POST /api/v1/init` or `nano-brain init --root=<path>` are skipped with a WARN log. Operators relying on implicit registration must run `nano-brain init` for each worktree explicitly.

  2. **HTTP write endpoints now return 400 instead of 503/200 for unregistered workspaces.** The new `workspaceRegisteredMiddleware` is applied to `POST /api/v1/{write,embed,reindex,update,summarize}`. Unregistered hash returns `HTTP 400 {"error":"workspace_not_registered"}`. The literal `"all"` is rejected on write endpoints (read endpoints unchanged — `"all"` still works for cross-workspace queries).

  3. **MCP `memory_write` and `memory_update` reject unregistered workspaces.** MCP transport bypasses HTTP middleware, so the same registration check is applied inside each write tool handler.

  4. **New CLI command `nano-brain cleanup-orphan-workspaces [--dry-run]`.** Removes documents and chunks whose `workspace_hash` is not in the `workspaces` table. Required before applying migration 00011 if any orphans exist; the migration will otherwise fail with PostgreSQL error 23503 (foreign-key violation).

  **Operator upgrade sequence (MUST follow in order):**

  ```bash
  # 1. STOP the running server (prevents stale binary from re-creating orphans)
  kill $(cat /path/to/nano-brain.pid)

  # 2. CLEANUP (dry-run first — output is your only backup of deleted summaries)
  nano-brain cleanup-orphan-workspaces --dry-run > orphan-backup.txt
  cat orphan-backup.txt  # review
  nano-brain cleanup-orphan-workspaces

  # 3. MIGRATE (will fail with error 23503 if cleanup was skipped)
  nano-brain db:migrate

  # 4. START the NEW binary (do NOT start the old one — it would auto-register orphans again)
  nano-brain
  ```

  **Notes:**
  - Orphan summary documents are LLM-generated and cannot be regenerated by the new code (the underlying sessions are now skipped). Save the `--dry-run` output as your record.
  - **V1 migration users:** if you previously imported data with `nano-brain db:migrate --from-v1` using the default `workspace_hash = "default"` (the implicit workspace when `--workspace` is not supplied), those documents are orphans and will block migration 00011 with error 23503. They will appear in `cleanup-orphan-workspaces --dry-run` output. Register the target workspace with `nano-brain init --root=<path>` before running cleanup, or let cleanup delete them and re-import post-migration with the new binary.
  - On large tables (>1M documents), migration 00011 validates every row and acquires a `ShareRowExclusiveLock`. Expect 30-60s of blocked writes. For very large deployments, consider applying `ADD CONSTRAINT ... NOT VALID` + `VALIDATE CONSTRAINT` manually.
  - Full test cycle artifacts at `ai/test-case/rri-t/summary/` (RRI-T), `openspec/changes/fix-summary-workspace-registration-leaks/` (proposal + spec deltas), and `docs/evidence/fix-summary-workspace-registration-leaks/` (Phase G user-flow evidence).

### Fixes
- fix(config): `NANO_BRAIN_CONFIG` (and `--config`) now strip leading/trailing whitespace, and a `WARNING:` is printed to stderr when the explicitly-set path does not exist (previously silently fell back to defaults — a production footgun for typos in container env values) (#224)
- fix(handler): wrap `ResetWorkspace` document+workspace deletion in a single transaction with rollback — matches the pattern already shipped in `RemoveWorkspace` (#155). Prevents orphaned documents if the workspace delete fails after docs are already removed (#225)

### Features

- **feat(summary): restore disk persistence for summaries (#258).** Session summaries are now written to disk as `.md` files in addition to PostgreSQL, enabling integration with Obsidian and other filesystem-based markdown tools. Disk writes are **enabled by default**.

  **File layout:**
  ```
  <output_dir>/<workspace_name>/<source>_<slug-title>_<YYYY-MM-DD>.md
  ```
  Default `output_dir`: `~/.nano-brain/summaries`. Tilde is expanded at config load time.

  **Opt out** (DB-only persistence, matching previous behavior since #192):
  ```yaml
  summarization:
    write_to_disk: false
  ```

  **Backfill existing summaries** to disk:
  ```bash
  # Preview
  nano-brain backfill-summaries --dry-run

  # Apply
  nano-brain backfill-summaries

  # Optional filters
  nano-brain backfill-summaries --workspace=nano-brain --since=2026-05-01
  ```

  **Safety properties:**
  - Disk write is atomic (`.tmp` + `os.Rename` on POSIX) — no partial files on crash
  - Disk failure (permissions, disk full) logs a WARN but does NOT roll back the DB transaction (DB is source of truth)
  - Idempotent: re-persisting the same session overwrites the same file
  - Collision-safe: if two sessions produce the same path with different content, the second gets a `_<sha8-of-session-id>` suffix

  **Note for upgrading operators**: After upgrade, summaries will start appearing in `~/.nano-brain/summaries/` automatically. To preserve the previous DB-only behavior, set `write_to_disk: false`. Pre-existing 167 summaries in the DB are not auto-backfilled — run `backfill-summaries` manually if you want them on disk.

- feat(cli): `get`, `tags`, `multi-get` commands — fetch a single document by source_path or UUID, list all tags with counts, and batch-fetch multiple documents in one round-trip; backed by `POST /api/v1/get`, `GET /api/v1/tags` (existing), and `POST /api/v1/multi-get` REST endpoints (#152)
- feat(cli): `--tags=t1,t2,t3` filter on query/search/vsearch — filters results to docs whose tags overlap (PostgreSQL `&&` array op) with the given set; works in --workspace= or --scope=all mode (#160)
- feat(cli): `workspaces remove --workspace=<hash>` — destructive workspace deletion with `--dry-run` preview and `--force` safety gate; backed by `DELETE /api/v1/workspaces/:hash` REST endpoint wrapped in a single transaction (#155)
- feat(cli): wake-up command — pretty/JSON workspace briefing at session start (#151, PR #216)
- feat(cli): `--scope=all` flag on query/search/vsearch for cross-workspace search (#156, PR #217)
- feat(config): `NANO_BRAIN_CONFIG` env var support — precedence `--config` flag > env var > `~/.nano-brain/config.yml`. Enables Docker/k8s deployments to point at a container-specific config without mounting over the host's default.

### Documentation
- docs(roadmap): reconcile ROADMAP.md status with actually shipped features (#214, PR #215)
- docs(readme): document `NANO_BRAIN_CONFIG` env var + Docker example

---

## [2026.5.267] — 2026-05-26

### Features
- feat(cli): `--tags=t1,t2,t3` filter on query/search/vsearch — filters results to docs whose tags overlap (PostgreSQL `&&` array op) with the given set; works in --workspace= or --scope=all mode (#160)
- feat(summarize): session summarization pipeline — LLM-powered structured summaries for harvested sessions
- feat(summarize): map-reduce pipeline with strip, parallel map, hierarchical reduce for sessions up to 1M tokens
- feat(summarize): OpenAI-compatible LLM client with SSE streaming, retry (3x backoff for 429/5xx)
- feat(summarize): persist summaries as `.md` files + embed in vector DB under `session-summary` collection
- feat(summarize): idempotent upsert via `summary://opencode/{id}` source path — unchanged sessions skipped
- feat(summarize): cross-session relationship links (parent/child/sibling) in summary header
- feat(harvest): wire summarizer into OpenCode SQLite + Claude Code harvesters — runs after successful harvest
- feat(config): `summarization` config block — provider_url, api_key, model, max_tokens, concurrency

### Documentation
- docs: add `summarization` config section to README with ai-proxy setup example
- docs: document `NANO_BRAIN_SUMMARIZE_API_KEY` env var

**Install:** `npm install nano-brain@2026.5.267` · `npx nano-brain@latest`

---

## [2026.8.19] — 2026-05-25

### Features
- feat(cli): `--tags=t1,t2,t3` filter on query/search/vsearch — filters results to docs whose tags overlap (PostgreSQL `&&` array op) with the given set; works in --workspace= or --scope=all mode (#160)
- feat: nano-brain v2 — complete greenfield rewrite (Go + PostgreSQL + pgvector)
- feat: interactive first-run setup wizard (closes #43)
- feat(graph): auto-zoom to selected node + neighbors on focus mode
- feat(graph): focus mode on node click + fix node overlap (#37)
- feat(obsidian,db): excludeFolders, frontmatter tags, db:clean --list-only (closes #34)
- feat(obsidian): Obsidian vault integration + fix entity API limit 100→2000 (closes #34)
- feat(kg): scheduled entity extraction job for memory documents (closes #30)
- feat(docs): extend Three.js neural graph to full-page background on all docs pages (closes #28)
- feat(docs): Three.js neural graph animation in hero — nodes + edges + pulse (closes #27)
- feat(docs): custom hover tooltips on MCP tools grid (closes #26)
- feat(docs): CHANGELOG.md + changelog page renders it via marked.js (closes #24)
- feat(docs): add setup guide page — Docker, config, Ollama, MCP connect, verify steps
- feat: add GitHub Pages site — landing, features, changelog, docs (closes #23)

### Bug Fixes
- fix(graph): allow dragging focused nodes — add pointerEvents:none to dimmed dots
- fix(graph): hide edge labels ('call') when edges dimmed in focus mode
- fix(graph): hide edge labels when edges are dimmed in focus mode
- fix(graph): Symbol Call Graph — default individual mode + fix cluster edge filter (closes #38)
- fix(graph): collapse unrelated nodes to dots on focus — no label overlap
- fix(watcher): address Gemini review — streaming body, yaml parser, async fs, ready event
- fix(db): eliminate SQLite corruption root causes — readonly check + RESTART checkpoint
- fix(db): add db:clean command and bootstrap orphan/corruption guards (closes #32, #33)
- fix(kg): raise entity API limit 100→2000 so all entities appear in graph
- fix(kg): start extraction cycle 30s after startup instead of 30min
- fix(kg): startup reindex + fast drain for entity extraction queue
- fix(ci,web): web deps install in CI + fix graph node overlap
- fix(web): favicon 404, search mark rendering, missing CI web build (closes #29)
- fix(docs): Three.js — use r128 UMD CDN, fix overflow, fix init timing
- fix(docs): npx nano-brain mcp + add ai-sandbox-wrapper container setup section (closes #25)
- fix(pages): redirect root index.html → docs/index.html
- fix(docs): correct codebase facts — 30 tools, 32 CLI, 11-stage pipeline, 5 languages, accurate MCP config
- fix(ci): skip GitHub release creation if tag already exists (closes #22)
- fix(ci): skip npm publish if version already exists on registry
- fix(ci): use --legacy-peer-deps for npm ci (tree-sitter peer conflict)

### Documentation
- docs: add npm, CI, license badges to README (closes #21)
- docs(claude): add rule to create GitHub issue before starting any task

### Other
- chore: update CHANGELOG.md for v2026.8.19 [skip ci]
- chore: update CHANGELOG.md for v2026.8.19 [skip ci]
- chore: fix homepage URL to GitHub Pages docs site
- chore: add repository, homepage, and bugs URLs to package.json
- chore: update CHANGELOG.md for v2026.8.19 [skip ci]

**Install:** `npm install nano-brain@2026.8.19` · `npx nano-brain@latest`

---

## [2026.8.19] — 2026-05-16

### Features
- feat(cli): `--tags=t1,t2,t3` filter on query/search/vsearch — filters results to docs whose tags overlap (PostgreSQL `&&` array op) with the given set; works in --workspace= or --scope=all mode (#160)
- feat: interactive first-run setup wizard (closes #43)
- feat(graph): auto-zoom to selected node + neighbors on focus mode
- feat(graph): focus mode on node click + fix node overlap (#37)
- feat(obsidian,db): excludeFolders, frontmatter tags, db:clean --list-only (closes #34)
- feat(obsidian): Obsidian vault integration + fix entity API limit 100→2000 (closes #34)
- feat(kg): scheduled entity extraction job for memory documents (closes #30)
- feat(docs): extend Three.js neural graph to full-page background on all docs pages (closes #28)
- feat(docs): Three.js neural graph animation in hero — nodes + edges + pulse (closes #27)
- feat(docs): custom hover tooltips on MCP tools grid (closes #26)
- feat(docs): CHANGELOG.md + changelog page renders it via marked.js (closes #24)
- feat(docs): add setup guide page — Docker, config, Ollama, MCP connect, verify steps
- feat: add GitHub Pages site — landing, features, changelog, docs (closes #23)

### Bug Fixes
- fix(graph): allow dragging focused nodes — add pointerEvents:none to dimmed dots
- fix(graph): hide edge labels ('call') when edges dimmed in focus mode
- fix(graph): hide edge labels when edges are dimmed in focus mode
- fix(graph): Symbol Call Graph — default individual mode + fix cluster edge filter (closes #38)
- fix(graph): collapse unrelated nodes to dots on focus — no label overlap
- fix(watcher): address Gemini review — streaming body, yaml parser, async fs, ready event
- fix(db): eliminate SQLite corruption root causes — readonly check + RESTART checkpoint
- fix(db): add db:clean command and bootstrap orphan/corruption guards (closes #32, #33)
- fix(kg): raise entity API limit 100→2000 so all entities appear in graph
- fix(kg): start extraction cycle 30s after startup instead of 30min
- fix(kg): startup reindex + fast drain for entity extraction queue
- fix(ci,web): web deps install in CI + fix graph node overlap
- fix(web): favicon 404, search mark rendering, missing CI web build (closes #29)
- fix(docs): Three.js — use r128 UMD CDN, fix overflow, fix init timing
- fix(docs): npx nano-brain mcp + add ai-sandbox-wrapper container setup section (closes #25)
- fix(pages): redirect root index.html → docs/index.html
- fix(docs): correct codebase facts — 30 tools, 32 CLI, 11-stage pipeline, 5 languages, accurate MCP config
- fix(ci): skip GitHub release creation if tag already exists (closes #22)
- fix(ci): skip npm publish if version already exists on registry
- fix(ci): use --legacy-peer-deps for npm ci (tree-sitter peer conflict)

### Documentation
- docs: add npm, CI, license badges to README (closes #21)
- docs(claude): add rule to create GitHub issue before starting any task

### Other
- chore: update CHANGELOG.md for v2026.8.19 [skip ci]
- chore: fix homepage URL to GitHub Pages docs site
- chore: add repository, homepage, and bugs URLs to package.json
- chore: update CHANGELOG.md for v2026.8.19 [skip ci]

**Install:** `npm install nano-brain@2026.8.19` · `npx nano-brain@latest`

---

## [2026.8.19] — 2026-05-16

### Features
- feat(cli): `--tags=t1,t2,t3` filter on query/search/vsearch — filters results to docs whose tags overlap (PostgreSQL `&&` array op) with the given set; works in --workspace= or --scope=all mode (#160)
- feat(graph): auto-zoom to selected node + neighbors on focus mode
- feat(graph): focus mode on node click + fix node overlap (#37)
- feat(obsidian,db): excludeFolders, frontmatter tags, db:clean --list-only (closes #34)
- feat(obsidian): Obsidian vault integration + fix entity API limit 100→2000 (closes #34)
- feat(kg): scheduled entity extraction job for memory documents (closes #30)
- feat(docs): extend Three.js neural graph to full-page background on all docs pages (closes #28)
- feat(docs): Three.js neural graph animation in hero — nodes + edges + pulse (closes #27)
- feat(docs): custom hover tooltips on MCP tools grid (closes #26)
- feat(docs): CHANGELOG.md + changelog page renders it via marked.js (closes #24)
- feat(docs): add setup guide page — Docker, config, Ollama, MCP connect, verify steps
- feat: add GitHub Pages site — landing, features, changelog, docs (closes #23)

### Bug Fixes
- fix(graph): allow dragging focused nodes — add pointerEvents:none to dimmed dots
- fix(graph): hide edge labels ('call') when edges dimmed in focus mode
- fix(graph): hide edge labels when edges are dimmed in focus mode
- fix(graph): Symbol Call Graph — default individual mode + fix cluster edge filter (closes #38)
- fix(graph): collapse unrelated nodes to dots on focus — no label overlap
- fix(watcher): address Gemini review — streaming body, yaml parser, async fs, ready event
- fix(db): eliminate SQLite corruption root causes — readonly check + RESTART checkpoint
- fix(db): add db:clean command and bootstrap orphan/corruption guards (closes #32, #33)
- fix(kg): raise entity API limit 100→2000 so all entities appear in graph
- fix(kg): start extraction cycle 30s after startup instead of 30min
- fix(kg): startup reindex + fast drain for entity extraction queue
- fix(ci,web): web deps install in CI + fix graph node overlap
- fix(web): favicon 404, search mark rendering, missing CI web build (closes #29)
- fix(docs): Three.js — use r128 UMD CDN, fix overflow, fix init timing
- fix(docs): npx nano-brain mcp + add ai-sandbox-wrapper container setup section (closes #25)
- fix(pages): redirect root index.html → docs/index.html
- fix(docs): correct codebase facts — 30 tools, 32 CLI, 11-stage pipeline, 5 languages, accurate MCP config
- fix(ci): skip GitHub release creation if tag already exists (closes #22)
- fix(ci): skip npm publish if version already exists on registry
- fix(ci): use --legacy-peer-deps for npm ci (tree-sitter peer conflict)

### Documentation
- docs: add npm, CI, license badges to README (closes #21)
- docs(claude): add rule to create GitHub issue before starting any task

### Other
- chore: fix homepage URL to GitHub Pages docs site
- chore: add repository, homepage, and bugs URLs to package.json
- chore: update CHANGELOG.md for v2026.8.19 [skip ci]

**Install:** `npm install nano-brain@2026.8.19` · `npx nano-brain@latest`

---

## [2026.8.19] — 2026-05-16

### Features
- feat(cli): `--tags=t1,t2,t3` filter on query/search/vsearch — filters results to docs whose tags overlap (PostgreSQL `&&` array op) with the given set; works in --workspace= or --scope=all mode (#160)
- feat(graph): auto-zoom to selected node + neighbors on focus mode
- feat(graph): focus mode on node click + fix node overlap (#37)
- feat(obsidian,db): excludeFolders, frontmatter tags, db:clean --list-only (closes #34)
- feat(obsidian): Obsidian vault integration + fix entity API limit 100→2000 (closes #34)
- feat(kg): scheduled entity extraction job for memory documents (closes #30)
- feat(docs): extend Three.js neural graph to full-page background on all docs pages (closes #28)
- feat(docs): Three.js neural graph animation in hero — nodes + edges + pulse (closes #27)
- feat(docs): custom hover tooltips on MCP tools grid (closes #26)
- feat(docs): CHANGELOG.md + changelog page renders it via marked.js (closes #24)
- feat(docs): add setup guide page — Docker, config, Ollama, MCP connect, verify steps
- feat: add GitHub Pages site — landing, features, changelog, docs (closes #23)

### Bug Fixes
- fix(graph): allow dragging focused nodes — add pointerEvents:none to dimmed dots
- fix(graph): hide edge labels ('call') when edges dimmed in focus mode
- fix(graph): hide edge labels when edges are dimmed in focus mode
- fix(graph): Symbol Call Graph — default individual mode + fix cluster edge filter (closes #38)
- fix(graph): collapse unrelated nodes to dots on focus — no label overlap
- fix(watcher): address Gemini review — streaming body, yaml parser, async fs, ready event
- fix(db): eliminate SQLite corruption root causes — readonly check + RESTART checkpoint
- fix(db): add db:clean command and bootstrap orphan/corruption guards (closes #32, #33)
- fix(kg): raise entity API limit 100→2000 so all entities appear in graph
- fix(kg): start extraction cycle 30s after startup instead of 30min
- fix(kg): startup reindex + fast drain for entity extraction queue
- fix(ci,web): web deps install in CI + fix graph node overlap
- fix(web): favicon 404, search mark rendering, missing CI web build (closes #29)
- fix(docs): Three.js — use r128 UMD CDN, fix overflow, fix init timing
- fix(docs): npx nano-brain mcp + add ai-sandbox-wrapper container setup section (closes #25)
- fix(pages): redirect root index.html → docs/index.html
- fix(docs): correct codebase facts — 30 tools, 32 CLI, 11-stage pipeline, 5 languages, accurate MCP config
- fix(ci): skip GitHub release creation if tag already exists (closes #22)
- fix(ci): skip npm publish if version already exists on registry
- fix(ci): use --legacy-peer-deps for npm ci (tree-sitter peer conflict)

### Documentation
- docs: add npm, CI, license badges to README (closes #21)
- docs(claude): add rule to create GitHub issue before starting any task

**Install:** `npm install nano-brain@2026.8.19` · `npx nano-brain@latest`

---

## [2026.8.19] — 2026-05-15

### Bug Fixes

- **Workspace guard** — server no longer creates a database for `--root` paths not declared in `config.workspaces`. Falls back to closest configured workspace via longest-prefix match, then first workspace. Prevents orphaned DB accumulation. Closes [#19](https://github.com/nano-step/nano-brain/issues/19).
- **CLI pre-resolution** — `cli/index.ts` no longer pre-resolves the DB path from `process.cwd()` for the `mcp` command, which was bypassing the workspace guard entirely.
- **CI: skip existing release** — `Publish Stable` workflow no longer fails when the GitHub release tag already exists. Closes [#22](https://github.com/nano-step/nano-brain/issues/22).
- **CI: peer deps** — use `--legacy-peer-deps` in GitHub Actions to resolve `tree-sitter` peer conflict.

### Infrastructure

- Added `develop` branch → `npm publish --tag beta` + GitHub pre-release on every push.
- Added `master` branch → `npm publish --tag latest` + GitHub release with auto-generated changelog.
- Added npm, CI, and license badges to README.
- CLAUDE.md: every npm publish must have a changelog; create GitHub issue before starting any task.

**Install:** `npm install nano-brain@2026.8.19` · `npx nano-brain@latest`

---

## [2026.6.2] — 2026-03-17

### Features — Memory Intelligence v2

- **Entity Pruning** — background job (6h interval) soft-deletes contradicted entities after 30 days and orphan entities after 90 days; hard-deletes after 30-day retention.
- **LLM Categorization** — async fire-and-forget assigns `llm:` category tags after every `memory_write`.
- **Preference Learning** — Thompson Sampling bandits track which content types the agent retrieves most; RRF blend weights adapt over time.
- **Schema v7** — `pruned_at` column on `memory_entities`.

### Features — Search Quality

- **Query Expansion** — LLM generates 2–3 query variants before search for better recall.
- **Tag Display** — `auto:` and `llm:` tags now visible in search results (verbose + compact modes).
- **Backfill CLI** — `nano-brain categorize-backfill` to LLM-categorize existing documents.
- Wave 7: recency boost for sessions/memory collections.
- Wave 6: length normalization penalty using `charLength`.
- Wave 5: temporal metadata (`createdAt`) in `SearchResult`.
- Wave 4: Qdrant `project_hash` payload filter and backfill.
- Wave 3: FTS workspace isolation (strict filter).
- Wave 2: fix `supersedeDocument` bug.
- Wave 1: `domain_type` and `last_reinforced_at` schema columns.

### Features — Token Reduction

- **MCP Response Caps** — hard limits on all unbounded tools: `memory_get` (200 lines), `code_impact` (depth 3, 50 entries), `code_context` (20 callers/callees).
- **Compact mode default** — search tools return compact format (~60% fewer tokens).
- Heading-aware chunking (900 tokens, 15% overlap).

### Infrastructure

- Replaced sqlite-vec with Qdrant as sole vector store.
- Added benchmark suite — data generator, runner, compare CLI.
- Consolidation decisions now reconciled automatically in the background.

**Install:** `npm install nano-brain@2026.6.2`

---

## Earlier releases

Full history available on [GitHub Releases](https://github.com/nano-step/nano-brain/releases).
