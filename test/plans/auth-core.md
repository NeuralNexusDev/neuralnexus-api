# Test plan: modules/auth (core package)

## account.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| AC-01 | NewAccountService | Happy Path | Service is wired to the store's own sub-stores | fake `Store` returns distinct fake `AccountStore`/`AccountSettingsStore` | Returned `AccountService`, when exercised, delegates to the exact `AccountStore`/`AccountSettingsStore` instances `store.Account()`/`store.AccountSettings()` returned | P2 |  |
| AC-02 | GetAccountByID | Happy Path | `as.GetAccountByID` succeeds | fake store returns a known `*Account` | Returns that `*Account`, nil error | P1 |  |
| AC-03 | GetAccountByID | Error Path | `as.GetAccountByID` fails | fake store returns `ErrNotFound` | Returns nil, `ErrNotFound` propagated unchanged | P2 |  |
| AC-04 | GetAccountByUsername | Happy Path | `as.GetAccountByUsername` succeeds | | Returns the account, nil | P1 |  |
| AC-05 | GetAccountByUsername | Error Path | `as.GetAccountByUsername` fails | | Error propagated unchanged | P2 |  |
| AC-06 | GetAccountByEmail | Happy Path | `as.GetAccountByEmail` succeeds | | Returns the account, nil | P1 |  |
| AC-07 | GetAccountByEmail | Error Path | `as.GetAccountByEmail` fails | | Error propagated unchanged | P2 |  |
| AC-08 | AddAccount | Happy Path | `as.AddAccountToDB` succeeds | | Returns nil | P1 |  |
| AC-09 | AddAccount | Error Path | `as.AddAccountToDB` returns `ErrUsernameAlreadyExists` | | Error propagated unchanged | P2 |  |
| AC-10 | UpdateAccount | Happy Path | `as.UpdateAccountInDB` succeeds | | Returns nil | P1 |  |
| AC-11 | UpdateAccount | Error Path | `as.UpdateAccountInDB` fails | | Error propagated unchanged | P2 |  |
| AC-12 | DeleteAccount | Happy Path | `as.DeleteAccountFromDB` succeeds | | Returns nil | P1 |  |
| AC-13 | DeleteAccount | Error Path | `as.DeleteAccountFromDB` fails | | Error propagated unchanged | P2 |  |
| AC-14 | IsPasswordAuthEnabled | Happy Path | `ass.GetAccountSettings` returns `{PasswordAuthEnabled: true}` | | Returns `(true, nil)` | P1 |  |
| AC-15 | IsPasswordAuthEnabled | Edge Case | `ass.GetAccountSettings` returns `{PasswordAuthEnabled: false}` | | Returns `(false, nil)` | P2 |  |
| AC-16 | IsPasswordAuthEnabled | Error Path | `ass.GetAccountSettings` fails | | Returns `(false, err)` | P2 |  |

## ratelimit.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RL-01 | NewRateLimitService | Happy Path | Service wired to `store.RateLimit()` | fake `Store` | Returned service delegates to the exact `RateLimitStore` the fake `Store` returned | P2 |  |
| RL-02 | GetRateLimit | Happy Path | `store.GetRateLimit` succeeds | fake store returns `(5, nil)` | Returns `(5, nil)` | P1 |  |
| RL-03 | GetRateLimit | Error Path | `store.GetRateLimit` fails | | Error propagated unchanged | P2 |  |
| RL-04 | SetRateLimit | Happy Path | `store.SetRateLimit` succeeds | | Returns nil | P1 |  |
| RL-05 | SetRateLimit | Error Path | `store.SetRateLimit` fails | | Error propagated unchanged | P2 |  |
| RL-06 | IncrRateLimit | Happy Path | `store.IncrementRateLimit` succeeds | | Returns nil | P1 |  |
| RL-07 | IncrRateLimit | Error Path | `store.IncrementRateLimit` fails | | Error propagated unchanged | P2 |  |

