#!/usr/bin/env node
"use strict";

const { spawn, execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");

const binName = os.platform() === "win32" ? "nano-brain.exe" : "nano-brain";
const binPath = path.join(__dirname, binName);

// Launcher metadata for `nano-brain service install|update`. The Go CLI
// reads these env vars to pin a stable [node, run.js] argv pair into the
// native service definition instead of an ephemeral npx cache path.
function launcherEnv() {
  const env = Object.assign({}, process.env);
  env.NANO_BRAIN_NPM_LAUNCHED = "true";
  env.NANO_BRAIN_NPM_RUNJS = path.resolve(__filename);
  env.NANO_BRAIN_NPM_NODE = process.execPath;
  // Only mark the invocation as global when this package resolves from a
  // global npm root; npx/local invocations leave the marker unset so the Go
  // side can reject pinning a persistent service to an ephemeral cache.
  const argv = process.argv.slice(2);
  if (argv[0] === "service" && (argv[1] === "install" || argv[1] === "update")) {
    try {
      const globalRoot = execFileSync("npm", ["root", "-g"], { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] }).trim();
      const pkgDir = path.dirname(require.resolve("../package.json"));
      const rel = path.relative(globalRoot, pkgDir);
      if (rel === "" || !rel.startsWith("..")) {
        env.NANO_BRAIN_NPM_GLOBAL = "true";
      }
    } catch (e) {
      // npm unavailable — leave the marker unset; the Go CLI falls back to
      // the direct os.Executable() path.
    }
  }
  return env;
}

// runBinary spawns the Go binary as a foreground child, forwards
// SIGTERM/SIGINT to it so a managed service never leaves an orphaned
// process, and exits with the child's status or signal.
function runBinary(bin) {
  const child = spawn(bin, process.argv.slice(2), { stdio: "inherit", env: launcherEnv() });
  const forward = (signal) => {
    if (child.exitCode === null && child.signalCode === null) {
      try {
        child.kill(signal);
      } catch (e) {
        // already gone
      }
    }
  };
  process.on("SIGTERM", () => forward("SIGTERM"));
  process.on("SIGINT", () => forward("SIGINT"));
  child.on("error", (err) => {
    // A spawn error (arch mismatch, missing libc, ETXTBSY, perms) would
    // otherwise be invisible under stdio:"inherit" — surface it.
    process.stderr.write(`Error: failed to execute binary at ${bin}: ${err.message}\n`);
    process.exit(1);
  });
  child.on("exit", (code, signal) => {
    if (signal) {
      // The child died from a signal (not a graceful exit). Re-raise on
      // ourselves so the wrapper's own exit status reflects the signal —
      // but drop our handlers first or the re-raise is swallowed.
      try {
        process.removeAllListeners(signal);
      } catch (e) {
        // ignore
      }
      process.kill(process.pid, signal);
    } else {
      process.exit(code === null ? 1 : code);
    }
  });
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
