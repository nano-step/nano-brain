## ADDED Requirements

### Requirement: Chunk stripped content for map stage
The pipeline SHALL chunk stripped session content using configurable target size (default 4000 chars) with 200-char overlap, splitting at paragraph boundaries when possible.

#### Scenario: Session with 50K stripped chars
- **WHEN** stripped content is 50,000 characters
- **THEN** the pipeline SHALL produce approximately 13 chunks of ~4000 chars each with 200-char overlaps

#### Scenario: Session fitting in single chunk
- **WHEN** stripped content is under 4000 characters
- **THEN** the pipeline SHALL skip map-reduce and send the content directly to a single summarization call

### Requirement: Parallel map with bounded concurrency
The map stage SHALL summarize each chunk independently using parallel goroutines with configurable concurrency (default 3, from config `summarization.concurrency`).

#### Scenario: 15 chunks with concurrency 3
- **WHEN** 15 chunks need summarization and concurrency is 3
- **THEN** the pipeline SHALL process 3 chunks at a time, completing all 15 in approximately 5 batches

#### Scenario: One chunk fails during map
- **WHEN** one chunk's LLM call fails after retries
- **THEN** the pipeline SHALL log a warning, skip that chunk, and continue with remaining chunks

### Requirement: Map prompt extracts structured information
Each map call SHALL use a prompt instructing the LLM to extract: key activities, decisions made, files mentioned, problems encountered, and learnings from that chunk.

#### Scenario: Chunk containing a debugging session
- **WHEN** a chunk describes debugging an embed overflow error
- **THEN** the map summary SHALL mention the problem (embed overflow), files involved, and resolution if present

### Requirement: Reduce merges chunk summaries into final markdown
The reduce stage SHALL merge all chunk summaries into a single structured markdown document with sections: Goal, Decisions Made, Files Touched, Problems Encountered, Key Learnings.

#### Scenario: 10 chunk summaries merged
- **WHEN** 10 chunk summaries are passed to reduce
- **THEN** the output SHALL be a single markdown document with deduplicated information organized into the 5 standard sections

#### Scenario: Chunk summaries exceed model context
- **WHEN** concatenated chunk summaries exceed the model's context window (estimated by character count)
- **THEN** the pipeline SHALL do hierarchical reduce: group summaries in batches of 10, reduce each batch, then do a final reduce on the batch summaries

### Requirement: Summary includes session metadata header
The final summary SHALL include a metadata header with: session title, date, duration, agent type, project path, session ID, and related session links.

#### Scenario: OpenCode session with parent
- **WHEN** summarizing an OpenCode session that has parent_id set
- **THEN** the summary header SHALL include `Parent Session: {parent_title} ({parent_id})` and list sibling sessions

#### Scenario: OpenCode session with children
- **WHEN** summarizing an OpenCode session that is a parent of subagent sessions
- **THEN** the summary header SHALL include `Child Sessions:` listing each child's title, agent type, and session ID

#### Scenario: Claude session without relationships
- **WHEN** summarizing a Claude JSONL session (no parent_id metadata)
- **THEN** the summary header SHALL omit relationship fields and include only title, date, and project path
