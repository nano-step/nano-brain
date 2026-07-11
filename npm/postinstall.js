#!/usr/bin/env node
"use strict";

const https = require("https");
const fs = require("fs");
const path = require("path");
const os = require("os");
const crypto = require("crypto");
const { execSync } = require("child_process");

const VERSION = require("../package.json").version;
const REPO = "nano-step/nano-brain";

const PLATFORM_MAP = {
  darwin: "darwin",
  linux: "linux",
};
const ARCH_MAP = {
  arm64: "arm64",
  x64: "amd64",
};

function getPlatformKey() {
  const platform = PLATFORM_MAP[os.platform()];
  const arch = ARCH_MAP[os.arch()];
  if (!platform || !arch) {
    console.error(`Unsupported platform: ${os.platform()}-${os.arch()}`);
    console.error("Build from source: CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain");
    process.exit(0);
  }
  return `${platform}-${arch}`;
}

// Remove a partial download without throwing. A socket error can fire before
// createWriteStream has opened the file, or after a redirect branch already
// removed it — an unguarded fs.unlinkSync then throws ENOENT *inside* the
// event callback, which is uncaught and crashes the whole postinstall (#592).
function safeUnlink(p) {
  // No existsSync pre-check: the try/catch already swallows ENOENT, and a
  // check-then-unlink is a TOCTOU race. Best-effort cleanup.
  try {
    fs.unlinkSync(p);
  } catch (_) {
    /* best-effort cleanup */
  }
}

// file.close() is asynchronous; the fd is only released when its callback
// fires. Every unlink / recursive-redirect / reject is therefore deferred into
// file.close(() => …) so the partial file is gone (and not re-opened by a
// racing redirect) before we act — otherwise Windows unlink fails with EPERM
// while the fd is open, and a raw request error would leak the write stream.
function download(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    https.get(url, (res) => {
      if (res.statusCode === 301 || res.statusCode === 302) {
        file.close(() => {
          safeUnlink(dest);
          download(res.headers.location, dest).then(resolve).catch(reject);
        });
        return;
      }
      if (res.statusCode !== 200) {
        file.close(() => {
          safeUnlink(dest);
          reject(new Error(`Download failed: HTTP ${res.statusCode}`));
        });
        return;
      }
      res.pipe(file);
      // pipe() does NOT forward source 'error' events; without this a socket
      // reset mid-transfer emits an unhandled 'error' on res and crashes the
      // postinstall (#592) — mirror downloadWithHash's handler.
      res.on("error", (err) => {
        file.close(() => {
          safeUnlink(dest);
          reject(err);
        });
      });
      file.on("finish", () => {
        file.close(resolve);
      });
    }).on("error", (err) => {
      file.close(() => {
        safeUnlink(dest);
        reject(err);
      });
    });
  });
}

function downloadWithHash(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    const hash = crypto.createHash("sha256");
    https.get(url, (res) => {
      if (res.statusCode === 301 || res.statusCode === 302) {
        file.close(() => {
          safeUnlink(dest);
          downloadWithHash(res.headers.location, dest).then(resolve).catch(reject);
        });
        return;
      }
      if (res.statusCode !== 200) {
        file.close(() => {
          safeUnlink(dest);
          reject(new Error(`Download failed: HTTP ${res.statusCode}`));
        });
        return;
      }
      res.on("data", (chunk) => hash.update(chunk));
      res.pipe(file);
      file.on("finish", () => {
        file.close(() => resolve(hash.digest("hex")));
      });
      res.on("error", (err) => {
        file.close(() => {
          safeUnlink(dest);
          reject(err);
        });
      });
    }).on("error", (err) => {
      file.close(() => {
        safeUnlink(dest);
        reject(err);
      });
    });
  });
}

function parseSHA256Line(content, targetFilename) {
  if (typeof content !== "string" || !content) return null;
  const lines = content.split(/\r?\n/);
  const re = /^([a-f0-9]{64})\s+(.+?)\s*$/;
  for (const line of lines) {
    const m = line.match(re);
    if (m && m[2] === targetFilename) return m[1];
  }
  return null;
}

