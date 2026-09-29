# Test plan: modules/auth/routes

## auth.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| AU-01 | LoginHandler | Happy Path | Login with username+password | Account exists for username, password matches, password auth enabled | 204 No Content, session cookie set | P1 |  |
| AU-02 | LoginHandler | Happy Path | Login with email+password (username field empty) | Account exists for email, password matches, password auth enabled | 204 No Content, session cookie set | P1 |  |
| AU-03 | LoginHandler | Error Path | Malformed request body | Body is not valid JSON | 400 Bad Request `msgInvalidUsernameOrPassword` | P2 |  |
| AU-04 | LoginHandler | Error Path | Account lookup fails | AccountService.GetAccountByUsername/GetAccountByEmail returns an error | 400 Bad Request `msgInvalidUsernameOrPassword` | P2 |  |
| AU-05 | LoginHandler | Error Path | Wrong password | Account found, Account.ValidateUser returns false | 400 Bad Request `msgInvalidUsernameOrPassword` | P2 |  |
| AU-06 | LoginHandler | Error Path | IsPasswordAuthEnabled errors | Account and password valid, AccountService.IsPasswordAuthEnabled returns an error | 500 Internal Server Error `msgAuthenticationFailed` | P2 |  |
| AU-07 | LoginHandler | Edge Case | Password auth disabled for account | Account and password valid, IsPasswordAuthEnabled returns (false, nil) | 400 Bad Request `msgInvalidUsernameOrPassword` | P2 |  |
| AU-08 | LoginHandler | Error Path | SessionService.AddSession fails | Account/password/enabled all valid, ss.AddSession returns an error | 500 Internal Server Error `msgAuthenticationFailed` | P2 |  |
| AU-09 | LoginHandler | Error Path | SessionService.CreateJWT fails | AddSession succeeds, ss.CreateJWT returns an error | 500 Internal Server Error `msgAuthenticationFailed` | P2 |  |
| AU-10 | LogoutHandler | Happy Path | Logout with a valid session | Context carries a non-nil *auth.Session, ss.DeleteSession succeeds | 204 No Content, session cookie cleared (empty value, expiry in the past) | P1 |  |
| AU-11 | LogoutHandler | Edge Case | Session in context is a nil *auth.Session | Context value under mw.SessionKey is (*auth.Session)(nil) | 400 Bad Request "Invalid session" | P3 |  |
| AU-12 | LogoutHandler | Error Path | SessionService.DeleteSession fails | Valid session in context, ss.DeleteSession returns an error | 500 Internal Server Error "Failed to delete session" | P2 |  |
| AU-13 | OAuthHandler | Error Path | Missing "code" query param | Request has no code param | 303 redirect to auth.NN_SITE_URL with a Bad Request problem in the "problem" query param | P1 |  |
| AU-14 | OAuthHandler | Error Path | Missing "state" query param | code param present, state param absent (delegates to decodeAndValidateState) | 303 redirect to auth.NN_SITE_URL with a Bad Request problem | P2 |  |
| AU-15 | OAuthHandler | Error Path | Mode=link with no session in context | code+valid state present, state.Mode=link, no session in request context (delegates to requireValidModeAndSession) | 303 redirect to state.RedirectURI with an Unauthorized problem | P2 |  |
| AU-17 | OpenIDHandler | Error Path | Missing "state" query param | state param absent (delegates to decodeAndValidateState) | 303 redirect to auth.NN_SITE_URL with a Bad Request problem | P2 |  |
| AU-18 | OpenIDHandler | Error Path | Mode=link with no session in context | Valid state present, state.Mode=link, no session in request context | 303 redirect to state.RedirectURI with an Unauthorized problem | P2 |  |
| AU-19 | OpenIDHandler | Error Path | VerifySteamOpenIDCallback rejects the assertion locally (bad openid.mode) | Valid state, query has openid.mode != "id_res" (no network call reached) | 303 redirect to state.RedirectURI with a Bad Request problem `msgInvalidState` | P1 |  |
| AU-20 | OpenIDHandler | Error Path | VerifySteamOpenIDCallback rejects the assertion locally (missing/invalid openid.claimed_id) | Valid state, openid.mode="id_res", openid.claimed_id missing/malformed | 303 redirect to state.RedirectURI with a Bad Request problem `msgInvalidState` | P2 |  |
| AU-50 | OAuthHandler | Happy Path | mode=login, platform=discord | code+valid state present, mode=login; http.DefaultTransport swapped for a fake RoundTripper standing in for Discord's token and `/users/@me` endpoints (both reached through clients with no Transport set, so they fall back to http.DefaultTransport - see swapTransport in modules/projects/projects_test.go for the same technique); AccountService/LinkAccountStore/SessionService mocks resolve a brand-new account | 303 redirect straight to state.RedirectURI (no problem param), session cookie set with the JWT from ss.CreateJWT | P1 | No live network dependency: the Discord HTTP calls are faked at the transport layer, not skipped. |
| AU-51 | OpenIDHandler | Happy Path | mode=login, platform=steam | Valid state, mode=login, openid.mode=id_res with a valid signed claimed_id; http.DefaultTransport swapped for a fake RoundTripper standing in for Steam's check_authentication and GetPlayerSummaries endpoints (steamHTTPClient also has no Transport set); linking.STEAM_API_KEY set to a test value for the duration of the subtest; AccountService/LinkAccountStore/SessionService mocks resolve a brand-new account | 303 redirect straight to state.RedirectURI (no problem param), session cookie set with the JWT from ss.CreateJWT | P1 | No live network dependency: the Steam HTTP calls are faked at the transport layer, not skipped. |
| AU-22 | decodeAndValidateState | Happy Path | Valid base64+JSON state, nonce cookie matches | state has Platform/Nonce/RedirectURI/Mode set, RedirectURI allowed, "nonce" cookie value == state.Nonce | Returns (state, true) | P1 |  |
| AU-23 | decodeAndValidateState | Error Path | Missing "state" query param | No state param on the request | Returns (zero state, false); redirectBadRequest to auth.NN_SITE_URL | P2 |  |
| AU-24 | decodeAndValidateState | Error Path | "state" is not valid base64 | state param is not URL-safe base64 | Returns (zero state, false); redirectBadRequest to auth.NN_SITE_URL, detail `msgInvalidState` | P2 |  |
| AU-25 | decodeAndValidateState | Error Path | Decoded bytes are not valid JSON | base64-valid but JSON-unmarshal fails | Returns (zero state, false); redirectBadRequest to auth.NN_SITE_URL, detail `msgInvalidState` | P2 |  |
| AU-26 | decodeAndValidateState | Edge Case | A required state field is empty (e.g. Platform="") | Otherwise-valid state JSON missing Platform/Nonce/RedirectURI/Mode | Returns (state, false); redirectBadRequest to auth.NN_SITE_URL, detail `msgInvalidState` | P2 |  |
| AU-27 | decodeAndValidateState | Error Path | RedirectURI not on the allowlist | state.RedirectURI has a different host than auth.NN_SITE_URL | Returns (state, false); redirectBadRequest to auth.NN_SITE_URL, detail `msgInvalidState` | P2 |  |
| AU-28 | decodeAndValidateState | Error Path | Missing "nonce" cookie | Valid state, request has no "nonce" cookie | Returns (state, false); redirectBadRequest to auth.NN_SITE_URL, detail `msgInvalidState` | P2 |  |
| AU-29 | decodeAndValidateState | Error Path | Nonce cookie value doesn't match state.Nonce | Valid state, "nonce" cookie present but value != state.Nonce | Returns (state, false); redirectBadRequest to auth.NN_SITE_URL, detail `msgInvalidState` | P2 |  |
| AU-30 | requireValidModeAndSession | Happy Path | mode=login | Any session state | Returns true, no response written | P1 |  |
| AU-31 | requireValidModeAndSession | Happy Path | mode=link with a valid, non-expired session in context | Context carries *auth.Session with IsValid()==true | Returns true, no response written | P1 |  |
| AU-32 | requireValidModeAndSession | Error Path | mode=link, no session key in context at all | r.Context().Value(mw.SessionKey) type-asserts ok=false | Returns false; redirectUnauthorized to redirectURI | P2 |  |
| AU-33 | requireValidModeAndSession | Edge Case | mode=link, session in context is a nil *auth.Session | ok=true, session==nil | Returns false; redirectUnauthorized to redirectURI | P3 |  |
| AU-34 | requireValidModeAndSession | Error Path | mode=link, session present but expired | session.IsValid()==false (ExpiresAt in the past) | Returns false; redirectUnauthorized to redirectURI | P2 |  |
| AU-35 | requireValidModeAndSession | Error Path | Unrecognized mode value | mode is neither linking.ModeLogin nor linking.ModeLink | Returns false; redirectBadRequest to redirectURI, detail `msgInvalidState` | P2 |  |
| AU-36 | createSessionJWTAndSetCookie | Happy Path | ss.CreateJWT succeeds | Valid session, ss.CreateJWT returns a JWT string | Returns nil error; Set-Cookie header carries the JWT value with Expires == time.Unix(session.ExpiresAt,0) | P1 |  |
| AU-37 | createSessionJWTAndSetCookie | Error Path | ss.CreateJWT fails | ss.CreateJWT returns an error | Returns that error; no Set-Cookie header written | P2 |  |
| AU-38 | redirectWithError | Happy Path | Well-formed target URL | target parses successfully with url.Parse | 303 redirect; redirect Location has a "problem" query param that base64url-decodes to the expected RFC 9457 JSON (status/title/detail) | P1 |  |
| AU-39 | redirectWithError | Edge Case | Target URL fails url.Parse | target contains an invalid percent-escape (e.g. "%zz") | 303 redirect straight to target, unmodified (no "problem" param appended) | P3 |  |
| AU-41 | redirectBadRequest | Accessor | Wraps redirectWithError with 400/"Bad Request" | any target/detail | redirectWithError called with http.StatusBadRequest, "Bad Request", the given detail | P2 |  |
| AU-42 | redirectUnauthorized | Accessor | Wraps redirectWithError with 401/"Unauthorized" | any target/detail | redirectWithError called with http.StatusUnauthorized, "Unauthorized", the given detail | P2 |  |
| AU-43 | redirectInternalServerError | Accessor | Wraps redirectWithError with 500/"Internal Server Error" | any target/detail | redirectWithError called with http.StatusInternalServerError, "Internal Server Error", the given detail | P2 |  |
| AU-44 | isAllowedRedirect | Happy Path | redirectURI scheme+host match auth.NN_SITE_URL | redirectURI = auth.NN_SITE_URL + some path | Returns true | P1 |  |
| AU-45 | isAllowedRedirect | Error Path | redirectURI has a different host | Same scheme, different host than auth.NN_SITE_URL | Returns false | P2 |  |
| AU-46 | isAllowedRedirect | Edge Case | redirectURI has a different scheme, same host | e.g. http vs https | Returns false | P3 |  |
| AU-47 | isAllowedRedirect | Edge Case | redirectURI fails url.Parse | redirectURI contains an invalid percent-escape | Returns false | P3 |  |
| AU-48 | isAllowedRedirect | Edge Case | auth.NN_SITE_URL itself fails url.Parse | auth.NN_SITE_URL temporarily overridden with an unparseable value, restored after the subtest | Returns false | P3 |  |
| AU-49 | sessionCookie | Accessor | Build a cookie for a given value/expiry | value="abc", expires=fixed time | Returns *http.Cookie with Name=mw.SessionCookieName, Domain=".neuralnexus.dev", Path="/", Expires=expires, Secure=true, HttpOnly=true, SameSite=Lax | P2 |  |

