---
created: 2026-06-29T05:24:41.463Z
title: Avoid full re-index on git checkout / worktree create
area: watcher
files:
  - internal/watcher/ (fsnotify watcher)
  - embed/index pipeline (chunk → embed)
---

## Problem

When switching branches (`git checkout`) or creating a new git worktree, nano-brain's file watcher appears to re-index ALL files in the workspace (full re-index, not incremental), causing significant lag/slowness.

Suspected cause: a checkout/worktree-add rewrites many files' mtimes (and a new worktree materializes the entire tree at once), so fsnotify fires a burst of change events and the pipeline treats every file as changed → re-chunks + re-embeds the whole tree. Embedding is the expensive step, so this stalls the daemon.

Priority: NOT urgent — captured for later improvement. Do not fix now.

## Solution

TBD — candidate directions to investigate:
- Content-hash gate before re-embed: skip files whose content hash is unchanged (mtime-only change from checkout shouldn't trigger re-embed).
- Debounce/coalesce the fsnotify burst that a checkout produces (batch + settle window) instead of processing each event.
- Detect git operations (HEAD change / `.git` activity) and treat the resulting mass-change as a single reconcile pass that diffs against stored hashes, rather than N independent change events.
- For new worktrees: avoid re-indexing files already indexed under the same content (dedupe by content hash across worktrees of the same repo).

Verify with a before/after: time a `git checkout` of a large branch and a `git worktree add`, and confirm embed-queue depth no longer spikes for unchanged files.
