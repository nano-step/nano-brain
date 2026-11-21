"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const crypto = require("node:crypto");
const { spawnSync } = require("node:child_process");
const https = require("node:https");
const { EventEmitter } = require("node:events");
const { Readable } = require("node:stream");
const { parseSHA256Line, isAllowedDownloadHost, tryAutoLink, safeUnlink, download, downloadWithHash, ensureBinary, binaryPath, getPlatformKey, platformExtension, binaryMatchesVersion } = require("./postinstall");
const VERSION = require("../package.json").version;
const isWin = os.platform() === "win32";

// #594: when a valid binary is already at binaryPath(), ensureBinary() must
// return it WITHOUT any network download. Stub the binary with a tiny script
// that echoes the expected `version --json` payload. Any real binary already
// there is moved aside and restored so the assertion always runs (not skipped).
test("ensureBinary: returns existing binary without downloading", { skip: isWin }, async () => {
  const bin = binaryPath();
  const backup = `${bin}.itest.bak`;
  const hadReal = fs.existsSync(bin);
  if (hadReal) fs.renameSync(bin, backup);
  fs.writeFileSync(bin, `#!/bin/sh\necho '{"version":"${VERSION}"}'\n`);
  fs.chmodSync(bin, 0o755);
  try {
    const resolved = await ensureBinary();
    assert.strictEqual(resolved, bin);
  } finally {
    safeUnlink(bin);
    if (hadReal) fs.renameSync(backup, bin);
  }
});

test("binaryMatchesVersion: normalizes v-prefix and leading zeros (#644)", () => {
  assert.strictEqual(binaryMatchesVersion('{"version":"v2026.9.0703"}\n', "2026.9.703"), true);
  assert.strictEqual(binaryMatchesVersion('{"version":"v2026.11.2101"}\n', "2026.11.2101"), true);
  assert.strictEqual(binaryMatchesVersion('{"version":"0.0.0-dev"}\n', "0.0.0-dev"), true);
  assert.strictEqual(binaryMatchesVersion('{"version":"v2026.11.2101"}\n', "2026.11.210"), false);
});

// #594 BLOCKER regression: an unsupported platform must REJECT (not process.exit),
// so run.js can report it and exit 1. getPlatformKey throws; ensureBinary propagates.
test("ensureBinary: rejects (does not exit) on unsupported platform", async () => {
  const realPlatform = os.platform;
  os.platform = () => "freebsd";
  try {
    await assert.rejects(() => ensureBinary(), /Unsupported platform/);
  } finally {
    os.platform = realPlatform;
  }
});

// #637: Windows was partially implemented (binaryPath, tryAutoLink, run.js all
// special-case win32) but PLATFORM_MAP never included it — postinstall threw
// "Unsupported platform: win32-x64". These guard that gap from regressing.
test("getPlatformKey: returns windows-amd64 on win32+x64", () => {
  const realPlatform = os.platform;
  const realArch = os.arch;
  os.platform = () => "win32";
  os.arch = () => "x64";
  try {
    assert.strictEqual(getPlatformKey(), "windows-amd64");
  } finally {
    os.platform = realPlatform;
    os.arch = realArch;
  }
});

test("getPlatformKey: returns windows-arm64 on win32+arm64", () => {
  const realPlatform = os.platform;
  const realArch = os.arch;
  os.platform = () => "win32";
  os.arch = () => "arm64";
  try {
    assert.strictEqual(getPlatformKey(), "windows-arm64");
  } finally {
    os.platform = realPlatform;
    os.arch = realArch;
  }
});

test("platformExtension: returns .exe on win32, '' elsewhere", () => {
  assert.strictEqual(platformExtension("win32"), ".exe");
  assert.strictEqual(platformExtension("darwin"), "");
  assert.strictEqual(platformExtension("linux"), "");
});

test("platformExtension: defaults to current os.platform() when called with no arg", () => {
  const expected = os.platform() === "win32" ? ".exe" : "";
  assert.strictEqual(platformExtension(), expected);
});

test("ensureBinary: accepts win32+x64 without 'Unsupported platform' (#637 regression)", async () => {
  const realPlatform = os.platform;
  const realArch = os.arch;
  const realExists = fs.existsSync;
  os.platform = () => "win32";
  os.arch = () => "x64";
  // No existing binary at binaryPath() — force ensureBinary into the download
  // path so getPlatformKey() runs end-to-end. ECONNREFUSED is acceptable; the
  // assertion is that the failure is NOT the "Unsupported platform" branch.
  fs.existsSync = () => false;
  try {
    await assert.rejects(() => ensureBinary(), (err) => {
      assert.ok(err instanceof Error);
      assert.ok(
        !/Unsupported platform/.test(err.message),
        `should not throw Unsupported platform on win32+x64, got: ${err.message}`,
      );
      return true;
    });
  } finally {
    os.platform = realPlatform;
    os.arch = realArch;
    fs.existsSync = realExists;
  }
});

