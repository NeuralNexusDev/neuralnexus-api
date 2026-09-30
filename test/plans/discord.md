# Test plan: discord

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| HW-01 | HandleDiscordWebhook | Error Path | Request `Content-Type` header is missing/not `application/json` | No `Content-Type` header set | `responses.UnsupportedMediaType` sent (415), `detail` is `msgRequestMustBeJSON` | P1 |  |
| HW-02 | HandleDiscordWebhook | Error Path | `Content-Type: application/json` but body is not valid JSON | Body is `"{not json"` | `responses.BadRequest` sent (400), `detail` is `msgInvalidRequestBody` | P1 |  |
| HW-03 | HandleDiscordWebhook | Edge Case | Valid PING payload but `version` field is not `1` | Body `{"version":2,"type":0}` | Processing continues (version mismatch only logged); `responses.NoContent` sent (204) | P2 |  |
| HW-04 | HandleDiscordWebhook | Happy Path | `type` is `WebhookTypePing` (0) | Body `{"version":1,"type":0}` | `responses.NoContent` sent (204) | P1 |  |
| HW-05 | HandleDiscordWebhook | Happy Path | `type` is Event, `event.type` is `APPLICATION_AUTHORIZED`, `integration_type` is `0` (GuildInstall) with guild data present | Body has `data.integration_type=0`, `data.guild.id="g1"` | `responses.NoContent` sent (204) | P1 |  |
| HW-06 | HandleDiscordWebhook | Happy Path | Same as HW-05 but `integration_type` is `1` (UserInstall) with user data present | Body has `data.integration_type=1`, `data.user.id="u1"` | `responses.NoContent` sent (204) | P2 |  |
| HW-07 | HandleDiscordWebhook | Error Path | `event.type` is `APPLICATION_AUTHORIZED` but `data.integration_type` is absent (null) | Body's `data` object has no `integration_type` key | `responses.BadRequest` sent (400) (`msgInvalidEventData`) | P1 |  |
| HW-08 | HandleDiscordWebhook | Edge Case | `event.type` is `APPLICATION_AUTHORIZED`, `data.integration_type` is an out-of-range value | Body has `data.integration_type=2` | `responses.BadRequest` sent (400) (`msgInvalidEventData`) | P2 |  |
| HW-09 | HandleDiscordWebhook | Happy Path | `event.type` is `APPLICATION_DEAUTHORIZED` with user data present | Body has `data.user.id="u2"` | `responses.NoContent` sent (204) | P1 |  |
| HW-10 | HandleDiscordWebhook | Edge Case | `event.type` is one of the grouped "unhandled" event types (e.g. `ENTITLEMENT_CREATE`) | Body's `event.type` is `"ENTITLEMENT_CREATE"` | `responses.NoContent` sent (204) | P2 |  |
| HW-11 | HandleDiscordWebhook | Edge Case | `event.type` is an unrecognized/future string | Body's `event.type` is `"SOME_FUTURE_EVENT"` | Falls to default branch; `responses.NoContent` sent (204) | P2 |  |
| HW-12 | HandleDiscordWebhook | Edge Case | `type` is Event and the `event` object is present but has an empty/zero `type` field (non-nil `Event`, zero-value `Type`) | Body `{"version":1,"type":1,"event":{"type":"","timestamp":"2024-01-01T00:00:00Z"}}` | No panic; falls to inner default branch; `responses.NoContent` sent (204) | P3 |  |
| HW-13 | HandleDiscordWebhook | Edge Case | Outer `type` field is an unrecognized `WebhookType` value | Body `{"version":1,"type":99}` | Falls to outer default branch; `responses.NoContent` sent (204) | P2 |  |
| HI-01 | HandleDiscordInteraction | Error Path | Request `Content-Type` header is missing/not `application/json` | No `Content-Type` header set | `responses.UnsupportedMediaType` sent (415), `detail` is `msgRequestMustBeJSON` | P1 |  |
| HI-02 | HandleDiscordInteraction | Error Path | `Content-Type: application/json` but body is not valid JSON | Body is `"{not json"` | `responses.BadRequest` sent (400), `detail` is `msgInvalidRequestBody` | P1 |  |
| HI-03 | HandleDiscordInteraction | Happy Path | `type` is `discordgo.InteractionPing` (1) | Body `{"type":1}` | `responses.SendStruct` sent with status 200 and a body encoding `discordgo.Interaction{Type: InteractionPing}` | P1 |  |
| HI-04 | HandleDiscordInteraction | Edge Case | `type` is each of the grouped "unhandled" interaction types (`ApplicationCommand`, `MessageComponent`, `ApplicationCommandAutocomplete`, `ModalSubmit`), each with a minimal valid `data` payload for its type | Bodies for types 2, 3, 4, 5 each with type-matching `data` | For every type in the group, decode succeeds and `responses.NoContent` sent (204) | P2 |  |
| HI-05 | HandleDiscordInteraction | Edge Case | Unrecognized `discordgo.InteractionType` value | Body `{"type":99}` | Falls to default branch; `responses.NoContent` sent (204) | P2 |  |

