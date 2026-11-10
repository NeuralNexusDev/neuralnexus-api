# Test plan: modules/auth/account

## service

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| AC-02 | GetAccountByID | Happy Path | Loading the account succeeds | The account store holds a known account | Returns that account and no error | P1 |  |
| AC-03 | GetAccountByID | Error Path | Loading the account fails | The account store returns `ErrNotFound` | Returns no account and `ErrNotFound`, unchanged | P2 |  |
| AC-04 | GetAccountByUsername | Happy Path | Loading the account by username succeeds | | Returns the account and no error | P1 |  |
| AC-05 | GetAccountByUsername | Error Path | Loading the account by username fails | | Error propagated unchanged | P2 |  |
| AC-06 | GetAccountByEmail | Happy Path | Loading the account by email succeeds | | Returns the account and no error | P1 |  |
| AC-07 | GetAccountByEmail | Error Path | Loading the account by email fails | | Error propagated unchanged | P2 |  |
| AC-08 | AddAccount | Happy Path | Storing the account succeeds | | Returns no error; the account is stored once | P1 |  |
| AC-09 | AddAccount | Error Path | Storing the account fails with `ErrUsernameAlreadyExists` | | Error propagated unchanged | P2 |  |
| AC-10 | UpdateAccount | Happy Path | Updating the account succeeds | | Returns no error | P1 |  |
| AC-11 | UpdateAccount | Error Path | Updating the account fails | | Error propagated unchanged | P2 |  |
| AC-12 | DeleteAccount | Happy Path | Deleting the account succeeds | | Returns no error | P1 |  |
| AC-13 | DeleteAccount | Error Path | Deleting the account fails | | Error propagated unchanged | P2 |  |
| AC-14 | IsPasswordAuthEnabled | Happy Path | Reading the account settings shows password login enabled | | Returns true and no error | P1 |  |
| AC-15 | IsPasswordAuthEnabled | Edge Case | Reading the account settings shows password login disabled | | Returns false and no error | P2 |  |
| AC-16 | IsPasswordAuthEnabled | Error Path | Reading the account settings fails | | Returns false and the error | P2 |  |
| AC-17 | NewSession | Happy Path | the account holds role ids | The role store holds three permissions for the account's role | Returns a session with a numeric (snowflake) id, the issue time and last-used time both set to now, the account's user id, the expiry time as given and the permissions exactly as the role store returned them; the role store is asked once, with the account's role ids | P1 |   |
| AC-18 | NewSession | Edge Case | the account holds two role ids | The role store holds permissions for both roles | The role store is asked once for both ids and the permissions of both roles are returned | P1 |   |
| AC-19 | NewSession | Edge Case | the account has no roles |  | The permissions are empty and not missing, the error is nil and the role store is never asked | P2 |   |
| AC-20 | NewSession | Error Path | the role store fails | The role store fails | Returns no session and an error matching the role store's error, so no session is issued with missing permissions | P1 |   |
| AC-21 | NewSession | Edge Case | the account's role grants nothing | The role store holds no permissions for the role | The permissions are empty and not missing | P1 |   |
| AC-24 | AddAccount | Edge Case | the call context is cancelled before the write | The account store records whether the context it receives is done and which marker value it carries | The call succeeds, and the insert sees a context that is not done and still carries the marker, so the account insert is not cancelled with the client | P1 |  |

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ACT-01 | NewAccount | Happy Path | Valid username, email and password | The snowflake id and the password hash are produced | Returns an account with a user id, the given username, the given email, and a non-empty hashed secret and salt | P1 |  |
| ACT-02 | NewAccount | Edge Case | `email == ""` | | `Account.Email == nil` | P2 |  |
| ACT-05 | NewPasswordLessAccount | Edge Case | `email == ""` | | `Account.Email == nil` | P3 |  |
| ACT-07 | IDKeyWithSecret | Happy Path | Same password/salt/secret/params called twice | | Identical output both times (deterministic) | P2 |  |
| ACT-08 | IDKeyWithSecret | Edge Case | Same password, salt and parameters, different secret | | Different output (the pepper/secret genuinely affects the digest) | P1 |  |

