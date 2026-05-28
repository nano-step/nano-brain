# Spec: Init Wizard — Harvester Prompt

## ADDED Requirements

### Requirement: Detect OpenCode before prompting
The init wizard MUST probe platform-specific paths for OpenCode storage before deciding whether to show a harvester prompt.

#### Scenario: OpenCode storage detected
- Given the OpenCode storage directory exists at the platform default path (or `OPENCODE_STORAGE_DIR` env var)
- When the user runs `nano-brain init`
- Then after the workspace-registration prompt, the wizard prints the detected path
- And prompts `Enable session harvesting? [Y/n]:`

#### Scenario: OpenCode not installed
- Given no OpenCode storage directory exists at any well-known platform path
- And `OPENCODE_STORAGE_DIR` is not set
- When the user runs `nano-brain init`
- Then no harvester prompt is shown
- And the generated config YAML has no `harvester:` section

### Requirement: Write harvester config on acceptance
When the user accepts the harvester prompt, the generated config YAML MUST include the harvester block.

#### Scenario: User accepts harvester setup
- Given OpenCode storage is detected at `/path/to/storage`
- When the user accepts the `Enable session harvesting?` prompt (Y or empty Enter)
- Then the generated `~/.nano-brain/config.yml` contains:
  ```yaml
  harvester:
    opencode:
      session_dir: /path/to/storage
    claudecode:
      enabled: false
      session_dir: ""
  ```

#### Scenario: User declines harvester setup
- Given OpenCode storage is detected
- When the user types `n` or `N` at the prompt
- Then the generated config has no `harvester:` section (same as if OpenCode was absent)

### Requirement: OPENCODE_STORAGE_DIR as detection source
If `OPENCODE_STORAGE_DIR` is set and the directory exists, the wizard MUST use it as the detected path.

#### Scenario: Env var takes priority
- Given `OPENCODE_STORAGE_DIR=/custom/path` and `/custom/path` exists
- And a platform default path also exists
- When the wizard runs detection
- Then the wizard shows `/custom/path` as the detected path (not the platform default)
