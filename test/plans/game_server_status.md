# Test plan: game_server_status

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| GSH-01 | GameServerStatusHandler | Happy Path | Valid host/port, raw not requested | host+port query params valid, fake GSSService.QueryGameServer returns a populated *GameServerStatus with a non-nil Raw | 200 OK, JSON body with the status fields, "raw" key absent from the body | P1 |  |
| GSH-02 | GameServerStatusHandler | Happy Path | Valid host/port, raw=true requested | Same as GSH-01 plus `raw=true` query param | 200 OK, JSON body includes the "raw" field with the service's raw value | P2 |  |
| GSH-03 | GameServerStatusHandler | Error Path | Missing host | `host` query param absent or empty | 400 Bad Request, problem detail `msgInvalidHost` | P2 |  |
| GSH-04 | GameServerStatusHandler | Error Path | Non-numeric port | `host` set, `port` query param is not an integer (e.g. "abc") | 400 Bad Request, problem detail `msgInvalidPort` | P2 |  |
| GSH-05 | GameServerStatusHandler | Error Path | Underlying service returns an error | host/port valid, fake GSSService.QueryGameServer returns (nil, someErr) | 404 Not Found, problem detail is `msgServerOffline` for `ErrServerOffline` | P1 |  |
| GSH-06 | GameServerStatusHandler | Edge Case | Unrecognized `query_type` query param | `query_type=totallybogus`, host/port valid | ParseQueryType falls back to QueryTypeUnknown; fake service's QueryGameServer is invoked with queryType==QueryTypeUnknown; 200 OK on service success | P3 |  |
| GSH-07 | SimpleGameServerStatus | Happy Path | Underlying service succeeds | host/port valid, fake GSSService.QueryGameServer returns (status, nil) | 200 OK, Content-Type text/plain, body "Online" | P1 |  |
| GSH-08 | SimpleGameServerStatus | Error Path | Missing host | `host` query param absent or empty | 400 Bad Request, problem detail `msgInvalidHost` | P2 |  |
| GSH-09 | SimpleGameServerStatus | Error Path | Non-numeric port | `host` set, `port` is not an integer | 400 Bad Request, problem detail `msgInvalidPort` | P2 |  |
| GSH-10 | SimpleGameServerStatus | Error Path | Underlying service returns an error | host/port valid, fake GSSService.QueryGameServer returns `ErrServerOffline` | 404 Not Found, body "Offline" | P1 |  |
| GSH-11 | GameServerStatusHandler | Error Path | Underlying service returns each known sentinel | fake `QueryGameServer` returns `ErrGameQQuery`, `ErrGameDigQuery`, `ErrReadBody`, `ErrDecodeBody`, `ErrNoGameQResponse`, `ErrGameUnsupported`, `mcstatus.ErrJavaStatus`, `mcstatus.ErrBedrockStatus`, or an unrecognized error | 502 Bad Gateway with the matching const (`msgGameQQueryFailed`, `msgGameDigQueryFailed`, `msgReadBodyFailed`, `msgDecodeBodyFailed`, `msgNoGameQResponse`, `msgJavaStatusFailed`, `msgBedrockStatusFailed`); 400 Bad Request with `msgGameUnsupported` for `ErrGameUnsupported`; 500 Internal Server Error with `msgQueryFailed` for an unrecognized error; no cause text | P1 | |
| GSH-13 | SimpleGameServerStatus | Error Path | Service returns an offline error | fake `QueryGameServer` returns `ErrServerOffline`, `mcstatus.ErrJavaStatus` or `mcstatus.ErrBedrockStatus` | 404 Not Found, `text/plain` body "Offline" | P1 | The endpoint is polled, so these are not logged |
| GSH-14 | SimpleGameServerStatus | Error Path | Service returns `ErrGameUnsupported` | fake `QueryGameServer` returns `ErrGameUnsupported` | 400 Bad Request with `msgGameUnsupported` | P1 |  |
| GSH-15 | SimpleGameServerStatus | Error Path | Service returns an upstream query error | fake `QueryGameServer` returns `ErrGameQQuery`, `ErrGameDigQuery`, `ErrReadBody`, `ErrDecodeBody` or `ErrNoGameQResponse` | 502 Bad Gateway with `msgUpstreamQueryFailed`; no cause text | P1 |  |
| GSH-16 | SimpleGameServerStatus | Error Path | Service returns an unrecognized error | fake `QueryGameServer` returns `testutil.ErrBoom` | 500 Internal Server Error with `msgQueryFailed`; no cause text | P1 |  |

## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| GSV-02 | QueryGameQ | Happy Path | Upstream returns 200 with one server keyed in the response map | Fake transport returns `{"srv":{"gq_online":true,...}}` | Returns the decoded *GameQResponse for that entry, nil error | P1 |  |
| GSV-03 | QueryGameQ | Error Path | http.Get fails at the transport level | Fake transport's RoundTrip returns an error | Returns nil, `ErrGameQQuery` | P2 |  |
| GSV-04 | QueryGameQ | Error Path | Upstream returns non-200 status | Fake transport returns 500 with a readable body | Returns nil, `ErrGameQQuery` | P2 |  |
| GSV-05 | QueryGameQ | Error Path | Upstream returns 200 with unparseable body | Fake transport returns 200 with body "not json" | Returns nil, `ErrDecodeBody` | P2 |  |
| GSV-06 | QueryGameQ | Edge Case | Upstream returns 200 with an empty response map | Fake transport returns body "{}" | Returns nil, `ErrNoGameQResponse` | P3 |  |
| GSV-07 | QueryGameDig | Happy Path | Upstream returns 200 with a valid GameDigResponse body | Fake transport returns a valid single-object JSON body | Returns the decoded *GameDigResponse, nil error | P1 |  |
| GSV-08 | QueryGameDig | Error Path | http.Get fails at the transport level | Fake transport's RoundTrip returns an error | Returns nil, `ErrGameDigQuery` | P2 |  |
| GSV-09 | QueryGameDig | Error Path | Upstream returns non-200 status | Fake transport returns 500 with a readable body | Returns nil, `ErrGameDigQuery` | P2 |  |
| GSV-10 | QueryGameDig | Error Path | Upstream returns 200 with unparseable body | Fake transport returns 200 with body "not json" | Returns nil, `ErrDecodeBody` | P2 |  |
| GSV-11 | DetermineOrVerifyQueryType | Happy Path | Game exclusive to MinecraftList, explicit matching type | game="bedrock" (in MinecraftList only), queryType=QueryTypeMinecraft | Returns (QueryTypeMinecraft, true) | P1 |  |
| GSV-12 | DetermineOrVerifyQueryType | Happy Path | Game exclusive to MinecraftList, QueryTypeUnknown resolves | game="bedrock", queryType=QueryTypeUnknown | Returns (QueryTypeMinecraft, true) | P1 |  |
| GSV-13 | DetermineOrVerifyQueryType | Edge Case | Game found in MinecraftList only, but queryType is valid only for a different list | game="bedrock", queryType=QueryTypeGameQ | Returns (QueryTypeUnknown, false) | P2 |  |
| GSV-14 | DetermineOrVerifyQueryType | Happy Path | Game exclusive to GameQList, explicit matching type | game="aa3" (in GameQList only), queryType=QueryTypeGameQ | Returns (QueryTypeGameQ, true) | P1 |  |
| GSV-15 | DetermineOrVerifyQueryType | Happy Path | Game exclusive to GameQList, QueryTypeUnknown resolves | game="aa3", queryType=QueryTypeUnknown | Returns (QueryTypeGameQ, true) | P1 |  |
| GSV-16 | DetermineOrVerifyQueryType | Edge Case | Game found in GameQList only, but queryType is valid only for a different list | game="aa3", queryType=QueryTypeGameDig | Returns (QueryTypeUnknown, false) | P2 |  |
| GSV-17 | DetermineOrVerifyQueryType | Happy Path | Game exclusive to GameDigList, explicit matching type | game="aoc" (in GameDigList only), queryType=QueryTypeGameDig | Returns (QueryTypeGameDig, true) | P1 |  |
| GSV-18 | DetermineOrVerifyQueryType | Happy Path | Game exclusive to GameDigList, QueryTypeUnknown resolves | game="aoc", queryType=QueryTypeUnknown | Returns (QueryTypeGameDig, true) | P1 |  |
| GSV-19 | DetermineOrVerifyQueryType | Edge Case | Game not present in any list | game="not-a-real-game-xyz", queryType=QueryTypeUnknown | Returns (QueryTypeUnknown, false) | P2 |  |
| GSV-20 | DetermineOrVerifyQueryType | Edge Case | Game present in multiple lists; rejected by the first list's switch, then resolved by falling through to a later list whose switch matches | game="minecraft" (present in MinecraftList, GameQList and GameDigList), queryType=QueryTypeGameQ | Returns (QueryTypeGameQ, true) — the MinecraftList iteration finds the game but its switch doesn't match QueryTypeGameQ, so it breaks out and the GameQList iteration matches instead | P2 |  |
| GSV-21 | QueryGameServer | Error Path | Game/queryType combination is not supported | DetermineOrVerifyQueryType returns (_, false), e.g. game="not-a-real-game-xyz" | Returns nil, `ErrGameUnsupported` | P1 | The unreachable `default` branch returns `ErrGameUnsupported` as well |
| GSV-22 | QueryGameServer | Error Path | Minecraft dispatch, Java branch (game=="minecraft"), underlying query fails | game="minecraft", host/port point at a closed local TCP listener (isBedrock=false) | Returns nil, non-nil error propagated from mcstatus.GetServerStatus | P1 |  |
| GSV-23 | QueryGameServer | Error Path | Minecraft dispatch, Bedrock branch (game!="minecraft"), underlying query fails | game="bedrock", host/port point at a closed local TCP listener (isBedrock=true) | Returns nil, non-nil error propagated from mcstatus.GetServerStatus | P1 |  |
| GSV-24 | QueryGameServer | Happy Path | Minecraft dispatch (Java or Bedrock) against a real server succeeds | MC_LIVE_TEST_SERVER env var set to a reachable `host:port` (test skips itself when unset); MC_LIVE_TEST_BEDROCK set selects the Bedrock branch | Returns non-nil *GameServerStatus with QueryType==QueryTypeMinecraft, nil error | P2 | Gated live-dependency test, same self-skip-when-unset pattern as modules/minecraft/store_test.go's TEST_POSTGRES_URL |
| GSV-25 | QueryGameServer | Happy Path | GameQ dispatch success | game="aa3", queryType=QueryTypeGameQ, fake transport returns a 200 body with gq_online=true | Returns *GameServerStatus with QueryType==QueryTypeGameQ and fields matching the normalized GameQResponse, nil error | P1 |  |
| GSV-26 | QueryGameServer | Error Path | GameQ dispatch, server reported offline | game="aa3", queryType=QueryTypeGameQ, fake transport returns a 200 body with gq_online=false | Returns nil, `ErrServerOffline` | P2 |  |
| GSV-27 | QueryGameServer | Error Path | GameQ dispatch, underlying transport error | game="aa3", queryType=QueryTypeGameQ, fake transport's RoundTrip returns an error | Returns nil, error propagated from QueryGameQ (`ErrGameQQuery`) | P2 |  |
| GSV-28 | QueryGameServer | Happy Path | GameDig dispatch success | game="aoc", queryType=QueryTypeGameDig, fake transport returns a valid 200 body | Returns *GameServerStatus with QueryType==QueryTypeGameDig and fields matching the normalized GameDigResponse, nil error | P1 |  |
| GSV-29 | QueryGameServer | Error Path | GameDig dispatch, underlying transport error | game="aoc", queryType=QueryTypeGameDig, fake transport's RoundTrip returns an error | Returns nil, error propagated from QueryGameDig (`ErrGameDigQuery`) | P2 |  |
| GSV-30 | QueryGameQ | Error Path | Upstream returns non-200 status and reading its body fails | Fake transport returns 500 with a body whose `Read` errors | Returns nil, `ErrReadBody` | P2 | |
| GSV-31 | QueryGameDig | Error Path | Upstream returns non-200 status and reading its body fails | Fake transport returns 500 with a body whose `Read` errors | Returns nil, `ErrReadBody` | P2 | |
| GSV-32 | QueryGameQ | Error Path | Cancelling the ctx stops the outbound GET | transport blocks until the request ctx is done; ctx cancelled once the transport is reached | Returns `ErrGameQQuery` without waiting for the transport | P1 |  |
| GSV-33 | QueryGameDig | Error Path | Cancelling the ctx stops the outbound GET | transport blocks until the request ctx is done; ctx cancelled once the transport is reached | Returns `ErrGameDigQuery` without waiting for the transport | P1 |  |
| GSV-34 | QueryGameQ | Error Path | Upstream returns non-200 status | Fake transport returns 500 with a body that records `Close` | Returns nil, `ErrGameQQuery`, and the body was closed | P1 |  |
| GSV-35 | QueryGameDig | Error Path | Upstream returns non-200 status | Fake transport returns 500 with a body that records `Close` | Returns nil, `ErrGameDigQuery`, and the body was closed | P1 |  |
| GSV-36 | QueryGameQ | Edge Case | Host contains `&port=1` | host `a&port=1`, port 27015; fake transport records the request | The query has `host=a&port=1` and exactly one `port=27015`; nil error | P1 |  |
| GSV-37 | QueryGameQ | Edge Case | Host contains a newline | host `a\nb`; fake transport records the request | The request is sent, the query `host` decodes back to `a\nb`; nil error | P2 |  |
| GSV-38 | QueryGameDig | Edge Case | Host contains `&port=1` | host `a&port=1`, port 27015; fake transport records the request | The query has `host=a&port=1` and exactly one `port=27015`; nil error | P1 |  |
| GSV-39 | QueryGameDig | Edge Case | Host contains a newline | host `a\nb`; fake transport records the request | The request is sent, the query `host` decodes back to `a\nb`; nil error | P2 |  |

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| GST-01 | NewGameServerStatus | Happy Path | Builds a status with a recognized queryType | queryType="minecraft", full set of fields/players given | Returned *GameServerStatus has matching Host/Port/Name/MapName/MaxPlayers/NumPlayers/Players/QueryType/Raw, and embedded ServerStatus.QueryType == gsspb.QueryType_MINECRAFT | P1 |  |
| GST-02 | NewGameServerStatus | Edge Case | queryType string not present in gsspb.QueryType_value | queryType="bogus" | gsspb.QueryType_value lookup misses and yields the zero value; embedded ServerStatus.QueryType == gsspb.QueryType_UNKNOWN (0), no panic | P3 |  |
| GST-03 | ParseQueryType | Happy Path | Recognized string "minecraft" | input "minecraft" | Returns QueryTypeMinecraft | P2 |  |
| GST-04 | ParseQueryType | Happy Path | Recognized string "gameq" | input "gameq" | Returns QueryTypeGameQ | P2 |  |
| GST-05 | ParseQueryType | Happy Path | Recognized string "gamedig" | input "gamedig" | Returns QueryTypeGameDig | P2 |  |
| GST-06 | ParseQueryType | Edge Case | Unrecognized string | input "" and input "xyz" | Returns QueryTypeUnknown | P3 |  |
| GST-07 | mcServerStatus.Normalize | Happy Path | Populated status with players | mcServerStatus with Host/Port/Name/Map/MaxPlayers/NumPlayers and 2 players (Name+Uuid) | Returned *GameServerStatus has matching fields, Players mapped to gsspb.Player{Name, Id: Uuid}, QueryType==QueryTypeMinecraft, Raw==the mcServerStatus value | P1 |  |
| GST-08 | mcServerStatus.Normalize | Edge Case | No players | mcServerStatus with a nil Players slice | Returned Players is a non-nil, zero-length slice; no panic | P3 |  |
| GST-09 | GameQResponse.Normalize | Happy Path | Populated response with players | GameQResponse with HostName/PortQuery/Name/MapName/MaxPlayers/NumPlayers and 2 player names | Returned *GameServerStatus has matching fields, Players mapped to gsspb.Player{Name} with empty Id, QueryType==QueryTypeGameQ, Raw==the GameQResponse value | P1 |  |
| GST-10 | GameQResponse.Normalize | Edge Case | No players | GameQResponse with a nil Players slice | Returned Players is a non-nil, zero-length slice; no panic | P3 |  |
| GST-11 | GameDigResponse.Normalize | Happy Path | Populated response with players | GameDigResponse with Connect/QueryPort/Name/Map/MaxPlayers/NumPlayers and 2 GameDigPlayer entries | Returned *GameServerStatus has matching fields, Players mapped to gsspb.Player{Name} with empty Id, QueryType==QueryTypeGameDig, Raw==the GameDigResponse value | P1 |  |
| GST-12 | GameDigResponse.Normalize | Edge Case | No players | GameDigResponse with a nil Players slice | Returned Players is a non-nil, zero-length slice; no panic | P3 |  |
