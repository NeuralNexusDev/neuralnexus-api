# Test plan: modules/auth/session

## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SE-08 | AddSession | Happy Path | `store.AddSessionToDB` and `store.AddSessionToCache` both succeed | | Returns nil; both called exactly once | P1 |  |
| SE-09 | AddSession | Error Path | `store.AddSessionToDB` fails | | Returns that error; `AddSessionToCache` never called | P1 |  |
| SE-10 | AddSession | Edge Case | `AddSessionToDB` succeeds but `AddSessionToCache` fails | | Returns nil anyway (cache failure is fail-open, only logged) | P0 |  |
| SE-11 | GetSession | Happy Path | `store.GetSessionFromCache` succeeds | | Returns the cached session; `GetSessionFromDB` never called | P1 |  |
| SE-12 | GetSession | Edge Case | Cache miss (`GetSessionFromCache` errors), DB hit | `store.GetSessionFromDB` succeeds | Returns the DB session; `AddSessionToCache` called once to repopulate | P1 |  |
| SE-13 | GetSession | Error Path | Cache miss and DB miss | `GetSessionFromDB` fails | Returns nil, error (`ErrNotFound`) | P2 |  |
| SE-14 | GetSession | Edge Case | Cache miss, DB hit, repopulating the cache fails | `AddSessionToCache` errors | Still returns the DB session with nil error (repopulation failure is fail-open) | P1 |  |
| SE-15 | UpdateSession | Happy Path | `UpdateSessionInDB` and `AddSessionToCache` both succeed | | Returns nil | P1 |  |
| SE-16 | UpdateSession | Error Path | `UpdateSessionInDB` fails | | Returns that error; cache never touched | P1 |  |
| SE-17 | UpdateSession | Edge Case | `UpdateSessionInDB` succeeds, `AddSessionToCache` fails | | Returns nil anyway (fail-open) | P1 |  |
| SE-18 | DeleteSession | Happy Path | `DeleteSessionInDB` and `DeleteSessionFromCache` both succeed | | Returns nil | P1 |  |
| SE-19 | DeleteSession | Error Path | `DeleteSessionInDB` fails | | Returns that error; cache eviction never attempted | P1 |  |
| SE-20 | DeleteSession | Error Path | `DeleteSessionInDB` succeeds, `DeleteSessionFromCache` fails | | Returns an error wrapping `testutil.ErrCacheDown` (fail-closed, unlike Add/Update) | P0 |  |
| SE-23 | ReadJWT | Happy Path | Valid JWT for an existing session whose `UserID` matches the token subject | `store.GetSession` finds it | Returns the session, nil error; `LastUsedAt` bumped via `UpdateSession` | P1 |  |
| SE-24 | ReadJWT | Error Path | Malformed/garbage token string | | Returns nil, parse error | P2 |  |
| SE-25 | ReadJWT | Error Path | Token signed with a different secret / wrong algorithm | | Returns nil, error (signature/alg validation failure) | P1 |  |
| SE-26 | ReadJWT | Error Path | Token's audience is empty | | Returns nil, `ErrMissingAudience` | P1 |  |
| SE-27 | ReadJWT | Error Path | Token's audience contains an empty-string entry | | Returns nil, `ErrEmptyAudienceEntry` | P2 |  |
| SE-28 | ReadJWT | Error Path | Token's audience doesn't match `NN_SITE_URL`/`NN_API_URL` | | Returns nil, an error wrapping `ErrInvalidAudience` | P1 |  |
| SE-29 | ReadJWT | Error Path | Session lookup fails (e.g. session was deleted/revoked) even though the JWT is validly signed and unexpired | `store.GetSession` returns an error | Returns nil, an error wrapping `ErrNotFound` | P0 |  |
| SE-30 | ReadJWT | Error Path | Session found but its `UserID` doesn't match the token's `Subject` claim | | Returns nil, `ErrSessionSubjectMismatch` | P0 |  |
| SE-31 | ReadJWT | Error Path | `UpdateSession` (LastUsedAt bump) fails after a valid lookup | `UpdateSessionInDB` returns `testutil.ErrDBDown` | Returns nil session and an error matching `testutil.ErrDBDown` | P2 |  |
| SE-38 | AddSession | Edge Case | the call context is cancelled before the writes | fake store records `ctx.Err()` and a marker value at each call | The call succeeds, and `AddSessionToDB` and `AddSessionToCache` each see a context that is not done and still carries the marker, so the session is stored in both places | P1 |  |
| SE-39 | UpdateSession | Edge Case | the call context is cancelled before the writes | fake store records `ctx.Err()` and a marker value at each call | The call succeeds, and `UpdateSessionInDB` and `AddSessionToCache` each see a context that is not done and still carries the marker, so the update lands in both places | P1 |  |

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SE-02 | HasPermission | Happy Path | `Permissions` holds the node bare, with a value, or with a value containing a colon | | Returns true | P1 |  |
| SE-03 | HasPermission | Error Path | no permissions, or only look-alike entries | entries such as `perm.nodes`, `perm.node.sub`, `perm.nod`, `perm.node2:value` and `x:perm.node` | Returns false | P1 |  |
| SE-04 | IsValid | Happy Path | `ExpiresAt` in the future | | Returns true | P1 |  |
| SE-05 | IsValid | Edge Case | `ExpiresAt == 0` (never expires) | | Returns true | P1 |  |
| SE-06 | IsValid | Edge Case | `ExpiresAt` in the past | | Returns false | P0 |  |
| SE-35 | HasPermissionValue | Happy Path | the session holds `node:a` and `node:b:c` | | true for `a` and `b:c` | P1 |  |
| SE-36 | HasPermissionValue | Error Path | the value is absent, empty, extends a held value, or is the node itself | | false | P1 |  |

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SES-19 | AddSessionToDB | Error Path | DB unreachable | `Exec` fails | Returns the raw error; the deferred `ClearExpiredSessions` (also failing against the same unreachable db) does not panic | P1 |  |
| SES-20 | GetSessionFromDB | Error Path | DB unreachable | | Returns nil, raw error | P1 |  |
| SES-21 | DeleteSessionInDB | Error Path | DB unreachable | | Returns raw error | P2 |  |
| SES-22 | UpdateSessionInDB | Error Path | DB unreachable | | Returns raw error | P2 |  |
| SES-23 | ClearExpiredSessions | Edge Case | DB unreachable | | Logs and swallows the error; returns normally without panicking (void method) | P1 |  |
| SES-24 | AddSessionToCache | Edge Case | `session.ExpiresAt` is in the past (nonzero) | redis client need not be reachable | Returns nil without ever calling `Set` (negative-TTL short-circuit) | P0 |  |
| SES-25 | AddSessionToCache | Error Path | `ExpiresAt == 0` (never expires) or in the future, redis unreachable | | `Set` is attempted and fails; raw redis error propagated | P1 |  |
| SES-26 | GetSessionFromCache | Error Path | Redis unreachable | | Returns nil, raw redis error (not treated as a cache miss) | P1 |  |
| SES-27 | DeleteSessionFromCache | Error Path | Redis unreachable | | Returns raw redis error | P2 |  |
| SES-76 | GetSessionFromDB | Error Path | No session with that ID | Real Postgres, unknown session ID | Returns nil, `ErrNotFound` | P2 | |
| SES-80 | GetSessionFromCache | Error Path | No cached session for that ID | Real Redis, unknown session ID | Returns nil, `ErrNotFound` | P2 | |
