"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const crypto = require("node:crypto");
const { parseSHA256Line, tryAutoLink, safeUnlink, download, downloadWithHash } = require("./postinstall");

// #592 regression: a request/socket error must REJECT the promise (so main()'s
// per-tag retry loop continues), never throw synchronously or crash the
// process via an unguarded unlink. Point at a closed port → ECONNREFUSED.
test("download: rejects (not throws) on connection error, no lingering file", async () => {
  const dest = path.join(os.tmpdir(), `nb-dl-refused-${process.pid}-${Date.now()}`);
  await assert.rejects(() => download("https://127.0.0.1:1/x", dest));
  assert.strictEqual(fs.existsSync(dest), false);
});

test("downloadWithHash: rejects (not throws) on connection error, no lingering file", async () => {
  const dest = path.join(os.tmpdir(), `nb-dlh-refused-${process.pid}-${Date.now()}`);
  await assert.rejects(() => downloadWithHash("https://127.0.0.1:1/x", dest));
  assert.strictEqual(fs.existsSync(dest), false);
});

test("safeUnlink: does not throw on a nonexistent path (#592 regression)", () => {
  const missing = path.join(os.tmpdir(), `nb-safeunlink-missing-${process.pid}-${Date.now()}`);
  assert.doesNotThrow(() => safeUnlink(missing));
});

test("safeUnlink: removes an existing file", () => {
  const p = path.join(os.tmpdir(), `nb-safeunlink-${process.pid}-${Date.now()}`);
  fs.writeFileSync(p, "x");
  safeUnlink(p);
  assert.strictEqual(fs.existsSync(p), false);
});

test("parseSHA256Line: returns hash for matching filename in single-line content", () => {
  const content = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789  nano-brain-linux-amd64\n";
  assert.strictEqual(
    parseSHA256Line(content, "nano-brain-linux-amd64"),
    "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
  );
});

test("parseSHA256Line: returns correct hash with multiple entries", () => {
  const content = [
    "1111111111111111111111111111111111111111111111111111111111111111  nano-brain-linux-amd64",
    "2222222222222222222222222222222222222222222222222222222222222222  nano-brain-linux-arm64",
    "3333333333333333333333333333333333333333333333333333333333333333  nano-brain-darwin-amd64",
    "4444444444444444444444444444444444444444444444444444444444444444  nano-brain-darwin-arm64",
    "",
  ].join("\n");
  assert.strictEqual(
    parseSHA256Line(content, "nano-brain-darwin-amd64"),
    "3333333333333333333333333333333333333333333333333333333333333333",
  );
});

test("parseSHA256Line: returns null when filename is not listed", () => {
  const content = "aaaa111122223333444455556666777788889999aaaabbbbccccddddeeeeffff  nano-brain-linux-amd64\n";
  assert.strictEqual(parseSHA256Line(content, "nano-brain-windows-amd64"), null);
});

test("parseSHA256Line: returns null for empty input", () => {
  assert.strictEqual(parseSHA256Line("", "anything"), null);
  assert.strictEqual(parseSHA256Line(null, "anything"), null);
  assert.strictEqual(parseSHA256Line(undefined, "anything"), null);
});

test("parseSHA256Line: returns null for malformed content (no hex)", () => {
  assert.strictEqual(parseSHA256Line("not a checksum line\n", "anything"), null);
  assert.strictEqual(parseSHA256Line("ZZZZ  nano-brain-linux-amd64\n", "nano-brain-linux-amd64"), null);
});

test("parseSHA256Line: ignores blank lines and unrelated text", () => {
  const content = [
    "",
    "# Comment line that should be ignored",
    "",
    "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789  nano-brain-linux-amd64",
    "",
  ].join("\n");
  assert.strictEqual(
    parseSHA256Line(content, "nano-brain-linux-amd64"),
    "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
  );
});

