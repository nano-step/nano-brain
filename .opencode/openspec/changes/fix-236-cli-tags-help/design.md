# Design

## Per-Command Help & Usage (`cmd/nano-brain/commands.go`)

1. Add `help bool` to `stubFlags` and handle `arg == "-h" || arg == "--help"` in `parseStubFlags` by setting `f.help = true` and returning immediately with no error string. Keep `default:` positional handling untouched so queries starting with a single dash (e.g. `"-race"`) continue to work as query strings.
2. Extract `printStubUsage(w io.Writer, endpoint string)` emitting the synopsis line (`Usage: nano-brain %s "<query>" --workspace <hash> [--scope all|workspace] [--tags t1,t2,...] [--created-after <ts|dur>] [--created-before <ts|dur>] [--updated-after <ts|dur>] [--updated-before <ts|dur>] [--json]`) and flag descriptions matching sibling CLI commands (`cmd_tags.go`, `cmd_get.go`, `cmd_workspace_remove.go`).
3. In `runStubCmd`:
   - When `f.help` is true: write `printStubUsage(os.Stdout, endpoint)` and return cleanly (exit code `0`, no HTTP call).
   - When `errMsg != ""`: write the error message to `os.Stderr` and exit `1`.
   - When `f.query == "" || workspaceVal == ""`: write `printStubUsage(os.Stderr, endpoint)` and exit `1`.

## Top-Level Help (`cmd/nano-brain/ops.go`)

Append `[--tags t1,t2,...]` to the inline flag summaries for `query`, `search`, and `vsearch` in `printUsage(w io.Writer)`.
