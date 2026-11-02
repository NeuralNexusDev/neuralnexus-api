# Test plan: minecraft

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| TY-01 | MarshalJSON | Happy Path | Player.ProfileActions is nil | MojangPlayer{ProfileActions: nil} | Marshaled JSON has no `profileActions` key | P2 |  |
| TY-02 | MarshalJSON | Edge Case | Player.ProfileActions is a non-nil empty slice | MojangPlayer{ProfileActions: []string{}} | Marshaled JSON has `"profileActions":[]` | P2 |  |
| TY-03 | MarshalJSON | Edge Case | Player.ProfileActions is populated | MojangPlayer{ProfileActions: []string{"a"}} | Marshaled JSON has `"profileActions":["a"]` | P3 |  |
| TY-04 | ParseProperties | Happy Path | Properties has one valid TEXTURES entry | base64+JSON-encoded TexturesValue in Property.Value | Returns decoded *TexturesValue matching input | P1 |  |
| TY-05 | ParseProperties | Edge Case | Properties is empty | MojangPlayer{Properties: nil} | Returns nil | P3 |  |
| TY-06 | ParseProperties | Edge Case | Property.Name != TEXTURES | Property{Name: "other"} | Entry skipped, returns nil | P3 |  |
| TY-07 | ParseProperties | Error Path | Property.Value is not valid base64 | Property{Name: TEXTURES, Value: "not-base64!!"} | Entry skipped (logged), returns nil | P2 |  |
| TY-08 | ParseProperties | Error Path | Property.Value is valid base64 but not valid JSON | base64("not json") | Entry skipped (logged), returns nil | P2 |  |
| TY-09 | ParseProperties | Edge Case | First property invalid, second is a valid TEXTURES entry | two Properties, first malformed | Returns nil: the first TEXTURES entry decides the result, so the second is never read | P2 |  |
| TY-10 | IsStale (Player) | Happy Path | LastSeen is recent | LastSeen = now | Returns false | P2 |  |
| TY-11 | IsStale (Player) | Edge Case | LastSeen older than stalenessThreshold | LastSeen = now - 25h | Returns true | P2 |  |
| TY-12 | ToProfile | Happy Path | Player has a valid TEXTURES property | populated Player | Returned *Profile mirrors ID/Name/Legacy/Demo/ProfileActions, Textures = ParseProperties() result | P1 |  |
| TY-13 | ToProfile | Edge Case | Player has no properties | MojangPlayer{Properties: nil} | Returned Profile.Textures is nil | P3 |  |
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
| TY-25 | ToPlayer | Edge Case | Profile.Textures is nil | MojangProfile{Textures: nil} | Returned Player.Properties is empty/nil, nil error | P3 |  |
| TY-26 | IsStale (GeyserPlayer) | Happy Path | LastSeen is recent | LastSeen = now | Returns false | P3 |  |
| TY-27 | IsStale (GeyserPlayer) | Edge Case | LastSeen older than stalenessThreshold | LastSeen = now - 25h | Returns true | P3 |  |
| TY-28 | IsStale (GeyserSkin) | Happy Path | LastSeen is recent | LastSeen = now | Returns false | P3 |  |
| TY-29 | IsStale (GeyserSkin) | Edge Case | LastSeen older than stalenessThreshold | LastSeen = now - 25h | Returns true | P3 |  |
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
| HD-02 | handler.GetMojangPlayerByName | Happy Path | Valid name, service succeeds | mock returns *MojangPlayer | 200 OK with player JSON body | P1 |  |
| HD-03 | handler.GetMojangPlayerByName | Error Path | Service returns ErrPlayerNotFound | mock error = ErrPlayerNotFound | 404 Not Found, `detail` is `msgPlayerNotFound` | P1 |  |
| HD-04 | handler.GetMojangPlayerByName | Error Path | Service returns a generic error | mock error = errors.New(...) | 500 Internal Server Error, `detail` is `msgFailedToGetPlayer` | P2 |  |
| HD-05 | handler.GetMojangPlayerByUUID | Error Path | Invalid UUID path value | r.PathValue("uuid") == "not-a-uuid" | 400 Bad Request, `detail` is `msgNotAValidUUID` followed by the id | P2 |  |
| HD-06 | handler.GetMojangPlayerByUUID | Happy Path | Valid UUID, service succeeds | mock returns *MojangPlayer | 200 OK with player JSON body | P1 |  |
| HD-07 | handler.GetMojangPlayerByUUID | Error Path | Service returns ErrPlayerNotFound | mock error = ErrPlayerNotFound | 404 Not Found, `detail` is `msgPlayerNotFound` | P1 |  |
| HD-08 | handler.GetMojangPlayerByUUID | Error Path | Service returns a generic error | mock error = errors.New(...) | 500 Internal Server Error, `detail` is `msgFailedToGetPlayer` | P2 |  |
| HD-09 | handler.GetMojangPlayersByNames | Error Path | Content-Type header is not application/json | header omitted | 415 Unsupported Media Type (`msgRequestMustBeJSON`) | P2 |  |
| HD-10 | handler.GetMojangPlayersByNames | Error Path | Body is not valid JSON | body = "{not json" | 400 Bad Request (`msgInvalidRequestBody`) | P2 |  |
| HD-11 | handler.GetMojangPlayersByNames | Error Path | names array is empty | body = "[]" | 400 Bad Request (`msgInvalidSize`) | P2 |  |
| HD-12 | handler.GetMojangPlayersByNames | Error Path | names array has more than 10 entries | 11 names | 400 Bad Request (`msgInvalidSize`) | P2 |  |
| HD-13 | handler.GetMojangPlayersByNames | Error Path | names array contains an empty string | ["a", ""] | 400 Bad Request (`msgInvalidProfileName`) | P3 |  |
| HD-14 | handler.GetMojangPlayersByNames | Happy Path | Valid names, service succeeds | mock returns []*MojangPlayer | 200 OK with players JSON body | P1 |  |
| HD-15 | handler.GetMojangPlayersByNames | Error Path | Service returns an error | mock error | 500 Internal Server Error (`msgFailedToGetPlayers`) | P2 |  |
| HD-16 | handler.GetMojangProfile | Error Path | Invalid UUID path value | r.PathValue("uuid") invalid | 400 Bad Request, `detail` is `msgNotAValidUUID` followed by the id | P2 |  |
| HD-17 | handler.GetMojangProfile | Happy Path | No "unsigned" query param, service succeeds | query omitted | signed=false passed to service; 200 OK | P1 |  |
| HD-18 | handler.GetMojangProfile | Edge Case | Query "unsigned=false" | ?unsigned=false | signed=true passed to service | P2 |  |
| HD-19 | handler.GetMojangProfile | Error Path | Service returns ErrPlayerNotFound | mock error | 204 No Content | P1 |  |
| HD-20 | handler.GetMojangProfile | Error Path | Service returns a generic error | mock error | 500 Internal Server Error, `detail` is `msgFailedToGetPlayerProfile` | P2 |  |
| HD-21 | handler.GetProfile | Error Path | Invalid UUID path value | invalid uuid | 400 Bad Request, `detail` is `msgNotAValidUUID` followed by the id | P2 |  |
| HD-22 | handler.GetProfile | Happy Path | Valid UUID, service succeeds | mock returns *MojangProfile | 200 OK with profile JSON body | P1 |  |
| HD-23 | handler.GetProfile | Error Path | Service returns ErrPlayerNotFound | mock error | 204 No Content | P1 |  |
| HD-24 | handler.GetProfile | Error Path | Service returns a generic error | mock error | 500 Internal Server Error, `detail` is `msgFailedToGetPlayerProfile` | P2 |  |
| HD-26 | handler.GetProfileByName | Happy Path | Valid name, service succeeds | mock returns *MojangProfile | 200 OK with profile JSON body | P1 |  |
| HD-27 | handler.GetProfileByName | Error Path | Service returns ErrPlayerNotFound | mock error | 204 No Content | P1 |  |
| HD-28 | handler.GetProfileByName | Error Path | Service returns a generic error | mock error | 500 Internal Server Error, `detail` is `msgFailedToGetPlayerProfile` | P2 |  |
| HD-30 | handler.GetGeyserPlayerByGamertag | Happy Path | Valid gamertag, service succeeds | mock returns *GeyserPlayer | 200 OK with player JSON body | P1 |  |
| HD-31 | handler.GetGeyserPlayerByGamertag | Error Path | Service returns ErrPlayerNotFound | mock error | 404 Not Found, `detail` is `msgPlayerNotFound` | P1 |  |
| HD-32 | handler.GetGeyserPlayerByGamertag | Error Path | Service returns ErrInvalidGeyserRequest | mock error | 400 Bad Request, `detail` is `msgInvalidGamertag` | P2 |  |
| HD-33 | handler.GetGeyserPlayerByGamertag | Error Path | Service returns a generic error | mock error | 500 Internal Server Error (`msgFailedToGetGeyserXUID`) | P2 |  |
| HD-34 | handler.GetGeyserSkin | Error Path | Non-numeric xuid path value | xuid == "abc" | 400 Bad Request, `detail` is `msgInvalidXuid` | P2 |  |
| HD-35 | handler.GetGeyserSkin | Happy Path | Valid xuid, service succeeds | mock returns *GeyserSkin | 200 OK with skin JSON body | P1 |  |
| HD-36 | handler.GetGeyserSkin | Error Path | Service returns ErrSkinNotFound | mock error | 204 No Content | P1 |  |
| HD-37 | handler.GetGeyserSkin | Error Path | Service returns ErrInvalidGeyserRequest | mock error | 400 Bad Request, `detail` is `msgInvalidXuid` | P2 |  |
| HD-38 | handler.GetGeyserSkin | Error Path | Service returns a generic error | mock error | 500 Internal Server Error (`msgFailedToGetGeyserSkin`) | P2 |  |
| HD-39 | handler.GetGeyserProfile | Error Path | uuid path value is not a derived Bedrock UUID | uuidToXUID fails | 400 Bad Request (`msgNotAValidBedrockUUID`) | P2 |  |
| HD-40 | handler.GetGeyserProfile | Happy Path | Valid Bedrock UUID, service succeeds | mock returns *GeyserProfile | 200 OK with profile JSON body | P1 |  |
| HD-41 | handler.GetGeyserProfile | Error Path | Service returns ErrPlayerNotFound | mock error | 404 Not Found, `detail` is `msgPlayerNotFound` | P1 |  |
| HD-42 | handler.GetGeyserProfile | Error Path | Service returns ErrInvalidGeyserRequest | mock error | 400 Bad Request, `detail` is `msgInvalidXuid` | P2 |  |
| HD-43 | handler.GetGeyserProfile | Error Path | Service returns a generic error | mock error | 500 Internal Server Error, `detail` is `msgFailedToGetGeyserProfile` | P2 |  |
| HD-45 | handler.GetGeyserProfileByName | Happy Path | Valid gamertag, service succeeds | mock returns *GeyserProfile | 200 OK with profile JSON body | P1 |  |
| HD-46 | handler.GetGeyserProfileByName | Error Path | Service returns ErrPlayerNotFound | mock error | 404 Not Found, `detail` is `msgPlayerNotFound` | P1 |  |
| HD-47 | handler.GetGeyserProfileByName | Error Path | Service returns ErrInvalidGeyserRequest | mock error | 400 Bad Request, `detail` is `msgInvalidGamertag` | P2 |  |
| HD-48 | handler.GetGeyserProfileByName | Error Path | Service returns a generic error | mock error | 500 Internal Server Error, `detail` is `msgFailedToGetGeyserProfile` | P2 |  |
| HD-50 | handler.GetTexture | Happy Path | Valid hash, service succeeds | mock returns *TextureResult | 200 OK, body bytes copied verbatim, Content-Type header set | P1 |  |
| HD-51 | handler.GetTexture | Error Path | Service returns ErrTextureNotFound | mock error | 404 Not Found, `detail` is `msgTextureNotFound` | P1 |  |
| HD-52 | handler.GetTexture | Error Path | Service returns a generic error | mock error | 502 Bad Gateway, `detail` is `msgFailedToGetTexture` | P2 |  |
| HD-57 | handler.GetGeyserProfileByName | Error Path | Service returns `ErrGeyserAPI` | mock service returns `nil, ErrGeyserAPI` | 500 Internal Server Error, `detail` is `msgFailedToGetGeyserProfile` | P1 |  |
| HD-58 | handler.GetGeyserProfile | Error Path | Service returns `ErrGeyserAPI` | mock service returns `nil, ErrGeyserAPI` | 500 Internal Server Error, `detail` is `msgFailedToGetGeyserProfile` | P1 |  |

## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SV-01 | NewService | Happy Path | client is nil | client == nil | Returned service uses http.DefaultClient | P2 |  |
| SV-02 | NewService | Edge Case | client is provided | non-nil client | Retained as-is; s3TextureUrl is the URL passed to NewService | P3 |  |
| SV-03 | mojangService.GetMojangPlayerByName | Happy Path | Cache hit | store cache returns player, nil | Returns cached player; no DB/HTTP calls | P1 |  |
| SV-04 | mojangService.GetMojangPlayerByName | Error Path | Cache error is not ErrCacheMiss | store cache error = plain error | Returns that error, no fallback | P2 |  |
| SV-05 | mojangService.GetMojangPlayerByName | Happy Path | Cache miss, DB entry fresh | cache=ErrCacheMiss, DB player not stale | Caches and returns DB player, no Mojang call | P1 |  |
| SV-06 | mojangService.GetMojangPlayerByName | Error Path | DB fresh entry, cacheStore.SetPlayerInCache fails | cacheStore.SetPlayerInCache error | Returns that error | P2 |  |
| SV-07 | mojangService.GetMojangPlayerByName | Happy Path | Cache+DB miss/stale, Mojang succeeds | Mojang 200 with player JSON | Upserts, caches, returns fetched player | P0 |  |
| SV-08 | mojangService.GetMojangPlayerByName | Error Path | Mojang returns 404 | Mojang status 404 | Returns ErrPlayerNotFound | P1 |  |
| SV-09 | mojangService.GetMojangPlayerByName | Error Path | Mojang returns non-200/404 | Mojang status 500 | Returns an error wrapping `ErrMojangAPI` | P2 |  |
| SV-10 | mojangService.GetMojangPlayerByName | Error Path | Mojang body is not valid JSON | malformed body | Returns decode error | P2 |  |
| SV-11 | mojangService.GetMojangPlayerByName | Error Path | mojangStore.UpsertPlayer fails after Mojang fetch | store error | Returns that error | P2 |  |
| SV-12 | mojangService.GetMojangPlayerByName | Error Path | cacheStore.SetPlayerInCache fails after Mojang fetch | store error | Returns that error | P2 |  |
| SV-13 | mojangService.GetMojangPlayerByName | Error Path | http.Client.Get fails | transport error | Returns that error | P2 |  |
| SV-14 | mojangService.GetMojangPlayerByUUID | Happy Path | Cache hit | store cache returns player | Returns cached player; no DB/HTTP calls | P1 |  |
| SV-15 | mojangService.GetMojangPlayerByUUID | Error Path | Cache error is not ErrCacheMiss | plain error | Returns that error | P2 |  |
| SV-16 | mojangService.GetMojangPlayerByUUID | Happy Path | Cache miss, DB entry fresh | DB player not stale | Caches and returns DB player | P1 |  |
| SV-17 | mojangService.GetMojangPlayerByUUID | Happy Path | Cache+DB miss, Mojang succeeds | Mojang 200 | Upserts, caches, returns fetched player | P0 |  |
| SV-18 | mojangService.GetMojangPlayerByUUID | Error Path | Mojang returns 404 | Mojang status 404 | Returns ErrPlayerNotFound | P1 |  |
| SV-19 | mojangService.GetMojangPlayersByNames | Error Path | names is empty | len(names)==0 | Returns `ErrNoNamesProvided` | P1 |  |
| SV-20 | mojangService.GetMojangPlayersByNames | Error Path | names has more than 10 entries | len(names)==11 | Returns `ErrBatchLimit` | P1 |  |
| SV-21 | mojangService.GetMojangPlayersByNames | Happy Path | All names are cache hits | every cacheStore.GetPlayerFromCache succeeds | Returns players from cache; no Mojang call | P1 |  |
| SV-22 | mojangService.GetMojangPlayersByNames | Happy Path | Cache miss but DB entry fresh for all names | cache=ErrCacheMiss, DB fresh | Resolved via DB+cache-set; no Mojang call | P1 |  |
| SV-23 | mojangService.GetMojangPlayersByNames | Error Path | DB-fresh path cacheStore.SetPlayerInCache fails | store error | Returns that error | P2 |  |
| SV-24 | mojangService.GetMojangPlayersByNames | Happy Path | Some names miss cache+DB | Mojang bulk 200 with fetched players | Upserts+caches fetched entries, appended to result | P0 |  |
| SV-25 | mojangService.GetMojangPlayersByNames | Error Path | Mojang bulk POST fails | transport error | Returns that error | P2 |  |
| SV-26 | mojangService.GetMojangPlayersByNames | Error Path | Mojang bulk returns non-200 | status 500 | Returns an error wrapping `ErrMojangAPI` | P2 |  |
| SV-27 | mojangService.GetMojangPlayersByNames | Error Path | Mojang bulk body is not valid JSON | malformed body | Returns decode error | P2 |  |
| SV-28 | mojangService.GetMojangPlayersByNames | Error Path | mojangStore.UpsertPlayer fails on a fetched entry | store error | Returns that error | P2 |  |
| SV-29 | mojangService.GetMojangPlayersByNames | Error Path | cacheStore.SetPlayerInCache fails on a fetched entry | store error | Returns that error | P2 |  |
| SV-30 | mojangService.GetMojangProfile | Happy Path | signed=true, cache hit | cacheStore.GetSignedProfileFromCache succeeds | Returns cached signed player | P1 |  |
| SV-31 | mojangService.GetMojangProfile | Error Path | signed=true, cache error not ErrCacheMiss | plain error | Returns that error | P2 |  |
| SV-32 | mojangService.GetMojangProfile | Happy Path | signed=true, cache miss | mojangService.fetchProfileFromMojang(id,true) succeeds | Returns fetched player | P1 |  |
| SV-33 | mojangService.GetMojangProfile | Error Path | signed=true, mojangService.fetchProfileFromMojang fails | mojang error | Returns that error (`ErrPlayerNotFound`) | P2 |  |
| SV-34 | mojangService.GetMojangProfile | Happy Path | signed=false | mojangService.resolveProfile succeeds | Returns profile.ToPlayer() result | P1 |  |
| SV-35 | mojangService.GetMojangProfile | Error Path | signed=false, mojangService.resolveProfile fails | mojangService.resolveProfile error | Returns that error | P2 |  |
| SV-36 | mojangService.GetProfile | Happy Path | mojangService.resolveProfile succeeds, SKIN+CAPE present | Textures with both | Both URLs rewritten to s3TextureUrl+"texture/"+hash | P1 |  |
| SV-37 | mojangService.GetProfile | Edge Case | mojangService.resolveProfile succeeds, Textures nil | Profile.Textures == nil | Returned unmodified, no panic | P2 |  |
| SV-38 | mojangService.GetProfile | Error Path | mojangService.resolveProfile fails | mojangService.resolveProfile error | Returns that error | P2 |  |
| SV-39 | mojangService.GetProfile | Edge Case | Textures present, CAPE nil | only SKIN set | Only SKIN.URL rewritten | P3 |  |
| SV-40 | mojangService.GetProfileByName | Happy Path | Name resolves, then profile resolves | both succeed | Returns resolved profile | P1 |  |
| SV-41 | mojangService.GetProfileByName | Error Path | mojangService.GetMojangPlayerByName fails | e.g. ErrPlayerNotFound | Returns `ErrPlayerNotFound` | P2 |  |
| SV-42 | mojangService.GetProfileByName | Error Path | mojangService.GetProfile fails after name resolves | mojangService.GetProfile error | Returns that error | P2 |  |
| SV-43 | mojangService.resolveProfile | Happy Path | Cache hit | cacheStore.GetProfileFromCache succeeds | Returns cached profile | P1 |  |
| SV-44 | mojangService.resolveProfile | Error Path | Cache error not ErrCacheMiss | plain error | Returns that error | P2 |  |
| SV-45 | mojangService.resolveProfile | Happy Path | DB profile fresh | not stale | Caches and returns DB profile | P1 |  |
| SV-46 | mojangService.resolveProfile | Edge Case | DB profile fresh, ProfileActions nil | ProfileActions == nil | Normalized to []string{} before caching/return | P2 |  |
| SV-47 | mojangService.resolveProfile | Error Path | DB profile fresh, cacheStore.SetProfileInCache fails | store error | Returns that error | P2 |  |
| SV-48 | mojangService.resolveProfile | Edge Case | DB error is not ErrPlayerNotFound | e.g. connection error | Logged, falls through to Mojang fetch | P2 |  |
| SV-49 | mojangService.resolveProfile | Happy Path | DB miss/stale | dbProfile nil or stale | Calls fetchProfileFromMojang(id,false) | P1 |  |
| SV-50 | mojangService.resolveProfile | Error Path | mojangService.fetchProfileFromMojang fails | mojang error | Returns that error | P2 |  |
| SV-51 | mojangService.fetchProfileFromMojang | Happy Path | signed=false, Mojang succeeds | 200 with textures | Upserts player, stores textures+hashes, sets profile cache, returns player+profile | P0 |  |
| SV-52 | mojangService.fetchProfileFromMojang | Happy Path | signed=true, Mojang succeeds | 200 | Request URL has "?unsigned=false"; sets signed cache instead of profile cache | P1 |  |
| SV-53 | mojangService.fetchProfileFromMojang | Error Path | http.Client.Get fails | transport error | Returns that error | P2 |  |
| SV-54 | mojangService.fetchProfileFromMojang | Error Path | Mojang returns 204 | status 204 | Returns ErrPlayerNotFound | P1 |  |
| SV-55 | mojangService.fetchProfileFromMojang | Error Path | Mojang returns non-200/204 | status 500 | Returns an error wrapping `ErrMojangAPI` | P2 |  |
| SV-56 | mojangService.fetchProfileFromMojang | Error Path | Mojang body is not valid JSON | malformed body | Returns decode error | P2 |  |
| SV-57 | mojangService.fetchProfileFromMojang | Edge Case | Response ProfileActions is nil | ProfileActions == nil | Normalized to []string{} | P2 |  |
| SV-58 | mojangService.fetchProfileFromMojang | Error Path | mojangStore.UpsertPlayer fails | store error | Returns that error; texture processing skipped | P1 |  |
| SV-59 | mojangService.fetchProfileFromMojang | Edge Case | profile.Textures is nil | no TEXTURES property | Texture-storage block skipped, still succeeds | P2 |  |
| SV-60 | mojangService.fetchProfileFromMojang | Edge Case | textureStore.UpsertTextureHash fails for skin/cape | store error | Logged only; function still succeeds | P2 |  |
| SV-61 | mojangService.fetchProfileFromMojang | Edge Case | textureStore.UpsertTextures fails | store error | Logged only; function still succeeds | P2 |  |
| SV-62 | mojangService.fetchProfileFromMojang | Error Path | signed=true, cacheStore.SetSignedProfileInCache fails | store error | Returns that error | P2 |  |
| SV-63 | mojangService.fetchProfileFromMojang | Error Path | signed=false, cacheStore.SetProfileInCache fails | store error | Returns that error | P2 |  |
| SV-64 | geyserService.GetGeyserPlayerByGamertag | Happy Path | DB entry fresh | not stale | Returned without HTTP call | P1 |  |
| SV-65 | geyserService.GetGeyserPlayerByGamertag | Happy Path | DB stale/missing, Geyser succeeds | XUID nonzero | Upserted and returned with derived UUID | P0 |  |
| SV-66 | geyserService.GetGeyserPlayerByGamertag | Error Path | Geyser returns 400 | status 400 | Returns ErrInvalidGeyserRequest | P1 |  |
| SV-67 | geyserService.GetGeyserPlayerByGamertag | Error Path | Geyser returns non-200/400 | status 500 | Returns an error wrapping `ErrGeyserAPI` | P2 |  |
| SV-68 | geyserService.GetGeyserPlayerByGamertag | Error Path | Geyser body is not valid JSON | malformed body | Returns decode error | P2 |  |
| SV-69 | geyserService.GetGeyserPlayerByGamertag | Edge Case | Response XUID == 0 | unknown gamertag | Returns ErrPlayerNotFound | P1 |  |
| SV-70 | geyserService.GetGeyserPlayerByGamertag | Error Path | geyserStore.UpsertGeyserPlayer fails | store error | Returns that error | P2 |  |
| SV-71 | geyserService.GetGeyserPlayerByGamertag | Edge Case | Gamertag contains special characters | gamertag = "a b#c" | Request path is url.PathEscape'd | P2 |  |
| SV-79 | geyserService.GetGeyserPlayerByXUID | Happy Path | DB entry fresh | not stale | Returned without HTTP call | P1 |  |
| SV-80 | geyserService.GetGeyserPlayerByXUID | Happy Path | DB stale/missing, Geyser succeeds | gamertag present | Upserted and returned | P1 |  |
| SV-81 | geyserService.GetGeyserPlayerByXUID | Error Path | Geyser returns 400 | status 400 | Returns ErrInvalidGeyserRequest | P2 |  |
| SV-82 | geyserService.GetGeyserPlayerByXUID | Error Path | Geyser returns non-200 | status 500 | Returns an error wrapping `ErrGeyserAPI` | P2 |  |
| SV-83 | geyserService.GetGeyserPlayerByXUID | Edge Case | Response gamertag == "" | unknown xuid | Returns ErrPlayerNotFound | P1 |  |
| SV-84 | geyserService.GetGeyserPlayerByXUID | Error Path | geyserStore.UpsertGeyserPlayer fails | store error | Returns that error | P2 |  |
| SV-93 | textureService.GetTextureContent | Happy Path | s3store.IsTextureInS3 true | present=true | Serves the texture from the S3 CDN | P1 |  |
| SV-94 | textureService.GetTextureContent | Happy Path | s3store.IsTextureInS3 false | present=false | Fetches the texture from Mojang and archives it | P1 |  |
| SV-95 | textureService.GetTextureContent | Error Path | s3store.IsTextureInS3 fails | store error | Returns that error | P2 |  |
| SV-96 | textureService.GetTextureContent | Happy Path | CDN returns 200 | body+content-type | Returns TextureResult with body+content-type | P1 |  |
| SV-97 | textureService.GetTextureContent | Error Path | CDN returns 404 | status 404 | Returns ErrTextureNotFound, body closed | P1 |  |
| SV-98 | textureService.GetTextureContent | Error Path | CDN returns other non-200 | status 500 | Returns an error wrapping `ErrBadStatusS3`, body closed | P2 |  |
| SV-99 | textureService.GetTextureContent | Error Path | http.Client.Get fails | transport error | Returns that error | P2 |  |
| SV-100 | textureService.GetTextureContent | Edge Case | Missing Content-Type header | no header | Defaults to "image/png" | P3 |  |
| SV-102 | textureService.GetTextureContent | Happy Path | Mojang returns 200, archive succeeds | 200 with bytes | Returns TextureResult with correct bytes+content-type | P0 |  |
| SV-103 | textureService.GetTextureContent | Error Path | Mojang returns 404 | status 404 | Returns ErrTextureNotFound | P1 |  |
| SV-104 | textureService.GetTextureContent | Error Path | Mojang returns other non-200 | status 500 | Returns an error wrapping `ErrBadStatusRemote` | P2 |  |
| SV-105 | textureService.GetTextureContent | Error Path | http.Client.Get fails | transport error | Returns that error | P2 |  |
| SV-106 | textureService.GetTextureContent | Error Path | Response body read fails | erroring body | Returns that error | P2 | The fake server lies about Content-Length then hijacks the connection, since httptest can't otherwise induce a body-read error |
| SV-107 | textureService.GetTextureContent | Edge Case | s3store.PutTextureInS3 fails | store error | Logged only; response still succeeds; UpsertTextureHash not called | P2 |  |
| SV-108 | textureService.GetTextureContent | Edge Case | s3store.PutTextureInS3 succeeds, textureStore.UpsertTextureHash fails | store error | Logged only; response still succeeds | P2 |  |
| SV-109 | textureService.GetTextureContent | Edge Case | Missing Content-Type header | no header | Defaults to "image/png" | P3 |  |
| SV-130 | geyserService.GetGeyserProfile | Error Path | Player lookup by xuid answers 400 | empty DB; fake Geyser endpoint answers 400 | Error is `ErrInvalidGeyserRequest` | P2 |  |
| SV-131 | geyserService.GetGeyserProfileByName | Error Path | Gamertag lookup answers 400 | empty DB; fake Geyser endpoint answers 400 | Error is `ErrInvalidGeyserRequest` | P2 |  |
| SV-135 | mojangService.GetMojangPlayerByName | Error Path | Cancelling the ctx stops the outbound GET | cache and DB miss; fake Mojang endpoint blocks until the client goes away; ctx cancelled once the endpoint is reached | Returns an error wrapping `context.Canceled` without waiting for the endpoint | P1 |  |
| SV-136 | mojangService.GetMojangPlayerByName | Happy Path | Writes after a fetch run on a ctx that outlives the cancelled request | ctx carries a marker value and is cancelled once the Mojang body is read; store records `ctx.Err()` and the marker in `mojangStore.UpsertPlayer` and `cacheStore.SetPlayerInCache` | Both writes see `ctx.Err() == nil` and the marker value | P1 |  |
| SV-137 | mojangService.GetMojangPlayersByNames | Error Path | Cancelling the ctx stops the bulk POST | cache and DB miss; fake bulk endpoint blocks until the client goes away; ctx cancelled once the endpoint is reached | Returns an error wrapping `context.Canceled` without waiting for the endpoint | P1 |  |
| SV-138 | mojangService.GetMojangPlayerByName | Edge Case | Cache write after a DB hit runs on a ctx that outlives the cancelled request | ctx carries a marker value and is already cancelled; cache miss; DB returns a fresh player; store records `ctx.Err()` and the marker in `cacheStore.SetPlayerInCache` | The write sees `ctx.Err() == nil` and the marker value | P1 |  |
| SV-139 | mojangService.GetMojangPlayerByUUID | Edge Case | Cache write after a DB hit runs on a ctx that outlives the cancelled request | ctx carries a marker value and is already cancelled; cache miss; DB returns a fresh player; store records `ctx.Err()` and the marker in `cacheStore.SetPlayerInCache` | The write sees `ctx.Err() == nil` and the marker value | P1 |  |
| SV-140 | mojangService.GetMojangPlayerByUUID | Edge Case | Writes after a Mojang fetch run on a ctx that outlives the cancelled request | ctx carries a marker value and is cancelled once the Mojang body is read; cache and DB miss; store records `ctx.Err()` and the marker in `mojangStore.UpsertPlayer` and `cacheStore.SetPlayerInCache` | Both writes see `ctx.Err() == nil` and the marker value | P1 |  |
| SV-141 | mojangService.GetMojangPlayersByNames | Edge Case | Cache write after a DB hit runs on a ctx that outlives the cancelled request | ctx carries a marker value and is already cancelled; cache miss; DB returns a fresh player; store records `ctx.Err()` and the marker in `cacheStore.SetPlayerInCache` | The write sees `ctx.Err() == nil` and the marker value | P1 |  |
| SV-142 | mojangService.GetMojangPlayersByNames | Edge Case | Writes after a batch fetch run on a ctx that outlives the cancelled request | ctx carries a marker value and is cancelled once the batch body is read; cache and DB miss; store records `ctx.Err()` and the marker in `mojangStore.UpsertPlayer` and `cacheStore.SetPlayerInCache` | Both writes see `ctx.Err() == nil` and the marker value | P1 |  |
| SV-143 | mojangService.resolveProfile | Edge Case | Cache write after a DB hit runs on a ctx that outlives the cancelled request | ctx carries a marker value and is already cancelled; cache miss; DB returns a fresh profile; store records `ctx.Err()` and the marker in `cacheStore.SetProfileInCache` | The write sees `ctx.Err() == nil` and the marker value | P1 |  |
| SV-144 | mojangService.fetchProfileFromMojang | Edge Case | Unsigned fetch writes run on a ctx that outlives the cancelled request | ctx carries a marker value and is cancelled once the Mojang body is read; the profile has a skin and a cape texture; store records `ctx.Err()` and the marker in `mojangStore.UpsertPlayer`, two `textureStore.UpsertTextureHash`, `textureStore.UpsertTextures` and `cacheStore.SetProfileInCache` | All five writes, in that order, see `ctx.Err() == nil` and the marker value | P1 |  |
| SV-145 | mojangService.fetchProfileFromMojang | Edge Case | Signed fetch writes run on a ctx that outlives the cancelled request | same response with `signed` true; store records `ctx.Err()` and the marker in `mojangStore.UpsertPlayer`, two `textureStore.UpsertTextureHash`, `textureStore.UpsertTextures` and `cacheStore.SetSignedProfileInCache` | All five writes, in that order, see `ctx.Err() == nil` and the marker value | P1 |  |
| SV-146 | textureService.GetTextureContent | Edge Case | Archive writes run on a ctx that outlives the cancelled request | ctx carries a marker value and is cancelled once the texture body is read; store records `ctx.Err()` and the marker in `s3store.PutTextureInS3` and `textureStore.UpsertTextureHash` | Both writes see `ctx.Err() == nil` and the marker value | P1 |  |

