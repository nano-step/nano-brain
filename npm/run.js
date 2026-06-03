#!/usr/bin/env node
"use strict";

const { execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");

const binName = os.platform() === "win32" ? "nano-brain.exe" : "nano-brain";
const binPath = path.join(__dirname, binName);

const envBin = process.env.NANO_BRAIN_BIN;
if (envBin && envBin.trim() !== "") {
  const trimmed = envBin.trim();
  if (!fs.existsSync(trimmed)) {
    process.stderr.write(`Error: NANO_BRAIN_BIN points to ${trimmed} which does not exist. Unset the variable or correct the path.\n`);
    process.exit(1);
  }
  const mode = fs.statSync(trimmed).mode;
  if ((mode & 0o111) === 0) {
    process.stderr.write(`Error: NANO_BRAIN_BIN points to ${trimmed} which is not executable. Run: chmod +x ${trimmed}\n`);
    process.exit(1);
  }
  try { execFileSync(trimmed, process.argv.slice(2), { stdio: "inherit" }); }
  catch (e) { process.exit(e.status || 1); }
  process.exit(0);
}

if (!fs.existsSync(binPath)) {
  console.error("nano-brain binary not found at " + binPath + ".");
  console.error("The postinstall script may have failed during npm install.");
  console.error("Try: npm install -g @nano-step/nano-brain --foreground-scripts");
  console.error("Or build from source: CGO_ENABLED=0 go build -o npm/nano-brain ./cmd/nano-brain");
  process.exit(1);
}

try {
  execFileSync(binPath, process.argv.slice(2), { stdio: "inherit" });
} catch (e) {
  process.exit(e.status || 1);
}
