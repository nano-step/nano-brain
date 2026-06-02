# watcher-file-filtering Specification

## Purpose
TBD - created by archiving change fix-watcher-binary-files. Update Purpose after archive.
## Requirements
### Requirement: Watcher skips files with binary extensions before disk read

The file watcher (`internal/watcher/watcher.go`) SHALL maintain a package-level map of file extensions known to contain binary content. Before reading file content from disk in `processFile()`, the watcher SHALL check the file's extension (case-insensitive) against this map and skip the file if matched. Skipped files emit an INFO-level log line and do NOT trigger any database write.

Binary extensions include (non-exhaustive): images (`.png .jpg .jpeg .gif .webp .bmp .ico .tiff .heic .heif`), archives (`.zip .tar .gz .7z .rar .bz2 .xz`), media (`.mp4 .mov .avi .mkv .webm .mp3 .wav .flac .ogg .aac .m4a`), executables (`.exe .dll .so .dylib .o .a .bin .obj .wasm`), fonts (`.woff .woff2 .ttf .otf .eot`), design (`.psd .ai .sketch`), databases (`.db .sqlite .sqlite3`), and documents (`.pdf`).

#### Scenario: PNG file is skipped before disk read

- **GIVEN** a watched workspace containing `image.png`
- **WHEN** the watcher's scan cycle encounters `image.png`
- **THEN** `isBinaryExtension("image.png")` returns true
- **AND** `os.ReadFile()` is NOT called for that path
- **AND** the watcher emits an INFO log: `skipping binary file (extension)` with `file=image.png`
- **AND** no document is created in the `documents` table for `image.png`

#### Scenario: Case-insensitive extension matching

- **WHEN** the watcher encounters files named `Image.PNG`, `photo.JPG`, `archive.Zip`
- **THEN** all three are matched by `isBinaryExtension` and skipped

#### Scenario: Text file with unknown extension is not skipped by extension check alone

- **WHEN** the watcher encounters `notes.xyz` containing valid UTF-8 markdown
- **THEN** `isBinaryExtension("notes.xyz")` returns false
- **AND** the file proceeds to the content-based check (next requirement)

### Requirement: Watcher skips files with non-UTF8 content after disk read

After reading file content from disk in `processFile()`, the watcher SHALL validate that the byte sequence is valid UTF-8 via `utf8.Valid(content)`. If the content is not valid UTF-8, the watcher SHALL skip the file with a WARN-level log line and SHALL NOT call any database upsert function. This is the safety net for files whose extension is not in the binary blacklist (e.g., binary files saved with `.txt` extension, files with no extension, or new binary formats not yet on the blacklist).

#### Scenario: Binary content with text-like extension is caught by content check

- **GIVEN** a watched workspace containing `data.txt` whose first bytes are `\x89PNG\r\n\x1a\n` (PNG magic)
- **WHEN** the watcher reads `data.txt`
- **THEN** `isBinaryExtension("data.txt")` returns false
- **AND** `os.ReadFile()` succeeds
- **AND** `isBinaryContent(content)` returns true (UTF-8 invalid)
- **AND** the watcher emits a WARN log: `skipping binary file (non-UTF8 content)` with `file=data.txt`
- **AND** no document is created in the `documents` table for `data.txt`
- **AND** no `UpsertDocumentBySourcePath` call is made

#### Scenario: Valid UTF-8 markdown passes both checks

- **GIVEN** `README.md` containing valid UTF-8 markdown
- **WHEN** the watcher processes the file
- **THEN** `isBinaryExtension` returns false
- **AND** `isBinaryContent` returns false
- **AND** the document is created via `UpsertDocumentBySourcePath` as before

#### Scenario: Empty file is treated as valid UTF-8

- **GIVEN** `empty.md` with zero bytes
- **WHEN** the watcher processes the file
- **THEN** `isBinaryContent([]byte{})` returns false (empty content is valid UTF-8)
- **AND** the document is created with empty content
- **AND** no spurious skip log is emitted

### Requirement: No SQLSTATE 22021 errors from watcher

After this change, the watcher MUST NOT produce any `SQLSTATE 22021` (invalid byte sequence for encoding UTF8) errors during normal operation. The combination of extension blacklist + UTF-8 validity check is sufficient to prevent any binary content from reaching `UpsertDocumentBySourcePath`.

