## MODIFIED Requirements

### Requirement: Node format includes class name
All Ruby call edges SHALL use the node format `file.rb::ClassName#methodName` for SourceNode, where ClassName is the enclosing class or module. This replaces the previous `file.rb::methodName` format.

#### Scenario: Controller method call
- **WHEN** `UsersController#create` calls `User.create(params)`
- **THEN** the extractor SHALL emit a calls edge with SourceNode=`app/controllers/users_controller.rb::UsersController#create`

#### Scenario: Model method call
- **WHEN** `User#save` calls `valid?` (same-file call)
- **THEN** the extractor SHALL emit a calls edge with SourceNode=`app/models/user.rb::User#save` and TargetNode=`valid?` (bare, same-file)

#### Scenario: Nested class method
- **WHEN** `Api::V1::UsersController#index` calls `User.all`
- **THEN** the extractor SHALL emit a calls edge with SourceNode=`app/controllers/api/v1/users_controller.rb::Api::V1::UsersController#index`

#### Scenario: Method outside any class
- **WHEN** a top-level method calls another method
- **THEN** the extractor SHALL use the file name as a surrogate class: SourceNode=`file.rb::<top>#methodName`

#### Scenario: Singleton method (class method)
- **WHEN** a file contains `def self.find_by_email(email)` inside `User` class
- **THEN** the extractor SHALL emit a contains edge with TargetNode=`app/models/user.rb::User.find_by_email`

## ADDED Requirements

### Requirement: Cross-file resolver handles bare method calls
The cross-file resolver SHALL resolve bare method calls (not just `ClassName.method` patterns) by scanning AST nodes and matching against the class index.

#### Scenario: Bare instance method call
- **WHEN** a controller contains `@user.save` and `User` is defined in `app/models/user.rb`
- **THEN** the resolver SHALL emit a calls edge with TargetNode=`app/models/user.rb::User#save`

#### Scenario: Bare class method call
- **WHEN** a service contains `Order.where(status: :pending)` and `Order` is in `app/models/order.rb`
- **THEN** the resolver SHALL emit a calls edge with TargetNode=`app/models/order.rb::Order#where`

#### Scenario: Unresolvable call
- **WHEN** a call target cannot be resolved to any known class
- **THEN** the resolver SHALL emit a calls edge with TargetNode=`method_name` (bare) and Metadata=`{"unresolved": true}`

#### Scenario: Ambiguous class resolution
- **WHEN** two files define the same class name (class reopening)
- **THEN** the resolver SHALL emit calls edges to ALL matching files with Metadata=`{"ambiguous": true}`

### Requirement: Rails DSL association extraction
The `RailsDSLEdgeExtractor` SHALL extract Rails association declarations as calls edges with DSL metadata.

#### Scenario: has_many association
- **WHEN** a model file contains `has_many :orders`
- **THEN** the extractor SHALL emit a calls edge with SourceNode=`app/models/user.rb::User#has_many`, TargetNode=`Order`, Kind=`calls`, and Metadata=`{"dsl": "association", "type": "has_many", "target_model": "Order"}`

#### Scenario: belongs_to association
- **WHEN** a model file contains `belongs_to :user`
- **THEN** the extractor SHALL emit a calls edge with SourceNode=`app/models/order.rb::Order#belongs_to`, TargetNode=`User`, Kind=`calls`, and Metadata=`{"dsl": "association", "type": "belongs_to", "target_model": "User"}`

#### Scenario: has_one association
- **WHEN** a model file contains `has_one :profile`
- **THEN** the extractor SHALL emit a calls edge with Metadata=`{"dsl": "association", "type": "has_one", "target_model": "Profile"}`

#### Scenario: has_and_belongs_to_many
- **WHEN** a model file contains `has_and_belongs_to_many :tags`
- **THEN** the extractor SHALL emit a calls edge with Metadata=`{"dsl": "association", "type": "has_and_belongs_to_many", "target_model": "Tag"}`

### Requirement: Rails DSL callback extraction
The `RailsDSLEdgeExtractor` SHALL extract Rails callback declarations as middleware edges.

#### Scenario: before_action in controller
- **WHEN** a controller contains `before_action :set_user`
- **THEN** the extractor SHALL emit a middleware edge with SourceNode=`app/controllers/users_controller.rb::UsersController#before_action`, TargetNode=`set_user`, Kind=`middleware`, and Metadata=`{"dsl": "callback", "type": "before_action"}`

#### Scenario: after_commit in model
- **WHEN** a model contains `after_commit :notify_slack, on: :create`
- **THEN** the extractor SHALL emit a middleware edge with Metadata=`{"dsl": "callback", "type": "after_commit", "on": "create"}`

#### Scenario: before_save in model
- **WHEN** a model contains `before_save :normalize_email`
- **THEN** the extractor SHALL emit a middleware edge with Metadata=`{"dsl": "callback", "type": "before_save"}`

