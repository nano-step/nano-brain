## ADDED Requirements

### Requirement: Debugging skill for agents
A debugging skill file SHALL be provided that teaches agents the optimal debugging workflow with nano-brain tools.

#### Scenario: Skill detects debugging intent
- **WHEN** user asks a debugging question (e.g., "why is X broken", "payment has wrong tax", "trade stuck")
- **THEN** the agent recognizes the debugging intent from the skill guidance
- **THEN** the agent follows the debugging workflow: search code → search sessions → search config → synthesize

#### Scenario: Skill suggests tool sequence for debugging
- **WHEN** agent detects debugging intent
- **THEN** the skill suggests calling `memory_search(query, mode="debugging")` first
- **THEN** if results are insufficient, the skill suggests `memory_graph` for callers/callees
- **THEN** if more context is needed, the skill suggests `memory_impact` for change analysis

#### Scenario: Skill teaches source-aware result interpretation
- **WHEN** agent receives results with `source` labels
- **THEN** the skill guides the agent to prioritize `source=code` for error paths
- **THEN** the skill guides the agent to check `source=session` for past debugging context
- **THEN** the skill guides the agent to check `source=config` for threshold/TTL values
