# Harness Gate Instructions

This directory contains per-gate protocol documents for the harness loop plugin.

## How It Works

When the harness loop detects a gate failure, it injects a continuation prompt that includes:
1. A reference to the gate's instruction doc (this directory)
2. Any configured skills for the gate
3. The specific failure details from the runner

The agent should read the instruction doc first to understand the project-specific protocol before attempting fixes.

## Gate Documents

| Gate | Purpose |
|------|---------|
| [pre-work.md](pre-work.md) | Ready to start a new feature |
| [in-progress.md](in-progress.md) | Development is on track |
| [pre-merge.md](pre-merge.md) | Final checks before merge |
| [post-merge.md](post-merge.md) | Post-merge cleanup |
| [next-ready.md](next-ready.md) | Ready to start next feature |

## Document Structure

Each gate doc follows this structure:

1. **Hard Rules** — Non-negotiable requirements
2. **Step-by-Step Procedure** — How to verify/fix
3. **Evidence Requirements** — What proof is needed
4. **FAIL Conditions** — What triggers a failure and how to fix it

## Configuration

Gates are configured in `.opencode/harness.config.json`:

```json
{
  "gate_instructions": {
    "pre-merge": {
      "doc": "docs/harness/gates/pre-merge.md",
      "skills": ["review-work"]
    }
  }
}
```

If `doc` is omitted, the plugin tries `docs/harness/gates/<gate>.md` by convention.

## Cross-References

These docs provide project-specific instructions. For the canonical gate specifications, see:
- [docs/HARNESS.md](../../HARNESS.md) — Full harness process
- [docs/HARNESS_GATES.md](../../HARNESS_GATES.md) — Gate specifications
