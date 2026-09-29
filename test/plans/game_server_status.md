# Test plan: game_server_status

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| HD-01 | GameServerStatusHandler | Happy Path | Valid host/port, raw not requested | host+port query params valid, fake GSSService.QueryGameServer returns a populated *GameServerStatus with a non-nil Raw | 200 OK, JSON body with the status fields, "raw" key absent from the body | P1 |  |
| HD-02 | GameServerStatusHandler | Happy Path | Valid host/port, raw=true requested | Same as HD-01 plus `raw=true` query param | 200 OK, JSON body includes the "raw" field with the service's raw value | P2 |  |
| HD-03 | GameServerStatusHandler | Error Path | Missing host | `host` query param absent or empty | 400 Bad Request, problem detail "Invalid host" | P2 |  |
| HD-04 | GameServerStatusHandler | Error Path | Non-numeric port | `host` set, `port` query param is not an integer (e.g. "abc") | 400 Bad Request, problem detail "Invalid port" | P2 |  |
| HD-05 | GameServerStatusHandler | Error Path | Underlying service returns an error | host/port valid, fake GSSService.QueryGameServer returns (nil, someErr) | 404 Not Found, problem detail equals someErr.Error() | P1 |  |
| HD-06 | GameServerStatusHandler | Edge Case | Unrecognized `query_type` query param | `query_type=totallybogus`, host/port valid | ParseQueryType falls back to QueryTypeUnknown; fake service's QueryGameServer is invoked with queryType==QueryTypeUnknown; 200 OK on service success | P3 |  |
| HD-07 | SimpleGameServerStatus | Happy Path | Underlying service succeeds | host/port valid, fake GSSService.QueryGameServer returns (status, nil) | 200 OK, Content-Type text/plain, body "Online" | P1 |  |
| HD-08 | SimpleGameServerStatus | Error Path | Missing host | `host` query param absent or empty | 400 Bad Request, problem detail "Invalid host" | P2 |  |
| HD-09 | SimpleGameServerStatus | Error Path | Non-numeric port | `host` set, `port` is not an integer | 400 Bad Request, problem detail "Invalid port" | P2 |  |
| HD-10 | SimpleGameServerStatus | Error Path | Underlying service returns an error | host/port valid, fake GSSService.QueryGameServer returns (nil, someErr) | 404 Not Found, body "Offline" | P1 |  |

## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SV-01 | NewService | Accessor | Constructs a usable service | none | Returns non-nil GSSService backed by *service | P3 |  |
| SV-02 | QueryGameQ | Happy Path | Upstream returns 200 with one server keyed in the response map | Fake transport returns `{"srv":{"gq_online":true,...}}` | Returns the decoded *GameQResponse for that entry, nil error | P1 |  |
| SV-03 | QueryGameQ | Error Path | http.Get fails at the transport level | Fake transport's RoundTrip returns an error | Returns nil, `ErrGameQQuery` | P2 |  |
| SV-04 | QueryGameQ | Error Path | Upstream returns non-200 status | Fake transport returns 500 with a readable body | Returns nil, `ErrGameQQuery` | P2 |  |
| SV-05 | QueryGameQ | Error Path | Upstream returns 200 with unparseable body | Fake transport returns 200 with body "not json" | Returns nil, `ErrDecodeBody` | P2 |  |
| SV-06 | QueryGameQ | Edge Case | Upstream returns 200 with an empty response map | Fake transport returns body "{}" | Returns nil, `ErrNoGameQResponse` | P3 |  |
| SV-07 | QueryGameDig | Happy Path | Upstream returns 200 with a valid GameDigResponse body | Fake transport returns a valid single-object JSON body | Returns the decoded *GameDigResponse, nil error | P1 |  |
| SV-08 | QueryGameDig | Error Path | http.Get fails at the transport level | Fake transport's RoundTrip returns an error | Returns nil, `ErrGameDigQuery` | P2 |  |
| SV-09 | QueryGameDig | Error Path | Upstream returns non-200 status | Fake transport returns 500 with a readable body | Returns nil, `ErrGameDigQuery` | P2 |  |
| SV-10 | QueryGameDig | Error Path | Upstream returns 200 with unparseable body | Fake transport returns 200 with body "not json" | Returns nil, `ErrDecodeBody` | P2 |  |
| SV-11 | DetermineOrVerifyQueryType | Happy Path | Game exclusive to MinecraftList, explicit matching type | game="bedrock" (in MinecraftList only), queryType=QueryTypeMinecraft | Returns (QueryTypeMinecraft, true) | P1 |  |
| SV-12 | DetermineOrVerifyQueryType | Happy Path | Game exclusive to MinecraftList, QueryTypeUnknown resolves | game="bedrock", queryType=QueryTypeUnknown | Returns (QueryTypeMinecraft, true) | P1 |  |
| SV-13 | DetermineOrVerifyQueryType | Edge Case | Game found in MinecraftList only, but queryType is valid only for a different list | game="bedrock", queryType=QueryTypeGameQ | Returns (QueryTypeUnknown, false) | P2 |  |
| SV-14 | DetermineOrVerifyQueryType | Happy Path | Game exclusive to GameQList, explicit matching type | game="aa3" (in GameQList only), queryType=QueryTypeGameQ | Returns (QueryTypeGameQ, true) | P1 |  |
| SV-15 | DetermineOrVerifyQueryType | Happy Path | Game exclusive to GameQList, QueryTypeUnknown resolves | game="aa3", queryType=QueryTypeUnknown | Returns (QueryTypeGameQ, true) | P1 |  |
| SV-16 | DetermineOrVerifyQueryType | Edge Case | Game found in GameQList only, but queryType is valid only for a different list | game="aa3", queryType=QueryTypeGameDig | Returns (QueryTypeUnknown, false) | P2 |  |
| SV-17 | DetermineOrVerifyQueryType | Happy Path | Game exclusive to GameDigList, explicit matching type | game="aoc" (in GameDigList only), queryType=QueryTypeGameDig | Returns (QueryTypeGameDig, true) | P1 |  |
| SV-18 | DetermineOrVerifyQueryType | Happy Path | Game exclusive to GameDigList, QueryTypeUnknown resolves | game="aoc", queryType=QueryTypeUnknown | Returns (QueryTypeGameDig, true) | P1 |  |
| SV-19 | DetermineOrVerifyQueryType | Edge Case | Game not present in any list | game="not-a-real-game-xyz", queryType=QueryTypeUnknown | Returns (QueryTypeUnknown, false) | P2 |  |
| SV-20 | DetermineOrVerifyQueryType | Edge Case | Game present in multiple lists; rejected by the first list's switch, then resolved by falling through to a later list whose switch matches | game="minecraft" (present in MinecraftList, GameQList and GameDigList), queryType=QueryTypeGameQ | Returns (QueryTypeGameQ, true) — the MinecraftList iteration finds the game but its switch doesn't match QueryTypeGameQ, so it breaks out and the GameQList iteration matches instead | P2 |  |
| SV-21 | QueryGameServer | Error Path | Game/queryType combination is not supported | DetermineOrVerifyQueryType returns (_, false), e.g. game="not-a-real-game-xyz" | Returns nil, `ErrGameUnsupported` | P1 | The unreachable `default` branch returns `ErrGameUnsupported` as well |
| SV-22 | QueryGameServer | Error Path | Minecraft dispatch, Java branch (game=="minecraft"), underlying query fails | game="minecraft", host/port point at a closed local TCP listener (isBedrock=false) | Returns nil, non-nil error propagated from mcstatus.GetServerStatus | P1 |  |
| SV-23 | QueryGameServer | Error Path | Minecraft dispatch, Bedrock branch (game!="minecraft"), underlying query fails | game="bedrock", host/port point at a closed local TCP listener (isBedrock=true) | Returns nil, non-nil error propagated from mcstatus.GetServerStatus | P1 |  |
| SV-24 | QueryGameServer | Happy Path | Minecraft dispatch (Java or Bedrock) against a real server succeeds | MC_LIVE_TEST_SERVER env var set to a reachable `host:port` (test skips itself when unset); MC_LIVE_TEST_BEDROCK set selects the Bedrock branch | Returns non-nil *GameServerStatus with QueryType==QueryTypeMinecraft, nil error | P2 | Gated live-dependency test, same self-skip-when-unset pattern as modules/minecraft/store_test.go's TEST_POSTGRES_URL |
| SV-25 | QueryGameServer | Happy Path | GameQ dispatch success | game="aa3", queryType=QueryTypeGameQ, fake transport returns a 200 body with gq_online=true | Returns *GameServerStatus with QueryType==QueryTypeGameQ and fields matching the normalized GameQResponse, nil error | P1 |  |
| SV-26 | QueryGameServer | Error Path | GameQ dispatch, server reported offline | game="aa3", queryType=QueryTypeGameQ, fake transport returns a 200 body with gq_online=false | Returns nil, `ErrServerOffline` | P2 |  |
| SV-27 | QueryGameServer | Error Path | GameQ dispatch, underlying transport error | game="aa3", queryType=QueryTypeGameQ, fake transport's RoundTrip returns an error | Returns nil, error propagated from QueryGameQ | P2 |  |
| SV-28 | QueryGameServer | Happy Path | GameDig dispatch success | game="aoc", queryType=QueryTypeGameDig, fake transport returns a valid 200 body | Returns *GameServerStatus with QueryType==QueryTypeGameDig and fields matching the normalized GameDigResponse, nil error | P1 |  |
| SV-29 | QueryGameServer | Error Path | GameDig dispatch, underlying transport error | game="aoc", queryType=QueryTypeGameDig, fake transport's RoundTrip returns an error | Returns nil, error propagated from QueryGameDig | P2 |  |
| SV-30 | QueryGameQ | Error Path | Upstream returns non-200 status and reading its body fails | Fake transport returns 500 with a body whose `Read` errors | Returns nil, `ErrReadBody` | P2 | |
| SV-31 | QueryGameDig | Error Path | Upstream returns non-200 status and reading its body fails | Fake transport returns 500 with a body whose `Read` errors | Returns nil, `ErrReadBody` | P2 | |

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| TY-01 | NewGameServerStatus | Happy Path | Builds a status with a recognized queryType | queryType="minecraft", full set of fields/players given | Returned *GameServerStatus has matching Host/Port/Name/MapName/MaxPlayers/NumPlayers/Players/QueryType/Raw, and embedded ServerStatus.QueryType == gsspb.QueryType_MINECRAFT | P1 |  |
| TY-02 | NewGameServerStatus | Edge Case | queryType string not present in gsspb.QueryType_value | queryType="bogus" | gsspb.QueryType_value lookup misses and yields the zero value; embedded ServerStatus.QueryType == gsspb.QueryType_UNKNOWN (0), no panic | P3 |  |
| TY-03 | ParseQueryType | Happy Path | Recognized string "minecraft" | input "minecraft" | Returns QueryTypeMinecraft | P2 |  |
| TY-04 | ParseQueryType | Happy Path | Recognized string "gameq" | input "gameq" | Returns QueryTypeGameQ | P2 |  |
| TY-05 | ParseQueryType | Happy Path | Recognized string "gamedig" | input "gamedig" | Returns QueryTypeGameDig | P2 |  |
| TY-06 | ParseQueryType | Edge Case | Unrecognized string | input "" and input "xyz" | Returns QueryTypeUnknown | P3 |  |
| TY-07 | mcServerStatus.Normalize | Happy Path | Populated status with players | mcServerStatus with Host/Port/Name/Map/MaxPlayers/NumPlayers and 2 players (Name+Uuid) | Returned *GameServerStatus has matching fields, Players mapped to gsspb.Player{Name, Id: Uuid}, QueryType==QueryTypeMinecraft, Raw==the mcServerStatus value | P1 |  |
| TY-08 | mcServerStatus.Normalize | Edge Case | No players | mcServerStatus with a nil Players slice | Returned Players is a non-nil, zero-length slice; no panic | P3 |  |
| TY-09 | GameQResponse.Normalize | Happy Path | Populated response with players | GameQResponse with HostName/PortQuery/Name/MapName/MaxPlayers/NumPlayers and 2 player names | Returned *GameServerStatus has matching fields, Players mapped to gsspb.Player{Name} with empty Id, QueryType==QueryTypeGameQ, Raw==the GameQResponse value | P1 |  |
| TY-10 | GameQResponse.Normalize | Edge Case | No players | GameQResponse with a nil Players slice | Returned Players is a non-nil, zero-length slice; no panic | P3 |  |
| TY-11 | GameDigResponse.Normalize | Happy Path | Populated response with players | GameDigResponse with Connect/QueryPort/Name/Map/MaxPlayers/NumPlayers and 2 GameDigPlayer entries | Returned *GameServerStatus has matching fields, Players mapped to gsspb.Player{Name} with empty Id, QueryType==QueryTypeGameDig, Raw==the GameDigResponse value | P1 |  |
| TY-12 | GameDigResponse.Normalize | Edge Case | No players | GameDigResponse with a nil Players slice | Returned Players is a non-nil, zero-length slice; no panic | P3 |  |
