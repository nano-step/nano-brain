<!-- OPENCODE-MEMORY:START -->
<!-- Managed block - do not edit manually. Updated by: npx @nano-step/nano-brain init -->

## Memory System (nano-brain)

This project uses **nano-brain** for persistent context across sessions.

### Quick Reference

All commands use the nano-brain CLI:

| I want to... | CLI |
|--------------|-----|
| Recall past work on a topic | `npx @nano-step/nano-brain query "topic"` |
| Find exact error/function name | `npx @nano-step/nano-brain search "exact term"` |
| Explore a concept semantically | `npx @nano-step/nano-brain vsearch "concept"` |
| Save a decision for future sessions | `npx @nano-step/nano-brain write "decision context" --tags=decision` |
| Check index health | `npx @nano-step/nano-brain status` |

### Session Workflow

**End of session:** Save key decisions, patterns discovered, and debugging insights.
```
npx @nano-step/nano-brain write "## Summary\n- Decision: ...\n- Why: ...\n- Files: ..." --tags=summary
```

### Code Intelligence Tools

nano-brain also provides symbol-level code analysis (requires `npx @nano-step/nano-brain reindex` with `workdir` set to the workspace — the `--root` flag is silently ignored):

| I want to... | CLI |
|--------------|-----|
| Understand a symbol's callers/callees/flows | `npx @nano-step/nano-brain context functionName` |
| Assess risk of changing a symbol | `npx @nano-step/nano-brain code-impact className --direction=upstream` |
| Map my git changes to affected symbols | `npx @nano-step/nano-brain detect-changes --scope=all` |

Use `file_path` parameter to disambiguate when multiple symbols share the same name.

### When to Search Memory vs Codebase vs Code Intelligence

- **"Have we done this before?"** → `npx @nano-step/nano-brain query "..."` (searches past sessions)
- **"Where is this in the code?"** → grep / ast-grep (searches current files)
- **"How does this concept work here?"** → Both (memory for past context + grep for current code)
- **"What calls this function?"** → `npx @nano-step/nano-brain context <name>` (symbol graph relationships)
- **"What breaks if I change X?"** → `npx @nano-step/nano-brain code-impact <name> --direction=...` (dependency + flow analysis)
- **"What did my changes affect?"** → `npx @nano-step/nano-brain detect-changes --scope=...` (git diff to symbol mapping)

<!-- OPENCODE-MEMORY:END -->
