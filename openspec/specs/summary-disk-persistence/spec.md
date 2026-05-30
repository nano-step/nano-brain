# summary-disk-persistence Specification

## Purpose
TBD - created by archiving change restore-summary-disk-persistence. Update Purpose after archive.
## Requirements
### Requirement: Summaries are persisted to disk by default

When `summarization.enabled: true`, the persister SHALL write each summary to disk in addition to PostgreSQL by default. The disk write is opt-out via `summarization.write_to_disk: false`. The disk layer is a derivative view of the database — DB remains source of truth.

The file path SHALL be:

```
<output_dir>/<workspace_name>/<source>_<slug-title>_<YYYY-MM-DD>.md
```

Where:
- `<output_dir>` is `summarization.output_dir` from config, tilde-expanded at config load
- `<workspace_name>` is from `workspaces.name`; if empty, fall back to `ws-<workspace_hash[:12]>`
- `<source>` is `opencode` or `claude`
- `<slug-title>` is the slugified session title (see "Slugify" requirement)
- `<YYYY-MM-DD>` is the session creation date (UTC)

#### Scenario: Default-enabled disk persistence

- **GIVEN** server starts with default config (no `write_to_disk` key set in YAML)
- **AND** workspace `nano-brain` is registered with name="nano-brain"
- **AND** a new session `ses_18729f24...` titled "Watcher binary file filter root cause" is harvested
- **WHEN** `Persister.Save()` is invoked for this session
- **THEN** the DB row exists in `documents` with `collection='session-summary'`
- **AND** a file is created at `~/.nano-brain/summaries/nano-brain/opencode_watcher-binary-file-filter-root-cause_2026-05-30.md`
- **AND** the file content is byte-identical to `documents.content`
- **AND** an INFO log line records the file path

#### Scenario: Opt-out via config

- **GIVEN** server config has `summarization.write_to_disk: false`
- **WHEN** `Persister.Save()` is invoked
- **THEN** the DB row is created as usual
- **AND** NO file is created on disk
- **AND** NO file-related log entries are emitted

#### Scenario: Tilde expansion

- **GIVEN** config has `summarization.output_dir: ~/.nano-brain/summaries`
- **WHEN** the config is loaded
- **THEN** the effective `OutputDir` value is the absolute path (e.g. `/home/user/.nano-brain/summaries` or `/Users/tamlh/.nano-brain/summaries`)

#### Scenario: Workspace name fallback

- **GIVEN** a workspace row exists with `name=""` and `hash="7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f"`
- **WHEN** a summary is persisted for that workspace
- **THEN** the file path uses `ws-7f443561795a` as the workspace folder name (prefix + first 12 chars of hash)

### Requirement: Slugify produces safe, meaningful filenames

The slugify function SHALL transform arbitrary session titles into filesystem-safe identifiers per these rules:

1. Convert to lowercase
2. Replace each non-alphanumeric character with `-`
3. Collapse consecutive `-` characters to a single `-`
4. Trim leading and trailing `-`
5. Truncate to 80 characters maximum
6. If the result is empty (input had no alphanumerics), return `untitled-session`

#### Scenario: Common title transformations

- **WHEN** slugify is called with various inputs
- **THEN** outputs match expected values:

| Input | Output |
|-------|--------|
| `Oracle Verify Epic 9 (@oracle)` | `oracle-verify-epic-9-oracle` |
| `Foo!!! @bar ###` | `foo-bar` |
| `` (empty) | `untitled-session` |
| `@#$%` | `untitled-session` |
| `a---b` | `a-b` |
| `-foo-` | `foo` |
| 200 chars of `a` | first 80 chars of `a` |

### Requirement: Atomic disk write

Disk writes SHALL be atomic at the filesystem level. The persister writes content to `<path>.tmp` first, then renames the temp file to the final path via `os.Rename`. This prevents partial files appearing on disk if the process crashes mid-write.

#### Scenario: Atomic write produces complete file

- **WHEN** the persister writes a 3000-byte summary to `~/.../foo.md`
- **THEN** the bytes are written to `~/.../foo.md.tmp` first
- **AND** `os.Rename("foo.md.tmp", "foo.md")` is called
- **AND** the final file `~/.../foo.md` is exactly 3000 bytes
- **AND** no `.tmp` file remains on disk after successful write

### Requirement: DB-first commit ordering — disk failures do not roll back DB

