# Test plan: game_server_status

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| GSH-01 | GameServerStatusHandler | Happy Path | Valid host and port, raw not requested | host and port query parameters are valid, the game server query returns a populated status with a raw value | 200 OK, JSON body with the status fields, `raw` key absent from the body | P1 |  |
| GSH-02 | GameServerStatusHandler | Happy Path | Valid host and port, `raw` set to `true` requested | Same as GSH-01 plus the query parameter `raw` set to `true` | 200 OK, JSON body includes the `raw` field with the raw value of the query | P2 |  |
| GSH-03 | GameServerStatusHandler | Error Path | Missing host | the host query parameter is absent or empty | 400 Bad Request, problem detail `msgInvalidHost`, and the game server query does not run | P2 |  |
| GSH-04 | GameServerStatusHandler | Error Path | Non-numeric port | host is set, the port query parameter is not an integer (for example `"abc"`) | 400 Bad Request, problem detail `msgInvalidPort`, and the game server query does not run | P2 |  |
| GSH-05 | GameServerStatusHandler | Error Path | The game server query fails because the server is offline | host and port are valid, the query fails with `ErrServerOffline` | 404 Not Found, problem detail `msgServerOffline` | P1 |  |
| GSH-06 | GameServerStatusHandler | Edge Case | Unrecognized `query_type` query parameter | `query_type` set to `"totallybogus"`, host and port valid | The game server query runs with an unknown query type, so the game alone decides how it is queried; 200 OK when the query succeeds | P3 |  |
| GSH-07 | SimpleGameServerStatus | Happy Path | The game server query succeeds | host and port are valid, the server is online | 200 OK, `Content-Type: text/plain`, body `"Online"` | P1 |  |
| GSH-08 | SimpleGameServerStatus | Error Path | Missing host | the host query parameter is absent or empty | 400 Bad Request, problem detail `msgInvalidHost`, and the game server query does not run | P2 |  |
| GSH-09 | SimpleGameServerStatus | Error Path | Non-numeric port | host is set, the port query parameter is not an integer | 400 Bad Request, problem detail `msgInvalidPort`, and the game server query does not run | P2 |  |
| GSH-11 | GameServerStatusHandler | Error Path | The game server query fails with each known error | the query fails with `ErrGameQQuery`, `ErrGameDigQuery`, `ErrReadBody`, `ErrDecodeBody`, `ErrNoGameQResponse`, `ErrGameUnsupported`, `ErrJavaStatus` or `ErrBedrockStatus` of the mcstatus package, or an unrecognized error | 502 Bad Gateway with the matching message (`msgGameQQueryFailed`, `msgGameDigQueryFailed`, `msgReadBodyFailed`, `msgDecodeBodyFailed`, `msgNoGameQResponse`, `msgJavaStatusFailed`, `msgBedrockStatusFailed`); 400 Bad Request with `msgGameUnsupported` for `ErrGameUnsupported`; 500 Internal Server Error with `msgQueryFailed` for an unrecognized error | P1 | |
| GSH-13 | SimpleGameServerStatus | Error Path | The game server query reports the server as offline | the query fails with `ErrServerOffline`, or with `ErrJavaStatus` or `ErrBedrockStatus` of the mcstatus package | 404 Not Found, `Content-Type: text/plain`, body `"Offline"` | P1 |  |
| GSH-14 | SimpleGameServerStatus | Error Path | The game server query fails with `ErrGameUnsupported` | the query fails with `ErrGameUnsupported` | 400 Bad Request with `msgGameUnsupported` | P1 |  |
| GSH-15 | SimpleGameServerStatus | Error Path | The game server query fails with an upstream query error | the query fails with `ErrGameQQuery`, `ErrGameDigQuery`, `ErrReadBody`, `ErrDecodeBody` or `ErrNoGameQResponse` | 502 Bad Gateway with `msgUpstreamQueryFailed`; no cause text | P1 |  |
| GSH-16 | SimpleGameServerStatus | Error Path | The game server query fails with an unrecognized error | the query fails for a reason the handler does not know | 500 Internal Server Error with `msgQueryFailed`; no cause text | P1 |  |

## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| GSV-11 | DetermineOrVerifyQueryType | Happy Path | Game listed only for the Minecraft query type, explicit matching type | game `"bedrock"` (listed only for the Minecraft query type), query type `"minecraft"` | Returns the `"minecraft"` query type and true | P1 |  |
| GSV-12 | DetermineOrVerifyQueryType | Happy Path | Game listed only for the Minecraft query type, unknown query type resolves | game `"bedrock"`, query type `"unknown"` | Returns the `"minecraft"` query type and true | P1 |  |
| GSV-13 | DetermineOrVerifyQueryType | Edge Case | Game listed only for the Minecraft query type, but the requested type belongs to a different list | game `"bedrock"`, query type `"gameq"` | Returns the `"unknown"` query type and false | P2 |  |
| GSV-14 | DetermineOrVerifyQueryType | Happy Path | Game listed only for the GameQ query type, explicit matching type | game `"aa3"` (listed only for the GameQ query type), query type `"gameq"` | Returns the `"gameq"` query type and true | P1 |  |
| GSV-15 | DetermineOrVerifyQueryType | Happy Path | Game listed only for the GameQ query type, unknown query type resolves | game `"aa3"`, query type `"unknown"` | Returns the `"gameq"` query type and true | P1 |  |
| GSV-16 | DetermineOrVerifyQueryType | Edge Case | Game listed only for the GameQ query type, but the requested type belongs to a different list | game `"aa3"`, query type `"gamedig"` | Returns the `"unknown"` query type and false | P2 |  |
| GSV-17 | DetermineOrVerifyQueryType | Happy Path | Game listed only for the GameDig query type, explicit matching type | game `"aoc"` (listed only for the GameDig query type), query type `"gamedig"` | Returns the `"gamedig"` query type and true | P1 |  |
| GSV-18 | DetermineOrVerifyQueryType | Happy Path | Game listed only for the GameDig query type, unknown query type resolves | game `"aoc"`, query type `"unknown"` | Returns the `"gamedig"` query type and true | P1 |  |
| GSV-19 | DetermineOrVerifyQueryType | Edge Case | Game not listed for any query type | game `"not-a-real-game-xyz"`, query type `"unknown"` | Returns the `"unknown"` query type and false | P2 |  |
| GSV-20 | DetermineOrVerifyQueryType | Edge Case | Game listed for several query types and the requested type is not the first one it is listed for | game `"minecraft"` (listed for the Minecraft, GameQ and GameDig query types), query type `"gameq"` | Returns the `"gameq"` query type and true | P2 |  |

