# Test plan: modules/rbac

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ST-01 | CreateRole / GetRole / GetRoleByName | Happy Path | a created role is read back by id and by name | live database; role created through the service | Both reads return the created id, name and description, with an empty non-nil permission list | P1 |  |
| ST-02 | CreateRole | Error Path | a role name is created twice | a role already holds the name | `ErrRoleNameTaken` | P1 |  |
| ST-03 | GetRole / GetRoleByName | Error Path | an unknown id and an unknown name | no such role | `ErrRoleNotFound` for both | P1 |  |
| ST-04 | ListRoles | Happy Path | roles with and without permissions exist | two roles, one with attached permissions | Both listed; the permissions of each are its own scopes | P1 |  |
| ST-05 | UpdateRole | Happy Path | name and description are changed | existing role | Reads return the new name and description | P1 |  |
| ST-06 | UpdateRole | Error Path | the new name belongs to another role | two roles | `ErrRoleNameTaken` | P2 |  |
| ST-07 | DeleteRole | Happy Path | a role with grants is deleted | role with an attached permission | The role is gone (`ErrRoleNotFound` on read), its grants are gone and the permission still exists | P1 |  |
| ST-08 | DeleteRole | Error Path | an account holds the role | account whose `role_ids` has the role | `ErrRoleInUse`; the role remains | P1 |  |
| ST-09 | DeleteRole | Error Path | the role does not exist | no such role | `ErrRoleNotFound` | P2 |  |
| ST-10 | UpdateRole | Error Path | the role does not exist | no such role | `ErrRoleNotFound` | P2 |  |
| ST-11 | CreatePermission / GetPermission / GetPermissionByScope | Happy Path | a created permission is read back | live database | Both reads return the created id and scope | P1 |  |
| ST-12 | CreatePermission | Error Path | a scope is created twice | permission with the scope exists | `ErrPermissionExists` | P1 |  |
| ST-13 | GetPermission / GetPermissionByScope | Error Path | an unknown id and an unknown scope | no such permission | `ErrPermissionNotFound` for both | P1 |  |
| ST-14 | ListPermissions | Happy Path | permissions exist | two permissions created | Both listed | P2 |  |
| ST-15 | DeletePermission | Happy Path | an unused permission is deleted | permission with no grants | It is gone (`ErrPermissionNotFound` on read) | P1 |  |
| ST-16 | DeletePermission | Error Path | a role grants the permission | permission attached to a role | `ErrPermissionInUse`; the permission and the grant remain | P1 |  |
| ST-17 | DeletePermission | Error Path | the permission does not exist | no such permission | `ErrPermissionNotFound` | P2 |  |
| ST-18 | AttachPermission | Edge Case | the same grant is attached twice | role and permission exist | Both calls succeed and the role lists the permission once | P1 |  |
| ST-19 | AttachPermission | Error Path | the role or the permission does not exist | one of each missing | `ErrRoleNotFound` for a missing role, `ErrPermissionNotFound` for a missing permission | P1 |  |
| ST-20 | DetachPermission | Happy Path | a grant is removed, then removed again | attached permission | The role no longer lists it and the second call succeeds | P1 |  |
| ST-21 | DetachPermission | Error Path | the role or the permission does not exist | one of each missing | `ErrRoleNotFound` / `ErrPermissionNotFound` | P2 |  |
| ST-22 | GetPermissionsForRoles | Happy Path | several roles share and differ in permissions | roles with overlapping grants | Distinct `name:value` strings for all roles | P1 |  |
| ST-23 | GetPermissionsForRoles | Edge Case | unknown role ids and no ids | no matching roles | An empty non-nil result and a nil error | P2 |  |
| ST-24 | Store | Error Path | the database is unreachable | pool pointing at a closed port; all 14 store methods | Each returns an error that is none of `ErrRoleNotFound`, `ErrPermissionNotFound`, `ErrRoleNameTaken`, `ErrPermissionExists`, `ErrRoleInUse` or `ErrPermissionInUse` | P2 |  |
| ST-25 | permissions table | Error Path | a scope name containing a colon is inserted directly | live database | the `permissions_scope_name_no_colon` constraint is violated | P1 |  |
| ST-26 | GetPermissionsForRoles | Edge Case | a scope value contains a colon | permission `rbtest` / `a:b` attached | the result is `rbtest:a:b` and a session holding it matches the scope | P1 |  |
| ST-27 | DeleteRole | Concurrency Invariant (live) | an assignment of the role is in flight, uncommitted, when the delete runs | transaction holding a share lock on the role and an account row naming it | The delete waits for the commit and then returns `ErrRoleInUse`; the role remains | P1 |  |
| ST-28 | seed | Happy Path (live) | the built-in roles | database created from `docker/testdb/init.sql` | `system` and `owner` grant all seven scopes and `bee_admin` grants the bee name generator scope | P1 |  |
| ST-29 | GetRoleByName | Edge Case (live) | the name given is a role's ID | store called directly | `ErrRoleNotFound` | P1 |  |
| ST-30 | UpdateRole | Error Path (live) | the role does not exist | store called directly | `ErrRoleNotFound` | P2 |  |
| ST-31 | GetRole / ListRoles | Edge Case (live) | permissions attached out of ID order | role with three permissions attached in a different order | With hash joins forced, permissions are returned in ID order; the roles list is in ID order | P2 |  |


## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SV-01 | CreateRole | Happy Path | valid role names | names from one letter to the length limit, with digits and underscores | Each is created with its name | P1 |  |
| SV-02 | CreateRole | Error Path | invalid role names | empty, upper-case, leading digit or underscore, other characters, over the length limit | `ErrInvalidRoleName` and the store is never called | P1 |  |
| SV-03 | CreateRole | Edge Case | description at and over the limit | fake store | At the limit succeeds; over it is `ErrInvalidDescription` without a store call | P2 |  |
| SV-04 | CreateRole | Happy Path | a role is created | fake store | The role id is the snowflake handed to the store and the permission list is empty and non-nil | P1 |  |
| SV-05 | CreatePermission | Error Path | invalid scopes | empty name or value, a colon in the name, over either limit | `ErrInvalidScope` and the store is never called | P1 |  |
| SV-06 | CreatePermission | Edge Case | scope at both limits | fake store | Created with the snowflake id given to the store and the scope reaching the store unchanged | P2 |  |
| SV-07 | Service | Error Path | a bad id | empty, non-numeric, zero, negative, fractional, overflowing | `ErrInvalidID` from every method taking an id and the store is never called | P1 |  |
| SV-08 | AttachPermission / DetachPermission | Happy Path | ids are parsed | fake store | The store receives the role id then the permission id | P1 |  |
| SV-09 | Service | Error Path | the store fails | fake store returns an error | Every method returns that error | P1 |  |
| SV-10 | UpdateRole | Edge Case | one field is omitted | stored role with a name and description | The omitted field keeps its value; an empty description clears it | P1 |  |
| SV-12 | CreatePermission | Edge Case | a scope value contains a colon | fake store | Created with the value unchanged at the store | P2 |  |
| SV-13 | CreatePermission / CreateRole | Error Path | NUL, invalid UTF-8, control characters, surrounding spaces, and limits counted in characters | scopes and descriptions of multi-byte characters | The bad inputs give `ErrInvalidScope` or `ErrInvalidDescription` before the store; inputs of exactly the limit in characters are created | P1 |  |
| SV-14 | GetRoleByName / GetPermissionByScope | Edge Case | names that cannot exist | NUL in the name, upper case, a leading digit, empty, a colon in a scope name | `ErrRoleNotFound` or `ErrPermissionNotFound` and the store is never called | P1 |  |
| SV-15 | DeleteRole | Error Path | deleting a built-in role | roles `system`, `owner` and `bee_admin`; and an ordinary role | `ErrBuiltinRole` for the built-ins without a store delete; the ordinary role is deleted | P1 |  |
| SV-16 | UpdateRole | Error Path | renaming a built-in role | role `owner` | `ErrBuiltinRole`; repeating the name while changing the description succeeds | P1 |  |
| SV-17 | DetachPermission | Error Path | removing `roles:*` from `system` or `owner` | fake store | `ErrBuiltinRole` without a store detach | P1 |  |
| SV-18 | DetachPermission | Edge Case | other detaches | other scope from `owner`, `roles:read` from `owner`, `roles:*` from other roles | The detach reaches the store | P2 |  |
| SV-11 | UpdateRole | Error Path | an invalid name or description | stored role | `ErrInvalidRoleName` or `ErrInvalidDescription` and no write reaches the store | P1 |  |


## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RH-01 | all handlers | Error Path | a session without the roles scope | sessions with no permissions, `roles` with another value, another scope | 403 with `msgNoPermission` and the service is never called | P1 |  |
| RH-02 | all handlers | Happy Path | a session with the roles scope | stub service | The documented status code, the path and body values reaching the service, and the response body carrying the service's result (empty for 204) | P1 |  |
| RH-03 | body handlers | Error Path | an unparseable body | create and update handlers | 400 with `msgUnableToParseBody` and the service is never called | P1 |  |
| RH-04 | all handlers | Error Path | the service returns each sentinel, a wrapped one and an unknown error | stub service | 400, 404 and 409 with the matching `msg*` detail (`msgBuiltinRole` for `ErrBuiltinRole`); an unknown error is 500 with `msgFailedToHandleRbac` | P1 |  |


## docker/testdb/init.sql

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SC-01 | init.sql | Edge Case | the test database init is compared with the schema file | both files | `docker/testdb/init.sql` contains `docker/rbac.sql` verbatim | P2 |  |


## docker/rbac_migration.sql

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| MG-01 | rbac_migration.sql | Happy Path | accounts hold role names | accounts with known, unknown and no role names | `role_ids` holds the ids of the known roles only | P1 |  |
| MG-02 | rbac_migration.sql | Edge Case | the migration finishes | accounts and sessions | the old `roles` column is gone and the sessions table is empty | P1 |  |
| MG-03 | docker/rbac.sql | Happy Path | built-in roles are seeded by the schema file | fresh schema | `system` and `owner` grant all seven scopes and `bee_admin` grants the bee name generator scope | P1 |  |

## empty tables

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| EM-01 | ListRoles / ListPermissions | Edge Case | no rows exist | scratch schema from `docker/rbac.sql` with every table emptied | Both return empty non-nil slices and nil errors | P2 |  |
