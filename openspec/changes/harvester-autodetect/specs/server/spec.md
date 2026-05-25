# Spec: Server Startup — Auto-Detect session_dir

## ADDED Requirements

### Requirement: Auto-detect before disabling harvester
Before logging "opencode session harvester disabled", the server MUST probe platform well-known paths.

#### Scenario: OpenCode detected at Linux default path
- Given `cfg.Harvester.OpenCode.SessionDir == ""`
- And `$HOME/.local/share/opencode/storage` exists
- When the server starts
- Then `cfg.Harvester.OpenCode.SessionDir` is set to that path in memory
- And the server logs `"auto-detected opencode storage dir"` at info level with the path
- And the harvester starts normally

#### Scenario: OpenCode detected via XDG_DATA_HOME
- Given `XDG_DATA_HOME=/custom/data` and `/custom/data/opencode/storage` exists
- When the server starts
- Then the detected path is `/custom/data/opencode/storage`

#### Scenario: OpenCode detected at macOS default path
- Given `$HOME/Library/Application Support/opencode/storage` exists
- When the server starts on macOS (`runtime.GOOS == "darwin"`)
- Then that path is used

#### Scenario: OPENCODE_STORAGE_DIR env var takes priority
- Given `OPENCODE_STORAGE_DIR=/custom` and `/custom` exists
- And a platform default path also exists
- When the server starts
- Then `/custom` is used (env var wins)

#### Scenario: OpenCode not found — behavior unchanged
- Given no platform default path exists and env var is not set
- When the server starts
- Then the existing log message `"opencode session harvester disabled (no session_dir configured)"` is emitted
- And behavior is identical to the current implementation

### Requirement: Auto-detect does NOT modify config file
The auto-detect MUST only affect the in-memory `cfg` struct, never write to `~/.nano-brain/config.yml`.

#### Scenario: Config file unchanged after auto-detect
- Given auto-detect finds an OpenCode storage dir
- When the server starts
- Then `~/.nano-brain/config.yml` contents are not modified
- And `harvester.opencode.session_dir` is still empty string in the config file

### Requirement: detectOpenCodeStorageDir is pure and testable
The detection function MUST only use env vars and `os.Stat` — no network calls, no file writes.

#### Scenario: Function returns first existing candidate
- Given candidate paths A (exists) and B (exists) in priority order
- When `detectOpenCodeStorageDir()` is called
- Then it returns A (first match)

#### Scenario: Function returns empty string when nothing found
- Given no candidate paths exist
- When `detectOpenCodeStorageDir()` is called
- Then it returns `""`
