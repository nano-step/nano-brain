#!/usr/bin/env node
"use strict";

const { execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");

const binName = os.platform() === "win32" ? "nano-brain.exe" : "nano-brain";
const binPath = path.join(__dirname, binName);

function runBinary(bin) {
  try {
    execFileSync(bin, process.argv.slice(2), { stdio: "inherit" });
  } catch (e) {
    // A numeric status means the binary ran and chose that exit code (it has
    // already produced its own output). A non-numeric status means we could
    // not spawn it at all (arch mismatch, missing libc, ETXTBSY, perms) —
    // surface that, since stdio:"inherit" shows nothing for a spawn failure.
    if (typeof e.status !== "number") {
      process.stderr.write(`Error: failed to execute binary at ${bin}: ${e.message}\n`);
    }
    process.exit(e.status || 1);
  }
  process.exit(0);
}

// Explicit override wins and skips any download.
const envBin = process.env.NANO_BRAIN_BIN;
if (envBin && envBin.trim() !== "") {
  const trimmed = envBin.trim();
  if (!fs.existsSync(trimmed)) {
    process.stderr.write(`Error: NANO_BRAIN_BIN points to ${trimmed} which does not exist. Unset the variable or correct the path.\n`);
    process.exit(1);
  }
  if ((fs.statSync(trimmed).mode & 0o111) === 0) {
    process.stderr.write(`Error: NANO_BRAIN_BIN points to ${trimmed} which is not executable. Run: chmod +x ${trimmed}\n`);
    process.exit(1);
  }
  runBinary(trimmed);
  return; // runBinary exits; explicit return makes the branches structurally exclusive
}

if (fs.existsSync(binPath)) {
  runBinary(binPath);
  return;
}

// The binary is missing — the postinstall download either failed (offline /
// proxy) or was not persisted by the package manager (some npm/node versions
// discard postinstall-created files, #594). Download it now, at run time, where
// the write always persists, then run. Progress goes to stderr.
process.stderr.write("nano-brain: binary not present; downloading on first run...\n");
require("./postinstall")
  .ensureBinary()
  .then((bin) => runBinary(bin))
  .catch((err) => {
    const msg = err && err.message ? err.message : String(err);
    process.stderr.write(`nano-brain: could not obtain the binary: ${msg}\n`);
    process.stderr.write("Fix: set NANO_BRAIN_BIN=/path/to/nano-brain, or build from source: CGO_ENABLED=0 go build -o npm/nano-brain ./cmd/nano-brain\n");
    process.exit(1);
  });
