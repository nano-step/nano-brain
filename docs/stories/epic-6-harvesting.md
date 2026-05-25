---
epic: 6
title: Session Harvesting
status: draft
depends_on: Epic 2 (Ingestion Pipeline)
---

# Epic 6: Session Harvesting — User Stories

**Epic summary:** OpenCode and Claude Code sessions are auto-harvested on a configurable poll interval. SHA-256 deduplication prevents duplicate documents. Harvested sessions land in the `sessions` collection and are searchable after the next embedding cycle.

**FRs covered:** FR-1, FR-2, FR-3, FR-4, FR-5, FR-6, FR-80
**ARs applied:** AR-5 (errgroup + context for harvester goroutine)
**NFRs enforced:** NFR-1 (concurrent harvesters safe), NFR-4 (dedup via content hash)
**Package:** `internal/harvest/`

---

#### Story 6.1: OpenCode Session Harvester

**Description:** As a developer, I want nano-brain to automatically discover and ingest all OpenCode session files from the configured directory so that past coding sessions are searchable without any manual API calls.

The harvester reads JSON session files from `harvester.opencode.session_dir`, sorts each session's messages chronologically, and renders them as a markdown document with YAML front-matter (session ID, timestamps, message count). The rendered document is inserted into the `sessions` collection via the Epic 2 ingestion pipeline. The harvester runs as a background goroutine managed by `errgroup` and does not block the HTTP API.

**Covers:** FR-1, FR-4
**Applies:** AR-5
**Complexity:** M

**Acceptance Criteria:**

- Given `harvester.opencode.session_dir` points to a directory containing one or more OpenCode JSON session files,
  When the harvester goroutine completes its first poll cycle,
  Then each session file is converted to a markdown document with YAML front-matter (`session_id`, `source: opencode`, `message_count`, `created_at`) and all messages present in chronological order.

- Given a session markdown document has been rendered,
  When it is submitted to the ingestion pipeline,
  Then it is stored in the `sessions` collection under the workspace derived from `harvester.opencode.session_dir`.

- Given the harvester goroutine is running,
  When `GET /api/status` is called,
  Then the response shows the harvester as active and does not indicate the HTTP API is blocked.

- Given `harvester.opencode.session_dir` is missing from config,
  When nano-brain starts,
  Then a descriptive config error is logged and the harvester goroutine does not start (other goroutines are unaffected).

---

#### Story 6.2: Claude Code Session Harvester

**Description:** As a developer who uses Claude Code alongside OpenCode, I want nano-brain to optionally harvest Claude Code session files so that my Claude Code conversations are also indexed without extra setup.

The harvester reads JSONL session files from `harvester.claude_code.session_dir` only when `harvester.claude_code.enabled = true`. It parses the JSONL format (one JSON object per line), sorts messages chronologically, and renders them to markdown using the same front-matter schema as the OpenCode harvester (with `source: claude_code`). The rendered documents are inserted into the `sessions` collection via the same ingestion pipeline.

**Covers:** FR-2, FR-4
**Applies:** AR-5
**Complexity:** S

**Acceptance Criteria:**

- Given `harvester.claude_code.enabled = false` (the default),
  When the harvester runs,
  Then no attempt is made to read `harvester.claude_code.session_dir` and no error is emitted.

- Given `harvester.claude_code.enabled = true` and `harvester.claude_code.session_dir` points to a directory with JSONL files,
  When the harvester completes a poll cycle,
  Then each JSONL file is rendered as a markdown document with `source: claude_code` in its front-matter and all messages present in chronological order.

- Given a Claude Code session file exists with 10 messages,
  When it is harvested,
  Then the rendered markdown contains exactly 10 message blocks in the order they appear in the JSONL file.

- Given `harvester.claude_code.enabled = true` but `session_dir` does not exist,
  When the harvester runs,
  Then the error is logged at warn level and the cycle continues without crashing.

---

#### Story 6.3: SHA-256 Content-Addressed Deduplication

**Description:** As an operator, I want the harvester to skip sessions that have not changed since the last harvest so that re-running harvest never creates duplicate documents and dedup state survives process restarts.