#### Scenario: Callback with inline block
- **WHEN** a controller contains `before_action { |c| c.require_login }`
- **THEN** the extractor SHALL emit a middleware edge with TargetNode=`(block)` and Metadata=`{"dsl": "callback", "type": "before_action"}`

### Requirement: Rails DSL concern extraction
The `RailsDSLEdgeExtractor` SHALL extract `include` and `extend` declarations as calls edges.

#### Scenario: include concern
- **WHEN** a model contains `include Authenticatable`
- **THEN** the extractor SHALL emit a calls edge with SourceNode=`app/models/user.rb::User#include`, TargetNode=`Authenticatable`, Kind=`calls`, and Metadata=`{"dsl": "concern", "type": "include", "target_module": "Authenticatable"}`

#### Scenario: extend module
- **WHEN** a class contains `extend Findable`
- **THEN** the extractor SHALL emit a calls edge with Metadata=`{"dsl": "concern", "type": "extend", "target_module": "Findable"}`

#### Scenario: include with namespace
- **WHEN** a class contains `include Devise::Models::Authenticatable`
- **THEN** the extractor SHALL emit a calls edge with TargetNode=`Devise::Models::Authenticatable`

### Requirement: Sidekiq job dispatch extraction
The `RailsDSLEdgeExtractor` SHALL detect Sidekiq job dispatch calls and emit integration edges.

#### Scenario: perform_async
- **WHEN** source code contains `OrderProcessor.perform_async(order.id)`
- **THEN** the extractor SHALL emit an integration edge with SourceNode=`caller_file.rb::CallerClass#method`, TargetNode=`OrderProcessor`, Kind=`integration`, and Metadata=`{"dsl": "sidekiq", "method": "perform_async"}`

#### Scenario: perform_in
- **WHEN** source code contains `OrderProcessor.perform_in(1.hour, order.id)`
- **THEN** the extractor SHALL emit an integration edge with Metadata=`{"dsl": "sidekiq", "method": "perform_in"}`

#### Scenario: Instance-level perform_async
- **WHEN** source code contains `processor.perform_async(args)` and `processor` is of type `OrderProcessor`
- **THEN** the extractor SHALL emit an integration edge with Metadata=`{"dsl": "sidekiq", "method": "perform_async"}`

### Requirement: Singleton method CFG extraction
The Ruby CFG extractor SHALL extract control-flow graphs for `singleton_method` (class method) definitions, not just regular `method` definitions.

#### Scenario: class method with if/else
- **WHEN** a file contains `def self.find_by_email(email) ... if condition ... else ... end ... end`
- **THEN** the extractor SHALL emit a CFG with Entry=`file.rb::find_by_email`, including decision nodes for the if/else

#### Scenario: scope definition
- **WHEN** a model contains `scope :active, -> { where(active: true) }`
- **THEN** the extractor SHALL emit a CFG for the scope's lambda body (if it has branching)

### Requirement: Expanded Rails convention paths
The class→file index SHALL resolve class names using expanded Rails directory conventions.

#### Scenario: Service class resolution
- **WHEN** the class index cannot find `PaymentProcessor` in contains edges
- **THEN** the convention fallback SHALL return `app/services/payment_processor.rb`

#### Scenario: Job class resolution
- **WHEN** the class index cannot find `OrderJob` in contains edges
- **THEN** the convention fallback SHALL return `app/jobs/order_job.rb`

#### Scenario: Mailer class resolution
- **WHEN** the class index cannot find `WelcomeMailer` in contains edges
- **THEN** the convention fallback SHALL return `app/mailers/welcome_mailer.rb`

#### Scenario: Worker class resolution
- **WHEN** the class index cannot find `EmailWorker` in contains edges
- **THEN** the convention fallback SHALL return `app/workers/email_worker.rb`

### Requirement: Rails-only scope
All Rails DSL extraction SHALL only activate when the Rails framework is detected (via `RequiresFrameworks` gate).

#### Scenario: Rails project
- **WHEN** the workspace contains a Gemfile with `gem 'rails'`
- **THEN** Rails DSL extraction SHALL be active

#### Scenario: Non-Rails Ruby
- **WHEN** the workspace does NOT contain Rails
- **THEN** Rails DSL extraction SHALL be skipped

### Requirement: Forced reindex on node format change
The system SHALL perform a forced reindex of all Ruby files when the node format changes.

#### Scenario: Existing workspace upgrade
- **WHEN** a workspace is indexed with the new node format
- **THEN** all existing Ruby graph edges SHALL be deleted and rebuilt with the new format

#### Scenario: New workspace
- **WHEN** a new workspace is indexed for the first time
- **THEN** all Ruby graph edges SHALL use the new node format from the start
