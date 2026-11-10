# Test plan: modules/auth/link

## oauth.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| OA-01 | ExtCodeForToken | Happy Path | Code exchange succeeds, token carries `scope` as a `[]interface{}` of strings | The provider's token endpoint is served by a test server returning a valid token JSON body with `"scope":["identify","email"]` | Returns an OAuth token with the access token, token type and refresh token copied and `Scope == []string{"identify","email"}`; err nil | P1 |  |
| OA-02 | ExtCodeForToken | Edge Case | Token carries `scope` as a bare string (Discord's single-scope shape) | Server returns `"scope":"identify"` | `Scope == []string{"identify"}`; err nil | P2 |  |
| OA-03 | ExtCodeForToken | Error Path | Token endpoint rejects the code | Server returns HTTP 400 with an OAuth2 error body | The provider's rejection error is returned, token nil | P1 |  |
| OA-04 | ExtCodeForToken | Error Path | Token response omits `scope` entirely | Server returns a valid token JSON with no `scope` field | Returns `ErrNoScopeInToken`, token nil | P2 |  |
| OA-05 | RefreshToken | Happy Path | Refresh succeeds, `[]interface{}` scope | The provider's token endpoint is served by a test server returning a fresh token with an array scope, for an existing token | Returns the refreshed token with its array scope; err nil | P1 |  |
| OA-06 | RefreshToken | Edge Case | Refresh succeeds, bare-string scope | Server returns `"scope":"identify"` | `Scope == []string{"identify"}`; err nil | P2 |  |
| OA-07 | RefreshToken | Error Path | Refresh endpoint failure | Server returns HTTP 400 | Error returned, token nil | P1 |  |
| OA-08 | RefreshToken | Error Path | Refreshed token omits `scope` | Server returns valid token JSON without `scope` | Returns `ErrNoScopeInToken` | P2 |  |
| OA-09 | ProcessOAuthLogin | Happy Path | Discord platform, brand-new platform identity | The Discord login configuration and Discord's user endpoint are served by test servers; no linked account exists for the Discord identity; creating the account, storing the linked account and creating the session all succeed | Returns a non-nil session carrying the permissions the account service built, err nil; the account and the linked account are each stored once and the session is created once | P1 |  |
| OA-10 | ProcessOAuthLogin | Happy Path | Discord platform, existing verified+login-enabled linked account | Looking up the linked account returns one that is verified and login-enabled | Returns a session for the existing account, loaded by the linked account's user; no account is created | P1 |  |
| OA-11 | ProcessOAuthLogin | Error Path | Unknown/unsupported platform in state | `state.Platform = "bogus"` | Returns `ErrInvalidPlatform`; no store or network calls made | P2 |  |
| OA-12 | ProcessOAuthLogin | Error Path | Code exchange fails | Token endpoint returns HTTP 400 | The token exchange error is propagated; no account or linked account is stored | P2 |  |
| OA-13 | ProcessOAuthLogin | Error Path | Linked account exists but is unverified or login-disabled | Looking up the linked account returns one that is unverified (and separately one with login disabled) | Returns `ErrPlatformLoginDisabled` | P1 |  |
| OA-14 | ProcessOAuthLogin | Happy Path | Minecraft/XboxLive platform, full XBL+Java chain succeeds for a brand-new account | The Microsoft login configuration and the provider's Xbox Live, XSTS and Minecraft endpoints are served by test servers producing a valid Xbox and Java identity; no existing links | Returns a session; both the Xbox and Java identities are linked (two linked accounts stored) | P1 |  |
| OA-15 | ProcessOAuthLogin | Error Path | Minecraft/XboxLive platform, Xbox/Minecraft identity fetch fails | XSTS endpoint returns a failure | Error propagated, no session created, no store calls | P2 |  |
| OA-16 | ProcessOAuthLogin | Error Path | Creating the session fails after the account is resolved | Creating the session returns an error | Error propagated, nil session | P2 |  |
| OA-17 | resolveOrCreateAccountForPlatformUser | Happy Path | No existing linked account | No linked account is found for the platform identity; creating the account and storing the linked account succeed | Returns the newly created account, err nil | P1 |  |
| OA-18 | resolveOrCreateAccountForPlatformUser | Happy Path | Existing verified+enabled linked account | Looking up the linked account returns one that is verified and login-enabled | Returns the account loaded by the linked account's user ID | P1 |  |
| OA-19 | resolveOrCreateAccountForPlatformUser | Error Path | Existing linked account is disabled for login | Looking up the linked account returns one that is unverified or login-disabled | Returns `ErrPlatformLoginDisabled` | P1 |  |
| OA-20 | resolveOrCreateAccountForPlatformUser | Error Path | Lookup fails with a non-`ErrNotFound` error | Looking up the linked account fails with a generic database error | Error propagated | P2 |  |
| OA-21 | resolveOrCreateAccountForPlatformUser | Error Path | Creating the account fails | Creating the account returns an error | Error propagated; the linked account is never stored | P2 |  |
| OA-22 | resolveOrCreateAccountForPlatformUser | Error Path | Storing the linked account fails with an error other than `ErrAlreadyLinked`, and deleting the new account succeeds | Storing the linked account returns a generic error; deleting the account succeeds | Returns the original error; the new account is deleted once | P0 |  |
| OA-23 | resolveOrCreateAccountForPlatformUser | Error Path | Same as OA-22, but deleting the account also fails | Storing the linked account and deleting the account both return errors | Returns the cleanup error | P1 |  |
| OA-24 | resolveOrCreateAccountForPlatformUser | Edge Case | Storing the linked account fails with `ErrAlreadyLinked` (lost the race); cleanup and winner re-fetch succeed | Storing returns `link.ErrAlreadyLinked`, deleting the account succeeds, and the winner's linked account and account are found | Returns the winner's account, err nil | P0 |  |
| OA-25 | resolveOrCreateAccountForPlatformUser | Error Path | Lost the race, cleanup succeeds, but re-fetching the winner's link fails | The second lookup of the linked account returns an error | Error propagated | P2 |  |
| OA-26 | resolveOrCreateAccountForPlatformUser | Error Path | Lost the race, winner link found, but loading the winner's account fails | Loading the winner's account returns an error | Error propagated | P2 |  |
| OA-27 | ProcessOAuthLink | Happy Path | Discord platform, valid session, identity not yet linked | A valid non-expired session is in the request context; no linked account exists for the identity; storing the linked account succeeds | Returns the same session, err nil; the identity is linked once | P1 |  |
| OA-30 | ProcessOAuthLink | Error Path | Unknown/unsupported platform | `state.Platform = "bogus"` | Returns `ErrInvalidPlatform` | P2 |  |
| OA-31 | ProcessOAuthLink | Error Path | Code exchange fails | Token endpoint returns HTTP 400 | Error propagated | P2 |  |
| OA-32 | ProcessOAuthLink | Happy Path | Minecraft/XboxLive platform, both Xbox and Java identities new | Full XBL/XSTS/Minecraft chain succeeds; neither identity linked yet | Returns session; both identities are linked (two linked accounts stored) | P1 |  |
| OA-33 | ProcessOAuthLink | Happy Path | Minecraft/XboxLive platform, account doesn't own Java Edition | Chain succeeds with `java == nil` | Returns session; only the Xbox identity is linked (one linked account stored) | P1 |  |
| OA-34 | ProcessOAuthLink | Error Path | Xbox identity already linked to a different account | The lookup for the Xbox Live identity returns a link owned by a different user | Returns `ErrPlatformAlreadyLinkedToDifferentAccount`; no linked account is stored | P0 |  |
| OA-35 | ProcessOAuthLink | Error Path | Java identity already linked to a different account (Xbox check passes) | Xbox check clean; the Java check returns a link owned by a different user | Returns `ErrPlatformAlreadyLinkedToDifferentAccount`; nothing is stored, so the Xbox account is not linked either | P0 |  |
| OA-36 | ProcessOAuthLink | Error Path | Xbox pre-link-check lookup fails with a non-`ErrNotFound` error | The lookup returns a generic error | Error propagated; nothing linked | P2 |  |
| OA-37 | ProcessOAuthLink | Error Path | Java pre-link-check lookup fails with a non-`ErrNotFound` error | The lookup returns a generic error | Error propagated; nothing linked | P2 |  |
| OA-38 | ProcessOAuthLink | Edge Case | Xbox and Java both already linked to the *same* session's account | Both lookups return links owned by the caller's account | Returns session, err nil; linking is a no-op (nothing is stored) | P1 |  |
| OA-39 | ProcessOAuthLink | Error Path | Minecraft/XboxLive platform, identity fetch itself fails | XSTS endpoint fails | Error propagated; no link-store calls at all | P2 |  |
| OA-40 | ProcessOAuthLink | Error Path | Xbox link write fails in the Minecraft/XboxLive branch | Storing the Xbox linked account fails | Error propagated; Java link is never attempted | P2 | Any failure to store a linked account is reported as `ErrLinkAccountFailed` (see OA-71) |
| OA-41 | ProcessOAuthLink | Error Path | Non-minecraft platform, platform user fetch fails | Fetching the Discord or Microsoft user from the provider fails | Error propagated | P2 |  |
| OA-42 | ProcessOAuthLink | Error Path | Non-minecraft platform, final link write fails | Linking the identity to the session fails (e.g. the identity is linked elsewhere) | Error propagated | P2 |  |
| OA-43 | resolveOrCreateAccountForMicrosoftUser | Happy Path | Neither identity linked yet, Java present | Both lookups `ErrNotFound`; `java != nil` | A new account is created using Java's username (preferred over the gamertag); both identities are linked to it | P0 |  |
| OA-44 | resolveOrCreateAccountForMicrosoftUser | Happy Path | Neither identity linked yet, Java absent (Xbox-only login) | Both lookups `ErrNotFound`; `java == nil` | New account created using the Xbox gamertag as username; only Xbox linked | P1 |  |
| OA-45 | resolveOrCreateAccountForMicrosoftUser | Happy Path | Xbox already linked, Java absent | Xbox lookup returns a link; `java == nil` | Returns the existing Xbox account, loaded by its ID; no new account, no new links | P1 |  |
| OA-46 | resolveOrCreateAccountForMicrosoftUser | Happy Path | Java already linked, Xbox not yet linked | Java lookup returns a link; Xbox lookup `ErrNotFound` | Resolves via the Java account, then links the Xbox identity to it | P1 |  |
| OA-47 | resolveOrCreateAccountForMicrosoftUser | Edge Case | Both Xbox and Java already linked to the same account | Both lookups return links with the same `UserID` | Returns that account, err nil; no linking is attempted (both account IDs already resolved) | P1 |  |
| OA-48 | resolveOrCreateAccountForMicrosoftUser | Error Path | Xbox and Java linked to two different accounts | Lookups return links with different `UserID`s | Returns `ErrConflictingMicrosoftIdentities` | P0 |  |
| OA-49 | resolveOrCreateAccountForMicrosoftUser | Error Path | Linked identity/identities exist but none are eligible (verified+enabled) | e.g. Xbox linked with login disabled, Java absent | Returns `ErrPlatformLoginDisabled` | P1 |  |
| OA-50 | resolveOrCreateAccountForMicrosoftUser | Edge Case | One identity ineligible but the other is eligible | Xbox linked+disabled, Java linked+eligible | Login succeeds (no `ErrPlatformLoginDisabled`); resolves via the eligible identity's account | P1 |  |
| OA-51 | resolveOrCreateAccountForMicrosoftUser | Error Path | Xbox lookup fails with a non-`ErrNotFound` error | The lookup returns a generic error | Error propagated | P2 |  |
| OA-52 | resolveOrCreateAccountForMicrosoftUser | Error Path | Java lookup fails with a non-`ErrNotFound` error | The lookup returns a generic error | Error propagated | P2 |  |
| OA-53 | resolveOrCreateAccountForMicrosoftUser | Error Path | Creating the new placeholder account fails | Creating the account returns an error | Error propagated | P2 |  |
| OA-54 | resolveOrCreateAccountForMicrosoftUser | Error Path | Loading the account fails for an already-resolved account ID | Loading the account returns an error | Error propagated | P2 |  |
| OA-55 | resolveOrCreateAccountForMicrosoftUser | Error Path | Linking the missing identity fails | Storing the linked account fails (not `ErrAlreadyLinked`) for the pending identity | Error propagated | P2 |  |
| OA-56 | ensureMicrosoftIdentityLinked | Happy Path | Link succeeds on first try | Storing the linked account succeeds | Returns `(a, false, nil)` | P1 |  |
| OA-57 | ensureMicrosoftIdentityLinked | Error Path | Link fails (not `ErrAlreadyLinked`), the account was created for this login, cleanup succeeds | Storing the linked account returns a generic error; deleting the account succeeds | Returns the original error; the account is deleted once | P0 |  |
| OA-58 | ensureMicrosoftIdentityLinked | Error Path | Same, but the account existed before this login | No cleanup attempted | Returns the original error; the account is never deleted | P1 |  |
| OA-59 | ensureMicrosoftIdentityLinked | Error Path | Same as OA-57, but deleting the account also fails | Both calls fail | Returns the cleanup error | P1 |  |
| OA-60 | ensureMicrosoftIdentityLinked | Edge Case | `ErrAlreadyLinked`, but re-fetch shows we already own it | Re-fetched linked account's `UserID == a.UserID` | Returns `(a, false, nil)`, no error | P0 |  |
| OA-61 | ensureMicrosoftIdentityLinked | Error Path | `ErrAlreadyLinked`, the re-fetch lookup fails (not `ErrNotFound`), the account was created for this login | Cleanup succeeds | Returns the lookup error; the new account is deleted once | P2 |  |
| OA-62 | ensureMicrosoftIdentityLinked | Error Path | Same as OA-61, but cleanup also fails | Both fail | Returns the cleanup failure | P2 |  |
| OA-63 | ensureMicrosoftIdentityLinked | Error Path | `ErrAlreadyLinked`, real conflict (the account existed before this login) | The re-fetched owner differs from the caller's account | Returns `ErrConflictingMicrosoftIdentities` | P0 |  |
| OA-64 | ensureMicrosoftIdentityLinked | Happy Path | `ErrAlreadyLinked`, real conflict but the account was created for this login (placeholder loses the race) | Deleting the account succeeds; loading the winner's account succeeds | Returns `(winner, false, nil)` | P0 |  |
| OA-65 | ensureMicrosoftIdentityLinked | Error Path | Same as OA-64, but cleanup fails | Deleting the account fails | Returns the cleanup failure | P1 |  |
| OA-66 | ensureMicrosoftIdentityLinked | Error Path | Same as OA-64, but loading the winner's account fails | Loading the winner's account returns an error | Error propagated | P2 |  |
| OA-67 | linkPlatformUserToSession | Happy Path | Identity not yet linked | No linked account is found; storing the linked account succeeds | Returns nil error; link written | P1 |  |
| OA-68 | linkPlatformUserToSession | Edge Case | Identity already linked to the same user | The lookup returns a link with the matching user ID | Returns nil error; nothing is stored (no-op) | P1 |  |
| OA-69 | linkPlatformUserToSession | Error Path | Identity already linked to a different user | The lookup returns a link with a different user ID | Returns `ErrPlatformAlreadyLinkedToDifferentAccount` | P0 |  |
| OA-70 | linkPlatformUserToSession | Error Path | Lookup fails with a non-`ErrNotFound` error | The lookup returns a generic error | Error propagated | P2 |  |
| OA-71 | linkPlatformUserToSession | Error Path | Storing the linked account fails | Storing the linked account returns an error | Returns `ErrLinkAccountFailed` | P2 |  |
| OA-72 | resolveOrCreateAccountForPlatformUser | Concurrency Invariant | Two concurrent callers race to resolve/create an account for the *same*, brand-new platform identity | Two goroutines call the function simultaneously against a shared, mutex-serialized store whose insert of a linked account enforces the real platform and platform-ID uniqueness (returning `link.ErrAlreadyLinked` to whichever call loses) | Both goroutines return without error and resolve to the *same* account (`UserID` equal); exactly one account row exists afterward — the loser's placeholder was cleaned up, not left orphaned | P0 |  |
| OA-73 | ensureMicrosoftIdentityLinked | Concurrency Invariant | Two concurrent callers, each holding its own freshly created placeholder account, race to link the *same* identity | Two goroutines call the function simultaneously with distinct placeholder accounts and the same identity, against the same race-serializing store | Both goroutines return without error; both resolve to the *same* account; exactly one account row exists afterward (the loser's placeholder is deleted) | P0 |  |
| OA-74 | ensureMicrosoftIdentityLinked | Concurrency Invariant | Partial-state race: a caller resolving an account that existed before this login races a concurrent caller with a *fresh* placeholder for the same identity | Two goroutines call the function simultaneously — one with an existing account, one with a brand-new placeholder — for the same, not-yet-linked identity | Whichever call loses is handled according to whether its account was created for this login: if the existing-account call loses, it gets `ErrConflictingMicrosoftIdentities` and its account is *never* deleted; if the placeholder call loses, it is cleaned up (the placeholder is then gone: `account.ErrNotFound` when loading it) and the pair converges on the existing account. In both outcomes the existing (non-placeholder) account is never deleted | P0 |  |
| OA-75 | resolveOrCreateAccountForMicrosoftUser | Concurrency Invariant | Two concurrent callers perform a full brand-new Microsoft login (neither Xbox nor Java identity linked yet) for the *same* Xbox+Java identity pair | Two goroutines call the function simultaneously with the same Xbox and Java identities, against the same race-serializing store | Both goroutines return without error and resolve to the *same* account, with both identities linked to it; exactly one account row exists afterward | P0 |  |
| OA-76 | ProcessOAuthLogin | Happy Path | Twitch platform, brand-new platform identity (selects the Twitch login configuration, distinct from Discord's) | The Twitch login configuration and Twitch's users endpoint are served by test servers; no existing link | Returns a non-nil session, err nil; the account and the linked account are each stored once, and the linked account's platform is Twitch | P1 |  |
| OA-77 | ProcessOAuthLogin | Happy Path | Microsoft (plain, non-Xbox) platform, brand-new platform identity (selects the plain Microsoft login configuration, distinct from Discord and from the Minecraft/XboxLive chain) | The plain Microsoft login configuration and the provider's user info endpoint are served by test servers; no existing link | Returns a non-nil session, err nil; the account and the linked account are each stored once, and the linked account's platform is Microsoft | P1 |  |
| OA-78 | ProcessOAuthLogin | Edge Case | The XboxLive platform fetches only the Xbox identity and never touches Java, even when the account owns it | Full Microsoft/XBL/XSTS chain succeeds; account owns Java (a valid Java profile would be returned if queried); the Minecraft login and profile endpoints fail the test if they are hit at all | Returns a non-nil session, err nil; the Minecraft Services login-with-xbox and profile endpoints are never called; one linked account is stored, for Xbox Live only | P0 |  |
| OA-79 | ProcessOAuthLink | Happy Path | Twitch platform, valid session, identity not yet linked | A valid non-expired session is in the request context; the Twitch login configuration and Twitch's users endpoint are served by test servers; storing the linked account succeeds | Returns the same session, err nil; one linked account is stored, and its platform is Twitch | P1 |  |
| OA-80 | ProcessOAuthLink | Happy Path | Microsoft (plain, non-Xbox) platform, valid session, identity not yet linked | A valid non-expired session is in the request context; the plain Microsoft login configuration and the provider's user info endpoint are served by test servers; storing the linked account succeeds | Returns the same session, err nil; one linked account is stored, and its platform is Microsoft | P1 |  |
| OA-81 | ProcessOAuthLink | Edge Case | The XboxLive platform fetches only the Xbox identity and never touches Java, even when the account owns it | Full Microsoft/XBL/XSTS chain succeeds; account owns Java; the Minecraft login and profile endpoints fail the test if they are hit at all | Returns the session, err nil; the Minecraft Services login-with-xbox and profile endpoints are never called; one linked account is stored, for Xbox Live only | P0 |  |
| OA-82 | linkPlatformUserToSession | Concurrency Invariant | Two concurrent callers race to link the *same* platform identity to two *different* users | Two goroutines call the function simultaneously against a store whose lookup holds both callers at a rendezvous barrier until each has independently seen `account.ErrNotFound` (deterministically reproducing the check-then-act window), then whose insert enforces the real platform and platform-ID uniqueness, returning `link.ErrAlreadyLinked` to whichever write loses | Exactly one goroutine returns nil error (the winner, whose write committed); the other returns a non-nil, observable error rather than silently succeeding, overwriting, or duplicating the link; exactly one linked-account row exists afterward, owned by whichever user won | P0 | The loser's specific error is not asserted, only that it is non-nil. |
| OA-85 | linkPlatformUserToSession | Error Path | Storing the linked account fails with `ErrAlreadyLinked` and the winner row is gone on the re-read | Storing returns `link.ErrAlreadyLinked`; the re-read returns `account.ErrNotFound` | Returns `ErrLinkAccountFailed` | P2 | |
| OA-86 | ensureMicrosoftIdentityLinked | Edge Case | `ErrAlreadyLinked`, then the re-fetch returns `account.ErrNotFound` (the owner row vanished between the two calls), and the account existed before this login | Storing the linked account returns `ErrAlreadyLinked`; the owner lookup returns `account.ErrNotFound` | The lookup error is not returned; the empty owner ID differs from the caller's account, so it returns `ErrConflictingMicrosoftIdentities` | P2 |  |
| OA-87 | resolveOrCreateAccountForPlatformUser | Edge Case | The request context is cancelled, the account was created, and storing the linked account fails with an error other than `ErrAlreadyLinked` | Storing the linked account fails with a database error; the call runs with an already-cancelled request context that carries a marker value; each store call records whether the context is cancelled and whether the marker is present | Returns the link error; storing the linked account and deleting the account each run once with a context that is not cancelled and carries the marker | P1 |  |
| OA-88 | ensureMicrosoftIdentityLinked | Edge Case | The request context is cancelled and storing the linked account succeeds | Storing the linked account succeeds; the call runs with an already-cancelled request context that carries a marker value; each store call records whether the context is cancelled and whether the marker is present | Returns `(a, false, nil)`; storing the linked account runs once with a context that is not cancelled and carries the marker | P1 |  |
| OA-89 | ensureMicrosoftIdentityLinked | Edge Case | The request context is cancelled, the account was created for this login, and storing the linked account fails with an error other than `ErrAlreadyLinked` | Storing the linked account fails with a database error; the call runs with an already-cancelled request context that carries a marker value; each store call records whether the context is cancelled and whether the marker is present | Returns the link error; deleting the account runs once with a context that is not cancelled and carries the marker | P1 |  |
| OA-90 | ensureMicrosoftIdentityLinked | Edge Case | The request context is cancelled, the account was created for this login, `ErrAlreadyLinked`, then the owner lookup fails with an error other than `account.ErrNotFound` | Storing returns `ErrAlreadyLinked`; the owner lookup fails with a database outage; the call runs with an already-cancelled request context that carries a marker value; each store call records whether the context is cancelled and whether the marker is present | Returns the lookup error; deleting the account runs once with a context that is not cancelled and carries the marker | P1 |  |
| OA-91 | ensureMicrosoftIdentityLinked | Edge Case | The request context is cancelled, the account was created for this login, `ErrAlreadyLinked`, and the lookup returns a different owner's linked account | Storing returns `ErrAlreadyLinked`; the owner lookup returns a link owned by another account, which loads fine; the call runs with an already-cancelled request context that carries a marker value; each store call records whether the context is cancelled and whether the marker is present | Returns `(winner, false, nil)`; deleting the account runs once with a context that is not cancelled and carries the marker | P1 |  |
| OA-92 | linkPlatformUserToSession | Edge Case | The request context is cancelled, no linked account exists for the platform user, and the link insert succeeds | The lookup finds no linked account; storing the linked account succeeds; the call runs with an already-cancelled request context that carries a marker value; each store call records whether the context is cancelled and whether the marker is present | Returns nil; storing the linked account runs once with a context that is not cancelled and carries the marker | P1 |  |
| OA-93 | linkPlatformUserToSession | Edge Case | The insert loses a race (`ErrAlreadyLinked`) and the re-read shows the same user as owner | The first lookup finds no linked account and the re-read finds one owned by the caller; storing returns `link.ErrAlreadyLinked` | Returns nil | P1 | |
| OA-94 | linkPlatformUserToSession | Edge Case | The insert loses a race (`ErrAlreadyLinked`) and the re-read shows another user as owner | The first lookup finds no linked account and the re-read finds one owned by another user; storing returns `link.ErrAlreadyLinked` | Returns `ErrPlatformAlreadyLinkedToDifferentAccount` | P1 | |
| OA-95 | linkPlatformUserToSession | Error Path | The insert loses a race (`ErrAlreadyLinked`) and the re-read fails | The first lookup finds no linked account and the re-read fails with a database outage; storing returns `link.ErrAlreadyLinked` | Returns `ErrLinkAccountFailed` | P1 | |
| OA-96 | linkPlatformUserToSession | Concurrency Invariant | Two goroutines link the same platform identity through the in-memory store | Run once with the same user twice and once with two users | The same user gets nil twice; two users get exactly one nil and one `ErrPlatformAlreadyLinkedToDifferentAccount` | P1 | |
| OA-97 | ProcessOAuthLogin | Edge Case | Minecraft login and profile fail while the Xbox Live identity is linked to account A | Full chain with the Minecraft login endpoint returning 403; the lookup returns a verified, login-enabled Xbox link for account A | Returns a session for account A; no account or linked account is stored | P0 | |
| OA-98 | ProcessOAuthLogin | Error Path | Minecraft login fails and the Xbox Live identity is not linked | Full chain with the Minecraft login endpoint returning 403; no linked account is found for the Xbox Live identity | Returns an error wrapping `ErrMinecraftLoginWithXbox`; no account is created and no session is added | P0 | |
| OA-99 | ProcessOAuthLogin | Error Path | Minecraft login fails and the Xbox Live link lookup errors | Full chain with the Minecraft login endpoint returning 403; the Xbox Live link lookup fails with a database outage | Returns the Java error (`ErrMinecraftLoginWithXbox`); no account is created and no session is added | P1 | |
| OA-100 | ProcessOAuthLink | Error Path | Minecraft login fails while the Xbox Live identity is linked to the caller | Valid session; full chain with the Minecraft login endpoint returning 403; the Xbox link belongs to the caller | Returns an error wrapping `ErrMinecraftLoginWithXbox`; no linked account is stored | P1 | |
| OA-101 | linkPlatformUserToSession | Error Path | Storing the linked account fails with `ErrUserAlreadyLinkedToPlatform` | Storing the linked account returns `ErrUserAlreadyLinkedToPlatform` | Returns `ErrUserAlreadyLinkedToPlatform` unchanged, not `ErrLinkAccountFailed` | P1 | |

## discord.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| DC-06 | GetDiscordUser | Happy Path | Discord's current-user endpoint returns a valid user | Discord's current-user endpoint is served by a test server returning a valid user JSON body | Returns the decoded Discord user data, err nil | P1 |  |
| DC-07 | GetDiscordUser | Error Path | Discord API returns a non-2xx status | Server returns HTTP 401 with a Discord-style error body | Returns nil, non-nil error | P1 |  |
| DC-08 | GetDiscordUser | Edge Case | Discord API returns malformed JSON | Server returns HTTP 200 with an invalid JSON body | Returns nil, non-nil error (unmarshal failure) | P2 |  |

## DiscordData

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| DC-04 | GetData | Accessor | Returns a JSON marshal of the wrapped user | Any populated Discord user data | Returned string unmarshals back to a user with the same ID and username | P3 |  |

## steam.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| LKS-06 | VerifySteamOpenIDCallback | Happy Path | Fully valid OpenID assertion, Steam confirms it | `openid.mode` is `"id_res"`, `openid.claimed_id` matches Steam's expected pattern, and `openid.signed` includes the claimed identity; Steam's OpenID endpoint is served by a test server answering that the assertion is valid | Returns the parsed SteamID64, err nil | P0 |  |
| LKS-07 | VerifySteamOpenIDCallback | Error Path | `openid.mode` isn't `"id_res"` | `openid.mode` is `"cancel"` | Returns `ErrInvalidAssertion`-wrapped error | P1 |  |
| LKS-08 | VerifySteamOpenIDCallback | Error Path | `openid.claimed_id` doesn't match the expected pattern | `openid.claimed_id` is `"not-a-steam-url"` | Returns `ErrInvalidAssertion`-wrapped error | P1 |  |
| LKS-09 | VerifySteamOpenIDCallback | Error Path | `openid.signed` doesn't cover `openid.claimed_id` | `openid.signed` is `"op_endpoint,identity,return_to"` (no claimed identity) | Returns `ErrInvalidAssertion`-wrapped error | P0 |  |
| LKS-10 | VerifySteamOpenIDCallback | Error Path | The verification request to Steam fails outright (network error) | Steam's OpenID endpoint points at an unreachable address | Returns a plain (non-`ErrInvalidAssertion`) error | P2 |  |
| LKS-11 | VerifySteamOpenIDCallback | Error Path | Steam's verification endpoint returns a non-2xx status | Server responds HTTP 500 | Returns an error wrapping `ErrSteamOpenIDCheck` (not `ErrInvalidAssertion`) | P2 |  |
| LKS-12 | VerifySteamOpenIDCallback | Error Path | Steam rejects the assertion (answers that it is invalid) | Server responds 200 saying the assertion is invalid | Returns `ErrInvalidAssertion`-wrapped error | P0 |  |
| LKS-13 | responseIsValid | Happy Path | Body contains a line saying the assertion is valid | `[]byte("ns:foo\nis_valid:true\n")` | Returns true | P2 |  |
| LKS-14 | responseIsValid | Edge Case | The validity line is present but false, or absent | `"is_valid:false"` and separately `"ns:foo"` alone | Returns false | P2 |  |
| LKS-15 | responseIsValid | Edge Case | Empty body | `[]byte("")` | Returns false | P3 |  |
| LKS-16 | GetSteamUser | Happy Path | API key set, player summary returned and ID matches | `STEAM_API_KEY` is non-empty; Steam's player summary endpoint is served by a test server returning one matching player | Returns populated Steam data, err nil | P1 |  |
| LKS-18 | GetSteamUser | Error Path | HTTP call fails (network) | Server URL unreachable | Error propagated | P2 |  |
| LKS-19 | GetSteamUser | Error Path | Non-2xx status | Server responds HTTP 500 | Returns an error wrapping `ErrSteamPlayerSummaryLookup` | P2 |  |
| LKS-20 | GetSteamUser | Edge Case | Malformed JSON body | Server responds 200 with invalid JSON | Decode error propagated | P2 |  |
| LKS-21 | GetSteamUser | Error Path | Empty players array | Server responds with `{"response":{"players":[]}}` | Returns `ErrSteamNoPlayers` | P1 |  |
| LKS-22 | GetSteamUser | Error Path | Returned player's SteamID64 doesn't match requested | Server returns a player with a different `steamid` | Returns an error matching `ErrSteamIDMismatch` | P1 |  |
| LKS-23 | ProcessSteamLogin | Happy Path | New Steam identity resolves and a session is created | No linked account exists for the Steam identity; creating the account, storing the linked account and creating the session all succeed | Returns a session carrying the permissions the account service built, err nil | P1 |  |
| LKS-24 | ProcessSteamLogin | Error Path | Account resolution fails | Resolving the account fails (e.g. creating the account returns an error) | Error propagated | P2 |  |
| LKS-25 | ProcessSteamLogin | Error Path | Creating the session fails | Creating the session returns an error | Error propagated | P2 |  |
| LKS-26 | ProcessSteamLink | Happy Path | Valid session, identity not yet linked | Session in the request context is valid; storing the linked account succeeds | Returns the same session, err nil | P1 |  |
| LKS-29 | ProcessSteamLink | Error Path | Identity already linked to a different account | The identity is already linked to a different account | Error propagated | P2 |  |

## SteamData

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| LKS-04 | GetData | Accessor | Returns a JSON marshal of the struct | Any populated Steam data | Returned string unmarshals back to equivalent Steam data | P3 |  |

## microsoft.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| MS-16 | xstsErrForCode | Edge Case | Unrecognized code | e.g. `999` | Returns an error wrapping `ErrXSTSAuthentication` | P3 |  |
| MS-17 | GetMicrosoftUser | Happy Path | Userinfo endpoint returns a valid identity | The provider's user info endpoint is served by a test server returning 200 + `{"sub":"s1","name":"n","email":"e"}` | Returns populated Microsoft user data, err nil | P1 |  |
| MS-18 | GetMicrosoftUser | Error Path | HTTP call fails | Server unreachable | Error propagated | P2 |  |
| MS-19 | GetMicrosoftUser | Error Path | Non-200 status | Server responds 401 | Returns an error wrapping `ErrMicrosoftUserinfoLookup` | P1 |  |
| MS-20 | GetMicrosoftUser | Edge Case | Malformed JSON | Server responds 200 with invalid JSON | Decode error propagated | P2 |  |
| MS-21 | GetMicrosoftUser | Error Path | Response missing `sub` | Server responds 200 with `{"name":"n"}` | Returns `ErrUserinfoMissingSub` | P1 |  |
| MS-22 | xblAuthenticate | Happy Path | Endpoint returns a Token | The Xbox Live authentication endpoint is served by a test server returning 200 + `{"Token":"t"}` | Returns `"t"`, err nil | P1 |  |
| MS-23 | xblAuthenticate | Error Path | HTTP call fails | Server unreachable | Error propagated | P2 |  |
| MS-24 | xblAuthenticate | Error Path | Non-200 status | Server responds 401 | Returns an error wrapping `ErrXboxLiveAuthentication` | P1 |  |
| MS-25 | xblAuthenticate | Edge Case | Malformed JSON | Server responds 200 with invalid JSON | Decode error propagated | P2 |  |
| MS-26 | xblAuthenticate | Error Path | Response missing `Token` | Server responds 200 with `{}` | Returns `ErrXboxLiveMissingToken` | P1 |  |
| MS-27 | xstsAuthorize | Happy Path | Xbox Live relying party succeeds | The XSTS authorization endpoint is served by a test server returning 200 + Token/Uhs/Xid/Gtg | Returns token, userHash, xuid, gamertag all populated, err nil | P1 |  |
| MS-28 | xstsAuthorize | Edge Case | Minecraft relying party succeeds (no xid/gtg) | Server returns Token/Uhs only, no Xid/Gtg | Returns userHash populated, xuid/gamertag empty, err nil | P1 |  |
| MS-29 | xstsAuthorize | Error Path | Response carries a nonzero `XErr` | Server returns 200 with `"XErr":2148916233` | Returns `ErrNoXboxAccount`, even though HTTP status is 200 | P0 |  |
| MS-30 | xstsAuthorize | Error Path | Non-200 status, `XErr` zero | Server responds 500, no `XErr` | Returns an error wrapping `ErrXSTSAuthorization` | P1 |  |
| MS-31 | xstsAuthorize | Edge Case | Malformed JSON body, status 200 | Server returns invalid JSON | Decode error surfaced after the status/XErr checks | P2 |  |
| MS-32 | xstsAuthorize | Error Path | Missing Token or empty DisplayClaims.Xui | Server returns 200 with `{"Token":""}` | Returns `ErrXSTSMissingTokenOrClaims` | P1 |  |
| MS-33 | xstsAuthorize | Error Path | DisplayClaims.Xui[0].Uhs empty | Server returns Token+Xui entry with an empty user hash | Returns `ErrXSTSMissingUHS` | P1 |  |
| MS-34 | xstsAuthorize | Error Path | HTTP call fails | Server unreachable | Error propagated | P2 |  |
| MS-35 | minecraftLoginWithXbox | Happy Path | Endpoint returns an access_token | The Minecraft login endpoint is served by a test server returning 200 + `{"access_token":"m"}` | Returns `"m"`, err nil | P1 |  |
| MS-36 | minecraftLoginWithXbox | Error Path | HTTP call fails | Server unreachable | Error propagated | P2 |  |
| MS-37 | minecraftLoginWithXbox | Error Path | Non-200 status | Server responds 403 with a body | Returns an error wrapping `ErrMinecraftLoginWithXbox` | P1 |  |
| MS-38 | minecraftLoginWithXbox | Edge Case | Malformed JSON | Server responds 200 with invalid JSON | Decode error propagated | P2 |  |
| MS-39 | minecraftLoginWithXbox | Error Path | Missing `access_token` | Server responds 200 with `{}` | Returns `ErrLoginMissingAccessToken` | P1 |  |
| MS-40 | getMinecraftProfile | Happy Path | Valid profile returned | The Minecraft profile endpoint is served by a test server returning 200 + valid id/name/skins/capes JSON | Returns the Minecraft profile with parsed UUID, err nil | P1 |  |
| MS-41 | getMinecraftProfile | Edge Case | HTTP 404 (no Java ownership) | Server responds 404 | Returns `(nil, nil)` — not an error | P0 |  |
| MS-42 | getMinecraftProfile | Edge Case | HTTP 200 with `"error":"NOT_FOUND"` body | Server responds 200 with that error field | Returns `(nil, nil)` | P0 |  |
| MS-43 | getMinecraftProfile | Error Path | HTTP call fails | Server unreachable | Error propagated | P2 |  |
| MS-44 | getMinecraftProfile | Error Path | Non-200, non-404 status | Server responds 500 | Returns an error wrapping `ErrMinecraftProfileLookup` | P1 |  |
| MS-45 | getMinecraftProfile | Edge Case | Malformed JSON, status 200 | Server returns invalid JSON | Decode error propagated | P2 |  |
| MS-46 | getMinecraftProfile | Error Path | Profile `id` isn't a valid UUID | Server returns `{"id":"not-a-uuid","name":"n"}` | Returns wrapped UUID-parse error | P1 |  |
| MS-47 | authenticateXboxLiveIdentity | Happy Path | XSTS authorize succeeds with xuid+gamertag | XSTS authorization (via a test server) returns both | Returns the Xbox Live identity, err nil | P1 |  |
| MS-48 | authenticateXboxLiveIdentity | Error Path | XSTS authorization fails | Server error | Error propagated | P2 |  |
| MS-49 | authenticateXboxLiveIdentity | Error Path | xuid or gamertag empty despite success | Server returns Token/Uhs but empty Xid/Gtg for the XboxLive RP | Returns `ErrXSTSMissingXIDOrGTG` | P1 |  |
| MS-50 | GetXboxUser | Happy Path | Full XBL auth + XSTS authorize succeed | Both httptest legs succeed | Returns the Xbox Live identity, err nil | P1 |  |
| MS-51 | GetXboxUser | Error Path | Xbox Live authentication fails | XBL endpoint fails | Error propagated; XSTS endpoint never called | P2 |  |
| MS-52 | GetXboxAndMinecraftUser | Happy Path | Full chain succeeds, Java owned | All 4 legs (XBL, XSTS-minecraft, mc-login, mc-profile) plus XSTS-xboxlive succeed | Returns `(xbox, java, nil)` both populated | P0 |  |
| MS-53 | GetXboxAndMinecraftUser | Edge Case | Full chain succeeds, Java not owned | The Minecraft profile leg returns 404 | Returns `(xbox, nil, nil)` — not an error | P0 |  |
| MS-54 | GetXboxAndMinecraftUser | Error Path | Xbox Live authentication fails | XBL endpoint fails | Returns `(nil, nil, err)` | P1 |  |
| MS-55 | GetXboxAndMinecraftUser | Error Path | Minecraft-scoped XSTS authorization fails | That leg fails | Returns `(nil, nil, err)` | P1 |  |
| MS-56 | GetXboxAndMinecraftUser | Edge Case | Minecraft login with Xbox fails, but the Xbox Live identity still resolves | mc-login leg fails; XSTS-xboxlive leg succeeds | Returns the Xbox Live identity, no Java identity and an error; the Xbox identity is not discarded | P0 |  |
| MS-57 | GetXboxAndMinecraftUser | Error Path | Minecraft login with Xbox fails AND the Xbox Live identity fetch also fails | Both mc-login and XSTS-xboxlive legs fail | Returns an error; both identities are nil | P1 |  |
| MS-58 | GetXboxAndMinecraftUser | Edge Case | Fetching the Minecraft profile fails after a successful login-with-xbox, Xbox identity still resolves | mc-profile leg fails; XSTS-xboxlive succeeds | Returns the Xbox Live identity, no Java identity and an error | P1 |  |
| MS-59 | minecraftLoginWithXbox | Error Path | The response is non-200 and reading its body fails midway | The test transport answers 403 with a body that yields "partial" and then an unexpected end-of-file error | Returns `ErrMinecraftLoginWithXbox` | P2 | |
| MS-60 | minecraftLoginWithXbox | Edge Case | The response is non-200 with an endless body | The test transport answers 403 with a body that never ends before 64 MiB | Returns `ErrMinecraftLoginWithXbox`; the body read stops at about the error-body size limit | P1 | |

## XboxLiveData

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| MS-04 | GetData | Accessor | JSON marshal round-trips | Populated struct | Unmarshals back equivalently | P3 |  |

## MicrosoftUserData

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| MS-09 | GetData | Accessor | JSON marshal round-trips | Populated struct | Unmarshals back equivalently | P3 |  |

## MinecraftData

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| MC-04 | GetData | Accessor | JSON marshal round-trips | Populated struct incl. Skins/Capes | Unmarshals back equivalently | P3 |  |

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| LKS-69 | SetPasswordAuthEnabled | Happy Path | Re-enabling after password authentication was disabled, account has a password | | Returns nil; settings read back with password authentication enabled | P1 |  |
| LKS-71 | SetPasswordAuthEnabled | Happy Path | Disabling, account has a password and one verified+enabled link | Guard allows | Returns nil; settings read back with password authentication disabled | P1 |  |
| LKS-72 | SetPasswordAuthEnabled | Concurrency Invariant | Disabling password authentication races deleting the sole linked account on the same account | Many trials | Exactly one succeeds every trial; account never ends up with neither a password nor a link (`ErrWouldLockAccount`) | P0 |  |
| LKS-73 | SetPasswordAuthEnabled | Concurrency Invariant | Disabling password authentication races disabling login for the sole linked account on the same account | Many trials | Exactly one succeeds every trial; account never ends up with neither (`ErrWouldLockAccount`) | P0 |  |

## store

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| LS-28 | AddLinkedAccountToDB | Error Path | DB unreachable | | Returns the raw connection error (not `ErrAlreadyLinked`, since it isn't a database constraint error) | P1 |  |
| LS-54 | DeleteLinkedAccount | Edge Case | Passwordless account, exactly one verified+enabled link, no other fallback | Guard denies removing the only usable login method | Returns `ErrWouldLockAccount`; the link still exists afterward | P0 |  |
| LS-55 | DeleteLinkedAccount | Happy Path | Account has a password (set, password authentication enabled) | Guard allows since the password remains usable | Returns nil; link removed (`ErrNotFound`) | P1 |  |
| LS-56 | DeleteLinkedAccount | Happy Path | Passwordless account with a second verified+enabled link on another platform | Guard allows | Returns nil; the other link is left untouched | P1 |  |
| LS-57 | DeleteLinkedAccount | Error Path | No linked account exists for that user and platform | Nothing to delete | Returns `ErrNotFound` (not `ErrWouldLockAccount`) | P1 |  |
| LS-58 | DeleteLinkedAccount | Edge Case | A second link exists on another platform but has `login_enabled = false` | Guard only counts verified+enabled others | Returns `ErrWouldLockAccount` | P2 |  |
| LS-59 | DeleteLinkedAccount | Edge Case | Account has a password but password authentication is turned off | Guard treats disabled password auth as unusable | Returns `ErrWouldLockAccount` | P1 |  |
| LS-60 | DeleteLinkedAccount | Concurrency Invariant | Two concurrent deletes target different platforms on the same account, each the other's sole fallback | Many trials against real Postgres | Exactly one of the two succeeds every trial; exactly one link always remains (`ErrWouldLockAccount`) | P0 |  |
| LS-61 | SetLinkedAccountLoginEnabled | Happy Path | Enabling login on a verified row | | Returns nil; `LoginEnabled` becomes true | P1 |  |
| LS-62 | SetLinkedAccountLoginEnabled | Error Path | Enabling login on an unverified row | No row matches because the row is unverified | Returns `ErrLinkedAccountUnverified` | P0 |  |
| LS-63 | SetLinkedAccountLoginEnabled | Edge Case | Disabling login, this is the account's last usable login method | Same guard as deleting a linked account | Returns `ErrWouldLockAccount`; `LoginEnabled` remains true | P0 |  |
| LS-64 | SetLinkedAccountLoginEnabled | Happy Path | Disabling login, another verified+enabled link exists | | Returns nil; `LoginEnabled` becomes false | P1 |  |
| LS-65 | SetLinkedAccountLoginEnabled | Edge Case | Disabling login, password authentication is turned off and this is the only enabled link | | Returns `ErrWouldLockAccount` | P1 |  |
| LS-66 | SetLinkedAccountLoginEnabled | Concurrency Invariant | Two concurrent disables target different platforms, each the other's sole fallback | Many trials | Exactly one succeeds every trial; exactly one link remains login-enabled (`ErrWouldLockAccount`) | P0 |  |
| LS-74 | GetLinkedAccountsByUserID | Happy Path | Two linked accounts exist for a user | Real Postgres | Returns both rows | P2 |  |
| LS-75 | AddLinkedAccountToDB | Error Path | Second insert reuses an existing platform and platform-ID pair under a different user | Real Postgres refuses it with a uniqueness violation | Returns `ErrAlreadyLinked` | P0 |  |
| LS-77 | GetLinkedAccountByPlatformName | Error Path | No linked account with that platform and username | Real Postgres | Returns nil, `ErrNotFound` | P2 | |
| LS-78 | GetLinkedAccountByPlatformName | Edge Case | Two linked accounts share the platform and username | Real Postgres, two users linked on the same platform with the same username | Returns nil, `ErrDuplicateLinkedAccount` | P2 | |
| LS-79 | AddLinkedAccountToDB | Error Path | A second insert for the same user and platform with another platform id | Real Postgres; one Discord link already exists for the user | Returns `ErrUserAlreadyLinkedToPlatform` | P1 | |

## oAuthTokenStore

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| OT-43 | AddOAuthTokenToDB | Error Path | DB unreachable | | Returns raw error | P2 |  |
| OT-79 | GetOAuthTokenByUserID | Error Path | No token for that user and platform | Real Postgres | Returns nil, `ErrNotFound` | P2 | |

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| LKT-26 | NewLinkedAccount | Happy Path | userID, platform, username, platformID, data given | | Returns a linked account with all fields copied, `Verified == true`, `LoginEnabled == true` | P2 |  |

## service

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| LKV-15 | UpdateFromPlatform | Happy Path | Brand-new platform identity | Looking up the linked account returns `ErrNotFound`; creating the account, storing the linked account and updating the linked account all succeed | Returns the placeholder account's user ID, nil; the account and the linked account are each stored once | P1 |  |
| LKV-16 | UpdateFromPlatform | Happy Path | Existing platform identity | Looking up the linked account succeeds immediately | Account creation skipped entirely (no account is stored); the linked account's data is updated; returns the existing link's user ID | P1 |  |
| LKV-17 | UpdateFromPlatform | Error Path | Initial lookup fails with a non-`ErrNotFound` error | | Error propagated immediately; no account is created | P2 |  |
| LKV-18 | UpdateFromPlatform | Error Path | Creating the account fails (new-identity path) | | Error propagated; the linked account is never stored | P2 |  |
| LKV-19 | UpdateFromPlatform | Error Path | Storing the linked account fails with a generic (not `ErrAlreadyLinked`) error; cleanup succeeds | Deleting the account succeeds | Returns the original error; the placeholder account is deleted once, by its user ID | P0 |  |
| LKV-20 | UpdateFromPlatform | Error Path | Same as LKV-19, but cleanup (deleting the account) also fails | | Returns the cleanup error | P1 |  |
| LKV-21 | UpdateFromPlatform | Edge Case | Storing the linked account fails with `ErrAlreadyLinked` ("lost the race"); cleanup and winner re-fetch succeed | | Proceeds using the winner's linked account (not the caller's placeholder); returns the winner's user ID, nil error | P0 |  |
| LKV-22 | UpdateFromPlatform | Error Path | Lost the race, cleanup succeeds, but the re-fetch of the linked account fails | | Error propagated | P2 |  |
| LKV-23 | UpdateFromPlatform | Error Path | Updating the linked account fails | | Error propagated | P2 |  |
| LKV-25 | UpdateFromPlatform | Concurrency Invariant | Two concurrent callers race to resolve/create an account for the same, brand-new platform identity | Two goroutines call the function simultaneously against a shared, mutex-serialized store enforcing the real platform and platform-ID uniqueness (the losing insert gets `ErrAlreadyLinked`) | Both goroutines return without error and resolve to the same user ID; exactly one account row exists afterward (the loser's placeholder was cleaned up) | P0 | 50 trials of 8 concurrent callers; a barrier holds every first lookup until all arrive, so the lost-race path is always hit |
| LKV-28 | GetUserLinkedAccounts | Happy Path | Looking up the user's linked accounts succeeds | | Returns the slice, nil | P1 |  |
| LKV-29 | GetUserLinkedAccounts | Error Path | Looking up the user's linked accounts fails | | Error propagated | P2 |  |
| LKV-30 | UnlinkPlatform | Happy Path | Deleting the linked account succeeds | | Returns nil | P1 |  |
| LKV-31 | UnlinkPlatform | Error Path | Deleting the linked account fails (e.g. `ErrWouldLockAccount`) | | Error propagated | P2 |  |
| LKV-32 | SetPlatformLoginEnabled | Happy Path | Setting login enabled on the linked account succeeds | | Returns nil | P1 |  |
| LKV-33 | SetPlatformLoginEnabled | Error Path | Setting login enabled on the linked account fails | | Error propagated | P2 |  |
| LKV-40 | UpdateFromPlatform | Edge Case | the call's ctx is cancelled before the placeholder account is created, the link insert fails | Looking up the linked account returns `ErrNotFound`; storing the linked account fails | The account insert, the link insert and the cleanup delete each run on a ctx that is not cancelled | P1 |  |

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| AU-24 | decodeAndValidateState | Error Path | "state" is not valid base64 | state param is not URL-safe base64 | Returns (zero state, false); redirectBadRequest to auth.NN_SITE_URL, detail `msgInvalidState` | P2 |  |
| AU-25 | decodeAndValidateState | Error Path | Decoded bytes are not valid JSON | base64-valid but JSON-unmarshal fails | Returns (zero state, false); redirectBadRequest to auth.NN_SITE_URL, detail `msgInvalidState` | P2 |  |
| AU-26 | decodeAndValidateState | Edge Case | A required state field is empty (e.g. Platform="") | Otherwise-valid state JSON missing Platform/Nonce/RedirectURI/Mode | Returns (state, false); redirectBadRequest to auth.NN_SITE_URL, detail `msgInvalidState` | P2 |  |
| AU-27 | decodeAndValidateState | Error Path | RedirectURI not on the allowlist | state.RedirectURI has a different host than auth.NN_SITE_URL | Returns (state, false); redirectBadRequest to auth.NN_SITE_URL, detail `msgInvalidState` | P2 |  |
| AU-28 | decodeAndValidateState | Error Path | Missing "nonce" cookie | Valid state, request has no "nonce" cookie | Returns (state, false); redirectBadRequest to auth.NN_SITE_URL, detail `msgInvalidState` | P2 |  |
| AU-29 | decodeAndValidateState | Error Path | Nonce cookie value doesn't match state.Nonce | Valid state, "nonce" cookie present but value != state.Nonce | Returns (state, false); redirectBadRequest to auth.NN_SITE_URL, detail `msgInvalidState` | P2 |  |
| AU-35 | requireValidModeAndSession | Error Path | Unrecognized mode value | mode is neither link.ModeLogin nor link.ModeLink | Returns false; redirectBadRequest to redirectURI, detail `msgInvalidState` | P2 |  |
| AU-46 | isAllowedRedirect | Edge Case | redirectURI has a different scheme, same host | e.g. http vs https | Returns false | P3 |  |
| AU-47 | isAllowedRedirect | Edge Case | redirectURI fails url.Parse | redirectURI contains an invalid percent-escape | Returns false | P3 |  |
| AU-48 | isAllowedRedirect | Edge Case | auth.NN_SITE_URL itself fails url.Parse | auth.NN_SITE_URL temporarily overridden with an unparseable value, restored after the subtest | Returns false | P3 |  |

## handler

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| AU-13 | OAuth | Error Path | Missing "code" query param | Request has no code param | 303 redirect to auth.NN_SITE_URL with a Bad Request problem in the "problem" query param (`msgInvalidRequest`) | P1 |  |
| AU-14 | OAuth | Error Path | Missing "state" query param | code param present, state param absent (delegates to decodeAndValidateState) | 303 redirect to auth.NN_SITE_URL with a Bad Request problem (`msgInvalidRequest`) | P2 |  |
| AU-15 | OAuth | Error Path | Mode=link with no session in context | code+valid state present, state.Mode=link, no session in request context (delegates to requireValidModeAndSession) | 303 redirect to state.RedirectURI with an Unauthorized problem (`msgLoginRequiredToLink`) | P2 |  |
| AU-17 | OpenID | Error Path | Missing "state" query param | state param absent (delegates to decodeAndValidateState) | 303 redirect to auth.NN_SITE_URL with a Bad Request problem (`msgInvalidRequest`) | P2 |  |
| AU-18 | OpenID | Error Path | Mode=link with no session in context | Valid state present, state.Mode=link, no session in request context | 303 redirect to state.RedirectURI with an Unauthorized problem (`msgLoginRequiredToLink`) | P2 |  |
| AU-19 | OpenID | Error Path | VerifySteamOpenIDCallback rejects the assertion locally (bad openid.mode) | Valid state, query has openid.mode != "id_res" (no network call reached) | 303 redirect to state.RedirectURI with a Bad Request problem `msgInvalidState` | P1 |  |
| AU-20 | OpenID | Error Path | VerifySteamOpenIDCallback rejects the assertion locally (missing/invalid openid.claimed_id) | Valid state, openid.mode="id_res", openid.claimed_id missing/malformed | 303 redirect to state.RedirectURI with a Bad Request problem `msgInvalidState` | P2 |  |
| AU-50 | OAuth | Happy Path | mode=login, platform=discord | code+valid state present, mode=login; Discord's token and current-user endpoints answer from test servers; the account, session and linked-account stores resolve a brand-new account | 303 redirect straight to state.RedirectURI (no problem param), session cookie set with the JWT from ss.CreateJWT | P1 | No live network dependency: the Discord calls are answered locally, not skipped. |
| AU-51 | OpenID | Happy Path | mode=login, platform=steam | Valid state, mode=login, openid.mode=id_res with a valid signed claimed_id; Steam's check_authentication and GetPlayerSummaries endpoints answer from test servers; the Steam API key is set to a test value for the duration of the subtest; the account, session and linked-account stores resolve a brand-new account | 303 redirect straight to state.RedirectURI (no problem param), session cookie set with the JWT from ss.CreateJWT | P1 | No live network dependency: the Steam calls are answered locally, not skipped. |
| AU-52 | OAuth | Error Path | OAuth login fails while creating the session | Valid state, mode=login, platform=discord; Discord calls answered as in AU-50; creating the session returns testutil.ErrDBDown | 303 redirect to state.RedirectURI with a problem of 500 whose detail is `msgAuthenticationFailed` (the cause is not exposed) | P2 |  |
| AU-54 | OpenID | Error Path | Steam check_authentication endpoint fails without rejecting the assertion | Valid state, platform=steam; check_authentication answers 500; player summary answers 200 | 303 redirect to state.RedirectURI with a problem of 500 whose detail is `msgAuthenticationFailed` | P2 |  |
| AU-55 | OpenID | Error Path | Steam player summary lookup fails | check_authentication answers 200, GetPlayerSummaries answers 500 | 303 redirect to state.RedirectURI with a problem of 500 whose detail is `msgAuthenticationFailed` | P2 |  |
| AU-56 | OpenID | Error Path | OpenID login fails while creating the session | Both Steam calls succeed; session.Service.AddSession returns testutil.ErrDBDown | 303 redirect to state.RedirectURI with a problem of 500 whose detail is `msgAuthenticationFailed` | P2 |  |
| AU-60 | OAuth | Error Path | the state names a platform that is not supported | Valid state with an unrecognized platform, mode=login | 303 redirect to state.RedirectURI whose problem is 400 `msgInvalidPlatform` | P2 |  |
| AU-61 | OAuth | Error Path | the platform's token response carries no scope | Valid state, mode=login, platform=discord, the token endpoint returns a token without a scope | 303 redirect whose problem is 400 `msgMissingOAuthScope` | P2 |  |
| AU-62 | OpenID | Error Path | Steam returns a player whose steamid is not the verified one | Valid state, both Steam endpoints answer as in AU-51 but the player summary lists a different steamid | 303 redirect whose problem is 400 `msgSteamAccountMismatch` | P2 |  |
| AU-63 | OAuth | Error Path | the platform account is linked to a different account | Valid state, mode=link, a live session for the caller's account, the linked account store returns a link for another account | 303 redirect whose problem is 409 `msgPlatformAlreadyLinked` | P2 |  |
| AU-64 | OAuth | Error Path | the Xbox Live and Minecraft identities belong to different accounts | Valid state, mode=login, platform=minecraft with the Microsoft, Xbox Live and Minecraft calls answered by test servers, the linked account store returns a link for one account on Xbox Live and for a different account on Minecraft | 303 redirect whose problem is 409 `msgConflictingMicrosoftIdentities` | P2 |  |
| AU-67 | OpenID | Error Path | the Steam account is linked to a different account | Valid state, mode=link, a live session for the caller's account, both Steam endpoints answer as in AU-51, the linked account store returns a link for another account | 303 redirect whose problem is 409 `msgPlatformAlreadyLinked` | P2 |  |
| AU-69 | OAuth | Edge Case | the session expires between the handler's check and the link call | Valid state, mode=link; the context serves a live session for the handler's check and an expired one for the next read | 303 redirect whose problem is 401 `msgSessionExpired` | P2 |  |
| AU-71 | OpenID | Edge Case | the session expires between the handler's check and the link call | Valid state, mode=link, both Steam endpoints answer as in AU-51; the context serves a live session first and an expired one after | 303 redirect whose problem is 401 `msgSessionExpired` | P2 |  |
| AU-72 | OAuth | Error Path | Login is disabled for the linked platform account | Valid state, mode=login, platform=discord with the Discord calls answered as in AU-50; the linked account store returns a verified link with login disabled | 303 redirect to state.RedirectURI with a problem of 403 whose detail is `msgPlatformLoginDisabled` | P2 |  |
| AU-73 | OpenID | Error Path | Steam returns no player for the verified steamid | Valid state, both Steam endpoints answer as in AU-51 but the player summary lists no players | 303 redirect to state.RedirectURI with a problem of 404 whose detail is `msgSteamAccountNotFound` | P2 |  |
| AU-74 | OpenID | Error Path | Login is disabled for the linked Steam account | Valid state, both Steam endpoints answer as in AU-51; the linked account store returns a verified link with login disabled | 303 redirect to state.RedirectURI with a problem of 403 whose detail is `msgPlatformLoginDisabled` | P2 |  |
| AU-75 | OAuth | Error Path | Link mode, and the store reports the user already has a linked account on this platform | Valid state, mode=link, platform=discord with the Discord calls answered as in AU-50, session for the caller's account; storing the linked account returns `ErrUserAlreadyLinkedToPlatform` | 303 redirect to state.RedirectURI with a problem of 409 whose detail is `msgUserAlreadyLinkedToPlatform` | P1 | |
| AU-76 | OpenID | Error Path | Link mode, and the store reports the user already has a linked account on this platform | Valid state, mode=link, both Steam endpoints answer as in AU-51, session for the caller's account; storing the linked account returns `ErrUserAlreadyLinkedToPlatform` | 303 redirect to state.RedirectURI with a problem of 409 whose detail is `msgUserAlreadyLinkedToPlatform` | P1 | |
| LKU-28 | GetUserLinkedAccounts | Happy Path | Self-lookup | session.UserID == path user_id | 200 OK with the linked-accounts list | P1 |  |
| LKU-29 | GetUserLinkedAccounts | Happy Path | Admin looks up another user's linked accounts | session.UserID != path user_id, session has ScopeAdminUsers | 200 OK with the linked-accounts list | P2 |  |
| LKU-30 | GetUserLinkedAccounts | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden `msgNoPermissionToViewLinkedAccounts` | P2 |  |
| LKU-31 | GetUserLinkedAccounts | Error Path | Service.GetUserLinkedAccounts errors | Permission check passes, service returns an error | 500 Internal Server Error `msgFailedToGetLinkedAccounts` | P2 |  |
| LKU-32 | UnlinkPlatform | Happy Path | Self unlinks a platform | session.UserID == path user_id, service.UnlinkPlatform succeeds | 204 No Content | P1 |  |
| LKU-33 | UnlinkPlatform | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden `msgNoPermissionToUnlinkPlatforms` | P2 |  |
| LKU-34 | UnlinkPlatform | Error Path | Platform not linked | service.UnlinkPlatform returns account.ErrNotFound | 404 Not Found `msgPlatformNotLinked` | P2 |  |
| LKU-35 | UnlinkPlatform | Error Path | Would lock the account | service.UnlinkPlatform returns account.ErrWouldLockAccount | 400 Bad Request `msgSetPasswordOrLinkBeforeUnlinking` | P2 |  |
| LKU-36 | UnlinkPlatform | Error Path | Unclassified service error | service.UnlinkPlatform returns some other error | 500 Internal Server Error `msgFailedToUnlinkPlatform` | P2 |  |
| LKU-37 | SetPlatformLoginEnabled | Happy Path | Self toggles login-enabled | session.UserID == path user_id, body {"login_enabled":true}, service succeeds | 204 No Content | P1 |  |
| LKU-38 | SetPlatformLoginEnabled | Error Path | No permission | session.UserID != path user_id, no ScopeAdminUsers | 403 Forbidden `msgNoPermissionToUpdatePlatforms` | P2 |  |
| LKU-39 | SetPlatformLoginEnabled | Error Path | Malformed body | Permission check passes, body is not valid JSON | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| LKU-40 | SetPlatformLoginEnabled | Edge Case | login_enabled field omitted | Body decodes with no error but LoginEnabled == nil | 400 Bad Request `msgInvalidRequestBody` | P2 |  |
| LKU-41 | SetPlatformLoginEnabled | Error Path | Platform not linked | service.SetPlatformLoginEnabled returns account.ErrNotFound | 404 Not Found `msgPlatformNotLinked` | P2 |  |
| LKU-42 | SetPlatformLoginEnabled | Error Path | Would lock the account | service.SetPlatformLoginEnabled returns account.ErrWouldLockAccount | 400 Bad Request `msgSetPasswordOrLinkBeforeDisabling` | P2 |  |
| LKU-43 | SetPlatformLoginEnabled | Error Path | Linked account unverified | service.SetPlatformLoginEnabled returns auth.ErrLinkedAccountUnverified | 400 Bad Request `msgLinkedAccountUnverified` | P2 |  |
| LKU-44 | SetPlatformLoginEnabled | Error Path | Unclassified service error | service.SetPlatformLoginEnabled returns some other error | 500 Internal Server Error `msgFailedToUpdatePlatform` | P2 |  |
