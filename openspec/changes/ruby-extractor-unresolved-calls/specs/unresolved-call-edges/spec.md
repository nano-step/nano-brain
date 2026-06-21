## MODIFIED Requirements

### Requirement: Ruby extractor emits call edges for all method invocations
The Ruby call graph extractor SHALL emit call edges for ALL method invocations found in the source code, not just those defined in the same file.

#### Scenario: Cross-file call in controller
- **WHEN** a controller file calls `User.create(params)` and `User` is defined in a different file
- **THEN** the extractor SHALL emit a call edge with TargetNode=`create` (bare name) from the enclosing controller method

#### Scenario: Bare method call (Rails framework)
- **WHEN** a controller file calls `render json: data` (inherited from ApplicationController)
- **THEN** the extractor SHALL emit a call edge with TargetNode=`render` from the enclosing controller method

#### Scenario: Same-file call preserved
- **WHEN** a controller file calls a method defined in the same file
- **THEN** the extractor SHALL emit the call edge exactly as before (no change to same-file behavior)

#### Scenario: Deduplication
- **WHEN** the same cross-file call appears multiple times in the same method
- **THEN** the extractor SHALL emit only one edge (deduplication by enclosing+->+callee)
