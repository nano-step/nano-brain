---
description: Cancel the active harness loop
---

Cancel the currently active harness loop. Clears the loop state, cancels any running background watcher subagent, and emits a summary toast showing which gate was active and at what iteration.

**Usage**

```
/harness-off
```

No flags. If no loop is active, shows an info toast and exits cleanly.

**What happens**

1. Reads `.opencode/harness-loop.local.json`
2. If `loop.watcher_task_id` is set, calls `background_cancel` on the watcher subagent
3. Clears the `loop` block in state (preserves `checkpoints` for post-mortem inspection)
4. Shows a toast: `🛑 Harness loop cancelled at gate "<name>" (iteration N)`

**When to use**

- You want to stop the loop mid-run and take over manually
- A gate is stuck and you need to intervene
- You want to restart with different flags (cancel then `/harness-on --force`)
