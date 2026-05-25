# Tasks: Container Server Guard

## Implementation

- [ ] Create `cmd/nano-brain/guard.go` with `isContainer()`, `autoConfigContainer()`, `checkExistingServer()`, `guardBeforeStart()`
- [ ] Update `cmd/nano-brain/main.go`: call `guardBeforeStart()` before `startServer()` and in `--daemon-child` path
- [ ] Update `cmd/nano-brain/daemon.go`: call `guardBeforeStart()` in `runServeCmd()` before fork

## Testing

- [ ] Build binary and verify: start server → try starting second → should be blocked
- [ ] Verify NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1 bypasses check
- [ ] Verify container detection works (/.dockerenv exists in our container)
- [ ] Verify auto-config sets NANO_BRAIN_HOST in container
- [ ] Verify CLI commands (query, status) still work from container to host
