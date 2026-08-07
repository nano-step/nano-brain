"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const { spawn, spawnSync } = require("node:child_process");

const runJs = path.join(__dirname, "run.js");
const isWin = os.platform() === "win32";

// Runs `node run.js <args>` with NANO_BRAIN_BIN pointing at a fake child
// script, returning { status, stdout, stderr }.
function runWrapper(binScript, args, env) {
  const res = spawnSync(process.execPath, [runJs, ...(args || [])], {
    encoding: "utf8",
    env: Object.assign({}, process.env, { NANO_BRAIN_BIN: binScript }, env || {}),
  });
  return res;
}

function makeScript(content) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "nb-run-"));
  const bin = path.join(dir, "fake-bin.sh");
  fs.writeFileSync(bin, `#!/bin/sh\n${content}\n`);
  fs.chmodSync(bin, 0o755);
  return bin;
}

test("exit status of the binary is propagated", { skip: isWin }, () => {
  const bin = makeScript("exit 7");
  const res = runWrapper(bin, []);
  assert.strictEqual(res.status, 7, `wrapper status = ${res.status}, want 7`);
});

test("successful binary run exits 0", { skip: isWin }, () => {
  const bin = makeScript("exit 0");
  const res = runWrapper(bin, []);
  assert.strictEqual(res.status, 0);
});

test("missing NANO_BRAIN_BIN target fails with a clear message", { skip: isWin }, () => {
  const res = runWrapper("/nonexistent/nano-brain", []);
  assert.strictEqual(res.status, 1);
  assert.match(res.stderr, /NANO_BRAIN_BIN/);
});

test("SIGTERM is forwarded to the child and the child is not orphaned", { skip: isWin }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "nb-sig-"));
  const started = path.join(dir, "child-started");
  const term = path.join(dir, "child-term");
  // A node child has deterministic signal semantics (an sh child defers a
  // trapped TERM while it waits on sleep, which makes the test racy).
  const bin = path.join(dir, "node-child.js");
  fs.writeFileSync(bin, `#!/usr/bin/env node\n` +
    `const fs = require("fs");\n` +
    `fs.writeFileSync(process.env.CHILD_STARTED, "started");\n` +
    `process.on("SIGTERM", () => { fs.writeFileSync(process.env.CHILD_TERM, "term"); process.exit(0); });\n` +
    `setInterval(() => {}, 1000);\n`);
  fs.chmodSync(bin, 0o755);

  const child = spawn(process.execPath, [runJs], {
    stdio: ["ignore", "pipe", "pipe"],
    env: Object.assign({}, process.env, {
      NANO_BRAIN_BIN: bin,
      CHILD_STARTED: started,
      CHILD_TERM: term,
    }),
  });
  child.stdout.on("data", () => {});
  child.stderr.on("data", () => {});

  // Wait for the child to be running, then terminate the wrapper.
  const startedDeadline = Date.now() + 5000;
  const readyPoll = setInterval(() => {
    if (fs.existsSync(started)) {
      clearInterval(readyPoll);
      setTimeout(() => child.kill("SIGTERM"), 100);
    } else if (Date.now() > startedDeadline) {
      clearInterval(readyPoll);
      child.kill("SIGKILL");
    }
  }, 15);

  return new Promise((resolve, reject) => {
    const deadline = setTimeout(() => reject(new Error("wrapper did not exit in time")), 10000);
    child.on("close", (code) => {
      clearTimeout(deadline);
      clearInterval(readyPoll);
      // The child writes the marker inside its SIGTERM handler before
      // exiting, so poll briefly for it rather than asserting at close.
      const termDeadline = Date.now() + 3000;
      const check = () => {
        if (fs.existsSync(term)) {
          assert.strictEqual(code, 0, `wrapper exit = ${code}, want 0`);
          resolve();
        } else if (Date.now() > termDeadline) {
          reject(new Error("child did not receive SIGTERM (marker missing)"));
        } else {
          setTimeout(check, 20);
        }
      };
      check();
    });
  });
});

test("service install exposes npm launcher metadata", { skip: isWin }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "nb-env-"));
  const envDump = path.join(dir, "env.json");
  const bin = makeScript(`node -e "require('fs').writeFileSync('${envDump}', JSON.stringify(process.env))"`);
  const res = runWrapper(bin, ["service", "install", "--json"]);
  assert.strictEqual(res.status, 0, `wrapper status = ${res.status}, stderr: ${res.stderr}`);
  const env = JSON.parse(fs.readFileSync(envDump, "utf8"));
  assert.strictEqual(env.NANO_BRAIN_NPM_LAUNCHED, "true");
  assert.strictEqual(env.NANO_BRAIN_NPM_RUNJS, path.resolve(runJs));
  assert.ok(env.NANO_BRAIN_NPM_NODE);
  // The repo is not inside the global npm root, so GLOBAL must stay unset —
  // the Go CLI then rejects pinning an npx/local launcher.
  assert.strictEqual(env.NANO_BRAIN_NPM_GLOBAL, undefined);
});
