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
| RLS-43 | GetRateLimit | Happy Path (live) | A key with no stored count is read | No count stored for the key | Returns 1 and nil; the count 1 is now stored for the key and expires within the window | P1 |  |
| RLS-44 | GetRateLimit | Happy Path (live) | A key with a stored count is read | Count 7 stored for the key | Returns 7 and nil | P1 |  |
| RLS-45 | SetRateLimit | Happy Path (live) | A count is stored for a key | No count stored for the key | The count is stored and expires within the window | P1 |  |
| RLS-46 | IncrementRateLimit | Happy Path (live) | A key with no stored count is incremented | No count stored for the key | The stored count is 1 and expires within the window | P1 |  |
| RLS-47 | IncrementRateLimit | Edge Case (live) | A key that has 30 seconds left is incremented | Count 3 stored for the key with 30 seconds left | The stored count is 4 and the key still expires within those 30 seconds, so the window does not restart | P1 |  |
