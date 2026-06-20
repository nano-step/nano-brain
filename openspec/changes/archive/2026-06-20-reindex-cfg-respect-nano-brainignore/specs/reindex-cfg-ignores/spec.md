# reindex-cfg-ignores Delta — Reuse watcher filter in reindex-cfg handler

## ADDED Requirements

### Requirement: Reuse watcher filter package

The `POST /api/v1/reindex-cfg` handler SHALL reuse the watcher filter package (`github.com/nano-brain/nano-brain/internal/watcher`) for all ignore logic instead of implementing its own.

#### Scenario: Ignored directories are skipped during full reindex
- **GIVEN** workspace has `.nano-brainignore` containing `**/docker-data/**`
- **WHEN** `POST /api/v1/reindex-cfg` with `full: true` is called
- **THEN** no files under `docker-data/` are CFG-extracted
- **AND** the response `files_processed` count excludes files in `docker-data/`

#### Scenario: Global ignore is respected
- **GIVEN** `~/.nano-brain/.nano-brainignore` contains `**/vendor/**`
- **WHEN** `POST /api/v1/reindex-cfg` with `full: true` is called on a workspace with `vendor/` directory
- **THEN** no files under `vendor/` are CFG-extracted

### Requirement: Watcher loads nested `.nano-brainignore` during walk

The watcher's `scanCollection` SHALL load nested `.nano-brainignore` files per subdirectory during walk, the same way it loads nested `.gitignore` files.

#### Scenario: Nested .nano-brainignore in subdirectory
- **GIVEN** `workspace/lib/.nano-brainignore` exists with `*.snap`
- **WHEN** the watcher walks `workspace/lib/`
- **THEN** files matching `*.snap` are skipped during indexing

### Requirement: incrementalExtract respects ignore rules

The `incrementalExtract` function SHALL check each document against the ignore filter before processing. Documents matching ignore rules SHALL be skipped.

#### Scenario: Previously-indexed file is now ignored
- **GIVEN** `workspace/tmp/file.js` was indexed before `.nano-brainignore` added `**/tmp/**`
- **WHEN** `incrementalExtract` runs
- **THEN** `workspace/tmp/file.js` is skipped (not re-extracted)

### Requirement: ShouldSkip uses correct base path

The `ShouldSkip` method SHALL compute relative paths from the filter's `rootDir`, not from the current working directory.

#### Scenario: Server CWD differs from codeRoot
- **GIVEN** server CWD is `/home/user` and codeRoot is `/workspace/project`
- **WHEN** `ShouldSkip("/workspace/project/node_modules/pkg/index.js", false)` is called
- **THEN** the path is correctly identified as under `node_modules/` and skipped

### Requirement: Progress logging shows ignored paths

During the walk, the progress log SHALL include a count of paths skipped due to ignore rules.

#### Scenario: Progress log includes skip count
- **GIVEN** a large workspace is being walked
- **WHEN** 10 seconds elapse between file processing
- **THEN** the log line includes `files_processed`, `cfgs_extracted`, and `skipped` counts