## users.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| US-01 | GetUserHandler | Happy Path | Self-lookup | session.UserID == path user_id | 200 OK with the user struct | P1 |  |
| US-02 | GetUserHandler | Happy Path | Admin looks up another user | session.UserID != path user_id, session has ScopeAdminUsers | 200 OK with the user struct | P1 |  |
| US-03 | GetUserHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden "You do not have permission to get this user" | P2 |  |
| US-04 | GetUserHandler | Error Path | UserService.GetUser errors | Permission check passes, service.GetUser returns an error | 404 Not Found `msgUserNotFound` | P2 |  |
| US-05 | GetUserFromPlatformHandler | Happy Path | Admin fetches a user by platform ID | session has ScopeAdminUsers, service.GetUserFromPlatform succeeds | 200 OK with the user struct | P1 |  |
| US-06 | GetUserFromPlatformHandler | Error Path | No permission | session lacks ScopeAdminUsers | 403 Forbidden "You do not have permission to get users" | P2 |  |
| US-07 | GetUserFromPlatformHandler | Error Path | UserService.GetUserFromPlatform errors | Admin session, service returns an error | 404 Not Found `msgUserNotFound` | P2 |  |
| US-08 | GetUserPermissionsHandler | Happy Path | Self-lookup | session.UserID == path user_id | 200 OK with the permissions list | P1 |  |
| US-09 | GetUserPermissionsHandler | Happy Path | Admin looks up another user's permissions | session.UserID != path user_id, session has ScopeAdminUsers | 200 OK with the permissions list | P2 |  |
| US-10 | GetUserPermissionsHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden "You do not have permission to get user permissions" | P2 |  |
| US-11 | GetUserPermissionsHandler | Error Path | UserService.GetUserPermissions errors | Permission check passes, service returns an error | 404 Not Found `msgUserNotFound` | P2 |  |
| US-12 | UpdateUserHandler | Happy Path | Admin updates a user | session has ScopeAdminUsers, valid JSON body, service.UpdateUser succeeds | 200 OK with the user struct; response UserID equals the path user_id (overridden after decode) | P1 |  |
| US-13 | UpdateUserHandler | Error Path | No permission | session lacks ScopeAdminUsers | 403 Forbidden `msgNoPermissionToUpdateUsers` | P2 |  |
| US-14 | UpdateUserHandler | Error Path | Malformed body | Admin session, body is not valid JSON | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| US-15 | UpdateUserHandler | Error Path | UserService.UpdateUser errors | Admin session, valid body, service returns an error | 400 Bad Request `msgFailedToUpdateUser` | P2 |  |
| US-16 | UpdateUserFromPlatformHandler | Happy Path | platform=discord, valid body | Admin session, body decodes into linking.DiscordData, service succeeds | 200 OK with the user struct | P1 |  |
| US-17 | UpdateUserFromPlatformHandler | Happy Path | platform=minecraft, valid body | Admin session, body decodes into linking.MinecraftData, service succeeds | 200 OK with the user struct | P2 |  |
| US-18 | UpdateUserFromPlatformHandler | Happy Path | platform=twitch, valid body | Admin session, body decodes into twitch.Data, service succeeds | 200 OK with the user struct | P2 |  |
| US-19 | UpdateUserFromPlatformHandler | Happy Path | platform=xboxlive, valid body | Admin session, body decodes into linking.XboxLiveData, service succeeds | 200 OK with the user struct | P2 |  |
| US-20 | UpdateUserFromPlatformHandler | Happy Path | platform=microsoft, valid body | Admin session, body decodes into linking.MicrosoftUserData, service succeeds | 200 OK with the user struct | P2 |  |
| US-21 | UpdateUserFromPlatformHandler | Error Path | Unsupported platform value | platform path value matches none of the five known platforms | 400 Bad Request "Unsupported platform" | P2 |  |
| US-22 | UpdateUserFromPlatformHandler | Error Path | No permission | session lacks ScopeAdminUsers | 403 Forbidden `msgNoPermissionToUpdateUsers` | P2 |  |
| US-23 | UpdateUserFromPlatformHandler | Error Path | Malformed body for the given platform | Admin session, platform=discord, body is not valid JSON | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| US-24 | UpdateUserFromPlatformHandler | Error Path | UserService.UpdateUserFromPlatform errors | Admin session, valid body, service returns an error | 400 Bad Request `msgFailedToUpdateUser` | P2 |  |
| US-25 | DeleteUserHandler | Happy Path | Admin deletes a user | session has ScopeAdminUsers, service.DeleteUser succeeds | 204 No Content | P1 |  |
| US-26 | DeleteUserHandler | Error Path | No permission | session lacks ScopeAdminUsers | 403 Forbidden "You do not have permission to delete users" | P2 |  |
| US-27 | DeleteUserHandler | Error Path | UserService.DeleteUser errors | Admin session, service returns an error | 400 Bad Request "Failed to delete user" | P2 |  |
| US-28 | GetUserLinkedAccountsHandler | Happy Path | Self-lookup | session.UserID == path user_id | 200 OK with the linked-accounts list | P1 |  |
| US-29 | GetUserLinkedAccountsHandler | Happy Path | Admin looks up another user's linked accounts | session.UserID != path user_id, session has ScopeAdminUsers | 200 OK with the linked-accounts list | P2 |  |
| US-30 | GetUserLinkedAccountsHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden "You do not have permission to view this user's linked accounts" | P2 |  |
| US-31 | GetUserLinkedAccountsHandler | Error Path | UserService.GetUserLinkedAccounts errors | Permission check passes, service returns an error | 500 Internal Server Error "Failed to get linked accounts" | P2 |  |
| US-32 | UnlinkPlatformHandler | Happy Path | Self unlinks a platform | session.UserID == path user_id, service.UnlinkPlatform succeeds | 204 No Content | P1 |  |
| US-33 | UnlinkPlatformHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden "You do not have permission to unlink this user's platforms" | P2 |  |
| US-34 | UnlinkPlatformHandler | Error Path | Platform not linked | service.UnlinkPlatform returns auth.ErrNotFound | 404 Not Found `msgPlatformNotLinked` | P2 |  |
| US-35 | UnlinkPlatformHandler | Error Path | Would lock the account | service.UnlinkPlatform returns auth.ErrWouldLockAccount | 400 Bad Request "Set a password or link another platform before unlinking your last one" | P2 |  |
| US-36 | UnlinkPlatformHandler | Error Path | Unclassified service error | service.UnlinkPlatform returns some other error | 500 Internal Server Error "Failed to unlink platform" | P2 |  |
| US-37 | SetPlatformLoginEnabledHandler | Happy Path | Self toggles login-enabled | session.UserID == path user_id, body {"login_enabled":true}, service succeeds | 204 No Content | P1 |  |
| US-38 | SetPlatformLoginEnabledHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden "You do not have permission to update this user's platforms" | P2 |  |
| US-39 | SetPlatformLoginEnabledHandler | Error Path | Malformed body | Permission check passes, body is not valid JSON | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| US-40 | SetPlatformLoginEnabledHandler | Edge Case | login_enabled field omitted | Body decodes with no error but LoginEnabled == nil | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| US-41 | SetPlatformLoginEnabledHandler | Error Path | Platform not linked | service.SetPlatformLoginEnabled returns auth.ErrNotFound | 404 Not Found `msgPlatformNotLinked` | P2 |  |
| US-42 | SetPlatformLoginEnabledHandler | Error Path | Would lock the account | service.SetPlatformLoginEnabled returns auth.ErrWouldLockAccount | 400 Bad Request "Set a password or link another platform before disabling your last login method" | P2 |  |
| US-43 | SetPlatformLoginEnabledHandler | Error Path | Linked account unverified | service.SetPlatformLoginEnabled returns auth.ErrLinkedAccountUnverified | 400 Bad Request "This linked account is unverified and can't be enabled for login" | P2 |  |
| US-44 | SetPlatformLoginEnabledHandler | Error Path | Unclassified service error | service.SetPlatformLoginEnabled returns some other error | 500 Internal Server Error "Failed to update platform" | P2 |  |
| US-45 | GetAccountSettingsHandler | Happy Path | Self-lookup | session.UserID == path user_id | 200 OK with the account settings | P1 |  |
| US-46 | GetAccountSettingsHandler | Happy Path | Admin looks up another user's settings | session.UserID != path user_id, session has ScopeAdminUsers | 200 OK with the account settings | P2 |  |
| US-47 | GetAccountSettingsHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden "You do not have permission to view this user's settings" | P2 |  |
| US-48 | GetAccountSettingsHandler | Error Path | UserService.GetAccountSettings errors | Permission check passes, service returns an error | 500 Internal Server Error "Failed to get account settings" | P2 |  |
| US-49 | UpdateAccountSettingsHandler | Happy Path | Self updates settings | session.UserID == path user_id, body {"password_auth":true}, service succeeds | 204 No Content | P1 |  |
| US-50 | UpdateAccountSettingsHandler | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden "You do not have permission to update this user's settings" | P2 |  |
| US-51 | UpdateAccountSettingsHandler | Error Path | Malformed body | Permission check passes, body is not valid JSON | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| US-52 | UpdateAccountSettingsHandler | Edge Case | password_auth field omitted | Body decodes with no error but PasswordAuthEnabled == nil | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| US-53 | UpdateAccountSettingsHandler | Error Path | Would lock the account | service.SetPasswordAuthEnabled returns auth.ErrWouldLockAccount | 400 Bad Request "Link and enable another login method before disabling your password" | P2 |  |
| US-54 | UpdateAccountSettingsHandler | Error Path | No password set | service.SetPasswordAuthEnabled returns auth.ErrNoPasswordSet | 400 Bad Request "Set a password before enabling password login" | P2 |  |
| US-55 | UpdateAccountSettingsHandler | Error Path | Unclassified service error | service.SetPasswordAuthEnabled returns some other error | 500 Internal Server Error "Failed to update user settings" | P2 |  |
