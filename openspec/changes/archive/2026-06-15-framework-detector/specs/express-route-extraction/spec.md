## MODIFIED Requirements

### Requirement: ExpressExtractor framework gating
The system SHALL only run the ExpressExtractor on files in workspaces where Express is detected in package.json.

#### Scenario: Express detected
- **WHEN** the workspace has Express detected in package.json
- **THEN** the ExpressExtractor runs on `.ts`, `.tsx`, `.js`, `.jsx` files

#### Scenario: Express not detected
- **WHEN** the workspace does not have Express in package.json dependencies or devDependencies
- **THEN** the ExpressExtractor is skipped and does not parse any files

#### Scenario: Detection failure
- **WHEN** framework detection fails for the workspace
- **THEN** the ExpressExtractor runs on all matching files (fail-open)
