## Why

The CFG extractor's `buildIf` function drops the continuation path when an `if` statement has no `else` block. This means guard clauses with early returns cause all subsequent code in the function to be disconnected from the CFG.

Example from `tradeit-backend/server/controllers/user.js::setUserEmail`:

```js
if (rules.email(email) === 'Invalid e-mail.' || !steamId) {
  return res.json({ success: false, message: 'Invalid Email' })
}
// ALL THIS CODE IS DISCONNECTED FROM CFG:
const existEmail = await userRepo.getUserByEmail(email)
await userRepo.updateEmail(steamId, email)  // THE ACTUAL DB UPDATE
return res.success()
```

The CFG only shows: start → try → steamId extraction → validation check → return error. The DB update, email check, commit — all missing.

## What Changes

- Fix `buildIf` in `js_cflow.go`: move `relabelPreds` inside `if alternative != nil` so the no-else fall-through path is preserved
- Add tests for guard-clause patterns with continuation code
- Follow the same pattern already used by `buildTry` for no-catch case

## Capabilities

### Modified Capabilities

- `cfg-branching`: `buildIf` now preserves continuation paths for `if` statements without `else` blocks

## Impact

- `internal/graph/js_cflow.go` — 1 line moved in `buildIf`
- `internal/graph/js_cflow_test.go` — new tests for guard-clause patterns
