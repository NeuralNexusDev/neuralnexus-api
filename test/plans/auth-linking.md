# Test plan: modules/auth/linking

## oauth.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| OA-01 | ExtCodeForToken | Happy Path | Code exchange succeeds, token carries `scope` as a `[]interface{}` of strings | `config` points at an httptest token endpoint returning a valid token JSON body with `"scope":["identify","email"]` | Returns `*auth.OAuthToken` with AccessToken/TokenType/RefreshToken/Expiry/ExpiresIn copied and `Scope == []string{"identify","email"}`; err nil | P1 |  |
| OA-02 | ExtCodeForToken | Edge Case | Token carries `scope` as a bare string (Discord's single-scope shape) | Server returns `"scope":"identify"` | `Scope == []string{"identify"}`; err nil | P2 |  |
| OA-03 | ExtCodeForToken | Error Path | Token endpoint rejects the code | Server returns HTTP 400 with an OAuth2 error body | `config.Exchange` error is returned, token nil | P1 |  |
| OA-04 | ExtCodeForToken | Error Path | Token response omits `scope` entirely | Server returns a valid token JSON with no `scope` field | Returns `ErrNoScopeInToken`, token nil | P2 |  |
| OA-05 | RefreshToken | Happy Path | Refresh succeeds, `[]interface{}` scope | `config`+existing `oauth2.Token` point at httptest token endpoint returning a fresh token with array scope | Returns populated `*auth.OAuthToken`; err nil | P1 |  |
| OA-06 | RefreshToken | Edge Case | Refresh succeeds, bare-string scope | Server returns `"scope":"identify"` | `Scope == []string{"identify"}`; err nil | P2 |  |
| OA-07 | RefreshToken | Error Path | Refresh endpoint failure | Server returns HTTP 400 | Error returned, token nil | P1 |  |
| OA-08 | RefreshToken | Error Path | Refreshed token omits `scope` | Server returns valid token JSON without `scope` | Returns `ErrNoScopeInToken` | P2 |  |
| OA-09 | ProcessOAuthLogin | Happy Path | Discord platform, brand-new platform identity | `discordConfig` + Discord user endpoint point at httptest servers; mock `las.GetLinkedAccountByPlatformID` → `auth.ErrNotFound`; mock `as.AddAccount`/`las.AddLinkedAccountToDB`/`ss.AddSession` succeed | Returns non-nil `*auth.Session`, err nil; `AddAccount` and `AddLinkedAccountToDB` each called once, `ss.AddSession` called once | P1 |  |
| OA-10 | ProcessOAuthLogin | Happy Path | Discord platform, existing verified+login-enabled linked account | Mock `GetLinkedAccountByPlatformID` returns a linked account with `Verified:true, LoginEnabled:true` | Returns session for the existing account via `GetAccountByID`; `AddAccount` never called | P1 |  |
| OA-11 | ProcessOAuthLogin | Error Path | Unknown/unsupported platform in state | `state.Platform = "bogus"` | Returns `ErrInvalidPlatform`; no store or network calls made | P2 |  |
| OA-12 | ProcessOAuthLogin | Error Path | Code exchange fails | Token endpoint returns HTTP 400 | Error propagated from `ExtCodeForToken`; no store calls | P2 |  |
| OA-13 | ProcessOAuthLogin | Error Path | Linked account exists but is unverified or login-disabled | Mock returns linked account with `Verified:false` (and separately one with `LoginEnabled:false`) | Returns `ErrPlatformLoginDisabled` | P1 |  |
| OA-14 | ProcessOAuthLogin | Happy Path | Minecraft/XboxLive platform, full XBL+Java chain succeeds for a brand-new account | `MicrosoftConfig` + XBL/XSTS/Minecraft endpoint vars point at httptest servers producing a valid xbox+java identity; no existing links | Returns a session; both Xbox and Java identities linked (`AddLinkedAccountToDB` called twice) | P1 |  |
| OA-15 | ProcessOAuthLogin | Error Path | Minecraft/XboxLive platform, Xbox/Minecraft identity fetch fails | XSTS endpoint returns a failure | Error propagated, no session created, no store calls | P2 |  |
| OA-16 | ProcessOAuthLogin | Error Path | `ss.AddSession` fails after a resolved account | Mock `ss.AddSession` returns an error | Error propagated, nil session | P2 |  |
| OA-17 | resolveOrCreateAccountForPlatformUser | Happy Path | No existing linked account | `las.GetLinkedAccountByPlatformID` → `auth.ErrNotFound`; `as.AddAccount` and `las.AddLinkedAccountToDB` succeed | Returns the newly created `*auth.Account`, err nil | P1 |  |
| OA-18 | resolveOrCreateAccountForPlatformUser | Happy Path | Existing verified+enabled linked account | Mock returns a linked account with `Verified:true, LoginEnabled:true` | Returns account via `as.GetAccountByID(la.UserID)` | P1 |  |
| OA-19 | resolveOrCreateAccountForPlatformUser | Error Path | Existing linked account is disabled for login | Mock returns linked account with `Verified:false` or `LoginEnabled:false` | Returns `ErrPlatformLoginDisabled` | P1 |  |
| OA-20 | resolveOrCreateAccountForPlatformUser | Error Path | Lookup fails with a non-`ErrNotFound` error | Mock `GetLinkedAccountByPlatformID` returns a generic DB error | Error propagated as-is | P2 |  |
| OA-21 | resolveOrCreateAccountForPlatformUser | Error Path | `as.AddAccount` fails | Mock `AddAccount` returns error | Error propagated; `AddLinkedAccountToDB` never called | P2 |  |
| OA-22 | resolveOrCreateAccountForPlatformUser | Error Path | `AddLinkedAccountToDB` fails with a non-`ErrAlreadyLinked` error and cleanup succeeds | Mock `AddLinkedAccountToDB` returns generic error; `DeleteAccount` succeeds | Returns original error; `DeleteAccount` called once | P0 |  |
| OA-23 | resolveOrCreateAccountForPlatformUser | Error Path | Same as OA-22, but cleanup (`DeleteAccount`) also fails | Both `AddLinkedAccountToDB` and `DeleteAccount` return errors | Returns a wrapped error mentioning both failures | P1 |  |
| OA-83 | resolveOrCreateAccountForPlatformUser | Error Path | Same as OA-23, asserting both causes | `AddLinkedAccountToDB` fails with `testerrors.ErrInsertFailed`; `DeleteAccount` fails with `testerrors.ErrBoom` | Returns an error wrapping both `testerrors.ErrInsertFailed` and `testerrors.ErrBoom` | P2 | |
| OA-24 | resolveOrCreateAccountForPlatformUser | Edge Case | `AddLinkedAccountToDB` fails with `ErrAlreadyLinked` (lost the race); cleanup and winner re-fetch succeed | Mock returns `auth.ErrAlreadyLinked`, cleanup succeeds, re-fetched linked account + account resolve fine | Returns the winner's `*auth.Account`, err nil | P0 |  |
| OA-25 | resolveOrCreateAccountForPlatformUser | Error Path | Lost the race, cleanup succeeds, but re-fetching the winner's link fails | `GetLinkedAccountByPlatformID` (second call) returns error | Error propagated | P2 |  |
| OA-26 | resolveOrCreateAccountForPlatformUser | Error Path | Lost the race, winner link found, but `GetAccountByID` for winner fails | Mock `GetAccountByID` returns error on second call | Error propagated | P2 |  |
| OA-27 | ProcessOAuthLink | Happy Path | Discord platform, valid session, identity not yet linked | Valid non-expired session in request context; mock `GetLinkedAccountByPlatformID` → `ErrNotFound`; `AddLinkedAccountToDB` succeeds | Returns the same session, err nil; `AddLinkedAccountToDB` called once | P1 |  |
| OA-28 | ProcessOAuthLink | Error Path | No session in request context | Request built without the session context value | Returns `ErrSessionNotFound` | P1 |  |
| OA-29 | ProcessOAuthLink | Error Path | Session present but expired | `session.IsValid()` false (`ExpiresAt` in the past) | Returns `ErrSessionExpired` | P1 |  |
| OA-30 | ProcessOAuthLink | Error Path | Unknown/unsupported platform | `state.Platform = "bogus"` | Returns `ErrInvalidPlatform` | P2 |  |
| OA-31 | ProcessOAuthLink | Error Path | Code exchange fails | Token endpoint returns HTTP 400 | Error propagated | P2 |  |
| OA-32 | ProcessOAuthLink | Happy Path | Minecraft/XboxLive platform, both Xbox and Java identities new | Full XBL/XSTS/Minecraft chain succeeds; neither identity linked yet | Returns session; both identities linked (`AddLinkedAccountToDB` called twice) | P1 |  |
| OA-33 | ProcessOAuthLink | Happy Path | Minecraft/XboxLive platform, account doesn't own Java Edition | Chain succeeds with `java == nil` | Returns session; only Xbox identity linked (`AddLinkedAccountToDB` called once) | P1 |  |
| OA-34 | ProcessOAuthLink | Error Path | Xbox identity already linked to a different account | `GetLinkedAccountByPlatformID(XboxLive,...)` returns a link owned by a different `UserID` | Returns `ErrPlatformAlreadyLinkedToDifferentAccount`; no `AddLinkedAccountToDB` calls made | P0 |  |
| OA-35 | ProcessOAuthLink | Error Path | Java identity already linked to a different account (Xbox check passes) | Xbox check clean; Java check returns a link owned by a different `UserID` | Returns `ErrPlatformAlreadyLinkedToDifferentAccount`; `AddLinkedAccountToDB` is never called, so the Xbox account is not linked either | P0 |  |
| OA-36 | ProcessOAuthLink | Error Path | Xbox pre-link-check lookup fails with a non-`ErrNotFound` error | Mock returns generic error | Error propagated; nothing linked | P2 |  |
| OA-37 | ProcessOAuthLink | Error Path | Java pre-link-check lookup fails with a non-`ErrNotFound` error | Mock returns generic error | Error propagated; nothing linked | P2 |  |
| OA-38 | ProcessOAuthLink | Edge Case | Xbox and Java both already linked to the *same* session's account | Both lookups return links owned by `session.UserID` | Returns session, err nil; linking is a no-op (no `AddLinkedAccountToDB` calls) | P1 |  |
| OA-39 | ProcessOAuthLink | Error Path | Minecraft/XboxLive platform, identity fetch itself fails | XSTS endpoint fails | Error propagated; no link-store calls at all | P2 |  |
| OA-40 | ProcessOAuthLink | Error Path | Xbox link write fails in the Minecraft/XboxLive branch | `AddLinkedAccountToDB` fails for the Xbox call | Error propagated; Java link is never attempted | P2 | `linkPlatformUserToSession` normalizes any `AddLinkedAccountToDB` failure to `ErrLinkAccountFailed` (see OA-71) |
| OA-41 | ProcessOAuthLink | Error Path | Non-minecraft platform, platform user fetch fails | `GetDiscordUser`/`GetMicrosoftUser` call fails | Error propagated | P2 |  |
| OA-42 | ProcessOAuthLink | Error Path | Non-minecraft platform, final link write fails | `linkPlatformUserToSession` returns error (e.g. identity linked elsewhere) | Error propagated | P2 |  |
| OA-43 | resolveOrCreateAccountForMicrosoftUser | Happy Path | Neither identity linked yet, Java present | Both lookups `ErrNotFound`; `java != nil` | New account created using Java's username (preferred over gamertag); both identities linked via `ensureMicrosoftIdentityLinked` | P0 |  |
| OA-44 | resolveOrCreateAccountForMicrosoftUser | Happy Path | Neither identity linked yet, Java absent (Xbox-only login) | Both lookups `ErrNotFound`; `java == nil` | New account created using the Xbox gamertag as username; only Xbox linked | P1 |  |
| OA-45 | resolveOrCreateAccountForMicrosoftUser | Happy Path | Xbox already linked, Java absent | Xbox lookup returns a link; `java == nil` | Returns the existing Xbox account via `GetAccountByID`; no new account, no new links | P1 |  |
| OA-46 | resolveOrCreateAccountForMicrosoftUser | Happy Path | Java already linked, Xbox not yet linked | Java lookup returns a link; Xbox lookup `ErrNotFound` | Resolves via the Java account, then links the Xbox identity to it | P1 |  |
| OA-47 | resolveOrCreateAccountForMicrosoftUser | Edge Case | Both Xbox and Java already linked to the same account | Both lookups return links with the same `UserID` | Returns that account, err nil; no `ensureMicrosoftIdentityLinked` calls (both account IDs already resolved) | P1 |  |
| OA-48 | resolveOrCreateAccountForMicrosoftUser | Error Path | Xbox and Java linked to two different accounts | Lookups return links with different `UserID`s | Returns `ErrConflictingMicrosoftIdentities` | P0 |  |
| OA-49 | resolveOrCreateAccountForMicrosoftUser | Error Path | Linked identity/identities exist but none are eligible (verified+enabled) | e.g. Xbox linked with `LoginEnabled:false`, Java absent | Returns `ErrPlatformLoginDisabled` | P1 |  |
| OA-50 | resolveOrCreateAccountForMicrosoftUser | Edge Case | One identity ineligible but the other is eligible | Xbox linked+disabled, Java linked+eligible | Login succeeds (no `ErrPlatformLoginDisabled`); resolves via the eligible identity's account | P1 |  |
| OA-51 | resolveOrCreateAccountForMicrosoftUser | Error Path | Xbox lookup fails with a non-`ErrNotFound` error | Mock returns generic error | Error propagated | P2 |  |
| OA-52 | resolveOrCreateAccountForMicrosoftUser | Error Path | Java lookup fails with a non-`ErrNotFound` error | Mock returns generic error | Error propagated | P2 |  |
| OA-53 | resolveOrCreateAccountForMicrosoftUser | Error Path | `as.AddAccount` fails when creating the new placeholder | Mock `AddAccount` returns error | Error propagated | P2 |  |
| OA-54 | resolveOrCreateAccountForMicrosoftUser | Error Path | `as.GetAccountByID` fails for an already-resolved account ID | Mock returns error | Error propagated | P2 |  |
| OA-55 | resolveOrCreateAccountForMicrosoftUser | Error Path | `ensureMicrosoftIdentityLinked` fails while linking the missing identity | Mock `AddLinkedAccountToDB` fails (non-`ErrAlreadyLinked`) for the pending identity | Error propagated | P2 |  |
| OA-56 | ensureMicrosoftIdentityLinked | Happy Path | Link succeeds on first try | `AddLinkedAccountToDB` returns nil | Returns `(a, false, nil)` | P1 |  |
| OA-57 | ensureMicrosoftIdentityLinked | Error Path | Link fails (non-`ErrAlreadyLinked`), `isNewAccount=true`, cleanup succeeds | Mock returns generic error; `DeleteAccount` succeeds | Returns `(nil, false, originalErr)`; `DeleteAccount` called | P0 |  |
| OA-58 | ensureMicrosoftIdentityLinked | Error Path | Same, `isNewAccount=false` | No cleanup attempted | Returns `(nil, false, originalErr)` directly; `DeleteAccount` never called | P1 |  |
| OA-59 | ensureMicrosoftIdentityLinked | Error Path | Same as OA-57, but cleanup `DeleteAccount` also fails | Both calls fail | Returns a wrapped error naming both failures | P1 |  |
| OA-84 | ensureMicrosoftIdentityLinked | Error Path | Same as OA-59, asserting both causes | `AddLinkedAccountToDB` fails with `testerrors.ErrInsertFailed`; `DeleteAccount` fails with `testerrors.ErrBoom` | Returns an error wrapping both `testerrors.ErrInsertFailed` and `testerrors.ErrBoom` | P2 | |
| OA-60 | ensureMicrosoftIdentityLinked | Edge Case | `ErrAlreadyLinked`, but re-fetch shows we already own it | Re-fetched linked account's `UserID == a.UserID` | Returns `(a, false, nil)`, no error | P0 |  |
| OA-61 | ensureMicrosoftIdentityLinked | Error Path | `ErrAlreadyLinked`, re-fetch lookup fails (non-`ErrNotFound`), `isNewAccount=true` | Cleanup succeeds | Returns cleanup+lookup wrapped/propagated error | P2 |  |
| OA-62 | ensureMicrosoftIdentityLinked | Error Path | Same as OA-61, but cleanup also fails | Both fail | Returns an error wrapping both `testerrors.ErrDBDown` (the re-fetch failure) and `testerrors.ErrBoom` (the cleanup failure) | P2 |  |
| OA-63 | ensureMicrosoftIdentityLinked | Error Path | `ErrAlreadyLinked`, real conflict (`isNewAccount=false`) | Re-fetched owner differs from `a.UserID` | Returns `ErrConflictingMicrosoftIdentities` | P0 |  |
| OA-64 | ensureMicrosoftIdentityLinked | Happy Path | `ErrAlreadyLinked`, real conflict but `isNewAccount=true` (placeholder loses the race) | Cleanup `DeleteAccount` succeeds; `GetAccountByID(winner)` succeeds | Returns `(winner, false, nil)` | P0 |  |
| OA-65 | ensureMicrosoftIdentityLinked | Error Path | Same as OA-64, but cleanup fails | `DeleteAccount` fails | Returns an error wrapping both `auth.ErrAlreadyLinked` and `testerrors.ErrBoom` (the cleanup failure) | P1 |  |
| OA-66 | ensureMicrosoftIdentityLinked | Error Path | Same as OA-64, but `GetAccountByID(winner)` fails after successful cleanup | Mock returns error | Error propagated | P2 |  |
| OA-67 | linkPlatformUserToSession | Happy Path | Identity not yet linked | `GetLinkedAccountByPlatformID` → `ErrNotFound`; `AddLinkedAccountToDB` succeeds | Returns nil error; link written | P1 |  |
| OA-68 | linkPlatformUserToSession | Edge Case | Identity already linked to the same `userID` | Mock returns a link with matching `UserID` | Returns nil error; `AddLinkedAccountToDB` never called (no-op) | P1 |  |
| OA-69 | linkPlatformUserToSession | Error Path | Identity already linked to a different `userID` | Mock returns a link with a different `UserID` | Returns `ErrPlatformAlreadyLinkedToDifferentAccount` | P0 |  |
| OA-70 | linkPlatformUserToSession | Error Path | Lookup fails with a non-`ErrNotFound` error | Mock returns generic error | Error propagated | P2 |  |
| OA-71 | linkPlatformUserToSession | Error Path | `AddLinkedAccountToDB` fails | Mock returns error | Returns an error wrapping both `ErrLinkAccountFailed` and the store failure (`testerrors.ErrInsertFailed`) | P2 |  |
| OA-85 | linkPlatformUserToSession | Error Path | `AddLinkedAccountToDB` fails with `auth.ErrAlreadyLinked` | Mock returns `auth.ErrAlreadyLinked` | Returns an error wrapping both `ErrLinkAccountFailed` and `auth.ErrAlreadyLinked` | P2 | |
| OA-72 | resolveOrCreateAccountForPlatformUser | Concurrency Invariant | Two concurrent callers race to resolve/create an account for the *same*, brand-new platform identity | Two goroutines call the function simultaneously against a shared, mutex-serialized store whose `AddLinkedAccountToDB` enforces the real `(platform, platform_id)` uniqueness constraint (returning `auth.ErrAlreadyLinked` to whichever call loses) | Both goroutines return without error and resolve to the *same* account (`UserID` equal); exactly one account row exists afterward — the loser's placeholder was cleaned up, not left orphaned | P0 |  |
| OA-73 | ensureMicrosoftIdentityLinked | Concurrency Invariant | Two concurrent callers, each holding its own freshly created placeholder (`isNewAccount=true`), race to link the *same* identity | Two goroutines call the function simultaneously with distinct placeholder accounts and the same identity, against the same race-serializing store | Both goroutines return without error; both resolve to the *same* account; exactly one account row exists afterward (the loser's placeholder is deleted, per the `isNewAccount` cleanup path) | P0 |  |
| OA-74 | ensureMicrosoftIdentityLinked | Concurrency Invariant | Partial-state race: a caller resolving an *existing* (non-placeholder) account (`isNewAccount=false`) races a concurrent caller with a *fresh* placeholder (`isNewAccount=true`) for the same identity | Two goroutines call the function simultaneously — one with an existing account, one with a brand-new placeholder — for the same, not-yet-linked identity | Whichever call loses is handled according to its own `isNewAccount`: if the existing-account call loses, it gets `ErrConflictingMicrosoftIdentities` and its account is *never* deleted; if the placeholder call loses, it is cleaned up (the placeholder is then gone: `auth.ErrNotFound` from `GetAccountByID`) and the pair converges on the existing account. In both outcomes the existing (non-placeholder) account is never deleted | P0 |  |
| OA-75 | resolveOrCreateAccountForMicrosoftUser | Concurrency Invariant | Two concurrent callers perform a full brand-new Microsoft login (neither Xbox nor Java identity linked yet) for the *same* Xbox+Java identity pair | Two goroutines call the function simultaneously with the same `xbox`/`java` identities, against the same race-serializing store | Both goroutines return without error and resolve to the *same* account, with both identities linked to it; exactly one account row exists afterward | P0 |  |
| OA-76 | ProcessOAuthLogin | Happy Path | Twitch platform, brand-new platform identity (`case auth.PlatformTwitch: config = twitch.Config` dispatch, distinct from Discord's) | `twitch.Config` + Twitch `/users` endpoint point at httptest servers; no existing link | Returns a non-nil session, err nil; `AddAccount` and `AddLinkedAccountToDB` each called once, with the linked account's `Platform == auth.PlatformTwitch` | P1 |  |
| OA-77 | ProcessOAuthLogin | Happy Path | Microsoft (plain, non-Xbox) platform, brand-new platform identity (`case auth.PlatformMicrosoft: config = MicrosoftLoginConfig` dispatch, distinct from Discord and from the Minecraft/XboxLive `MicrosoftConfig` chain) | `MicrosoftLoginConfig` + `microsoftUserInfoURL` point at httptest servers; no existing link | Returns a non-nil session, err nil; `AddAccount` and `AddLinkedAccountToDB` each called once, with the linked account's `Platform == auth.PlatformMicrosoft` | P1 |  |
| OA-78 | ProcessOAuthLogin | Edge Case | `state.Platform == auth.PlatformXboxLive` takes the `GetXboxUser` call (not `GetXboxAndMinecraftUser`) and never touches Java, even when the account owns it | Full Microsoft/XBL/XSTS chain succeeds; account owns Java (a valid Java profile would be returned if queried); `minecraftLoginURL`/`minecraftProfileURL` wired to fail the test if hit at all | Returns a non-nil session, err nil; the Minecraft Services login-with-xbox and profile endpoints are never called; `AddLinkedAccountToDB` called once, only for `auth.PlatformXboxLive` | P0 |  |
| OA-79 | ProcessOAuthLink | Happy Path | Twitch platform, valid session, identity not yet linked | Valid non-expired session in request context; `twitch.Config` + Twitch `/users` endpoint point at httptest servers; `AddLinkedAccountToDB` succeeds | Returns the same session, err nil; `AddLinkedAccountToDB` called once, with the linked account's `Platform == auth.PlatformTwitch` | P1 |  |
| OA-80 | ProcessOAuthLink | Happy Path | Microsoft (plain, non-Xbox) platform, valid session, identity not yet linked | Valid non-expired session in request context; `MicrosoftLoginConfig` + `microsoftUserInfoURL` point at httptest servers; `AddLinkedAccountToDB` succeeds | Returns the same session, err nil; `AddLinkedAccountToDB` called once, with the linked account's `Platform == auth.PlatformMicrosoft` | P1 |  |
| OA-81 | ProcessOAuthLink | Edge Case | `state.Platform == auth.PlatformXboxLive` takes the `GetXboxUser` call and never touches Java, even when the account owns it | Full Microsoft/XBL/XSTS chain succeeds; account owns Java; `minecraftLoginURL`/`minecraftProfileURL` wired to fail the test if hit at all | Returns the session, err nil; the Minecraft Services login-with-xbox and profile endpoints are never called; `AddLinkedAccountToDB` called once, only for `auth.PlatformXboxLive` | P0 |  |
| OA-82 | linkPlatformUserToSession | Concurrency Invariant | Two concurrent callers race to link the *same* platform identity to two *different* sessions (`userID`s) | Two goroutines call the function simultaneously against a store whose `GetLinkedAccountByPlatformID` holds both callers at a rendezvous barrier until each has independently observed `auth.ErrNotFound` (deterministically reproducing the check-then-act window), then whose `AddLinkedAccountToDB` enforces the real `(platform, platform_id)` uniqueness constraint, returning `auth.ErrAlreadyLinked` to whichever write loses | Exactly one goroutine returns nil error (the winner, whose write committed); the other returns a non-nil, observable error rather than silently succeeding, overwriting, or duplicating the link; exactly one linked-account row exists afterward, owned by whichever `userID` won | P0 | Currently the loser's error is the generic `ErrLinkAccountFailed` rather than a distinguishable sentinel, but it is never nil/swallowed - this row locks in that the race is still handled safely even though the specific cause is masked |

## discord.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| DC-01 | GetID | Accessor | Returns the underlying Discord user ID | `DiscordData{User: &discordgo.User{ID: "123"}}` | `GetID() == "123"` | P3 |  |
| DC-02 | GetUsername | Accessor | Returns the underlying username | `Username: "alice"` | `GetUsername() == "alice"` | P3 |  |
| DC-03 | GetEmail | Accessor | Returns the underlying email | `Email: "a@b.com"` | `GetEmail() == "a@b.com"` | P3 |  |
| DC-04 | GetData | Accessor | Returns a JSON marshal of the wrapped user | Any populated `DiscordData` | Returned string unmarshals back to an equivalent `discordgo.User` | P3 |  |
| DC-05 | CreateLinkedAccount | Happy Path | Builds a `*auth.LinkedAccount` for platform Discord | `DiscordData{ID: "123", Username: "alice"}` | Returned `LinkedAccount` has `Platform: PlatformDiscord`, `UserID`, `PlatformUsername: "alice"`, `PlatformID: "123"`, `Verified` and `LoginEnabled` true | P2 |  |
| DC-06 | GetDiscordUser | Happy Path | Discord's `/users/@me` returns a valid user | `discordgo.EndpointUsers` overridden to an httptest server returning a valid user JSON body | Returns `*DiscordData` wrapping the decoded user, err nil | P1 |  |
| DC-07 | GetDiscordUser | Error Path | Discord API returns a non-2xx status | Server returns HTTP 401 with a Discord-style error body | Returns nil, non-nil error | P1 |  |
| DC-08 | GetDiscordUser | Edge Case | Discord API returns malformed JSON | Server returns HTTP 200 with an invalid JSON body | Returns nil, non-nil error (unmarshal failure) | P2 |  |

## steam.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ST-01 | GetID | Accessor | Returns SteamID64 | `SteamData{SteamID64: "765..."}` | `GetID() == "765..."` | P3 |  |
| ST-02 | GetUsername | Accessor | Returns PersonaName | `PersonaName: "bob"` | `GetUsername() == "bob"` | P3 |  |
| ST-03 | GetEmail | Accessor | Always empty (Steam never exposes an email) | Any `SteamData` | `GetEmail() == ""` | P3 |  |
| ST-04 | GetData | Accessor | Returns a JSON marshal of the struct | Any populated `SteamData` | Returned string unmarshals back to an equivalent `SteamData` | P3 |  |
| ST-05 | CreateLinkedAccount | Happy Path | Builds a `*auth.LinkedAccount` for platform Steam | `SteamData{SteamID64: "765...", PersonaName: "bob"}` | `Platform: PlatformSteam`, `PlatformUsername: "bob"`, `PlatformID: "765..."` | P2 |  |
| ST-06 | VerifySteamOpenIDCallback | Happy Path | Fully valid OpenID assertion, Steam confirms it | `query` has `openid.mode=id_res`, a matching `claimed_id`, `openid.signed` containing `claimed_id`; `steamOpenIDLoginURL` httptest server responds `is_valid:true` | Returns the parsed SteamID64, err nil | P0 |  |
| ST-07 | VerifySteamOpenIDCallback | Error Path | `openid.mode` isn't `id_res` | `query.Set("openid.mode","cancel")` | Returns `ErrInvalidAssertion`-wrapped error, no HTTP call made | P1 |  |
| ST-08 | VerifySteamOpenIDCallback | Error Path | `claimed_id` missing or doesn't match the expected pattern | `openid.claimed_id` unset or garbage | Returns `ErrInvalidAssertion`-wrapped error | P1 |  |
| ST-09 | VerifySteamOpenIDCallback | Error Path | `openid.signed` doesn't cover `claimed_id` | `openid.signed = "op_endpoint,assoc_handle"` (no `claimed_id`) | Returns `ErrInvalidAssertion`-wrapped error | P0 |  |
| ST-10 | VerifySteamOpenIDCallback | Error Path | `check_authentication` request fails outright (network error) | `steamOpenIDLoginURL` points at an unreachable address | Returns a plain (non-`ErrInvalidAssertion`) error | P2 |  |
| ST-11 | VerifySteamOpenIDCallback | Error Path | `check_authentication` returns a non-2xx status | Server responds HTTP 500 | Returns an error wrapping `ErrSteamOpenIDCheck` (not `ErrInvalidAssertion`) | P2 |  |
| ST-12 | VerifySteamOpenIDCallback | Error Path | Steam rejects the assertion (`is_valid:false`/absent) | Server responds 200 with `is_valid:false` | Returns `ErrInvalidAssertion`-wrapped error | P0 |  |
| ST-13 | responseIsValid | Happy Path | Body contains an `is_valid:true` line | `[]byte("ns:foo\nis_valid:true\n")` | Returns true | P2 |  |
| ST-14 | responseIsValid | Edge Case | `is_valid` present but false, or key absent | `"is_valid:false"` and separately `"ns:foo"` alone | Returns false | P2 |  |
| ST-15 | responseIsValid | Edge Case | Empty body | `[]byte("")` | Returns false | P3 |  |
| ST-16 | GetSteamUser | Happy Path | API key set, player summary returned and ID matches | `STEAM_API_KEY` non-empty; `steamPlayerSummaryURL` httptest server returns one matching player | Returns populated `*SteamData`, err nil | P1 |  |
| ST-17 | GetSteamUser | Error Path | `STEAM_API_KEY` unset | `STEAM_API_KEY = ""` | Returns `ErrSteamAPIKeyUnset`; no HTTP call made | P1 |  |
| ST-18 | GetSteamUser | Error Path | HTTP call fails (network) | Server URL unreachable | Error propagated | P2 |  |
| ST-19 | GetSteamUser | Error Path | Non-2xx status | Server responds HTTP 500 | Returns an error wrapping `ErrSteamPlayerSummaryLookup` | P2 |  |
| ST-20 | GetSteamUser | Edge Case | Malformed JSON body | Server responds 200 with invalid JSON | Decode error propagated | P2 |  |
| ST-21 | GetSteamUser | Error Path | Empty players array | Server responds with `{"response":{"players":[]}}` | Returns `ErrSteamNoPlayers` | P1 |  |
| ST-22 | GetSteamUser | Error Path | Returned player's SteamID64 doesn't match requested | Server returns a player with a different `steamid` | Returns an error matching `ErrSteamIDMismatch` | P1 |  |
| ST-23 | ProcessSteamLogin | Happy Path | New Steam identity resolves and a session is created | Mocks: `GetLinkedAccountByPlatformID` → `ErrNotFound`, `AddAccount`/`AddLinkedAccountToDB`/`ss.AddSession` succeed | Returns a session, err nil | P1 |  |
| ST-24 | ProcessSteamLogin | Error Path | Account resolution fails | `resolveOrCreateAccountForPlatformUser` path fails (e.g. `AddAccount` error) | Error propagated, nil session | P2 |  |
| ST-25 | ProcessSteamLogin | Error Path | `ss.AddSession` fails | Mock `AddSession` returns error | Error propagated | P2 |  |
| ST-26 | ProcessSteamLink | Happy Path | Valid session, identity not yet linked | Session in request context valid; `AddLinkedAccountToDB` succeeds | Returns the same session, err nil | P1 |  |
| ST-27 | ProcessSteamLink | Error Path | No session in request context | Plain request, no context value | Returns `ErrSessionNotFound` | P1 |  |
| ST-28 | ProcessSteamLink | Error Path | Session expired | `session.IsValid()` false | Returns `ErrSessionExpired` | P1 |  |
| ST-29 | ProcessSteamLink | Error Path | Identity already linked to a different account | `linkPlatformUserToSession` returns `ErrPlatformAlreadyLinkedToDifferentAccount` | Error propagated | P2 |  |

## microsoft.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| MS-01 | (XboxLiveData) GetID | Accessor | Returns XUID | `XboxLiveData{XUID:"x1"}` | `GetID() == "x1"` | P3 |  |
| MS-02 | (XboxLiveData) GetEmail | Accessor | Always empty | Any value | `GetEmail() == ""` | P3 |  |
| MS-03 | (XboxLiveData) GetUsername | Accessor | Returns Gamertag | `Gamertag:"Tag"` | `GetUsername() == "Tag"` | P3 |  |
| MS-04 | (XboxLiveData) GetData | Accessor | JSON marshal round-trips | Populated struct | Unmarshals back equivalently | P3 |  |
| MS-05 | (XboxLiveData) CreateLinkedAccount | Happy Path | Builds LinkedAccount for PlatformXboxLive | `XboxLiveData{XUID:"x1",Gamertag:"Tag"}` | `Platform: PlatformXboxLive`, `PlatformID:"x1"`, `PlatformUsername:"Tag"` | P2 |  |
| MS-06 | (MicrosoftUserData) GetID | Accessor | Returns Sub | `Sub:"sub1"` | `GetID() == "sub1"` | P3 |  |
| MS-07 | (MicrosoftUserData) GetEmail | Accessor | Returns Email | `Email:"a@b.com"` | `GetEmail() == "a@b.com"` | P3 |  |
| MS-08 | (MicrosoftUserData) GetUsername | Accessor | Returns Name | `Name:"Alice"` | `GetUsername() == "Alice"` | P3 |  |
| MS-09 | (MicrosoftUserData) GetData | Accessor | JSON marshal round-trips | Populated struct | Unmarshals back equivalently | P3 |  |
| MS-10 | (MicrosoftUserData) CreateLinkedAccount | Happy Path | Builds LinkedAccount for PlatformMicrosoft | `MicrosoftUserData{Sub:"sub1",Name:"Alice"}` | `Platform: PlatformMicrosoft`, `PlatformID:"sub1"`, `PlatformUsername:"Alice"` | P2 |  |
| MS-11 | xstsErrForCode | Edge Case | Code 2148916233 | n/a | Returns `ErrNoXboxAccount` | P2 |  |
| MS-12 | xstsErrForCode | Edge Case | Code 2148916235 | n/a | Returns `ErrXboxLiveUnavailable` | P2 |  |
| MS-13 | xstsErrForCode | Edge Case | Code 2148916236 | n/a | Returns `ErrAdultVerificationRequired` | P2 |  |
| MS-14 | xstsErrForCode | Edge Case | Code 2148916237 | n/a | Returns `ErrAgeVerificationRequired` | P2 |  |
| MS-15 | xstsErrForCode | Edge Case | Code 2148916238 | n/a | Returns `ErrAccountIsChild` | P2 |  |
| MS-16 | xstsErrForCode | Edge Case | Unrecognized code | e.g. `999` | Returns an error wrapping `ErrXSTSAuthentication` | P3 |  |
| MS-17 | GetMicrosoftUser | Happy Path | Userinfo endpoint returns a valid identity | `microsoftUserInfoURL` httptest server returns 200 + `{"sub":"s1","name":"n","email":"e"}` | Returns populated `*MicrosoftUserData`, err nil | P1 |  |
| MS-18 | GetMicrosoftUser | Error Path | HTTP call fails | Server unreachable | Error propagated | P2 |  |
| MS-19 | GetMicrosoftUser | Error Path | Non-200 status | Server responds 401 | Returns an error wrapping `ErrMicrosoftUserinfoLookup` | P1 |  |
| MS-20 | GetMicrosoftUser | Edge Case | Malformed JSON | Server responds 200 with invalid JSON | Decode error propagated | P2 |  |
| MS-21 | GetMicrosoftUser | Error Path | Response missing `sub` | Server responds 200 with `{"name":"n"}` | Returns `ErrUserinfoMissingSub` | P1 |  |
| MS-22 | xblAuthenticate | Happy Path | Endpoint returns a Token | `xboxLiveAuthenticateURL` server returns 200 + `{"Token":"t"}` | Returns `"t"`, err nil | P1 |  |
| MS-23 | xblAuthenticate | Error Path | HTTP call fails | Server unreachable | Error propagated | P2 |  |
| MS-24 | xblAuthenticate | Error Path | Non-200 status | Server responds 401 | Returns an error wrapping `ErrXboxLiveAuthentication` | P1 |  |
| MS-25 | xblAuthenticate | Edge Case | Malformed JSON | Server responds 200 with invalid JSON | Decode error propagated | P2 |  |
| MS-26 | xblAuthenticate | Error Path | Response missing `Token` | Server responds 200 with `{}` | Returns `ErrXboxLiveMissingToken` | P1 |  |
| MS-27 | xstsAuthorize | Happy Path | Xbox Live relying party succeeds | `xstsAuthorizeURL` server returns 200 + Token/Uhs/Xid/Gtg | Returns token, userHash, xuid, gamertag all populated, err nil | P1 |  |
| MS-28 | xstsAuthorize | Edge Case | Minecraft relying party succeeds (no xid/gtg) | Server returns Token/Uhs only, no Xid/Gtg | Returns userHash populated, xuid/gamertag empty, err nil | P1 |  |
| MS-29 | xstsAuthorize | Error Path | Response carries a nonzero `XErr` | Server returns 200 with `"XErr":2148916233` | Returns `ErrNoXboxAccount` (via `xstsErrForCode`), even though HTTP status is 200 | P0 |  |
| MS-30 | xstsAuthorize | Error Path | Non-200 status, `XErr` zero | Server responds 500, no `XErr` | Returns an error wrapping `ErrXSTSAuthorization` | P1 |  |
| MS-31 | xstsAuthorize | Edge Case | Malformed JSON body, status 200 | Server returns invalid JSON | Decode error surfaced after the status/XErr checks | P2 |  |
| MS-32 | xstsAuthorize | Error Path | Missing Token or empty DisplayClaims.Xui | Server returns 200 with `{"Token":""}` | Returns `ErrXSTSMissingTokenOrClaims` | P1 |  |
| MS-33 | xstsAuthorize | Error Path | DisplayClaims.Xui[0].Uhs empty | Server returns Token+Xui entry with empty `uhs` | Returns `ErrXSTSMissingUHS` | P1 |  |
| MS-34 | xstsAuthorize | Error Path | HTTP call fails | Server unreachable | Error propagated | P2 |  |
| MS-35 | minecraftLoginWithXbox | Happy Path | Endpoint returns an access_token | `minecraftLoginURL` server returns 200 + `{"access_token":"m"}` | Returns `"m"`, err nil | P1 |  |
| MS-36 | minecraftLoginWithXbox | Error Path | HTTP call fails | Server unreachable | Error propagated | P2 |  |
| MS-37 | minecraftLoginWithXbox | Error Path | Non-200 status | Server responds 403 with a body | Returns an error wrapping `ErrMinecraftLoginWithXbox` | P1 |  |
| MS-38 | minecraftLoginWithXbox | Edge Case | Malformed JSON | Server responds 200 with invalid JSON | Decode error propagated | P2 |  |
| MS-39 | minecraftLoginWithXbox | Error Path | Missing `access_token` | Server responds 200 with `{}` | Returns `ErrLoginMissingAccessToken` | P1 |  |
| MS-40 | getMinecraftProfile | Happy Path | Valid profile returned | `minecraftProfileURL` server returns 200 + valid id/name/skins/capes JSON | Returns `*MinecraftData` with parsed UUID, err nil | P1 |  |
| MS-41 | getMinecraftProfile | Edge Case | HTTP 404 (no Java ownership) | Server responds 404 | Returns `(nil, nil)` — not an error | P0 |  |
| MS-42 | getMinecraftProfile | Edge Case | HTTP 200 with `"error":"NOT_FOUND"` body | Server responds 200 with that error field | Returns `(nil, nil)` | P0 |  |
| MS-43 | getMinecraftProfile | Error Path | HTTP call fails | Server unreachable | Error propagated | P2 |  |
| MS-44 | getMinecraftProfile | Error Path | Non-200, non-404 status | Server responds 500 | Returns an error wrapping `ErrMinecraftProfileLookup` | P1 |  |
| MS-45 | getMinecraftProfile | Edge Case | Malformed JSON, status 200 | Server returns invalid JSON | Decode error propagated | P2 |  |
| MS-46 | getMinecraftProfile | Error Path | Profile `id` isn't a valid UUID | Server returns `{"id":"not-a-uuid","name":"n"}` | Returns wrapped UUID-parse error | P1 |  |
| MS-47 | authenticateXboxLiveIdentity | Happy Path | XSTS authorize succeeds with xuid+gamertag | `xstsAuthorize` (via httptest) returns both | Returns `*XboxLiveData`, err nil | P1 |  |
| MS-48 | authenticateXboxLiveIdentity | Error Path | `xstsAuthorize` fails | Server error | Error propagated | P2 |  |
| MS-49 | authenticateXboxLiveIdentity | Error Path | xuid or gamertag empty despite success | Server returns Token/Uhs but empty Xid/Gtg for the XboxLive RP | Returns `ErrXSTSMissingXIDOrGTG` | P1 |  |
| MS-50 | GetXboxUser | Happy Path | Full XBL auth + XSTS authorize succeed | Both httptest legs succeed | Returns `*XboxLiveData`, err nil | P1 |  |
| MS-51 | GetXboxUser | Error Path | `xblAuthenticate` fails | XBL endpoint fails | Error propagated; XSTS endpoint never called | P2 |  |
| MS-52 | GetXboxAndMinecraftUser | Happy Path | Full chain succeeds, Java owned | All 4 legs (XBL, XSTS-minecraft, mc-login, mc-profile) plus XSTS-xboxlive succeed | Returns `(xbox, java, nil)` both populated | P0 |  |
| MS-53 | GetXboxAndMinecraftUser | Edge Case | Full chain succeeds, Java not owned | `getMinecraftProfile` leg returns 404 | Returns `(xbox, nil, nil)` — not an error | P0 |  |
| MS-54 | GetXboxAndMinecraftUser | Error Path | `xblAuthenticate` fails | XBL endpoint fails | Returns `(nil, nil, err)`; no further legs called | P1 |  |
| MS-55 | GetXboxAndMinecraftUser | Error Path | Minecraft-scoped `xstsAuthorize` fails | That leg fails | Returns `(nil, nil, err)` | P1 |  |
| MS-56 | GetXboxAndMinecraftUser | Edge Case | `minecraftLoginWithXbox` fails, but the Xbox Live identity still resolves | mc-login leg fails; XSTS-xboxlive leg succeeds | Returns `(xbox, nil, minecraftErr)` — xbox is not discarded | P0 |  |
| MS-57 | GetXboxAndMinecraftUser | Error Path | `minecraftLoginWithXbox` fails AND the Xbox Live identity fetch also fails | Both mc-login and XSTS-xboxlive legs fail | Returns `(nil, nil, xboxErr)` — the Xbox Live error takes precedence | P1 |  |
| MS-58 | GetXboxAndMinecraftUser | Edge Case | `getMinecraftProfile` fails after a successful login-with-xbox, Xbox identity still resolves | mc-profile leg fails; XSTS-xboxlive succeeds | Returns `(xbox, nil, minecraftErr)` | P1 |  |

## minecraft.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| MC-01 | GetID | Accessor | Returns the UUID's string form | `MinecraftData{ID: u}` | `GetID() == u.String()` | P3 |  |
| MC-02 | GetEmail | Accessor | Always empty | Any value | `GetEmail() == ""` | P3 |  |
| MC-03 | GetUsername | Accessor | Returns Username | `Username:"Steve"` | `GetUsername() == "Steve"` | P3 |  |
| MC-04 | GetData | Accessor | JSON marshal round-trips | Populated struct incl. Skins/Capes | Unmarshals back equivalently | P3 |  |
| MC-05 | CreateLinkedAccount | Happy Path | Builds LinkedAccount for PlatformMinecraft | `MinecraftData{ID:u, Username:"Steve"}` | `Platform: PlatformMinecraft`, `PlatformID: u.String()`, `PlatformUsername:"Steve"` | P2 |  |
