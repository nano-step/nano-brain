# ruby-flowchart-format Specification

## Purpose
TBD - created by archiving change fix-graph-tools. Update Purpose after archive.
## Requirements
### Requirement: Accept Ruby Class#method format for flowchart lookup
The system SHALL accept `file.rb::ClassName#method` format as a valid node identifier for flowchart lookups.

#### Scenario: Valid Ruby method format
- **WHEN** a user requests flowchart for `app/controllers/users_controller.rb::UsersController#create`
- **THEN** the system looks up the flowchart for that method
- **AND** returns the control-flow graph if it exists

#### Scenario: Ruby method with namespace
- **WHEN** a user requests flowchart for `app/controllers/api/v1/users_controller.rb::Api::V1::UsersController#create`
- **THEN** the system looks up the flowchart for that method

### Requirement: Support both Ruby and JS/TS formats
The system SHALL support both `file.rb::ClassName#method` (Ruby) and `file::startLine-endLine` (JS/TS) formats.

#### Scenario: JS/TS format still works
- **WHEN** a user requests flowchart for `src/index.ts::15-48`
- **THEN** the system looks up the flowchart for that line range

#### Scenario: Ruby format works
- **WHEN** a user requests flowchart for `app/models/user.rb::User#validate`
- **THEN** the system looks up the flowchart for that method

### Requirement: Graceful handling of missing flowchart
The system SHALL return a clear error message when no flowchart exists for the requested node.

#### Scenario: No flowchart exists
- **WHEN** a user requests flowchart for `app/models/user.rb::User#nonexistent`
- **THEN** the system returns `{"found": false, "error": "No flowchart found for node"}`

### Requirement: Store Ruby method as entry in function_flowcharts
The system SHALL store Ruby methods with entry format `file.rb::ClassName#method` in the `function_flowcharts` table.

#### Scenario: Ruby method extraction
- **WHEN** a Ruby file is indexed
- **THEN** each method is stored with entry = `file.rb::ClassName#method`
- **AND** the control-flow graph is stored in the `cfg` column

