## ADDED Requirements

### Requirement: Log levels with configurable threshold
The logger SHALL support log levels (`error`, `warn`, `info`, `debug`) with a configurable threshold. Messages below the threshold SHALL be silently dropped.

#### Scenario: Default log level is info
- **WHEN** no `logging.level` is set in config
- **THEN** `error`, `warn`, and `info` messages SHALL be written to the log file
- **THEN** `debug` messages SHALL be silently dropped

#### Scenario: Debug level enabled via config
- **WHEN** `logging.level: debug` is set in config
- **THEN** all messages including `debug` SHALL be written to the log file

#### Scenario: Noisy store logs demoted to debug
- **WHEN** `insertEmbeddingLocal` is called
- **THEN** the log message SHALL be emitted at `debug` level (not `info`)

### Requirement: Log file rotation by size
The logger SHALL rotate log files when they exceed 50MB. Old log files SHALL be deleted after 7 days.

#### Scenario: Log file exceeds 50MB
- **WHEN** a log write would cause the current log file to exceed 50MB
- **THEN** the current log file SHALL be renamed with a `.1` suffix
- **THEN** a new log file SHALL be created for subsequent writes

#### Scenario: Old rotated logs cleaned up
- **WHEN** the logger initializes or rotates
- **THEN** log files older than 7 days SHALL be deleted from the logs directory

### Requirement: Log function accepts level parameter
The `log()` function SHALL accept an optional level parameter. When omitted, the default level SHALL be `info`.

#### Scenario: Existing log calls without level
- **WHEN** `log('tag', 'message')` is called (no level parameter)
- **THEN** the message SHALL be treated as `info` level

#### Scenario: Debug-level log call
- **WHEN** `log('tag', 'message', 'debug')` is called
- **THEN** the message SHALL only be written if the configured threshold is `debug`
