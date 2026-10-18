# Test plan: modules/auth/ratelimit

## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RL-01 | NewService | Happy Path | Service delegates to the `Store` it is built with | fake `Store` | `GetRateLimit` reaches the fake store exactly once and returns its value | P2 |  |
| RL-02 | GetRateLimit | Happy Path | `store.GetRateLimit` succeeds | fake store returns `(5, nil)` | Returns `(5, nil)` | P1 |  |
| RL-03 | GetRateLimit | Error Path | `store.GetRateLimit` fails | | Error propagated unchanged | P2 |  |
| RL-04 | SetRateLimit | Happy Path | `store.SetRateLimit` succeeds | | Returns nil | P1 |  |
| RL-05 | SetRateLimit | Error Path | `store.SetRateLimit` fails | | Error propagated unchanged | P2 |  |
| RL-06 | IncrRateLimit | Happy Path | `store.IncrementRateLimit` succeeds | | Returns nil | P1 |  |
| RL-07 | IncrRateLimit | Error Path | `store.IncrementRateLimit` fails | | Error propagated unchanged | P2 |  |

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RLS-01 | NewStore | Happy Path | Given a redis client | | Returns a non-nil `Store` wrapping that client | P2 |  |
| RLS-40 | GetRateLimit | Error Path | Redis unreachable | `Get` fails with a non-`redis.Nil` error | Returns `(0, err)`; the auto-provision-on-miss branch is never taken | P1 |  |
| RLS-41 | SetRateLimit | Error Path | Redis unreachable | | Returns raw error | P2 |  |
| RLS-42 | IncrementRateLimit | Error Path | Redis unreachable | `Incr` fails | Returns raw error; `ExpireNX` never attempted | P2 |  |
