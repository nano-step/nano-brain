## ADDED Requirements

### Requirement: config show command

The CLI SHALL display the effective configuration when `nano-brain config show` is invoked.

#### Scenario: default output
- Given a valid config file exists
- When the user runs `nano-brain config show`
- Then the CLI MUST print the merged config as formatted YAML
- And database URL password MUST be masked as `***`
- And voyage_api_key MUST be masked if present

#### Scenario: JSON output
- Given a valid config file exists
- When the user runs `nano-brain config show --json`
- Then the CLI MUST print valid JSON with the same masking rules

#### Scenario: no config file
- Given no config file exists at the default or specified path
- When the user runs `nano-brain config show`
- Then the CLI MUST print default values
- And indicate that no config file was found

---

### Requirement: config check command

The CLI SHALL validate the current configuration when `nano-brain config check` is invoked.

#### Scenario: all checks pass
- Given a valid config with reachable PostgreSQL and embedding provider
- When the user runs `nano-brain config check`
- Then the CLI MUST print each check with OK status
- And exit with code 0

#### Scenario: some checks fail
- Given a config with unreachable PostgreSQL
- When the user runs `nano-brain config check`
- Then the CLI MUST print failed checks with FAIL status and hints
- And exit with code 1

#### Scenario: JSON output
- When the user runs `nano-brain config check --json`
- Then the CLI MUST print valid JSON with per-check status

#### Scenario: relationship to doctor
- The config check command MUST reuse the doctor check functions (checkPostgres, checkPgvector, checkEmbeddingProvider, checkEmbeddingModel)
- The config check command SHALL NOT duplicate doctor logic
