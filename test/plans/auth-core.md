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