## service

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| GSV-02 | QueryGameQ | Happy Path | Upstream returns 200 with one server keyed in the response map | the upstream answers `{"srv":{"gq_online":true}}` with more server fields | Returns the decoded GameQ response for that entry and no error | P1 |  |
| GSV-03 | QueryGameQ | Error Path | The request to the upstream fails at the transport level | the upstream request fails with a transport error | Returns no response and `ErrGameQQuery` | P2 |  |
| GSV-05 | QueryGameQ | Error Path | Upstream returns 200 with an unparseable body | the upstream answers 200 with the body `"not json"` | Returns no response and `ErrDecodeBody` | P2 |  |
| GSV-06 | QueryGameQ | Edge Case | Upstream returns 200 with an empty response map | the upstream answers with the body `"{}"` | Returns no response and `ErrNoGameQResponse` | P3 |  |
| GSV-07 | QueryGameDig | Happy Path | Upstream returns 200 with a valid GameDig body | the upstream answers with a valid single-object JSON body | Returns the decoded GameDig response and no error | P1 |  |
| GSV-08 | QueryGameDig | Error Path | The request to the upstream fails at the transport level | the upstream request fails with a transport error | Returns no response and `ErrGameDigQuery` | P2 |  |
| GSV-10 | QueryGameDig | Error Path | Upstream returns 200 with an unparseable body | the upstream answers 200 with the body `"not json"` | Returns no response and `ErrDecodeBody` | P2 |  |
| GSV-21 | QueryGameServer | Error Path | The game and query type combination is not supported | game `"not-a-real-game-xyz"` with an unknown query type, which no query type lists | Returns no status and `ErrGameUnsupported` | P1 | A query type the service cannot dispatch also returns `ErrGameUnsupported` |
| GSV-22 | QueryGameServer | Error Path | Game `"minecraft"` is queried as a Java server and the query fails | game `"minecraft"`, query type `"minecraft"`, host and port point at an unused local TCP port | Returns no status and an error | P1 |  |
| GSV-23 | QueryGameServer | Error Path | A Minecraft-listed game other than `"minecraft"` is queried as a Bedrock server and the query fails | game `"bedrock"`, query type `"minecraft"`, host and port point at an unused local TCP port | Returns no status and an error | P1 |  |
| GSV-24 | QueryGameServer | Happy Path | Minecraft query against a local Java server succeeds | local TCP listener on 127.0.0.1 answering the Java status handshake with a fixed status packet | Returns a status whose query type is `"minecraft"` and no error | P2 |  |
| GSV-25 | QueryGameServer | Happy Path | GameQ query succeeds | game `"aa3"`, query type `"gameq"`, the upstream answers 200 with `gq_online` true | Returns a status whose query type is `"gameq"`, whose name, map name, player limit and player count come from the GameQ response and whose raw value is that response, and no error | P1 |  |
| GSV-26 | QueryGameServer | Error Path | GameQ query reports the server as offline | game `"aa3"`, query type `"gameq"`, the upstream answers 200 with `gq_online` false | Returns no status and `ErrServerOffline` | P2 |  |
| GSV-27 | QueryGameServer | Error Path | GameQ query fails at the transport level | game `"aa3"`, query type `"gameq"`, the upstream request fails with a transport error | Returns no status and `ErrGameQQuery` | P2 |  |
| GSV-28 | QueryGameServer | Happy Path | GameDig query succeeds | game `"aoc"`, query type `"gamedig"`, the upstream answers 200 with a valid body | Returns a status whose query type is `"gamedig"`, whose name, map name, player limit and player count come from the GameDig response and whose raw value is that response, and no error | P1 |  |
| GSV-29 | QueryGameServer | Error Path | GameDig query fails at the transport level | game `"aoc"`, query type `"gamedig"`, the upstream request fails with a transport error | Returns no status and `ErrGameDigQuery` | P2 |  |
| GSV-30 | QueryGameQ | Error Path | Upstream returns a non-200 status and reading its body fails | the upstream answers 500 with a body that fails when read | Returns no response and `ErrReadBody` | P2 | |
| GSV-31 | QueryGameDig | Error Path | Upstream returns a non-200 status and reading its body fails | the upstream answers 500 with a body that fails when read | Returns no response and `ErrReadBody` | P2 | |
| GSV-32 | QueryGameQ | Error Path | Cancelling the request stops the outbound GET | the upstream blocks until the request is cancelled; the request is cancelled once the upstream is reached | Returns `ErrGameQQuery` without waiting for the upstream | P1 |  |
| GSV-33 | QueryGameDig | Error Path | Cancelling the request stops the outbound GET | the upstream blocks until the request is cancelled; the request is cancelled once the upstream is reached | Returns `ErrGameDigQuery` without waiting for the upstream | P1 |  |
| GSV-34 | QueryGameQ | Error Path | Upstream returns a non-200 status | the upstream answers 500 with a body that records whether it was closed | Returns no response and `ErrGameQQuery`, and the body was closed | P1 |  |
| GSV-35 | QueryGameDig | Error Path | Upstream returns a non-200 status | the upstream answers 500 with a body that records whether it was closed | Returns no response and `ErrGameDigQuery`, and the body was closed | P1 |  |
| GSV-36 | QueryGameQ | Edge Case | Host contains `"&port=1"` | host `"a&port=1"`, port 27015; the outbound request is recorded | The query has `"host=a&port=1"` and exactly one `"port=27015"`; no error | P1 |  |
| GSV-37 | QueryGameQ | Edge Case | Host contains a newline | host `"a\nb"`; the outbound request is recorded | The request is sent, the query `host` decodes back to `"a\nb"`; no error | P2 |  |
| GSV-38 | QueryGameDig | Edge Case | Host contains `"&port=1"` | host `"a&port=1"`, port 27015; the outbound request is recorded | The query has `"host=a&port=1"` and exactly one `"port=27015"`; no error | P1 |  |
| GSV-39 | QueryGameDig | Edge Case | Host contains a newline | host `"a\nb"`; the outbound request is recorded | The request is sent, the query `host` decodes back to `"a\nb"`; no error | P2 |  |

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| GST-03 | ParseQueryType | Happy Path | Recognized string `"minecraft"` | input `"minecraft"` | Returns the `"minecraft"` query type | P2 |  |
| GST-04 | ParseQueryType | Happy Path | Recognized string `"gameq"` | input `"gameq"` | Returns the `"gameq"` query type | P2 |  |
| GST-05 | ParseQueryType | Happy Path | Recognized string `"gamedig"` | input `"gamedig"` | Returns the `"gamedig"` query type | P2 |  |
| GST-06 | ParseQueryType | Edge Case | Unrecognized string | input `""` and input `"xyz"` | Returns the `"unknown"` query type | P3 |  |

## mcServerStatus

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| GST-07 | Normalize | Happy Path | Populated Minecraft status with players | a Minecraft status with host, port, name, map, player limit, player count and 2 players, each with a name and a UUID | The returned status has matching fields, the players keep their names and UUIDs as ids, the query type is `"minecraft"` and the raw value is the Minecraft status | P1 |  |
| GST-08 | Normalize | Edge Case | No players | a Minecraft status with no player list | The returned player list is empty and not missing; no panic | P3 |  |

## GameQResponse

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| GST-09 | Normalize | Happy Path | Populated response with players | a GameQ response with host name, query port, name, map name, player limit, player count and 2 player names | The returned status has matching fields, the players have the given names and an empty id, the query type is `"gameq"` and the raw value is the GameQ response | P1 |  |
| GST-10 | Normalize | Edge Case | No players | a GameQ response with no player list | The returned player list is empty and not missing; no panic | P3 |  |

## GameDigResponse

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| GST-11 | Normalize | Happy Path | Populated response with players | a GameDig response with connect address, query port, name, map, player limit, player count and 2 players | The returned status has matching fields, the players have the given names and an empty id, the query type is `"gamedig"` and the raw value is the GameDig response | P1 |  |
| GST-12 | Normalize | Edge Case | No players | a GameDig response with no player list | The returned player list is empty and not missing; no panic | P3 |  |
