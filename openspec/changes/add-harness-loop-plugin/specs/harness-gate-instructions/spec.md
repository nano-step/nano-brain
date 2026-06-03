## ADDED Requirements

### Requirement: Config SHALL declare per-gate instruction docs and skills

The harness config SHALL support a `gate_instructions` object mapping each gate name to optional `doc` (file path) and `skills` (array of skill names). The plugin SHALL embed these references in continuation prompts so agents have project-specific protocol guidance when fixing failures.

#### Scenario: Config declares full instruction mapping for a gate
- **WHEN** `harness.config.json` contains `"gate_instructions": {"e2e": {"doc": "docs/harness/gates/e2e.md", "skills": ["e2e-test-generator", "playwright"]}}`
- **THEN** every continuation prompt emitted for the "e2e" gate SHALL include both the doc path and the skill list, formatted per the harness-loop-plugin spec

#### Scenario: Config declares doc only, no skills
- **WHEN** `gate_instructions.<gate>` contains `{"doc": "path/to/doc.md"}` with no `skills` field
- **THEN** the continuation prompt SHALL include the doc reference and SHALL omit the "Load skills" section entirely

#### Scenario: Config declares skills only, no doc
- **WHEN** `gate_instructions.<gate>` contains `{"skills": ["review-work"]}` with no `doc` field
- **THEN** the plugin SHALL apply the convention-based fallback path (see below) before deciding the doc section; if the fallback file does not exist, the prompt SHALL include only the skills section

#### Scenario: Config has no entry for a gate
- **WHEN** the loop reaches a gate that has no entry in `gate_instructions`
- **THEN** the plugin SHALL apply the convention-based fallback path and use empty skills array

### Requirement: Plugin SHALL apply a convention-based fallback path for missing doc

When `gate_instructions.<gate>.doc` is absent, the plugin SHALL look for a file at the conventional path `docs/harness/gates/<gate>.md` (relative to project root) and use it if it exists.

#### Scenario: Fallback file exists
- **WHEN** config has no `doc` for gate "pre-merge" and `docs/harness/gates/pre-merge.md` exists
- **THEN** the continuation prompt SHALL reference `docs/harness/gates/pre-merge.md` as if it were explicitly configured

#### Scenario: Fallback file does not exist
- **WHEN** config has no `doc` for gate "pre-merge" and `docs/harness/gates/pre-merge.md` does not exist
- **THEN** the plugin SHALL embed a warning in the continuation prompt: "⚠️ No protocol doc found for gate <gate>. Use general best practices; consider creating docs/harness/gates/<gate>.md."

#### Scenario: Explicit doc path overrides convention
- **WHEN** config sets `doc: "custom/path/e2e-spec.md"` for gate "e2e"
- **THEN** the plugin SHALL use the explicit path and SHALL NOT consult the convention fallback (even if the convention path also exists)

### Requirement: Plugin SHALL validate doc paths on loop start, flexibly

On `/harness-on`, the plugin SHALL check that every configured `doc` path resolves to an existing file. Missing files SHALL emit warnings but SHALL NOT block loop start. This is the "flexible mode" default.

#### Scenario: All configured docs exist
- **WHEN** all `gate_instructions.<gate>.doc` paths resolve to existing files (or fallback files exist for gates without explicit docs)
- **THEN** the plugin SHALL start the loop silently without warnings

#### Scenario: One configured doc is missing
- **WHEN** `gate_instructions.e2e.doc` is set to "docs/harness/gates/e2e.md" but that file does not exist
- **THEN** the plugin SHALL emit toast "⚠️ Gate doc missing: docs/harness/gates/e2e.md — agent will proceed without project protocol for gate e2e" and SHALL start the loop

#### Scenario: Multiple docs missing
- **WHEN** several configured doc paths do not exist
- **THEN** the plugin SHALL emit one consolidated toast listing all missing paths, then start the loop

#### Scenario: Strict mode enabled
- **WHEN** config has `strict_instructions: true` and any configured or convention-resolved doc is missing
- **THEN** the plugin SHALL refuse to start, print "Strict instructions enabled but docs missing: <list>", and require either the missing files to be created or strict mode to be disabled

### Requirement: Plugin SHALL validate skill names against the OpenCode registry when possible

