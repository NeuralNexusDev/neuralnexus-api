# Test plan: pet_pictures

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| TY-01 | GetPetPictureURL | Accessor | build the CDN URL from ID and FileExt | `PetPicture{ID: "abc123", FileExt: "jpg"}` | Returns `CDN_URL + CDN_PATH + "abc123.jpg"` | P3 | |

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| ST-01 | NewStore | Accessor | wrap a pool in a store | any `*pgxpool.Pool` (may be `nil` — DI is unused, since every store method dials its own connection instead) | Returns a non-nil `PetPicStore` | P3 | |
| ST-02 | CreatePet | Happy Path | insert a pet with a unique name | live DB, unique name | Returns `*Pet` with matching `Name`, nonzero `ID`, nil `ProfilePicture`, nil error | P1 | `ProfilePicture` is `*string`; nil means the column came back SQL NULL |
| ST-03 | CreatePet | Error Path | insert a pet whose name already exists | live DB, name already created by a prior `CreatePet` | Returns `nil`, non-nil error (unique constraint violation) | P2 | |
| ST-04 | CreatePet | Error Path | database unreachable | `DATABASE_URL` points at a closed local port | Returns `nil`, non-nil connection error | P2 | |
| ST-05 | CreatePet | Concurrency Invariant | N goroutines call `CreatePet` with the identical name simultaneously | live DB, one shared unique name per round | Exactly one call succeeds (nonzero `ID`, nil error); the rest fail with a unique-constraint error | P0 | design: 5 rounds x 8 concurrent callers, fresh name per round |
| ST-26 | CreatePet | Error Path | insert a pet with an empty name | live DB, `name = ""` | Returns `nil`, `ErrPetNameEmpty` (translated from the `pets_name_not_empty` CHECK violation, not a raw Postgres error) | P1 | |
| ST-06 | GetPet | Error Path | id has no matching row | live DB, id not present in `pets` | Returns `nil`, `pgx.ErrNoRows` | P2 | |
| ST-07 | GetPetByName | Happy Path | name has a matching row | live DB, pet created via `CreatePet` | Returns `*Pet` with matching `ID`/`Name`/`ProfilePicture`, nil error | P1 | |
| ST-08 | GetPetByName | Error Path | name has no matching row | live DB | Returns `nil`, `pgx.ErrNoRows` | P2 | |
| ST-27 | GetPet | Happy Path | id has a matching row | live DB, pet created via `CreatePet` | Returns a `*Pet` equal to the created one, nil error | P1 | |
| ST-09 | GetPetByName | Error Path | database unreachable | closed-port `DATABASE_URL` | Returns `nil`, non-nil connection error | P2 | |
| ST-10 | UpdatePet | Happy Path | update an existing pet's name/profile_picture | live DB, pet created via `CreatePet` | Returns the same `*Pet` passed in, nil error; a follow-up `GetPetByName` on the new name confirms the row was updated | P1 | |
| ST-11 | UpdatePet | Error Path | update targets an id with no matching row | live DB, nonexistent id | Returns `nil`, `pgx.ErrNoRows` (an `UPDATE` matching zero rows means the pet no longer exists) | P2 | |
| ST-12 | UpdatePet | Error Path | database unreachable | closed-port `DATABASE_URL` | Returns `nil`, non-nil connection error | P2 | |
| ST-13 | CreatePetPicture | Happy Path | insert a picture with a unique id | live DB, unique id | Returns `*PetPicture` with the given `ID`/`FileExt`/`PrimarySubject`/`OthersSubjects`/`Aliases` and zero-value `Created`, nil error | P1 | |
| ST-14 | CreatePetPicture | Error Path | insert a picture whose id already exists | live DB, id already created | Returns `nil`, non-nil error (unique constraint violation) | P2 | |
| ST-15 | CreatePetPicture | Error Path | database unreachable | closed-port `DATABASE_URL` | Returns `nil`, non-nil connection error | P2 | |
| ST-16 | CreatePetPicture | Concurrency Invariant | N goroutines call `CreatePetPicture` with the identical id simultaneously (e.g. the same file uploaded twice, same sha) | live DB, one shared unique id per round | Exactly one call succeeds; the rest fail with a unique-constraint error | P0 | design: 5 rounds x 8 concurrent callers, fresh id per round |
| ST-17 | GetRandPetPictureByName | Error Path | pet name has no matching pet | live DB, name never created | Returns `nil`, the upstream `GetPetByName` error (`pgx.ErrNoRows`) | P2 | |
| ST-18 | GetRandPetPictureByName | Error Path | pet exists but has no matching pictures | live DB, pet created, no pictures reference it | Returns `nil`, `pgx.ErrNoRows` | P2 | |
| ST-19 | GetPetPicture | Error Path | id has no matching row | live DB, id not present in `pictures` | Returns `nil`, `pgx.ErrNoRows` | P2 | |
| ST-28 | GetPetPicture | Happy Path | id has a matching row | live DB, picture created via `CreatePetPicture` | Returns the row's `ID`/`FileExt`/`PrimarySubject`/`OthersSubjects`/`Aliases`, and `Created` as an RFC3339 UTC timestamp ending in `Z`, nil error | P1 | |
| ST-29 | GetRandPetPictureByName | Happy Path | pet has a picture | live DB, pet created and one picture with `prime_subj` = its id | Returns that picture, nil error | P1 | |
| ST-20 | GetPetPicture | Error Path | database unreachable | closed-port `DATABASE_URL` | Returns `nil`, non-nil connection error | P2 | |
| ST-21 | UpdatePetPicture | Error Path | database unreachable | closed-port `DATABASE_URL` | Returns `nil`, non-nil connection error | P2 | |
| ST-22 | UpdatePetPicture | Happy Path | update an existing picture's fields | live DB, picture created via `CreatePetPicture` | Returns `*PetPicture` reflecting the updated fields, with `Created` read back from the database as RFC3339 UTC, nil error | P1 | |
| ST-30 | UpdatePetPicture | Error Path | update targets an id with no matching row | live DB, nonexistent id | Returns `nil`, `pgx.ErrNoRows` | P2 | |
| ST-23 | DeletePetPicture | Happy Path | delete an existing picture | live DB, picture created via `CreatePetPicture` | Returns `&PetPicture{ID: id}`, nil error; a follow-up `GetPetPicture(id)` confirms the row is gone (`pgx.ErrNoRows`) | P1 | |
| ST-25 | DeletePetPicture | Edge Case | delete an id with no matching row | live DB, nonexistent id | Returns `&PetPicture{ID: id}`, nil error (a `DELETE` matching zero rows is not itself an error) | P2 | |
| ST-24 | DeletePetPicture | Error Path | database unreachable | closed-port `DATABASE_URL` | Returns `nil`, non-nil connection error | P2 | |

