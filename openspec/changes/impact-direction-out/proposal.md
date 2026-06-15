# Proposal: Add direction="out" to memory_impact

**Tracking:** #419

## Intent

Add `direction` parameter to `memory_impact` MCP tool to support traversing outbound dependencies (what a node calls/imports), in addition to the existing inbound traversal (who calls/imports a node).

## Problem

`memory_impact` currently only traverses **inbound** edges: given a node, it returns what depends on that node (who calls/imports it). There is no way to traverse **outbound** edges: what does this node call or import?

This gap makes `memory_impact` blind to a common agent use case — specifically **code review of new files**.

### Use case: reviewing new code

When an agent reviews a PR that adds a **new file**, querying memory for the new file itself returns nothing (it has no history). The review value is in the **existing modules the new file calls** — those have bugs, incidents, and decisions recorded.

Current workaround: the agent must grep/read the file's imports manually, then issue separate `memory_query` calls for each callee. This is slow, error-prone, and not expressible as a single tool call.

With `direction: "out"`, the agent can do:

```json
{
  "tool": "memory_impact",
  "params": {
    "workspace": "...",
    "node": "server/service/payoutSecurityService.js",
    "direction": "out",
    "max_depth": 1
  }
}
```

And get back the callees:
- `server/repository/saleRevenuePayoutRepo.js`
- `server/service/payment/nowPaymentService.js`
- `server/service/slackService.js`
- `server/service/userService.js`

The agent then queries memory for each of these to surface relevant prior history — all in tool calls, no manual grep.

## Proposed Change

Add `direction` parameter to `memory_impact`:

| Value | Behaviour | Current? |
|---|---|---|
| `"in"` (default) | Who calls/imports this node | ✅ existing |
| `"out"` | What this node calls/imports | ❌ missing |
| `"both"` | Union of both directions | ❌ missing |

Return schema stays the same. `direction: "in"` remains the default so existing callers are unaffected.

## Relationship to Existing Issues

- **#378** (reverse import edges): that issue is about *storing* reverse edges in the graph. This issue is about *querying* forward edges that should already exist when a file is indexed.
- **#382** (multi-file impact): orthogonal — that issue is about accepting multiple input nodes. This is about direction of traversal.

## Why This Is the Right Primitive

Agents don't need a high-level "review mode" composite tool. They need the right graph primitive and the right instructions. `direction: "out"` is minimal, composable, and unlocks the new-file review pattern without adding complexity elsewhere.
