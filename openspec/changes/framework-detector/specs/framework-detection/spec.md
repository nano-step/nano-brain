## ADDED Requirements

### Requirement: Manifest-based framework detection
The system SHALL detect web frameworks present in a workspace by reading manifest files (go.mod, package.json) at workspace registration time.

#### Scenario: Go project with Echo dependency
- **WHEN** the workspace contains a go.mod file with `github.com/labstack/echo` in the require block
- **THEN** the detector identifies frameworks `["echo", "go"]`

#### Scenario: Go project with Gin dependency
- **WHEN** the workspace contains a go.mod file with `github.com/gin-gonic/gin` in the require block
- **THEN** the detector identifies frameworks `["gin", "go"]`

#### Scenario: Go project with multiple frameworks
- **WHEN** the workspace contains a go.mod file with both `github.com/labstack/echo` and `github.com/gin-gonic/gin`
- **THEN** the detector identifies frameworks `["echo", "gin", "go"]`

#### Scenario: Go project with only stdlib
- **WHEN** the workspace contains a go.mod file with no web framework dependencies
- **THEN** the detector identifies frameworks `["go"]`

#### Scenario: JavaScript project with Express dependency
- **WHEN** the workspace contains a package.json with `express` in the `dependencies` or `devDependencies` field
- **THEN** the detector identifies frameworks `["express"]`

#### Scenario: No manifest files
- **WHEN** the workspace contains neither go.mod nor package.json
- **THEN** the detector returns an empty framework list `[]`

#### Scenario: Malformed manifest file
- **WHEN** the workspace contains a go.mod or package.json that cannot be parsed
- **THEN** the detector logs a warning and falls back to an empty framework list for that language

### Requirement: Framework-aware extractor filtering
The system SHALL skip graph extractors whose declared framework is not in the detected set, while always running extractors that don't declare a framework.

#### Scenario: Extractor with matching framework runs
- **WHEN** the detected frameworks include `"echo"` and an extractor declares `RequiresFrameworks() ["echo"]`
- **THEN** the extractor runs on matching files

#### Scenario: Extractor with non-matching framework is skipped
- **WHEN** the detected frameworks are `["go"]` and an extractor declares `RequiresFrameworks() ["express"]`
- **THEN** the extractor is skipped and does not parse any files

#### Scenario: Extractor without framework declaration always runs
- **WHEN** an extractor does not implement FrameworkAwareExtractor (or returns empty RequiresFrameworks)
- **THEN** the extractor runs on all matching files regardless of detected frameworks

#### Scenario: Detection failure falls back to all extractors
- **WHEN** framework detection fails (manifest unreadable, parse error, permission denied)
- **THEN** all extractors run (current behavior), and a warning is logged

### Requirement: Re-detection on manifest file changes
The system SHALL re-run framework detection when go.mod or package.json files change, updating the active extractor set for the workspace.

#### Scenario: Adding a framework dependency
- **WHEN** a go.mod file is modified to add `github.com/labstack/echo`
- **THEN** the detector re-runs and updates detected frameworks to include `"echo"`

#### Scenario: Removing a framework dependency
- **WHEN** a package.json is modified to remove `express` from dependencies
- **THEN** the detector re-runs and removes `"express"` from detected frameworks

#### Scenario: Manifest file created
- **WHEN** a go.mod file is created in a workspace that previously had none
- **THEN** the detector re-runs and detects frameworks from the new go.mod

### Requirement: Detection observability
The system SHALL log detected frameworks at DEBUG level and log detection failures at WARN level.

#### Scenario: Successful detection
- **WHEN** framework detection completes successfully for a workspace
- **THEN** a DEBUG log entry is emitted with the detected framework list and workspace path

#### Scenario: Detection failure
- **WHEN** framework detection fails for any reason
- **THEN** a WARN log entry is emitted with the specific error (file not found, parse error, permission denied)
