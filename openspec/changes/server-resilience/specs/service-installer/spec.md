## ADDED Requirements

### Requirement: Service install command
`nano-brain serve install` SHALL detect the current platform and generate the appropriate service configuration file:

- **macOS**: `~/Library/LaunchAgents/com.nano-brain.server.plist` with `KeepAlive: true`, `RunAtLoad: true`
- **Linux**: `~/.config/systemd/user/nano-brain.service` with `Restart=always`, `RestartSec=2`

The command SHALL print the generated file path and instructions to start the service.

#### Scenario: Install on macOS
- **WHEN** `nano-brain serve install` is run on macOS
- **THEN** a launchd plist is written to `~/Library/LaunchAgents/com.nano-brain.server.plist` AND the command prints "Service installed. Start with: launchctl load ~/Library/LaunchAgents/com.nano-brain.server.plist"

#### Scenario: Install on Linux
- **WHEN** `nano-brain serve install` is run on Linux
- **THEN** a systemd user service is written to `~/.config/systemd/user/nano-brain.service` AND the command prints "Service installed. Start with: systemctl --user enable --now nano-brain"

#### Scenario: Already installed
- **WHEN** `nano-brain serve install` is run and the service file already exists
- **THEN** the command prints "Service already installed at {path}. Use --force to overwrite."

### Requirement: Service uninstall command
`nano-brain serve uninstall` SHALL stop the service (if running) and remove the service configuration file.

#### Scenario: Uninstall on macOS
- **WHEN** `nano-brain serve uninstall` is run on macOS
- **THEN** the service is unloaded via `launchctl unload` AND the plist file is deleted

#### Scenario: Uninstall on Linux
- **WHEN** `nano-brain serve uninstall` is run on Linux
- **THEN** the service is stopped and disabled via `systemctl --user disable --now nano-brain` AND the service file is deleted
