# PHASE 2: DISCOVER — Persona Interviews

Feature: `feat/config-env-var` (NANO_BRAIN_CONFIG env var)

## Persona 1 — End User (developer using nano-brain locally)

> "I just want to point nano-brain at a different config without changing my shell aliases."

Scenarios I care about:
- E1. Set `NANO_BRAIN_CONFIG=path` and `nano-brain status` reads from that path.
- E2. Override env once with `--config` flag for one-off test.
- E3. Unset env → fall back to `~/.nano-brain/config.yml` (no breakage).
- E4. Empty `NANO_BRAIN_CONFIG=""` → fall back to default (don't crash).
- E5. `config show` displays the resolved path so I know which file is active.
- E6. Works for ALL 12 commands refactored, not just `serve`.

Failure modes I dread:
- F1. Silent fallback to default when my `NANO_BRAIN_CONFIG` has a typo (file doesn't exist).
- F2. Different commands resolve differently (inconsistency).
- F3. `--config` flag silently ignored if env is set.

## Persona 2 — Business Analyst (PM verifying the feature delivers the promise)

> "Issue #219 says 12-factor pattern. Does the PR actually deliver that?"

Requirements traceability:
- B1. Precedence documented: flag > env > default. Verified via `config show`.
- B2. Refactor covers ALL 12 call sites (no missed command).
- B3. README documents the env var with clear example.
- B4. CHANGELOG `[Unreleased]` mentions the feature.
- B5. Backward compat preserved (existing users see no change if env unset).
- B6. Test coverage: at least one test per precedence case (4 total claimed).

## Persona 3 — QA Destroyer (tries to break things)

> "What weird input can I throw at this?"

Adversarial scenarios:
- Q1. `NANO_BRAIN_CONFIG` set to a directory (not a file) → graceful error.
- Q2. Set to a file with bad YAML → graceful error, mention file path.
- Q3. Set to file with no read permission → graceful error.
- Q4. Set to symlink → follows symlink correctly.
- Q5. Set to relative path → resolved against CWD.
- Q6. Set to extremely long path (> PATH_MAX) → graceful error.
- Q7. Set with leading/trailing whitespace → does it trim? (probably not — should fail to find file)
- Q8. Set via `env=""` with `--config=""` → both empty → default.
- Q9. Set both env and flag, flag points to nonexistent → does flag still win (fail) or does it fall through to env?
- Q10. Run two servers simultaneously with different configs (3100 default + 8899 custom) → no cross-contamination.

## Persona 4 — DevOps Tester (deploys to Docker/k8s)

> "I need this for container deployments. Show me it actually works there."

Operational scenarios:
- D1. Docker container with `-e NANO_BRAIN_CONFIG=/etc/nano-brain/config.yml -v /host/cfg.yml:/etc/...`
- D2. k8s ConfigMap mounted + env var → server picks up correct config.
- D3. Container restart preserves env-pointed config (no state leak).
- D4. Two containers, same image, different `NANO_BRAIN_CONFIG` → fully isolated.
- D5. Log output identifies which config was loaded (for debugging in prod).
- D6. `nano-brain doctor` reads from env-pointed config (not default).
- D7. `host.docker.internal` works for cross-container DB.

## Persona 5 — Security Auditor

> "Does this open any new attack surface?"

Security scenarios:
- S1. Path traversal: `NANO_BRAIN_CONFIG=/etc/passwd` — does the YAML parser error gracefully (not exfiltrate)?
- S2. World-readable config: secrets (DB password, API keys) in config — env var doesn't change perms required.
- S3. Env var injection via untrusted source — does nano-brain log the resolved path (audit trail)?
- S4. TOCTOU: env var changes between resolve and load — does it re-read or cache?
- S5. Process listing reveals env vars — risk of exposing config path (low, but documented?)
- S6. Symlink to outside intended dir — does ResolveConfigPath follow without check?

## Cross-Persona Coverage Map (per dimension)

| Dim | E (User) | B (BA) | Q (Destroyer) | D (DevOps) | S (Security) |
|---|---|---|---|---|---|
| API | E1,E2,E5 | B1 | Q10 | D5,D6 | S3 |
| Performance | E1 | — | — | D3 | — |
| Security | F1 | — | Q1,Q2,Q3 | — | S1,S2,S4,S5,S6 |
| Data Integrity | E1,E2,E3,E4 | B5,B6 | Q4,Q5,Q8,Q9 | D2,D4 | S4 |
| Infrastructure | E6 | B2 | Q6,Q10 | D1,D2,D4,D7 | — |
| Edge Cases | E3,E4 | — | Q1-Q9 | D3 | S1,S6 |

26 distinct scenarios. STRUCTURE phase will collapse to Q-A-R-P-T test cases.