## Account

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ACT-09 | HashPassword | Happy Path | Any password | | The account gets a hashed secret of 32 bytes and a salt of 16 bytes | P1 |  |
| ACT-10 | HashPassword | Edge Case | Called twice with the same password on two different accounts | | Produces different `Salt`/`HashedSecret` each time (random per-call salt) | P2 |  |
| ACT-12 | ValidateUser | Happy Path | an account hashed with a fixed salt | `ValidateUser("correct")` | Returns true | P0 |  |
| ACT-13 | ValidateUser | Edge Case | Same setup | `ValidateUser("wrong")` | Returns false | P0 |  |
| ACT-14 | ValidateUser | Edge Case | `HashedSecret`/`Salt` both nil (password never set) | | Returns false without panicking | P1 |  |
| ACT-19 | RemoveRole | Happy Path | `Roles == ["a","b","c"]` | `RemoveRole("b")` | `Roles == ["a","c"]` | P1 |  |
| ACT-20 | RemoveRole | Edge Case | Role not present | `RemoveRole("z")` | `Roles` unchanged, no panic | P2 |  |
| ACT-21 | RemoveRole | Edge Case | `Roles` empty | `RemoveRole("z")` | No panic, `Roles` remains empty | P3 |  |

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ACS-08 | translateAccountConstraintErr | Edge Case | The error is not a database error | A plain error that is not a database constraint error | Returned unchanged | P1 |  |
| ACS-09 | translateAccountConstraintErr | Edge Case | A database error whose code is not a unique violation | | Returned unchanged | P2 |  |
| ACS-10 | translateAccountConstraintErr | Edge Case | A unique violation on a constraint the store does not know | | Returned unchanged (falls through the switch) | P1 |  |
| ACS-11 | translateAccountConstraintErr | Happy Path | `Code == "23505"`, `ConstraintName == "accounts_email_key"` | | Returns `ErrEmailAlreadyExists` | P0 |  |
| ACS-12 | translateAccountConstraintErr | Happy Path | `Code == "23505"`, `ConstraintName == "accounts_username_key"` | | Returns `ErrUsernameAlreadyExists` | P0 |  |

## store

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ACS-13 | AddAccountToDB | Error Path | DB unreachable | The insert fails with a plain connection error, not a database constraint error | Returns that connection error unchanged (not translated to a sentinel) | P1 |  |
| ACS-47 | GetAccountByID | Happy Path | No account row exists for userID | Real Postgres, live query | Returns nil, `ErrNotFound` | P1 |  |
| ACS-48 | GetAccountByUsername | Happy Path | No account row exists for username | | Returns nil, `ErrNotFound` | P2 |  |
| ACS-49 | GetAccountByEmail | Happy Path | No account row exists for email | | Returns nil, `ErrNotFound` | P2 |  |
| ACS-50 | AddAccountToDB | Edge Case | Two accounts inserted with `Email == nil` | Real Postgres; the email column must be unique but may be missing | Both inserts succeed; two missing emails do not collide | P2 |  |
| ACS-51 | AddAccountToDB | Edge Case | Two accounts inserted with no username | Real Postgres; an empty username is stored as missing, so the unique username rule does not apply | Both inserts succeed (regression test for a previously-fixed empty-username collision bug) | P0 |  |
| ACS-52 | AddAccountToDB | Error Path | Second account reuses an existing non-empty username | Real Postgres; the database refuses the duplicate username | Returns `ErrUsernameAlreadyExists` | P1 |  |
| ACS-53 | AddAccountToDB | Error Path | Second account reuses an existing non-empty email | Real Postgres; the database refuses the duplicate email | Returns `ErrEmailAlreadyExists` | P1 |  |
| ACS-85 | AddAccountToDB | Happy Path (live) | an account is added with two existing role ids | Real Postgres; the two roles exist | Loading the account by id returns the same two role ids in order, and loading it by username and by email each return two role ids | P1 |   |
| ACS-86 | UpdateAccountInDB | Happy Path (live) | the role ids of a stored account are replaced | Real Postgres; account without roles, an existing role | The next read returns only the new ids | P1 |   |
| ACS-87 | AddAccountToDB | Edge Case (live) | an account is added with nil roles | Real Postgres | The stored account has no role ids | P2 |   |
| ACS-88 | AddAccountToDB | Error Path (live) | a role id is not numeric | Real Postgres; one existing role id and one role id that is not a number | `ErrInvalidRoleID` and nothing is stored (`ErrNotFound` on read) | P1 |   |
| ACS-89 | UpdateAccountInDB | Error Path (live) | a role id is below one | Real Postgres; role id `0` | `ErrInvalidRoleID` | P2 |   |
| ACS-91 | AddAccountToDB | Error Path (live) | one of the role ids has no role | Real Postgres | `ErrUnknownRoleID` and nothing is stored (`ErrNotFound` on read) | P1 |   |
| ACS-92 | UpdateAccountInDB | Error Path (live) | the role id has no role | Real Postgres | `ErrUnknownRoleID` and no role ids are stored | P1 |   |
| ACS-93 | UpdateAccountInDB | Error Path (live) | role ids with a leading zero, a plus sign or a space | Real Postgres | `ErrInvalidRoleID` for each | P2 |   |
| ACS-94 | UpdateAccountInDB | Edge Case (live) | the same existing role id is given twice | Real Postgres | nil error and the stored account holds the id once | P2 |   |
| ACS-95 | AddAccountToDB | Concurrency Invariant (live) | a delete of the role is in flight, uncommitted, when the account is added | transaction deleting the role | The add waits for the commit and then returns `ErrUnknownRoleID`; nothing is stored | P1 |   |
| ACS-96 | ListAccounts | Happy Path (live) | accounts with a short ID and long IDs are stored, one without a username | An account with the short id 95 and accounts with long ids ending 601 and 602 stored, and a long id ending 603 stored without a username | The accounts come back ordered by number, so 95 is before the long ids; every account lists its roles as a non-nil list; the account without a username lists an empty username | P1 |  |
| ACS-97 | ListAccounts | Happy Path (live) | a limit and an offset select a page | The accounts above | A page of 2 starting at the account ending 601 returns the accounts ending 601 and 602; a page of 1 starting one later returns only the one ending 602; a page of 1 starting at the account ending 601 returns only that one | P1 |  |
| ACS-98 | ListAccounts | Edge Case (live) | the offset is past the last account | offset 1000000 | An empty page and a nil error | P2 |  |

