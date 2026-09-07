# macOS native smoke test transcript (#615)

Isolated environment: `HOME=/tmp/nb-smoke/home`, config port **3199**
(`nanobrain_test` DB, `localhost`). Binary: freshly built
`CGO_ENABLED=0 go build -o /tmp/nb-smoke/nano-brain ./cmd/nano-brain`.
The dev server on :3100 was left untouched. Launchd label
`com.nano-step.nano-brain` in the real per-user `gui/<uid>` domain, fully
uninstalled at the end.

## Transcript (verbatim)

```
===== 1. service install =====
nano-brain service install complete — definition at /tmp/nb-smoke/home/Library/LaunchAgents/com.nano-step.nano-brain.plist (see 'nano-brain service status')
plist OK:
/tmp/nb-smoke/home/Library/LaunchAgents/com.nano-step.nano-brain.plist: OK

===== 2. launchctl print (registered job) =====
	state = running
	program = /tmp/nb-smoke/nano-brain
		state = active

===== 3. wait for /health ready (up to 30s) =====
health ready at t=4s: {"status":"ok","ready":true,"version":"dev","uptime_s":2,"workspace_count":6290}

===== 4. service status --json =====
{
  "platform": "darwin",
  "registered": true,
  "supervisor_state": "active",
  "health_reachable": true,
  "ready": true,
  "endpoint": "http://localhost:3199/health",
  "version": "dev",
  "error": ""
}

===== 5. kill the foreground serve process -> launchd KeepAlive must restart it =====
killing serve PID 98379
service auto-restarted and healthy

===== 6. service update =====
nano-brain service update complete — definition at /tmp/nb-smoke/home/Library/LaunchAgents/com.nano-step.nano-brain.plist (see 'nano-brain service status')

===== 7. service uninstall =====
nano-brain service uninstalled

SMOKE-PASS
```

## Generated plist (isolated fixture, generic values only)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.nano-step.nano-brain</string>
  <key>ProgramArguments</key>
  <array>
    <string>/tmp/nb-smoke/nano-brain</string>
    <string>--config</string>
    <string>/tmp/nb-smoke/home/.nano-brain/config.yml</string>
    <string>serve</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>ThrottleInterval</key>
  <integer>5</integer>
  <key>StandardOutPath</key>
  <string>/tmp/nb-smoke/home/.nano-brain/logs/service.log</string>
  <key>StandardErrorPath</key>
  <string>/tmp/nb-smoke/home/.nano-brain/logs/service.err.log</string>
</dict>
</plist>
```