## store.go

Rows marked "(live)" require `TEST_POSTGRES_URL` and/or `TEST_REDIS_URL` and `t.Skip` themselves when unset. Rows marked "(local fake)" exercise a `httptest.Server`/unreachable-port stand-in and never touch a live service, so they are never skipped. Live rows seed their own fixtures via the store's own Upsert* methods (unique per-test IDs), not raw SQL, so they never depend on run order or another row's state.

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ST-02 | mojangStore.GetPlayerByUUID | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged (not `ErrPlayerNotFound`) | P2 |  |
| ST-03 | mojangStore.GetPlayerByUUID | Happy Path (live) | Row seeded via mojangStore.UpsertPlayer | live DB | Returns matching Player (ProfileActions left zero-value: lax mapping) | P1 |  |
| ST-04 | mojangStore.GetPlayerByUUID | Error Path (live) | No matching id | live DB, unknown id | Returns `ErrPlayerNotFound` | P2 |  |
| ST-05 | mojangStore.GetPlayerByName | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-06 | mojangStore.GetPlayerByName | Happy Path (live) | Row seeded via mojangStore.UpsertPlayer | live DB | Returns matching Player | P1 |  |
| ST-07 | mojangStore.GetPlayerByName | Error Path (live) | No matching name | live DB, unknown name | Returns `ErrPlayerNotFound` | P2 |  |
| ST-08 | mojangStore.GetProfileByUUID | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-11 | mojangStore.GetProfileByUUID | Error Path (live) | No matching player | live DB, unknown id | Returns `ErrPlayerNotFound` | P2 |  |
| ST-12 | textureStore.GetTextures | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P3 |  |
| ST-15 | mojangStore.UpsertPlayer | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-17 | mojangStore.UpsertPlayer | Happy Path (live) | updateProfile=false, new id | live DB | Row inserted with legacy=false, demo=false, profile_actions=[] | P1 |  |
| ST-18 | mojangStore.UpsertPlayer | Edge Case (live) | Same id upserted twice | live DB | Second call updates in place; exactly one row for that id | P2 |  |
| ST-19 | mojangStore.UpsertPlayer | Concurrency Invariant (live) | N goroutines mojangStore.UpsertPlayer the same id concurrently | live DB | No unique-violation errors from any goroutine; `players.id` being a PRIMARY KEY then guarantees exactly one surviving row (looped trials) | P0 | 50 concurrent writers — fewer wouldn't reliably reproduce a broken ON CONFLICT guard's race on every run |
| ST-20 | textureStore.UpsertTextures | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-26 | textureStore.UpsertTextureHash | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-27 | textureStore.UpsertTextureHash | Happy Path (live) | New hash | live DB | Row inserted | P1 |  |
| ST-28 | textureStore.UpsertTextureHash | Edge Case (live) | Duplicate hash | live DB | ON CONFLICT DO NOTHING; no error | P2 |  |
| ST-76 | textureStore.UpsertTextureHash | Concurrency Invariant (live) | N goroutines textureStore.UpsertTextureHash the same hash concurrently | live DB | No unique-violation errors from any goroutine; `textures.hash` being a PRIMARY KEY then guarantees exactly one surviving row (looped trials) | P0 | 50 concurrent writers — fewer wouldn't reliably reproduce a broken ON CONFLICT guard's race on every run |
| ST-29 | cacheStore.GetPlayerFromCache | Error Path | Redis unreachable | closed-port client | Raw connection error passed through unchanged (not `ErrCacheMiss`) | P2 |  |
| ST-30 | cacheStore.GetPlayerFromCache | Happy Path (live) | Key set via cacheStore.SetPlayerInCache | live Redis | Returns decoded *Player | P1 |  |
| ST-31 | cacheStore.GetPlayerFromCache | Error Path (live) | Missing key | live Redis, unknown key | Returns ErrCacheMiss | P1 |  |
| ST-32 | cacheStore.GetPlayerFromCache | Error Path (live) | Value is not valid JSON | live Redis, malformed value | Returns unmarshal error | P2 |  |
| ST-33 | cacheStore.SetPlayerInCache | Error Path | Redis unreachable | closed-port client | Raw connection error passed through unchanged | P2 |  |
| ST-34 | cacheStore.SetPlayerInCache | Happy Path (live) | Valid player | live Redis | Both ID-keyed and Name-keyed entries set with TTL, readable back | P1 |  |
| ST-35 | cacheStore.GetProfileFromCache | Error Path | Redis unreachable | closed-port client | Raw connection error passed through unchanged | P2 |  |
| ST-36 | cacheStore.GetProfileFromCache | Happy Path (live) | Key set via cacheStore.SetProfileInCache | live Redis | Returns decoded *Profile | P1 |  |
| ST-37 | cacheStore.GetProfileFromCache | Error Path (live) | Missing key | live Redis, unknown key | Returns ErrCacheMiss | P2 |  |
| ST-38 | cacheStore.SetProfileInCache | Error Path | Redis unreachable | closed-port client | Raw connection error passed through unchanged | P2 |  |
| ST-39 | cacheStore.SetProfileInCache | Happy Path (live) | Valid profile | live Redis | Key set with TTL, readable back | P1 |  |
| ST-40 | cacheStore.GetSignedProfileFromCache | Error Path | Redis unreachable | closed-port client | Raw connection error passed through unchanged | P2 |  |
| ST-41 | cacheStore.GetSignedProfileFromCache | Happy Path (live) | Key set via cacheStore.SetSignedProfileInCache | live Redis | Returns decoded *Player | P1 |  |
| ST-42 | cacheStore.GetSignedProfileFromCache | Error Path (live) | Missing key | live Redis, unknown key | Returns ErrCacheMiss | P2 |  |
| ST-43 | cacheStore.SetSignedProfileInCache | Error Path | Redis unreachable | closed-port client | Raw connection error passed through unchanged | P2 |  |
| ST-44 | cacheStore.SetSignedProfileInCache | Happy Path (live) | Valid player | live Redis | Key set with TTL, readable back | P1 |  |
| ST-45 | s3store.IsTextureInS3 | Happy Path (local fake) | HeadObject returns 200 | fake S3 server | Returns (true, nil) | P1 |  |
| ST-46 | s3store.IsTextureInS3 | Edge Case (local fake) | HeadObject returns 404 | fake S3 server | Returns (false, nil) | P1 |  |
| ST-47 | s3store.IsTextureInS3 | Error Path (local fake) | HeadObject returns 500 | fake S3 server | Returns (false, non-nil error) | P2 |  |
| ST-48 | geyserStore.GetGeyserPlayerByGamertag | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-50 | geyserStore.GetGeyserPlayerByGamertag | Error Path (live) | No matching gamertag | live DB, unknown gamertag | Returns `ErrPlayerNotFound` (pgx.ErrNoRows is mapped) | P2 |  |
| ST-51 | geyserStore.GetGeyserPlayerByXUID | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-53 | geyserStore.GetGeyserPlayerByXUID | Error Path (live) | No matching xuid | live DB, unknown xuid | Returns `ErrPlayerNotFound` (pgx.ErrNoRows is mapped) | P2 |  |
| ST-54 | geyserStore.UpsertGeyserPlayer | Error Path | DB unreachable | closed-port pool | Raw connection error passed through unchanged | P2 |  |
| ST-68 | s3store.PutTextureInS3 | Happy Path (local fake) | Body implements Len(); PUT returns 200 | fake S3 server | Content-Length header equals Len(); nil error | P1 |  |
| ST-69 | s3store.PutTextureInS3 | Edge Case (local fake) | Body does not implement Len() | fake S3 server | ContentLength left unset; still succeeds | P2 |  |
| ST-70 | s3store.PutTextureInS3 | Error Path (local fake) | PUT returns 500 | fake S3 server | Returns an error wrapping `ErrUploadS3` | P2 |  |
