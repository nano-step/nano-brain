# smoke:e2e — #594 (lazy-download binary in run.js)

Change-type: bug-fix (npm run wrapper). Verified end-to-end: pack this branch
(version pinned to the real release 2026.7.1103), install where the package
manager does NOT persist the postinstall-downloaded binary, then invoke the
CLI and confirm the binary self-heals at run time.

## Setup
```
# pin version to an existing release so ensureBinary can fetch its asset
npm pack           # -> nano-step-nano-brain-2026.7.1103.tgz
npm install ./pkg.tgz   # postinstall runs; on npm 11.16 / node 26.3 the binary is NOT persisted
```

## Post-install: binary absent (the #594 condition)
```
$ ls node_modules/@nano-step/nano-brain/npm/nano-brain
ls: .../npm/nano-brain: No such file or directory
```

## Run the CLI → lazy download kicks in, then executes
```
$ node node_modules/@nano-step/nano-brain/npm/run.js version
nano-brain: binary not present; downloading on first run...        (stderr)
Downloading nano-brain v2026.7.1103 for darwin-arm64...             (stderr)
nano-brain v2026.7.1103 installed successfully from v2026.7.1103.   (stderr)
nano-brain v2026.7.1103                                             (stdout)
```

The real command output (`nano-brain v2026.7.1103`) lands on **stdout**; all
download progress is on **stderr**, so piping (`... version --json`) stays clean.
The download uses the same HTTPS path proven in #592
(`HTTP/1.1 302` github.com → `HTTP/1.1 200` release-assets, 45 MB), now invoked
at run time where the write always persists.

## Unit
```
$ node --test npm/postinstall.test.js
✔ ensureBinary: returns existing binary without downloading
... 23/23 pass
```

## Isolation
Scratch dirs under /tmp; `package.json` version bump reverted (stays 0.0.0-dev);
no dev DB / :3100 / Docker touched.
