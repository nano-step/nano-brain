## MODIFIED Requirements

### Requirement: Contains edges capture class and module definitions
The Ruby contains query SHALL capture class and module definitions alongside method definitions.

#### Scenario: Extract class definition
- **WHEN** a Ruby file defines `class UsersController`
- **THEN** the contains edge TargetNode SHALL be `file.rb::UsersController` with kind="contains"

#### Scenario: Extract module definition
- **WHEN** a Ruby file defines `module Api::V1`
- **THEN** the contains edge TargetNode SHALL be `file.rb::Api::V1` with kind="contains"

#### Scenario: Extract nested class
- **WHEN** a Ruby file defines `class Api::V1::TokensController`
- **THEN** the contains edge TargetNode SHALL be `file.rb::TokensController` with kind="contains"
