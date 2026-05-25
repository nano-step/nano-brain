## 1. Store Layer

- [ ] 1.1 Add `getTopAccessedDocuments()` to Store interface in types.ts
- [ ] 1.2 Add `getRecentDocumentsByTags()` to Store interface in types.ts
- [ ] 1.3 Implement both methods in store.ts with prepared statements

## 2. Core Module

- [ ] 2.1 Create src/wake-up.ts with BriefingResult type and generateBriefing() function

## 3. Surfaces

- [ ] 3.1 Add handleWakeUp() to src/index.ts CLI handler
- [ ] 3.2 Add memory_wake_up MCP tool to src/server.ts
- [ ] 3.3 Add GET/POST /api/wake-up HTTP routes to src/server.ts

## 4. Testing & Validation

- [ ] 4.1 Add test/wake-up.test.ts with unit tests
- [ ] 4.2 Validate with `openspec validate "wake-up-context-briefing" --strict --no-interactive`
