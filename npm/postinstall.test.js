"use strict";

const test = require("node:test");
const assert = require("node:assert");
const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const crypto = require("node:crypto");
const { parseSHA256Line } = require("./postinstall");

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
