## ADDED Requirements

### Requirement: Completed dogfooding trackers have a finding disposition record
The repository SHALL retain an evidence record for a completed dogfooding
tracker that lists every reported finding and its final disposition. Each row
MUST identify the associated shipped change, a separately owned follow-up, or
an explicit by-design decision.

#### Scenario: A finding was fixed in a focused change
- **WHEN** a tracker finding shipped through a focused issue and pull request
- **THEN** its disposition record links to that issue and pull request

#### Scenario: A finding requires separately scoped work
- **WHEN** a tracker finding is not safely implementable within the tracker
- **THEN** its disposition record links to the follow-up issue and does not
  describe the finding as fixed

### Requirement: Tracker closure preserves explicit non-fix decisions
The repository SHALL state the rationale when a tracker finding is closed as
won't-fix or by-design.

#### Scenario: A product boundary is retained
- **WHEN** a finding is outside the intended product behavior
- **THEN** the disposition record names that boundary and the tracker closure
  communicates the decision
