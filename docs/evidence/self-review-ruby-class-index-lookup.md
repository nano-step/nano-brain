# Self-Review: fix/ruby-class-index-lookup (Issue #470, PR #471)

Date: 2026-06-21
Reviewer: Sisyphus orchestration + Oracle deep-design pipeline

## Findings

| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| 1 | critical | `ruby_class_index.go` | Lookup returns all entries for short name, no namespace awareness | FIXED |
| 2 | critical | `ruby_resolver.go` | BuildReconcileEdges strips namespace before lookup | FIXED |
| 3 | major | `ruby_class_index.go` | railsConventionPath hardcodes app/models/ | FIXED |
| 4 | minor | `ruby_class_index.go` | No controller directory preference | FIXED |

## Changes

- Full-name lookup for namespaced classes (Api::V1::TokensController)
- preferByNamespace filters entries matching namespace path
- preferController reorders entries so app/controllers/ paths come first
- railsConventionPath handles namespaces (Admin::UsersController → admin/users_controller.rb)
- BuildReconcileEdges tries full namespaced name before short name fallback
- 6 new class index tests + 1 resolver collision test

## Pre-existing Failures (NOT from this PR)

| Check | Failure | Root Cause |
|-------|---------|------------|
| 3.3 Integration tests | `TestResolveWorkspaceParam_*` | Pre-existing in resolve_integration_test.go |
| 3.4 golangci-lint | `js_cflow.go`, `reindex_cfg.go`, etc. | Pre-existing lint issues in files NOT touched by this PR |

## Verification

- `go build ./...` ✅
- `go test -race -short ./...` ✅ (all packages)
- `go test -race -short ./internal/graph/...` ✅ (27s)

## Summary
- Critical: 2 found, 2 fixed
- Major: 1 found, 1 fixed
- Minor: 1 found, 1 fixed
- Pre-existing failures: 2 — documented above, not introduced by this PR
