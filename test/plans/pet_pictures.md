# Test plan: pet_pictures

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|

## store

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| PPS-02 | CreatePet | Happy Path | insert a pet with a unique name | live DB, unique name | Returns the new pet with matching `Name`, a nonzero `ID`, no profile picture, nil error | P1 | |
| PPS-03 | CreatePet | Error Path | insert a pet whose name already exists | database reachable, a pet with this name was created before | Returns `nil`, `ErrPetNameExists` (the database refuses the duplicate name) | P2 | |
| PPS-04 | CreatePet | Error Path | database unreachable | the database is unreachable | Returns `nil`, non-nil connection error | P2 | |
| PPS-05 | CreatePet | Concurrency Invariant | several callers create a pet with the identical name at the same time | database reachable, one shared unique name per round | Exactly one call succeeds (nonzero `ID`, nil error); every other call fails with `ErrPetNameExists` | P0 | design: 5 rounds x 8 concurrent callers, fresh name per round |
| PPS-06 | GetPet | Error Path | id has no matching row | database reachable, no pet has this id | Returns `nil`, `ErrPetNotFound` | P2 | |
| PPS-07 | GetPetByName | Happy Path | name has a matching row | database reachable, the pet was created beforehand | Returns the pet with matching `ID`/`Name`/`ProfilePicture`, nil error | P1 | |
| PPS-08 | GetPetByName | Error Path | name has no matching row | live DB | Returns `nil`, `ErrPetNotFound` | P2 | |
| PPS-10 | UpdatePet | Happy Path | update an existing pet's name and profile picture | database reachable, the pet was created beforehand | Returns the same pet that was passed in, nil error; a follow-up lookup by the new name confirms the pet was updated | P1 | |
| PPS-11 | UpdatePet | Error Path | update targets an id with no matching row | live DB, nonexistent id | Returns `nil`, `ErrPetNotFound` (updating zero rows means the pet no longer exists) | P2 | |
| PPS-13 | CreatePetPicture | Happy Path | insert a picture with a unique id | live DB, unique id | Returns the new picture with the given `ID`/`FileExt`/`PrimarySubject`/`OthersSubjects`/`Aliases` and a zero-value `Created`, nil error | P1 | |
| PPS-14 | CreatePetPicture | Error Path | insert a picture whose id already exists | live DB, id already created | Returns `nil`, non-nil error (the database refuses the duplicate id) | P2 | |
| PPS-16 | CreatePetPicture | Concurrency Invariant | several callers create a picture with the identical id at the same time (for example the same file uploaded twice) | live DB, one shared unique id per round | Exactly one call succeeds; the rest fail because the id exists already | P0 | design: 5 rounds x 8 concurrent callers, fresh id per round |
| PPS-17 | GetRandPetPictureByName | Error Path | pet name has no matching pet | live DB, name never created | Returns `nil`, the error from looking up the pet by name (`ErrPetNotFound`) | P2 | |
| PPS-18 | GetRandPetPictureByName | Error Path | pet exists but has no matching pictures | live DB, pet created, no pictures reference it | Returns `nil`, `ErrPetPictureNotFound` | P2 | |
| PPS-19 | GetPetPicture | Error Path | id has no matching row | database reachable, no picture has this id | Returns `nil`, `ErrPetPictureNotFound` | P2 | |
| PPS-22 | UpdatePetPicture | Happy Path | update an existing picture's fields | database reachable, the picture was created beforehand, the database session uses a non-UTC time zone (America/Los_Angeles, Asia/Kolkata) | Returns the picture with the updated fields and `Created` read back as the same instant in UTC, as RFC 3339 text, nil error | P1 | |
| PPS-23 | DeletePetPicture | Happy Path | delete an existing picture | database reachable, the picture was created beforehand | Returns a picture holding only the deleted id, nil error; loading the picture afterwards fails with `ErrPetPictureNotFound` | P1 | |
| PPS-25 | DeletePetPicture | Edge Case | delete an id with no matching row | live DB, nonexistent id | Returns a picture holding only the id, nil error (deleting zero rows is not an error) | P2 | |
| PPS-26 | CreatePet | Error Path | insert a pet with an empty name | database reachable, the name is empty | Returns `nil`, `ErrPetNameEmpty` (the database refuses the empty name, and the caller gets the sentinel instead of a raw database error) | P1 | |
| PPS-27 | GetPet | Happy Path | id has a matching row | database reachable, the pet was created beforehand | Returns a pet equal to the created one, nil error | P1 | |
| PPS-28 | GetPetPicture | Happy Path | id has a matching row | database reachable, the picture was created beforehand, the database session uses a non-UTC time zone (America/Los_Angeles, Asia/Kolkata) | Returns the row's `ID`/`FileExt`/`PrimarySubject`/`OthersSubjects`/`Aliases`, and `Created` as the same instant in UTC, as RFC 3339 text ending in Z, nil error | P1 | |
| PPS-29 | GetRandPetPictureByName | Happy Path | pet has a picture | live DB, pet created and one picture whose primary subject is its id | Returns that picture, nil error | P1 | |
| PPS-30 | UpdatePetPicture | Error Path | update targets an id with no matching row | live DB, nonexistent id | Returns `nil`, `ErrPetPictureNotFound` | P2 | |
| PPS-31 | CreatePetPicture | Happy Path | a picture with no other subjects keeps only its primary subject | database reachable, no other subjects | Loading the picture returns `PrimarySubject` set and `OthersSubjects` nil | P1 | |
| PPS-32 | CreatePetPicture | Happy Path | primary and other subjects keep their order | database reachable, primary subject 3, other subjects 4 and 5 | Loading the picture returns `PrimarySubject` 3 and `OthersSubjects` 4 then 5 | P1 | |
| PPS-33 | CreatePetPicture | Error Path | store a picture row that has no subjects at all | database reachable, a picture row written directly with an empty list of subjects | The database refuses the row because a picture needs at least one subject | P2 | |
| PPS-34 | UpdatePet | Error Path | rename a pet to the name of another pet | database reachable, two pets were created beforehand | Returns `nil`, `ErrPetNameExists` | P2 | |

