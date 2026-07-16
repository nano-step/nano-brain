## 1. Agent install plumbing

- [x] 1.1 Extend `nano-brain init` parsing so `init -- <agent>` is recognized as an install target instead of an unknown flag
- [x] 1.2 Add the OpenCode install path that writes a workspace-local `.opencode/commands/nano-brain.md` from an embedded template
- [x] 1.3 Ensure the install path is idempotent and leaves `~/.nano-brain/config.yml` untouched

## 2. Verification

- [x] 2.1 Add unit coverage for supported-target dispatch, unsupported-target rejection, and command-file content
- [x] 2.2 Run the Go test suite covering the changed command/config paths

## 3. Docs

- [x] 3.1 Update the setup guide and user-facing docs to describe the new `npx @nano-step/nano-brain init -- opencode` flow