## service.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| SV-01 | NewService | Accessor | construct a service around a store | any `PetPicStore` | `GetStore()` returns that same store instance | P3 | also covers `GetStore` |
| SV-03 | UploadPetPicture | Happy Path | successful upload | mock `CreatePetPicture` returns a `*PetPicture`, CDN transport returns 200 | Request goes to `CDN_URL+"/upload"`; multipart fields `upload_key`/`upload_path` set; form file field name is `<mock ID>.<ext>`; `db.CreatePetPicture` called with the sha256 id and given fileExt/primarySubject/othersSubjects/aliases; returns the mock's `*PetPicture` | P1 | uploaded body bytes not asserted — `UploadPetPicture` always sends zero bytes there (see finding) |
| SV-04 | UploadPetPicture | Error Path | `db.CreatePetPicture` returns an error | mock returns `nil, err` | Returns `nil`, that same error; no CDN request attempted | P2 | |
| SV-05 | UploadPetPicture | Error Path | `os.Rename` fails because the source file is gone | temp file whose path is removed after opening (fd stays valid for the hash read via the inode) | Returns `nil`, a non-nil rename error | P2 | |
| SV-06 | UploadPetPicture | Error Path | the CDN HTTP request fails | transport swapped for a `RoundTripper` returning an error | Returns `nil`, that transport error | P2 | |
| SV-07 | UploadPetPicture | Edge Case | source file name has multiple dots | file named like `photo-*.tar.gz` | `fileExt` computed as `"gz"` (last dot-separated segment); forwarded to `CreatePetPicture` | P3 | |
| SV-08 | UploadPetPicture | Edge Case | source file name has no dot | file named like `photo123456` (no `.`), opened via a relative path so `file.Name()` starts with `./` | `fileExt` computed as the entire basename (no dot found, so `strings.Split` returns one element); forwarded to `CreatePetPicture` | P3 | |

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| HD-01 | CreatePetHandler | Error Path | session lacks `ScopeAdminPetPictures` | session with unrelated/no permissions | 403 Forbidden; `store.CreatePet` never called | P1 | |
| HD-02 | CreatePetHandler | Happy Path | path value `name` present, session has permission | mock `CreatePet` returns `*Pet` | 201 Created with the pet JSON; `CreatePet` called with the path name | P1 | |
| HD-03 | CreatePetHandler | Edge Case | path value absent, body supplies name | JSON body `{"name":"Fido"}` | `CreatePet` called with `"Fido"`; 201 Created | P2 | |
| HD-04 | CreatePetHandler | Error Path | both path value and body name absent/empty | empty body, no path value | 400 Bad Request; `store.CreatePet` never called | P2 | |
| HD-05 | CreatePetHandler | Error Path | `store.CreatePet` returns an error | mock returns `nil, err` | 500 Internal Server Error | P2 | |
| HD-06 | GetPetHandler | Happy Path | path value `id` present and numeric | mock `GetPet` returns `*Pet` | 200 OK with pet JSON; `GetPet` called with the parsed id | P1 | |
| HD-07 | GetPetHandler | Edge Case | path value absent, body supplies id | JSON body `{"id":7}` | `GetPet` called with `7`; 200 OK | P2 | |
| HD-08 | GetPetHandler | Error Path | both path value and body id absent/zero | empty body, no path value | 400 Bad Request; `store.GetPet` never called | P2 | |
| HD-09 | GetPetHandler | Edge Case | path value present but non-numeric, no body fallback | path id = `"abc"` | falls back to id `0`; 400 Bad Request | P3 | |
| HD-10 | GetPetHandler | Error Path | `store.GetPet` returns an error | mock returns `nil, err` | 404 Not Found | P2 | |
| HD-11 | UpdatePetHandler | Error Path | request body fails to decode | malformed JSON body | 400 Bad Request; `store.UpdatePet` never called | P2 | |
| HD-12 | UpdatePetHandler | Error Path | session lacks permission for the decoded pet's name | session scoped to a different pet name | 403 Forbidden; `store.UpdatePet` never called | P1 | |
| HD-13 | UpdatePetHandler | Happy Path | valid body, session has permission | mock `UpdatePet` succeeds | 200 OK with the decoded pet JSON; `UpdatePet` called with the decoded pet | P1 | |
| HD-14 | UpdatePetHandler | Error Path | `store.UpdatePet` returns an error | mock returns `nil, err` | 500 Internal Server Error | P2 | |
| HD-35 | UpdatePetHandler | Error Path | `store.UpdatePet` returns `pgx.ErrNoRows` | mock returns `nil, pgx.ErrNoRows` | 404 Not Found | P2 | |
| HD-15 | GetRandPetPictureByNameHandler | Happy Path | path value `name` present | mock returns `*PetPicture` | 200 OK with picture JSON; `GetRandPetPictureByName` called with the path name | P1 | |
| HD-16 | GetRandPetPictureByNameHandler | Edge Case | path value absent, body supplies name | JSON body `{"name":"Rex"}` | `GetRandPetPictureByName` called with `"Rex"`; 200 OK | P2 | |
| HD-17 | GetRandPetPictureByNameHandler | Error Path | both path value and body name absent/empty | empty body | 400 Bad Request | P2 | |
| HD-18 | GetRandPetPictureByNameHandler | Error Path | `store` returns an error | mock returns `nil, err` | 404 Not Found | P2 | |
| HD-19 | GetPetPictureHandler | Happy Path | path value `id` present | mock returns `*PetPicture` | 200 OK with picture JSON; `GetPetPicture` called with the path id | P1 | |
| HD-20 | GetPetPictureHandler | Edge Case | path value absent, body supplies id | JSON body `{"id":"abc123"}` | `GetPetPicture` called with `"abc123"`; 200 OK | P2 | |
| HD-21 | GetPetPictureHandler | Error Path | both path value and body id absent/empty | empty body | 400 Bad Request | P2 | |
| HD-22 | GetPetPictureHandler | Error Path | `store` returns an error | mock returns `nil, err` | 404 Not Found | P2 | |
| HD-23 | UpdatePetPictureHandler | Error Path | request body fails to decode | malformed JSON body | 400 Bad Request; `store.GetPet`/`UpdatePetPicture` never called | P2 | |
| HD-24 | UpdatePetPictureHandler | Error Path | `store.GetPet(PrimarySubject)` returns an error | mock `GetPet` returns `nil, err` | 404 Not Found; `store.UpdatePetPicture` never called | P2 | |
| HD-25 | UpdatePetPictureHandler | Error Path | session lacks permission for the pet's name | mock `GetPet` succeeds, session scoped to a different name | 403 Forbidden; `store.UpdatePetPicture` never called | P1 | |
| HD-26 | UpdatePetPictureHandler | Happy Path | valid body, pet found, session has permission | mock `UpdatePetPicture` succeeds | 200 OK with picture JSON | P1 | |
| HD-27 | UpdatePetPictureHandler | Error Path | `store.UpdatePetPicture` returns an error | mock returns `nil, err` | 500 Internal Server Error | P2 | |
| HD-36 | UpdatePetPictureHandler | Error Path | `store.UpdatePetPicture` returns `pgx.ErrNoRows` | mock returns `nil, pgx.ErrNoRows` | 404 Not Found | P2 | |
| HD-28 | DeletePetPictureHandler | Happy Path | path value `id` present, all checks pass | mocks succeed, session has permission | 204 No Content; `DeletePetPicture` called with the id | P1 | |
| HD-29 | DeletePetPictureHandler | Edge Case | path value absent, body supplies id | JSON body `{"id":"abc123"}` | `GetPetPicture` called with `"abc123"`; 204 No Content | P2 | |
| HD-30 | DeletePetPictureHandler | Error Path | both path value and body id absent/empty | empty body | 400 Bad Request | P2 | |
| HD-31 | DeletePetPictureHandler | Error Path | `store.GetPetPicture` returns an error | mock returns `nil, err` | 404 Not Found; `store.DeletePetPicture` never called | P2 | |
| HD-32 | DeletePetPictureHandler | Error Path | `store.GetPet(PrimarySubject)` returns an error | mock `GetPetPicture` succeeds, `GetPet` returns `nil, err` | 404 Not Found; `store.DeletePetPicture` never called | P2 | |
| HD-33 | DeletePetPictureHandler | Error Path | session lacks permission for the pet's name | mocks succeed, session scoped to a different name | 403 Forbidden; `store.DeletePetPicture` never called | P1 | |
| HD-34 | DeletePetPictureHandler | Error Path | `store.DeletePetPicture` returns an error | mock returns `nil, err` | 500 Internal Server Error | P2 | |
