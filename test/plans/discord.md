# Test plan: discord

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| HI-01 | HandleDiscordInteraction | Error Path | Request `Content-Type` header is missing/not `application/json` | No `Content-Type` header set | `responses.UnsupportedMediaType` sent (415), `detail` is `msgRequestMustBeJSON` | P1 |  |
| HI-02 | HandleDiscordInteraction | Error Path | `Content-Type: application/json` but body is not valid JSON | Body is `"{not json"` | `responses.BadRequest` sent (400), `detail` is `msgInvalidRequestBody` | P1 |  |
| HI-03 | HandleDiscordInteraction | Happy Path | `type` is `discordgo.InteractionPing` (1) | Body `{"type":1}` | `responses.SendStruct` sent with status 200 and a body encoding `discordgo.Interaction{Type: InteractionPing}` | P1 |  |
| HI-04 | HandleDiscordInteraction | Edge Case | `type` is each of the grouped "unhandled" interaction types (`ApplicationCommand`, `MessageComponent`, `ApplicationCommandAutocomplete`, `ModalSubmit`), each with a minimal valid `data` payload for its type | Bodies for types 2, 3, 4, 5 each with type-matching `data` | For every type in the group, decode succeeds and `responses.NoContent` sent (204) | P2 |  |
| HI-05 | HandleDiscordInteraction | Edge Case | Unrecognized `discordgo.InteractionType` value | Body `{"type":99}` | Falls to default branch; `responses.NoContent` sent (204) | P2 |  |
| HW-01 | HandleDiscordWebhook | Error Path | Request `Content-Type` header is missing/not `application/json` | No `Content-Type` header set | `responses.UnsupportedMediaType` sent (415), `detail` is `msgRequestMustBeJSON` | P1 |  |
| HW-02 | HandleDiscordWebhook | Error Path | `Content-Type: application/json` but body is not valid JSON | Body is `"{not json"` | `responses.BadRequest` sent (400), `detail` is `msgInvalidRequestBody` | P1 |  |
| HW-03 | HandleDiscordWebhook | Edge Case | Valid PING payload but `version` field is not `1` | Body `{"version":2,"type":0}` | Processing continues (version mismatch only logged); `responses.NoContent` sent (204) | P2 |  |
| HW-04 | HandleDiscordWebhook | Happy Path | `type` is `WebhookTypePing` (0) | Body `{"version":1,"type":0}` | `responses.NoContent` sent (204) | P1 |  |
| HW-05 | HandleDiscordWebhook | Happy Path | `type` is Event, `event.type` is `APPLICATION_AUTHORIZED`, `integration_type` is `0` (GuildInstall) with guild data present | Body has `data.integration_type=0`, `data.guild.id="g1"` | `responses.NoContent` sent (204) | P1 |  |
| HW-06 | HandleDiscordWebhook | Happy Path | Same as HW-05 but `integration_type` is `1` (UserInstall) with user data present | Body has `data.integration_type=1`, `data.user.id="u1"` | `responses.NoContent` sent (204) | P2 |  |
| HW-07 | HandleDiscordWebhook | Edge Case | `event.type` is `APPLICATION_AUTHORIZED` and `data.integration_type` is absent (null); an absent integration_type is treated as a user install | Body's `data` object has no `integration_type` key | `responses.NoContent` sent (204) | P1 |  |
| HW-08 | HandleDiscordWebhook | Edge Case | `event.type` is `APPLICATION_AUTHORIZED`, `data.integration_type` is an out-of-range value | Body has `data.integration_type=2` | `responses.BadRequest` sent (400) (`msgInvalidEventData`) | P2 |  |
| HW-09 | HandleDiscordWebhook | Happy Path | `event.type` is `APPLICATION_DEAUTHORIZED` with user data present | Body has `data.user.id="u2"` | `responses.NoContent` sent (204) | P1 |  |
| HW-10 | HandleDiscordWebhook | Edge Case | `event.type` is one of the grouped "unhandled" event types (e.g. `ENTITLEMENT_CREATE`) | Body's `event.type` is `"ENTITLEMENT_CREATE"` | `responses.NoContent` sent (204) | P2 |  |
| HW-11 | HandleDiscordWebhook | Edge Case | `event.type` is an unrecognized/future string | Body's `event.type` is `"SOME_FUTURE_EVENT"` | Falls to default branch; `responses.NoContent` sent (204) | P2 |  |
| HW-13 | HandleDiscordWebhook | Edge Case | Outer `type` field is an unrecognized `WebhookType` value | Body `{"version":1,"type":99}` | Falls to outer default branch; `responses.NoContent` sent (204) | P2 |  |

## WebhookEvent

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| WEU-01 | UnmarshalJSON | Happy Path | Valid JSON for an `APPLICATION_AUTHORIZED` event | Raw JSON has `type`, `timestamp`, and `data.integration_type`/`data.user`/`data.guild` | No error; `Type`, `Timestamp` set correctly; `Data` is an `ApplicationAuthorizedWebhookData` with matching `IntegrationType`, `User.ID`, `Guild.ID` | P1 |  |
| WEU-02 | UnmarshalJSON | Happy Path | Valid JSON for an `APPLICATION_DEAUTHORIZED` event | Raw JSON has `type`, `timestamp`, `data.user` | No error; `Data` is an `ApplicationDeauthorizedWebhookData` with matching `User.ID` | P1 |  |
| WEU-03 | UnmarshalJSON | Edge Case | Event type outside the two handled by the inner switch (e.g. `ENTITLEMENT_CREATE`) | Raw JSON `type` is `"ENTITLEMENT_CREATE"`, arbitrary/absent `data` | No error; `Data` field left `nil` | P2 |  |
| WEU-04 | UnmarshalJSON | Error Path | Malformed top-level JSON | Raw bytes `"{not json"` | Returns the underlying decode error | P1 |  |
| WEU-05 | UnmarshalJSON | Error Path | Valid envelope, `type` is `APPLICATION_AUTHORIZED`, but `data.integration_type` has the wrong JSON type (string instead of number) | `data.integration_type = "zero"` | Returns the inner decode error from unmarshaling into `ApplicationAuthorizedWebhookData` | P2 |  |
