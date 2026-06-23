# rails-association-edges Specification

## Purpose
TBD - created by archiving change fix-graph-tools. Update Purpose after archive.
## Requirements
### Requirement: Extract has_many associations as graph edges
The system SHALL extract `has_many :collection` declarations as graph edges with:
- Source: `file.rb::ClassName#has_many`
- Target: Model class name (e.g., `User` from `:users`)
- Edge type: `calls`
- Metadata: `{"dsl": true, "type": "association", "association_type": "has_many"}`

#### Scenario: has_many with symbol argument
- **WHEN** a Ruby file contains `has_many :users`
- **THEN** the system creates an edge from `file.rb::ClassName#has_many` to `User`
- **AND** the edge metadata includes `"association_type": "has_many"`

#### Scenario: has_many with class_name option
- **WHEN** a Ruby file contains `has_many :posts, class_name: "Article"`
- **THEN** the system creates an edge to `Article` (from class_name option)

### Requirement: Extract belongs_to associations as graph edges
The system SHALL extract `belongs_to :association` declarations as graph edges with:
- Source: `file.rb::ClassName#belongs_to`
- Target: Model class name (e.g., `User` from `:user`)
- Edge type: `calls`

#### Scenario: belongs_to with symbol argument
- **WHEN** a Ruby file contains `belongs_to :user`
- **THEN** the system creates an edge to `User`

### Requirement: Extract has_one associations as graph edges
The system SHALL extract `has_one :association` declarations as graph edges with:
- Source: `file.rb::ClassName#has_one`
- Target: Model class name

#### Scenario: has_one with symbol argument
- **WHEN** a Ruby file contains `has_one :profile`
- **THEN** the system creates an edge to `Profile`

### Requirement: Extract has_and_belongs_to_many as graph edges
The system SHALL extract `has_and_belongs_to_many :collection` declarations as graph edges with:
- Source: `file.rb::ClassName#has_and_belongs_to_many`
- Target: Model class name

#### Scenario: has_and_belongs_to_many with symbol argument
- **WHEN** a Ruby file contains `has_and_belongs_to_many :tags`
- **THEN** the system creates an edge to `Tag`