## webhook.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| WT-01 | WebhookType.String | Accessor | Stringify `WebhookTypePing` | `t = WebhookTypePing` | Returns `"Ping"` | P3 |  |
| WT-02 | WebhookType.String | Accessor | Stringify `WebhookTypeEvent` | `t = WebhookTypeEvent` | Returns `"Event"` | P3 |  |
| WT-03 | WebhookType.String | Edge Case | Stringify an out-of-range value | `t = WebhookType(99)` | Returns `"WebhookType(99)"` | P3 |  |
| WET-01 | WebhookEventType.String | Accessor | Stringify each of the 12 named `WebhookEventType` constants | Table of all 12 constants | Each returns its documented Go-identifier-style name (e.g. `ApplicationAuthorized` → `"ApplicationAuthorized"`) | P3 |  |
| WET-02 | WebhookEventType.String | Edge Case | Stringify an unrecognized value | `t = WebhookEventType("SOMETHING_ELSE")` | Returns `"WebhookEventType(SOMETHING_ELSE)"` | P3 |  |
| IC-01 | InstallationContext.String | Accessor | Stringify `GuildInstall` and `UserInstall` | `c = GuildInstall` / `c = UserInstall` | Returns `"GuildInstall"` / `"UserInstall"` respectively | P3 |  |
| IC-02 | InstallationContext.String | Edge Case | Stringify an out-of-range value | `c = InstallationContext(5)` | Returns `"InstallationContext(5)"` | P3 |  |
| AAD-01 | ApplicationAuthorizedWebhookData.Type | Accessor | Call `Type()` on a populated value | Any `ApplicationAuthorizedWebhookData{}` value | Returns `ApplicationAuthorized` | P3 |  |
| ADD-01 | ApplicationDeauthorizedWebhookData.Type | Accessor | Call `Type()` on a populated value | Any `ApplicationDeauthorizedWebhookData{}` value | Returns `ApplicationDeauthorized` | P3 |  |
| WEU-01 | WebhookEvent.UnmarshalJSON | Happy Path | Valid JSON for an `APPLICATION_AUTHORIZED` event | Raw JSON has `type`, `timestamp`, and `data.integration_type`/`data.user`/`data.guild` | No error; `Type`, `Timestamp` set correctly; `Data` is an `ApplicationAuthorizedWebhookData` with matching `IntegrationType`, `User.ID`, `Guild.ID` | P1 |  |
| WEU-02 | WebhookEvent.UnmarshalJSON | Happy Path | Valid JSON for an `APPLICATION_DEAUTHORIZED` event | Raw JSON has `type`, `timestamp`, `data.user` | No error; `Data` is an `ApplicationDeauthorizedWebhookData` with matching `User.ID` | P1 |  |
| WEU-03 | WebhookEvent.UnmarshalJSON | Edge Case | Event type outside the two handled by the inner switch (e.g. `ENTITLEMENT_CREATE`) | Raw JSON `type` is `"ENTITLEMENT_CREATE"`, arbitrary/absent `data` | No error; `Data` field left `nil` | P2 |  |
| WEU-04 | WebhookEvent.UnmarshalJSON | Error Path | Malformed top-level JSON | Raw bytes `"{not json"` | Returns the underlying decode error | P1 |  |
| WEU-05 | WebhookEvent.UnmarshalJSON | Error Path | Valid envelope, `type` is `APPLICATION_AUTHORIZED`, but `data.integration_type` has the wrong JSON type (string instead of number) | `data.integration_type = "zero"` | Returns the inner decode error from unmarshaling into `ApplicationAuthorizedWebhookData` | P2 |  |
| WAD-01 | WebhookEvent.ApplicationAuthorizedData | Happy Path | Called when `Event.Type == ApplicationAuthorized` | `Data` holds an `ApplicationAuthorizedWebhookData` | Returns that exact value | P1 |  |
| WAD-02 | WebhookEvent.ApplicationAuthorizedData | Error Path | Called when `Event.Type != ApplicationAuthorized` | `Type = ApplicationDeauthorized` | Panics with message `"ApplicationAuthorizedData called on interaction of type ApplicationDeauthorized"` | P2 |  |
| WDD-01 | WebhookEvent.ApplicationDeauthorizedData | Happy Path | Called when `Event.Type == ApplicationDeauthorized` | `Data` holds an `ApplicationDeauthorizedWebhookData` | Returns that exact value | P1 |  |
| WDD-02 | WebhookEvent.ApplicationDeauthorizedData | Error Path | Called when `Event.Type != ApplicationDeauthorized` | `Type = ApplicationAuthorized` | Panics with message `"ApplicationDeauthorizedData called on interaction of type ApplicationAuthorized"` | P2 |  |
