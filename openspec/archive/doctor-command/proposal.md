# Proposal: `doctor` Command

## Why

Users who install via `npx nano-brain` need a way to verify their environment before first use. Without this, setup failures surface as cryptic errors with no guidance.

## Problem

Users install nano-brain via `npx nano-brain` but have no way to verify their environment is correctly set up. Prerequisites (PostgreSQL + pgvector, Ollama + embedding model, config file) must all be working before nano-brain can serve requests. Currently, failures surface as cryptic connection errors or silent embedding failures.

## Solution

Add a `nano-brain doctor` CLI command that checks all prerequisites and reports their status with clear pass/fail output. This gives users a single command to diagnose setup issues.

## Scope

- New CLI subcommand: `doctor`
- New file: `cmd/nano-brain/doctor.go`
- Update: `cmd/nano-brain/main.go` (add case for "doctor")
- No database schema changes
- No API changes

## Risk Classification

Normal lane (2 flags: multi-file change, external provider checks).
