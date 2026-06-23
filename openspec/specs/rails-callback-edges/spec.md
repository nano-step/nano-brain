# rails-callback-edges Specification

## Purpose
TBD - created by archiving change fix-graph-tools. Update Purpose after archive.
## Requirements
### Requirement: Extract before_action callbacks as graph edges
The system SHALL extract `before_action :method_name` declarations as graph edges with:
- Source: `file.rb::ClassName#before_action`
- Target: `ClassName#method_name` (qualified method reference)
- Edge type: `middleware`
- Metadata: `{"dsl": true, "type": "callback", "callback_type": "before_action"}`

#### Scenario: before_action with symbol argument
- **WHEN** a Ruby controller contains `before_action :authenticate`
- **THEN** the system creates an edge from `file.rb::UsersController#before_action` to `UsersController#authenticate`

#### Scenario: before_action with multiple methods
- **WHEN** a Ruby controller contains `before_action :authenticate, :authorize`
- **THEN** the system creates edges to both `UsersController#authenticate` and `UsersController#authorize`

### Requirement: Extract after_action callbacks as graph edges
The system SHALL extract `after_action :method_name` declarations as graph edges with:
- Source: `file.rb::ClassName#after_action`
- Target: `ClassName#method_name`
- Edge type: `middleware`

#### Scenario: after_action with symbol argument
- **WHEN** a Ruby controller contains `after_action :log_activity`
- **THEN** the system creates an edge to `UsersController#log_activity`

### Requirement: Extract before_save callbacks as graph edges
The system SHALL extract `before_save :method_name` declarations as graph edges with:
- Source: `file.rb::ClassName#before_save`
- Target: `ClassName#method_name`
- Edge type: `middleware`

#### Scenario: before_save with symbol argument
- **WHEN** a Ruby model contains `before_save :validate_email`
- **THEN** the system creates an edge to `User#validate_email`

### Requirement: Extract after_save callbacks as graph edges
The system SHALL extract `after_save :method_name` declarations as graph edges with:
- Source: `file.rb::ClassName#after_save`
- Target: `ClassName#method_name`
- Edge type: `middleware`

#### Scenario: after_save with symbol argument
- **WHEN** a Ruby model contains `after_save :send_welcome_email`
- **THEN** the system creates an edge to `User#send_welcome_email`

### Requirement: Extract before_create callbacks as graph edges
The system SHALL extract `before_create :method_name` declarations as graph edges with:
- Source: `file.rb::ClassName#before_create`
- Target: `ClassName#method_name`
- Edge type: `middleware`

#### Scenario: before_create with symbol argument
- **WHEN** a Ruby model contains `before_create :set_defaults`
- **THEN** the system creates an edge to `User#set_defaults`

### Requirement: Extract after_create callbacks as graph edges
The system SHALL extract `after_create :method_name` declarations as graph edges with:
- Source: `file.rb::ClassName#after_create`
- Target: `ClassName#method_name`
- Edge type: `middleware`

#### Scenario: after_create with symbol argument
- **WHEN** a Ruby model contains `after_create :create_profile`
- **THEN** the system creates an edge to `User#create_profile`

### Requirement: Extract before_update callbacks as graph edges
The system SHALL extract `before_update :method_name` declarations as graph edges with:
- Source: `file.rb::ClassName#before_update`
- Target: `ClassName#method_name`
- Edge type: `middleware`

#### Scenario: before_update with symbol argument
- **WHEN** a Ruby model contains `before_update :validate_changes`
- **THEN** the system creates an edge to `User#validate_changes`

### Requirement: Extract after_update callbacks as graph edges
The system SHALL extract `after_update :method_name` declarations as graph edges with:
- Source: `file.rb::ClassName#after_update`
- Target: `ClassName#method_name`
- Edge type: `middleware`

#### Scenario: after_update with symbol argument
- **WHEN** a Ruby model contains `after_update :sync_cache`
- **THEN** the system creates an edge to `User#sync_cache`

### Requirement: Extract before_destroy callbacks as graph edges
The system SHALL extract `before_destroy :method_name` declarations as graph edges with:
- Source: `file.rb::ClassName#before_destroy`
- Target: `ClassName#method_name`
- Edge type: `middleware`

#### Scenario: before_destroy with symbol argument
- **WHEN** a Ruby model contains `before_destroy :cleanup_dependents`
- **THEN** the system creates an edge to `User#cleanup_dependents`

### Requirement: Extract after_destroy callbacks as graph edges
The system SHALL extract `after_destroy :method_name` declarations as graph edges with:
- Source: `file.rb::ClassName#after_destroy`
- Target: `ClassName#method_name`
- Edge type: `middleware`

#### Scenario: after_destroy with symbol argument
- **WHEN** a Ruby model contains `after_destroy :remove_from_index`
- **THEN** the system creates an edge to `User#remove_from_index`

### Requirement: Extract after_commit callbacks as graph edges
The system SHALL extract `after_commit :method_name` declarations as graph edges with:
- Source: `file.rb::ClassName#after_commit`
- Target: `ClassName#method_name`
- Edge type: `middleware`

#### Scenario: after_commit with symbol argument
- **WHEN** a Ruby model contains `after_commit :notify_admins`
- **THEN** the system creates an edge to `User#notify_admins`

