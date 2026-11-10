# Test plan: modules/auth/ratelimit

## service

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RL-02 | GetRateLimit | Happy Path | Reading the counter succeeds | The counter store holds 5 for the key | Returns 5 and no error | P1 |  |
| RL-03 | GetRateLimit | Error Path | Reading the counter fails | | The error is returned unchanged | P2 |  |
| RL-04 | SetRateLimit | Happy Path | Setting the counter succeeds | | Returns no error | P1 |  |
| RL-05 | SetRateLimit | Error Path | Setting the counter fails | | The error is returned unchanged | P2 |  |
| RL-06 | IncrRateLimit | Happy Path | Incrementing the counter succeeds | | Returns no error | P1 |  |
| RL-07 | IncrRateLimit | Error Path | Incrementing the counter fails | | The error is returned unchanged | P2 |  |
| RL-09 | IncrRateLimit | Edge Case | the call context is cancelled before the increment | The counter store records whether the context it receives is done and which marker value it carries | The call succeeds, and the increment runs on a context that is not done and still carries the marker, so the increment is not cancelled with the client | P1 |  |

## store

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RLS-40 | GetRateLimit | Error Path | The cache is unreachable | Reading the counter fails with a connection error, not with "the cache holds no entry" | Returns 0 and the connection error | P1 |  |
