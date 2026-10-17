# Test plan: modules/auth (core package)

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ST-01 | NewStore | Happy Path | Given a db pool and redis client | | Returns a non-nil `Store` wrapping both | P2 |  |
| ST-02 | Account | Accessor | | | Returns a non-nil `AccountStore` | P3 |  |
| ST-03 | AccountSettings | Accessor | | | Returns a non-nil `AccountSettingsStore` | P3 |  |
| ST-04 | Session | Accessor | | | Returns a non-nil `SessionStore` | P3 |  |
| ST-05 | LinkAccount | Accessor | | | Same pointer-identity check | P3 |  |
| ST-07 | OAuthToken | Accessor | | | Same pointer-identity check | P3 |  |
| ST-28 | AddLinkedAccountToDB | Error Path | DB unreachable | | Returns the raw connection error (not `ErrAlreadyLinked`, since it isn't a `*pgconn.PgError`) | P1 |  |
| ST-29 | UpdateLinkedAccount | Error Path | DB unreachable | | Returns raw error | P2 |  |
| ST-30 | GetLinkedAccountByPlatformID | Error Path | DB unreachable | | Returns nil, raw error | P1 |  |
| ST-31 | GetLinkedAccountByPlatformName | Error Path | DB unreachable | | Returns nil, raw error | P2 |  |
| ST-32 | GetLinkedAccountByUserID | Error Path | DB unreachable | | Returns nil, raw error | P2 |  |
| ST-33 | GetLinkedAccountsByUserID | Error Path | DB unreachable | | Returns nil, raw error | P2 |  |
| ST-34 | DeleteLinkedAccount | Error Path | DB unreachable | `s.db.Begin` fails immediately | Returns the raw error before the advisory-lock/guard logic runs | P1 |  |
| ST-35 | SetLinkedAccountLoginEnabled | Error Path | DB unreachable, `enabled=true` | Plain `Exec` (no tx) fails | Returns raw error | P1 |  |
| ST-36 | SetLinkedAccountLoginEnabled | Error Path | DB unreachable, `enabled=false` | `s.db.Begin` fails | Returns raw error before the advisory-lock/guard logic runs | P1 |  |
| ST-43 | AddOAuthTokenToDB | Error Path | DB unreachable | | Returns raw error | P2 |  |
| ST-44 | GetOAuthTokenByUserID | Error Path | DB unreachable | | Returns nil, raw error | P2 |  |
| ST-45 | UpdateOAuthToken | Error Path | DB unreachable | | Returns raw error | P2 |  |
| ST-46 | DeleteOAuthToken | Error Path | DB unreachable | | Returns raw error | P2 |  |
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
| ST-69 | SetPasswordAuthEnabled | Happy Path | `enabled=true`, account has `hashed_secret` | | Returns nil; settings read back `PasswordAuthEnabled=true` | P1 |  |
| ST-71 | SetPasswordAuthEnabled | Happy Path | `enabled=false`, account has a password and one verified+enabled link | Guard allows | Returns nil; settings read back `PasswordAuthEnabled=false` | P1 |  |
| ST-72 | SetPasswordAuthEnabled | Concurrency Invariant | `SetPasswordAuthEnabled(false)` races `DeleteLinkedAccount` (sole link) on the same account | Same lock, many trials | Exactly one succeeds every trial; account never ends up with neither a password nor a link (`ErrWouldLockAccount`) | P0 |  |
| ST-73 | SetPasswordAuthEnabled | Concurrency Invariant | `SetPasswordAuthEnabled(false)` races `SetLinkedAccountLoginEnabled` (sole link, false) on the same account | Same lock, many trials | Exactly one succeeds every trial; account never ends up with neither (`ErrWouldLockAccount`) | P0 |  |
| ST-74 | GetLinkedAccountsByUserID | Happy Path | Two linked accounts exist for a user | Real Postgres | Returns both rows | P2 |  |
| ST-75 | AddLinkedAccountToDB | Error Path | Second insert reuses an existing `(platform, platformID)` pair under a different `userID` | Real Postgres unique-violation on `linked_accounts_platform_unique` | Returns `ErrAlreadyLinked` | P0 |  |
| ST-77 | GetLinkedAccountByPlatformName | Error Path | No linked account with that platform and username | Real Postgres | Returns nil, `ErrNotFound` | P2 | |
| ST-78 | GetLinkedAccountByPlatformName | Edge Case | Two linked accounts share the platform and username | Real Postgres, two users linked on the same platform with the same username | Returns nil, `ErrDuplicateLinkedAccount` | P2 | |
| ST-79 | GetOAuthTokenByUserID | Error Path | No token for that user and platform | Real Postgres | Returns nil, `ErrNotFound` | P2 | |

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| TY-26 | NewLinkedAccount | Happy Path | userID, platform, username, platformID, data given | | Returns `*LinkedAccount` with all fields copied, `Verified == true`, `LoginEnabled == true` | P2 |  |

## user.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| US-01 | NewUserService | Happy Path | Service wired to the store's sub-stores | fake `Store` | Returned `UserService` delegates to the exact `AccountStore`/`LinkAccountStore`/`AccountSettingsStore` instances returned | P2 |  |
| US-02 | GetUser | Happy Path | `as.GetAccountByID` succeeds | | Returns the account, nil | P1 |  |
| US-03 | GetUser | Error Path | `as.GetAccountByID` fails | | Error propagated unchanged | P2 |  |
| US-04 | GetUserFromPlatform | Happy Path | `als.GetLinkedAccountByPlatformID` then `as.GetAccountByID` both succeed | | Returns the resolved account, nil | P1 |  |
| US-05 | GetUserFromPlatform | Error Path | `als` lookup fails (e.g. `ErrNotFound`) | | Returns nil, error; `as.GetAccountByID` never called | P2 |  |
| US-06 | GetUserFromPlatform | Error Path | `als` lookup succeeds but `as.GetAccountByID` fails | | Error propagated unchanged | P2 |  |
| US-07 | GetUserPermissions | Happy Path | `as.GetAccountByID` returns an account holding one role id | fake role store returns permission strings | Returns the role store's node permissions | P1 |  |
| US-08 | GetUserPermissions | Error Path | `as.GetAccountByID` fails | | Returns nil, error | P2 |  |
| US-09 | GetUserPermissions | Edge Case | `Account.Roles` holds two role ids | fake role store | The role store is called once with both ids and the permissions of both are returned | P1 |  |
| US-10 | GetUserPermissions | Edge Case | `Account.Roles` empty | | Returns empty non-nil permissions, nil error, without a role lookup | P2 |  |
| US-11 | UpdateUser | Happy Path | `as.GetAccountByID` succeeds; `user` has `Username`/`Email`/`Roles` set | | Merges those fields onto the fetched account and calls `as.UpdateAccountInDB` once with the merged result | P1 |  |
| US-12 | UpdateUser | Edge Case | `user.Username == ""`, `user.Email == nil`, `user.Roles == nil` | | Existing account's `Username`/`Email`/`Roles` are preserved unchanged (only non-zero fields overwrite) | P0 |  |
| US-13 | UpdateUser | Error Path | `as.GetAccountByID` fails | | Returns error; `UpdateAccountInDB` never called | P2 |  |
| US-14 | UpdateUser | Error Path | `as.UpdateAccountInDB` fails | | Error propagated unchanged | P2 |  |
| US-15 | UpdateUserFromPlatform | Happy Path | Brand-new platform identity | `als.GetLinkedAccountByPlatformID` → `ErrNotFound`; `AddAccountToDB`/`AddLinkedAccountToDB`/`UpdateLinkedAccount`/`GetAccountByID` all succeed | Returns the new account, nil; `AddAccountToDB` and `AddLinkedAccountToDB` each called once | P1 |  |
| US-16 | UpdateUserFromPlatform | Happy Path | Existing platform identity | `als.GetLinkedAccountByPlatformID` succeeds immediately | Account creation skipped entirely (`AddAccountToDB` never called); `la.Data` updated via `UpdateLinkedAccount`; resolved account returned | P1 |  |
| US-17 | UpdateUserFromPlatform | Error Path | Initial lookup fails with a non-`ErrNotFound` error | | Error propagated immediately; nothing else called | P2 |  |
| US-18 | UpdateUserFromPlatform | Error Path | `as.AddAccountToDB` fails (new-identity path) | | Error propagated; `als.AddLinkedAccountToDB` never called | P2 |  |
| US-19 | UpdateUserFromPlatform | Error Path | `als.AddLinkedAccountToDB` fails with a generic (non-`ErrAlreadyLinked`) error; cleanup succeeds | `as.DeleteAccountFromDB` succeeds | Returns the original error; `DeleteAccountFromDB` called once with the placeholder's `UserID` | P0 |  |
| US-20 | UpdateUserFromPlatform | Error Path | Same as US-19, but cleanup (`DeleteAccountFromDB`) also fails | | Returns the cleanup error; the link failure is logged | P1 |  |
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
| US-38 | UpdateUserFromPlatform | Error Path | Same as US-20, asserting the cleanup error | `AddLinkedAccountToDB` fails with `testerrors.ErrInsertFailed`; `DeleteAccountFromDB` fails with `testerrors.ErrBoom` | Returns an error matching `testerrors.ErrBoom` (the cleanup failure) | P2 | |
| US-39 | GetAccountSettings | Error Path | `ass.GetAccountSettings` returns `ErrNotFound` | fake settings store returns `ErrNotFound` | Returns an error matching `ErrNotFound` | P2 |  |
| US-40 | SetPasswordAuthEnabled | Error Path | `ass.SetPasswordAuthEnabled` returns `ErrNotFound` | fake settings store returns `ErrNotFound` | Returns an error matching `ErrNotFound` | P2 |  |
| US-41 | GetUserPermissions | Error Path | the role store fails | role store returns `testerrors.ErrBoom` | Returns nil permissions and an error matching the store error | P1 |   |
| US-42 | GetUserPermissions | Edge Case | the account's role grants nothing | role store returns no permissions | Returns empty non-nil permissions, nil error | P1 |  |
| US-43 | ListUsers | Happy Path | a page of users is listed | store returns two accounts | Returns the store's accounts in order and the store received the same limit and offset | P1 |  |
| US-44 | ListUsers | Error Path | the store fails | store returns `testerrors.ErrBoom` | Returns nil accounts and the store's error | P2 |  |