// #594: run.js executes the binary NANO_BRAIN_BIN points to (override path),
// end-to-end via a subprocess — proves the wrapper's exec path works.
test("run.js: runs NANO_BRAIN_BIN target and exits 0", { skip: isWin }, () => {
  const stub = path.join(os.tmpdir(), `nb-runjs-stub-${process.pid}-${Date.now()}`);
  fs.writeFileSync(stub, `#!/bin/sh\necho NB_STUB_OK\n`);
  fs.chmodSync(stub, 0o755);
  try {
    const r = spawnSync(process.execPath, [path.join(__dirname, "run.js"), "anyarg"], {
      env: { ...process.env, NANO_BRAIN_BIN: stub },
      encoding: "utf8",
    });
    assert.strictEqual(r.status, 0);
    assert.match(r.stdout, /NB_STUB_OK/);
  } finally {
    safeUnlink(stub);
  }
});

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

// --- #659: checksum verification fails closed; redirects stay on GitHub ---

// Replace https.get with an in-memory responder so these tests never touch the
// network. `respond(url)` returns { status, headers, body } or { error }.
// Returns the list of requested URLs.
const REAL_HTTPS_GET = https.get;
function stubHttps(t, respond) {
  const requested = [];
  https.get = (url, opts, cb) => {
    if (typeof opts === "function") cb = opts;
    requested.push(String(url));
    const req = new EventEmitter();
    const r = respond(String(url));
    setImmediate(() => {
      if (r.error) return req.emit("error", r.error);
      const res = Readable.from(r.body === undefined ? [] : [Buffer.from(r.body)]);
      res.statusCode = r.status;
      res.headers = r.headers || {};
      cb(res);
    });
    return req;
  };
  t.after(() => { https.get = REAL_HTTPS_GET; });
  return requested;
}

// Force ensureBinary() down the download path: move any real binary aside and
// restore it afterwards, removing whatever the test installed.
function withoutInstalledBinary(t) {
  const bin = binaryPath();
  const backup = `${bin}.itest659.bak`;
  const hadReal = fs.existsSync(bin);
  if (hadReal) fs.renameSync(bin, backup);
  t.after(() => {
    safeUnlink(bin);
    if (hadReal) fs.renameSync(backup, bin);
  });
  return bin;
}

function captureStderr(t) {
  const lines = [];
  const realError = console.error;
  console.error = (...args) => lines.push(args.join(" "));
  t.after(() => { console.error = realError; });
  return lines;
}

// main() and ensureBinary() key off err.message starting with "SECURITY:", so
// assert on the message itself (assert.rejects' regex form matches String(err)).
function securityError(re) {
  return (err) => {
    assert.ok(err.message.startsWith("SECURITY:"), err.message);
    assert.match(err.message, re);
    return true;
  };
}

// Redirect the first request only; anything requested after that is served,
// so code that wrongly follows the redirect fails an assertion instead of
// looping forever.
function redirectOnce(location) {
  let first = true;
  return () => {
    if (!first) return { status: 200, body: "followed" };
    first = false;
    return { status: 302, headers: { location } };
  };
}

const FAKE_BINARY = "fake nano-brain binary for #659 tests\n";
const FAKE_BINARY_SHA = crypto.createHash("sha256").update(FAKE_BINARY).digest("hex");
const ASSET_NAME = `nano-brain-${getPlatformKey()}${platformExtension()}`;
const isAssetURL = (url) => url.endsWith(`/${ASSET_NAME}`);
const isSumsURL = (url) => url.endsWith("/SHA256SUMS");

test("ensureBinary: missing SHA256SUMS is a SECURITY failure and leaves no binary (#659)", async (t) => {
  const bin = withoutInstalledBinary(t);
  captureStderr(t);
  stubHttps(t, (url) => (isAssetURL(url) ? { status: 200, body: FAKE_BINARY } : { status: 404 }));

  await assert.rejects(() => ensureBinary(), (err) => {
    assert.match(err.message, /^SECURITY: cannot verify /);
    assert.match(err.message, /SHA256SUMS could not be downloaded \(Download failed: HTTP 404\)/);
    assert.match(err.message, /NANO_BRAIN_SKIP_SHA_VERIFY=1/);
    assert.match(err.message, /go build/);
    return true;
  });
  assert.strictEqual(fs.existsSync(bin), false, "unverified binary must not stay installed");
  assert.strictEqual(fs.existsSync(`${bin}.SHA256SUMS.tmp`), false);
});

