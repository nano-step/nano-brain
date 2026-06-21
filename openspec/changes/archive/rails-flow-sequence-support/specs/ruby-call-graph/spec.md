## ADDED Requirements

### Requirement: Ruby method definition extraction
The system SHALL extract method definitions from Ruby classes, producing Edge structures with kind="contains" that link classes to their methods.

#### Scenario: Extract controller actions
- **WHEN** a Rails controller file (app/controllers/*.rb) is parsed
- **THEN** the extractor SHALL produce contains edges from the controller class to each public method (action)

#### Scenario: Extract service methods
- **WHEN** a Rails service file (app/services/*.rb) is parsed
- **THEN** the extractor SHALL produce contains edges from the service class to each public method

#### Scenario: Extract model methods
- **WHEN** a Rails model file (app/models/*.rb) is parsed
- **THEN** the extractor SHALL produce contains edges from the model class to each public method

### Requirement: Ruby method call extraction
The system SHALL extract method calls within Ruby methods, producing Edge structures with kind="calls" that link callers to callees.

#### Scenario: Extract explicit method calls
- **WHEN** a Ruby method calls another method (e.g., `service.process(order)`)
- **THEN** the extractor SHALL produce a calls edge from the calling method to the called method

#### Scenario: Extract ActiveRecord queries
- **WHEN** a Ruby method calls ActiveRecord methods (where, find, create, etc.)
- **THEN** the extractor SHALL produce calls edges to the model class methods

#### Scenario: Skip stdlib and gem calls
- **WHEN** a Ruby method calls methods from Ruby standard library or external gems
- **THEN** the extractor SHALL NOT produce calls edges (these are external dependencies)

### Requirement: Ruby extractor implements Extractor interface
The Ruby extractor SHALL implement the Extractor interface with Supports and ExtractEdges methods.

#### Scenario: Support .rb extension
- **WHEN** Supports is called with ".rb"
- **THEN** it SHALL return true

#### Scenario: Require rails framework
- **WHEN** RequiresFrameworks is called
- **THEN** it SHALL return ["rails"]

### Requirement: Ruby extractor produces language metadata
All edges produced by the Ruby extractor SHALL include Language="ruby" in their metadata.

#### Scenario: Set language metadata
- **WHEN** edges are extracted from a Ruby file
- **THEN** each edge SHALL have Metadata["language"] = "ruby"
