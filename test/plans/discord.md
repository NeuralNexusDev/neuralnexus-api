# Test plan: discord

## handler

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| HI-01 | Interaction | Error Path | Request has no `Content-Type` header | No `Content-Type` header set | A 415 response, `detail` is `msgRequestMustBeJSON` | P1 |  |
| HI-02 | Interaction | Error Path | `Content-Type: application/json` but the body is not valid JSON | Body is `"{not json"` | A 400 response, `detail` is `msgInvalidRequestBody` | P1 |  |
| HI-03 | Interaction | Happy Path | Ping interaction (`type` 1) | Body `{"type":1}` | A 200 response with a JSON body whose `type` is 1, a ping interaction | P1 |  |
| HI-04 | Interaction | Edge Case | Each of the grouped unhandled interaction types: application command (2), message component (3), application command autocomplete (4) and modal submit (5), each with a minimal valid `data` payload for its type | Bodies for types 2, 3, 4 and 5, each with type-matching `data` | For every type in the group, a 204 response | P2 |  |
| HI-05 | Interaction | Edge Case | Unrecognized interaction type | Body `{"type":99}` | A 204 response | P2 |  |
| HW-01 | Webhook | Error Path | Request has no `Content-Type` header | No `Content-Type` header set | A 415 response, `detail` is `msgRequestMustBeJSON` | P1 |  |
| HW-02 | Webhook | Error Path | `Content-Type: application/json` but the body is not valid JSON | Body is `"{not json"` | A 400 response, `detail` is `msgInvalidRequestBody` | P1 |  |
| HW-03 | Webhook | Edge Case | Valid ping payload but `version` is not 1 | Body `{"version":2,"type":0}` | A 204 response | P2 |  |
| HW-04 | Webhook | Happy Path | Webhook ping (`type` 0) | Body `{"version":1,"type":0}` | A 204 response | P1 |  |
| HW-05 | Webhook | Happy Path | Event `type`, `event.type` is `APPLICATION_AUTHORIZED` and `data.integration_type` is 0 (guild install) with guild data present | Body has `data.integration_type` 0 and a `data.guild` with id "g1" | A 204 response | P1 |  |
| HW-06 | Webhook | Happy Path | Same as HW-05 but `data.integration_type` is 1 (user install) with user data present | Body has `data.integration_type` 1 and a `data.user` with id "u1" | A 204 response | P2 |  |
| HW-07 | Webhook | Edge Case | `event.type` is `APPLICATION_AUTHORIZED` and `data.integration_type` is absent | Body's `data` object has no integration type and carries guild data | A 204 response | P1 |  |
| HW-08 | Webhook | Edge Case | `event.type` is `APPLICATION_AUTHORIZED` and `data.integration_type` is an unknown value | Body has `data.integration_type` 2 | A 400 response, `detail` is `msgInvalidEventData` | P2 |  |
| HW-09 | Webhook | Happy Path | `event.type` is `APPLICATION_DEAUTHORIZED` with user data present | Body has a `data.user` with id "u2" | A 204 response | P1 |  |
| HW-10 | Webhook | Edge Case | `event.type` is one of the grouped unhandled event types (for example `ENTITLEMENT_CREATE`) | Body's `event.type` is `ENTITLEMENT_CREATE` | A 204 response | P2 |  |
| HW-11 | Webhook | Edge Case | `event.type` is an unrecognized or future string | Body's `event.type` is `"SOME_FUTURE_EVENT"` | A 204 response | P2 |  |
| HW-13 | Webhook | Edge Case | Outer `type` is an unrecognized webhook type | Body `{"version":1,"type":99}` | A 204 response | P2 |  |

## WebhookEvent

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| WEU-01 | UnmarshalJSON | Happy Path | Valid JSON for an `APPLICATION_AUTHORIZED` event | Raw JSON has `type`, `timestamp`, `data.integration_type`, `data.user` and `data.guild` | No error; the event has the type and timestamp from the JSON, and its data holds integration type 0 (guild install), user id "u1" and guild id "g1" | P1 |  |
| WEU-02 | UnmarshalJSON | Happy Path | Valid JSON for an `APPLICATION_DEAUTHORIZED` event | Raw JSON has `type`, `timestamp` and `data.user` | No error; the event data holds user id "u2" | P1 |  |
| WEU-03 | UnmarshalJSON | Edge Case | Event type that has no data to decode (for example `ENTITLEMENT_CREATE`) | Raw JSON `type` is `ENTITLEMENT_CREATE` and there is no `data` | No error; the event carries no data | P2 |  |
| WEU-04 | UnmarshalJSON | Error Path | Malformed top-level JSON | Raw bytes `"{not json"` | Returns a decode error | P1 |  |
| WEU-05 | UnmarshalJSON | Error Path | `APPLICATION_AUTHORIZED` event whose `data.integration_type` has the wrong JSON type (a string instead of a number) | `data.integration_type` is `"zero"` | Returns a decode error | P2 |  |
