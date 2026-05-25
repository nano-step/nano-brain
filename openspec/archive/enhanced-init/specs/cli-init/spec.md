## MODIFIED Requirements

### Requirement: interactive init wizard enhancements

The interactive init wizard SHALL be enhanced with auto-detection, preview, and confirmation.

#### Scenario: Ollama auto-detection
- When the wizard starts
- Then it MUST attempt HTTP GET to `http://localhost:11434` with 2s timeout
- If reachable: show "Ollama detected at localhost:11434" and use as default
- If not reachable: ask for Ollama URL with default `http://localhost:11434`

#### Scenario: config preview
- After all questions are answered
- Then the wizard MUST display the full YAML config
- And ask "Save this config? [Y/n]:"
- If user answers n/N: abort without writing
- If user answers Y/y/Enter: write config and continue

#### Scenario: existing config overwrite
- Given a config file already exists
- When the wizard starts
- Then it MUST show "Config exists at <path>. Overwrite? [Y/n]:"
- If user answers n/N: exit gracefully
- If user answers Y/y/Enter: continue with current values as defaults

#### Scenario: workspace directory prompt
- After config is saved and doctor runs
- Then the wizard MUST ask "Register workspace directory? [current dir]:"
- If user provides path or accepts default: call `init --root <path>` via HTTP API if server reachable
- If server not reachable: print hint to start server first, then run `nano-brain init --root <path>`
