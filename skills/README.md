# Agent Skills

This directory contains [skills](https://docs.claude.com/en/docs/claude-code/skills) that teach AI coding agents (Claude Code, OpenCode, MCP-aware tools) how to use nano-brain effectively.

## Available

| Skill | Purpose |
|---|---|
| [`nano-brain/`](./nano-brain/SKILL.md) | When + how to call nano-brain (MCP, CLI, HTTP) with best-practice recipes |

## Install

Each skill is a self-contained directory. Copy it into your agent's skills root — see the individual `SKILL.md` for per-agent install instructions.

```bash
# Example: install the nano-brain skill into OpenCode (user-level)
cp -r skills/nano-brain ~/.config/opencode/skills/
```

## Contribute

To add a new skill or update an existing one:

1. Branch off `master`
2. Edit `skills/<name>/SKILL.md` (and `references/*.md` if multi-file)
3. Bump `metadata.version` in the YAML frontmatter following semver
4. Open a PR — CI will verify YAML frontmatter parses

Skills follow the `name + description + body` contract — frontmatter `description` is the trigger string an agent matches against to decide when to load the skill, so keep it specific and discoverable.
