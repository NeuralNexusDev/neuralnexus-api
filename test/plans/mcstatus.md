# Test plan: mcstatus

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| HD-01 | ServerStatusHandler | Happy Path | `raw` query param absent/false | mock `GetServerStatus` returns a status with a non-nil `Raw` | Response is 200 with the status JSON; `raw` key is omitted (nilled before encoding) | P1 |  |
| HD-02 | ServerStatusHandler | Edge Case | `raw=true` query param | same mock success | Response is 200; `raw` key is present with the original value | P2 |  |
| HD-03 | ServerStatusHandler | Error Path | `s.GetServerStatus` returns an error | mock returns `nil, err` | Response is 502 via `responses.BadGateway`, body `detail` is `msgJavaStatusFailed` for `ErrJavaStatus` | P1 |  |
| HD-04 | ServerStatusHandler | Edge Case | host has no `:port` suffix, `bedrock` param absent | host = `"example.com"` | Port parse fails, defaults to 25565, forwarded to `GetServerStatus` | P2 |  |
| HD-05 | ServerStatusHandler | Edge Case | host has no `:port` suffix, `bedrock=true` | host = `"example.com"` | Port defaults to 19132, forwarded to `GetServerStatus` | P2 |  |
| HD-06 | ServerStatusHandler | Edge Case | `query_port` param missing/non-numeric | host has an explicit port | `queryPort` forwarded equals the parsed `port` | P2 |  |
| HD-07 | ServerStatusHandler | Happy Path | `bedrock=true`, `query=true`, explicit `query_port` given | full query string | `isBedrock`, `queryEnabled`, `queryPort`, and `host`/`port` all forwarded to `GetServerStatus` unchanged | P1 |  |
| HD-08 | IconHandler | Happy Path | valid host, `bedrock` absent, `GetJavaServerStatus` succeeds with a non-nil icon | mock returns a status with `Icon` set | Response `Content-Type: image/png`, 200, body is a valid PNG decoding to the same bounds | P1 |  |
| HD-09 | IconHandler | Error Path | `GetJavaServerStatus` returns an error | mock returns `nil, err` | Response is 502 via `responses.BadGateway`, body `detail` is `msgJavaStatusFailed` for `ErrJavaStatus`; handler returns without writing a PNG | P1 |  |
| HD-10 | IconHandler | Edge Case | host has no `:port` suffix | host = `"example.com"` | Port parse fails, defaults to 25565 (regardless of `bedrock`), forwarded to `GetJavaServerStatus` | P2 |  |
| HD-11 | IconHandler | Error Path | `bedrock=true` | any host | `responses.BadRequest` writes a 400 response; only the first-write invariants (status code 400, first problem body `detail` is `msgBedrockNoIcons`) are asserted | P1 | httptest's ResponseRecorder ignores WriteHeader calls after the first, so w.Code reflects the first write; the body may include bytes from a later write if the handler writes twice. |
| HD-12 | SimpleStatusHandler | Happy Path | `GetServerStatus` succeeds | mock success | Response 200, `Content-Type: text/plain`, body `"Online"` | P1 |  |
| HD-13 | SimpleStatusHandler | Error Path | `GetServerStatus` fails | mock returns `nil, err` | Response 404, body `"Offline"` (`ErrJavaStatus`) | P1 |  |
| HD-14 | SimpleStatusHandler | Edge Case | host has no `:port` suffix, `bedrock=true` | host = `"example.com"` | Port defaults to 19132, forwarded to `GetServerStatus` | P2 |  |
| HD-15 | SimpleStatusHandler | Happy Path | `bedrock=true`, `query=true`, explicit `query_port` given | full query string | `isBedrock`, `queryEnabled`, `queryPort`, `port` all forwarded to `GetServerStatus` unchanged | P2 |  |
| HD-16 | ServerStatusHandler | Error Path | `s.GetServerStatus` returns `ErrJavaStatus`, `ErrBedrockStatus` (also wrapped with a cause) or an unrecognized error | mock returns `nil, err` | 502 Bad Gateway with `msgJavaStatusFailed` or `msgBedrockStatusFailed` for the sentinels; 500 Internal Server Error with `msgFailedToGetServerStatus` for an unrecognized error; no cause text | P1 | |
| HD-17 | IconHandler | Error Path | `GetJavaServerStatus` returns an unrecognized error | mock returns `nil, testerrors.ErrBoom` | 500 Internal Server Error, `detail` is `msgFailedToGetServerStatus` | P2 |  |

## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SV-01 | NewService | Accessor | construct a service | none | Returns a non-nil `MCStatusService` backed by `*service` | P3 |  |
| SV-02 | GetJavaServerStatus | Error Path | all of Ping17/16/14/Beta18 fail because the host is unreachable | `host`/`port` point at a closed local TCP port, `queryEnabled=false` | Returns `nil, ErrJavaStatus` | P1 |  |
| SV-03 | GetJavaServerStatus | Edge Case | same as SV-02 but `queryEnabled=true`, so `QueryFull` also fails on the same unreachable host | closed local port, `queryEnabled=true` | Still returns `nil, ErrJavaStatus`; the query-enabled branch does not change the outcome when nothing succeeds | P2 |  |
| SV-05 | GetBedrockServerStatus | Error Path | `bedrockping.Query` fails because the host is unreachable | `host`/`port` point at a closed local port | Returns `nil` and an error wrapping both `ErrBedrockStatus` and the underlying `net.Error` | P1 |  |
| SV-09 | GetJavaServerStatus | Happy Path | a real, reachable Java server responds to at least one ping variant | `MC_LIVE_JAVA_SERVER` set to `host:port` of a live Java server; `queryEnabled=false` | Returns a non-nil `*MCServerStatus`, nil error; `Host`/`Port` match the input | P1 | Gated: `t.Skip`s when `MC_LIVE_JAVA_SERVER` is unset, same self-skip pattern as `TEST_POSTGRES_URL` |
| SV-10 | GetBedrockServerStatus | Happy Path | a real, reachable Bedrock server responds | `MC_LIVE_BEDROCK_SERVER` set to `host:port` of a live Bedrock server | Returns a non-nil `*MCServerStatus`, nil error | P1 | Gated: `t.Skip`s when `MC_LIVE_BEDROCK_SERVER` is unset, same self-skip pattern as `TEST_POSTGRES_URL` |
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
| TY-16 | GetPing17Status | Happy Path | full `Status17` with sample players, icon, description | `Description` implements `Stringer`, `Icon` set | All fields mapped correctly; input `s.Icon` is cleared (nilled) as a side effect | P1 |  |
| TY-17 | GetPing17Status | Edge Case | `SamplePlayers` empty | — | `Players` is a non-nil empty slice | P3 |  |
| TY-18 | GetPing16Status | Happy Path | full `Status16` | `MOTD` contains a real newline | All fields mapped; `Version` hardcoded `"1.6"`, `Favicon` `""`, `Icon` `nil` | P1 |  |
| TY-19 | GetPing14Status | Happy Path | full `Status14` | `MOTD` contains a real newline | All fields mapped; `Version` hardcoded `"1.4-1.5"` | P1 |  |
| TY-20 | GetBeta18Status | Happy Path | full `StatusBeta18` | `MOTD` contains a real newline | All fields mapped; `Version` hardcoded `"b1.8-1.3"` | P1 |  |
| TY-21 | GetQueryStatus | Happy Path | full `FullQueryStatus` with sample player names | — | All fields mapped; `Players` built from plain name strings with empty `Uuid` | P1 |  |
| TY-22 | GetQueryStatus | Edge Case | `SamplePlayers` empty | — | `Players` is a non-nil empty slice | P3 |  |
| TY-23 | GetBedrockStatus | Happy Path | `Extra` has a MOTD second line and a map name (len 3) | `Extra = [_, line1, mapName]` | `Motd` includes `ServerName + "\n" + Extra[1]` (escaped), `Map = Extra[2]`, other fields mapped; `ServerType`/proto enum and `Name` not asserted | P1 |  |
| TY-24 | GetBedrockStatus | Edge Case | `Extra` is empty/nil | — | `Motd == Name == ServerName`, `Map == ""` | P2 |  |
| TY-25 | GetBedrockStatus | Edge Case | `Extra` has exactly 2 elements (boundary) | `len(Extra) == 2` | Second MOTD line appended (`len(Extra) > 1`), but `Map` stays `""` (`len(Extra) > 2` is false); `Name` not asserted | P2 |  |