async function verifySHA256(tag, assetName, binPath, computedHex) {
  const sumsUrl = `https://github.com/${REPO}/releases/download/${tag}/SHA256SUMS`;
  const sumsPath = `${binPath}.SHA256SUMS.tmp`;
  let sumsContent;
  try {
    await download(sumsUrl, sumsPath);
    sumsContent = fs.readFileSync(sumsPath, "utf8");
  } catch (err) {
    console.warn(`⚠ SHA256SUMS not available for ${tag} (${err.message}); skipping integrity verification.`);
    return;
  } finally {
    safeUnlink(sumsPath);
  }

  const expectedHex = parseSHA256Line(sumsContent, assetName);
  if (!expectedHex) {
    console.warn(`⚠ ${assetName} not listed in SHA256SUMS for ${tag}; skipping integrity verification.`);
    return;
  }

  if (expectedHex.toLowerCase() !== computedHex.toLowerCase()) {
    // safeUnlink so an unrelated fs error (e.g. EACCES) can't turn a SHA
    // mismatch into a non-"SECURITY:" rejection that main() would treat as a
    // retryable download failure, bypassing the hard-fail fail-safe.
    safeUnlink(binPath);
    throw new Error(
      `SECURITY: SHA-256 mismatch for ${assetName}\n` +
      `  expected: ${expectedHex}\n` +
      `  computed: ${computedHex}\n` +
      `Binary has been deleted. Build from source: CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain`
    );
  }
}

// npm normalizes leading zeros in semver numeric identifiers (e.g. tag
// v2026.6.0101 publishes as 2026.6.101). Auto-tag uses a fixed-width 4-digit
// patch {DD}{NN} for new tags, but older history has 1-3 digit patches. Try
// the canonical form first, then a zero-padded reconstruction, then API.
function candidateTagsForVersion(version) {
  const parts = version.split(".");
  const candidates = [`v${version}`];
  if (parts.length === 3) {
    const patch = parts[2];
    if (/^\d+$/.test(patch) && patch.length < 4) {
      const padded = patch.padStart(4, "0");
      candidates.push(`v${parts[0]}.${parts[1]}.${padded}`);
    }
  }
  return candidates;
}

function normalizeVersion(v) {
  return v.replace(/^v/, "").split(".").map((p) => p.replace(/^0+/, "") || "0").join(".");
}

function httpGetJSON(url) {
  return new Promise((resolve, reject) => {
    https.get(url, { headers: { "User-Agent": "nano-brain-postinstall" } }, (res) => {
      if (res.statusCode !== 200) {
        res.resume();
        return reject(new Error(`API ${url} returned HTTP ${res.statusCode}`));
      }
      let body = "";
      res.on("data", (chunk) => { body += chunk; });
      res.on("end", () => {
        try { resolve(JSON.parse(body)); } catch (e) { reject(e); }
      });
    }).on("error", reject);
  });
}

async function resolveTagFromAPI(version, assetName) {
  const api = `https://api.github.com/repos/${REPO}/releases`;
  const releases = await httpGetJSON(`${api}?per_page=30`);
  const normalizedTarget = normalizeVersion(version);
  for (const r of releases) {
    if (!r.tag_name) continue;
    if (normalizeVersion(r.tag_name) === normalizedTarget) {
      if (r.assets && r.assets.some((a) => a.name === assetName)) {
        return r.tag_name;
      }
    }
  }
  return null;
}

