## ADDED Requirements

### Requirement: Strip system prompts from OpenCode sessions
The strip stage SHALL remove the `system` field content from OpenCode session message data, as it contains the full skills list (50K+ chars of noise).

#### Scenario: Message with system prompt
- **WHEN** a rendered OpenCode session markdown contains system prompt text (skills list, agent instructions)
- **THEN** the stripper SHALL remove it entirely, preserving only user/assistant conversation content

### Requirement: Replace tool output with compact placeholders
The strip stage SHALL replace tool call result bodies with short placeholders preserving the tool name and output size.

#### Scenario: Tool result with large output
- **WHEN** a tool result contains output exceeding 200 characters
- **THEN** the stripper SHALL replace the output body with `[tool: {name}, {line_count} lines]`

#### Scenario: Tool result with short output
- **WHEN** a tool result contains output of 200 characters or fewer
- **THEN** the stripper SHALL keep the output as-is

### Requirement: Collapse large code blocks
The strip stage SHALL replace code blocks exceeding 20 lines with a compact placeholder.

#### Scenario: Large code block
- **WHEN** a fenced code block (``` delimited) exceeds 20 lines
- **THEN** the stripper SHALL replace it with `[code block: {line_count} lines, {language}]`

#### Scenario: Small code block
- **WHEN** a fenced code block is 20 lines or fewer
- **THEN** the stripper SHALL keep it as-is

### Requirement: Deduplicate repeated error messages
The strip stage SHALL collapse repeated identical error messages, keeping the first occurrence plus a count.

#### Scenario: Same error repeated 5 times
- **WHEN** the same error message appears 5 times in the session
- **THEN** the stripper SHALL keep the first occurrence and append `(repeated 4 more times)`

### Requirement: Remove base64 and binary data
The strip stage SHALL remove base64-encoded strings and binary data patterns.

#### Scenario: Base64 image data in content
- **WHEN** content contains a base64-encoded string (matching `[A-Za-z0-9+/]{100,}={0,2}`)
- **THEN** the stripper SHALL replace it with `[base64 data removed]`

### Requirement: Preserve high-value content
The strip stage SHALL always preserve: user messages, assistant reasoning text, file paths, decision statements, error messages (first instance), and timestamps.

#### Scenario: Mixed content with reasoning and tool output
- **WHEN** an assistant message contains reasoning text followed by tool calls
- **THEN** the stripper SHALL keep the reasoning text intact and only strip/replace the tool output bodies

### Requirement: Format-specific strip for Claude JSONL
The strip stage SHALL handle Claude JSONL format: strip `tool_output.output` bodies from `tool_result` entries and long commands from `tool_use` entries.

#### Scenario: Claude tool_result with large output
- **WHEN** a Claude `tool_result` entry has `tool_output.output` exceeding 200 characters
- **THEN** the stripper SHALL replace it with `[tool: {tool_name}, {line_count} lines]`

#### Scenario: Claude tool_use with long command
- **WHEN** a Claude `tool_use` entry has `tool_input.command` exceeding 5 lines
- **THEN** the stripper SHALL replace it with `[command: {first_line}...]`
