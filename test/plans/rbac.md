# Test plan: modules/rbac

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ST-01 | CreateRole / GetRole / GetRoleByName | Happy Path | a created role is read back by id and by name | live database; role created through the service | Both reads return the created id, name and description, with an empty non-nil permission list | P1 |  |
| ST-02 | CreateRole | Error Path | a role name is created twice | a role already holds the name | `ErrRoleNameTaken` | P1 |  |
| ST-03 | GetRole / GetRoleByName | Error Path | an unknown id and an unknown name | no such role | `ErrRoleNotFound` for both | P1 |  |
| ST-04 | ListRoles | Happy Path | roles with and without permissions exist | two roles, one with attached permissions | Both listed, each with only its own permissions | P1 |  |
| ST-05 | UpdateRole | Happy Path | name and description are changed | existing role | Reads return the new name and description | P1 |  |
| ST-06 | UpdateRole | Error Path | the new name belongs to another role | two roles | `ErrRoleNameTaken` | P2 |  |
| ST-07 | DeleteRole | Happy Path | a role with grants is deleted | role with an attached permission | The role is gone (`ErrRoleNotFound` on read), its grants are gone and the permission still exists | P1 |  |
| ST-08 | DeleteRole | Error Path | an account holds the role | account whose `role_ids` has the role | `ErrRoleInUse`; the role remains | P1 |  |
| ST-09 | DeleteRole | Error Path | the role does not exist | no such role | `ErrRoleNotFound` | P2 |  |
| ST-10 | UpdateRole | Error Path | the role does not exist | no such role | `ErrRoleNotFound` | P2 |  |
| ST-11 | CreatePermission / GetPermission / GetPermissionByNode | Happy Path | a created permission is read back | live database | Both reads return the created id, node, description, value type and merge rule | P1 |  |
| ST-12 | CreatePermission | Error Path | a node is created twice | permission with the node exists | `ErrPermissionExists` | P1 |  |
| ST-13 | GetPermission / GetPermissionByNode | Error Path | an unknown id and an unknown node | no such permission | `ErrPermissionNotFound` for both | P1 |  |
| ST-14 | ListPermissions | Happy Path | permissions exist | two permissions created | Both listed | P2 |  |
| ST-15 | DeletePermission | Happy Path | an unused permission is deleted | permission with no grants | It is gone (`ErrPermissionNotFound` on read) | P1 |  |
| ST-16 | DeletePermission | Error Path | a role grants the permission | permission attached to a role | `ErrPermissionInUse`; the permission and the grant remain | P1 |  |
| ST-17 | DeletePermission | Error Path | the permission does not exist | no such permission | `ErrPermissionNotFound` | P2 |  |
| ST-18 | AttachPermission | Edge Case | the same grant is attached twice | role and permission exist | Both calls succeed and the role lists the permission once | P1 |  |
| ST-19 | AttachPermission | Error Path | the role or the permission does not exist | one of each missing | `ErrRoleNotFound` for a missing role, `ErrPermissionNotFound` for a missing permission | P1 |  |
| ST-20 | DetachPermission | Happy Path | a grant is removed, then removed again | attached permission | The role no longer lists it and the second call succeeds | P1 |  |
| ST-21 | DetachPermission | Error Path | the role or the permission does not exist | one of each missing | `ErrRoleNotFound` / `ErrPermissionNotFound` | P2 |  |
| ST-22 | GetPermissionsForRoles | Happy Path | several roles share and differ in permissions | roles with overlapping grants | Distinct node strings for all the roles | P1 |  |
| ST-23 | GetPermissionsForRoles | Edge Case | unknown role ids and no ids | no matching roles | An empty non-nil result and a nil error | P2 |  |
| ST-24 | Store | Error Path | the database is unreachable | pool pointing at a closed port; all 14 store methods | Each returns an error that is none of `ErrRoleNotFound`, `ErrPermissionNotFound`, `ErrRoleNameTaken`, `ErrPermissionExists`, `ErrRoleInUse` or `ErrPermissionInUse` | P2 |  |
| ST-25 | permissions table | Error Path | nodes in the wrong format and merge rules that do not fit the value type are inserted directly | live database; upper case, colon, double dot, unknown type, merge without a type | the `permissions_node_format` or `permissions_merge_matches_type` constraint is violated for each | P1 |  |
| ST-26 | GetPermissionsForRoles | Edge Case | a list-valued permission is granted with a value containing a colon | permission `rbtest.pets` of type `string_list` granted `b:c` and `a` | the result is `rbtest.pets:a` and `rbtest.pets:b:c`, and a session holding them matches the node and each value | P1 |  |
| ST-27 | DeleteRole | Concurrency Invariant (live) | an assignment of the role is in flight, uncommitted, when the delete runs | transaction holding a share lock on the role and an account row naming it | The delete waits for the commit and then returns `ErrRoleInUse`; the role remains | P1 |  |
| ST-28 | seed | Happy Path (live) | the built-in roles | database created from `docker/testdb/init.sql` | `system` and `owner` grant every admin node and `ratelimit:1000`, and `bee_admin` grants `beenamegenerator.admin` | P1 |  |
| ST-29 | GetRoleByName | Edge Case (live) | the name given is a role's ID | store called directly | `ErrRoleNotFound` | P1 |  |
| ST-30 | UpdateRole | Error Path (live) | the role does not exist | store called directly | `ErrRoleNotFound` | P2 |  |
| ST-31 | GetRole / ListRoles | Edge Case (live) | permissions attached out of ID order | role with three permissions attached in a different order | With hash joins forced, permissions are returned in ID order; the roles list is in ID order | P2 |  |
| ST-32 | AttachPermission / GetRole | Edge Case (live) | a valued permission is granted twice with different values and a bare one once | role, an `int` permission and a bare permission | the role lists the last value, the `int` type and the `max` merge rule for the valued permission, and no value, type or merge rule for the bare one | P1 |  |
| ST-33 | GetPermissionsForRoles | Happy Path (live) | two roles grant the same valued permissions | an `int` permission with merge min (100 and 1000) and a `string_list` permission with overlapping lists | the lowest int and the sorted union of the lists | P1 |  |
| ST-34 | GetPermissionsForRoles | Edge Case (live) | a string permission with merge first is granted by two roles | hash joins forced; a second `string`/`first` permission granted in the opposite order (low role first); the account lists its roles in either order | the lower role ID's value for both permissions in both orders | P1 |  |
| ST-35 | GetRole | Edge Case (live) | an int value of 2^53 is granted | live database | the role reads the value back as the exact number 9007199254740992 | P2 |  |
| ST-36 | ListPermissions / ListRoles / GetRole / GetPermissionsForRoles | Edge Case (live) | IDs of different digit counts: roles 9 and 10 and a snowflake role, permissions 9 and 10 and a snowflake permission | a `string`/`first` permission granted to roles 10 and 9; hash joins forced; the account lists its roles in either order | the value of role 9 wins in both orders; a role lists its permissions 9 then 10; both lists are in numeric ID order | P1 |  |


## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SV-01 | CreateRole | Happy Path | valid role names | names from one letter to 200 characters, with digits and underscores | Each is created with its name | P1 |  |
| SV-02 | CreateRole | Error Path | invalid role names | empty, upper-case, leading digit or underscore, other characters | `ErrInvalidRoleName` and the store is never called | P1 |  |
| SV-03 | CreateRole | Edge Case | a very long description | fake store | A 10000-character description is accepted and reaches the store unchanged | P2 |  |
| SV-04 | CreateRole | Happy Path | a role is created | fake store | The role id is the snowflake handed to the store and the permission list is empty and non-nil | P1 |  |
| SV-05 | CreatePermission | Error Path | invalid nodes | empty, upper case, leading digit or underscore in a word, empty word, a leading or trailing dot, other characters, a colon, non-ASCII | `ErrInvalidNode` and the store is never called | P1 |  |
| SV-06 | CreatePermission | Happy Path | valid nodes | a single word, dotted words, digits and underscores, a 300-character node | created with the snowflake id and the node and description reaching the store unchanged | P1 |  |
| SV-07 | Service | Error Path | a bad id | empty, non-numeric, zero, negative, fractional, overflowing | `ErrInvalidID` from every method taking an id and the store is never called | P1 |  |
| SV-08 | AttachPermission / DetachPermission | Happy Path | ids are parsed | fake store | The store receives the role id then the permission id | P1 |  |
| SV-09 | Service | Error Path | the store fails | fake store returns an error | Every method returns that error | P1 |  |
| SV-10 | UpdateRole | Edge Case | one field is omitted | stored role with a name and description | The omitted field keeps its value; an empty description clears it | P1 |  |
| SV-12 | CreatePermission | Edge Case | value types and merge rules | valid pairs, defaults for string and string_list, and invalid pairs | valid pairs are stored with the merge rule filled in; invalid pairs give `ErrInvalidValueType` before the store | P1 |  |
| SV-13 | CreatePermission / CreateRole | Error Path | NUL, U+FFFD and invalid UTF-8 in descriptions | role and permission descriptions | `ErrInvalidDescription` before the store; a newline and a joined emoji are accepted | P1 |  |
| SV-14 | GetRoleByName / GetPermissionByNode | Edge Case | names that cannot exist | NUL in the name, upper case, a leading digit, empty, a colon or double dot in a node | `ErrRoleNotFound` or `ErrPermissionNotFound` and the store is never called; valid names reach the store | P1 |  |
| SV-15 | DeleteRole | Error Path | deleting a built-in role | roles `system` and `owner`; and `bee_admin` and an ordinary role | `ErrBuiltinRole` for `system` and `owner` without a store delete; `bee_admin` and the ordinary role are deleted | P1 |  |
| SV-16 | UpdateRole | Error Path | renaming a built-in role | role `owner`, and `bee_admin` | `ErrBuiltinRole` for `owner`; repeating the name while changing the description succeeds; `bee_admin` can be renamed | P1 |  |
| SV-17 | DetachPermission | Error Path | removing `roles.admin` from `system` or `owner` | fake store | `ErrBuiltinRole` without a store detach | P1 |  |
| SV-18 | DetachPermission | Edge Case | other detaches | other nodes from `owner`, `roles` and `roles.administrator` from `owner`, `roles.admin` from other roles | The detach reaches the store | P2 |  |
| SV-19 | AttachPermission | Error Path | a permission without a value type is given a value, or none is given | fake store | no value is stored as NULL; any value gives `ErrInvalidValue` without an attach | P1 |  |
| SV-20 | AttachPermission | Edge Case | int values | permission of type `int`; whole numbers in several Go and JSON number forms, including plus and minus 2^53 in every form; fractions, strings, nil, lists, and numbers beyond plus or minus 2^53 in every form | whole numbers are stored as JSON integers; the others give `ErrInvalidValue` without an attach | P1 |  |
| SV-21 | AttachPermission | Edge Case | string values | permission of type `string`; text with colons; empty, padded, NUL, format, control and non-string values; a 1000-character string | valid text, including a long string, is stored as a JSON string; the others give `ErrInvalidValue` without an attach | P1 |  |
| SV-22 | AttachPermission | Edge Case | list values | permission of type `string_list`; unsorted lists with duplicates; empty, non-string and bad-text lists; a list of 1000 items | lists are stored sorted and deduplicated; the others give `ErrInvalidValue` without an attach | P1 |  |
| SV-23 | AttachPermission | Error Path | the permission does not exist | store returns `ErrPermissionNotFound` | `ErrPermissionNotFound` and no attach | P2 |  |
| SV-11 | UpdateRole | Error Path | an invalid name or description | stored role | `ErrInvalidRoleName` or `ErrInvalidDescription` and no write reaches the store | P1 |  |


## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RH-01 | all handlers | Error Path | a session without the roles.admin node | sessions with no permissions, `roles.other`, `roles.admins`, `other.admin` and bare `roles` | 403 with `msgNoPermission` and the service is never called | P1 |  |
| RH-02 | all handlers | Happy Path | a session with the roles.admin node | stub service | The documented status code, the path and body values reaching the service, and the response body carrying the service's result (empty for 204) | P1 |  |
| RH-03 | body handlers | Error Path | an unparseable body | create and update handlers | 400 with `msgUnableToParseBody` and the service is never called | P1 |  |
| RH-04 | all handlers | Error Path | the service returns each sentinel, a wrapped one and an unknown error | stub service; the sentinels each handler handles are listed per handler | each handled sentinel gives its 400, 404 or 409 with the matching `msg*` detail, a wrapped one the same, and every other sentinel and an unknown error give 500 `msgFailedToHandleRbac` | P1 |  |
| RH-05 | AttachPermissionHandler | Edge Case | int values in the body, with the real service | permission of type int | `1000`, 2^53 and -2^53 are stored as given; a number beyond plus or minus 2^53, `1e2`, `5.0`, `1.5`, a string and a missing value give 400 `msgInvalidValue` and no attach | P1 |  |


## grants.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| GR-01 | flattenGrants | Edge Case | no grants | none | an empty non-nil list | P2 |  |
| GR-02 | flattenGrants | Happy Path | bare grants of the same node from several roles | duplicate and distinct nodes | each node once, in the order given | P1 |  |
| GR-03 | flattenGrants | Happy Path | int values with merge max and min | several values including negatives | the highest for max and the lowest for min | P1 |  |
| GR-04 | flattenGrants | Happy Path | string values | two roles | the first value in role order | P1 |  |
| GR-05 | flattenGrants | Happy Path | list values | overlapping lists | one `node:value` entry per distinct element, sorted | P1 |  |
| GR-06 | flattenGrants | Edge Case | values containing colons | list element `a:b` and string `x:y:z` | the value is kept whole after the first colon | P1 |  |
| GR-07 | flattenGrants | Edge Case | valued nodes without a value | nil value, empty list, unknown type, and a nil value beside a real one | a bare node, except the real value wins | P2 |  |
| GR-08 | flattenGrants | Error Path | stored values of the wrong shape | text for an int, a fraction for an int, a number for a string, a string or broken JSON for a list | an error and no result | P2 |  |

## empty tables

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| EM-01 | ListRoles / ListPermissions | Edge Case | no rows exist | scratch schema with empty copies of the three tables | Both return empty non-nil slices and nil errors | P2 |  |