test("parseSHA256Line: handles CRLF line endings", () => {
  const content =
    "1111111111111111111111111111111111111111111111111111111111111111  nano-brain-linux-amd64\r\n" +
    "2222222222222222222222222222222222222222222222222222222222222222  nano-brain-linux-arm64\r\n";
  assert.strictEqual(
    parseSHA256Line(content, "nano-brain-linux-arm64"),
    "2222222222222222222222222222222222222222222222222222222222222222",
  );
});

test("parseSHA256Line: rejects hashes shorter than 64 hex chars", () => {
  const content = "abc  nano-brain-linux-amd64\n";
  assert.strictEqual(parseSHA256Line(content, "nano-brain-linux-amd64"), null);
});

test("parseSHA256Line: filename match is exact, not substring", () => {
  const content = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789  nano-brain-linux-amd64-extra\n";
  assert.strictEqual(parseSHA256Line(content, "nano-brain-linux-amd64"), null);
});

test("parseSHA256Line: hash format is independent of computed digest format", () => {
  const payload = Buffer.from("the quick brown fox jumps over the lazy dog");
  const computed = crypto.createHash("sha256").update(payload).digest("hex");
  const line = `${computed}  testfile\n`;
  assert.strictEqual(parseSHA256Line(line, "testfile"), computed);
  assert.strictEqual(
    parseSHA256Line(line, "testfile").toLowerCase(),
    computed.toLowerCase(),
    "hash comparison must be lowercase-insensitive in caller",
  );
});

test("end-to-end: full integrity flow with mocked content matches expected hash", () => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "nb-sha-test-"));
  try {
    const fakeBinary = Buffer.from("\x7fELF fake binary contents for test\n".repeat(100));
    const binPath = path.join(tmpDir, "nano-brain-linux-amd64");
    fs.writeFileSync(binPath, fakeBinary);
    const expectedHash = crypto.createHash("sha256").update(fakeBinary).digest("hex");
    const sumsContent = `${expectedHash}  nano-brain-linux-amd64\n`;
    const parsed = parseSHA256Line(sumsContent, "nano-brain-linux-amd64");
    assert.strictEqual(parsed, expectedHash, "parse must round-trip a real SHA-256 hex");

    const corrupted = Buffer.concat([fakeBinary, Buffer.from("X")]);
    const corruptedHash = crypto.createHash("sha256").update(corrupted).digest("hex");
    assert.notStrictEqual(corruptedHash, expectedHash, "different content must produce different hash");
  } finally {
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
});

test("TestAutoLink_OptIn_Linux_HappyPath", (t) => {
  process.env.NANO_BRAIN_AUTO_LINK = "1";
  t.after(() => { delete process.env.NANO_BRAIN_AUTO_LINK; });

  const HOME = os.homedir();
  const targetDir = path.join(HOME, ".local", "bin");
  const targetPath = path.join(targetDir, "nano-brain");
  const calls = [];

  const origExists = fs.existsSync;
  const origMkdir = fs.mkdirSync;
  const origCopy = fs.copyFileSync;
  const origChmod = fs.chmodSync;

  fs.existsSync = (p) => {
    if (p === path.join(HOME, ".nano-brain", "auto-link")) return false;
    if (p === targetPath) return false;
    return origExists(p);
  };
  fs.mkdirSync = (dir, opts) => { calls.push({ fn: "mkdirSync", dir, opts }); };
  fs.copyFileSync = (src, dst) => { calls.push({ fn: "copyFileSync", src, dst }); };
  fs.chmodSync = (p, mode) => { calls.push({ fn: "chmodSync", p, mode }); };

  const output = [];
  const origLog = console.log;
  console.log = (...args) => output.push(args.join(" "));
  try {
    tryAutoLink("/fake/nano-brain", "linux");
  } finally {
    console.log = origLog;
    fs.existsSync = origExists;
    fs.mkdirSync = origMkdir;
    fs.copyFileSync = origCopy;
    fs.chmodSync = origChmod;
  }

  assert.ok(calls.some((c) => c.fn === "copyFileSync" && c.src === "/fake/nano-brain" && c.dst === targetPath), "should copy binary to ~/.local/bin/nano-brain");
  assert.ok(calls.some((c) => c.fn === "chmodSync" && c.p === targetPath && c.mode === 0o755), "should chmod 0755");
  assert.ok(output.some((line) => line.includes(targetPath) && line.includes("PATH")), "should print PATH guidance");
});

