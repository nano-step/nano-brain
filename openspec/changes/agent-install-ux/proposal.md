## Why

The current setup flow still makes users stitch together agent-specific install steps by hand, and `nano-brain init -- <agent>` is not a supported UX. OpenCode users in particular have to edit global config today; this change moves the setup experience toward a workspace-local install path that is easier to repeat and less likely to leak settings across projects.

## What Changes

- Add an agent-aware `init` install mode that accepts an agent target after `--` and installs the matching setup surface instead of treating the token as an unknown flag.
- Add a workspace-local OpenCode bootstrap path that writes setup state under the project root, including a `.nanobrain/config.yml` overlay or equivalent merged output, rather than forcing edits to `~/.nano-brain/config.yml`.
- Preserve existing `nano-brain init` behavior for the current interactive and `--yes` paths.
- Update the setup docs and generated agent instructions so the new install flow is the documented path for OpenCode and other supported agents.

## Capabilities

### New Capabilities
- `agent-install-ux`: agent-aware install flow for `nano-brain init -- <agent>` plus workspace-local setup artifacts.

### Modified Capabilities
- None

## Impact

- `cmd/nano-brain/commands.go` and related init helpers for parsing the new install mode.
- `cmd/nano-brain/*` support code and tests for agent-specific setup outputs.
- `internal/config` if workspace-local config layering is implemented directly in the daemon config loader.
- `docs/SETUP_AGENT.md`, `README.md`, and the shipped agent instruction files under `skills/` and `.opencode/`.
- Packaging for the npm-distributed binary if the installer needs embedded setup templates or command assets.
