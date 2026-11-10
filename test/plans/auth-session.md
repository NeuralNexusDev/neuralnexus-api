# Test plan: modules/auth/session

## service

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SE-08 | AddSession | Happy Path | Storing the session and caching the session both succeed | | Returns no error; the session is stored once and cached once | P1 |  |
| SE-09 | AddSession | Error Path | Storing the session fails | | Returns that error; the session is never cached | P1 |  |
| SE-10 | AddSession | Edge Case | Storing the session succeeds but caching it fails | | Returns no error anyway, because a cache failure does not fail the call | P0 |  |
| SE-11 | GetSession | Happy Path | The cache holds the session | | Returns the cached session; the database is not read | P1 |  |
| SE-12 | GetSession | Edge Case | The cache holds no entry and the database holds the session | Loading the session from the database succeeds | Returns the stored session; the session is cached once, so the next read finds it | P1 |  |
| SE-13 | GetSession | Error Path | The cache holds no entry and the database holds no session | Loading the session from the database fails | Returns no session and `ErrNotFound` | P2 |  |
| SE-14 | GetSession | Edge Case | The cache holds no entry, the database holds the session, and caching the session fails | Caching the session fails | Still returns the stored session and no error | P1 |  |
| SE-15 | UpdateSession | Happy Path | Updating the stored session and caching the session both succeed | | Returns no error | P1 |  |
| SE-16 | UpdateSession | Error Path | Updating the stored session fails | | Returns that error; the session is never cached | P1 |  |
| SE-17 | UpdateSession | Edge Case | Updating the stored session succeeds but caching the session fails | | Returns no error anyway | P1 |  |
| SE-18 | DeleteSession | Happy Path | Deleting the stored session and removing the session from the cache both succeed | | Returns no error | P1 |  |
| SE-19 | DeleteSession | Error Path | Deleting the stored session fails | | Returns that error; the session is never removed from the cache | P1 |  |
| SE-20 | DeleteSession | Error Path | Deleting the stored session succeeds but removing the session from the cache fails | | Returns an error wrapping the cache failure, so a session that may still be cached is reported, unlike adding or updating | P0 |  |
| SE-23 | ReadJWT | Happy Path | Valid token for an existing session whose user matches the token subject | Loading the session finds it | Returns the session and no error; the stored session is updated once | P1 |  |
| SE-24 | ReadJWT | Error Path | The token is not a well-formed token | | Returns no session and an error | P2 |  |
| SE-25 | ReadJWT | Error Path | The token is signed with a different secret | | Returns no session and an error | P1 |  |
| SE-26 | ReadJWT | Error Path | The token's audience is empty | | Returns no session and `ErrMissingAudience` | P1 |  |
| SE-27 | ReadJWT | Error Path | Token's audience contains an empty-string entry | | Returns nil, `ErrEmptyAudienceEntry` | P2 |  |
| SE-28 | ReadJWT | Error Path | The token's audience is not one of the configured site and API addresses | | Returns no session and an error wrapping `ErrInvalidAudience` | P1 |  |
| SE-29 | ReadJWT | Error Path | The token is validly signed and unexpired, but its session no longer exists | Loading the session fails with `ErrNotFound` | Returns no session and an error wrapping `ErrNotFound` | P0 |  |
| SE-30 | ReadJWT | Error Path | The session found belongs to a different user than the token subject | | Returns no session and `ErrSessionSubjectMismatch` | P0 |  |
| SE-31 | ReadJWT | Error Path | Updating the stored session fails after a valid lookup | Updating the stored session fails with a database error | Returns no session and an error wrapping the database error | P2 |  |
| SE-38 | AddSession | Edge Case | the call context is cancelled before the writes | The session store records whether the context it receives is done and which marker value it carries, for each call | The call succeeds, and storing the session and caching the session each see a context that is not done and still carries the marker, so the session is stored in both places | P1 |  |
| SE-39 | UpdateSession | Edge Case | the call context is cancelled before the writes | The session store records whether the context it receives is done and which marker value it carries, for each call | The call succeeds, and updating the stored session and caching the session each see a context that is not done and still carries the marker, so the update lands in both places | P1 |  |

## Session

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SE-02 | HasPermission | Happy Path | The held permissions include the node bare, with a value, or with a value that itself contains a colon | | Returns true | P1 |  |
| SE-03 | HasPermission | Error Path | No permissions are held, or only look-alikes of the node | The held entries are another node, the node with an extra letter, the node with a subnode, the node cut short, the node with a digit before the colon and a value, and the node after a prefix | Returns false | P1 |  |
| SE-04 | IsValid | Happy Path | The expiry time is in the future | | Returns true | P1 |  |
| SE-05 | IsValid | Edge Case | The expiry time is 0, so the session never expires | | Returns true | P1 |  |
| SE-06 | IsValid | Edge Case | The expiry time is in the past | | Returns false | P0 |  |
| SE-35 | HasPermissionValue | Happy Path | The session holds two values under one node, and the second value contains a colon | | True for each held value | P1 |  |
| SE-36 | HasPermissionValue | Error Path | The value is not held, is empty, extends a held value, or is the node itself | | false | P1 |  |

## store

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SES-19 | AddSessionToDB | Error Path | The database is unreachable | The insert fails with a connection error | Returns the connection error unchanged and does not panic | P1 |  |
| SES-24 | AddSessionToCache | Edge Case | The expiry time is in the past and not 0 | The cache client is not set | Returns no error and does not contact the cache | P0 |  |
| SES-25 | AddSessionToCache | Error Path | The expiry time is 0 (never expires), and the cache is unreachable | | The cache write is attempted and fails; the connection error is returned unchanged | P1 |  |
| SES-76 | GetSessionFromDB | Error Path | No session with that ID | Real Postgres, unknown session ID | Returns nil, `ErrNotFound` | P2 | |
| SES-80 | GetSessionFromCache | Error Path | No cached session for that ID | Real Redis, unknown session ID | Returns nil, `ErrNotFound` | P2 | |
