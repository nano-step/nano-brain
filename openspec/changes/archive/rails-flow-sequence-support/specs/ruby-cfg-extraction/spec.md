## ADDED Requirements

### Requirement: Ruby control flow graph extraction
The system SHALL extract control flow graphs from Ruby method bodies, producing CFGNode and CFGEdge structures compatible with the existing flow visualization pipeline.

#### Scenario: Extract if/else control flow
- **WHEN** a Ruby method contains an if/else statement
- **THEN** the extractor SHALL produce a decision node for the condition, with yes/no branches leading to separate step nodes for each branch

#### Scenario: Extract loop control flow
- **WHEN** a Ruby method contains a while, until, or for loop
- **THEN** the extractor SHALL produce a decision node for the loop condition, with a loop branch leading to the body and a next branch exiting

#### Scenario: Extract begin/rescue exception handling
- **WHEN** a Ruby method contains a begin/rescue block
- **THEN** the extractor SHALL produce a step node for the begin body, with an error branch leading to the rescue handler

#### Scenario: Skip non-Ruby files
- **WHEN** the extractor is called with a file that does not have a .rb extension
- **THEN** the extractor SHALL return nil and no error

#### Scenario: Handle parse errors gracefully
- **WHEN** the Ruby file contains syntax errors that prevent parsing
- **THEN** the extractor SHALL return a CFG with Status="parse_error" and an empty node/edge list

### Requirement: Ruby CFG extractor implements ControlFlowExtractor interface
The Ruby CFG extractor SHALL implement the ControlFlowExtractor interface with SupportsCFG and ExtractCFGs methods.

#### Scenario: Support .rb extension
- **WHEN** SupportsCFG is called with ".rb"
- **THEN** it SHALL return true

#### Scenario: Reject non-Ruby extensions
- **WHEN** SupportsCFG is called with ".go", ".ts", ".py", or ".java"
- **THEN** it SHALL return false

### Requirement: Ruby CFG respects node limit
The Ruby CFG extractor SHALL enforce the maxCFGNodes limit (default 500) to prevent unbounded growth.

#### Scenario: Truncate large CFGs
- **WHEN** a Ruby method produces more than 500 CFG nodes
- **THEN** the extractor SHALL set Status="truncated" and stop adding nodes after the limit