#### Scenario: Binary file in watched directory triggers zero upserts

- **GIVEN** a watched collection on a temp directory containing `image.png` (real PNG magic bytes)
- **WHEN** `processFile` is invoked on the PNG path
- **THEN** the mock querier records `upsertDocCalls == 0` (no DB write attempted)
- **AND** the test log captures `skipping binary file (extension)`

#### Scenario: PNG-as-txt trap triggers UTF-8 safety net

- **GIVEN** a watched collection on a temp directory containing `data.txt` whose bytes are PNG magic (`\x89PNG\r\n\x1a\n`)
- **WHEN** `processFile` is invoked on `data.txt`
- **THEN** the extension check returns false (`.txt` is not in the blacklist)
- **AND** `os.ReadFile` succeeds
- **AND** the UTF-8 validity check returns true (bytes are not valid UTF-8)
- **AND** the mock querier records `upsertDocCalls == 0`
- **AND** the test log captures `skipping binary file (non-UTF8 content)`

### Requirement: Watcher loads workspace-local ignore file at collection registration

The watcher SHALL look for a file at `<rootDir>/.nano-brainignore` whenever `newFileFilter` is invoked for a collection (server startup, workspace registration via `POST /api/v1/init`, or collection add via `POST /api/v1/collections`). `<rootDir>` is the collection's root directory as passed to `WatchWithFilter`. If the file exists and is parseable as gitignore syntax (per the `github.com/sabhiram/go-gitignore` library), the watcher SHALL compile it and attach the matcher to the `fileFilter` for that collection only (workspace-scoped, NOT shared across collections).

If the file is missing, the watcher SHALL proceed without a local matcher and SHALL NOT emit any log entry (avoids per-collection noise — most collection roots will not have one). If the file exists but cannot be read (permission denied, the path resolves to a directory, or any other `os.ReadFile` failure), the watcher SHALL log a WARN with the read error and SHALL continue with a nil local matcher (the affected collection works as if the file were absent). Server startup and workspace registration MUST NOT fail because of one unreadable local ignore file.

Note: the `github.com/sabhiram/go-gitignore` library tolerates any pattern content — it does not produce parse errors. Only file-read failures (IO-level) reach the WARN path.

If the file exists and loads successfully, the watcher SHALL emit a DEBUG log entry containing the `dir` (collection rootDir) and `path` (full path to the ignore file).

#### Scenario: File exists with valid patterns

- **GIVEN** a workspace at `/tmp/myproj` is registered via `POST /api/v1/init`
- **AND** `/tmp/myproj/.nano-brainignore` contains:
  ```
  *.snap
  fixtures/
  *.generated.go
  ```
- **WHEN** the watcher's `WatchWithFilter` invokes `newFileFilter` for the `code` collection rooted at `/tmp/myproj`
- **THEN** the watcher emits a DEBUG log `loaded workspace .nano-brainignore` with `dir=/tmp/myproj` and `path=/tmp/myproj/.nano-brainignore`
- **AND** the `fileFilter` for that collection has a non-nil `localIgnore` matcher
- **AND** the matcher applies ONLY to that collection's filter (not shared across other registered workspaces)

#### Scenario: File does not exist

- **GIVEN** a workspace at `/tmp/other` is registered
- **AND** no `.nano-brainignore` file exists at `/tmp/other`
- **WHEN** the watcher invokes `newFileFilter` for the `code` collection
- **THEN** the watcher emits NO log entry related to local ignore
- **AND** the collection's `fileFilter.localIgnore` is nil
- **AND** all other filtering layers (defaults, global ignore, `.gitignore`, excludePatterns, allowedExtensions) operate exactly as before this feature

#### Scenario: File is unreadable

- **GIVEN** `/tmp/badproj/.nano-brainignore` exists but cannot be read (for example, mode `0000`, or the path resolves to a directory rather than a file)
- **WHEN** the watcher invokes `newFileFilter` for the `code` collection
- **THEN** `newFileFilter` returns the IO error to its caller
- **AND** `WatchWithFilter` logs a WARN with the error and `path=/tmp/badproj/.nano-brainignore` and the message `workspace .nano-brainignore failed to load, continuing without local matcher`
- **AND** the collection's `fileFilter.localIgnore` is nil
- **AND** the watch is established successfully (other filtering layers active)
- **AND** the server start sequence is not aborted

