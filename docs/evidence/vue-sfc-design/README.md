# Vue SFC Code Intelligence Support — Design Pipeline

**Date**: 2026-06-26
**Status**: Synthesis done + **codebase-verified** (claims checked against gotreesitter v0.19.1 via live parse; key "language injection" unknown resolved). Pending Phase 2.5 + Phase 3.

> Verification pass (2026-06-26): confirmed `VueLanguage()` emits `template_element`/`script_element`/`style_element` with script body as a single `raw_text`. No language injection → two-pass extraction mandatory. Component detection switched from regex to AST `tag_name`. Resolved universal-vs-`detectVue` contradiction. Added fixture list, `lang` matrix, CFG-reindex/chunker gaps.

## Files

| File | Contents |
|------|----------|
| `design-brief.md` | Architecture decisions, conflict resolution, risks |
| `pending-decisions.md` | 3 questions that need user input |
| `research-findings.md` | What agents actually need from nano-brain |
| `implementation-plan.md` | Phased implementation with priorities |

## Quick Links

- [Pending Decisions](pending-decisions.md) — What you need to decide
- [Design Brief](design-brief.md) — Settled architecture decisions
- [Research Findings](research-findings.md) — Evidence-based recommendations