Each rendered markdown document is SHA-256-hashed before any database write. The hash is stored in the `documents` table alongside the document content (reusing the same `content_hash` column used by the Epic 2 ingestion pipeline). On each harvest cycle the harvester computes the hash of the rendered output and checks whether a document with that hash already exists in the `sessions` collection. If it does, the session is skipped entirely. Because hash state lives in PostgreSQL, it survives server restarts with no additional persistence layer.

**Covers:** FR-3
**Applies:** AR-5
**Complexity:** S

**Acceptance Criteria:**

- Given a session file has been harvested and stored,
  When the harvester runs again on the same unchanged session file,
  Then no new document is inserted and the `documents` table row count for that session remains 1.

- Given the nano-brain server is restarted after a harvest cycle,
  When the harvester runs its first cycle after restart,
  Then previously harvested unchanged sessions are skipped (dedup survives restart).

- Given a session file is updated between two harvest cycles (new messages appended),
  When the harvester processes the updated file,
  Then a new document is inserted (or the existing document is updated via upsert) reflecting the new content, and the old hash is superseded.

- Given the harvester runs on a directory with 100 sessions all identical to the previous cycle,
  When the cycle completes,
  Then zero new database writes occur (0 inserts, 0 updates).

---

#### Story 6.4: Configurable Poll Interval and Harvester Goroutine Lifecycle

**Description:** As an operator, I want the harvest frequency and goroutine lifecycle to be configurable and properly supervised so that I can tune latency versus load, and so the harvester shuts down cleanly when the server stops.

The poll interval defaults to 120 seconds and is set via `intervals.session_poll`. The harvester goroutine is started in `main()` under an `errgroup` alongside the file watcher and embedder. When the root context is cancelled (SIGTERM or SIGINT), the harvester finishes its current cycle and exits. `GET /api/status` reflects the harvest configuration.

**Covers:** FR-5
**Applies:** AR-5
**Complexity:** S

**Acceptance Criteria:**

- Given `intervals.session_poll = 30` in config,
  When the server starts and two harvest cycles complete,
  Then the gap between cycle completions is approximately 30 seconds (within 5-second tolerance).

- Given `intervals.session_poll` is not set,
  When the server starts,
  Then the harvester uses a 120-second poll interval.

- Given the server receives SIGTERM during an active harvest cycle,
  When the cycle completes,
  Then the harvester goroutine exits cleanly and `errgroup.Wait()` returns without error within 5 seconds.

- Given `intervals.session_poll = 0` or a negative value,
  When the server starts,
  Then a descriptive config error is returned and startup halts.

---

#### Story 6.5: Concurrent Harvester Safety and CLI Trigger

**Description:** As an operator running multiple nano-brain instances against the same PostgreSQL cluster, I want concurrent harvesters to produce no duplicates or data corruption. I also want a CLI command to trigger an immediate harvest cycle without waiting for the poll interval.

Concurrent safety relies on PostgreSQL's `ON CONFLICT DO NOTHING` upsert semantics (the same mechanism used throughout Epic 2). No application-level global lock is used or needed. The CLI command `nano-brain harvest` calls `POST /api/harvest` (or equivalent internal trigger), which runs one complete harvest cycle synchronously and returns when done.

**Covers:** FR-6, FR-80
**Applies:** AR-5
**Complexity:** M

**Acceptance Criteria:**

- Given two harvester goroutines start simultaneously against the same PostgreSQL database with the same session directory,
  When both complete their first poll cycle,
  Then the `documents` table contains exactly the same number of session documents as one harvester would produce — no duplicates, no constraint violations.

- Given the same scenario with `go test -race` enabled,
  When both goroutines run concurrently,
  Then the race detector reports no data races.

- Given the server is running,
  When the user runs `nano-brain harvest` from the CLI,
  Then one complete harvest cycle executes immediately and the CLI returns exit code 0 on success.

- Given `nano-brain harvest` is run with `--json`,
  When the cycle completes,
  Then the output is valid JSON containing `{harvested: N, skipped: M, errors: K}` where N, M, K are non-negative integers.

- Given two `nano-brain harvest` commands run simultaneously (simulating concurrent operators),
  When both complete,
  Then the document count matches what a single run would produce — no extra rows inserted.
