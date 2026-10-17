# Test plan: modules/auth (core package)

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ST-01 | NewStore | Happy Path | Given a db pool and redis client | | Returns a non-nil `Store` wrapping both | P2 |  |
| ST-02 | Account | Accessor | | | Returns a non-nil `AccountStore` | P3 |  |
| ST-03 | AccountSettings | Accessor | | | Returns a non-nil `AccountSettingsStore` | P3 |  |
| ST-04 | Session | Accessor | | | Returns a non-nil `SessionStore` | P3 |  |
| ST-05 | LinkAccount | Accessor | | | Returns a non-nil `LinkAccountStore` | P3 |  |
| ST-07 | OAuthToken | Accessor | | | Returns a non-nil `OAuthTokenStore` | P3 |  |
| ST-69 | SetPasswordAuthEnabled | Happy Path | `enabled=true`, account has `hashed_secret` | | Returns nil; settings read back `PasswordAuthEnabled=true` | P1 |  |
| ST-71 | SetPasswordAuthEnabled | Happy Path | `enabled=false`, account has a password and one verified+enabled link | Guard allows | Returns nil; settings read back `PasswordAuthEnabled=false` | P1 |  |
| ST-72 | SetPasswordAuthEnabled | Concurrency Invariant | `SetPasswordAuthEnabled(false)` races `DeleteLinkedAccount` (sole link) on the same account | Same lock, many trials | Exactly one succeeds every trial; account never ends up with neither a password nor a link (`ErrWouldLockAccount`) | P0 |  |
| ST-73 | SetPasswordAuthEnabled | Concurrency Invariant | `SetPasswordAuthEnabled(false)` races `SetLinkedAccountLoginEnabled` (sole link, false) on the same account | Same lock, many trials | Exactly one succeeds every trial; account never ends up with neither (`ErrWouldLockAccount`) | P0 |  |

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
| US-24 | UpdateUserFromPlatform | Error Path | Final `as.GetAccountByID` fails | | Error propagated | P2 |  |
| US-83 | UpdateUserFromPlatform | Happy Path | The linking service returns a user ID | `UserService` built with a stub `linking.Service` returning `"resolved"`; `as.GetAccountByID` returns an account | Returns that account, nil; `GetAccountByID` called with `"resolved"` | P1 |  |
| US-84 | UpdateUserFromPlatform | Error Path | The linking service fails | stub `linking.Service` returns `testerrors.ErrBoom` | Returns nil and the linking error; `GetAccountByID` never called | P1 |  |
| US-26 | DeleteUser | Happy Path | `as.DeleteAccountFromDB` succeeds | | Returns nil | P1 |  |
| US-27 | DeleteUser | Error Path | `as.DeleteAccountFromDB` fails | | Error propagated unchanged | P2 |  |
| US-34 | GetAccountSettings | Happy Path | `ass.GetAccountSettings` succeeds | | Returns the settings, nil | P1 |  |
| US-35 | GetAccountSettings | Error Path | `ass.GetAccountSettings` fails | | Error propagated unchanged | P2 |  |
| US-36 | SetPasswordAuthEnabled | Happy Path | `ass.SetPasswordAuthEnabled` succeeds | | Returns nil | P1 |  |
| US-37 | SetPasswordAuthEnabled | Error Path | `ass.SetPasswordAuthEnabled` fails | | Error propagated unchanged | P2 |  |
| US-39 | GetAccountSettings | Error Path | `ass.GetAccountSettings` returns `ErrNotFound` | fake settings store returns `ErrNotFound` | Returns an error matching `ErrNotFound` | P2 |  |
| US-40 | SetPasswordAuthEnabled | Error Path | `ass.SetPasswordAuthEnabled` returns `ErrNotFound` | fake settings store returns `ErrNotFound` | Returns an error matching `ErrNotFound` | P2 |  |
| US-41 | GetUserPermissions | Error Path | the role store fails | role store returns `testerrors.ErrBoom` | Returns nil permissions and an error matching the store error | P1 |   |
| US-42 | GetUserPermissions | Edge Case | the account's role grants nothing | role store returns no permissions | Returns empty non-nil permissions, nil error | P1 |  |
| US-43 | ListUsers | Happy Path | a page of users is listed | store returns two accounts | Returns the store's accounts in order and the store received the same limit and offset | P1 |  |
| US-44 | ListUsers | Error Path | the store fails | store returns `testerrors.ErrBoom` | Returns nil accounts and the store's error | P2 |  |
