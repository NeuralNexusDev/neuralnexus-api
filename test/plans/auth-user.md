# Test plan: modules/auth/user

## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| US-01 | NewService | Happy Path | Service wired to the store's sub-stores | fake sub-stores | Returned `Service` delegates to the exact `AccountStore`/`LinkAccountStore`/`AccountSettingsStore` instances returned | P2 |  |
| US-02 | GetUser | Happy Path | `as.GetAccountByID` succeeds | | Returns the user for the account, nil | P1 |  |
| US-03 | GetUser | Error Path | `as.GetAccountByID` fails | | Error propagated unchanged | P2 |  |
| US-04 | GetUserFromPlatform | Happy Path | `als.GetLinkedAccountByPlatformID` then `as.GetAccountByID` both succeed | | Returns the resolved user whose JSON equals the account's JSON (user_id, username, roles, updated_at only), nil | P1 |  |
| US-05 | GetUserFromPlatform | Error Path | `als` lookup fails (e.g. `ErrNotFound`) | | Returns nil, error; `as.GetAccountByID` never called | P2 |  |
| US-06 | GetUserFromPlatform | Error Path | `als` lookup succeeds but `as.GetAccountByID` fails | | Error propagated unchanged | P2 |  |
| US-07 | GetUserPermissions | Happy Path | `as.GetAccountByID` returns an account holding one role id | fake role store returns permission strings | Returns the role store's node permissions | P1 |  |
| US-08 | GetUserPermissions | Error Path | `as.GetAccountByID` fails | | Returns nil, error | P2 |  |
| US-09 | GetUserPermissions | Edge Case | `Account.Roles` holds two role ids | fake role store | The role store is called once with both ids and the permissions of both are returned | P1 |  |
| US-10 | GetUserPermissions | Edge Case | `Account.Roles` empty | | Returns empty non-nil permissions, nil error, without a role lookup | P2 |  |
| US-11 | UpdateUser | Happy Path | `as.GetAccountByID` succeeds; `update` has `Username` and `Roles` set | | Merges those fields onto the fetched account and calls `as.UpdateAccountInDB` once with the merged result | P1 |  |
| US-12 | UpdateUser | Edge Case | `update` is empty (`Username == ""`, `Roles == nil`) | | Existing account's `Username`, `Email` and `Roles` are preserved unchanged (only non-zero fields overwrite) | P0 |  |
| US-13 | UpdateUser | Error Path | `as.GetAccountByID` fails | | Returns error; `UpdateAccountInDB` never called | P2 |  |
| US-14 | UpdateUser | Error Path | `as.UpdateAccountInDB` fails | | Error propagated unchanged | P2 |  |
| US-24 | UpdateUserFromPlatform | Error Path | Final `as.GetAccountByID` fails | | Error propagated | P2 |  |
| US-83 | UpdateUserFromPlatform | Happy Path | The linking service returns a user ID | `Service` built with a stub `linking.Service` returning `"resolved"`; `as.GetAccountByID` returns an account | Returns the user whose JSON equals the account's JSON, nil; `GetAccountByID` called with `"resolved"` | P1 |  |
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
| US-43 | ListUsers | Happy Path | a page of users is listed | store returns two accounts | Returns users in store order whose JSON equals the accounts' JSON, and the store received the same limit and offset | P1 |  |
| US-44 | ListUsers | Error Path | the store fails | store returns `testerrors.ErrBoom` | Returns nil accounts and the store's error | P2 |  |
| US-85 | userFromAccount | Happy Path | `GetUser` reads an account with every field set: email, hashed secret, salt, roles and updated_at | `as.GetAccountByID` returns the account | Returns a user whose JSON equals the account's JSON; email, hashed secret and salt do not appear | P0 |  |
| US-86 | userFromAccount | Edge Case | `as.GetAccountByID` returns a nil account and nil error | fake store with no account | `GetUser` returns nil user and nil error | P2 |  |

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| UR-01 | GetUserHandler | Happy Path | Self-lookup | session.UserID == path user_id | 200 OK with the user struct | P1 |  |
| UR-02 | GetUserHandler | Happy Path | Admin looks up another user | session.UserID != path user_id, session has ScopeAdminUsers | 200 OK with the user struct | P1 |  |
| UR-03 | GetUserHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden `msgNoPermissionToGetUser` | P2 |  |
| UR-04 | GetUserHandler | Error Path | UserService.GetUser fails with a non-not-found error | Permission check passes, service.GetUser returns `testerrors.ErrDBDown` | 500 Internal Server Error `msgFailedToGetUser` | P2 |  |
| UR-05 | GetUserFromPlatformHandler | Happy Path | Admin fetches a user by platform ID | session has ScopeAdminUsers, service.GetUserFromPlatform succeeds | 200 OK with the user struct | P1 |  |
| UR-06 | GetUserFromPlatformHandler | Error Path | No permission | session lacks ScopeAdminUsers | 403 Forbidden `msgNoPermissionToGetUsers` | P2 |  |
| UR-07 | GetUserFromPlatformHandler | Error Path | UserService.GetUserFromPlatform errors | Admin session, service returns `account.ErrNotFound` | 404 Not Found `msgUserNotFound` | P2 |  |
| UR-08 | GetUserPermissionsHandler | Happy Path | Self-lookup | session.UserID == path user_id | 200 OK with the permissions list | P1 |  |
| UR-09 | GetUserPermissionsHandler | Happy Path | Admin looks up another user's permissions | session.UserID != path user_id, session has ScopeAdminUsers | 200 OK with the permissions list | P2 |  |
| UR-10 | GetUserPermissionsHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden `msgNoPermissionToGetUserPermissions` | P2 |  |
| UR-11 | GetUserPermissionsHandler | Error Path | UserService.GetUserPermissions fails with a non-not-found error | Permission check passes, service returns `testerrors.ErrDBDown` | 500 Internal Server Error `msgFailedToGetUser` | P2 |  |
| UR-12 | UpdateUserHandler | Happy Path | Admin updates a user | session has ScopeAdminUsers, valid JSON body, service.UpdateUser succeeds | 200 OK with the account the service reads back; `UpdateUser` receives the path user_id (overriding the body's) | P1 |  |
| UR-13 | UpdateUserHandler | Error Path | No permission | session lacks ScopeAdminUsers | 403 Forbidden `msgNoPermissionToUpdateUsers` | P2 |  |
| UR-14 | UpdateUserHandler | Error Path | Malformed body | Admin session, body is not valid JSON | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| UR-15 | UpdateUserHandler | Error Path | UserService.UpdateUser fails with an unclassified error | Admin session, valid body, service returns `testerrors.ErrDBDown` | 500 Internal Server Error `msgFailedToUpdateUser` | P2 |  |
| UR-16 | UpdateUserFromPlatformHandler | Happy Path | platform=discord, valid body | Admin session, body decodes into linking.DiscordData, service succeeds | 200 OK with the user struct | P1 |  |
| UR-17 | UpdateUserFromPlatformHandler | Happy Path | platform=minecraft, valid body | Admin session, body decodes into linking.MinecraftData, service succeeds | 200 OK with the user struct | P2 |  |
| UR-18 | UpdateUserFromPlatformHandler | Happy Path | platform=twitch, valid body | Admin session, body decodes into twitch.Data, service succeeds | 200 OK with the user struct | P2 |  |
| UR-19 | UpdateUserFromPlatformHandler | Happy Path | platform=xboxlive, valid body | Admin session, body decodes into linking.XboxLiveData, service succeeds | 200 OK with the user struct | P2 |  |
| UR-20 | UpdateUserFromPlatformHandler | Happy Path | platform=microsoft, valid body | Admin session, body decodes into linking.MicrosoftUserData, service succeeds | 200 OK with the user struct | P2 |  |
| UR-21 | UpdateUserFromPlatformHandler | Error Path | Unsupported platform value | platform path value matches none of the five known platforms | 400 Bad Request `msgUnsupportedPlatform` | P2 |  |
| UR-22 | UpdateUserFromPlatformHandler | Error Path | No permission | session lacks ScopeAdminUsers | 403 Forbidden `msgNoPermissionToUpdateUsers` | P2 |  |
| UR-23 | UpdateUserFromPlatformHandler | Error Path | Malformed body for the given platform | Admin session, platform=discord, body is not valid JSON | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| UR-24 | UpdateUserFromPlatformHandler | Error Path | UserService.UpdateUserFromPlatform fails with an unclassified error | Admin session, valid body, service returns `testerrors.ErrDBDown` | 500 Internal Server Error `msgFailedToUpdateUser` | P2 |  |
| UR-25 | DeleteUserHandler | Happy Path | Admin deletes a user | session has ScopeAdminUsers, service.DeleteUser succeeds | 204 No Content | P1 |  |
| UR-26 | DeleteUserHandler | Error Path | No permission | session lacks ScopeAdminUsers | 403 Forbidden `msgNoPermissionToDeleteUsers` | P2 |  |
| UR-27 | DeleteUserHandler | Error Path | UserService.DeleteUser fails with an unclassified error | Admin session, service returns `testerrors.ErrDBDown` | 500 Internal Server Error `msgFailedToDeleteUser` | P2 |  |
| UR-28 | GetUserLinkedAccountsHandler | Happy Path | Self-lookup | session.UserID == path user_id | 200 OK with the linked-accounts list | P1 |  |
| UR-29 | GetUserLinkedAccountsHandler | Happy Path | Admin looks up another user's linked accounts | session.UserID != path user_id, session has ScopeAdminUsers | 200 OK with the linked-accounts list | P2 |  |
| UR-30 | GetUserLinkedAccountsHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden `msgNoPermissionToViewLinkedAccounts` | P2 |  |
| UR-31 | GetUserLinkedAccountsHandler | Error Path | UserService.GetUserLinkedAccounts errors | Permission check passes, service returns an error | 500 Internal Server Error `msgFailedToGetLinkedAccounts` | P2 |  |
| UR-32 | UnlinkPlatformHandler | Happy Path | Self unlinks a platform | session.UserID == path user_id, service.UnlinkPlatform succeeds | 204 No Content | P1 |  |
| UR-33 | UnlinkPlatformHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden `msgNoPermissionToUnlinkPlatforms` | P2 |  |
| UR-34 | UnlinkPlatformHandler | Error Path | Platform not linked | service.UnlinkPlatform returns account.ErrNotFound | 404 Not Found `msgPlatformNotLinked` | P2 |  |
| UR-35 | UnlinkPlatformHandler | Error Path | Would lock the account | service.UnlinkPlatform returns account.ErrWouldLockAccount | 400 Bad Request `msgSetPasswordOrLinkBeforeUnlinking` | P2 |  |
| UR-36 | UnlinkPlatformHandler | Error Path | Unclassified service error | service.UnlinkPlatform returns some other error | 500 Internal Server Error `msgFailedToUnlinkPlatform` | P2 |  |
| UR-37 | SetPlatformLoginEnabledHandler | Happy Path | Self toggles login-enabled | session.UserID == path user_id, body {"login_enabled":true}, service succeeds | 204 No Content | P1 |  |
| UR-38 | SetPlatformLoginEnabledHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden `msgNoPermissionToUpdatePlatforms` | P2 |  |
| UR-39 | SetPlatformLoginEnabledHandler | Error Path | Malformed body | Permission check passes, body is not valid JSON | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| UR-40 | SetPlatformLoginEnabledHandler | Edge Case | login_enabled field omitted | Body decodes with no error but LoginEnabled == nil | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| UR-41 | SetPlatformLoginEnabledHandler | Error Path | Platform not linked | service.SetPlatformLoginEnabled returns account.ErrNotFound | 404 Not Found `msgPlatformNotLinked` | P2 |  |
| UR-42 | SetPlatformLoginEnabledHandler | Error Path | Would lock the account | service.SetPlatformLoginEnabled returns account.ErrWouldLockAccount | 400 Bad Request `msgSetPasswordOrLinkBeforeDisabling` | P2 |  |
| UR-43 | SetPlatformLoginEnabledHandler | Error Path | Linked account unverified | service.SetPlatformLoginEnabled returns auth.ErrLinkedAccountUnverified | 400 Bad Request `msgLinkedAccountUnverified` | P2 |  |
| UR-44 | SetPlatformLoginEnabledHandler | Error Path | Unclassified service error | service.SetPlatformLoginEnabled returns some other error | 500 Internal Server Error `msgFailedToUpdatePlatform` | P2 |  |
| UR-45 | GetAccountSettingsHandler | Happy Path | Self-lookup | session.UserID == path user_id | 200 OK with the account settings | P1 |  |
| UR-46 | GetAccountSettingsHandler | Happy Path | Admin looks up another user's settings | session.UserID != path user_id, session has ScopeAdminUsers | 200 OK with the account settings | P2 |  |
| UR-47 | GetAccountSettingsHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden `msgNoPermissionToViewSettings` | P2 |  |
| UR-48 | GetAccountSettingsHandler | Error Path | UserService.GetAccountSettings errors | Permission check passes, service returns an error | 500 Internal Server Error `msgFailedToGetAccountSettings` | P2 |  |
| UR-49 | UpdateAccountSettingsHandler | Happy Path | Self updates settings | session.UserID == path user_id, body {"password_auth":true}, service succeeds | 204 No Content | P1 |  |
| UR-50 | UpdateAccountSettingsHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden `msgNoPermissionToUpdateSettings` | P2 |  |
| UR-51 | UpdateAccountSettingsHandler | Error Path | Malformed body | Permission check passes, body is not valid JSON | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| UR-52 | UpdateAccountSettingsHandler | Edge Case | password_auth field omitted | Body decodes with no error but PasswordAuthEnabled == nil | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| UR-53 | UpdateAccountSettingsHandler | Error Path | Would lock the account | service.SetPasswordAuthEnabled returns account.ErrWouldLockAccount | 400 Bad Request `msgEnableLoginMethodBeforeDisablingPassword` | P2 |  |
| UR-54 | UpdateAccountSettingsHandler | Error Path | No password set | service.SetPasswordAuthEnabled returns account.ErrNoPasswordSet | 400 Bad Request `msgSetPasswordBeforeEnablingPasswordLogin` | P2 |  |
| UR-55 | UpdateAccountSettingsHandler | Error Path | Unclassified service error | service.SetPasswordAuthEnabled returns some other error | 500 Internal Server Error `msgFailedToUpdateUserSettings` | P2 |  |
| UR-56 | UpdateUserFromPlatformHandler | Error Path | Malformed body for the Minecraft, Twitch, Xbox Live and Microsoft platforms | Admin session, body is not valid JSON | 400 Bad Request `msgInvalidRequestBody`; `UpdateUserFromPlatform` never called | P2 | |
| UR-57 | GetUserFromPlatformHandler | Error Path | UserService.GetUserFromPlatform fails with a non-not-found error | Admin session, service returns `testerrors.ErrDBDown` | 500 Internal Server Error `msgFailedToGetUser` | P2 |  |
| UR-58 | GetUserHandler | Error Path | UserService.GetUser returns not found | Permission check passes, service returns `account.ErrNotFound` | 404 Not Found `msgUserNotFound` | P2 |  |
| UR-59 | GetUserPermissionsHandler | Error Path | UserService.GetUserPermissions returns not found | Permission check passes, service returns `account.ErrNotFound` | 404 Not Found `msgUserNotFound` | P2 |  |
| UR-60 | UpdateUserHandler | Error Path | UserService.UpdateUser returns not found | Admin session, valid body, service returns `account.ErrNotFound` | 404 Not Found `msgUserNotFound` | P2 |  |
| UR-61 | UpdateUserHandler | Error Path | UserService.UpdateUser returns a duplicate email | Admin session, valid body, service returns `account.ErrEmailAlreadyExists` | 409 Conflict `msgEmailAlreadyExists` | P2 |  |
| UR-62 | UpdateUserHandler | Error Path | UserService.UpdateUser returns a duplicate username | Admin session, valid body, service returns `account.ErrUsernameAlreadyExists` | 409 Conflict `msgUsernameAlreadyExists` | P2 |  |
| UR-63 | UpdateUserFromPlatformHandler | Error Path | UserService.UpdateUserFromPlatform returns not found | Admin session, valid body, service returns `account.ErrNotFound` | 404 Not Found `msgUserNotFound` | P2 |  |
| UR-64 | UpdateUserFromPlatformHandler | Error Path | UserService.UpdateUserFromPlatform returns a duplicate email | Admin session, valid body, service returns `account.ErrEmailAlreadyExists` | 409 Conflict `msgEmailAlreadyExists` | P2 |  |
| UR-65 | UpdateUserFromPlatformHandler | Error Path | UserService.UpdateUserFromPlatform returns a duplicate username | Admin session, valid body, service returns `account.ErrUsernameAlreadyExists` | 409 Conflict `msgUsernameAlreadyExists` | P2 |  |
| UR-66 | GetAccountSettingsHandler | Error Path | UserService.GetAccountSettings returns not found | Permission check passes, service returns `account.ErrNotFound` | 404 Not Found `msgUserNotFound` | P2 |  |
| UR-67 | UpdateAccountSettingsHandler | Error Path | UserService.SetPasswordAuthEnabled returns not found | Permission check passes, valid body, service returns `account.ErrNotFound` | 404 Not Found `msgUserNotFound` | P2 |  |
| UR-68 | UpdateUserHandler | Error Path | UserService.UpdateUser returns an invalid role id | Admin session, valid body, service returns `account.ErrInvalidRoleID` | 400 Bad Request `msgInvalidRoleID` | P1 |  |
| UR-69 | UpdateUserHandler | Error Path | UserService.UpdateUser returns an unknown role id | Admin session, valid body, service returns `account.ErrUnknownRoleID` | 400 Bad Request `msgUnknownRoleID` | P1 |  |
| UR-70 | UpdateUserFromPlatformHandler | Error Path | UserService.UpdateUserFromPlatform returns an invalid role id | Admin session, valid body, service returns `account.ErrInvalidRoleID` | 400 Bad Request `msgInvalidRoleID` | P2 |  |
| UR-71 | UpdateUserFromPlatformHandler | Error Path | UserService.UpdateUserFromPlatform returns an unknown role id | Admin session, valid body, service returns `account.ErrUnknownRoleID` | 400 Bad Request `msgUnknownRoleID` | P2 |  |
| UR-72 | UpdateUserHandler | Edge Case | the body repeats a role id | Admin session, body roles `["1","1","3"]`, the service reads back roles `["1","3"]` | 200 OK whose roles are the stored roles, without the duplicate | P1 |  |
| UR-73 | UpdateUserHandler | Error Path | reading the account back fails after the update | Admin session, `UpdateUser` succeeds, `GetUser` returns `testerrors.ErrDBDown` | 500 `msgFailedToUpdateUser` after `UpdateUser` was called | P2 |  |
| UR-74 | ListUsersHandler | Happy Path | list users with no query | Admin session, service returns two accounts | 200 OK with the two users in order and no email, hashed secret or salt; the service got limit 50 and offset 0 | P1 |  |
| UR-75 | ListUsersHandler | Happy Path | limit and offset in the query | Admin session, `?limit=200&offset=40` | 200 OK and the service got limit 200 and offset 40 | P1 |  |
| UR-76 | ListUsersHandler | Error Path | no permission | Session without ScopeAdminUsers | 403 Forbidden `msgNoPermissionToGetUsers` and the service is not called | P1 |  |
| UR-77 | ListUsersHandler | Error Path | invalid limit | Admin session, limit of 0, -1, 201, `abc`, `1.5` or a space | 400 Bad Request `msgInvalidLimit` and the service is not called | P2 |  |
| UR-78 | ListUsersHandler | Error Path | invalid offset | Admin session, offset of -1, `abc`, `1.5` or a space | 400 Bad Request `msgInvalidOffset` and the service is not called | P2 |  |
| UR-79 | ListUsersHandler | Edge Case | the page is empty | Admin session, service returns no accounts | 200 OK with body `[]` | P2 |  |
| UR-80 | ListUsersHandler | Error Path | the service fails | Admin session, service returns `testerrors.ErrDBDown` | 500 Internal Server Error `msgFailedToListUsers` | P2 |  |
| UR-81 | ListUsersHandler | Error Path | no permission and an invalid query | Session without ScopeAdminUsers, `?limit=0&offset=-1` | 403 Forbidden `msgNoPermissionToGetUsers`, not 400 | P2 |  |
| UR-82 | ListUsersHandler | Edge Case | the smallest limit | Admin session, `?limit=1` | 200 OK and the service got limit 1 | P2 |  |