## service

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| PPV-03 | UploadPetPicture | Happy Path | successful upload | creating the picture succeeds; the CDN answers 200 | The picture is uploaded to the CDN's upload endpoint; the upload carries the CDN key, the upload path and a file named after the picture id plus its extension; the picture is created with the sha256 of the file as id and the given extension, primary subject, other subjects and aliases; returns the created picture | P1 | The uploaded file content is not asserted: the upload always sends zero bytes there (see finding) |
| PPV-04 | UploadPetPicture | Error Path | creating the picture fails | creating the picture fails with an unexpected error | Returns `nil`, that same error; no request to the CDN | P2 | |
| PPV-05 | UploadPetPicture | Error Path | renaming the source file fails because the file is gone | temp file removed after it was opened (the open handle still reads it for the hash) | Returns `nil`, a non-nil rename error | P2 | |
| PPV-06 | UploadPetPicture | Error Path | the CDN HTTP request fails | the request to the CDN fails | Returns `nil`, an error that wraps the transport error | P2 | |
| PPV-07 | UploadPetPicture | Edge Case | source file name has multiple dots | file name with several dots, ending in .tar.gz | The extension is "gz" (the text after the last dot); the picture is created with it | P3 | |
| PPV-08 | UploadPetPicture | Edge Case | source file name has no dot | file name with no dot, opened through a relative path | The extension is the whole file name (there is no dot to split on); the picture is created with it | P3 | |
| PPV-09 | UploadPetPicture | Happy Path | the caller's context reaches creating the picture, and the record write outlives a cancelled request | the caller's context carries a marker value and is cancelled before the call; the CDN answers 200 | Creating the picture receives the marker value and a context that is not cancelled; the request to the CDN carries the marker value; the upload returns a nil error | P1 | |

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| PPH-01 | CreatePetHandler | Error Path | session lacks `petpictures.admin` | session with unrelated/no permissions | 403 Forbidden; no pet is created (`msgNoPermissionToCreatePet`) | P1 | |
| PPH-02 | CreatePetHandler | Happy Path | path value `name` present, session has permission | creating the pet succeeds | 201 Created with the pet JSON; the pet is created with the path name | P1 | |
| PPH-03 | CreatePetHandler | Edge Case | path value absent, body supplies name | JSON body `{"name":"Fido"}` | The pet is created with the name "Fido"; 201 Created | P2 | |
| PPH-04 | CreatePetHandler | Error Path | both path value and body name absent/empty | empty body, no path value | 400 Bad Request; no pet is created, `detail` is `msgPetNameIsRequired` | P2 | |
| PPH-05 | CreatePetHandler | Error Path | creating the pet fails | an unexpected error occurs | 500 Internal Server Error (`msgUnableToCreatePet`) | P2 | |
| PPH-06 | GetPetHandler | Happy Path | path value `id` present and numeric | the pet exists | 200 OK with pet JSON; the pet with the parsed id is loaded | P1 | |
| PPH-07 | GetPetHandler | Edge Case | path value absent, body supplies id | JSON body `{"id":7}` | The pet with id 7 is loaded; 200 OK | P2 | |
| PPH-08 | GetPetHandler | Error Path | both path value and body id absent/zero | empty body, no path value | 400 Bad Request; the pet is not loaded (`msgPetIDIsRequired`) | P2 | |
| PPH-09 | GetPetHandler | Edge Case | path value present but non-numeric, no body fallback | path id = `"abc"` | falls back to id `0`; 400 Bad Request (`msgPetIDIsRequired`) | P3 | |
| PPH-10 | GetPetHandler | Error Path | loading the pet fails with `ErrPetNotFound` | the pet does not exist | 404 Not Found, `detail` is `msgPetNotFound` | P2 | |
| PPH-11 | UpdatePetHandler | Error Path | request body fails to decode | malformed JSON body | 400 Bad Request; the pet is not updated, `detail` is `msgUnableToParseBody` | P2 | |
| PPH-12 | UpdatePetHandler | Error Path | session lacks permission for the decoded pet's name | session scoped to a different pet name | 403 Forbidden; the pet is not updated, `detail` is `msgNoPermissionToUpdatePet` | P1 | |
| PPH-13 | UpdatePetHandler | Happy Path | valid body, session has permission | updating the pet succeeds | 200 OK with the decoded pet JSON; the pet is updated with the decoded pet | P1 | |
| PPH-14 | UpdatePetHandler | Error Path | updating the pet fails | an unexpected error occurs | 500 Internal Server Error (`msgUnableToUpdatePet`) | P2 | |
| PPH-15 | GetRandPetPictureByNameHandler | Happy Path | path value `name` present | the pet has a picture | 200 OK with picture JSON; the picture is loaded for the path name | P1 | |
| PPH-16 | GetRandPetPictureByNameHandler | Edge Case | path value absent, body supplies name | JSON body `{"name":"Rex"}` | The picture is loaded for the name "Rex"; 200 OK | P2 | |
| PPH-17 | GetRandPetPictureByNameHandler | Error Path | both path value and body name absent/empty | empty body | 400 Bad Request, `detail` is `msgPetNameIsRequired` | P2 | |
| PPH-18 | GetRandPetPictureByNameHandler | Error Path | loading fails with `ErrPetPictureNotFound` | the picture does not exist | 404 Not Found, `detail` is `msgPetPictureNotFound` | P2 | |
| PPH-19 | GetPetPictureHandler | Happy Path | path value `id` present | the picture exists | 200 OK with picture JSON; the picture with the path id is loaded | P1 | |
| PPH-20 | GetPetPictureHandler | Edge Case | path value absent, body supplies id | JSON body `{"id":"abc123"}` | The picture with id "abc123" is loaded; 200 OK | P2 | |
| PPH-21 | GetPetPictureHandler | Error Path | both path value and body id absent/empty | empty body | 400 Bad Request, `detail` is `msgPetPictureIDIsRequired` | P2 | |
| PPH-22 | GetPetPictureHandler | Error Path | loading fails with `ErrPetPictureNotFound` | the picture does not exist | 404 Not Found, `detail` is `msgPetPictureNotFound` | P2 | |
| PPH-23 | UpdatePetPictureHandler | Error Path | request body fails to decode | malformed JSON body | 400 Bad Request; the pet is not loaded and the picture is not updated, `detail` is `msgUnableToParseBody` | P2 | |
| PPH-24 | UpdatePetPictureHandler | Error Path | loading the pet of the picture's primary subject fails with `ErrPetNotFound` | the pet does not exist | 404 Not Found; the picture is not updated, `detail` is `msgPetNotFound` | P2 | |
| PPH-25 | UpdatePetPictureHandler | Error Path | session lacks permission for the pet's name | the pet exists, session scoped to a different name | 403 Forbidden; the picture is not updated, `detail` is `msgNoPermissionToUpdatePet` | P1 | |
| PPH-26 | UpdatePetPictureHandler | Happy Path | valid body, pet found, session has permission | updating the picture succeeds | 200 OK with picture JSON | P1 | |
| PPH-27 | UpdatePetPictureHandler | Error Path | updating the picture fails | an unexpected error occurs | 500 Internal Server Error (`msgUnableToUpdatePetPicture`) | P2 | |
| PPH-28 | DeletePetPictureHandler | Happy Path | path value `id` present, all checks pass | the picture and its pet exist, session has permission | 204 No Content; the picture with the id is deleted | P1 | |
| PPH-29 | DeletePetPictureHandler | Edge Case | path value absent, body supplies id | JSON body `{"id":"abc123"}` | The picture with id "abc123" is loaded; 204 No Content | P2 | |
| PPH-30 | DeletePetPictureHandler | Error Path | both path value and body id absent/empty | empty body | 400 Bad Request, `detail` is `msgPetPictureIDIsRequired` | P2 | |
| PPH-31 | DeletePetPictureHandler | Error Path | loading the picture fails with `ErrPetPictureNotFound` | the picture does not exist | 404 Not Found; the picture is not deleted, `detail` is `msgPetPictureNotFound` | P2 | |
| PPH-32 | DeletePetPictureHandler | Error Path | loading the pet of the picture's primary subject fails with `ErrPetNotFound` | the picture is found; its pet does not exist | 404 Not Found; the picture is not deleted, `detail` is `msgPetNotFound` | P2 | |
| PPH-33 | DeletePetPictureHandler | Error Path | session lacks permission for the pet's name | the picture and its pet exist, session scoped to a different name | 403 Forbidden; the picture is not deleted, `detail` is `msgNoPermissionToUpdatePet` | P1 | |
| PPH-34 | DeletePetPictureHandler | Error Path | deleting the picture fails | an unexpected error occurs | 500 Internal Server Error (`msgUnableToDeletePetPicture`) | P2 | |
| PPH-35 | UpdatePetHandler | Error Path | updating the pet fails with `ErrPetNotFound` | the pet does not exist | 404 Not Found, `detail` is `msgPetNotFound` | P2 | |
| PPH-36 | UpdatePetPictureHandler | Error Path | updating the picture fails with `ErrPetPictureNotFound` | the picture does not exist | 404 Not Found, `detail` is `msgPetPictureNotFound` | P2 | |
| PPH-37 | CreatePetHandler | Error Path | creating the pet fails with `ErrPetNameEmpty` | admin session, the failure is `ErrPetNameEmpty` | 400 Bad Request, `detail` is `msgPetNameMustNotBeEmpty` | P2 |  |
| PPH-38 | UpdatePetHandler | Error Path | updating the pet fails with `ErrPetNameEmpty` | scoped session, the failure is `ErrPetNameEmpty` | 400 Bad Request, `detail` is `msgPetNameMustNotBeEmpty` | P2 |  |
| PPH-39 | GetPetHandler | Error Path | loading the pet fails for a reason other than not found | the database is down | 500 Internal Server Error, `detail` is `msgUnableToGetPet` | P2 |  |
| PPH-40 | GetRandPetPictureByNameHandler | Error Path | loading fails for a reason other than not found | the database is down | 500 Internal Server Error, `detail` is `msgUnableToGetRandomPetPicture` | P2 |  |
| PPH-41 | GetPetPictureHandler | Error Path | loading the picture fails for a reason other than not found | the database is down | 500 Internal Server Error, `detail` is `msgUnableToGetPetPicture` | P2 |  |
| PPH-42 | UpdatePetPictureHandler | Error Path | loading the pet of the picture's primary subject fails for a reason other than not found | the database is down | 500 Internal Server Error; the picture is not updated, `detail` is `msgUnableToGetPet` | P2 |  |
| PPH-43 | DeletePetPictureHandler | Error Path | loading the picture fails for a reason other than not found | the database is down | 500 Internal Server Error; the picture is not deleted, `detail` is `msgUnableToGetPetPicture` | P2 |  |
| PPH-44 | DeletePetPictureHandler | Error Path | loading the pet of the picture's primary subject fails for a reason other than not found | the picture is found; the database is down when the pet is loaded | 500 Internal Server Error; the picture is not deleted, `detail` is `msgUnableToGetPet` | P2 |  |
| PPH-45 | GetRandPetPictureByNameHandler | Error Path | loading fails with `ErrPetNotFound` for an unknown pet name | the pet does not exist | 404 Not Found, `detail` is `msgPetNotFound` | P2 |  |
| PPH-46 | CreatePetHandler | Error Path | creating the pet fails with `ErrPetNameExists` | the failure is `ErrPetNameExists` | 409 Conflict, `detail` is `msgPetNameAlreadyExists` | P1 | |
| PPH-47 | UpdatePetHandler | Error Path | updating the pet fails with `ErrPetNameExists` | the failure is `ErrPetNameExists` | 409 Conflict, `detail` is `msgPetNameAlreadyExists` | P1 | |
| PPH-48 | CreatePetHandler | Error Path | name from the path contains a NUL byte | path value `"Re\x00x"` | 400 Bad Request, `detail` is `msgInvalidPetName`; no pet is created | P1 | |
| PPH-49 | CreatePetHandler | Error Path | name from the path is not valid UTF-8 | path value `"Re\xffx"` | 400 Bad Request, `detail` is `msgInvalidPetName`; no pet is created | P1 | |
| PPH-50 | UpdatePetHandler | Error Path | body name contains a NUL byte | body `{"id":1,"name":"Re\u0000x"}`, session permitted for that name | 400 Bad Request, `detail` is `msgInvalidPetName`; the pet is not updated | P1 | |
