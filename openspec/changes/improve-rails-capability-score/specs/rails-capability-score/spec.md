## ADDED Requirements

### Requirement: Rails graph traversal resolves realistic node names

Rails graph traversal tools SHALL resolve realistic Ruby/Rails node inputs to graph node candidates before traversing edges.

#### Scenario: Trace from class method input
- **WHEN** a caller requests a trace from `BillingWorker#perform`
- **THEN** traversal SHALL consider matching file-qualified source nodes such as `app/workers/billing_worker.rb::BillingWorker#perform`
- **AND** outgoing call edges from the resolved node SHALL be included in the trace result.

#### Scenario: Impact from model method input
- **WHEN** a caller requests impact for `Story#create_print_orders`
- **THEN** traversal SHALL consider matching file-qualified target nodes such as `app/models/story.rb::Story#create_print_orders`
- **AND** incoming call edges to the resolved node SHALL be included in the impact result.

#### Scenario: Bare class input
- **WHEN** a caller requests graph traversal for `DropboxUploadManager`
- **THEN** traversal SHALL consider matching class/module definitions and associated file-qualified nodes before returning an empty result.

#### Scenario: Generic name fanout guard
- **WHEN** a bare name such as `save` or `where` matches more candidates than the configured guard allows
- **THEN** traversal SHALL avoid expanding all candidates blindly
- **AND** it SHALL prefer exact or same-file candidates when available.
- **AND** the default guard threshold SHALL be `8` candidates unless explicitly configured otherwise.

### Requirement: Trace and impact share reconciliation semantics with flow

The trace and impact tools SHALL use reconciliation behavior consistent with flow traversal so a node found by `memory_flow` can be followed by `memory_trace` or analyzed by `memory_impact` without requiring the user to know internal file-qualified IDs.

#### Scenario: Flow-discovered handler is traceable
- **WHEN** `memory_flow` returns a Rails handler or downstream method node
- **THEN** `memory_trace` called with the node's bare `Class#method` form SHALL traverse the same graph neighborhood where matching call edges exist.

#### Scenario: Flow-discovered method is impactable
- **WHEN** `memory_flow` returns a Rails method node
- **THEN** `memory_impact` called with the node's bare `Class#method` form SHALL find incoming callers where matching call edges exist.

### Requirement: Rails flow supports non-HTTP class entries

Rails flow traversal SHALL support background job, worker, and service class entries when no HTTP route entry matches.

#### Scenario: Job class flow entry
- **WHEN** a caller requests flow for `DropboxFolderUpdateJob`
- **AND** no HTTP edge has that exact source node
- **THEN** flow SHALL attempt class/job entry resolution through contains, calls, reconcile, or integration graph edges
- **AND** it SHALL start traversal from matching job or worker graph nodes without requiring an HTTP route edge.

#### Scenario: Service class flow entry
- **WHEN** a caller requests flow for a Rails service class such as `DropboxUploadManager`
- **AND** no HTTP edge has that exact source node
- **THEN** flow SHALL attempt service class entry resolution through graph nodes already indexed for the workspace
- **AND** it SHALL not synthesize edges for code that was not indexed.

#### Scenario: HTTP route remains preferred
- **WHEN** a caller requests flow for `POST /api/v2/stories/sync`
- **THEN** flow SHALL continue to use the matching HTTP route edge as the primary entry.

### Requirement: Ruby constants are indexed as symbols

The Ruby symbol extractor SHALL index constant assignments as `const` symbols.

#### Scenario: Status constant in model concern
- **WHEN** Ruby source defines `STATUS_ORDER_PAID = "paid"`
- **THEN** the symbols API and MCP symbol tool SHALL be able to return `STATUS_ORDER_PAID` as a constant symbol.

#### Scenario: Existing method/class/module extraction preserved
- **WHEN** Ruby source defines methods, classes, and modules
- **THEN** the existing method/class/module symbol extraction behavior SHALL continue to work
- **AND** constants SHALL be additional results, not replacements.

### Requirement: Rails capability benchmark evidence remains privacy-safe

Rails capability benchmark evidence SHALL not commit private workspace names, hashes, raw private filesystem paths, or raw runtime result artifacts.

#### Scenario: Runtime workspace supplied by environment
- **WHEN** the Rails capability benchmark runs against a real workspace
- **THEN** the workspace identifier SHALL be supplied at runtime through environment variables
- **AND** committed benchmark files SHALL use generic placeholders such as `rails-app`.

#### Scenario: Score evidence committed or documented
- **WHEN** benchmark evidence is added to docs or PR text
- **THEN** it SHALL include only sanitized overall/category/task score summaries
- **AND** it SHALL omit private workspace identifiers and raw private paths.

### Requirement: Rails capability score improves measurably

The implementation SHALL improve Rails capability benchmark behavior on the target categories instead of only changing benchmark expectations.

#### Scenario: Score target after implementation
- **WHEN** the benchmark is run after implementation against an indexed Rails workspace
- **THEN** overall recall SHOULD be at least `0.35`
- **AND** `trace` and `impact` category recall SHALL be greater than `0.0`, unless documented blockers explain why graph data is absent.
