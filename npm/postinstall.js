#!/usr/bin/env node
"use strict";

const https = require("https");
const fs = require("fs");
const path = require("path");
const os = require("os");
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

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    https.get(url, (res) => {
      if (res.statusCode === 301 || res.statusCode === 302) {
        file.close();
        fs.unlinkSync(dest);
        return download(res.headers.location, dest).then(resolve).catch(reject);
      }
      if (res.statusCode !== 200) {
        file.close();
        fs.unlinkSync(dest);
        return reject(new Error(`Download failed: HTTP ${res.statusCode}`));
      }
      res.pipe(file);
      file.on("finish", () => {
        file.close(resolve);
      });
    }).on("error", (err) => {
      fs.unlinkSync(dest);
      reject(err);
    });
  });
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
    const patchNum = parseInt(patch, 10);
    if (!isNaN(patchNum) && patch.length < 4) {
      const padded = String(patchNum).padStart(4, "0");
      if (padded !== patch) {
        candidates.push(`v${parts[0]}.${parts[1]}.${padded}`);
      }
    }
  }
  return candidates;
}

function httpGetJSON(url) {
  return new Promise((resolve, reject) => {
    https.get(url, { headers: { "User-Agent": "nano-brain-postinstall" } }, (res) => {
      if (res.statusCode !== 200) {
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
  for (const r of releases) {
    if (!r.tag_name) continue;
    if (r.tag_name === `v${version}` || r.tag_name.replace(/^v/, "").replace(/^0+/, "") === version) {
      if (r.assets && r.assets.some((a) => a.name === assetName)) {
        return r.tag_name;
      }
    }
  }
  return null;
}

async function main() {
  const platformKey = getPlatformKey();
  const binName = os.platform() === "win32" ? "nano-brain.exe" : "nano-brain";
  const binPath = path.join(__dirname, binName);

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
      await download(url, binPath);
      fs.chmodSync(binPath, 0o755);
      console.log(`nano-brain v${VERSION} installed successfully from ${tag}.`);
      return;
    } catch (err) {
      lastErr = err;
    }
  }

  try {
    const tag = await resolveTagFromAPI(VERSION, assetName);
    if (tag) {
      const url = `https://github.com/${REPO}/releases/download/${tag}/${assetName}`;
      await download(url, binPath);
      fs.chmodSync(binPath, 0o755);
      console.log(`nano-brain v${VERSION} installed successfully from ${tag} (API fallback).`);
      return;
    }
  } catch (err) {
    lastErr = err;
  }

  console.error(`Failed to download binary: ${lastErr && lastErr.message}`);
  console.error("Build from source: CGO_ENABLED=0 go build -o npm/nano-brain ./cmd/nano-brain");
  process.exit(0);
}

main();
