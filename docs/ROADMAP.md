# nano-brain Roadmap

> Draft — brainstorm phase. Not finalized.
> Last updated: 2026-05-25

---

## Vision

nano-brain là persistent memory + code intelligence layer cho AI agents.
Goal: agent biết context của project, lịch sử decision, và có thể dự đoán/chuẩn bị trước task tiếp theo.

---

## Pillar 1: Code Intelligence

**What:** Hiểu codebase như một senior engineer.

| Feature | Description | Status |
|---|---|---|
| File indexing | Watch + chunk + embed toàn bộ source files | ✅ partial (watcher bug) |
| Symbol extraction | Functions, types, interfaces, constants | 🔲 |
| Knowledge graph | Module → function → dependency relationships | 🔲 |
| Impact analytics | Thay đổi X → affects Y, Z (cross-file) | 🔲 |
| Call chain tracing | Trace execution path từ entry point | 🔲 |

---

## Pillar 2: Session Harvesting

**What:** Thu thập + summarize sessions từ AI tools, không lưu raw.

### 2a. OpenCode Harvester (SQLite)

OpenCode đã migrate sang SQLite (`~/.local/share/opencode/opencode.db`).
Current JSONL-based harvester không hoạt động.

| Field | Value |
|---|---|
| DB path | `~/.local/share/opencode/opencode.db` |
| Tables | `session`, `message`, `part`, `project`, `todo` |
| Data | 6,744 sessions, 249,614 messages (trên máy user) |

**Flow:**
```
opencode.db → query sessions/messages → LLM summary → chunk → embed → index
```

**Config (proposed):**
```yaml
harvester:
  opencode:
    db_path: ~/.local/share/opencode/opencode.db
    output_dir: ~/.nano-brain/sessions/opencode/   # user-configurable
    since: 2026-01-01                              # incremental
```

### 2b. Claude Code Harvester (JSONL)

Claude Code lưu transcripts dưới dạng JSONL.

```
~/.claude/
├── transcripts/ses_*.jsonl    # Full conversation history
├── metrics/costs.jsonl        # Token usage / cost tracking
├── projects/
│   └── <project-hash>/
│       └── memory/            # Per-project auto-memory
├── history.jsonl              # Command history
└── sessions/                  # Active session state
```

**Flow:**
```
ses_*.jsonl → parse messages → LLM summary → chunk → embed → index
```

**Config (proposed):**
```yaml
harvester:
  claude:
    transcripts_dir: ~/.claude/transcripts/
    output_dir: ~/.nano-brain/sessions/claude/   # user-configurable
    include_costs: true                          # harvest costs.jsonl too
```

### 2c. Shared Harvesting Principles

- **No raw storage** — LLM summarizes session trước khi lưu
- **User-configurable output folder** — không hardcode path
- **Incremental** — chỉ harvest sessions mới (track last-harvested timestamp)
- **Dedup** — skip sessions đã harvested (by session ID)
- **LLM summary format:**
  - What was the goal?
  - What decisions were made?
  - What files were touched?
  - What problems were encountered?
  - Key learnings / patterns

---

## Pillar 3: Memory

**What:** Persistent cross-session memory cho AI agents.

| Feature | Description | Status |
|---|---|---|
| Write memory | `nano-brain write "..."` | ✅ |
| Semantic search | `nano-brain query "..."` | ✅ |
| Tag-based filter | `--tags decision,auth` | ✅ |
| Supersede | Replace stale memory entries | ✅ |
| Auto-memory from sessions | Extract decisions từ harvested sessions | 🔲 |

---

## Pillar 4: Self-Learning & Prediction

**What:** Học pattern từ user behavior → chuẩn bị context trước.

> ⚠️ Cần discuss thêm với user về scope/approach.

### 4a. Pattern Learning
- Phân tích prompt history từ harvested sessions
- Nhận diện recurring workflows (e.g., "user thường fix bug → run test → commit")
- Build user-specific workflow graph

### 4b. Proactive Context Pre-loading
- Dựa trên current task → dự đoán task tiếp theo
- Pre-fetch relevant code symbols, memory entries, past decisions
- Surface as "you might need next: ..."

### 4c. Self-Lesson Learn
- Sau mỗi session: extract lessons ("what worked", "what failed")
- Store as tagged memory entries
- Surface relevant lessons khi bắt đầu similar task

### 4d. Auto-execution (Stretch)
- Nano-brain tự trigger next step mà không cần user prompt
- Requires: high confidence prediction + user opt-in flag
- Risk: false positives → cần confidence threshold

---

## Implementation Order (Proposed)

```
Phase 1 — Foundation (Now)
  ├── Fix watcher directory-read bug (#174)
  ├── OpenCode SQLite harvester (#175)
  └── Claude JSONL harvester (#176)

Phase 2 — Code Intelligence
  ├── Symbol extraction
  ├── Knowledge graph
  └── Impact analytics

Phase 3 — Memory Enhancement
  └── Auto-memory extraction from harvested sessions

Phase 4 — Self-Learning (Discuss)
  ├── Pattern learning
  ├── Proactive context pre-loading
  └── Self-lesson learn

Phase 5 — Auto-execution (Stretch, Discuss)
  └── Confidence-gated auto task execution
```

---

## Open Questions (To Discuss)

1. **Pillar 4 scope**: Proactive suggestions only, hay auto-execution?
2. **LLM for summarization**: dùng provider nào? Same embedding provider, hay separate?
3. **Output dir**: default path cho harvested sessions?
4. **Incremental harvest**: trigger on schedule (cron), on demand, hay watch DB?
5. **Claude projects/memory**: có harvest `~/.claude/projects/<hash>/memory/` không?
6. **costs.jsonl**: có index token cost data không, hay chỉ dùng cho analytics?
