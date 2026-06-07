# search-benchmark Specification

## Purpose
TBD - created by archiving change enhanced-code-understanding. Update Purpose after archive.
## Requirements
### Requirement: Benchmark query suite
The system SHALL implement the following behavior:
WHEN any search enhancement is deployed THEN benchmark suite must run and pass quality gate before feature flag is enabled for all traffic. Suite contains 50+ queries across 4 categories.

#### Scenario: Benchmark validates enhancement
- **GIVEN** baseline nDCG@5 = 0.72 (current search without enhancements)
- **WHEN** entity linking enabled and benchmark runs
- **THEN** new nDCG@5 = 0.76 (improvement of 5.5%)
- **AND** quality gate PASSES (improvement >= 3%)
- **AND** entity linking proceeds to full rollout

### Requirement: Quality gate enforcement
The system SHALL implement the following behavior:
WHEN benchmark completes THEN it reports nDCG@5, nDCG@10, Recall@5, Recall@10, MRR. WHEN nDCG@5 drops >5% compared to baseline THEN gate FAILS. WHEN nDCG@5 improves >=3% THEN gate PASSES.

#### Scenario: Regression detected and blocked
- **GIVEN** baseline nDCG@5 = 0.72
- **WHEN** experimental feature enabled and benchmark runs
- **THEN** new nDCG@5 = 0.67 (regression of 7%)
- **AND** quality gate FAILS (regression > 5%)
- **AND** feature flag remains disabled
- **AND** warning logged

### Requirement: Baseline recording
The system SHALL implement the following behavior:
WHEN benchmark is first created THEN baseline metrics are recorded with current search (no enhancements) as reference point.

#### Scenario: Initial baseline capture
- **GIVEN** benchmark suite created with 50 query/result pairs
- **WHEN** run against current search (no enhancements)
- **THEN** baseline recorded: nDCG@5=0.72, Recall@10=0.85, MRR=0.68
- **AND** stored as reference for all future comparisons

### Requirement: Dual-path comparison
The system SHALL implement the following behavior:
WHEN `search.dual_path_enabled = true` THEN each query MUST execute both search pipelines in parallel (using errgroup), log delta metrics to `search_comparison` table, and return enhanced results to the caller. Dual-path mode SHALL run for a maximum of 2 weeks, after which it MUST be committed (enable enhanced path permanently) or reverted (disable enhanced features). The dual-path MUST NOT block queries — if the baseline path times out (>500ms), only enhanced results are logged.

#### Scenario: Side-by-side comparison during rollout
- **GIVEN** dual-path enabled (baseline + enhanced)
- **WHEN** benchmark runs
- **THEN** reports both: baseline nDCG@5=0.72, enhanced nDCG@5=0.78
- **AND** delta clearly shown: +8.3% improvement

#### Scenario: Dual-path does not increase user-facing latency
- **GIVEN** dual_path_enabled = true
- **WHEN** user query arrives
- **THEN** enhanced path returns result to user immediately
- **AND** baseline path runs in background goroutine for comparison logging only
- **AND** user-facing latency equals enhanced path latency (not sum of both)

