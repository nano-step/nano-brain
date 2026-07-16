## Context

`nano-brain init` already has a rich interactive path for writing config and registering a workspace, but the current parser only understands the existing flag set. The issue asks for a friendlier OpenCode-first setup path that can be invoked as `npx @nano-step/nano-brain init -- opencode`, and the repo already contains project-local command/skill patterns under `.opencode/`.

The important constraints are:

- Keep the existing `init`, `init --yes`, and `init --root` behavior unchanged.
- The install experience must be workspace-local, not a mutation of `~/.nano-brain/config.yml`.
- OpenCode is the first-class target because the requested UX explicitly ends with `/nano-brain .`.

## Goals / Non-Goals

**Goals:**

- Add an agent-targeted install mode to `nano-brain init` using the `--` separator.
- Install a project-local OpenCode slash command in the workspace.
- Make the installed command steer the user toward a workspace-local `.nanobrain/config.yml`.
- Keep install behavior idempotent and safe to rerun.

**Non-Goals:**

- Reworking the daemon config loader to merge home and workspace config files at runtime.
- Changing the current interactive init wizard beyond what is needed to support the new install entrypoint.
- Introducing new external dependencies or a new packaging system.

## Decisions

1. **Use `--` as the install-mode boundary.**
   - `nano-brain init -- opencode` is unambiguous and preserves existing `init` flags.
   - Alternative considered: a new explicit flag like `--agent`. Rejected for now because the issue already names the `--` form and the separator cleanly distinguishes install-target tokens from current flags.

2. **Install OpenCode assets into the workspace, not the home directory.**
   - The command template will be written to `.opencode/commands/nano-brain.md` in the current project.
   - The command file is the user-facing bootstrap surface; it keeps the setup discoverable inside the workspace where the agent is already operating.
   - Alternative considered: a user-global OpenCode command under the agent home directory. Rejected because the issue explicitly asks for workspace-local setup and the repo already favors project-local agent artifacts.

3. **Reuse the existing init wizard for the local config path.**
   - The installed OpenCode command will direct the agent to invoke the existing init flow against `.nanobrain/config.yml`.
   - If `~/.nano-brain/config.yml` exists, the command instructions will seed the workspace-local file from it before prompting so the new local config starts from the user's current baseline.
   - Alternative considered: add runtime config layering in `internal/config`. Rejected for this change because it widens the blast radius and is not needed to deliver the user-facing install flow.

4. **Generate the command from an embedded template.**
   - The binary will carry the command text in-tree so the npm-distributed executable can install it without depending on repo checkout files.
   - This keeps installation deterministic and avoids a new packaging dependency.

## Risks / Trade-offs

- [Risk] The installed slash command is still an instruction file executed by the agent, so correctness depends on the agent following the template faithfully. → Mitigation: keep the template short, explicit, and rooted in the existing `nano-brain init` flow.
- [Risk] The workspace-local command is project-scoped, so each workspace needs its own install step. → Mitigation: that is intentional for per-project setup; rerunning the install is idempotent.
- [Risk] Seeding `.nanobrain/config.yml` from the home config is a copy-on-install, not a live merge. → Mitigation: document the behavior clearly and keep the local file as the workspace source of truth.

## Migration Plan

- No database or server migration is required.
- Users rerun `nano-brain init -- opencode` in any workspace that needs the new slash command.
- Rollback is manual and local: delete the generated `.opencode/commands/nano-brain.md` and the workspace-local `.nanobrain/config.yml` if they want to revert to the previous setup model.

## Open Questions

- Should the same install syntax be expanded to Claude Code and Codex in this change, or should those remain on the existing MCP-config path for now?