## session.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SE-01 | ToProto | Happy Path | Populated `*Session` | | Returns `*sessionpb.Session` with Id/UserId/Permissions/IssuedAt/LastUsedAt/ExpiresAt copied 1:1 | P2 |  |
| SE-02 | HasPermission | Happy Path | `Permissions` contains `"name\|value"` for the checked scope | | Returns true | P1 |
| SE-03 | HasPermission | Edge Case | `Permissions` empty, or contains only non-matching entries | | Returns false | P2 |  |
| SE-04 | IsValid | Happy Path | `ExpiresAt` in the future | | Returns true | P1 |  |
| SE-05 | IsValid | Edge Case | `ExpiresAt == 0` (never expires) | | Returns true | P1 |  |
| SE-06 | IsValid | Edge Case | `ExpiresAt` in the past | | Returns false | P0 |  |
| SE-07 | NewSessionService | Happy Path | Service wired to `store.Session()` | fake `Store` | Returned service delegates to the exact `SessionStore` the fake `Store` returned | P2 |  |
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
| SE-20 | DeleteSession | Error Path | `DeleteSessionInDB` succeeds, `DeleteSessionFromCache` fails | | Returns an error wrapping `testerrors.ErrCacheDown` (fail-closed, unlike Add/Update) | P0 |  |
| SE-21 | CreateJWT | Happy Path | Session with nonzero `ExpiresAt` | | Returns a signed JWT string, err nil; decodes back to matching claims | P1 |  |
| SE-22 | CreateJWT | Edge Case | `Session.ExpiresAt == 0` | | Returned JWT omits the `exp` claim entirely | P0 |  |
| SE-23 | ReadJWT | Happy Path | Valid JWT for an existing session whose `UserID` matches the token subject | `store.GetSession` finds it | Returns the session, nil error; `LastUsedAt` bumped via `UpdateSession` | P1 |  |
| SE-24 | ReadJWT | Error Path | Malformed/garbage token string | | Returns nil, parse error | P2 |  |
| SE-25 | ReadJWT | Error Path | Token signed with a different secret / wrong algorithm | | Returns nil, error (signature/alg validation failure) | P1 |  |
| SE-26 | ReadJWT | Error Path | Token's audience is empty | | Returns nil, `ErrMissingAudience` | P1 |  |
| SE-27 | ReadJWT | Error Path | Token's audience contains an empty-string entry | | Returns nil, `ErrEmptyAudienceEntry` | P2 |  |
| SE-28 | ReadJWT | Error Path | Token's audience doesn't match `NN_SITE_URL`/`NN_API_URL` | | Returns nil, an error wrapping `ErrInvalidAudience` | P1 |  |
| SE-29 | ReadJWT | Error Path | Session lookup fails (e.g. session was deleted/revoked) even though the JWT is validly signed and unexpired | `store.GetSession` returns an error | Returns nil, an error wrapping `ErrNotFound` | P0 |  |
| SE-30 | ReadJWT | Error Path | Session found but its `UserID` doesn't match the token's `Subject` claim | | Returns nil, `ErrSessionSubjectMismatch` | P0 |  |
| SE-31 | ReadJWT | Error Path | `UpdateSession` (LastUsedAt bump) fails after a valid lookup | `UpdateSessionInDB` returns `testerrors.ErrDBDown` | Returns nil session and an error matching `testerrors.ErrDBDown` | P2 |  |
| SE-32 | init | Happy Path | Package loads under the required `JWT_SECRET`/`NN_SITE_URL`/`NN_API_URL` env (the precondition every other test in this file already runs under) | | `JWT_SECRET` and `validAudiences` are populated from env without `log.Fatal` firing; `validAudiences == []string{NN_SITE_URL, NN_API_URL}` | P2 | Asserts init's already-established postcondition rather than re-invoking it |
| SE-33 | init | Error Path | `JWT_SECRET` unset | Test binary re-exec'd as a subprocess with `JWT_SECRET=""`, other required env vars inherited unchanged | Subprocess exits non-zero via `log.Fatal(msgJWTSecretUnset)` | P1 | Re-exec/TestCrasher pattern (see `os/exec` docs) — init() runs unconditionally at process start, before any `-test.run` filtering |
| SE-34 | init | Error Path | `NN_SITE_URL` or `NN_API_URL` unset (`JWT_SECRET` set) | Test binary re-exec'd as a subprocess with each cleared in turn | Subprocess exits non-zero via `log.Fatal(msgSiteAPIURLUnset)` | P1 | Same re-exec pattern; exercises both var slots of the OR condition |

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ST-01 | NewStore | Happy Path | Given a db pool and redis client | | Returns a non-nil `Store` wrapping both | P2 |  |
| ST-02 | Account | Accessor | | | Returned `AccountStore` type-asserts back to the same `*store` pointer | P3 |  |
| ST-03 | AccountSettings | Accessor | | | Same pointer-identity check | P3 |  |
| ST-04 | Session | Accessor | | | Same pointer-identity check | P3 |  |
| ST-05 | LinkAccount | Accessor | | | Same pointer-identity check | P3 |  |
| ST-06 | RateLimit | Accessor | | | Same pointer-identity check | P3 |  |
| ST-07 | OAuthToken | Accessor | | | Same pointer-identity check | P3 |  |
| ST-08 | translateAccountConstraintErr | Edge Case | `err` is not a `*pgconn.PgError` | a plain error that is not a `*pgconn.PgError` (`testerrors.ErrBoom`) | Returned unchanged | P1 |  |
| ST-09 | translateAccountConstraintErr | Edge Case | `*pgconn.PgError` with `Code != "23505"` | | Returned unchanged | P2 |  |
| ST-10 | translateAccountConstraintErr | Edge Case | `Code == "23505"`, unrecognized `ConstraintName` | | Returned unchanged (falls through the switch) | P1 |  |
| ST-11 | translateAccountConstraintErr | Happy Path | `Code == "23505"`, `ConstraintName == "accounts_email_key"` | | Returns `ErrEmailAlreadyExists` | P0 |  |
| ST-12 | translateAccountConstraintErr | Happy Path | `Code == "23505"`, `ConstraintName == "accounts_username_key"` | | Returns `ErrUsernameAlreadyExists` | P0 |  |
| ST-13 | AddAccountToDB | Error Path | DB unreachable | `Exec` fails with a plain connection error (not `*pgconn.PgError`) | Returns that raw error unchanged (not translated to a sentinel) | P1 |  |
| ST-14 | GetAccountByID | Error Path | DB unreachable | `Query` fails before any row scan | Returns nil, the raw connection error (not `ErrNotFound`) | P1 |  |
| ST-15 | GetAccountByUsername | Error Path | DB unreachable | | Returns nil, raw connection error | P2 |  |
| ST-16 | GetAccountByEmail | Error Path | DB unreachable | | Returns nil, raw connection error | P2 |  |
| ST-17 | UpdateAccountInDB | Error Path | DB unreachable | | Returns the raw connection error unchanged | P1 |  |
| ST-18 | DeleteAccountFromDB | Error Path | DB unreachable | | Returns the raw connection error | P2 |  |
| ST-19 | AddSessionToDB | Error Path | DB unreachable | `Exec` fails | Returns the raw error; the deferred `ClearExpiredSessions` (also failing against the same unreachable db) does not panic | P1 |  |
| ST-20 | GetSessionFromDB | Error Path | DB unreachable | | Returns nil, raw error | P1 |  |
| ST-21 | DeleteSessionInDB | Error Path | DB unreachable | | Returns raw error | P2 |  |
| ST-22 | UpdateSessionInDB | Error Path | DB unreachable | | Returns raw error | P2 |  |
| ST-23 | ClearExpiredSessions | Edge Case | DB unreachable | | Logs and swallows the error; returns normally without panicking (void method) | P1 |  |
| ST-24 | AddSessionToCache | Edge Case | `session.ExpiresAt` is in the past (nonzero) | redis client need not be reachable | Returns nil without ever calling `Set` (negative-TTL short-circuit) | P0 |  |
| ST-25 | AddSessionToCache | Error Path | `ExpiresAt == 0` (never expires) or in the future, redis unreachable | | `Set` is attempted and fails; raw redis error propagated | P1 |  |
| ST-26 | GetSessionFromCache | Error Path | Redis unreachable | | Returns nil, raw redis error (not treated as a cache miss) | P1 |  |
| ST-27 | DeleteSessionFromCache | Error Path | Redis unreachable | | Returns raw redis error | P2 |  |
| ST-28 | AddLinkedAccountToDB | Error Path | DB unreachable | | Returns the raw connection error (not `ErrAlreadyLinked`, since it isn't a `*pgconn.PgError`) | P1 |  |
| ST-29 | UpdateLinkedAccount | Error Path | DB unreachable | | Returns raw error | P2 |  |
| ST-30 | GetLinkedAccountByPlatformID | Error Path | DB unreachable | | Returns nil, raw error | P1 |  |
| ST-31 | GetLinkedAccountByPlatformName | Error Path | DB unreachable | | Returns nil, raw error | P2 |  |
| ST-32 | GetLinkedAccountByUserID | Error Path | DB unreachable | | Returns nil, raw error | P2 |  |
| ST-33 | GetLinkedAccountsByUserID | Error Path | DB unreachable | | Returns nil, raw error | P2 |  |
| ST-34 | DeleteLinkedAccount | Error Path | DB unreachable | `s.db.Begin` fails immediately | Returns the raw error before the advisory-lock/guard logic runs | P1 |  |
| ST-35 | SetLinkedAccountLoginEnabled | Error Path | DB unreachable, `enabled=true` | Plain `Exec` (no tx) fails | Returns raw error | P1 |  |
| ST-36 | SetLinkedAccountLoginEnabled | Error Path | DB unreachable, `enabled=false` | `s.db.Begin` fails | Returns raw error before the advisory-lock/guard logic runs | P1 |  |
| ST-37 | GetAccountSettings | Error Path | DB unreachable | | Returns nil, raw error (the `DefaultAccountSettings` fallback is `ErrNoRows`-specific and not reached) | P1 |  |
| ST-38 | SetPasswordAuthEnabled | Error Path | DB unreachable, `enabled=true` | `s.db.Begin` fails | Returns raw error | P1 |  |
| ST-39 | SetPasswordAuthEnabled | Error Path | DB unreachable, `enabled=false` | | Returns raw error | P2 |  |
| ST-40 | GetRateLimit | Error Path | Redis unreachable | `Get` fails with a non-`redis.Nil` error | Returns `(0, err)`; the auto-provision-on-miss branch is never taken | P1 |  |
| ST-41 | SetRateLimit | Error Path | Redis unreachable | | Returns raw error | P2 |  |
| ST-42 | IncrementRateLimit | Error Path | Redis unreachable | `Incr` fails | Returns raw error; `ExpireNX` never attempted | P2 |  |
| ST-43 | AddOAuthTokenToDB | Error Path | DB unreachable | | Returns raw error | P2 |  |
| ST-44 | GetOAuthTokenByUserID | Error Path | DB unreachable | | Returns nil, raw error | P2 |  |
| ST-45 | UpdateOAuthToken | Error Path | DB unreachable | | Returns raw error | P2 |  |
| ST-46 | DeleteOAuthToken | Error Path | DB unreachable | | Returns raw error | P2 |  |
| ST-47 | GetAccountByID | Happy Path | No account row exists for userID | Real Postgres, live query | Returns nil, `ErrNotFound` | P1 |  |
| ST-48 | GetAccountByUsername | Happy Path | No account row exists for username | | Returns nil, `ErrNotFound` | P2 |  |
| ST-49 | GetAccountByEmail | Happy Path | No account row exists for email | | Returns nil, `ErrNotFound` | P2 |  |
| ST-50 | AddAccountToDB | Edge Case | Two accounts inserted with `Email == nil` | Real Postgres NULL semantics on a `UNIQUE` column | Both inserts succeed, no false collision | P2 |  |
| ST-51 | AddAccountToDB | Edge Case | Two accounts inserted via `NewIDOnlyAccount` (`Username == ""`) | `NULLIF` converts `""` to `NULL` before insert | Both inserts succeed (regression test for a previously-fixed empty-username collision bug) | P0 |  |
| ST-52 | AddAccountToDB | Error Path | Second account reuses an existing non-empty username | Real Postgres unique-violation on `accounts_username_key` | Returns `ErrUsernameAlreadyExists` | P1 |  |
| ST-53 | AddAccountToDB | Error Path | Second account reuses an existing non-empty email | Real Postgres unique-violation on `accounts_email_key` | Returns `ErrEmailAlreadyExists` | P1 |  |
| ST-54 | DeleteLinkedAccount | Edge Case | Passwordless account, exactly one verified+enabled link, no other fallback | Guard denies removing the only usable login method | Returns `ErrWouldLockAccount`; the link still exists afterward | P0 |  |
| ST-55 | DeleteLinkedAccount | Happy Path | Account has a password (`hashed_secret` set, password auth enabled) | Guard allows since the password remains usable | Returns nil; link removed (`ErrNotFound`) | P1 |  |
| ST-56 | DeleteLinkedAccount | Happy Path | Passwordless account with a second verified+enabled link on another platform | Guard allows | Returns nil; the other link is left untouched | P1 |  |
| ST-57 | DeleteLinkedAccount | Error Path | No `linked_accounts` row exists for `(userID, platform)` | Existence check after the 0-row guard result | Returns `ErrNotFound` (not `ErrWouldLockAccount`) | P1 |  |
| ST-58 | DeleteLinkedAccount | Edge Case | A second link exists on another platform but has `login_enabled = false` | Guard only counts verified+enabled others | Returns `ErrWouldLockAccount` | P2 |  |
| ST-59 | DeleteLinkedAccount | Edge Case | Account has `hashed_secret` but `account_settings.password_auth` is false | Guard treats disabled password auth as unusable | Returns `ErrWouldLockAccount` | P1 |  |
| ST-60 | DeleteLinkedAccount | Concurrency Invariant | Two concurrent deletes target different platforms on the same account, each the other's sole fallback | `pg_advisory_xact_lock(27745, hashtext(userID))` serializes them; run many trials against real Postgres | Exactly one of the two succeeds every trial; exactly one link always remains (`ErrWouldLockAccount`) | P0 |  |
| ST-61 | SetLinkedAccountLoginEnabled | Happy Path | `enabled=true` on a `Verified` row | | Returns nil; `LoginEnabled` becomes true | P1 |  |
| ST-62 | SetLinkedAccountLoginEnabled | Error Path | `enabled=true` on an unverified row | 0 rows match the `verified = true` clause | Returns `ErrLinkedAccountUnverified` | P0 |  |
| ST-63 | SetLinkedAccountLoginEnabled | Edge Case | `enabled=false`, this is the account's last usable login method | Same guard as `DeleteLinkedAccount` | Returns `ErrWouldLockAccount`; `LoginEnabled` remains true | P0 |  |
| ST-64 | SetLinkedAccountLoginEnabled | Happy Path | `enabled=false`, another verified+enabled link exists | | Returns nil; `LoginEnabled` becomes false | P1 |  |
| ST-65 | SetLinkedAccountLoginEnabled | Edge Case | `enabled=false`, `account_settings.password_auth` is false and this is the only enabled link | | Returns `ErrWouldLockAccount` | P1 |  |
| ST-66 | SetLinkedAccountLoginEnabled | Concurrency Invariant | Two concurrent disables target different platforms, each the other's sole fallback | Same lock as ST-60, many trials | Exactly one succeeds every trial; exactly one link remains login-enabled (`ErrWouldLockAccount`) | P0 |  |
| ST-67 | GetAccountSettings | Edge Case | The account exists but has no `account_settings` row yet | Real Postgres, `pgx.ErrNoRows` | Returns `DefaultAccountSettings(userID)` (`PasswordAuthEnabled: true`), nil error | P1 |  |
| ST-68 | SetPasswordAuthEnabled | Error Path | `enabled=true`, account has no `hashed_secret` | 0 rows match the `INSERT ... WHERE EXISTS` clause | Returns `ErrNoPasswordSet` | P1 |  |
| ST-69 | SetPasswordAuthEnabled | Happy Path | `enabled=true`, account has `hashed_secret` | | Returns nil; settings read back `PasswordAuthEnabled=true` | P1 |  |
| ST-70 | SetPasswordAuthEnabled | Edge Case | `enabled=false`, account has a password but no linked platform | Guard denies (no fallback) | Returns `ErrWouldLockAccount`; settings unchanged | P0 |  |
| ST-71 | SetPasswordAuthEnabled | Happy Path | `enabled=false`, account has a password and one verified+enabled link | Guard allows | Returns nil; settings read back `PasswordAuthEnabled=false` | P1 |  |
| ST-72 | SetPasswordAuthEnabled | Concurrency Invariant | `SetPasswordAuthEnabled(false)` races `DeleteLinkedAccount` (sole link) on the same account | Same lock, many trials | Exactly one succeeds every trial; account never ends up with neither a password nor a link (`ErrWouldLockAccount`) | P0 |  |
| ST-73 | SetPasswordAuthEnabled | Concurrency Invariant | `SetPasswordAuthEnabled(false)` races `SetLinkedAccountLoginEnabled` (sole link, false) on the same account | Same lock, many trials | Exactly one succeeds every trial; account never ends up with neither (`ErrWouldLockAccount`) | P0 |  |
| ST-74 | GetLinkedAccountsByUserID | Happy Path | Two linked accounts exist for a user | Real Postgres | Returns both rows | P2 |  |
| ST-75 | AddLinkedAccountToDB | Error Path | Second insert reuses an existing `(platform, platformID)` pair under a different `userID` | Real Postgres unique-violation on `linked_accounts_platform_unique` | Returns `ErrAlreadyLinked` | P0 |  |
| ST-76 | GetSessionFromDB | Error Path | No session with that ID | Real Postgres, unknown session ID | Returns nil, `ErrNotFound` | P2 | |
| ST-77 | GetLinkedAccountByPlatformName | Error Path | No linked account with that platform and username | Real Postgres | Returns nil, `ErrNotFound` | P2 | |
| ST-78 | GetLinkedAccountByPlatformName | Edge Case | Two linked accounts share the platform and username | Real Postgres, two users linked on the same platform with the same username | Returns nil, `ErrDuplicateLinkedAccount` | P2 | |
| ST-79 | GetOAuthTokenByUserID | Error Path | No token for that user and platform | Real Postgres | Returns nil, `ErrNotFound` | P2 | |
| ST-80 | GetSessionFromCache | Error Path | No cached session for that ID | Real Redis, unknown session ID | Returns nil, `ErrNotFound` | P2 | |
| ST-81 | GetAccountSettings | Error Path (live) | No account exists for userID | Real Postgres, unknown user ID | Returns nil settings and `ErrNotFound` | P1 |  |
| ST-82 | SetPasswordAuthEnabled | Error Path (live) | Enable for a user ID with no account | Real Postgres, unknown user ID | Returns `ErrNotFound`, not `ErrNoPasswordSet` | P1 |  |
| ST-83 | SetPasswordAuthEnabled | Error Path (live) | Disable for a user ID with no account | Real Postgres, unknown user ID | Returns `ErrNotFound`, not `ErrWouldLockAccount` | P1 |  |

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| TY-01 | NewAccount | Happy Path | Valid username/email/password | `database.GenSnowflake`/`HashPassword` succeed | Returns `*Account` with `UserID` set, `Username` set, `Email == &email`, non-empty `HashedSecret`/`Salt`, nil error | P1 |  |
| TY-02 | NewAccount | Edge Case | `email == ""` | | `Account.Email == nil` | P2 |  |
| TY-04 | NewPasswordLessAccount | Happy Path | Valid username/email | | Returns `*Account` with `UserID`, `Username`, `Email == &email`; `HashedSecret`/`Salt` nil | P2 |  |
| TY-05 | NewPasswordLessAccount | Edge Case | `email == ""` | | `Account.Email == nil` | P3 |  |
| TY-06 | NewIDOnlyAccount | Happy Path | n/a | | Returns `*Account` with only `UserID` set | P2 |  |
| TY-07 | IDKeyWithSecret | Happy Path | Same password/salt/secret/params called twice | | Identical output both times (deterministic) | P2 |  |
| TY-08 | IDKeyWithSecret | Edge Case | Same password/salt/params, different `secret` | | Different output (the pepper/secret genuinely affects the digest) | P1 |  |
| TY-09 | HashPassword | Happy Path | Any password | `rand.Read` succeeds | Sets `user.HashedSecret` (32 bytes) and `user.Salt` (16 bytes); nil error | P1 |  |
| TY-10 | HashPassword | Edge Case | Called twice with the same password on two different accounts | | Produces different `Salt`/`HashedSecret` each time (random per-call salt) | P2 |  |
| TY-12 | ValidateUser | Happy Path | `HashPassword("correct")` already applied | `ValidateUser("correct")` | Returns true | P0 |  |
| TY-13 | ValidateUser | Edge Case | Same setup | `ValidateUser("wrong")` | Returns false | P0 |  |
| TY-14 | ValidateUser | Edge Case | `HashedSecret`/`Salt` both nil (password never set) | | Returns false without panicking | P1 |  |
| TY-15 | DummyValidateUser | Happy Path | Any password | | Completes without panicking or returning a value | P2 |  |
| TY-17 | AddRole | Happy Path | `Roles == nil` | `AddRole("admin")` | `Roles == ["admin"]` | P2 |  |
| TY-18 | AddRole | Edge Case | `Roles` already contains `"admin"` | `AddRole("admin")` again | `Roles == ["admin","admin"]` (no de-duplication) | P3 |  |
| TY-19 | RemoveRole | Happy Path | `Roles == ["a","b","c"]` | `RemoveRole("b")` | `Roles == ["a","c"]` | P1 |  |
| TY-20 | RemoveRole | Edge Case | Role not present | `RemoveRole("z")` | `Roles` unchanged, no panic | P2 |  |
| TY-21 | RemoveRole | Edge Case | `Roles` empty | `RemoveRole("z")` | No panic, `Roles` remains empty | P3 |  |
| TY-22 | DefaultAccountSettings | Accessor | `userID = "u1"` | | Returns `&AccountSettings{UserID:"u1", PasswordAuthEnabled:true}` | P2 |  |
| TY-23 | NewSession (Account) | Happy Path | `Account.Roles == ["system"]` | `perms.GetRoleByName("system")` succeeds | Returns `*Session` with `Permissions` built from `RoleSystem`'s scopes (`name\|value` pairs), `UserID` matching, `ExpiresAt` equal to the given value, nil error | P1 |
| TY-24 | NewSession (Account) | Edge Case | `Account.Roles` contains an unknown role name | `GetRoleByName` fails for it | That role's permissions are silently skipped; no error surfaced | P1 |  |
| TY-25 | NewSession (Account) | Edge Case | `Account.Roles` empty | | `Session.Permissions` empty, nil error | P2 |  |
| TY-26 | NewLinkedAccount | Happy Path | userID, platform, username, platformID, data given | | Returns `*LinkedAccount` with all fields copied, `Verified == true`, `LoginEnabled == true` | P2 |  |
| TY-27 | init | Happy Path | Package loads under the required `PEPPER` env (the precondition every other test in this file already runs under) | | `pepper` is populated from env without `log.Fatal` firing | P2 | Asserts init's already-established postcondition rather than re-invoking it |
| TY-28 | init | Error Path | `PEPPER` unset | Test binary re-exec'd as a subprocess with `PEPPER=""` | Subprocess exits non-zero via `log.Fatal(msgPepperUnset)` | P1 | Re-exec/TestCrasher pattern (see `os/exec` docs) |

## user.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| US-01 | NewUserService | Happy Path | Service wired to the store's sub-stores | fake `Store` | Returned `UserService` delegates to the exact `AccountStore`/`LinkAccountStore`/`AccountSettingsStore` instances returned | P2 |  |
| US-02 | GetUser | Happy Path | `as.GetAccountByID` succeeds | | Returns the account, nil | P1 |  |
| US-03 | GetUser | Error Path | `as.GetAccountByID` fails | | Error propagated unchanged | P2 |  |
| US-04 | GetUserFromPlatform | Happy Path | `als.GetLinkedAccountByPlatformID` then `as.GetAccountByID` both succeed | | Returns the resolved account, nil | P1 |  |
| US-05 | GetUserFromPlatform | Error Path | `als` lookup fails (e.g. `ErrNotFound`) | | Returns nil, error; `as.GetAccountByID` never called | P2 |  |
| US-06 | GetUserFromPlatform | Error Path | `als` lookup succeeds but `as.GetAccountByID` fails | | Error propagated unchanged | P2 |  |
| US-07 | GetUserPermissions | Happy Path | `as.GetAccountByID` returns `Account{Roles:["system"]}` | `GetRoleByName` resolves | Returns the flattened `"name\|value"` permissions for `RoleSystem` | P1 |
| US-08 | GetUserPermissions | Error Path | `as.GetAccountByID` fails | | Returns nil, error | P2 |  |
| US-09 | GetUserPermissions | Edge Case | `Account.Roles` contains an unknown role name among valid ones | `GetRoleByName` fails for it | That role is skipped, no error surfaced, other roles' permissions still included | P1 |  |
| US-10 | GetUserPermissions | Edge Case | `Account.Roles` empty | | Returns nil permissions, nil error | P2 |  |
| US-11 | UpdateUser | Happy Path | `as.GetAccountByID` succeeds; `user` has `Username`/`Email`/`Roles` set | | Merges those fields onto the fetched account and calls `as.UpdateAccountInDB` once with the merged result | P1 |  |
| US-12 | UpdateUser | Edge Case | `user.Username == ""`, `user.Email == nil`, `user.Roles == nil` | | Existing account's `Username`/`Email`/`Roles` are preserved unchanged (only non-zero fields overwrite) | P0 |  |
| US-13 | UpdateUser | Error Path | `as.GetAccountByID` fails | | Returns error; `UpdateAccountInDB` never called | P2 |  |
| US-14 | UpdateUser | Error Path | `as.UpdateAccountInDB` fails | | Error propagated unchanged | P2 |  |
| US-15 | UpdateUserFromPlatform | Happy Path | Brand-new platform identity | `als.GetLinkedAccountByPlatformID` → `ErrNotFound`; `AddAccountToDB`/`AddLinkedAccountToDB`/`UpdateLinkedAccount`/`GetAccountByID` all succeed | Returns the new account, nil; `AddAccountToDB` and `AddLinkedAccountToDB` each called once | P1 |  |
| US-16 | UpdateUserFromPlatform | Happy Path | Existing platform identity | `als.GetLinkedAccountByPlatformID` succeeds immediately | Account creation skipped entirely (`AddAccountToDB` never called); `la.Data` updated via `UpdateLinkedAccount`; resolved account returned | P1 |  |
| US-17 | UpdateUserFromPlatform | Error Path | Initial lookup fails with a non-`ErrNotFound` error | | Error propagated immediately; nothing else called | P2 |  |
| US-18 | UpdateUserFromPlatform | Error Path | `as.AddAccountToDB` fails (new-identity path) | | Error propagated; `als.AddLinkedAccountToDB` never called | P2 |  |
| US-19 | UpdateUserFromPlatform | Error Path | `als.AddLinkedAccountToDB` fails with a generic (non-`ErrAlreadyLinked`) error; cleanup succeeds | `as.DeleteAccountFromDB` succeeds | Returns the original error; `DeleteAccountFromDB` called once with the placeholder's `UserID` | P0 |  |
| US-20 | UpdateUserFromPlatform | Error Path | Same as US-19, but cleanup (`DeleteAccountFromDB`) also fails | | Returns a wrapped error naming both failures | P1 |  |
| US-21 | UpdateUserFromPlatform | Edge Case | `als.AddLinkedAccountToDB` fails with `ErrAlreadyLinked` ("lost the race"); cleanup and winner re-fetch succeed | | Proceeds using the winner's linked account (not the caller's placeholder); resolves and returns the winner's account, nil error | P0 |  |
| US-22 | UpdateUserFromPlatform | Error Path | Lost the race, cleanup succeeds, but the re-fetch (`GetLinkedAccountByPlatformID`) fails | | Error propagated | P2 |  |
| US-23 | UpdateUserFromPlatform | Error Path | `als.UpdateLinkedAccount` fails | | Error propagated | P2 |  |
| US-24 | UpdateUserFromPlatform | Error Path | Final `as.GetAccountByID` fails | | Error propagated | P2 |  |
| US-25 | UpdateUserFromPlatform | Concurrency Invariant | Two concurrent callers race to resolve/create an account for the same, brand-new platform identity | Two goroutines call `UpdateUserFromPlatform` simultaneously against a shared, mutex-serialized fake store enforcing the real `(platform, platformID)` uniqueness constraint (the losing `AddLinkedAccountToDB` call gets `ErrAlreadyLinked`) | Both goroutines return without error and resolve to the same account (`UserID` equal); exactly one account row exists afterward (the loser's placeholder was cleaned up) | P0 | design: 50 trials x 8 concurrent callers; a barrier holds each goroutine's first lookup until all 8 arrive, since an untimed trial isn't guaranteed to hit the "lost the race" branch |
| US-26 | DeleteUser | Happy Path | `as.DeleteAccountFromDB` succeeds | | Returns nil | P1 |  |
| US-27 | DeleteUser | Error Path | `as.DeleteAccountFromDB` fails | | Error propagated unchanged | P2 |  |
| US-28 | GetUserLinkedAccounts | Happy Path | `als.GetLinkedAccountsByUserID` succeeds | | Returns the slice, nil | P1 |  |
| US-29 | GetUserLinkedAccounts | Error Path | `als.GetLinkedAccountsByUserID` fails | | Error propagated unchanged | P2 |  |
| US-30 | UnlinkPlatform | Happy Path | `als.DeleteLinkedAccount` succeeds | | Returns nil | P1 |  |
| US-31 | UnlinkPlatform | Error Path | `als.DeleteLinkedAccount` fails (e.g. `ErrWouldLockAccount`) | | Error propagated unchanged | P2 |  |
| US-32 | SetPlatformLoginEnabled | Happy Path | `als.SetLinkedAccountLoginEnabled` succeeds | | Returns nil | P1 |  |
| US-33 | SetPlatformLoginEnabled | Error Path | `als.SetLinkedAccountLoginEnabled` fails | | Error propagated unchanged | P2 |  |
| US-34 | GetAccountSettings | Happy Path | `ass.GetAccountSettings` succeeds | | Returns the settings, nil | P1 |  |
| US-35 | GetAccountSettings | Error Path | `ass.GetAccountSettings` fails | | Error propagated unchanged | P2 |  |
| US-36 | SetPasswordAuthEnabled | Happy Path | `ass.SetPasswordAuthEnabled` succeeds | | Returns nil | P1 |  |
| US-37 | SetPasswordAuthEnabled | Error Path | `ass.SetPasswordAuthEnabled` fails | | Error propagated unchanged | P2 |  |
| US-38 | UpdateUserFromPlatform | Error Path | Same as US-20, asserting both causes | `AddLinkedAccountToDB` fails with `testerrors.ErrInsertFailed`; `DeleteAccountFromDB` fails with `testerrors.ErrBoom` | Returns an error wrapping both `testerrors.ErrInsertFailed` and `testerrors.ErrBoom` | P2 | |
| US-39 | GetAccountSettings | Error Path | `ass.GetAccountSettings` returns `ErrNotFound` | fake settings store returns `ErrNotFound` | Returns an error matching `ErrNotFound` | P2 |  |
| US-40 | SetPasswordAuthEnabled | Error Path | `ass.SetPasswordAuthEnabled` returns `ErrNotFound` | fake settings store returns `ErrNotFound` | Returns an error matching `ErrNotFound` | P2 |  |
