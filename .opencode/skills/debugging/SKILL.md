# Debugging Skill

Use `mode=debugging` on `memory_search` or `memory_query` to search code, sessions, and config in a single call.

## When to Use

Detect debugging intent from user queries like:
- "Why is X broken?"
- "Payment has wrong tax"
- "Trade stuck / not executing"
- "Error in Y service"
- "What's causing Z to fail?"

## Workflow

1. **Search** — call `memory_search(query="<user query>", mode="debugging")`
2. **Read source labels** — each result has a `source` field: `"code"`, `"session"`, or `"config"`
3. **Prioritize by source:**
   - `source=code` — error paths, stack traces, code logic
   - `source=session` — past debugging history, what was tried before
   - `source=config` — threshold values, TTLs, feature flags
4. **Dig deeper if needed:**
   - `memory_graph(node="<function>", direction="in")` — who calls this function
   - `memory_impact(node="<symbol>", max_depth=2)` — what breaks if changed
   - `memory_get(path="<file>")` — read full file content

## Example

```
User: "stripe payment has wrong tax calculated"

Step 1: memory_search(query="stripe payment wrong tax", mode="debugging")
  → Returns results with source labels

Step 2: Check source=code results for tax calculation logic
Step 3: Check source=session for past debugging sessions about tax
Step 4: Check source=config for tax-related config values

Step 5: If needed:
  memory_graph(node="internal/tax/calculate.go::CalculateTax", direction="in")
  → Find all callers of the tax function
```

## Source Interpretation

| Source | What it means | Look for |
|--------|--------------|----------|
| `code` | Indexed codebase files | Error handling, logic paths, function signatures |
| `session` | Harvested AI sessions | Past debugging attempts, decisions made |
| `config` | Memory/config documents | Thresholds, feature flags, environment values |
