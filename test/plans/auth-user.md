# Test plan: modules/auth/user

## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| US-85 | userFromAccount | Happy Path | Getting a user whose account has every field set: email, hashed secret, salt, roles and updated_at | Loading the account returns that account | Returns a user whose JSON equals the account's JSON | P0 |  |

## service

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| US-03 | GetUser | Error Path | Loading the account fails | | Error propagated unchanged | P2 |  |
| US-04 | GetUserFromPlatform | Happy Path | Looking up the linked account and then loading the account both succeed | | Returns the resolved user, whose JSON equals the account's JSON, and no error | P1 |  |
| US-05 | GetUserFromPlatform | Error Path | Looking up the linked account fails (for example with `ErrNotFound`) | | Returns no user and that error; the account is never loaded | P2 |  |
| US-06 | GetUserFromPlatform | Error Path | Looking up the linked account succeeds but loading the account fails | | Error propagated unchanged | P2 |  |
| US-07 | GetUserPermissions | Happy Path | Loading the account returns an account holding one role id | The role store holds permissions for that role | Returns the permissions of that role | P1 |  |
| US-08 | GetUserPermissions | Error Path | Loading the account fails | | Returns nil, error | P2 |  |
| US-09 | GetUserPermissions | Edge Case | The account holds two role ids | The role store holds permissions for both roles | The role store is asked once for both roles and the permissions of both are returned | P1 |  |
| US-10 | GetUserPermissions | Edge Case | The account holds no role ids | | Returns empty non-nil permissions, nil error, without a role lookup | P2 |  |
| US-11 | UpdateUser | Happy Path | Loading the account succeeds, and the update carries a new username and new roles | | Updates the account once with the new username and the new roles | P1 |  |
| US-12 | UpdateUser | Edge Case | The update is empty: no username and no roles | | The existing username, email and roles are kept (only fields that are set overwrite) | P0 |  |
| US-13 | UpdateUser | Error Path | Loading the account fails | | Returns the error; the account is never updated | P2 |  |
| US-14 | UpdateUser | Error Path | Updating the account fails | | Error propagated unchanged | P2 |  |
| US-24 | UpdateUserFromPlatform | Error Path | Loading the account at the end fails | | Error propagated | P2 |  |
| US-26 | DeleteUser | Happy Path | Deleting the account succeeds | | Returns no error; the account is deleted once | P1 |  |
| US-27 | DeleteUser | Error Path | Deleting the account fails | | Error propagated unchanged | P2 |  |
| US-34 | GetAccountSettings | Happy Path | Reading the account settings succeeds | | Returns the settings and no error | P1 |  |
| US-35 | GetAccountSettings | Error Path | Reading the account settings fails | | Error propagated unchanged | P2 |  |
| US-36 | SetPasswordAuthEnabled | Happy Path | Switching password login succeeds | | Returns no error; the switch is made once | P1 |  |
| US-37 | SetPasswordAuthEnabled | Error Path | Switching password login fails | | Error propagated unchanged | P2 |  |
| US-41 | GetUserPermissions | Error Path | the role store fails | The role store fails | Returns no permissions and the role store's error | P1 |   |
| US-42 | GetUserPermissions | Edge Case | the account's role grants nothing | role store returns no permissions | Returns empty non-nil permissions, nil error | P1 |  |
| US-43 | ListUsers | Happy Path | a page of users is listed | store returns two accounts | Returns users in store order whose JSON equals the accounts' JSON, and the store received the same limit and offset | P1 |  |
| US-44 | ListUsers | Error Path | the store fails | The store fails | Returns no accounts and the store's error | P2 |  |
| US-83 | UpdateUserFromPlatform | Happy Path | The link service returns a user ID | The link service resolves the platform identity to user id "resolved"; loading the account returns an account | Returns the user whose JSON equals the account's JSON, and no error; the account loaded is the one for "resolved" | P1 |  |
| US-84 | UpdateUserFromPlatform | Error Path | The link service fails | The link service fails | Returns no user and the link service's error; the account is never loaded | P1 |  |

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| UR-01 | GetUserHandler | Happy Path | Self-lookup | session.UserID == path user_id | 200 OK with the user struct | P1 |  |
| UR-02 | GetUserHandler | Happy Path | Admin looks up another user | session.UserID != path user_id, session has ScopeAdminUsers | 200 OK with the user struct | P1 |  |
| UR-03 | GetUserHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden `msgNoPermissionToGetUser` | P2 |  |
| UR-04 | GetUserHandler | Error Path | Service.GetUser fails with a non-not-found error | Permission check passes, the service fails with a database error | 500 Internal Server Error `msgFailedToGetUser` | P2 |  |
| UR-05 | GetUserFromPlatformHandler | Happy Path | Admin fetches a user by platform ID | session has ScopeAdminUsers, service.GetUserFromPlatform succeeds | 200 OK with the user struct | P1 |  |
| UR-06 | GetUserFromPlatformHandler | Error Path | No permission | session lacks ScopeAdminUsers | 403 Forbidden `msgNoPermissionToGetUsers` | P2 |  |
| UR-07 | GetUserFromPlatformHandler | Error Path | Service.GetUserFromPlatform errors | Admin session, service returns `account.ErrNotFound` | 404 Not Found `msgUserNotFound` | P2 |  |
| UR-08 | GetUserPermissionsHandler | Happy Path | Self-lookup | session.UserID == path user_id | 200 OK with the permissions list | P1 |  |
| UR-09 | GetUserPermissionsHandler | Happy Path | Admin looks up another user's permissions | session.UserID != path user_id, session has ScopeAdminUsers | 200 OK with the permissions list | P2 |  |
| UR-10 | GetUserPermissionsHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden `msgNoPermissionToGetUserPermissions` | P2 |  |
| UR-11 | GetUserPermissionsHandler | Error Path | Service.GetUserPermissions fails with a non-not-found error | Permission check passes, the service fails with a database error | 500 Internal Server Error `msgFailedToGetUser` | P2 |  |
| UR-12 | UpdateUserHandler | Happy Path | Admin updates a user | session has ScopeAdminUsers, valid JSON body, service.UpdateUser succeeds | 200 OK with the account the service reads back; the update is applied to the path user_id | P1 |  |
| UR-13 | UpdateUserHandler | Error Path | No permission | session lacks ScopeAdminUsers | 403 Forbidden `msgNoPermissionToUpdateUsers` | P2 |  |
| UR-14 | UpdateUserHandler | Error Path | Malformed body | Admin session, body is not valid JSON | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| UR-15 | UpdateUserHandler | Error Path | Service.UpdateUser fails with an unclassified error | Admin session, valid body, the service fails with a database error | 500 Internal Server Error `msgFailedToUpdateUser` | P2 |  |
| UR-16 | UpdateUserFromPlatformHandler | Happy Path | platform=discord, valid body | Admin session, body decodes into link.DiscordData, service succeeds | 200 OK with the user struct | P1 |  |
| UR-17 | UpdateUserFromPlatformHandler | Happy Path | platform=minecraft, valid body | Admin session, body decodes into link.MinecraftData, service succeeds | 200 OK with the user struct | P2 |  |
| UR-18 | UpdateUserFromPlatformHandler | Happy Path | platform=twitch, valid body | Admin session, body decodes into twitch.Data, service succeeds | 200 OK with the user struct | P2 |  |
| UR-19 | UpdateUserFromPlatformHandler | Happy Path | platform=xboxlive, valid body | Admin session, body decodes into link.XboxLiveData, service succeeds | 200 OK with the user struct | P2 |  |
| UR-20 | UpdateUserFromPlatformHandler | Happy Path | platform=microsoft, valid body | Admin session, body decodes into link.MicrosoftUserData, service succeeds | 200 OK with the user struct | P2 |  |
| UR-21 | UpdateUserFromPlatformHandler | Error Path | Unsupported platform value | platform path value matches none of the five known platforms | 400 Bad Request `msgUnsupportedPlatform` | P2 |  |
| UR-22 | UpdateUserFromPlatformHandler | Error Path | No permission | session lacks ScopeAdminUsers | 403 Forbidden `msgNoPermissionToUpdateUsers` | P2 |  |
| UR-23 | UpdateUserFromPlatformHandler | Error Path | Malformed body for the given platform | Admin session, platform=discord, body is not valid JSON | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| UR-24 | UpdateUserFromPlatformHandler | Error Path | Service.UpdateUserFromPlatform fails with an unclassified error | Admin session, valid body, the service fails with a database error | 500 Internal Server Error `msgFailedToUpdateUser` | P2 |  |
| UR-25 | DeleteUserHandler | Happy Path | Admin deletes a user | session has ScopeAdminUsers, service.DeleteUser succeeds | 204 No Content | P1 |  |
| UR-26 | DeleteUserHandler | Error Path | No permission | session lacks ScopeAdminUsers | 403 Forbidden `msgNoPermissionToDeleteUsers` | P2 |  |
| UR-27 | DeleteUserHandler | Error Path | Service.DeleteUser fails with an unclassified error | Admin session, the service fails with a database error | 500 Internal Server Error `msgFailedToDeleteUser` | P2 |  |
| UR-45 | GetAccountSettingsHandler | Happy Path | Self-lookup | session.UserID == path user_id | 200 OK with the account settings | P1 |  |
| UR-46 | GetAccountSettingsHandler | Happy Path | Admin looks up another user's settings | session.UserID != path user_id, session has ScopeAdminUsers | 200 OK with the account settings | P2 |  |
| UR-47 | GetAccountSettingsHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden `msgNoPermissionToViewSettings` | P2 |  |
| UR-48 | GetAccountSettingsHandler | Error Path | Service.GetAccountSettings errors | Permission check passes, service returns an error | 500 Internal Server Error `msgFailedToGetAccountSettings` | P2 |  |
| UR-49 | UpdateAccountSettingsHandler | Happy Path | Self updates settings | session.UserID == path user_id, body {"password_auth":true}, service succeeds | 204 No Content | P1 |  |
| UR-50 | UpdateAccountSettingsHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden `msgNoPermissionToUpdateSettings` | P2 |  |
| UR-51 | UpdateAccountSettingsHandler | Error Path | Malformed body | Permission check passes, body is not valid JSON | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| UR-52 | UpdateAccountSettingsHandler | Edge Case | password_auth field omitted | Body decodes with no error but PasswordAuthEnabled == nil | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| UR-53 | UpdateAccountSettingsHandler | Error Path | Would lock the account | service.SetPasswordAuthEnabled returns account.ErrWouldLockAccount | 400 Bad Request `msgEnableLoginMethodBeforeDisablingPassword` | P2 |  |
| UR-54 | UpdateAccountSettingsHandler | Error Path | No password set | service.SetPasswordAuthEnabled returns account.ErrNoPasswordSet | 400 Bad Request `msgSetPasswordBeforeEnablingPasswordLogin` | P2 |  |
| UR-55 | UpdateAccountSettingsHandler | Error Path | Unclassified service error | service.SetPasswordAuthEnabled returns some other error | 500 Internal Server Error `msgFailedToUpdateUserSettings` | P2 |  |
| UR-56 | UpdateUserFromPlatformHandler | Error Path | Malformed body for the Minecraft, Twitch, Xbox Live and Microsoft platforms | Admin session, body is not valid JSON | 400 Bad Request `msgInvalidRequestBody`; the service is never called | P2 | |
| UR-57 | GetUserFromPlatformHandler | Error Path | Service.GetUserFromPlatform fails with a non-not-found error | Admin session, the service fails with a database error | 500 Internal Server Error `msgFailedToGetUser` | P2 |  |
| UR-58 | GetUserHandler | Error Path | Service.GetUser returns not found | Permission check passes, service returns `account.ErrNotFound` | 404 Not Found `msgUserNotFound` | P2 |  |
| UR-59 | GetUserPermissionsHandler | Error Path | Service.GetUserPermissions returns not found | Permission check passes, service returns `account.ErrNotFound` | 404 Not Found `msgUserNotFound` | P2 |  |
| UR-60 | UpdateUserHandler | Error Path | Service.UpdateUser returns not found | Admin session, valid body, service returns `account.ErrNotFound` | 404 Not Found `msgUserNotFound` | P2 |  |
| UR-61 | UpdateUserHandler | Error Path | Service.UpdateUser returns a duplicate email | Admin session, valid body, service returns `account.ErrEmailAlreadyExists` | 409 Conflict `msgEmailAlreadyExists` | P2 |  |
| UR-62 | UpdateUserHandler | Error Path | Service.UpdateUser returns a duplicate username | Admin session, valid body, service returns `account.ErrUsernameAlreadyExists` | 409 Conflict `msgUsernameAlreadyExists` | P2 |  |
| UR-63 | UpdateUserFromPlatformHandler | Error Path | Service.UpdateUserFromPlatform returns not found | Admin session, valid body, service returns `account.ErrNotFound` | 404 Not Found `msgUserNotFound` | P2 |  |
| UR-64 | UpdateUserFromPlatformHandler | Error Path | Service.UpdateUserFromPlatform returns a duplicate email | Admin session, valid body, service returns `account.ErrEmailAlreadyExists` | 409 Conflict `msgEmailAlreadyExists` | P2 |  |
| UR-65 | UpdateUserFromPlatformHandler | Error Path | Service.UpdateUserFromPlatform returns a duplicate username | Admin session, valid body, service returns `account.ErrUsernameAlreadyExists` | 409 Conflict `msgUsernameAlreadyExists` | P2 |  |
| UR-66 | GetAccountSettingsHandler | Error Path | Service.GetAccountSettings returns not found | Permission check passes, service returns `account.ErrNotFound` | 404 Not Found `msgUserNotFound` | P2 |  |
| UR-67 | UpdateAccountSettingsHandler | Error Path | Service.SetPasswordAuthEnabled returns not found | Permission check passes, valid body, service returns `account.ErrNotFound` | 404 Not Found `msgUserNotFound` | P2 |  |
| UR-68 | UpdateUserHandler | Error Path | Service.UpdateUser returns an invalid role id | Admin session, valid body, service returns `account.ErrInvalidRoleID` | 400 Bad Request `msgInvalidRoleID` | P1 |  |
| UR-69 | UpdateUserHandler | Error Path | Service.UpdateUser returns an unknown role id | Admin session, valid body, service returns `account.ErrUnknownRoleID` | 400 Bad Request `msgUnknownRoleID` | P1 |  |
| UR-70 | UpdateUserFromPlatformHandler | Error Path | Service.UpdateUserFromPlatform returns an invalid role id | Admin session, valid body, service returns `account.ErrInvalidRoleID` | 400 Bad Request `msgInvalidRoleID` | P2 |  |
| UR-71 | UpdateUserFromPlatformHandler | Error Path | Service.UpdateUserFromPlatform returns an unknown role id | Admin session, valid body, service returns `account.ErrUnknownRoleID` | 400 Bad Request `msgUnknownRoleID` | P2 |  |
| UR-73 | UpdateUserHandler | Error Path | reading the account back fails after the update | Admin session, the update succeeds, reading the user back fails with a database error | 500 `msgFailedToUpdateUser` after the update was made | P2 |  |
| UR-74 | ListUsersHandler | Happy Path | list users with no query | Admin session; the user list holds two users | 200 OK with a JSON array of the two users in order, each carrying `user_id` and `username`; the default page is the first 50 users, from offset 0 | P1 |  |
| UR-75 | ListUsersHandler | Happy Path | limit and offset in the query | Admin session, limit 200 and offset 40 | 200 OK and the service got limit 200 and offset 40 | P1 |  |
| UR-76 | ListUsersHandler | Error Path | no permission | Session without ScopeAdminUsers | 403 Forbidden `msgNoPermissionToGetUsers` and the service is not called | P1 |  |
| UR-77 | ListUsersHandler | Error Path | invalid limit | Admin session, limit 0, -1, 201, a word, a decimal or a space | 400 Bad Request `msgInvalidLimit` and the service is not called | P2 |  |
| UR-78 | ListUsersHandler | Error Path | invalid offset | Admin session, offset -1, a word, a decimal or a space | 400 Bad Request `msgInvalidOffset` and the service is not called | P2 |  |
| UR-79 | ListUsersHandler | Edge Case | the page is empty | Admin session, the service returns no users | 200 OK with an empty JSON array | P2 |  |
| UR-80 | ListUsersHandler | Error Path | the service fails | Admin session, the service fails with a database error | 500 Internal Server Error `msgFailedToListUsers` | P2 |  |
| UR-81 | ListUsersHandler | Error Path | no permission and an invalid query | Session without ScopeAdminUsers, limit 0 and offset -1 | 403 Forbidden `msgNoPermissionToGetUsers`, not 400 | P2 |  |
| UR-82 | ListUsersHandler | Edge Case | the smallest limit | Admin session, limit 1 | 200 OK and the service got limit 1 | P2 |  |
