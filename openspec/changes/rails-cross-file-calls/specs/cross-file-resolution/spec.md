## ADDED Requirements

### Requirement: Cross-file call resolution
The system SHALL resolve cross-file method calls by building a class→file index from contains edges and rewriting bare call targets to qualified format.

#### Scenario: Resolve controller→model call
- **WHEN** a controller file contains `User.where(active: true)`
- **THEN** the resolver SHALL emit a calls edge with TargetNode=`user.rb::where` (qualified)

#### Scenario: Resolve controller→service call
- **WHEN** a controller file contains `PaymentProcessor.new.process(order)`
- **THEN** the resolver SHALL emit a calls edge with TargetNode=`payment_processor.rb::process` (qualified)

#### Scenario: Unresolvable call emits bare + metadata
- **WHEN** a call target cannot be resolved to any known class
- **THEN** the resolver SHALL emit a calls edge with TargetNode=`method_name` (bare) and Metadata=`{"unresolved": true}`

#### Scenario: Ambiguous class resolution
- **WHEN** two files define the same class name (class reopening or different namespaces)
- **THEN** the resolver SHALL emit calls edges to ALL matching files with Metadata=`{"ambiguous": true}`

#### Scenario: ActiveRecord class-level method
- **WHEN** a file contains `User.create(name: "test")`
- **THEN** the resolver SHALL emit a calls edge with TargetNode=`user.rb::create` (qualified)

#### Scenario: ActiveRecord instance method via association
- **WHEN** a file contains `@user.orders.build`
- **THEN** the resolver SHALL emit a calls edge with TargetNode=`order.rb::build` (qualified)

### Requirement: Resolver runs as post-extraction pass
The resolver SHALL run after per-file extraction in the watcher, using all contains edges to build the class→file index.

#### Scenario: Index built from contains edges
- **WHEN** all .rb files have been extracted
- **THEN** the class→file index SHALL map each class name to all files containing that class definition

#### Scenario: Resolver rewrites existing edges
- **WHEN** the resolver runs on extracted edges
- **THEN** previously-emitted bare calls edges SHALL be rewritten to qualified format where resolvable

### Requirement: Rails-only scope
Cross-file resolution SHALL only activate when Rails framework is detected (via RequiresFrameworks gate).

#### Scenario: Rails project
- **WHEN** the workspace contains a Gemfile with `gem 'rails'`
- **THEN** cross-file resolution SHALL be active

#### Scenario: Non-Rails Ruby
- **WHEN** the workspace does NOT contain Rails
- **THEN** cross-file resolution SHALL be skipped (same-file only, current behavior)