test("TestAutoLink_OptIn_MarkerFile_Linux", () => {
  delete process.env.NANO_BRAIN_AUTO_LINK;

  const HOME = os.homedir();
  const markerPath = path.join(HOME, ".nano-brain", "auto-link");
  const targetDir = path.join(HOME, ".local", "bin");
  const targetPath = path.join(targetDir, "nano-brain");
  const calls = [];

  const origExists = fs.existsSync;
  const origMkdir = fs.mkdirSync;
  const origCopy = fs.copyFileSync;
  const origChmod = fs.chmodSync;

  fs.existsSync = (p) => {
    if (p === markerPath) return true;
    if (p === targetPath) return false;
    return origExists(p);
  };
  fs.mkdirSync = (dir, opts) => { calls.push({ fn: "mkdirSync", dir }); };
  fs.copyFileSync = (src, dst) => { calls.push({ fn: "copyFileSync", src, dst }); };
  fs.chmodSync = (p, mode) => { calls.push({ fn: "chmodSync", p, mode }); };

  try {
    tryAutoLink("/fake/nano-brain", "linux");
  } finally {
    fs.existsSync = origExists;
    fs.mkdirSync = origMkdir;
    fs.copyFileSync = origCopy;
    fs.chmodSync = origChmod;
  }

  assert.ok(calls.some((c) => c.fn === "copyFileSync" && c.dst === targetPath), "marker file should trigger copy to " + targetPath);
});

test("TestAutoLink_ExistingTarget_Skipped", (t) => {
  process.env.NANO_BRAIN_AUTO_LINK = "1";
  t.after(() => { delete process.env.NANO_BRAIN_AUTO_LINK; });

  const HOME = os.homedir();
  const targetDir = path.join(HOME, ".local", "bin");
  const targetPath = path.join(targetDir, "nano-brain");

  const origExists = fs.existsSync;
  const origCopy = fs.copyFileSync;

  let copied = false;
  fs.existsSync = (p) => {
    if (p === path.join(HOME, ".nano-brain", "auto-link")) return false;
    if (p === targetPath) return true;
    return origExists(p);
  };
  fs.copyFileSync = () => { copied = true; };

  const output = [];
  const origLog = console.log;
  console.log = (...args) => output.push(args.join(" "));
  try {
    tryAutoLink("/fake/nano-brain", "linux");
  } finally {
    console.log = origLog;
    fs.existsSync = origExists;
    fs.copyFileSync = origCopy;
  }

  assert.strictEqual(copied, false, "should not copy when target exists");
  assert.ok(output.some((line) => line.includes("WARN") && line.includes("already exists")), "should warn about existing file");
});

test("TestAutoLink_Windows_Skipped", (t) => {
  process.env.NANO_BRAIN_AUTO_LINK = "1";
  t.after(() => { delete process.env.NANO_BRAIN_AUTO_LINK; });

  const origCopy = fs.copyFileSync;
  let copied = false;
  fs.copyFileSync = () => { copied = true; };

  const output = [];
  const origLog = console.log;
  console.log = (...args) => output.push(args.join(" "));
  try {
    tryAutoLink("/fake/nano-brain", "win32");
  } finally {
    console.log = origLog;
    fs.copyFileSync = origCopy;
  }

  assert.strictEqual(copied, false, "should not copy on Windows");
  assert.ok(output.some((line) => line.includes("INFO") && line.includes("Windows")), "should print INFO for Windows");
});

