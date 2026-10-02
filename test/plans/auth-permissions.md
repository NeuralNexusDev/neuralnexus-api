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
| RB-09 | ScopeRoles | Happy Path | Called with a normal, non-empty value | `value = "read"` | Returns `Scope{Name:"roles", Description:"Roles", Value:"read"}` | P2 |  |
| RB-10 | ScopeRoles | Edge Case | Called with an empty string value | `value = ""` | Returns `Scope{Name:"roles", Description:"Roles", Value:""}` | P3 |  |
| RB-11 | ScopeAdminRoles | Happy Path | The package variable is read | | Equals `Scope{Name:"roles", Description:"Roles", Value:"*"}` | P1 |  |
