## AC-1: migration_version is real

**Given** the server has started and migrations have run  
**When** `GET /api/status` is called  
**Then** `migration_version` equals the actual goose version from the DB (e.g. `9` after all 9 migrations)  
**And** it is NOT hardcoded to `1`

## AC-2: migration_version survives restart

**Given** the server restarts  
**When** `/api/status` is called  
**Then** `migration_version` still reflects the real DB version (not reset to 1)

## AC-3: init --force harvest result is visible

**Given** the user runs `nano-brain init --root <path> --force`  
**When** harvest completes (success or partial error)  
**Then** the CLI prints the harvest outcome (harvested N, skipped M, errors K) to stdout  
**And** the user does not need to check logs to know if harvest succeeded

## AC-4: init non-force path unchanged

**Given** the user runs `nano-brain init --root <path>` (no --force)  
**When** it completes  
**Then** behavior is identical to before this change
