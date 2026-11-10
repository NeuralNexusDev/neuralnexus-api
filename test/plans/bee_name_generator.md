# Test plan: bee_name_generator

## handler

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| BNH-01 | GetBeeName | Happy Path | Store returns a name | The store returns the name "Buzzy" | 200 OK, body decodes to BeeName{Name:"Buzzy"} | P1 |  |
| BNH-02 | GetBeeName | Error Path | Store returns an error | The store returns an error | 500 Internal Server Error (`msgFailedToGetBeeName`) | P2 |  |
| BNH-03 | UploadBeeName | Error Path | Session lacks the beenamegenerator.admin node | session without the bee name admin permission | 403 Forbidden, store not called (`msgNoPermissionToUploadBeeNames`) | P1 |  |
| BNH-05 | UploadBeeName | Happy Path | Valid name, authorized session | The store accepts the name and returns it | 200 OK, body decodes to BeeName{Name:name} | P1 |  |
| BNH-06 | UploadBeeName | Error Path | Store returns an error | The store returns an error | 500 Internal Server Error (`msgFailedToUploadBeeName`) | P2 |  |
| BNH-07 | DeleteBeeName | Error Path | Session lacks the beenamegenerator.admin node | session without the bee name admin permission | 403 Forbidden, store not called (`msgNoPermissionToDeleteBeeNames`) | P1 |  |
| BNH-09 | DeleteBeeName | Happy Path | Valid name, authorized session | The store deletes the name and returns it | 204 No Content | P1 |  |
| BNH-10 | DeleteBeeName | Error Path | Store returns an error | The store returns an error | 500 Internal Server Error (`msgFailedToDeleteBeeName`) | P2 |  |
| BNH-12 | SubmitBeeName | Happy Path | Valid name (no auth required) | The store accepts the name and returns it | 200 OK, body decodes to BeeName{Name:name} | P1 |  |
| BNH-13 | SubmitBeeName | Error Path | Store returns an error | The store returns an error | 500 Internal Server Error (`msgFailedToSubmitBeeName`) | P2 |  |
| BNH-14 | GetBeeNameSuggestions | Error Path | Session lacks the beenamegenerator.admin node | session without the bee name admin permission | 403 Forbidden, store not called (`msgNoPermissionToGetBeeNameSuggestions`) | P1 |  |
| BNH-17 | GetBeeNameSuggestions | Edge Case | Amount is non-numeric | amount = "abc" | 400 Bad Request (`msgInvalidAmountProvided`) | P2 |  |
| BNH-18 | GetBeeNameSuggestions | Happy Path | Amount is a valid positive integer | amount = "5"; the store returns the suggestions "a" and "b" | 200 OK, body decodes to BeeNameSuggestions{Suggestions:["a","b"]} | P1 |  |
| BNH-19 | GetBeeNameSuggestions | Error Path | Store returns an error | The store returns an error | 500 Internal Server Error (`msgFailedToGetBeeNameSuggestions`) | P2 |  |
| BNH-20 | AcceptBeeNameSuggestion | Error Path | Session lacks the beenamegenerator.admin node | session without the bee name admin permission | 403 Forbidden, store not called (`msgNoPermissionToAcceptBeeNameSuggestions`) | P1 |  |
| BNH-22 | AcceptBeeNameSuggestion | Happy Path | Valid name, authorized session | The store accepts the suggestion and returns the name | 200 OK, body decodes to BeeName{Name:name} | P1 |  |
| BNH-23 | AcceptBeeNameSuggestion | Error Path | Store returns an error | The store returns an error | 500 Internal Server Error (`msgFailedToAcceptBeeNameSuggestion`) | P2 |  |
| BNH-24 | RejectBeeNameSuggestion | Error Path | Session lacks the beenamegenerator.admin node | session without the bee name admin permission | 403 Forbidden, store not called (`msgNoPermissionToRejectBeeNameSuggestions`) | P1 |  |
| BNH-26 | RejectBeeNameSuggestion | Happy Path | Valid name, authorized session | The store rejects the suggestion and returns the name | 204 No Content | P1 |  |
| BNH-27 | RejectBeeNameSuggestion | Error Path | Store returns an error | The store returns an error | 500 Internal Server Error (`msgFailedToRejectBeeNameSuggestion`) | P2 |  |
| BNH-28 | UploadBeeName | Error Path | Store reports the name already exists | The store reports that the name already exists | 409 Conflict, detail is "That bee name already exists" | P2 |  |
| BNH-29 | SubmitBeeName | Error Path | Store reports the suggestion already exists | The store reports that the suggestion already exists | 409 Conflict, detail is "That bee name has already been suggested" | P2 |  |
| BNH-30 | AcceptBeeNameSuggestion | Error Path | Store reports the name already exists | The store reports that the name already exists | 409 Conflict, detail is "That bee name already exists" | P2 |  |
| BNH-32 | UploadBeeName | Edge Case | Name contains a NUL byte | name = "bee\x00name"; session has the node where one is needed | 400 Bad Request, store not called, `detail` is `msgInvalidName` | P1 | Runs for the upload, delete, submit, accept and reject handlers |
| BNH-33 | UploadBeeName | Edge Case | Whitespace-only name | name = `"  \t "` | 400 Bad Request, store not called, `detail` is `msgInvalidName` | P2 | Runs for the upload, delete, submit, accept and reject handlers |
| BNH-34 | UploadBeeName | Edge Case | Name is not valid UTF-8 | name = "bee\xffname" | 400 Bad Request, store not called, `detail` is `msgInvalidName` | P2 | Runs for the upload, delete, submit, accept and reject handlers |
| BNH-35 | GetBeeNameSuggestions | Edge Case | Amount is negative | amount = "-1" | 400 Bad Request, store not called (`msgInvalidAmountProvided`) | P2 |  |
| BNH-36 | DeleteBeeName | Error Path | Store reports the name is absent | The store reports that the name is absent | 404 Not Found, detail is "Bee name not found" | P2 |  |
| BNH-37 | RejectBeeNameSuggestion | Error Path | Store reports the suggestion is absent | The store reports that the suggestion is absent | 404 Not Found, detail is "Bee name suggestion not found" | P2 |  |
| BNH-38 | AcceptBeeNameSuggestion | Error Path | Store reports the name was never suggested | The store reports that the name was never suggested | 404 Not Found, detail is "Bee name suggestion not found" | P2 |  |