### Requirement: Workspace-local patterns apply additively with global and `.gitignore`

When a workspace-local `.nano-brainignore` matcher is present for a collection, every `fileFilter.shouldSkip` decision SHALL evaluate the local matcher against the path RELATIVE to the collection rootDir. The local matcher is checked AFTER `defaultExcludeDirs` and `globalIgnore` but BEFORE `gitignoreMatcher`, `excludePatterns`, and `allowedExtensions`. Each layer is a short-circuit OR — any matcher returning a match SHALL cause the file to be skipped.

The watcher SHALL NOT apply cross-file negation semantics. A `!pattern` in workspace-local cannot "un-exclude" a path matched by global or `.gitignore`. Each ignore file evaluates as an independent gitignore matcher.

#### Scenario: Local pattern skips files git tracks

- **GIVEN** workspace `/tmp/proj` has `.gitignore` containing only `build/`
- **AND** `/tmp/proj/.nano-brainignore` contains `*.generated.go`
- **WHEN** the watcher walks the workspace
- **THEN** `/tmp/proj/build/x.o` is skipped (by `.gitignore`)
- **AND** `/tmp/proj/src/api.generated.go` is skipped (by local `.nano-brainignore`)
- **AND** `/tmp/proj/src/api.go` is indexed

#### Scenario: Local and global apply independently

- **GIVEN** `~/.nano-brain/.nano-brainignore` contains `*.log`
- **AND** workspace `/tmp/proj` has `.nano-brainignore` containing `*.snap`
- **WHEN** the watcher walks `/tmp/proj`
- **THEN** `/tmp/proj/app.log` is skipped (matched by global)
- **AND** `/tmp/proj/test.snap` is skipped (matched by local)
- **AND** `/tmp/proj/main.go` is indexed

#### Scenario: Other registered workspaces are unaffected

- **GIVEN** workspace `/tmp/A` is registered with `.nano-brainignore` containing `*.snap`
- **AND** workspace `/tmp/B` is registered WITHOUT a `.nano-brainignore`
- **WHEN** the watcher walks both workspaces
- **THEN** `/tmp/A/test.snap` is skipped
- **AND** `/tmp/B/test.snap` is indexed (no local matcher for B)

### Requirement: Re-registration picks up changes without restart

The watcher SHALL re-read the workspace-local `.nano-brainignore` file each time `WatchWithFilter` is invoked for a given rootDir. When `POST /api/v1/init` is called for an already-registered `root_path`, the watcher SHALL construct a fresh `fileFilter` (which loads the current contents of `.nano-brainignore`) and overwrite the existing entry in the `collections` map.

This is the v1 reload mechanism. The watcher SHALL NOT establish an fsnotify watch on the ignore file itself, and `POST /api/reload-config` SHALL NOT trigger re-reads of workspace-local ignore files (the existing reload-config code path only updates search config and log level; this is unchanged).

#### Scenario: Re-init picks up updated patterns

- **GIVEN** workspace `/tmp/proj` was registered with `.nano-brainignore` containing `*.tmp`
- **AND** the user edits `.nano-brainignore` to add `*.snap`
- **WHEN** the user calls `POST /api/v1/init` again with `root_path=/tmp/proj`
- **THEN** the watcher invokes `newFileFilter` for the `code` collection again
- **AND** the new `fileFilter` has a `localIgnore` matcher compiled from the updated file contents (both `*.tmp` AND `*.snap`)
- **AND** subsequent walks/events for that collection use the updated matcher

#### Scenario: reload-config does NOT pick up workspace-local changes

- **GIVEN** workspace `/tmp/proj` is registered with `.nano-brainignore` containing `*.tmp`
- **AND** the user edits `.nano-brainignore` to add `*.snap`
- **WHEN** the user calls `POST /api/reload-config`
- **THEN** the response indicates config reloaded
- **AND** the collection's `fileFilter.localIgnore` is unchanged (still only `*.tmp`)
- **AND** to pick up the change, the user must re-register the workspace OR restart the server

