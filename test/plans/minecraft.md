# Test plan: minecraft

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| TY-01 | MarshalJSON | Happy Path | Player.ProfileActions is nil | Player{ProfileActions: nil} | Marshaled JSON has no `profileActions` key | P2 |  |
| TY-02 | MarshalJSON | Edge Case | Player.ProfileActions is a non-nil empty slice | Player{ProfileActions: []string{}} | Marshaled JSON has `"profileActions":[]` | P2 |  |
| TY-03 | MarshalJSON | Edge Case | Player.ProfileActions is populated | Player{ProfileActions: []string{"a"}} | Marshaled JSON has `"profileActions":["a"]` | P3 |  |
| TY-04 | ParseProperties | Happy Path | Properties has one valid TEXTURES entry | base64+JSON-encoded TexturesValue in Property.Value | Returns decoded *TexturesValue matching input | P1 |  |
| TY-05 | ParseProperties | Edge Case | Properties is empty | Player{Properties: nil} | Returns nil | P3 |  |
| TY-06 | ParseProperties | Edge Case | Property.Name != TEXTURES | Property{Name: "other"} | Entry skipped, returns nil | P3 |  |
| TY-07 | ParseProperties | Error Path | Property.Value is not valid base64 | Property{Name: TEXTURES, Value: "not-base64!!"} | Entry skipped (logged), returns nil | P2 |  |
| TY-08 | ParseProperties | Error Path | Property.Value is valid base64 but not valid JSON | base64("not json") | Entry skipped (logged), returns nil | P2 |  |
| TY-09 | ParseProperties | Edge Case | First property invalid, second is a valid TEXTURES entry | two Properties, first malformed | Loop continues past the bad entry and returns the second's decoded value | P2 |  |
| TY-10 | IsStale (Player) | Happy Path | LastSeen is recent | LastSeen = now | Returns false | P2 |  |
| TY-11 | IsStale (Player) | Edge Case | LastSeen older than stalenessThreshold | LastSeen = now - 25h | Returns true | P2 |  |
| TY-12 | ToProfile | Happy Path | Player has a valid TEXTURES property | populated Player | Returned *Profile mirrors ID/Name/Legacy/Demo/ProfileActions, Textures = ParseProperties() result | P1 |  |
| TY-13 | ToProfile | Edge Case | Player has no properties | Player{Properties: nil} | Returned Profile.Textures is nil | P3 |  |
| TY-14 | String (Property) | Happy Path | Signature is set | Property{Signature: "sig"} | Returned string includes the signature segment | P3 |  |
| TY-15 | String (Property) | Edge Case | Signature is empty | Property{Signature: ""} | Returned string omits the signature segment | P3 |  |
| TY-16 | ToProperty | Happy Path | Populated TexturesValue | TexturesValue{...} | Returns *Property{Name: TEXTURES, Value: base64(JSON(t))}, nil error | P1 |  |
| TY-17 | ToProperty | Edge Case | Receiver is nil | var t *TexturesValue = nil | Returns (nil, nil) | P2 |  |
| TY-18 | Hash (Texture) | Happy Path | URL is "http://textures.minecraft.net/texture/abc123" | valid URL | Returns "abc123" | P1 |  |
| TY-19 | Hash (Texture) | Edge Case | Receiver is nil | var t *Texture = nil | Returns "" | P2 |  |
| TY-20 | Hash (Texture) | Edge Case | URL contains no "/" | Texture{URL: "abc123"} | Returns "" | P3 |  |
| TY-21 | Hash (Texture) | Edge Case | URL ends in "/" | Texture{URL: "http://x/"} | Returns "" | P3 |  |
| TY-22 | IsStale (Profile) | Happy Path | LastSeen is recent | LastSeen = now | Returns false | P2 |  |
| TY-23 | IsStale (Profile) | Edge Case | LastSeen older than stalenessThreshold | LastSeen = now - 25h | Returns true | P2 |  |
| TY-24 | ToPlayer | Happy Path | Profile has Textures | populated Profile | Returned Player.Properties has exactly one TEXTURES entry, nil error | P1 |  |
| TY-25 | ToPlayer | Edge Case | Profile.Textures is nil | Profile{Textures: nil} | Returned Player.Properties is empty/nil, nil error | P3 |  |
| TY-26 | IsStale (GeyserPlayer) | Happy Path | LastSeen is recent | LastSeen = now | Returns false | P3 |  |
| TY-27 | IsStale (GeyserPlayer) | Edge Case | LastSeen older than stalenessThreshold | LastSeen = now - 25h | Returns true | P3 |  |
| TY-28 | IsStale (GeyserSkin) | Happy Path | LastSeen is recent | LastSeen = now | Returns false | P3 |  |
| TY-29 | IsStale (GeyserSkin) | Edge Case | LastSeen older than stalenessThreshold | LastSeen = now - 25h | Returns true | P3 |  |
| TY-30 | SkinURL | Happy Path | Value is base64(JSON) with Textures.SKIN set | valid GeyserSkin.Value | Returns the embedded SKIN.URL | P1 |  |
| TY-31 | SkinURL | Error Path | Value is not valid base64 | Value: "not-base64!!" | Returns "" | P2 |  |
| TY-32 | SkinURL | Error Path | Value is valid base64 but not valid JSON | base64("not json") | Returns "" | P2 |  |
| TY-33 | SkinURL | Edge Case | Decodes fine but Textures.SKIN is nil | JSON with no SKIN key | Returns "" | P2 |  |
| TY-34 | xuidToUUID | Happy Path | xuid is a nonzero int64 | xuid = 123456789 | Returns zero-padded-hex value formatted as a dashed UUID string | P1 |  |
| TY-35 | xuidToUUID | Edge Case | xuid is 0 | xuid = 0 | Returns "00000000-0000-0000-0000-000000000000" | P3 |  |
| TY-36 | uuidToXUID | Happy Path | id is a UUID produced by xuidToUUID | id = xuidToUUID(N) | Returns (N, nil) | P1 |  |
| TY-37 | uuidToXUID | Error Path | id is not a valid UUID string | id = "not-a-uuid" | Returns (0, non-nil parse error) | P2 |  |
| TY-38 | uuidToXUID | Error Path | id is a well-formed UUID whose high 64 bits are nonzero | a real random uuid.New() | Returns (0, `ErrNotDerivedBedrockUUID`) | P2 |  |
| TY-39 | Value (TexturesRow) | Happy Path | Skin and Cape set, Model nil | TexturesRow{Skin, Cape} | Returns *TexturesValue with SKIN.URL and CAPE.URL prefixed by textureUrl, no Metadata | P1 |  |
| TY-40 | Value (TexturesRow) | Edge Case | Model == SLIM | TexturesRow{Skin, Model: &SLIM} | Returned SKIN.Metadata.Model == SLIM | P2 |  |
| TY-41 | Value (TexturesRow) | Edge Case | Receiver is nil | var t *TexturesRow = nil | Returns nil | P3 |  |
| TY-42 | Value (TexturesRow) | Edge Case | Skin and Cape both nil | TexturesRow{Skin: nil, Cape: nil} | Returns nil | P2 |  |
| TY-43 | Value (TexturesRow) | Edge Case | Only Skin set | TexturesRow{Skin: &h} | Returned Textures.CAPE is nil | P3 |  |

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| HD-01 | GetMojangPlayerByNameHandler | Error Path | Empty name path value | r.PathValue("name") == "" | 400 Bad Request | P2 |  |
| HD-02 | GetMojangPlayerByNameHandler | Happy Path | Valid name, service succeeds | mock returns *Player | 200 OK with player JSON body | P1 |  |
| HD-03 | GetMojangPlayerByNameHandler | Error Path | Service returns ErrPlayerNotFound | mock error = ErrPlayerNotFound | 404 Not Found | P1 |  |
| HD-04 | GetMojangPlayerByNameHandler | Error Path | Service returns a generic error | mock error = errors.New(...) | 500 Internal Server Error | P2 |  |
| HD-05 | GetMojangPlayerByUUIDHandler | Error Path | Invalid UUID path value | r.PathValue("uuid") == "not-a-uuid" | 400 Bad Request | P2 |  |
| HD-06 | GetMojangPlayerByUUIDHandler | Happy Path | Valid UUID, service succeeds | mock returns *Player | 200 OK with player JSON body | P1 |  |
| HD-07 | GetMojangPlayerByUUIDHandler | Error Path | Service returns ErrPlayerNotFound | mock error = ErrPlayerNotFound | 404 Not Found | P1 |  |
| HD-08 | GetMojangPlayerByUUIDHandler | Error Path | Service returns a generic error | mock error = errors.New(...) | 500 Internal Server Error | P2 |  |
| HD-09 | GetMojangPlayersByNamesHandler | Error Path | Content-Type header is not application/json | header omitted | 415 Unsupported Media Type | P2 |  |
| HD-10 | GetMojangPlayersByNamesHandler | Error Path | Body is not valid JSON | body = "{not json" | 400 Bad Request | P2 |  |
| HD-11 | GetMojangPlayersByNamesHandler | Error Path | names array is empty | body = "[]" | 400 Bad Request | P2 |  |
| HD-12 | GetMojangPlayersByNamesHandler | Error Path | names array has more than 10 entries | 11 names | 400 Bad Request | P2 |  |
| HD-13 | GetMojangPlayersByNamesHandler | Error Path | names array contains an empty string | ["a", ""] | 400 Bad Request | P3 |  |
| HD-14 | GetMojangPlayersByNamesHandler | Happy Path | Valid names, service succeeds | mock returns []*Player | 200 OK with players JSON body | P1 |  |
| HD-15 | GetMojangPlayersByNamesHandler | Error Path | Service returns an error | mock error | 500 Internal Server Error | P2 |  |
| HD-16 | GetMojangProfileHandler | Error Path | Invalid UUID path value | r.PathValue("uuid") invalid | 400 Bad Request | P2 |  |
| HD-17 | GetMojangProfileHandler | Happy Path | No "unsigned" query param, service succeeds | query omitted | signed=false passed to service; 200 OK | P1 |  |
| HD-18 | GetMojangProfileHandler | Edge Case | Query "unsigned=false" | ?unsigned=false | signed=true passed to service | P2 |  |
| HD-19 | GetMojangProfileHandler | Error Path | Service returns ErrPlayerNotFound | mock error | 204 No Content | P1 |  |
| HD-20 | GetMojangProfileHandler | Error Path | Service returns a generic error | mock error | 500 Internal Server Error | P2 |  |
| HD-21 | GetProfileHandler | Error Path | Invalid UUID path value | invalid uuid | 400 Bad Request | P2 |  |
| HD-22 | GetProfileHandler | Happy Path | Valid UUID, service succeeds | mock returns *Profile | 200 OK with profile JSON body | P1 |  |
| HD-23 | GetProfileHandler | Error Path | Service returns ErrPlayerNotFound | mock error | 204 No Content | P1 |  |
| HD-24 | GetProfileHandler | Error Path | Service returns a generic error | mock error | 500 Internal Server Error | P2 |  |
| HD-25 | GetProfileByNameHandler | Error Path | Empty name path value | name == "" | 400 Bad Request | P2 |  |
| HD-26 | GetProfileByNameHandler | Happy Path | Valid name, service succeeds | mock returns *Profile | 200 OK with profile JSON body | P1 |  |
| HD-27 | GetProfileByNameHandler | Error Path | Service returns ErrPlayerNotFound | mock error | 204 No Content | P1 |  |
| HD-28 | GetProfileByNameHandler | Error Path | Service returns a generic error | mock error | 500 Internal Server Error | P2 |  |
| HD-29 | GetGeyserXUIDHandler | Error Path | Empty gamertag path value | gamertag == "" | 400 Bad Request | P2 |  |
| HD-30 | GetGeyserXUIDHandler | Happy Path | Valid gamertag, service succeeds | mock returns *GeyserPlayer | 200 OK with player JSON body | P1 |  |
| HD-31 | GetGeyserXUIDHandler | Error Path | Service returns ErrPlayerNotFound | mock error | 404 Not Found | P1 |  |
| HD-32 | GetGeyserXUIDHandler | Error Path | Service returns ErrInvalidGeyserRequest | mock error | 400 Bad Request | P2 |  |
| HD-33 | GetGeyserXUIDHandler | Error Path | Service returns a generic error | mock error | 500 Internal Server Error | P2 |  |
| HD-34 | GetGeyserSkinHandler | Error Path | Non-numeric xuid path value | xuid == "abc" | 400 Bad Request | P2 |  |
| HD-35 | GetGeyserSkinHandler | Happy Path | Valid xuid, service succeeds | mock returns *GeyserSkin | 200 OK with skin JSON body | P1 |  |
| HD-36 | GetGeyserSkinHandler | Error Path | Service returns ErrSkinNotFound | mock error | 204 No Content | P1 |  |
| HD-37 | GetGeyserSkinHandler | Error Path | Service returns ErrInvalidGeyserRequest | mock error | 400 Bad Request | P2 |  |
| HD-38 | GetGeyserSkinHandler | Error Path | Service returns a generic error | mock error | 500 Internal Server Error | P2 |  |
| HD-39 | GetGeyserProfileHandler | Error Path | uuid path value is not a derived Bedrock UUID | uuidToXUID fails | 400 Bad Request | P2 |  |
| HD-40 | GetGeyserProfileHandler | Happy Path | Valid Bedrock UUID, service succeeds | mock returns *GeyserProfile | 200 OK with profile JSON body | P1 |  |
| HD-41 | GetGeyserProfileHandler | Error Path | Service returns ErrPlayerNotFound | mock error | 404 Not Found | P1 |  |
| HD-42 | GetGeyserProfileHandler | Error Path | Service returns ErrInvalidGeyserRequest | mock error | 400 Bad Request | P2 |  |
| HD-43 | GetGeyserProfileHandler | Error Path | Service returns a generic error | mock error | 500 Internal Server Error | P2 |  |
| HD-44 | GetGeyserProfileByNameHandler | Error Path | Empty gamertag path value | gamertag == "" | 400 Bad Request | P2 |  |
| HD-45 | GetGeyserProfileByNameHandler | Happy Path | Valid gamertag, service succeeds | mock returns *GeyserProfile | 200 OK with profile JSON body | P1 |  |
| HD-46 | GetGeyserProfileByNameHandler | Error Path | Service returns ErrPlayerNotFound | mock error | 404 Not Found | P1 |  |
| HD-47 | GetGeyserProfileByNameHandler | Error Path | Service returns ErrInvalidGeyserRequest | mock error | 400 Bad Request | P2 |  |
| HD-48 | GetGeyserProfileByNameHandler | Error Path | Service returns a generic error | mock error | 500 Internal Server Error | P2 |  |
| HD-49 | GetTextureHandler | Error Path | Empty hash path value | hash == "" | 400 Bad Request | P2 |  |
| HD-50 | GetTextureHandler | Happy Path | Valid hash, service succeeds | mock returns *TextureResult | 200 OK, body bytes copied verbatim, Content-Type header set | P1 |  |
| HD-51 | GetTextureHandler | Error Path | Service returns ErrTextureNotFound | mock error | 404 Not Found | P1 |  |
| HD-52 | GetTextureHandler | Error Path | Service returns a generic error | mock error | 502 Bad Gateway | P2 |  |
| HD-53 | GetGeyserTextureHandler | Error Path | Empty hash path value | hash == "" | 400 Bad Request | P2 |  |
| HD-54 | GetGeyserTextureHandler | Happy Path | Valid hash, service succeeds | mock returns *TextureResult | 200 OK, body bytes copied verbatim | P1 |  |
| HD-55 | GetGeyserTextureHandler | Error Path | Service returns ErrTextureNotFound | mock error | 404 Not Found | P1 |  |
| HD-56 | GetGeyserTextureHandler | Error Path | Service returns a generic error | mock error | 502 Bad Gateway | P2 |  |

## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SV-01 | NewService | Happy Path | client is nil | client == nil | Returned service uses http.DefaultClient | P2 |  |
| SV-02 | NewService | Edge Case | client is provided | non-nil client | Retained as-is; nnGeyserTextureUrl == nnTextureUrl+"geyser/" | P3 |  |
| SV-03 | GetMojangPlayerByName | Happy Path | Cache hit | store cache returns player, nil | Returns cached player; no DB/HTTP calls | P1 |  |
| SV-04 | GetMojangPlayerByName | Error Path | Cache error is not redis.Nil | store cache error = plain error | Returns that error, no fallback | P2 |  |
| SV-05 | GetMojangPlayerByName | Happy Path | Cache miss, DB entry fresh | cache=redis.Nil, DB player not stale | Caches and returns DB player, no Mojang call | P1 |  |
| SV-06 | GetMojangPlayerByName | Error Path | DB fresh entry, SetPlayerInCache fails | SetPlayerInCache error | Returns that error | P2 |  |
| SV-07 | GetMojangPlayerByName | Happy Path | Cache+DB miss/stale, Mojang succeeds | Mojang 200 with player JSON | Upserts, caches, returns fetched player | P0 |  |
| SV-08 | GetMojangPlayerByName | Error Path | Mojang returns 404 | Mojang status 404 | Returns ErrPlayerNotFound | P1 |  |
| SV-09 | GetMojangPlayerByName | Error Path | Mojang returns non-200/404 | Mojang status 500 | Returns an error wrapping `ErrMojangAPI` (message `mojang API error: <status>`) | P2 |  |
| SV-128 | GetMojangPlayerByName | Error Path | Mojang returns non-200/404, pinning the message | Mojang status 500 | The error text is exactly `mojang API error: 500 Internal Server Error` | P2 | |
| SV-10 | GetMojangPlayerByName | Error Path | Mojang body is not valid JSON | malformed body | Returns decode error | P2 |  |
| SV-11 | GetMojangPlayerByName | Error Path | UpsertPlayer fails after Mojang fetch | store error | Returns that error | P2 |  |
| SV-12 | GetMojangPlayerByName | Error Path | SetPlayerInCache fails after Mojang fetch | store error | Returns that error | P2 |  |
| SV-13 | GetMojangPlayerByName | Error Path | http.Client.Get fails | transport error | Returns that error | P2 |  |
| SV-14 | GetMojangPlayerByUUID | Happy Path | Cache hit | store cache returns player | Returns cached player; no DB/HTTP calls | P1 |  |
| SV-15 | GetMojangPlayerByUUID | Error Path | Cache error is not redis.Nil | plain error | Returns that error | P2 |  |
| SV-16 | GetMojangPlayerByUUID | Happy Path | Cache miss, DB entry fresh | DB player not stale | Caches and returns DB player | P1 |  |
| SV-17 | GetMojangPlayerByUUID | Happy Path | Cache+DB miss, Mojang succeeds | Mojang 200 | Upserts, caches, returns fetched player | P0 |  |
| SV-18 | GetMojangPlayerByUUID | Error Path | Mojang returns 404 | Mojang status 404 | Returns ErrPlayerNotFound | P1 |  |
| SV-19 | GetMojangPlayersByNames | Error Path | names is empty | len(names)==0 | Returns `ErrNoNamesProvided` (message `no names provided`) | P1 |  |
| SV-20 | GetMojangPlayersByNames | Error Path | names has more than 10 entries | len(names)==11 | Returns `ErrBatchLimit` (message `batch lookup is limited to 10 names`) | P1 |  |
| SV-21 | GetMojangPlayersByNames | Happy Path | All names are cache hits | every GetPlayerFromCache succeeds | Returns players from cache; no Mojang call | P1 |  |
| SV-22 | GetMojangPlayersByNames | Happy Path | Cache miss but DB entry fresh for all names | cache=redis.Nil, DB fresh | Resolved via DB+cache-set; no Mojang call | P1 |  |
| SV-23 | GetMojangPlayersByNames | Error Path | DB-fresh path SetPlayerInCache fails | store error | Returns that error | P2 |  |
| SV-24 | GetMojangPlayersByNames | Happy Path | Some names miss cache+DB | Mojang bulk 200 with fetched players | Upserts+caches fetched entries, appended to result | P0 |  |
| SV-25 | GetMojangPlayersByNames | Error Path | Mojang bulk POST fails | transport error | Returns that error | P2 |  |
| SV-26 | GetMojangPlayersByNames | Error Path | Mojang bulk returns non-200 | status 500 | Returns an error wrapping `ErrMojangAPI` (message `mojang API error: <status>`) | P2 |  |
| SV-27 | GetMojangPlayersByNames | Error Path | Mojang bulk body is not valid JSON | malformed body | Returns decode error | P2 |  |
| SV-28 | GetMojangPlayersByNames | Error Path | UpsertPlayer fails on a fetched entry | store error | Returns that error | P2 |  |
| SV-29 | GetMojangPlayersByNames | Error Path | SetPlayerInCache fails on a fetched entry | store error | Returns that error | P2 |  |
| SV-30 | GetMojangProfile | Happy Path | signed=true, cache hit | GetSignedProfileFromCache succeeds | Returns cached signed player | P1 |  |
| SV-31 | GetMojangProfile | Error Path | signed=true, cache error not redis.Nil | plain error | Returns that error | P2 |  |
| SV-32 | GetMojangProfile | Happy Path | signed=true, cache miss | fetchProfileFromMojang(id,true) succeeds | Returns fetched player | P1 |  |
| SV-33 | GetMojangProfile | Error Path | signed=true, fetchProfileFromMojang fails | mojang error | Returns that error | P2 |  |
| SV-34 | GetMojangProfile | Happy Path | signed=false | resolveProfile succeeds | Returns profile.ToPlayer() result | P1 |  |
| SV-35 | GetMojangProfile | Error Path | signed=false, resolveProfile fails | resolveProfile error | Returns that error | P2 |  |
| SV-36 | GetProfile | Happy Path | resolveProfile succeeds, SKIN+CAPE present | Textures with both | Both URLs rewritten to nnTextureUrl+hash | P1 |  |
| SV-37 | GetProfile | Edge Case | resolveProfile succeeds, Textures nil | Profile.Textures == nil | Returned unmodified, no panic | P2 |  |
| SV-38 | GetProfile | Error Path | resolveProfile fails | resolveProfile error | Returns that error | P2 |  |
| SV-39 | GetProfile | Edge Case | Textures present, CAPE nil | only SKIN set | Only SKIN.URL rewritten | P3 |  |
| SV-40 | GetProfileByName | Happy Path | Name resolves, then profile resolves | both succeed | Returns resolved profile | P1 |  |
| SV-41 | GetProfileByName | Error Path | GetMojangPlayerByName fails | e.g. ErrPlayerNotFound | Returns that error | P2 |  |
| SV-42 | GetProfileByName | Error Path | GetProfile fails after name resolves | GetProfile error | Returns that error | P2 |  |
| SV-43 | resolveProfile | Happy Path | Cache hit | GetProfileFromCache succeeds | Returns cached profile | P1 |  |
| SV-44 | resolveProfile | Error Path | Cache error not redis.Nil | plain error | Returns that error | P2 |  |
| SV-45 | resolveProfile | Happy Path | DB profile fresh | not stale | Caches and returns DB profile | P1 |  |
| SV-46 | resolveProfile | Edge Case | DB profile fresh, ProfileActions nil | ProfileActions == nil | Normalized to []string{} before caching/return | P2 |  |
| SV-47 | resolveProfile | Error Path | DB profile fresh, SetProfileInCache fails | store error | Returns that error | P2 |  |
| SV-48 | resolveProfile | Edge Case | DB error is not ErrPlayerNotFound | e.g. connection error | Logged, falls through to Mojang fetch | P2 |  |
| SV-49 | resolveProfile | Happy Path | DB miss/stale | dbProfile nil or stale | Calls fetchProfileFromMojang(id,false) | P1 |  |
| SV-50 | resolveProfile | Error Path | fetchProfileFromMojang fails | mojang error | Returns that error | P2 |  |
| SV-51 | fetchProfileFromMojang | Happy Path | signed=false, Mojang succeeds | 200 with textures | Upserts player, stores textures+hashes, sets profile cache, returns player+profile | P0 |  |
| SV-52 | fetchProfileFromMojang | Happy Path | signed=true, Mojang succeeds | 200 | Request URL has "?unsigned=false"; sets signed cache instead of profile cache | P1 |  |
| SV-53 | fetchProfileFromMojang | Error Path | http.Client.Get fails | transport error | Returns that error | P2 |  |
| SV-54 | fetchProfileFromMojang | Error Path | Mojang returns 204 | status 204 | Returns ErrPlayerNotFound | P1 |  |
| SV-55 | fetchProfileFromMojang | Error Path | Mojang returns non-200/204 | status 500 | Returns an error wrapping `ErrMojangAPI` (message `mojang API error: <status>`) | P2 |  |
| SV-56 | fetchProfileFromMojang | Error Path | Mojang body is not valid JSON | malformed body | Returns decode error | P2 |  |
| SV-57 | fetchProfileFromMojang | Edge Case | Response ProfileActions is nil | ProfileActions == nil | Normalized to []string{} | P2 |  |
| SV-58 | fetchProfileFromMojang | Error Path | UpsertPlayer fails | store error | Returns that error; texture processing skipped | P1 |  |
| SV-59 | fetchProfileFromMojang | Edge Case | profile.Textures is nil | no TEXTURES property | Texture-storage block skipped, still succeeds | P2 |  |
| SV-60 | fetchProfileFromMojang | Edge Case | UpsertTextureHash fails for skin/cape | store error | Logged only; function still succeeds | P2 |  |
| SV-61 | fetchProfileFromMojang | Edge Case | UpsertTextures fails | store error | Logged only; function still succeeds | P2 |  |
| SV-62 | fetchProfileFromMojang | Error Path | signed=true, SetSignedProfileInCache fails | store error | Returns that error | P2 |  |
| SV-63 | fetchProfileFromMojang | Error Path | signed=false, SetProfileInCache fails | store error | Returns that error | P2 |  |
| SV-64 | GetGeyserXUID | Happy Path | DB entry fresh | not stale | Returned without HTTP call | P1 |  |
| SV-65 | GetGeyserXUID | Happy Path | DB stale/missing, Geyser succeeds | XUID nonzero | Upserted and returned with derived UUID | P0 |  |
| SV-66 | GetGeyserXUID | Error Path | Geyser returns 400 | status 400 | Returns ErrInvalidGeyserRequest | P1 |  |
| SV-67 | GetGeyserXUID | Error Path | Geyser returns non-200/400 | status 500 | Returns an error wrapping `ErrGeyserAPI` (message `geyser API error: <status>`) | P2 |  |
| SV-68 | GetGeyserXUID | Error Path | Geyser body is not valid JSON | malformed body | Returns decode error | P2 |  |
| SV-69 | GetGeyserXUID | Edge Case | Response XUID == 0 | unknown gamertag | Returns ErrPlayerNotFound | P1 |  |
| SV-70 | GetGeyserXUID | Error Path | UpsertGeyserPlayer fails | store error | Returns that error | P2 |  |
| SV-71 | GetGeyserXUID | Edge Case | Gamertag contains special characters | gamertag = "a b#c" | Request path is url.PathEscape'd | P2 |  |
| SV-72 | GetGeyserSkin | Happy Path | DB skin fresh | not stale | Returned without HTTP call | P1 |  |
| SV-73 | GetGeyserSkin | Happy Path | DB stale/missing, Geyser succeeds | hash present | Upserted and returned | P0 |  |
| SV-74 | GetGeyserSkin | Error Path | Geyser returns 400 | status 400 | Returns ErrInvalidGeyserRequest | P1 |  |
| SV-75 | GetGeyserSkin | Error Path | Geyser returns non-200/400 | status 500 | Returns an error wrapping `ErrGeyserAPI` (message `geyser API error: <status>`) | P2 |  |
| SV-76 | GetGeyserSkin | Error Path | Geyser body is not valid JSON | malformed body | Returns decode error | P2 |  |
| SV-77 | GetGeyserSkin | Edge Case | Response hash == "" | unconverted player | Returns ErrSkinNotFound | P1 |  |
| SV-78 | GetGeyserSkin | Error Path | UpsertGeyserSkin fails | store error | Returns that error | P2 |  |
| SV-79 | resolveGeyserPlayerByXUID | Happy Path | DB entry fresh | not stale | Returned without HTTP call | P1 |  |
| SV-80 | resolveGeyserPlayerByXUID | Happy Path | DB stale/missing, Geyser succeeds | gamertag present | Upserted and returned | P1 |  |
| SV-81 | resolveGeyserPlayerByXUID | Error Path | Geyser returns 400 | status 400 | Returns ErrInvalidGeyserRequest | P2 |  |
| SV-82 | resolveGeyserPlayerByXUID | Error Path | Geyser returns non-200 | status 500 | Returns an error wrapping `ErrGeyserAPI` (message `geyser API error: <status>`) | P2 |  |
| SV-83 | resolveGeyserPlayerByXUID | Edge Case | Response gamertag == "" | unknown xuid | Returns ErrPlayerNotFound | P1 |  |
| SV-84 | resolveGeyserPlayerByXUID | Error Path | UpsertGeyserPlayer fails | store error | Returns that error | P2 |  |
| SV-85 | GetGeyserProfile | Happy Path | Player and skin both resolve | both succeed | Returns combined GeyserProfile with Skin set | P1 |  |
| SV-86 | GetGeyserProfile | Edge Case | GetGeyserSkin returns ErrSkinNotFound | no skin | Returns GeyserProfile with Skin nil, no error | P2 |  |
| SV-87 | GetGeyserProfile | Error Path | resolveGeyserPlayerByXUID fails | error | Returns that error (an error wrapping `ErrGeyserAPI` for a Geyser 500) | P2 |  |
| SV-88 | GetGeyserProfile | Error Path | GetGeyserSkin fails with a non-ErrSkinNotFound error | store/transport error | Returns that error | P2 | GetGeyserSkin discards the store's own lookup error as a cache-miss signal and falls through to a live fetch; the mocked non-2xx response there is what actually produces the propagated error |
| SV-89 | GetGeyserProfileByGamertag | Happy Path | XUID and skin both resolve | both succeed | Returns combined GeyserProfile with Skin set | P1 |  |
| SV-90 | GetGeyserProfileByGamertag | Edge Case | GetGeyserSkin returns ErrSkinNotFound | no skin | Returns GeyserProfile with Skin nil | P2 |  |
| SV-91 | GetGeyserProfileByGamertag | Error Path | GetGeyserXUID fails | error | Returns that error (an error wrapping `ErrGeyserAPI` for a Geyser 500) | P2 |  |
| SV-92 | GetGeyserProfileByGamertag | Error Path | GetGeyserSkin fails with a non-ErrSkinNotFound error | error | Returns that error | P2 |  |
| SV-93 | GetTextureContent | Happy Path | IsTextureInS3 true | present=true | Delegates to serveFromS3 | P1 |  |
| SV-94 | GetTextureContent | Happy Path | IsTextureInS3 false | present=false | Delegates to fetchAndArchive | P1 |  |
| SV-95 | GetTextureContent | Error Path | IsTextureInS3 fails | store error | Returns that error | P2 |  |
| SV-96 | serveFromS3 | Happy Path | CDN returns 200 | body+content-type | Returns TextureResult with body+content-type | P1 |  |
| SV-97 | serveFromS3 | Error Path | CDN returns 404 | status 404 | Returns ErrTextureNotFound, body closed | P1 |  |
| SV-98 | serveFromS3 | Error Path | CDN returns other non-200 | status 500 | Returns an error wrapping `ErrBadStatusS3`, body closed | P2 |  |
| SV-99 | serveFromS3 | Error Path | http.Client.Get fails | transport error | Returns that error | P2 |  |
| SV-100 | serveFromS3 | Edge Case | Missing Content-Type header | no header | Defaults to "image/png" | P3 |  |
| SV-101 | Close (bytesReadCloser) | Accessor | Called on any bytesReadCloser | any instance | Always returns nil | P3 |  |
| SV-102 | fetchAndArchive | Happy Path | Mojang returns 200, archive succeeds | 200 with bytes | Returns TextureResult with correct bytes+content-type | P0 |  |
| SV-103 | fetchAndArchive | Error Path | Mojang returns 404 | status 404 | Returns ErrTextureNotFound | P1 |  |
| SV-104 | fetchAndArchive | Error Path | Mojang returns other non-200 | status 500 | Returns an error wrapping `ErrBadStatusRemote` | P2 |  |
| SV-105 | fetchAndArchive | Error Path | http.Client.Get fails | transport error | Returns that error | P2 |  |
| SV-106 | fetchAndArchive | Error Path | Response body read fails | erroring body | Returns that error | P2 | The fake server lies about Content-Length then hijacks the connection, since httptest can't otherwise induce a body-read error |
| SV-107 | fetchAndArchive | Edge Case | PutTextureInS3 fails | store error | Logged only; response still succeeds; UpsertTextureHash not called | P2 |  |
| SV-108 | fetchAndArchive | Edge Case | PutTextureInS3 succeeds, UpsertTextureHash fails | store error | Logged only; response still succeeds | P2 |  |
| SV-109 | fetchAndArchive | Edge Case | Missing Content-Type header | no header | Defaults to "image/png" | P3 |  |
| SV-110 | GetGeyserTextureContent | Happy Path | IsGeyserTextureInS3 true | present=true | Delegates to serveGeyserFromS3 | P1 |  |
| SV-111 | GetGeyserTextureContent | Happy Path | IsGeyserTextureInS3 false | present=false | Delegates to fetchAndArchiveGeyserTexture | P1 |  |
| SV-112 | GetGeyserTextureContent | Error Path | IsGeyserTextureInS3 fails | store error | Returns that error | P2 |  |
| SV-113 | serveGeyserFromS3 | Happy Path | CDN returns 200 | body+content-type | Returns TextureResult | P1 |  |
| SV-114 | serveGeyserFromS3 | Error Path | CDN returns 404 | status 404 | Returns ErrTextureNotFound | P1 |  |
| SV-115 | serveGeyserFromS3 | Error Path | CDN returns other non-200 | status 500 | Returns an error wrapping `ErrBadStatusS3` | P2 |  |
| SV-116 | serveGeyserFromS3 | Error Path | http.Client.Get fails | transport error | Returns that error | P2 |  |
| SV-117 | serveGeyserFromS3 | Edge Case | Missing Content-Type header | no header | Defaults to "image/png" | P3 |  |
| SV-118 | fetchAndArchiveGeyserTexture | Happy Path | Skin found with valid SkinURL, HTTP 200 | full success path | Archived and returned with correct bytes | P0 |  |
| SV-119 | fetchAndArchiveGeyserTexture | Error Path | GetGeyserSkinByHash fails | store error | Returns that error | P2 |  |
| SV-120 | fetchAndArchiveGeyserTexture | Edge Case | Skin is nil | hash unknown | Returns ErrTextureNotFound | P1 |  |
| SV-121 | fetchAndArchiveGeyserTexture | Edge Case | skin.SkinURL() == "" | undecodable Value | Returns ErrTextureNotFound | P2 |  |
| SV-122 | fetchAndArchiveGeyserTexture | Error Path | http.Client.Get fails | transport error | Returns that error | P2 |  |
| SV-123 | fetchAndArchiveGeyserTexture | Error Path | Skin host returns 404 | status 404 | Returns ErrTextureNotFound | P2 |  |
| SV-124 | fetchAndArchiveGeyserTexture | Error Path | Skin host returns other non-200 | status 500 | Returns an error wrapping `ErrBadStatusRemote` | P2 |  |
| SV-125 | fetchAndArchiveGeyserTexture | Error Path | Response body read fails | erroring body | Returns that error | P2 |  |
| SV-126 | fetchAndArchiveGeyserTexture | Edge Case | PutGeyserTextureInS3 fails | store error | Logged only; response still succeeds | P2 |  |
| SV-127 | fetchAndArchiveGeyserTexture | Edge Case | Missing Content-Type header | no header | Defaults to "image/png" | P3 |  |

