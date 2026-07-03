# Phase 14: curl|bash install script - Context

**Gathered:** 2026-07-03
**Status:** Implemented (lean phase — single script + release wiring + docs)

<domain>
## Phase Boundary

Add a `curl | bash` one-line installer as the **primary** install path, alongside the existing npm package (kept as an alternative). The installer downloads the prebuilt platform binary from GitHub Releases, verifies its SHA-256 against the release's `SHA256SUMS`, and installs it onto PATH — no Node.js or Go toolchain required.

Reuses the artifacts the release pipeline already publishes (`nano-brain-{darwin,linux}-{amd64,arm64}` + `SHA256SUMS`); no change to the binary build matrix.

</domain>

<decisions>
## Implementation Decisions

- **D-01:** Keep npm — curl|bash is additive, not a replacement. npm suits users already in a JS/agent toolchain (npx assumptions). README/SETUP_AGENT put curl|bash first, npm + build-from-source as alternatives.
- **D-02:** `install.sh` at repo root. Canonical URL `https://raw.githubusercontent.com/nano-step/nano-brain/master/install.sh`; also published as a release asset (`releases/latest/download/install.sh`) via `release.yml` so both URLs resolve.
- **D-03:** Mandatory SHA-256 verification against the release `SHA256SUMS` before install — mismatch or missing sums file aborts, never installs an unverified binary. Supports both `sha256sum` (Linux) and `shasum -a 256` (macOS).
- **D-04:** Platform detection via `uname` → `darwin/linux` × `amd64/arm64`; unsupported platform prints a build-from-source hint and exits. No Windows (no bash; daemon is Unix-only anyway).
- **D-05:** Install dir precedence: `$NANO_BRAIN_BIN_DIR` → `/usr/local/bin` if writable → `~/.local/bin` (warn if not on PATH). `NANO_BRAIN_VERSION` pins a release tag (default `latest`).
- **D-06:** Version display uses `nano-brain version` (the actual subcommand — `--version` is not a defined flag; caught during real smoke testing).
- **D-07:** `tmp` is a global var with an EXIT-trap cleanup (a `main()`-local would be unbound in the trap's global scope under `set -u` — caught during real smoke testing).

</decisions>

<code_context>
## Files

- `install.sh` (new) — the installer
- `.github/workflows/release.yml` — checkout + stage install.sh into release assets
- `README.md` — Install section leads with curl|bash; Start section is `nano-brain init`
- `docs/SETUP_AGENT.md` — Step 5 leads with curl|bash (Node only for npm path), fixes `--version` → `version`

## Evidence (real, on this machine — darwin-arm64, against live release v2026.7.0201)

- `bash -n install.sh` — syntax OK
- Clean install into isolated `NANO_BRAIN_BIN_DIR` → binary runs `nano-brain version` = v2026.7.0201, exit 0
- Version pin `NANO_BRAIN_VERSION=v2026.7.0104` → installs that exact version
- Checksum mismatch → aborts, no install
- Bad/nonexistent version (404) → aborts cleanly, no file left behind
- PATH-not-included → prints the export hint

</code_context>

<deferred>
## Deferred Ideas

- Windows PowerShell one-liner installer (daemon is Unix-only today)
- Homebrew tap / formula
- `shellcheck` in CI (not installed locally; script written to shellcheck-clean conventions)

</deferred>