test("TestAutoLink_OptOut_Default", () => {
  delete process.env.NANO_BRAIN_AUTO_LINK;

  const HOME = os.homedir();
  const origExists = fs.existsSync;
  const origCopy = fs.copyFileSync;

  let copied = false;
  fs.existsSync = (p) => {
    if (p === path.join(HOME, ".nano-brain", "auto-link")) return false;
    return origExists(p);
  };
  fs.copyFileSync = () => { copied = true; };

  const output = [];
  const origLog = console.log;
  console.log = (...args) => output.push(args.join(" "));
  try {
    tryAutoLink("/fake/nano-brain", "linux");
  } finally {
    console.log = origLog;
    fs.existsSync = origExists;
    fs.copyFileSync = origCopy;
  }

  assert.strictEqual(copied, false, "should not copy when opt-out");
  assert.strictEqual(output.length, 0, "should produce no output when opted out");
});

test("TestAutoLink_CopyFailure_NonFatal", (t) => {
  process.env.NANO_BRAIN_AUTO_LINK = "1";
  t.after(() => { delete process.env.NANO_BRAIN_AUTO_LINK; });

  const HOME = os.homedir();
  const targetDir = path.join(HOME, ".local", "bin");
  const targetPath = path.join(targetDir, "nano-brain");

  const origExists = fs.existsSync;
  const origMkdir = fs.mkdirSync;
  const origCopy = fs.copyFileSync;

  fs.existsSync = (p) => {
    if (p === path.join(HOME, ".nano-brain", "auto-link")) return false;
    if (p === targetPath) return false;
    return origExists(p);
  };
  fs.mkdirSync = () => {};
  fs.copyFileSync = () => { throw new Error("EACCES: permission denied"); };

  const output = [];
  const origLog = console.log;
  console.log = (...args) => output.push(args.join(" "));

  assert.doesNotThrow(() => {
    tryAutoLink("/fake/nano-brain", "linux");
  }, "should not throw on copy failure");

  console.log = origLog;
  fs.existsSync = origExists;
  fs.mkdirSync = origMkdir;
  fs.copyFileSync = origCopy;

  assert.ok(output.some((line) => line.includes("WARN") && line.includes("failed to auto-link")), "should warn on failure");
});

test("TestAutoLink_MacOS_TargetPath", (t) => {
  process.env.NANO_BRAIN_AUTO_LINK = "1";
  t.after(() => { delete process.env.NANO_BRAIN_AUTO_LINK; });

  const HOME = os.homedir();
  const targetDir = path.join(HOME, "Library", "nano-brain", "bin");
  const targetPath = path.join(targetDir, "nano-brain");
  const calls = [];

  const origExists = fs.existsSync;
  const origMkdir = fs.mkdirSync;
  const origCopy = fs.copyFileSync;
  const origChmod = fs.chmodSync;

  fs.existsSync = (p) => {
    if (p === path.join(HOME, ".nano-brain", "auto-link")) return false;
    if (p === targetPath) return false;
    return origExists(p);
  };
  fs.mkdirSync = (dir, opts) => { calls.push({ fn: "mkdirSync", dir }); };
  fs.copyFileSync = (src, dst) => { calls.push({ fn: "copyFileSync", src, dst }); };
  fs.chmodSync = (p, mode) => { calls.push({ fn: "chmodSync", p, mode }); };

  try {
    tryAutoLink("/fake/nano-brain", "darwin");
  } finally {
    fs.existsSync = origExists;
    fs.mkdirSync = origMkdir;
    fs.copyFileSync = origCopy;
    fs.chmodSync = origChmod;
  }

  assert.ok(calls.some((c) => c.fn === "copyFileSync" && c.dst === targetPath), "should copy to ~/Library/nano-brain/bin/nano-brain");
  assert.ok(calls.some((c) => c.fn === "mkdirSync" && c.dir === targetDir), "should mkdir ~/Library/nano-brain/bin");
});
