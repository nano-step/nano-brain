# watcher-file-filtering Delta — New Capability

## ADDED Requirements

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

#### Scenario: Real PNG drop produces zero PG errors

- **GIVEN** a workspace registered for `/tmp/test-binary/`
- **WHEN** a real PNG file (first 8 bytes `\x89PNG\r\n\x1a\n` followed by IHDR chunk + data) is copied into the watched directory
- **AND** the watcher's debounce timer fires
- **THEN** the server log contains 0 occurrences of the string `SQLSTATE 22021`
- **AND** 0 occurrences of `index failed`
- **AND** 1 occurrence of `skipping binary file (extension)` for the PNG
