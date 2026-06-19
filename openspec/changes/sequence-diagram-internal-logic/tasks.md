## 1. CFG Loading

- [ ] 1.1 Add function to load CFG from `function_flowcharts` table for a given entry point (file + function name)
- [ ] 1.2 Add function to resolve entry point from flow entry (e.g., `POST /purchase` → `trade.js::purchaseHandler`)
- [ ] 1.3 Integrate CFG loading into `RenderSequenceDiagram` — load CFG when available, skip when not

## 2. Internal Logic Rendering

- [ ] 2.1 Implement `renderInternalLogic` function that walks CFG nodes and emits self-messages for step nodes
- [ ] 2.2 Map CFG decision nodes to Mermaid `alt`/`opt` blocks (yes/no → alt, single → opt)
- [ ] 2.3 Map CFG loop edges to Mermaid `loop` blocks
- [ ] 2.4 Render terminal nodes as return/self-message
- [ ] 2.5 Emit middleware guard notes from flow edges (already implemented — verify compatibility)

## 3. Depth & Size Limits

- [ ] 3.1 Add max depth parameter (default 3) to `renderInternalLogic`
- [ ] 3.2 Add max message count (default 50) with truncation note
- [ ] 3.3 Emit truncation note: "Internal logic too complex — see full CFG at /api/v1/graph/flowchart"

## 4. Testing & Verification

- [ ] 4.1 Add unit tests for `renderInternalLogic` with simple CFG (start → step → terminal)
- [ ] 4.2 Add unit test for alt/loop block rendering
- [ ] 4.3 Add unit test for depth truncation
- [ ] 4.4 Add unit test for missing CFG (fallback to cross-actor only)
- [ ] 4.5 Run `go test -race -short ./...`
- [ ] 4.6 E2E test: `POST /purchase` sequence diagram shows internal logic
- [ ] 4.7 Run `go test -race -tags=integration ./...`
