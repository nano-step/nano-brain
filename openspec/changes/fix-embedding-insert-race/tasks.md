## 1. Race-Safe Storage Contract

- [x] 1.1 Add a failing isolated-PostgreSQL regression test for inserting after the source chunk is deleted.
- [x] 1.2 Make the embedding upsert conditional on a locked live source chunk and regenerate sqlc.

## 2. Production Writer Handling

- [x] 2.1 Add a failing queue test for a no-row embedding insert and handle it as a stale job.
- [x] 2.2 Add a failing direct-endpoint batch test for a no-row embedding insert and continue the batch.

## 3. Verification

- [x] 3.1 Run focused unit and integration tests, including the controlled stale-chunk scenario.
- [ ] 3.2 Run the normal-lane validation, smoke test, and independent review; record evidence.
