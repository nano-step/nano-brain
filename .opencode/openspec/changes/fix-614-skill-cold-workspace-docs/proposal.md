# Update skill cold workspace diagnostics for current API behavior (#614)

## Why

Distributed `nano-brain` skill docs (`SKILL.md` and `references/*.md` under `.opencode/skills/nano-brain/` and `skills/nano-brain/`) still describe pre-fix cold-workspace symptoms or omit unified cold-workspace diagnostics — causing agents to misinterpret unregistered workspaces, asynchronous watcher ingestion (`chunks_enqueued: 0` on incremental `/api/v1/reindex`), graph/symbol re-extraction (`/api/v1/update`), and empty impact responses (`impacted: []` + `node_found`).

## What Change

- Add and align a code-verified **Cold workspace diagnostics** reference across `.opencode/skills/nano-brain/SKILL.md` and `skills/nano-brain/SKILL.md`, and sync surrounding tool/HTTP contracts in `.opencode/skills/nano-brain/references/{http-api.md,code-intelligence.md}`, `skills/nano-brain/references/{http-api.md,code-intelligence.md}`, and `skills/nano-brain/AGENTS_SNIPPET.md`.
- Document exact current server behavior verified on `origin/master` (#309, #161, #622/#694, #584/#705, #703/#708, #304, #625/#692) and fix stale CLI `init --root <path>` flag examples (#623).
- Record the fix in `CHANGELOG.md` under `[Unreleased] -> ### Fixed`.
