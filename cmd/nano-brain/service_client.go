package main

import (
	"context"
	"fmt"
	"os"
	"time"
)

// managedRestartTimeout bounds native-manager (re)start calls during client
// recovery so a wedged supervisor cannot hang the CLI forever.
const managedRestartTimeout = 30 * time.Second

// newServicePlatformFn is the platform factory hook; tests override it to
// inject a fake runner without touching a live supervisor.
var newServicePlatformFn = newServicePlatform

// serviceHealthyWaitFn is the health-wait hook; tests override it to avoid
// polling a real server.
var serviceHealthyWaitFn = waitForServerHealthy

// startManagedServiceIfRegistered restarts a registered managed service
// through its native supervisor and waits for health. It returns
// (managed, started):
//
//	managed=false → no managed service is registered (platform unusable or
//	                definition missing) — the caller may use the legacy
//	                PID-file path.
//	managed=true, started=true → service (re)started and healthy.
//	managed=true, started=false → a managed definition exists but the
//	                restart/health failed — the caller MUST NOT fall back to
//	                the legacy daemon (it would compete with the service).
func startManagedServiceIfRegistered() (managed, started bool) {
	platform := newServicePlatformFn()
	if err := platform.usable(); err != nil {
		return false, false
	}
	if !registered(platform) {
		return false, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), managedRestartTimeout)
	defer cancel()
	if err := platform.register(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: managed service restart failed: %s\n", err)
		return true, false
	}
	if err := serviceHealthyWaitFn(serverHealthTimeout); err != nil {
		fmt.Fprintln(os.Stderr, "Managed service restarted but did not become healthy. Check 'nano-brain service status' and the service logs.")
		return true, false
	}
	return true, true
}
