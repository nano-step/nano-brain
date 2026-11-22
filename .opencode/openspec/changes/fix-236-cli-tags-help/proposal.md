# Document `--tags` and Supported Flags in `query`, `search`, and `vsearch` Help (#236)

## Why

`nano-brain query`, `search`, and `vsearch` share `parseStubFlags` / `runStubCmd` in `cmd/nano-brain/commands.go` and already support `--tags` (`--tags t1,t2,...` / `--tags=t1,t2,...`) as well as time-range flags (`--created-after`, `--created-before`, `--updated-after`, `--updated-before`), `--scope`, `--workspace`, and `--json`. However:

1. Passing `--help` to `query`, `search`, or `vsearch` fails with `unknown flag: --help` (exit 1), and passing `-h` with `--scope=all` or `--workspace` sends `"-h"` as a literal search query to the server.
2. The per-command usage output in `cmd/nano-brain/commands.go` omits `--tags` and the four time-range flags (`--created-after`, `--created-before`, `--updated-after`, `--updated-before`).
3. The top-level `nano-brain help` command table (`printUsage` in `cmd/nano-brain/ops.go`) lists `--scope` and `--workspace` inline on `query`, `search`, and `vsearch` without `--tags`.

## What

- Handle `-h` and `--help` in `parseStubFlags` / `runStubCmd` so `nano-brain <query|search|vsearch> --help` (and `-h`) prints complete usage/flag help to stdout and exits `0` without making any HTTP request.
- Update the per-command usage helper (`printStubUsage` in `cmd/nano-brain/commands.go`) to document `--workspace`, `--scope`, `--tags`, `--created-after`, `--created-before`, `--updated-after`, `--updated-before`, `--json`, and `-h, --help` for both explicit help (stdout, exit 0) and missing-argument errors (stderr, exit 1).
- Update `printUsage` in `cmd/nano-brain/ops.go` so the `query`, `search`, and `vsearch` summary lines include `[--tags t1,t2,...]`.
