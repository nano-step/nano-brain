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

async function main() {
  const platformKey = getPlatformKey();
  const binName = os.platform() === "win32" ? "nano-brain.exe" : "nano-brain";
  const binPath = path.join(__dirname, binName);

  if (fs.existsSync(binPath)) {
    try {
      const output = execSync(`"${binPath}" status --json`, { timeout: 5000 }).toString();
      if (output.includes(VERSION)) {
        console.log(`nano-brain v${VERSION} already installed.`);
        return;
      }
    } catch {
      // Wrong version or can't run — redownload
    }
  }

  const url = `https://github.com/${REPO}/releases/download/v${VERSION}/nano-brain-${platformKey}`;
  console.log(`Downloading nano-brain v${VERSION} for ${platformKey}...`);

  try {
    await download(url, binPath);
    fs.chmodSync(binPath, 0o755);
    console.log(`nano-brain v${VERSION} installed successfully.`);
  } catch (err) {
    console.error(`Failed to download binary: ${err.message}`);
    console.error("Build from source: CGO_ENABLED=0 go build -o npm/nano-brain ./cmd/nano-brain");
    process.exit(0);
  }
}

main();
