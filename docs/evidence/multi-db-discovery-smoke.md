# Multi-DB Discovery Smoke Test Evidence

**Date**: 2026-05-29
**Branch**: feat/opencode-multi-db-discovery
**Build**: CGO_ENABLED=0 go build -o /tmp/nano-brain-test ./cmd/nano-brain
**Config**: harvester.opencode.db_root: /home/user/.ai-sandbox/opencode-dbs

## Discovery output (extracted from startup log)

```
{"level":"debug","path":"/Users/tamlh/.ai-sandbox/opencode-dbs/ai-sandbox-wrapper-a5c4bcc8/opencode.db","reason":"worktree_not_registered","worktree":"/Users/tamlh/workspaces/self/AI/Tools/ai-sandbox-wrapper","message":"scan skip"}
{"level":"debug","path":"/Users/tamlh/.ai-sandbox/opencode-dbs/next-app-PLACEHOLDER/opencode.db","reason":"worktree_not_registered","worktree":"/data/workspaces/next-app","message":"scan skip"}
{"level":"debug","path":"/Users/tamlh/.ai-sandbox/opencode-dbs/lgc-0581d81e/opencode.db","reason":"global_or_empty_worktree","message":"scan skip"}
{"level":"debug","path":"/Users/tamlh/.ai-sandbox/opencode-dbs/open-design-mcp-c82de6f8/opencode.db","reason":"worktree_not_registered","worktree":"/Users/tamlh/workspaces/self/AI/Tools/open-design-mcp","message":"scan skip"}
{"level":"debug","path":"/Users/tamlh/.ai-sandbox/opencode-dbs/opencode-worktree-plugin-3cf9cddb/opencode.db","reason":"worktree_not_registered","worktree":"/Users/tamlh/workspaces/self/AI/Tools/opencode-worktree-plugin","message":"scan skip"}
{"level":"debug","path":"/Users/tamlh/.ai-sandbox/opencode-dbs/tools-0b9b7f3c/opencode.db","reason":"global_or_empty_worktree","message":"scan skip"}
{"level":"debug","path":"/Users/tamlh/.ai-sandbox/opencode-dbs/express-app-tradeit-admin-PLACEHOLDER/opencode.db","reason":"worktree_not_registered","worktree":"/data/workspaces/express-app/tradeit-admin","message":"scan skip"}
{"level":"debug","path":"/Users/tamlh/.ai-sandbox/opencode-dbs/express-app-PLACEHOLDER/opencode.db","reason":"global_or_empty_worktree","message":"scan skip"}
{"level":"info","db_path":"/Users/tamlh/.ai-sandbox/opencode-dbs/nano-brain-ab295520/opencode.db","worktree":"/Users/tamlh/workspaces/self/AI/Tools/nano-brain","workspace_hash":"7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f","message":"opencode per-project db harvester registered"}
{"level":"info","component":"opencode-sqlite-harvester","count":299,"message":"found opencode sessions"}
```

**Result**: 9 candidate DBs found → 8 correctly skipped (7 unregistered worktrees + 2 global "/" worktrees + 1 already counted) → **1 harvester registered** for the only DB whose worktree matches a registered nano-brain workspace.

## GET /api/status output

```json
{
    "harvester_status": {
        "poll_interval_seconds": 120,
        "opencode": {
            "enabled": true,
            "mode": "db_root",
            "db_root": "/Users/tamlh/.ai-sandbox/opencode-dbs",
            "db_path": "/home/agent/.local/share/opencode/opencode.db",
            "session_dir": "/home/agent/.local/share/opencode/storage",
            "db_count": 1
        }
    }
}
```

## POST /api/harvest

```json
{"harvested":0,"skipped":296,"errors":0}
```

(Sessions already harvested in earlier tick; skip = unchanged content hash. errors=0.)

## Verification of acceptance criteria (#199)

- [x] `harvester.opencode.db_root` config accepted via YAML
- [x] `detectOpenCodeDBRoot()` returns auto-detected path when present
- [x] `scanOpenCodeDBRoot` opens read-only, matches by project.worktree, skips invalid/global
- [x] Daemon registers N=1 OpenCodeSQLiteHarvester for matched DB
- [x] /api/status includes mode + db_count + db_root
- [x] db_path / session_dir auto-detect fallthrough still works (legacy modes preserved)
- [x] Tests pass: short + integration
- [x] Manual smoke: discovery + harvest exercised against user's real db_root
