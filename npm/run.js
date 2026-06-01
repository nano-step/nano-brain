#!/usr/bin/env node
"use strict";

const { execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");

const binName = os.platform() === "win32" ? "nano-brain.exe" : "nano-brain";
const binPath = path.join(__dirname, binName);

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