test("ensureBinary: SHA256SUMS network error is a SECURITY failure and leaves no binary (#659)", async (t) => {
  const bin = withoutInstalledBinary(t);
  captureStderr(t);
  stubHttps(t, (url) => (isAssetURL(url) ? { status: 200, body: FAKE_BINARY } : { error: new Error("ECONNRESET") }));

  await assert.rejects(() => ensureBinary(), securityError(/cannot verify .*ECONNRESET/));
  assert.strictEqual(fs.existsSync(bin), false);
});

test("ensureBinary: asset not listed in SHA256SUMS is a SECURITY failure and leaves no binary (#659)", async (t) => {
  const bin = withoutInstalledBinary(t);
  captureStderr(t);
  stubHttps(t, (url) => {
    if (isAssetURL(url)) return { status: 200, body: FAKE_BINARY };
    if (isSumsURL(url)) return { status: 200, body: `${FAKE_BINARY_SHA}  nano-brain-some-other-platform\n` };
    return { status: 404 };
  });

  await assert.rejects(() => ensureBinary(), (err) => {
    assert.match(err.message, /^SECURITY: cannot verify /);
    assert.ok(err.message.includes(`SHA256SUMS has no entry for ${ASSET_NAME}`), err.message);
    return true;
  });
  assert.strictEqual(fs.existsSync(bin), false, "unverified binary must not stay installed");
});

test("ensureBinary: NANO_BRAIN_SKIP_SHA_VERIFY installs without SHA256SUMS and warns (#659)", async (t) => {
  const bin = withoutInstalledBinary(t);
  const stderr = captureStderr(t);
  const prev = process.env.NANO_BRAIN_SKIP_SHA_VERIFY;
  process.env.NANO_BRAIN_SKIP_SHA_VERIFY = "1";
  t.after(() => {
    if (prev === undefined) delete process.env.NANO_BRAIN_SKIP_SHA_VERIFY;
    else process.env.NANO_BRAIN_SKIP_SHA_VERIFY = prev;
  });
  const requested = stubHttps(t, (url) => (isAssetURL(url) ? { status: 200, body: FAKE_BINARY } : { status: 404 }));

  assert.strictEqual(await ensureBinary(), bin);
  assert.strictEqual(fs.readFileSync(bin, "utf8"), FAKE_BINARY);
  assert.ok(stderr.some((l) => l.includes("NANO_BRAIN_SKIP_SHA_VERIFY is set")), "must warn that verification is skipped");
  assert.ok(!requested.some(isSumsURL), "opt-out must not fetch SHA256SUMS");
});

test("ensureBinary: verified install through a GitHub-only redirect chain (#659)", async (t) => {
  const bin = withoutInstalledBinary(t);
  captureStderr(t);
  const cdn = "https://release-assets.githubusercontent.com/github-production-release-asset/1/";
  const requested = stubHttps(t, (url) => {
    if (url.startsWith("https://github.com/") && isAssetURL(url)) {
      return { status: 302, headers: { location: "https://objects.githubusercontent.com/hop/bin" } };
    }
    if (url === "https://objects.githubusercontent.com/hop/bin") return { status: 307, headers: { location: `${cdn}bin` } };
    if (url === `${cdn}bin`) return { status: 200, body: FAKE_BINARY };
    if (url.startsWith("https://github.com/") && isSumsURL(url)) return { status: 301, headers: { location: `${cdn}sums` } };
    if (url === `${cdn}sums`) return { status: 200, body: `${FAKE_BINARY_SHA}  ${ASSET_NAME}\n` };
    return { status: 404 };
  });

  assert.strictEqual(await ensureBinary(), bin);
  assert.strictEqual(fs.readFileSync(bin, "utf8"), FAKE_BINARY);
  assert.ok(requested.includes(`${cdn}bin`) && requested.includes(`${cdn}sums`));
});

test("ensureBinary: binary redirected off GitHub is a SECURITY failure, target never requested (#659)", async (t) => {
  const bin = withoutInstalledBinary(t);
  captureStderr(t);
  const requested = stubHttps(t, (url) => {
    if (isAssetURL(url)) return { status: 302, headers: { location: "https://evil.example.com/nano-brain" } };
    return { status: 200, body: FAKE_BINARY };
  });

  await assert.rejects(() => ensureBinary(), securityError(/refusing redirect .* to https:\/\/evil\.example\.com/));
  assert.strictEqual(fs.existsSync(bin), false);
  assert.ok(!requested.some((u) => u.includes("evil.example.com")), "must not contact the redirect target");
});

