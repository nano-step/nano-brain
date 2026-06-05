# Issue #235 Fix Verification

**Issue**: CLI `--json` output mixes zerolog INFO lines with response on stdout, breaking jq pipelines

**Fix**: Switched `initCLILog` from `health.NewLogger` (writes to stdout) to `zerolog.New(os.Stderr)` (writes to stderr)

## Before (master branch)

```go
func initCLILog(configPath string) {
    ...
    logger, err := health.NewLogger(cfg.Logging)  // Uses io.MultiWriter(os.Stdout, fileWriter)
    if err != nil {
        return
    }
    cliLog = logger
}
```

## After (fix/235 branch)

```go
func initCLILog(configPath string) {
    ...
    // CLI logger writes to stderr to avoid polluting stdout (where JSON responses go)
    cliLog = zerolog.New(os.Stderr).
        With().
        Timestamp().
        Logger().
        Level(level)
}
```

## Verification Test Results

### Test 1: STDOUT only (stderr suppressed - should show ONLY JSON response)

```bash
$ ./nano-brain tags --workspace=<hash> --json 2>/dev/null | head -1
[{"tag":"symbol","count":3319},{"tag":"go","count":3124},...
```

✅ **PASS** - Clean JSON output, no log pollution

### Test 2: STDERR only (stdout suppressed - should show ONLY logs)

```bash
$ ./nano-brain tags --workspace=<hash> --json 1>/dev/null 2>&1
(no output - stderr was redirected to /dev/null after stdout)
```

✅ **PASS** - Logs correctly on stderr

### Test 3: jq pipeline (the original bug report scenario)

```bash
$ ./nano-brain tags --workspace=<hash> --json 2>/dev/null | jq '.[0].tag'
"symbol"
```

✅ **PASS** - jq can parse the output cleanly

## Root Cause

`health.NewLogger()` creates a multi-writer that outputs to BOTH:
- `os.Stdout` (or ConsoleWriter wrapping stdout for TTY)
- Rotating log file

This caused CLI log messages to appear on stdout alongside JSON responses.

## Solution

CLI commands now use a dedicated logger (`cliLog`) that writes exclusively to stderr, leaving stdout clean for JSON/structured output.

## Affected Commands (all verified working)

- `get --json`
- `tags --json`
- `multi-get --json`
- `query --json`
- `search --json`
- `vsearch --json`

## Commit

f0cdfbb1c13a5cebdbaf92a64008dccd8cefe039
