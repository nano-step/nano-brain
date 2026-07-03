#!/usr/bin/env bash
#
# nano-brain installer — downloads the prebuilt binary for your platform from
# GitHub Releases, verifies its SHA-256 against the release's SHA256SUMS, and
# installs it onto your PATH. No Node.js, no build toolchain required.
#
#   curl -fsSL https://raw.githubusercontent.com/nano-step/nano-brain/master/install.sh | bash
#
# Prefer to read before running? Download it, inspect it, then run:
#   curl -fsSL -o install.sh https://raw.githubusercontent.com/nano-step/nano-brain/master/install.sh
#   less install.sh && bash install.sh
#
# Environment overrides:
#   NANO_BRAIN_VERSION   pin a release tag (e.g. v2026.7.0201); default: latest
#   NANO_BRAIN_BIN_DIR   install directory; default: /usr/local/bin if writable,
#                        else ~/.local/bin
#
set -euo pipefail

REPO="nano-step/nano-brain"

err() { printf 'nano-brain install: %s\n' "$1" >&2; }
die() { err "$1"; exit 1; }

# ── Detect platform ──────────────────────────────────────────────────────────
detect_os() {
  case "$(uname -s)" in
    Darwin) echo "darwin" ;;
    Linux) echo "linux" ;;
    *) die "unsupported OS '$(uname -s)'. Build from source: CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) echo "amd64" ;;
    arm64 | aarch64) echo "arm64" ;;
    *) die "unsupported architecture '$(uname -m)'. Build from source: CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain" ;;
  esac
}

# ── SHA-256 helper (Linux sha256sum vs macOS shasum) ─────────────────────────
sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    die "no sha256 tool found (need 'sha256sum' or 'shasum'); cannot verify download"
  fi
}

need() { command -v "$1" >/dev/null 2>&1 || die "required command '$1' not found"; }

# tmp is global (not a main() local) so the EXIT trap, which runs in global
# scope, can reference it under `set -u` without an unbound-variable error.
tmp=""
cleanup() { [ -n "$tmp" ] && rm -rf "$tmp"; }

main() {
  need curl

  local os arch asset version base
  os="$(detect_os)"
  arch="$(detect_arch)"
  asset="nano-brain-${os}-${arch}"
  version="${NANO_BRAIN_VERSION:-latest}"

  if [ "$version" = "latest" ]; then
    base="https://github.com/${REPO}/releases/latest/download"
  else
    base="https://github.com/${REPO}/releases/download/${version}"
  fi

  trap cleanup EXIT
  tmp="$(mktemp -d "${TMPDIR:-/tmp}/nano-brain-install.XXXXXX")"

  printf 'nano-brain install: downloading %s (%s)...\n' "$asset" "$version" >&2
  curl -fsSL -o "$tmp/$asset" "${base}/${asset}" \
    || die "download failed: ${base}/${asset} (no release asset for ${os}-${arch}? check https://github.com/${REPO}/releases)"
  curl -fsSL -o "$tmp/SHA256SUMS" "${base}/SHA256SUMS" \
    || die "could not fetch SHA256SUMS from ${base} — refusing to install an unverified binary"

  # ── Verify checksum ────────────────────────────────────────────────────────
  local want got
  # awk exits after the first match (no `head`, which under pipefail could
  # SIGPIPE awk → exit 141 → abort); tr strips any CR from a CRLF sums file.
  want="$(awk -v a="$asset" '$2 == a || $2 == "*"a { print $1; exit }' "$tmp/SHA256SUMS" | tr -d '\r')"
  [ -n "$want" ] || die "no SHA256SUMS entry for $asset — refusing to install"
  got="$(sha256_of "$tmp/$asset")"
  if [ "$want" != "$got" ]; then
    die "checksum mismatch for $asset (expected $want, got $got) — refusing to install"
  fi
  printf 'nano-brain install: checksum verified.\n' >&2

  # ── Choose install dir ─────────────────────────────────────────────────────
  local bindir
  if [ -n "${NANO_BRAIN_BIN_DIR:-}" ]; then
    bindir="$NANO_BRAIN_BIN_DIR"
  elif [ -w /usr/local/bin ]; then
    bindir="/usr/local/bin"
  else
    bindir="$HOME/.local/bin"
  fi
  mkdir -p "$bindir" || die "cannot create install dir $bindir"

  # ── Install atomically ─────────────────────────────────────────────────────
  chmod +x "$tmp/$asset"
  # Unlink any existing binary first: a running nano-brain would make an
  # in-place `mv` fail with ETXTBSY (Text file busy) on Linux. Removing the
  # inode is safe — a running process keeps its open handle to the old file.
  rm -f "$bindir/nano-brain"
  mv -f "$tmp/$asset" "$bindir/nano-brain" \
    || die "cannot install to $bindir (try: NANO_BRAIN_BIN_DIR=\$HOME/.local/bin, or run with sudo)"

  printf 'nano-brain install: installed to %s/nano-brain\n' "$bindir" >&2

  # ── PATH check ─────────────────────────────────────────────────────────────
  case ":$PATH:" in
    *":$bindir:"*) : ;;
    *) printf '\n%s is not on your PATH. Add it:\n  export PATH="%s:$PATH"\n' "$bindir" "$bindir" >&2 ;;
  esac

  # Surface an exec failure (e.g. glibc/arch mismatch) instead of hiding it
  # behind a generic fallback string.
  local version_info
  if version_info="$("$bindir/nano-brain" version 2>/dev/null)"; then
    printf '\nInstalled: %s\nNext step: run the interactive setup wizard\n\n  nano-brain init\n\n' "$version_info" >&2
  else
    printf '\nInstalled to %s/nano-brain, but running it failed — the binary may be incompatible with this system (architecture or libc mismatch).\nCheck: %s/nano-brain version\n\n' "$bindir" "$bindir" >&2
  fi
}

main "$@"