for (const fn of [download, downloadWithHash]) {
  test(`${fn.name}: redirect to a non-GitHub host is a SECURITY failure (#659)`, async (t) => {
    const dest = path.join(os.tmpdir(), `nb-redir-host-${fn.name}-${process.pid}-${Date.now()}`);
    for (const location of [
      "https://evil.example.com/x",
      "https://githubusercontent.com.evil.example.com/x",
      "https://evilgithubusercontent.com/x",
      "https://github.com.evil.example.com/x",
      "//evil.example.com/x",
    ]) {
      const requested = stubHttps(t, redirectOnce(location));
      await assert.rejects(() => fn("https://github.com/o/r/releases/download/v1/x", dest), securityError(/refusing redirect/), location);
      assert.strictEqual(requested.length, 1, `must not follow ${location}`);
      assert.strictEqual(fs.existsSync(dest), false);
    }
  });

  test(`${fn.name}: http:// redirect is a SECURITY failure even on a GitHub host (#659)`, async (t) => {
    const dest = path.join(os.tmpdir(), `nb-redir-http-${fn.name}-${process.pid}-${Date.now()}`);
    const requested = stubHttps(t, redirectOnce("http://objects.githubusercontent.com/x"));
    await assert.rejects(() => fn("https://github.com/o/r/releases/download/v1/x", dest), securityError(/refusing redirect .* to http:\/\//));
    assert.strictEqual(requested.length, 1);
    assert.strictEqual(fs.existsSync(dest), false);
  });

  test(`${fn.name}: follows at most 5 redirects (#659)`, async (t) => {
    const dest = path.join(os.tmpdir(), `nb-redir-cap-${fn.name}-${process.pid}-${Date.now()}`);
    t.after(() => safeUnlink(dest));
    const chain = (hops) => (url) => {
      const n = Number(new URL(url).searchParams.get("n") || 0);
      if (n < hops) return { status: 302, headers: { location: `https://objects.githubusercontent.com/x?n=${n + 1}` } };
      return { status: 200, body: "ok" };
    };

    const tooMany = stubHttps(t, chain(6));
    await assert.rejects(() => fn("https://github.com/x", dest), (err) => {
      assert.match(err.message, /too many redirects/);
      return true;
    });
    assert.strictEqual(tooMany.length, 6, "initial request + 5 followed redirects, the 6th is refused");
    assert.strictEqual(fs.existsSync(dest), false);

    stubHttps(t, chain(5));
    await fn("https://github.com/x", dest);
    assert.strictEqual(fs.readFileSync(dest, "utf8"), "ok");
  });

  test(`${fn.name}: follows 303/307/308 and resolves a relative Location (#659)`, async (t) => {
    const dest = path.join(os.tmpdir(), `nb-redir-rel-${fn.name}-${process.pid}-${Date.now()}`);
    t.after(() => safeUnlink(dest));
    const requested = stubHttps(t, (url) => {
      if (url === "https://github.com/a/b") return { status: 303, headers: { location: "/c/d" } };
      if (url === "https://github.com/c/d") return { status: 308, headers: { location: "https://Objects.GitHubUserContent.com/e" } };
      if (url === "https://objects.githubusercontent.com/e") return { status: 307, headers: { location: "f" } };
      if (url === "https://objects.githubusercontent.com/f") return { status: 200, body: "ok" };
      return { status: 404 };
    });
    await fn("https://github.com/a/b", dest);
    assert.deepStrictEqual(requested, [
      "https://github.com/a/b",
      "https://github.com/c/d",
      "https://objects.githubusercontent.com/e",
      "https://objects.githubusercontent.com/f",
    ]);
    assert.strictEqual(fs.readFileSync(dest, "utf8"), "ok");
  });

  test(`${fn.name}: redirect without Location rejects (#659)`, async (t) => {
    const dest = path.join(os.tmpdir(), `nb-redir-noloc-${fn.name}-${process.pid}-${Date.now()}`);
    stubHttps(t, redirectOnce(undefined));
    await assert.rejects(() => fn("https://github.com/x", dest), /redirect without a Location/);
    assert.strictEqual(fs.existsSync(dest), false);
  });
}

test("isAllowedDownloadHost: github.com and *.githubusercontent.com only (#659)", () => {
  for (const h of ["github.com", "GitHub.com", "objects.githubusercontent.com", "release-assets.githubusercontent.com", "raw.githubusercontent.com"]) {
    assert.strictEqual(isAllowedDownloadHost(h), true, h);
  }
  for (const h of ["githubusercontent.com", "api.github.com", "evilgithub.com", "github.com.evil.example.com", "evilgithubusercontent.com", "example.com", ""]) {
    assert.strictEqual(isAllowedDownloadHost(h), false, h);
  }
});