The persister SHALL commit the DB transaction BEFORE attempting any disk write. Disk write failures (permission denied, disk full, invalid path) SHALL log a WARN and SHALL NOT cause the DB transaction to roll back.

#### Scenario: Disk failure does not lose DB row

- **GIVEN** the configured `output_dir` is on a read-only filesystem
- **WHEN** `Persister.Save()` is invoked for a new summary
- **THEN** the DB row is created in `documents` as usual
- **AND** an attempt is made to write to disk
- **AND** the disk write returns an error (EROFS or EACCES)
- **AND** the persister emits a WARN log with the error and the path
- **AND** `Persister.Save()` returns `nil` (no error to caller)
- **AND** subsequent searches still find the summary in the DB

### Requirement: Idempotent and collision-safe filenames

When the same session is persisted twice (same `session_id`, same date, same title), the file path SHALL be identical and the second write SHALL overwrite atomically with no duplicate files. When two DIFFERENT sessions produce the same intended path (same date + same slugified title, different session_ids), the second write SHALL append `_<sha8-of-session-id>` to the basename to avoid clobbering.

#### Scenario: Re-persisting same session is idempotent

- **GIVEN** session `ses_XXX` titled "Foo" was persisted at `~/.../opencode_foo_2026-05-30.md`
- **WHEN** the same session is persisted again (same title, same date)
- **THEN** the file path is the same `~/.../opencode_foo_2026-05-30.md`
- **AND** no new file is created
- **AND** no `_<sha8>` suffix is added

#### Scenario: Different sessions, same title and date

- **GIVEN** session A (`ses_aaaa1111`) titled "Foo" was persisted at `~/.../opencode_foo_2026-05-30.md`
- **AND** session B (`ses_bbbb2222`) is then persisted with the same title "Foo" on the same date
- **WHEN** the persister detects the path collision (existing file has different content/session_id)
- **THEN** session B is written to `~/.../opencode_foo_2026-05-30_bbbb2222.md`
- **AND** session A's file is unchanged

### Requirement: Backfill CLI command

The CLI SHALL expose `nano-brain backfill-summaries` to export existing summaries from the database to disk. The command SHALL support the same path/slug logic used by `Persister.Save()`.

Flags:
- `--output-dir=<path>` (override config; default uses `summarization.output_dir`)
- `--workspace=<name|hash>` (filter, optional)
- `--since=<RFC3339-date>` (filter, optional — only summaries created at or after this date)
- `--dry-run` (list paths that WOULD be written, no actual writes)

#### Scenario: Backfill exports all summaries

- **GIVEN** the DB has 167 summary documents in collection `session-summary`
- **WHEN** the operator runs `nano-brain backfill-summaries`
- **THEN** 167 files are created across the appropriate `<output_dir>/<workspace_name>/` folders
- **AND** the command prints a summary line `Written 167 files (0 skipped, 0 overwritten, 0 failed).`

#### Scenario: Backfill is idempotent

- **GIVEN** all 167 summaries already have files on disk
- **WHEN** the operator runs `nano-brain backfill-summaries` again
- **THEN** the command detects existing files with matching content
- **AND** prints `Written 0 files (167 skipped — already exist with identical content).`
- **AND** no files are modified

#### Scenario: Dry-run reports without writing

- **GIVEN** the DB has 5 summaries, none on disk yet
- **WHEN** the operator runs `nano-brain backfill-summaries --dry-run`
- **THEN** the command prints 5 lines listing the paths that WOULD be written
- **AND** no files are created
- **AND** the command exits with status 0

#### Scenario: Pre-flight server-running warning

- **GIVEN** a nano-brain server is running on `http://localhost:3100`
- **WHEN** the operator runs `nano-brain backfill-summaries` (without `--dry-run`)
- **THEN** the command prints a WARNING that the server is running and live writes could race with backfill
- **AND** the command proceeds (advisory only, does not abort)

### Requirement: File content fidelity

The byte content of the disk file SHALL be byte-for-byte identical to `documents.content` from PostgreSQL. The persister SHALL NOT add YAML frontmatter, modify the header, or apply any transformation to the markdown body.

#### Scenario: File content matches DB

- **GIVEN** a summary was persisted with content "abc\nxyz"
- **WHEN** the file is read from disk
- **AND** the corresponding `documents.content` is fetched from PG
- **THEN** the two values are byte-identical (including trailing newlines)

