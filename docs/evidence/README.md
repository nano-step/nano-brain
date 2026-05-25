# Evidence

<!-- generated-by: harness-init v0.1.0 -->

Screenshots, recordings, and test artifacts for high-risk changes.

## Layout

One subdirectory per change or story name:

```
docs/evidence/
├── README.md
├── add-otp-login/
│   ├── playwright-login-success.png
│   ├── playwright-rate-limit.png
│   └── review-verdict.md
└── migrations-mysql-cleanup/
    └── alembic-upgrade-fresh-db.txt
```

## When to add evidence

Required for **high-risk lane** user-facing changes (see `docs/HARNESS.md`
§ User-Flow Testing).

Optional for:
- Normal lane with web UI changes (helpful but not mandatory)
- Infrastructure changes where a recorded run helps future debugging

Skip entirely for:
- Doc-only changes
- Pure refactors with no observable difference

## Naming

- Screenshots: `<tool>-<scenario>.png` (e.g. `playwright-login-success.png`)
- Logs: `<tool>-<scenario>.txt` (e.g. `alembic-upgrade-fresh-db.txt`)
- Reviews: `review-verdict.md`

## Linking from issues / stories

Reference evidence files in story Evidence section and tracking issue comments:

```markdown
Evidence: docs/evidence/add-otp-login/playwright-login-success.png
```

GitHub renders relative paths in issue comments when the comment is created
via the repo (not when posted from a fork).
