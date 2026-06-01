# chunker Specification

## Purpose
TBD - created by archiving change fix-chunker-hard-split. Update Purpose after archive.
## Requirements
### Requirement: Bounded Chunk Size

`chunk.Split` SHALL guarantee that every returned Chunk's content length in bytes is `<= TargetSize + searchWindow/2`. With `DefaultConfig` this evaluates to **3000 bytes** (down from 4000 in the previous spec), matching the embed pipeline's `defaultMaxEmbedChars` budget exactly. Callsites using `DefaultConfig` will therefore never produce chunks that the embed queue must truncate.

#### Scenario: Default-config chunk fits embed budget without truncation

- **WHEN** content is split via `chunk.Split(content, chunk.DefaultConfig())`
- **AND** the embed queue uses its default `defaultMaxEmbedChars = 3000`
- **THEN** every produced chunk has `len(Content) <= 3000`
- **AND** the embed queue does NOT emit `chunk truncated before embedding` for any chunk from this pipeline

#### Scenario: Trace JSON file that previously triggered truncation

- **WHEN** the input is a single-line JSON of approximately 3700 chars (matching the user-reported file shape: harvested trace from issue #300)
- **THEN** `chunk.Split` returns multiple chunks
- **AND** every chunk has `len(Content) <= 3000`
- **AND** no chunk falls in the previously-broken `3000 < len <= 4000` band

#### Scenario: Custom-config chunker still honors its own threshold

- **WHEN** a caller supplies a custom `chunk.Config{TargetSize: 5000, ...}`
- **THEN** the chunker may produce chunks up to `5000 + searchWindow/2 = 5400` bytes
- **AND** it is the caller's responsibility to ensure the downstream embed budget is at least 5400 bytes
- **AND** the embed queue's safety-net truncation still applies if the caller's chunks exceed the queue's `maxChars`

### Requirement: UTF-8 Validity Preservation

Hard-split SHALL NOT cut multibyte UTF-8 sequences mid-rune. Every emitted chunk's `Content` MUST satisfy `utf8.ValidString(Content) == true`.

#### Scenario: Input contains multibyte UTF-8 (CJK, emoji)

- **WHEN** the input is a single 5,000-character line where each character is a 3-byte UTF-8 rune (CJK)
- **THEN** every emitted chunk's `Content` is valid UTF-8 per `utf8.ValidString`
- **AND** no chunk has length 0
- **AND** the concatenation of all chunk contents equals the input byte-for-byte

#### Scenario: Emoji every 100 characters

- **WHEN** the input is a 6,000-character line of ASCII with a 4-byte emoji every 100 chars
- **THEN** every emitted chunk's `Content` is valid UTF-8
- **AND** no emoji appears split across two chunks

### Requirement: Boundary Preference Order

When hard-split is required, the cut boundary SHALL be chosen from the range `[TargetSize * 3/4, TargetSize]` in priority order: blank-line marker (`\n\n`), single newline (`\n`), sentence terminator (`. `, `! `, `? `, `。`), whitespace, then nearest valid UTF-8 rune boundary.

#### Scenario: Prose with sentence boundaries available

- **WHEN** the input is a 5,000-character paragraph with full sentences separated by `. `
- **THEN** the chunk boundaries fall on sentence terminators, not arbitrary positions
- **AND** no sentence is split across two chunks except when the sentence itself exceeds `TargetSize`

#### Scenario: No sub-line boundaries available

- **WHEN** the input is 5,000 contiguous non-whitespace characters (e.g. base64 blob)
- **THEN** the chunk boundary falls at the nearest valid UTF-8 rune start within the upper range
- **AND** the resulting chunks are still `<= TargetSize`

### Requirement: Line-Aware Splitting Preserved

`chunk.Split` SHALL continue to prefer line-boundary splits (via `findSplitPoints`) for any input whose lines are within `TargetSize`. Hard-split SHALL only activate as a post-process for chunks that would otherwise exceed `TargetSize + searchWindow/2`.

#### Scenario: Normal markdown content

- **WHEN** the input is a typical markdown document with paragraphs of 200-500 chars per line and total length 12,000 chars
- **THEN** the chunks returned are identical (byte-for-byte) to chunks produced by the pre-fix algorithm
- **AND** chunk boundaries are at section headers / blank lines as before

