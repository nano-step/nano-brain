# Test Matrix

| Story | Change type | Unit | Integration | User flow | Review |
| --- | --- | --- | --- | --- | --- |
| US-600 embedding insert race | bug-fix | Queue and handler stale-result tests | Deleted-chunk and locking tests in isolated PostgreSQL | Direct embed endpoint with deterministic local provider | Pending |

## Evidence Rules

- Store command output under `docs/evidence/<change>/`.
- Use `nanobrain_test` and port 3199 for integration and smoke tests.
- Update the row with the final review verdict and PR after merge.
