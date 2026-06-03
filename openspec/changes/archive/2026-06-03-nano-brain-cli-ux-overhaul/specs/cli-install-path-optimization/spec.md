## ADDED Requirements

### Requirement: `npm/postinstall.js` SHALL support opt-in placement of the binary in a PATH-visible directory

After successfully downloading and verifying the platform binary, `npm/postinstall.js` SHALL check for an opt-in marker (`NANO_BRAIN_AUTO_LINK=1` env var OR `~/.nano-brain/auto-link` marker file). When the marker is present, the postinstall script SHALL copy (NOT symlink) the binary to a platform-specific PATH-visible directory and print guidance. Without the marker, postinstall behaviour is unchanged.

#### Scenario: Opt-in via env var on Linux
- **WHEN** `NANO_BRAIN_AUTO_LINK=1` is set in the environment AND the user runs `npm install -g @nano-step/nano-brain` on Linux AND `~/.local/bin/` exists or can be created AND `~/.local/bin/nano-brain` does not already exist
- **THEN** the binary is copied (not symlinked) from the npm install location to `~/.local/bin/nano-brain` with mode 0755 AND postinstall prints `Copied nano-brain to ~/.local/bin/nano-brain. Ensure ~/.local/bin is in your PATH.` AND postinstall exits 0

#### Scenario: Opt-in via marker file on macOS
- **WHEN** `~/.nano-brain/auto-link` marker file exists (any content) AND the user runs `npm install -g @nano-step/nano-brain` on macOS AND `~/Library/nano-brain/bin/` can be created AND target does not exist
- **THEN** the binary is copied to `~/Library/nano-brain/bin/nano-brain` with mode 0755 AND postinstall prints PATH guidance referencing `~/Library/nano-brain/bin`

#### Scenario: Existing target file is never overwritten
- **WHEN** opt-in is enabled (env or marker) AND `~/.local/bin/nano-brain` already exists (regular file, symlink, or anything else)
- **THEN** postinstall prints `WARN: ~/.local/bin/nano-brain already exists; skipping auto-link to preserve your existing file.` AND does NOT touch the existing file AND postinstall exits 0

#### Scenario: Windows skip
- **WHEN** the postinstall runs on Windows (`process.platform === "win32"`) regardless of opt-in marker
- **THEN** postinstall prints `INFO: Auto-link is not supported on Windows; nano-brain binary remains at <npm-install-path>.` AND no copy is attempted

#### Scenario: Opt-out is the default
- **WHEN** neither `NANO_BRAIN_AUTO_LINK=1` nor `~/.nano-brain/auto-link` is present AND the user runs `npm install -g @nano-step/nano-brain` on any platform
- **THEN** postinstall does NOT copy the binary anywhere outside the npm install location AND no PATH guidance is printed AND postinstall exits 0

#### Scenario: Target directory created if missing
- **WHEN** opt-in is enabled AND target parent directory (`~/.local/bin/` on Linux, `~/Library/nano-brain/bin/` on macOS) does not exist
- **THEN** postinstall creates the directory recursively with mode 0755 before copying

#### Scenario: Copy failure is non-fatal
- **WHEN** opt-in is enabled AND the copy fails (permission denied, disk full, etc.)
- **THEN** postinstall prints `WARN: failed to auto-link binary: <error message>. Binary remains at <npm-install-path>.` AND postinstall exits 0 (does not break the npm install)

#### Scenario: PATH guidance is informational only
- **WHEN** postinstall successfully copies the binary
- **THEN** the printed PATH guidance is a single human-readable line; the postinstall MUST NOT modify the user's `.bashrc`, `.zshrc`, `.profile`, or any other shell configuration file

### Requirement: Auto-linked binary SHALL be reported by `nano-brain version --which`

When the user runs `nano-brain version --which` against a binary that was auto-linked via this feature, the reported `path` field SHALL be the auto-linked path (e.g. `~/.local/bin/nano-brain`), and `source` SHALL be the appropriate category (`npm-global` if installed into a global npm prefix, else `path`).

#### Scenario: Linked binary reports its actual location
- **WHEN** opt-in auto-link is enabled AND the user later runs `nano-brain version --which` and the running binary is `~/.local/bin/nano-brain`
- **THEN** stdout `path:` field is the absolute path `~/.local/bin/nano-brain` (with `~` expanded)
