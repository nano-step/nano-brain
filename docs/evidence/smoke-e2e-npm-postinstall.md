# smoke:e2e — #592 (npm postinstall ENOENT crash on transient download error)

Change-type: bug-fix (npm install script). The "endpoint" here is the GitHub
release-asset download the postinstall performs; verified over real HTTPS in
the same network that reproduced the crash. No server/DB involved.

## Repro of the bug (BEFORE — old postinstall, published 2026.7.1102)

```
$ npm install ./pkg.tgz --foreground-scripts
...
Error: ENOENT: no such file or directory, unlink '.../npm/nano-brain.SHA256SUMS.tmp'
    at ClientRequest.<anonymous> (npm/postinstall.js:53)
    at TLSSocket.socketErrorListener (node:_http_client:607:5)
npm error code 1
npm error command failed  (postinstall crashed → no binary installed → npx unusable)
```

## The real download path the postinstall exercises

The asset URL 302-redirects to `release-assets.githubusercontent.com`; a raw
`https.get` observes:

```
HTTP/1.1 302 Found      github.com/nano-step/nano-brain/releases/download/v2026.7.1102/nano-brain-darwin-arm64
HTTP/1.1 200 OK         release-assets.githubusercontent.com/...   (45,825,362 bytes)
```

So the network is fine — the crash was purely the unguarded `fs.unlinkSync`
firing twice across the 302 branch and the socket-error handler.

## AFTER (fixed postinstall — safeUnlink guards every unlink)

Ran the fixed `npm/postinstall.js` against the real release (version pinned to
2026.7.1102 for the run):

```
$ node npm/postinstall.js
Downloading nano-brain v2026.7.1102 for darwin-arm64...
nano-brain v2026.7.1102 installed successfully from v2026.7.1102.
```

No ENOENT, no crash. The `installed successfully` line is printed only after
`downloadWithHash` → `verifySHA256` → `fs.chmodSync(binPath)` all complete, so
the full download+verify+chmod path ran to completion. `npm install ./pkg.tgz
--foreground-scripts` likewise printed `installed successfully` and exited 0
(vs the old crash).

## Unit

```
$ node --test npm/postinstall.test.js
✔ safeUnlink: does not throw on a nonexistent path (#592 regression)
✔ safeUnlink: removes an existing file
... 20/20 pass
```

## Isolation
Test installs used scratch dirs under /tmp; `package.json` version bump was
reverted (stays `0.0.0-dev`); downloaded binary removed; no dev DB / :3100 /
Docker touched (this change has no server surface).
