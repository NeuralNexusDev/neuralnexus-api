# Test plan: mcstatus

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| HD-01 | ServerStatusHandler | Happy Path | `raw` query param absent/false | mock `GetServerStatus` returns a status with a non-nil `Raw` | Response is 200 with the status JSON; `raw` key is omitted (nilled before encoding) | P1 |  |
| HD-02 | ServerStatusHandler | Edge Case | `raw=true` query param | same mock success | Response is 200; `raw` key is present with the original value | P2 |  |
| HD-03 | ServerStatusHandler | Error Path | `s.GetServerStatus` returns an error | mock returns `nil, err` | Response is 404 via `responses.NotFound`, body `detail` is `msgJavaStatusFailed` for `ErrJavaStatus` | P1 |  |
| HD-04 | ServerStatusHandler | Edge Case | host has no `:port` suffix, `bedrock` param absent | host = `"example.com"` | Port parse fails, defaults to 25565, the bare host is forwarded to `GetServerStatus` | P2 |  |
| HD-05 | ServerStatusHandler | Edge Case | host has no `:port` suffix, `bedrock=true` | host = `"example.com"` | Port defaults to 19132, the bare host is forwarded to `GetServerStatus` | P2 |  |
| HD-06 | ServerStatusHandler | Edge Case | `query_port` param missing | host has an explicit port | `queryPort` forwarded equals the parsed `port` | P2 |  |
| HD-07 | ServerStatusHandler | Happy Path | `bedrock=true`, `query=true`, explicit `query_port` given | full query string | `isBedrock`, `queryEnabled`, `queryPort`, and `port` all forwarded to `GetServerStatus` unchanged, and `host` is forwarded without the port suffix | P1 |  |
| HD-08 | IconHandler | Happy Path | valid host, `bedrock` absent, `GetJavaServerStatus` succeeds with a non-nil icon | mock returns a status with `Icon` set | Response `Content-Type: image/png`, 200, body is a valid PNG decoding to the same bounds; `GetJavaServerStatus` is called once with `queryEnabled` false and `queryPort` 0 | P1 |  |
| HD-09 | IconHandler | Error Path | `GetJavaServerStatus` returns an error | mock returns `ErrJavaStatus` | Response is 404 via `responses.NotFound`, body `detail` is `msgJavaStatusFailed`; handler returns without writing a PNG | P1 |  |
| HD-10 | IconHandler | Edge Case | host has no `:port` suffix | host = `"example.com"` | Port parse fails, defaults to 25565 (regardless of `bedrock`), the bare host is forwarded to `GetJavaServerStatus` | P2 |  |
| HD-11 | IconHandler | Happy Path | `bedrock=true` | any host | Response 200, `Content-Type: image/png`, body is the `bedrockIconFile` image from `iconDir`, and `GetJavaServerStatus` is never called | P1 |  |
| HD-12 | SimpleStatusHandler | Happy Path | `GetServerStatus` succeeds | mock success | Response 200, `Content-Type: text/plain`, body `"Online"` | P1 |  |
| HD-13 | SimpleStatusHandler | Error Path | `GetServerStatus` fails | mock returns `nil, err` | Response 404, body `"Offline"` | P1 |  |
| HD-14 | SimpleStatusHandler | Edge Case | host has no `:port` suffix, `bedrock=true` | host = `"example.com"` | Port defaults to 19132, the bare host is forwarded to `GetServerStatus` | P2 |  |
| HD-15 | SimpleStatusHandler | Happy Path | `bedrock=true`, `query=true`, explicit `query_port` given | full query string | `isBedrock`, `queryEnabled`, `queryPort`, `port` all forwarded to `GetServerStatus` unchanged, and `host` is forwarded without the port suffix | P2 |  |
| HD-16 | ServerStatusHandler | Error Path | `s.GetServerStatus` returns `ErrJavaStatus`, `ErrBedrockStatus` (also wrapped with a cause) or an unrecognized error | mock returns `nil, err` | 404 Not Found with `msgJavaStatusFailed` or `msgBedrockStatusFailed` for the sentinels, and 500 Internal Server Error with `msgFailedToGetServerStatus` for an unrecognized error; no cause text | P1 | |
| HD-17 | IconHandler | Error Path | `GetJavaServerStatus` returns an unrecognized error | mock returns `nil, testerrors.ErrBoom` | 500 Internal Server Error, `detail` is `msgFailedToGetServerStatus` | P2 |  |
| HD-18 | ServerStatusHandler | Edge Case | `query_port` is outside 1-65535 or not a number | `query=true` with `query_port` of `0`, `-5`, `65536` or `abc`, and a host carrying a port | `GetServerStatus` is called with `queryPort` equal to the server port; the boundary values `1` and `65535` are passed through unchanged | P2 |  |
| HD-19 | SimpleStatusHandler | Edge Case | `query_port` is outside 1-65535 or not a number | `query=true` with `query_port` of `0`, `-5`, `65536` or `abc`, and a host carrying a port | `GetServerStatus` is called with `queryPort` equal to the server port; the boundary values `1` and `65535` are passed through unchanged | P2 |  |
| HD-20 | SimpleStatusHandler | Edge Case | `query_port` param missing | `query=true`, host has an explicit port | `queryPort` forwarded equals the parsed `port` | P2 |  |
| HD-21 | ServerStatusHandler | Happy Path | java host carries a `:port` suffix | host = `"mc.example.com:25570"` | `GetServerStatus` receives host `"mc.example.com"` and port 25570 | P1 |  |
| HD-22 | ServerStatusHandler | Happy Path | `bedrock=true` and host carries a `:port` suffix | host = `"mc.example.com:19140"` | `GetServerStatus` receives host `"mc.example.com"` and port 19140 | P1 |  |
| HD-23 | IconHandler | Happy Path | host carries a `:port` suffix | host = `"mc.example.com:25570"` | `GetJavaServerStatus` receives host `"mc.example.com"` and port 25570 | P1 |  |
| HD-24 | SimpleStatusHandler | Happy Path | java host carries a `:port` suffix | host = `"mc.example.com:25570"` | `GetServerStatus` receives host `"mc.example.com"` and port 25570 | P1 |  |
| HD-25 | SimpleStatusHandler | Happy Path | `bedrock=true` and host carries a `:port` suffix | host = `"mc.example.com:19140"` | `GetServerStatus` receives host `"mc.example.com"` and port 19140 | P1 |  |
| HD-26 | SimpleStatusHandler | Edge Case | host has no `:port` suffix, `bedrock` param absent | host = `"example.com"` | Port defaults to 25565, the bare host is forwarded to `GetServerStatus` | P2 |  |
| HD-27 | ServerStatusHandler | Edge Case | java host has a non-numeric `:` suffix | host = `"mc.example.com:abc"` | The whole string is forwarded as the host and the port defaults to 25565 | P2 |  |
| HD-28 | ServerStatusHandler | Edge Case | `bedrock=true` and host has a non-numeric `:` suffix | host = `"mc.example.com:abc"` | The whole string is forwarded as the host and the port defaults to 19132 | P2 |  |
| HD-29 | SimpleStatusHandler | Edge Case | java host has a non-numeric `:` suffix | host = `"mc.example.com:abc"` | The whole string is forwarded as the host and the port defaults to 25565 | P2 |  |
| HD-30 | SimpleStatusHandler | Edge Case | `bedrock=true` and host has a non-numeric `:` suffix | host = `"mc.example.com:abc"` | The whole string is forwarded as the host and the port defaults to 19132 | P2 |  |
| HD-31 | IconHandler | Edge Case | `GetJavaServerStatus` succeeds but the status has a nil `Icon` | mock returns a status with no icon | Response 200, `Content-Type: image/png`, body is the `defaultIconFile` image from `iconDir`; the handler does not panic | P1 |  |
| HD-32 | IconHandler | Error Path | the status has a nil `Icon` and `defaultIconFile` is missing from `iconDir` | empty icon directory | Response is 500 via `responses.InternalServerError`, body `detail` is `msgIconUnavailable` | P2 |  |
| HD-33 | IconHandler | Error Path | `bedrock=true` and `bedrockIconFile` is missing from `iconDir` | empty icon directory | Response is 500 via `responses.InternalServerError`, body `detail` is `msgIconUnavailable` | P2 |  |
| HD-34 | IconHandler | Edge Case | the status has a nil `Icon` and `Legacy` set (from `GetPing16Status`) | mock returns a legacy status | Response 200, `Content-Type: image/png`, body is the `legacyIconFile` image from `iconDir` | P1 |  |

## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SV-01 | NewService | Accessor | construct a service | none | Returns a non-nil `MCStatusService` backed by `*service` | P3 |  |
| SV-02 | GetJavaServerStatus | Error Path | all of Ping17/16/14/Beta18 fail because the host is unreachable | `host`/`port` point at a closed local TCP port, `queryEnabled=false` | Returns `nil, ErrJavaStatus` | P1 |  |
| SV-03 | GetJavaServerStatus | Edge Case | same as SV-02 but `queryEnabled=true`, so `QueryFull` also fails on the same unreachable host | closed local port, `queryEnabled=true` | Still returns `nil, ErrJavaStatus`; the query-enabled branch does not change the outcome when nothing succeeds | P2 |  |
| SV-05 | GetBedrockServerStatus | Error Path | `bedrockping.Query` fails because the host is unreachable | `host`/`port` point at a closed local port | Returns `nil` and an error wrapping both `ErrBedrockStatus` and the underlying `net.Error` | P1 |  |
| SV-09 | GetJavaServerStatus | Happy Path | a real, reachable Java server responds to at least one ping variant | `MC_LIVE_JAVA_SERVER` set to `host:port` of a live Java server; `queryEnabled=false` | Returns a non-nil `*MCServerStatus`, nil error; `Host`/`Port` match the input | P1 | Gated: `t.Skip`s when `MC_LIVE_JAVA_SERVER` is unset, same self-skip pattern as `TEST_POSTGRES_URL` |
| SV-10 | GetBedrockServerStatus | Happy Path | a real, reachable Bedrock server responds | `MC_LIVE_BEDROCK_SERVER` set to `host:port` of a live Bedrock server | Returns a non-nil `*MCServerStatus`, nil error | P1 | Gated: `t.Skip`s when `MC_LIVE_BEDROCK_SERVER` is unset, same self-skip pattern as `TEST_POSTGRES_URL` |
| SV-11 | GetJavaServerStatus | Edge Case | Query is enabled and `queryPort` differs from `port` | local UDP recorders on two distinct ports (one as `port`, one as `queryPort`), `queryEnabled=true` | The recorder on `queryPort` receives a query handshake packet (prefix `FE FD 09`); the recorder on `port` receives nothing; returns `ErrJavaStatus` | P1 |  |
| SV-12 | GetJavaServerStatus | Edge Case | Query is enabled and `queryPort` equals `port` | local UDP recorder on `port`, `queryEnabled=true`, `queryPort == port` | The recorder receives a query handshake packet; returns `ErrJavaStatus` | P1 |  |
| SV-13 | GetJavaServerStatus | Edge Case | Query is disabled | local UDP recorders on `port` and `queryPort`, `queryEnabled=false` | Neither recorder receives a packet; returns `ErrJavaStatus` | P2 |  |
| SV-14 | GetJavaServerStatus | Happy Path | the server answers both the 1.7 ping and the legacy 1.6 ping | local TCP server answering both | The 1.7 status is returned (`Version` from the 1.7 reply, `Legacy` false) and no legacy ping connection is made | P1 |  |
| SV-15 | GetJavaServerStatus | Edge Case | the server refuses the 1.7 ping but answers the legacy 1.6 ping | local TCP server answering only the 1.6 ping | The 1.6 status is returned (`Version` `"1.6"`, `Legacy` true) after exactly one legacy ping connection | P1 |  |
| SV-16 | mergeQueryStatus | Edge Case | a ping status with an icon, a favicon and `Legacy` set is merged with a query status | ping status with `Icon`, `Favicon` and `Legacy` set | The query status is returned carrying the ping's `Icon`, `Favicon` and `Legacy` | P2 |  |
| SV-17 | mergeQueryStatus | Edge Case | the ping status has an empty favicon and the query status has one | ping `Favicon` empty, query `Favicon` set | The query status keeps its own `Favicon` | P3 |  |
| SV-07 | GetServerStatus | Happy Path | `isBedrock=false` | unreachable host | Delegates to `GetJavaServerStatus`; returns its distinct error `ErrJavaStatus` | P1 |  |
| SV-08 | GetServerStatus | Happy Path | `isBedrock=true` | unreachable host | Delegates to `GetBedrockServerStatus`; returns its distinct error `ErrBedrockStatus` | P1 |  |

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| TY-01 | NewServerStatus | Happy Path | all constructor args populated | java server type, non-nil players/icon/raw | All `MCServerStatus`/embedded `ServerStatus` fields mapped 1:1, including proto `ServerType` enum `JAVA` | P1 |  |
| TY-02 | NewServerStatus | Edge Case | `serverType = ServerTypeBedrock` | — | Wrapper `ServerType` == `bedrock`, proto enum == `ServerType_BEDROCK` | P2 |  |
| TY-03 | NewServerStatus | Edge Case | `players = nil` | — | `Players` field stays `nil` | P3 |  |
| TY-04 | NewServerStatus | Edge Case | `icon = nil` | — | `Icon` field stays `nil`, no panic | P3 |  |
| TY-05 | NewServerStatus | Edge Case | `serverType` is an unrecognized string | `ServerType("unknown")` | `mcstatuspb.ServerType_value` map lookup misses, proto enum defaults to the zero value (`JAVA`); wrapper `ServerType` keeps the raw string unchanged (documents Go map-miss semantics, not a bug) | P3 |  |
| TY-06 | MOTDToName | Happy Path | plain motd, no newlines/color codes | `"Hello World"` | Returned unchanged | P2 |  |
| TY-07 | MOTDToName | Edge Case | motd contains `\n` | `"Line1\nLine2"` | Newline replaced with a space | P2 |  |
| TY-08 | MOTDToName | Edge Case | motd contains Minecraft color codes | `"§aRed§r Text"` | Color codes stripped | P2 |  |
| TY-09 | MOTDToName | Edge Case | motd has leading/trailing whitespace after newline replacement | `"  \nHello\n  "` | Result is trimmed | P3 |  |
| TY-10 | MOTDToName | Edge Case | empty string | `""` | Returns `""` | P3 |  |
| TY-11 | ImgToBase64 | Happy Path | valid `image.Image` | 2x2 RGBA image | Returns `"data:image/png;base64,"` + valid base64 PNG payload decoding to the same bounds | P2 |  |
| TY-12 | ImgToBase64 | Edge Case | `nil` image | — | Returns `""` | P2 |  |
| TY-13 | LoadImgFromFile | Happy Path | valid PNG file on disk | temp file with an encoded PNG | Returns the decoded `image.Image`, nil error | P1 |  |
| TY-14 | LoadImgFromFile | Error Path | file does not exist | nonexistent path | Returns `nil`, a non-nil `os.Open` error | P2 |  |
| TY-15 | LoadImgFromFile | Error Path | file exists but is not a valid image | temp file with garbage bytes | Returns `nil`, a non-nil decode error | P2 |  |
| TY-16 | GetPing17Status | Happy Path | full `Status17` with sample players, icon, description | `Description` implements `Stringer`, `Icon` set | All fields mapped correctly; input `s.Icon` is cleared (nilled) as a side effect ; `Legacy` is false | P1 |  |
| TY-17 | GetPing17Status | Edge Case | `SamplePlayers` empty | — | `Players` is a non-nil empty slice | P3 |  |
| TY-18 | GetPing16Status | Happy Path | full `Status16` | `MOTD` contains a real newline | All fields mapped; `Version` hardcoded `"1.6"`, `Favicon` `""`, `Icon` `nil` ; `Legacy` is true | P1 |  |
| TY-19 | GetPing14Status | Happy Path | full `Status14` | `MOTD` contains a real newline | All fields mapped; `Version` hardcoded `"1.4-1.5"` ; `Legacy` is true | P1 |  |
| TY-20 | GetBeta18Status | Happy Path | full `StatusBeta18` | `MOTD` contains a real newline | All fields mapped; `Version` hardcoded `"b1.8-1.3"` ; `Legacy` is true | P1 |  |
| TY-21 | GetQueryStatus | Happy Path | full `FullQueryStatus` with sample player names | — | All fields mapped; `Players` built from plain name strings with empty `Uuid` ; `Legacy` is false | P1 |  |
| TY-22 | GetQueryStatus | Edge Case | `SamplePlayers` empty | — | `Players` is a non-nil empty slice | P3 |  |
| TY-23 | GetBedrockStatus | Happy Path | `Extra` has a MOTD second line and a map name (len 3) | `Extra = [_, line1, mapName]` | `Motd` includes `ServerName + "\n" + Extra[1]` (escaped), `Map = Extra[2]`, other fields mapped; `ServerType`/proto enum and `Name` not asserted | P1 |  |
| TY-24 | GetBedrockStatus | Edge Case | `Extra` is empty/nil | — | `Motd == Name == ServerName`, `Map == ""` | P2 |  |
| TY-25 | GetBedrockStatus | Edge Case | `Extra` has exactly 2 elements (boundary) | `len(Extra) == 2` | Second MOTD line appended (`len(Extra) > 1`), but `Map` stays `""` (`len(Extra) > 2` is false); `Name` not asserted | P2 |  |
