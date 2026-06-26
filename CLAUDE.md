# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

**All project context lives in [AGENTS.md](AGENTS.md).** Read that file for architecture, conventions, testing, and agent-oriented design principles.

## Quick Reference

```bash
CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain   # Build
go test -race -short ./...                                 # Unit tests
go test -race -tags=integration ./...                      # Integration tests
sqlc generate                                              # SQL codegen
```