## store

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| BNS-02 | GetBeeName | Error Path | DB connection unreachable | The database address points at a closed local port | Returns a connection error | P2 |  |
| BNS-03 | GetBeeName | Happy Path | A bee name exists | A name seeded by uploading it (requires live DB) | Returns a non-empty name and no error | P1 |  |
| BNS-04 | GetBeeName | Edge Case | No bee name exists | The bee name table is cleared first (requires live DB) | Returns ("", `ErrBeeNameNotFound`) | P2 | The module owns that table exclusively, so clearing it is safe and avoids relying on test order. |
| BNS-06 | UploadBeeName | Happy Path | Valid name | live store (requires live DB) | Returns (name, nil); the name is stored exactly once | P1 |  |
| BNS-08 | DeleteBeeName | Happy Path | Name previously uploaded | seeded via UploadBeeName (requires live DB) | Returns (name, nil); the name is no longer stored | P1 |  |
| BNS-09 | DeleteBeeName | Edge Case | Name does not exist | live store, unused name (requires live DB) | Returns ("", `ErrBeeNameNotFound`) - DELETE matched zero rows | P3 |  |
| BNS-11 | SubmitBeeName | Happy Path | Valid name | live store (requires live DB) | Returns (name, nil); the suggestion is stored exactly once | P1 |  |
| BNS-13 | GetBeeNameSuggestions | Happy Path | Several suggestions exist and the amount asked for equals their number | Suggestions seeded by submitting them, after the suggestion table is cleared (requires live DB) | Returns as many suggestions as asked for, and no error | P1 | The suggestion table is cleared first so other tests' rows do not skew the count; the module owns it. |
| BNS-14 | GetBeeNameSuggestions | Edge Case | No rows match (empty suggestion table for a fresh unique batch) | The suggestion table is cleared first (requires live DB) | Returns ([]string{}, nil) | P2 | The module owns that table exclusively, so clearing it is safe and avoids relying on test order. |
| BNS-17 | AcceptBeeNameSuggestion | Concurrency Invariant | Two goroutines concurrently accept the same freshly-submitted suggestion name | one suggestion seeded by submitting it; store shared across goroutines; the name is unique in the bee name table (requires live DB) | Exactly one of the two calls succeeds; after both complete, the name is absent from the suggestions and present exactly once among the bee names (looped across trials) | P0 | Looped over 20 trials: one trial cannot tell the uniqueness guard from a race that sometimes lets both through. |
| BNS-19 | RejectBeeNameSuggestion | Happy Path | Suggestion previously submitted | seeded via SubmitBeeName (requires live DB) | Returns (name, nil); the name is no longer a suggestion | P1 |  |
| BNS-20 | RejectBeeNameSuggestion | Edge Case | Name does not exist in suggestions | live store, unused name (requires live DB) | Returns ("", `ErrBeeNameSuggestionNotFound`) - DELETE matched zero rows | P3 |  |
| BNS-21 | UploadBeeName | Concurrency Invariant | Two goroutines concurrently upload the same name | store shared across goroutines; the name is unique in the bee name table (requires live DB) | Exactly one of the two calls succeeds; after both complete, the name is stored exactly once (looped across trials) | P0 | Looped over 20 trials: one trial cannot tell the uniqueness guard from a race that sometimes lets both through. |
| BNS-22 | SubmitBeeName | Concurrency Invariant | Two goroutines concurrently submit the same name | store shared across goroutines; the name is unique among suggestions (requires live DB) | Exactly one of the two calls succeeds; after both complete, the suggestion is stored exactly once (looped across trials) | P0 | Looped over 20 trials: one trial cannot tell the uniqueness guard from a race that sometimes lets both through. |
| BNS-23 | GetBeeNameSuggestions | Error Path | Reading the rows fails after the scan loop starts (a genuine iteration error, not zero matching rows) | live store, row iteration fails mid-scan (requires live DB) | Returns nil and an error | P2 | Iteration failure is injected by truncating the server response on the wire |
| BNS-24 | UploadBeeName | Error Path | Whitespace-only name | live store, name = "  \t " on the shipped database schema | Returns ("", error) where the database refuses the name with a check violation (SQLSTATE 23514) from its not-blank rule for bee names; no name is stored | P2 | The handlers reject it with 400; the constraint guards the store |
| BNS-25 | SubmitBeeName | Error Path | Whitespace-only name | live store, name = "  \t " on the shipped database schema | Returns ("", error) where the database refuses the name with a check violation (SQLSTATE 23514) from its not-blank rule for suggestions; no suggestion is stored | P2 | |
| BNS-27 | AcceptBeeNameSuggestion | Happy Path | Suggestion previously submitted | seeded via SubmitBeeName (requires live DB) | Returns (name, nil); the name is stored once as a bee name and is no longer a suggestion | P1 |  |
| BNS-28 | AcceptBeeNameSuggestion | Error Path | Name was never suggested | live store, unused name (requires live DB) | Returns ("", `ErrBeeNameSuggestionNotFound`); the name is not stored as a bee name | P1 |  |
| BNS-29 | AcceptBeeNameSuggestion | Error Path | Name already in bee_name and still suggested | name seeded as both a bee name and a suggestion (requires live DB) | Returns ("", `ErrBeeNameExists`); the suggestion is still present | P1 | The transaction rolls the delete back, so the suggestion stays. |
