# Test plan: modules/auth/permissions

## rbac.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RB-01 | ScopePetPictures | Happy Path | Called with a normal, non-empty value | `value = "*"` | Returns `Scope{Name:"petpictures", Description:"Pet pictures", Value:"*"}` | P2 |  |
| RB-02 | ScopePetPictures | Edge Case | Called with an empty string value | `value = ""` | Returns `Scope{Name:"petpictures", Description:"Pet pictures", Value:""}` (no validation, passed through as-is) | P3 |  |
| RB-03 | ScopeDataStore | Happy Path | Called with a normal, non-empty value | `value = "read"` | Returns `Scope{Name:"datastore", Description:"Data store", Value:"read"}` | P2 |  |
| RB-04 | ScopeDataStore | Edge Case | Called with an empty string value | `value = ""` | Returns `Scope{Name:"datastore", Description:"Data store", Value:""}` | P3 |  |
| RB-05 | ScopeNumberStore | Happy Path | Called with a normal, non-empty value | `value = "write"` | Returns `Scope{Name:"numberstore", Description:"Number store", Value:"write"}` | P2 |  |
| RB-06 | ScopeNumberStore | Edge Case | Called with an empty string value | `value = ""` | Returns `Scope{Name:"numberstore", Description:"Number store", Value:""}` | P3 |  |
| RB-07 | ScopeUsers | Happy Path | Called with a normal, non-empty value | `value = "read"` | Returns `Scope{Name:"users", Description:"Users", Value:"read"}` | P2 |  |
| RB-08 | ScopeUsers | Edge Case | Called with an empty string value | `value = ""` | Returns `Scope{Name:"users", Description:"Users", Value:""}` | P3 |  |
| RB-09 | GetRoleByName | Happy Path | Name matches the system role | `name = "system"` | Returns `(RoleSystem, nil)` (exact package var, including its full `Permissions` slice) | P1 |  |
| RB-10 | GetRoleByName | Happy Path | Name matches the owner role | `name = "owner"` | Returns `(RoleOwner, nil)` (exact package var, including its full `Permissions` slice) | P1 |  |
| RB-11 | GetRoleByName | Error Path | Name matches no known role | `name = "unknown-role"` | Returns `(Role{}, error)` with message `"role not found"` | P1 |  |
| RB-12 | GetRoleByName | Edge Case | Empty string name | `name = ""` | Returns `(Role{}, error)` with message `"role not found"` (falls to the default switch case, same as any unrecognized name) | P3 |  |
| RB-13 | GetRoleByName | Edge Case | Name differs from a known role only in case | `name = "System"` | Returns `(Role{}, error)` with message `"role not found"` (switch match is exact/case-sensitive) | P3 |  |

## Function inventory self-check
- [x] ScopePetPictures — covered by RB-01, RB-02
- [x] ScopeDataStore — covered by RB-03, RB-04
- [x] ScopeNumberStore — covered by RB-05, RB-06
- [x] ScopeUsers — covered by RB-07, RB-08
- [x] GetRoleByName — covered by RB-09, RB-10, RB-11, RB-12, RB-13