## settingsStore

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ACS-37 | GetAccountSettings | Error Path | DB unreachable | The database is unreachable | Returns no settings and the connection error unchanged; the default settings are not returned in its place | P1 |  |
| ACS-67 | GetAccountSettings | Edge Case | The account exists but has no settings row yet | Real Postgres; the settings read finds no row | Returns settings with password login enabled and no error | P1 |  |
| ACS-68 | SetPasswordAuthEnabled | Error Path | Enabling password login for an account that has no password | The account has no hashed secret, so the database stores no settings | Returns `ErrNoPasswordSet` | P1 |  |
| ACS-70 | SetPasswordAuthEnabled | Edge Case | Disabling password login for an account that has a password but no linked platform | Password login is the account's only login method | Returns `ErrWouldLockAccount`; password login stays enabled | P0 |  |
| ACS-81 | GetAccountSettings | Error Path (live) | No account exists for userID | Real Postgres, unknown user ID | Returns nil settings and `ErrNotFound` | P1 |  |
| ACS-82 | SetPasswordAuthEnabled | Error Path (live) | Enable for a user ID with no account | Real Postgres, unknown user ID | Returns `ErrNotFound` | P1 |  |
| ACS-83 | SetPasswordAuthEnabled | Error Path (live) | Disable for a user ID with no account | Real Postgres, unknown user ID | Returns `ErrNotFound` | P1 |  |
| ACS-84 | SetPasswordAuthEnabled | Concurrency Invariant (live) | The account is deleted while the enable call waits on its row | Real Postgres; a separate transaction has deleted the account but not committed; the call is running and blocked on the account row | After the delete commits the call returns `ErrNotFound` | P1 |  |

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
| ACH-10 | LogoutHandler | Happy Path | Logout with a valid session | Context carries a non-nil *auth.Session, ss.DeleteSession succeeds | 204 No Content, session cookie cleared (empty value, expiry in the past) | P1 |  |
| ACH-12 | LogoutHandler | Error Path | session.Service.DeleteSession fails | Valid session in context, ss.DeleteSession returns an error | 500 Internal Server Error `msgFailedToDeleteSession` | P2 |  |
| ACH-58 | LoginHandler | Error Path | the account service fails to build the session | Account, password and password login are valid; building the session fails with an error | 500 Internal Server Error, `detail` is `msgAuthenticationFailed`, and no session is stored | P2 |  |
| ACH-59 | LoginHandler | Error Path | Account lookup fails with a non-not-found error | Looking up the account fails with a database error | 500 Internal Server Error `msgAuthenticationFailed`; no session cookie | P2 |  |
