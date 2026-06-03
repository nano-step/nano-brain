<!-- OPENCODE-MEMORY:START -->
<!-- Managed block - do not edit manually. Updated by: npx nano-brain init -->

## Memory System (nano-brain)

This project uses **nano-brain** for persistent context across sessions.

### Quick Reference

All commands use the nano-brain CLI:

| I want to... | CLI |
|--------------|-----|
| Recall past work on a topic | `npx nano-brain query "topic"` |
| Find exact error/function name | `npx nano-brain search "exact term"` |
| Explore a concept semantically | `npx nano-brain vsearch "concept"` |
| Save a decision for future sessions | `npx nano-brain write "decision context" --tags=decision` |
| Check index health | `npx nano-brain status` |

### Session Workflow

**End of session:** Save key decisions, patterns discovered, and debugging insights.
```
npx nano-brain write "## Summary\n- Decision: ...\n- Why: ...\n- Files: ..." --tags=summary
```

### Code Intelligence Tools

nano-brain also provides symbol-level code analysis (requires a prior `npx nano-brain reindex` — invoke with `workdir` set to the workspace root, OR pass `--root=<path> --workspace=<hash>` explicitly):

| I want to... | CLI |
|--------------|-----|
| Understand a symbol's callers/callees/flows | `npx nano-brain context functionName` |
| Assess risk of changing a symbol | `npx nano-brain code-impact className --depth=2` |
| Map my staged git changes to affected symbols | `npx nano-brain detect-changes --staged` |
| Map ALL changed files to affected symbols | `npx nano-brain detect-changes --all` |

### When to Search Memory vs Codebase vs Code Intelligence

- **"Have we done this before?"** → `npx nano-brain query "..."` (searches past sessions)
- **"Where is this in the code?"** → grep / ast-grep (searches current files)
- **"How does this concept work here?"** → Both (memory for past context + grep for current code)
- **"What calls this function?"** → `npx nano-brain context <name>` (symbol graph relationships)
- **"What breaks if I change X?"** → `npx nano-brain code-impact <name> --depth=2` (reverse-impact BFS)
- **"What did my staged changes affect?"** → `npx nano-brain detect-changes --staged` (git diff to symbol mapping)

<!-- OPENCODE-MEMORY:END -->
