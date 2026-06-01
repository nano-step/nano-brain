# Spec — Chunker Hard-Split Safety Net

## ADDED Requirements

### Requirement: Bounded Chunk Size

`chunk.Split` SHALL guarantee that every returned Chunk's content length in bytes is `<= TargetSize + searchWindow/2` (default: 4000 bytes). No chunk shall be emitted whose length exceeds this bound, regardless of input shape.

#### Scenario: Single line longer than TargetSize

- **WHEN** the input contains a single line of 10,000 characters with no internal newlines
- **THEN** `chunk.Split` returns multiple chunks
- **AND** each chunk's `len(Content)` is `<= 4000`
- **AND** the concatenation of all chunk contents (modulo overlap) equals the input

#### Scenario: Content trapped inside an unclosed code fence

- **WHEN** the input contains an unclosed ` ``` ` fence followed by 8,000 characters of code
- **THEN** `chunk.Split` returns multiple chunks
- **AND** each chunk's `len(Content)` is `<= 4000`

#### Scenario: Pathological input — one megabyte of contiguous non-whitespace

- **WHEN** the input is 1,000,000 characters of `x` with no newlines or spaces
- **THEN** `chunk.Split` returns approximately 250+ chunks
- **AND** every chunk's `len(Content)` is `<= 4000`
- **AND** no panic, no infinite loop, completes in bounded time

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

## MODIFIED Requirements

### Requirement: Line-Aware Splitting Preserved

`chunk.Split` SHALL continue to prefer line-boundary splits (via `findSplitPoints`) for any input whose lines are within `TargetSize`. Hard-split SHALL only activate as a post-process for chunks that would otherwise exceed `TargetSize + searchWindow/2`.

#### Scenario: Normal markdown content

- **WHEN** the input is a typical markdown document with paragraphs of 200-500 chars per line and total length 12,000 chars
- **THEN** the chunks returned are identical (byte-for-byte) to chunks produced by the pre-fix algorithm
- **AND** chunk boundaries are at section headers / blank lines as before