For each skill name in `gate_instructions.<gate>.skills`, the plugin SHALL attempt to verify the skill exists in OpenCode's skill registry on loop start. Best-effort; silent fallthrough if the registry API is unavailable.

#### Scenario: All skills exist in registry
- **WHEN** every skill name in every gate's skill list resolves to a registered OpenCode skill
- **THEN** the plugin SHALL start the loop silently

#### Scenario: Unknown skill in config
- **WHEN** `gate_instructions.pre-merge.skills` includes "review-work-nonexistent" which is not registered
- **THEN** the plugin SHALL emit warning "⚠️ Unknown skill referenced in gate_instructions: review-work-nonexistent (gate=pre-merge) — agent will skip loading this skill" and SHALL start the loop

#### Scenario: Registry API unavailable
- **WHEN** the OpenCode runtime version does not expose a skill registry query API
- **THEN** the plugin SHALL skip skill validation entirely, log a debug message, and start the loop without warnings

### Requirement: Continuation prompts SHALL embed doc reference imperatively, not as a suggestion

When a gate has an effective doc path (configured or convention-resolved), the continuation prompt SHALL phrase the reference as a mandatory step, not as a hint.

#### Scenario: Doc reference phrasing
- **WHEN** the continuation prompt is built for gate "e2e" with doc "docs/harness/gates/e2e.md"
- **THEN** the prompt SHALL include the literal text:
  ```
  📖 Read project's gate protocol FIRST (mandatory):
     docs/harness/gates/e2e.md
  ```
  and SHALL NOT phrase it as optional ("you may read...")

#### Scenario: No doc available phrasing
- **WHEN** the continuation prompt is built for a gate with no configured doc and no convention fallback
- **THEN** the prompt SHALL include the literal text:
  ```
  ⚠️ No protocol doc found for gate <gate>. Use general best practices.
  ```

### Requirement: Skill references SHALL be embedded as load directives

When a gate has skills configured, the continuation prompt SHALL list them under a "Load skills" section so the agent knows to invoke `skill(name="...")` before fixing.

#### Scenario: Single skill
- **WHEN** `gate_instructions.pre-merge.skills` = `["review-work"]`
- **THEN** the prompt SHALL include:
  ```
  🔧 Load skills before attempting fix:
     - review-work
  ```

#### Scenario: Multiple skills
- **WHEN** skills array has multiple entries
- **THEN** each SHALL appear as its own `- <skill>` bullet under the "Load skills" header

#### Scenario: Empty or omitted skills
- **WHEN** skills is `[]` or absent
- **THEN** the "Load skills" section SHALL be entirely omitted from the prompt (no empty header)

### Requirement: Doc content SHALL NOT be inlined into prompts

The plugin SHALL embed only the doc *path*, not the doc's content, in continuation prompts. Agents read the doc on demand via the standard `read` tool.

#### Scenario: 500-line doc not inlined
- **WHEN** the configured doc is `docs/harness/gates/e2e.md` containing 500 lines of protocol detail
- **THEN** the continuation prompt SHALL include only the path string `docs/harness/gates/e2e.md` and SHALL NOT include any portion of the file's content

#### Scenario: Doc content read on first iteration only
- **WHEN** the agent encounters the doc reference in iteration 3 and reads the file
- **THEN** the doc content enters the agent's context once via the `read` tool result; subsequent iterations continue to embed only the path, and the agent benefits from already having the content in conversation history

### Requirement: Instruction mapping SHALL be project-owned, not plugin-owned

The plugin SHALL ship no built-in instruction docs. Every doc referenced by any project SHALL live in the project's own repository. The plugin's role is limited to: declaring the config schema, validating paths, embedding references in prompts.

#### Scenario: Nano-brain ships its own gate docs
- **WHEN** nano-brain adopts the plugin
- **THEN** nano-brain creates `docs/harness/gates/pre-work.md`, `docs/harness/gates/smoke-e2e.md`, etc., in its own repo

#### Scenario: Capyhome ships its own gate docs
- **WHEN** capyhome adopts the plugin
- **THEN** capyhome creates its own docs at conventional path or custom location, referenced from its own `harness.config.json`

#### Scenario: Plugin repository contains no gate docs
- **WHEN** inspecting the plugin source (`.opencode/plugin/harness-loop/`)
- **THEN** there SHALL be no `gates/` directory or any project-specific protocol documents inside the plugin; only example references in the plugin's README
