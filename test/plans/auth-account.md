# Test plan: modules/auth/account

## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| AC-01 | NewService | Happy Path | Service is wired to the Store and SettingsStore passed to NewService | fake `Store` and fake `SettingsStore` passed to `NewService` | Returned `Service`, when exercised, delegates account methods to the exact `Store` passed in (built by `NewStore`) and settings methods to the exact `SettingsStore` passed in (built by `NewSettingsStore`) | P2 |  |
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
| AC-17 | NewSession (service) | Happy Path | the account holds role ids | fake role store returns three permissions | Returns a `*Session` with a snowflake `ID`, `IssuedAt` and `LastUsedAt` set to now, `UserID`, `ExpiresAt` as given and `Permissions` exactly as the role store returned them; the role store is called once with the account's role ids | P1 |   |
| AC-18 | NewSession (service) | Edge Case | the account holds two role ids | fake role store | The role store is called once with both ids and the permissions of both roles are returned | P1 |   |
| AC-19 | NewSession (service) | Edge Case | the account has no roles |  | `Permissions` is empty and non-nil, the error is nil and the role store is never called | P2 |   |
| AC-20 | NewSession (service) | Error Path | the role store fails | role store returns `testutil.ErrBoom` | Returns a nil session and an error matching the store error, so no session is issued with missing permissions | P1 |   |
| AC-21 | NewSession (service) | Edge Case | the account's role grants nothing | role store returns no permissions | `Permissions` is empty and non-nil | P1 |   |
| AC-22 | NewSession (service) | Error Path | the snowflake generator fails | the test binary re-executes itself with `SNOWFLAKE_NODE_ID` above 31 | Returns a nil session and an error | P2 | Re-exec because the snowflake settings are read when the package loads |
| AC-23 | GetAccountByID (service) | Happy Path | a context value set on the call | fake `Store` | The store receives the same context, so the value reads back from it | P1 |  |

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ACT-01 | NewAccount | Happy Path | Valid username/email/password | `database.GenSnowflake`/`HashPassword` succeed | Returns `*Account` with `UserID` set, `Username` set, `Email == &email`, non-empty `HashedSecret`/`Salt`, nil error | P1 |  |
| ACT-02 | NewAccount | Edge Case | `email == ""` | | `Account.Email == nil` | P2 |  |
| ACT-04 | NewPasswordLessAccount | Happy Path | Valid username/email | | Returns `*Account` with `UserID`, `Username`, `Email == &email`; `HashedSecret`/`Salt` nil | P2 |  |
| ACT-05 | NewPasswordLessAccount | Edge Case | `email == ""` | | `Account.Email == nil` | P3 |  |
| ACT-06 | NewIDOnlyAccount | Happy Path | n/a | | Returns `*Account` with only `UserID` set | P2 |  |
| ACT-07 | IDKeyWithSecret | Happy Path | Same password/salt/secret/params called twice | | Identical output both times (deterministic) | P2 |  |
| ACT-08 | IDKeyWithSecret | Edge Case | Same password/salt/params, different `secret` | | Different output (the pepper/secret genuinely affects the digest) | P1 |  |
| ACT-09 | HashPassword | Happy Path | Any password | `rand.Read` succeeds | Sets `user.HashedSecret` (32 bytes) and `user.Salt` (16 bytes); nil error | P1 |  |
| ACT-10 | HashPassword | Edge Case | Called twice with the same password on two different accounts | | Produces different `Salt`/`HashedSecret` each time (random per-call salt) | P2 |  |
| ACT-12 | ValidateUser | Happy Path | `HashPassword("correct")` already applied | `ValidateUser("correct")` | Returns true | P0 |  |
| ACT-13 | ValidateUser | Edge Case | Same setup | `ValidateUser("wrong")` | Returns false | P0 |  |
| ACT-14 | ValidateUser | Edge Case | `HashedSecret`/`Salt` both nil (password never set) | | Returns false without panicking | P1 |  |
| ACT-15 | DummyValidateUser | Happy Path | Any password | | Completes without panicking or returning a value | P2 |  |
| ACT-17 | AddRole | Happy Path | `Roles == nil` | `AddRole("admin")` | `Roles == ["admin"]` | P2 |  |
| ACT-18 | AddRole | Edge Case | `Roles` already contains `"admin"` | `AddRole("admin")` again | `Roles == ["admin","admin"]` (no de-duplication) | P3 |  |
| ACT-19 | RemoveRole | Happy Path | `Roles == ["a","b","c"]` | `RemoveRole("b")` | `Roles == ["a","c"]` | P1 |  |
| ACT-20 | RemoveRole | Edge Case | Role not present | `RemoveRole("z")` | `Roles` unchanged, no panic | P2 |  |
| ACT-21 | RemoveRole | Edge Case | `Roles` empty | `RemoveRole("z")` | No panic, `Roles` remains empty | P3 |  |
| ACT-22 | DefaultAccountSettings | Accessor | `userID = "u1"` | | Returns `&AccountSettings{UserID:"u1", PasswordAuthEnabled:true}` | P2 |  |
| ACT-27 | init | Happy Path | Package loads under the required `PEPPER` env (the precondition every other test in this file already runs under) | | `pepper` is populated from env without `log.Fatal` firing | P2 | Asserts init's already-established postcondition rather than re-invoking it |
| ACT-28 | init | Error Path | `PEPPER` unset | Test binary re-exec'd as a subprocess with `PEPPER=""` | Subprocess exits non-zero via `log.Fatal(msgPepperUnset)` | P1 | Re-exec/TestCrasher pattern (see `os/exec` docs) |

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ACS-08 | translateAccountConstraintErr | Edge Case | `err` is not a `*pgconn.PgError` | a plain error that is not a `*pgconn.PgError` (`testutil.ErrBoom`) | Returned unchanged | P1 |  |
| ACS-09 | translateAccountConstraintErr | Edge Case | `*pgconn.PgError` with `Code != "23505"` | | Returned unchanged | P2 |  |
| ACS-10 | translateAccountConstraintErr | Edge Case | `Code == "23505"`, unrecognized `ConstraintName` | | Returned unchanged (falls through the switch) | P1 |  |
| ACS-11 | translateAccountConstraintErr | Happy Path | `Code == "23505"`, `ConstraintName == "accounts_email_key"` | | Returns `ErrEmailAlreadyExists` | P0 |  |
| ACS-12 | translateAccountConstraintErr | Happy Path | `Code == "23505"`, `ConstraintName == "accounts_username_key"` | | Returns `ErrUsernameAlreadyExists` | P0 |  |
| ACS-13 | AddAccountToDB | Error Path | DB unreachable | `Exec` fails with a plain connection error (not `*pgconn.PgError`) | Returns that raw error unchanged (not translated to a sentinel) | P1 |  |
| ACS-14 | GetAccountByID | Error Path | DB unreachable | `Query` fails before any row scan | Returns nil, the raw connection error (not `ErrNotFound`) | P1 |  |
| ACS-15 | GetAccountByUsername | Error Path | DB unreachable | | Returns nil, raw connection error | P2 |  |
| ACS-16 | GetAccountByEmail | Error Path | DB unreachable | | Returns nil, raw connection error | P2 |  |
| ACS-17 | UpdateAccountInDB | Error Path | DB unreachable | | Returns the raw connection error unchanged | P1 |  |
| ACS-18 | DeleteAccountFromDB | Error Path | DB unreachable | | Returns the raw connection error | P2 |  |
| ACS-37 | GetAccountSettings | Error Path | DB unreachable | | Returns nil, raw error (the `DefaultAccountSettings` fallback is `ErrNoRows`-specific and not reached) | P1 |  |
| ACS-38 | SetPasswordAuthEnabled | Error Path | DB unreachable, `enabled=true` | `s.db.Begin` fails | Returns raw error | P1 |  |
| ACS-39 | SetPasswordAuthEnabled | Error Path | DB unreachable, `enabled=false` | | Returns raw error | P2 |  |
| ACS-47 | GetAccountByID | Happy Path | No account row exists for userID | Real Postgres, live query | Returns nil, `ErrNotFound` | P1 |  |
| ACS-48 | GetAccountByUsername | Happy Path | No account row exists for username | | Returns nil, `ErrNotFound` | P2 |  |
| ACS-49 | GetAccountByEmail | Happy Path | No account row exists for email | | Returns nil, `ErrNotFound` | P2 |  |
| ACS-50 | AddAccountToDB | Edge Case | Two accounts inserted with `Email == nil` | Real Postgres NULL semantics on a `UNIQUE` column | Both inserts succeed, no false collision | P2 |  |
| ACS-51 | AddAccountToDB | Edge Case | Two accounts inserted via `NewIDOnlyAccount` (`Username == ""`) | `NULLIF` converts `""` to `NULL` before insert | Both inserts succeed (regression test for a previously-fixed empty-username collision bug) | P0 |  |
| ACS-52 | AddAccountToDB | Error Path | Second account reuses an existing non-empty username | Real Postgres unique-violation on `accounts_username_key` | Returns `ErrUsernameAlreadyExists` | P1 |  |
| ACS-53 | AddAccountToDB | Error Path | Second account reuses an existing non-empty email | Real Postgres unique-violation on `accounts_email_key` | Returns `ErrEmailAlreadyExists` | P1 |  |
| ACS-67 | GetAccountSettings | Edge Case | The account exists but has no `account_settings` row yet | Real Postgres, `pgx.ErrNoRows` | Returns `DefaultAccountSettings(userID)` (`PasswordAuthEnabled: true`), nil error | P1 |  |
| ACS-68 | SetPasswordAuthEnabled | Error Path | `enabled=true`, account has no `hashed_secret` | 0 rows match the `INSERT ... WHERE EXISTS` clause | Returns `ErrNoPasswordSet` | P1 |  |
| ACS-70 | SetPasswordAuthEnabled | Edge Case | `enabled=false`, account has a password but no linked platform | Guard denies (no fallback) | Returns `ErrWouldLockAccount`; settings unchanged | P0 |  |
| ACS-81 | GetAccountSettings | Error Path (live) | No account exists for userID | Real Postgres, unknown user ID | Returns nil settings and `ErrNotFound` | P1 |  |
| ACS-82 | SetPasswordAuthEnabled | Error Path (live) | Enable for a user ID with no account | Real Postgres, unknown user ID | Returns `ErrNotFound` | P1 |  |
| ACS-83 | SetPasswordAuthEnabled | Error Path (live) | Disable for a user ID with no account | Real Postgres, unknown user ID | Returns `ErrNotFound` | P1 |  |
| ACS-84 | SetPasswordAuthEnabled | Concurrency Invariant (live) | The account is deleted while the enable call waits on its row | Real Postgres; a separate transaction has deleted the account but not committed; the call is running and blocked on the account row | After the delete commits the call returns `ErrNotFound` | P1 |  |
| ACS-85 | AddAccountToDB | Happy Path (live) | an account is added with two existing role ids | Real Postgres; the two roles exist | `GetAccountByID`, `GetAccountByUsername` and `GetAccountByEmail` return the same two ids in order | P1 |   |
| ACS-86 | UpdateAccountInDB | Happy Path (live) | the role ids of a stored account are replaced | Real Postgres; account without roles, an existing role | The next read returns only the new ids | P1 |   |
| ACS-87 | AddAccountToDB | Edge Case (live) | an account is added with nil roles | Real Postgres | The stored account has no role ids | P2 |   |
| ACS-88 | AddAccountToDB | Error Path (live) | a role id is not numeric | Real Postgres; roles `7` and `admin` | `ErrInvalidRoleID` and nothing is stored (`ErrNotFound` on read) | P1 |   |
| ACS-89 | UpdateAccountInDB | Error Path (live) | a role id is below one | Real Postgres; role id `0` | `ErrInvalidRoleID` | P2 |   |
| ACS-91 | AddAccountToDB | Error Path (live) | one of the role ids has no role | Real Postgres | `ErrUnknownRoleID` and nothing is stored (`ErrNotFound` on read) | P1 |   |
| ACS-92 | UpdateAccountInDB | Error Path (live) | the role id has no role | Real Postgres | `ErrUnknownRoleID` and no role ids are stored | P1 |   |
| ACS-93 | UpdateAccountInDB | Error Path (live) | role ids with a leading zero, a plus sign or a space | Real Postgres | `ErrInvalidRoleID` for each | P2 |   |
| ACS-94 | UpdateAccountInDB | Edge Case (live) | the same existing role id is given twice | Real Postgres | nil error and the stored account holds the id once | P2 |   |
| ACS-95 | AddAccountToDB | Concurrency Invariant (live) | a delete of the role is in flight, uncommitted, when the account is added | transaction deleting the role | The add waits for the commit and then returns `ErrUnknownRoleID`; nothing is stored | P1 |   |
| ACS-96 | ListAccounts | Happy Path (live) | accounts with a short ID and long IDs are stored, one without a username | accounts `95`, `...601`, `...602`, and `...603` stored without a username | The accounts come back ordered by number, so `95` is before the long IDs; every account lists its roles as a non-nil slice; the account without a username lists an empty username | P1 |  |
| ACS-97 | ListAccounts | Happy Path (live) | a limit and an offset select a page | the accounts above | `ListAccounts(2, i)` returns `...601` and `...602`, `ListAccounts(1, i+1)` returns only `...602` and `ListAccounts(1, i)` returns only `...601` | P1 |  |
| ACS-98 | ListAccounts | Edge Case (live) | the offset is past the last account | offset 1000000 | An empty page and a nil error | P2 |  |

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ACH-01 | LoginHandler | Happy Path | Login with username+password | Account exists for username, password matches, password auth enabled | 204 No Content, session cookie set, and the stored session carries the permissions the account service built | P1 |  |
| ACH-02 | LoginHandler | Happy Path | Login with email+password (username field empty) | Account exists for email, password matches, password auth enabled | 204 No Content, session cookie set | P1 |  |
| ACH-03 | LoginHandler | Error Path | Malformed request body | Body is not valid JSON | 400 Bad Request `msgInvalidUsernameOrPassword` | P2 |  |
| ACH-04 | LoginHandler | Error Path | Account lookup finds no account | Service.GetAccountByUsername/GetAccountByEmail returns `account.ErrNotFound` | 400 Bad Request `msgInvalidUsernameOrPassword` | P2 |  |
| ACH-05 | LoginHandler | Error Path | Wrong password | Account found, Account.ValidateUser returns false | 400 Bad Request `msgInvalidUsernameOrPassword` | P2 |  |
| ACH-06 | LoginHandler | Error Path | IsPasswordAuthEnabled errors | Account and password valid, Service.IsPasswordAuthEnabled returns an error | 500 Internal Server Error `msgAuthenticationFailed` | P2 |  |
| ACH-07 | LoginHandler | Edge Case | Password auth disabled for account | Account and password valid, IsPasswordAuthEnabled returns (false, nil) | 400 Bad Request `msgInvalidUsernameOrPassword` | P2 |  |
| ACH-08 | LoginHandler | Error Path | session.Service.AddSession fails | Account/password/enabled all valid, ss.AddSession returns an error | 500 Internal Server Error `msgAuthenticationFailed` | P2 |  |
| ACH-09 | LoginHandler | Error Path | session.Service.CreateJWT fails | AddSession succeeds, ss.CreateJWT returns an error | 500 Internal Server Error `msgAuthenticationFailed` | P2 |  |
| ACH-10 | LogoutHandler | Happy Path | Logout with a valid session | Context carries a non-nil *auth.Session, ss.DeleteSession succeeds | 204 No Content, session cookie cleared (empty value, expiry in the past) | P1 |  |
| ACH-11 | LogoutHandler | Edge Case | Session in context is a nil *auth.Session | Context value under reqctx.SessionKey is (*auth.Session)(nil) | 400 Bad Request `msgInvalidSession` | P3 |  |
| ACH-12 | LogoutHandler | Error Path | session.Service.DeleteSession fails | Valid session in context, ss.DeleteSession returns an error | 500 Internal Server Error `msgFailedToDeleteSession` | P2 |  |
| ACH-58 | LoginHandler | Error Path | the account service fails to build the session | Account/password/enabled all valid; `NewSession` returns `testutil.ErrBoom` | 500 Internal Server Error, `detail` is `msgAuthenticationFailed`, and no session is stored | P2 |  |
| ACH-59 | LoginHandler | Error Path | Account lookup fails with a non-not-found error | Service.GetAccountByUsername returns `testutil.ErrDBDown` | 500 Internal Server Error `msgAuthenticationFailed`; no session cookie | P2 |  |
| ACH-36 | SetSessionCookie | Happy Path | ss.CreateJWT succeeds | Valid session, ss.CreateJWT returns a JWT string | Returns nil error; Set-Cookie header carries the JWT value with Expires == time.Unix(session.ExpiresAt,0) | P1 |  |
| ACH-37 | SetSessionCookie | Error Path | ss.CreateJWT fails | ss.CreateJWT returns an error | Returns that error; no Set-Cookie header written | P2 |  |
| ACH-49 | sessionCookie | Accessor | Build a cookie for a given value/expiry | value="abc", expires=fixed time | Returns *http.Cookie with Name=mw.SessionCookieName, Domain=".neuralnexus.dev", Path="/", Expires=expires, Secure=true, HttpOnly=true, SameSite=Lax | P2 |  |
