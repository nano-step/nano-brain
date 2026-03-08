## MODIFIED Requirements

### Requirement: Session harvester processes sessions incrementally
The harvester SHALL track the number of messages previously harvested per session. When a session has new messages, the harvester SHALL read only the new messages and append them to the existing markdown file. When message count is unchanged, the harvester SHALL skip the session entirely.

#### Scenario: First harvest of a new session
- **WHEN** a session file has no entry in harvest state
- **THEN** the harvester reads all messages, writes full markdown with frontmatter, and saves state with `messageCount` equal to total messages

#### Scenario: Session with new messages (incremental)
- **WHEN** a session file has `messageCount: 5` in state and now has 7 messages
- **THEN** the harvester reads parts for only messages 6 and 7, appends formatted markdown to existing file, and updates state to `messageCount: 7`

#### Scenario: Session with unchanged message count
- **WHEN** a session file has `messageCount: 5` in state and still has 5 messages (even if mtime changed)
- **THEN** the harvester skips the session entirely (no file I/O, no state change)

#### Scenario: Session with no messages (subagent/empty)
- **WHEN** a session has 0 messages or all messages have empty text content
- **THEN** the harvester marks the session as `skipped: true` in state immediately on first detection, preventing retry loops

#### Scenario: Backward compatibility with existing state
- **WHEN** harvest state contains entries without `messageCount` field (from previous version)
- **THEN** the harvester treats missing `messageCount` as 0 and performs a full harvest on first run

## ADDED Requirements

### Requirement: Harvest state tracks message count
The `HarvestStateEntry` interface SHALL include an optional `messageCount` field that records how many messages were last harvested for each session.

#### Scenario: State entry after successful harvest
- **WHEN** a session with 10 messages is successfully harvested
- **THEN** the state entry contains `{ mtime: <number>, messageCount: 10 }`
