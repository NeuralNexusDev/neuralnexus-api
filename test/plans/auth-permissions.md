# Test plan: modules/auth/permissions

## rbac.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RB-01 | ScopeAdminBeeNameGenerator | Happy Path | The package variable is read | | Equals `Scope{Node:"beenamegenerator.admin", Description:"Bee name generator"}` | P1 |  |
| RB-02 | ScopeAdminPetPictures | Happy Path | The package variable is read | | Equals `Scope{Node:"petpictures.admin", Description:"Pet pictures"}` | P1 |  |
| RB-03 | ScopeAdminDataStore | Happy Path | The package variable is read | | Equals `Scope{Node:"datastore.admin", Description:"Data store"}` | P1 |  |
| RB-04 | ScopeAdminNumberStore | Happy Path | The package variable is read | | Equals `Scope{Node:"numberstore.admin", Description:"Number store"}` | P1 |  |
| RB-05 | ScopeAdminUsers | Happy Path | The package variable is read | | Equals `Scope{Node:"users.admin", Description:"Users"}` | P1 |  |
| RB-06 | ScopeAdminRoles | Happy Path | The package variable is read | | Equals `Scope{Node:"roles.admin", Description:"Roles and permissions"}` | P1 |  |
| RB-07 | ScopePetPictures | Happy Path | The package variable is read | | Equals `Scope{Node:"petpictures.pets", Description:"Pet pictures"}` | P1 |  |
| RB-08 | ScopeRateLimit | Happy Path | The package variable is read | | Equals `Scope{Node:"ratelimit", Description:"Rate limit"}` | P1 |  |