// --- Opt-in auto-link ---
function tryAutoLink(srcBinPath, platform) {
  platform = platform || process.platform;

  const optIn = process.env.NANO_BRAIN_AUTO_LINK === "1" ||
    fs.existsSync(path.join(os.homedir(), ".nano-brain", "auto-link"));

  if (!optIn) return;

  if (platform === "win32") {
    console.log(`INFO: Auto-link is not supported on Windows; nano-brain binary remains at ${srcBinPath}.`);
    return;
  }

  let targetDir, targetPath;
  if (platform === "linux") {
    targetDir = path.join(os.homedir(), ".local", "bin");
    targetPath = path.join(targetDir, "nano-brain");
  } else if (platform === "darwin") {
    targetDir = path.join(os.homedir(), "Library", "nano-brain", "bin");
    targetPath = path.join(targetDir, "nano-brain");
  } else {
    console.log(`INFO: Auto-link is not supported on ${platform}; nano-brain binary remains at ${srcBinPath}.`);
    return;
  }

  if (fs.existsSync(targetPath)) {
    console.log(`WARN: ${targetPath} already exists; skipping auto-link to preserve your existing file.`);
    return;
  }

  try {
    fs.mkdirSync(targetDir, { recursive: true, mode: 0o755 });
    fs.copyFileSync(srcBinPath, targetPath);
    fs.chmodSync(targetPath, 0o755);
    console.log(`Copied nano-brain to ${targetPath}. Ensure ${targetDir} is in your PATH.`);
  } catch (e) {
    console.log(`WARN: failed to auto-link binary: ${e.message}. Binary remains at ${srcBinPath}.`);
  }
}

async function main() {
  const platformKey = getPlatformKey();
  const binName = os.platform() === "win32" ? "nano-brain.exe" : "nano-brain";
  const binPath = path.join(__dirname, binName);

  const skipVerify = !!process.env.NANO_BRAIN_SKIP_SHA_VERIFY;
  if (skipVerify) {
    console.warn("⚠ NANO_BRAIN_SKIP_SHA_VERIFY is set; binary integrity check will be skipped.");
  }

  if (fs.existsSync(binPath)) {
    try {
      const output = execSync(`"${binPath}" version --json`, { timeout: 5000 }).toString();
      if (output.includes(VERSION)) {
        console.log(`nano-brain v${VERSION} already installed.`);
        return;
      }
    } catch {
      // Wrong version or can't run — redownload
    }
  }

  console.log(`Downloading nano-brain v${VERSION} for ${platformKey}...`);

  const assetName = `nano-brain-${platformKey}`;
  let lastErr;
  for (const tag of candidateTagsForVersion(VERSION)) {
    const url = `https://github.com/${REPO}/releases/download/${tag}/${assetName}`;
    try {
      if (skipVerify) {
        await download(url, binPath);
      } else {
        const computedHex = await downloadWithHash(url, binPath);
        await verifySHA256(tag, assetName, binPath, computedHex);
      }
      fs.chmodSync(binPath, 0o755);
      console.log(`nano-brain v${VERSION} installed successfully from ${tag}.`);
      tryAutoLink(binPath);
      return;
    } catch (err) {
      if (err && typeof err.message === "string" && err.message.startsWith("SECURITY:")) {
        console.error(err.message);
        process.exit(1);
      }
      lastErr = err;
    }
  }

  try {
    const tag = await resolveTagFromAPI(VERSION, assetName);
    if (tag) {
      const url = `https://github.com/${REPO}/releases/download/${tag}/${assetName}`;
      if (skipVerify) {
        await download(url, binPath);
      } else {
        const computedHex = await downloadWithHash(url, binPath);
        await verifySHA256(tag, assetName, binPath, computedHex);
      }
      fs.chmodSync(binPath, 0o755);
      console.log(`nano-brain v${VERSION} installed successfully from ${tag} (API fallback).`);
      tryAutoLink(binPath);
      return;
    }
  } catch (err) {
    if (err && typeof err.message === "string" && err.message.startsWith("SECURITY:")) {
      console.error(err.message);
      process.exit(1);
    }
    lastErr = err;
  }

  console.error(`Failed to download binary: ${lastErr && lastErr.message}`);
  console.error("Build from source: CGO_ENABLED=0 go build -o npm/nano-brain ./cmd/nano-brain");
  process.exit(0);
}

if (require.main === module) {
  main();
}

module.exports = { parseSHA256Line, download, downloadWithHash, verifySHA256, tryAutoLink, safeUnlink };