## store.go

Rows marked "(live)" require `TEST_POSTGRES_URL` and/or `TEST_REDIS_URL` and `t.Skip` themselves when unset. Rows marked "(local fake)" exercise a `httptest.Server`/unreachable-port stand-in and never touch a live service, so they are never skipped. Live rows seed their own fixtures via the store's own Upsert* methods (unique per-test IDs), not raw SQL, so they never depend on run order or another row's state.

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ST-01 | NewStore | Accessor | Construct with given db/rdb/s3 | valid args | Returned *store wraps exactly the given values | P3 |  |
| ST-02 | GetPlayerByUUID | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-03 | GetPlayerByUUID | Happy Path (live) | Row seeded via UpsertPlayer | live DB | Returns matching Player (ProfileActions left zero-value: lax mapping) | P1 |  |
| ST-04 | GetPlayerByUUID | Error Path (live) | No matching id | live DB, unknown id | Returns non-nil error | P2 |  |
| ST-05 | GetPlayerByName | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-06 | GetPlayerByName | Happy Path (live) | Row seeded via UpsertPlayer | live DB | Returns matching Player | P1 |  |
| ST-07 | GetPlayerByName | Error Path (live) | No matching name | live DB, unknown name | Returns non-nil error | P2 |  |
| ST-08 | GetProfileByUUID | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-09 | GetProfileByUUID | Happy Path (live) | Player+textures seeded | live DB | Returns Profile with decoded Textures | P1 |  |
| ST-10 | GetProfileByUUID | Edge Case (live) | Player seeded, no textures row | live DB | Returns Profile with Textures == nil | P2 |  |
| ST-11 | GetProfileByUUID | Error Path (live) | No matching player | live DB, unknown id | Returns non-nil error | P2 |  |
| ST-12 | getTextures | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P3 |  |
| ST-13 | getTextures | Happy Path (live) | Texture row present | live DB | Returns decoded *TexturesValue | P2 |  |
| ST-14 | getTextures | Edge Case (live) | No texture row for player | live DB | Returns (nil, nil) | P2 |  |
| ST-15 | UpsertPlayer | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-16 | UpsertPlayer | Happy Path (live) | updateProfile=true, new id | live DB | Row inserted with given profile_actions; name history row inserted | P1 |  |
| ST-17 | UpsertPlayer | Happy Path (live) | updateProfile=false, new id | live DB | Row inserted with legacy=false, demo=false, profile_actions=[] | P1 |  |
| ST-18 | UpsertPlayer | Edge Case (live) | Same id upserted twice | live DB | Second call updates in place; exactly one row for that id | P2 |  |
| ST-19 | UpsertPlayer | Concurrency Invariant (live) | N goroutines UpsertPlayer the same id concurrently | live DB | No unique-violation errors from any goroutine; `players.id` being a PRIMARY KEY then guarantees exactly one surviving row (looped trials) | P0 | 50 concurrent writers — fewer wouldn't reliably reproduce a broken ON CONFLICT guard's race on every run |
| ST-20 | UpsertTextures | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-21 | UpsertTextures | Happy Path (live) | Skin+cape+SLIM model, hashes pre-registered | live DB | Row inserted with model stored | P1 |  |
| ST-22 | UpsertTextures | Edge Case (live) | Same conflict key upserted twice | live DB | Second call updates last_seen; no duplicate row | P2 |  |
| ST-23 | UpsertTextures | Concurrency Invariant (live) | N goroutines UpsertTextures the same conflict key concurrently | live DB | No unique-violation errors from any goroutine; `player_textures_unique` then guarantees a single surviving row (looped trials) | P0 |  |
| ST-24 | textureHash | Happy Path | Texture with a parsable URL | Texture{URL: ".../abc"} | Returns pointer to "abc" | P2 |  |
| ST-25 | textureHash | Edge Case | Texture.Hash() == "" | nil Texture or malformed URL | Returns nil | P2 |  |
| ST-26 | UpsertTextureHash | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-27 | UpsertTextureHash | Happy Path (live) | New hash | live DB | Row inserted | P1 |  |
| ST-28 | UpsertTextureHash | Edge Case (live) | Duplicate hash | live DB | ON CONFLICT DO NOTHING; no error | P2 |  |
| ST-76 | UpsertTextureHash | Concurrency Invariant (live) | N goroutines UpsertTextureHash the same hash concurrently | live DB | No unique-violation errors from any goroutine; `textures.hash` being a PRIMARY KEY then guarantees exactly one surviving row (looped trials) | P0 | 50 concurrent writers — fewer wouldn't reliably reproduce a broken ON CONFLICT guard's race on every run |
| ST-29 | GetPlayerFromCache | Error Path | Redis unreachable | closed-port client | Raw connection error passed through unchanged | P2 |  |
| ST-30 | GetPlayerFromCache | Happy Path (live) | Key set via SetPlayerInCache | live Redis | Returns decoded *Player | P1 |  |
| ST-31 | GetPlayerFromCache | Error Path (live) | Missing key | live Redis, unknown key | Returns redis.Nil | P1 |  |
| ST-32 | GetPlayerFromCache | Error Path (live) | Value is not valid JSON | live Redis, malformed value | Returns unmarshal error | P2 |  |
| ST-33 | SetPlayerInCache | Error Path | Redis unreachable | closed-port client | Raw connection error passed through unchanged | P2 |  |
| ST-34 | SetPlayerInCache | Happy Path (live) | Valid player | live Redis | Both ID-keyed and Name-keyed entries set with TTL, readable back | P1 |  |
| ST-35 | GetProfileFromCache | Error Path | Redis unreachable | closed-port client | Raw connection error passed through unchanged | P2 |  |
| ST-36 | GetProfileFromCache | Happy Path (live) | Key set via SetProfileInCache | live Redis | Returns decoded *Profile | P1 |  |
| ST-37 | GetProfileFromCache | Error Path (live) | Missing key | live Redis, unknown key | Returns redis.Nil | P2 |  |
| ST-38 | SetProfileInCache | Error Path | Redis unreachable | closed-port client | Raw connection error passed through unchanged | P2 |  |
| ST-39 | SetProfileInCache | Happy Path (live) | Valid profile | live Redis | Key set with TTL, readable back | P1 |  |
| ST-40 | GetSignedProfileFromCache | Error Path | Redis unreachable | closed-port client | Raw connection error passed through unchanged | P2 |  |
| ST-41 | GetSignedProfileFromCache | Happy Path (live) | Key set via SetSignedProfileInCache | live Redis | Returns decoded *Player | P1 |  |
| ST-42 | GetSignedProfileFromCache | Error Path (live) | Missing key | live Redis, unknown key | Returns redis.Nil | P2 |  |
| ST-43 | SetSignedProfileInCache | Error Path | Redis unreachable | closed-port client | Raw connection error passed through unchanged | P2 |  |
| ST-44 | SetSignedProfileInCache | Happy Path (live) | Valid player | live Redis | Key set with TTL, readable back | P1 |  |
| ST-45 | IsTextureInS3 | Happy Path (local fake) | HeadObject returns 200 | fake S3 server | Returns (true, nil) | P1 |  |
| ST-46 | IsTextureInS3 | Edge Case (local fake) | HeadObject returns 404 | fake S3 server | Returns (false, nil) | P1 |  |
| ST-47 | IsTextureInS3 | Error Path (local fake) | HeadObject returns 500 | fake S3 server | Returns (false, non-nil error) | P2 |  |
| ST-48 | GetGeyserPlayerByGamertag | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-49 | GetGeyserPlayerByGamertag | Happy Path (live) | Row seeded via UpsertGeyserPlayer | live DB | Returns GeyserPlayer with derived UUID | P1 |  |
| ST-50 | GetGeyserPlayerByGamertag | Error Path (live) | No matching gamertag | live DB, unknown gamertag | Returns non-nil error | P2 |  |
| ST-51 | GetGeyserPlayerByXUID | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-52 | GetGeyserPlayerByXUID | Happy Path (live) | Row seeded via UpsertGeyserPlayer | live DB | Returns GeyserPlayer with derived UUID | P1 |  |
| ST-53 | GetGeyserPlayerByXUID | Error Path (live) | No matching xuid | live DB, unknown xuid | Returns non-nil error | P2 |  |
| ST-54 | UpsertGeyserPlayer | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-55 | UpsertGeyserPlayer | Happy Path (live) | New xuid | live DB | Row inserted | P1 |  |
| ST-56 | UpsertGeyserPlayer | Edge Case (live) | Same xuid upserted twice | live DB | Second call updates gamertag/last_seen in place | P2 |  |
| ST-57 | UpsertGeyserPlayer | Concurrency Invariant (live) | N goroutines UpsertGeyserPlayer the same xuid concurrently | live DB | No unique-violation errors from any goroutine; `geyser_players`' xuid PRIMARY KEY then guarantees a single surviving row (looped trials) | P0 |  |
| ST-58 | GetGeyserSkin | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-59 | GetGeyserSkin | Happy Path (live) | Row seeded via UpsertGeyserSkin | live DB | Returns most-recently-seen skin | P1 |  |
| ST-60 | GetGeyserSkin | Error Path (live) | No matching xuid | live DB, unknown xuid | Returns non-nil error | P2 |  |
| ST-61 | GetGeyserSkinByHash | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-62 | GetGeyserSkinByHash | Happy Path (live) | Row seeded via UpsertGeyserSkin | live DB | Returns matching skin | P1 |  |
| ST-63 | GetGeyserSkinByHash | Edge Case (live) | No matching hash | live DB, unknown hash | Returns (nil, nil) | P2 |  |
| ST-64 | UpsertGeyserSkin | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-65 | UpsertGeyserSkin | Happy Path (live) | Empty Signature | live DB | Stored as NULL; read back as "" via COALESCE | P1 |  |
| ST-66 | UpsertGeyserSkin | Edge Case (live) | Same (xuid,hash) upserted twice | live DB | Second call updates fields in place | P2 |  |
| ST-67 | UpsertGeyserSkin | Concurrency Invariant (live) | N goroutines UpsertGeyserSkin the same (xuid,hash) concurrently | live DB | No unique-violation errors from any goroutine; `geyser_player_textures_unique` then guarantees a single surviving row (looped trials) | P0 |  |
| ST-68 | PutTextureInS3 | Happy Path (local fake) | Body implements Len(); PUT returns 200 | fake S3 server | Content-Length header equals Len(); nil error | P1 |  |
| ST-69 | PutTextureInS3 | Edge Case (local fake) | Body does not implement Len() | fake S3 server | ContentLength left unset; still succeeds | P2 |  |
| ST-70 | PutTextureInS3 | Error Path (local fake) | PUT returns 500 | fake S3 server | Returns an error wrapping `ErrUploadS3` (message `failed to upload to s3: <cause>`) | P2 |  |
| ST-77 | PutTextureInS3 | Error Path (local fake) | PUT returns 500, pinning the message | fake S3 server | The error text starts with `failed to upload to s3: ` | P2 | |
| ST-71 | IsGeyserTextureInS3 | Happy Path (local fake) | HeadObject returns 200 at Geyser key | fake S3 server | Returns (true, nil); request key uses GeyserS3KeyPrefix | P1 |  |
| ST-72 | IsGeyserTextureInS3 | Edge Case (local fake) | HeadObject returns 404 | fake S3 server | Returns (false, nil) | P2 |  |
| ST-73 | IsGeyserTextureInS3 | Error Path (local fake) | HeadObject returns 500 | fake S3 server | Returns (false, non-nil error) | P2 |  |
| ST-74 | PutGeyserTextureInS3 | Happy Path (local fake) | PUT returns 200 | fake S3 server | nil error; request key uses GeyserS3KeyPrefix | P1 |  |
| ST-75 | PutGeyserTextureInS3 | Error Path (local fake) | PUT returns 500 | fake S3 server | Returns an error wrapping `ErrUploadS3` (message `failed to upload to s3: <cause>`) | P2 |  |
