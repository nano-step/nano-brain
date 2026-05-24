# Spec: Interactive Init Wizard

## ADDED Requirements

### Requirement: Interactive config setup

The system MUST provide an interactive setup wizard when `nano-brain init` is called without `--root` flag.

#### Scenario: First-time setup with defaults

- Given no config file exists at `~/.nano-brain/config.yml`
- When user runs `nano-brain init`
- Then the wizard prompts for: PostgreSQL URL, embedding provider, provider URL/key, embedding model, server port
- And each prompt shows a sensible default in brackets
- And pressing Enter accepts the default
- And config file is written to `~/.nano-brain/config.yml`
- And doctor checks run automatically after config is written

#### Scenario: Config file already exists

- Given config file exists at `~/.nano-brain/config.yml`
- When user runs `nano-brain init`
- Then the wizard shows current values as defaults
- And user can accept or override each value
- And config file is overwritten with new values

#### Scenario: Voyage provider selected

- Given user selects "voyage" as embedding provider
- When the wizard reaches provider-specific prompts
- Then it asks for Voyage API key instead of Ollama URL
- And it skips Ollama-specific prompts

### Requirement: Backward-compatible init with --root

The system MUST preserve the existing `nano-brain init --root <path>` behavior for workspace registration.

#### Scenario: Root flag provided

- Given user runs `nano-brain init --root /path/to/project`
- Then the command registers the workspace via HTTP API
- And no interactive prompts are shown
- And behavior is identical to current implementation
