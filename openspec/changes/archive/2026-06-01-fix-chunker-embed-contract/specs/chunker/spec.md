# Spec — Chunker/Embed Contract Tightening

## MODIFIED Requirements

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
