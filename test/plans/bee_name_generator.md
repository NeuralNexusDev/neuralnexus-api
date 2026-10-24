# Test plan: bee_name_generator

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| BNH-01 | GetBeeNameHandler | Happy Path | Store returns a name | fake.GetBeeName -> ("Buzzy", nil) | 200 OK, body decodes to BeeName{Name:"Buzzy"} | P1 |  |
| BNH-02 | GetBeeNameHandler | Error Path | Store returns an error | fake.GetBeeName -> ("", err) | 500 Internal Server Error (`msgFailedToGetBeeName`) | P2 |  |
| BNH-03 | UploadBeeNameHandler | Error Path | Session lacks the beenamegenerator.admin node | session without ScopeAdminBeeNameGenerator | 403 Forbidden, store not called (`msgNoPermissionToUploadBeeNames`) | P1 |  |
| BNH-04 | UploadBeeNameHandler | Edge Case | Empty name path value | session has the node; name = "" | 400 Bad Request, store not called, `detail` is `msgInvalidName` | P2 |  |
| BNH-05 | UploadBeeNameHandler | Happy Path | Valid name, authorized session | fake.UploadBeeName -> (name, nil) | 200 OK, body decodes to BeeName{Name:name} | P1 |  |
| BNH-06 | UploadBeeNameHandler | Error Path | Store returns an error | fake.UploadBeeName -> ("", err) | 500 Internal Server Error (`msgFailedToUploadBeeName`) | P2 |  |
| BNH-07 | DeleteBeeNameHandler | Error Path | Session lacks the beenamegenerator.admin node | session without ScopeAdminBeeNameGenerator | 403 Forbidden, store not called (`msgNoPermissionToDeleteBeeNames`) | P1 |  |
| BNH-08 | DeleteBeeNameHandler | Edge Case | Empty name path value | session has the node; name = "" | 400 Bad Request, store not called, `detail` is `msgInvalidName` | P2 |  |
| BNH-09 | DeleteBeeNameHandler | Happy Path | Valid name, authorized session | fake.DeleteBeeName -> (name, nil) | 204 No Content | P1 |  |
| BNH-10 | DeleteBeeNameHandler | Error Path | Store returns an error | fake.DeleteBeeName -> ("", err) | 500 Internal Server Error (`msgFailedToDeleteBeeName`) | P2 |  |
| BNH-11 | SubmitBeeNameHandler | Edge Case | Empty name path value | name = "" | 400 Bad Request, store not called, `detail` is `msgInvalidName` | P2 |  |
| BNH-12 | SubmitBeeNameHandler | Happy Path | Valid name (no auth required) | fake.SubmitBeeName -> (name, nil) | 200 OK, body decodes to BeeName{Name:name} | P1 |  |
| BNH-13 | SubmitBeeNameHandler | Error Path | Store returns an error | fake.SubmitBeeName -> ("", err) | 500 Internal Server Error (`msgFailedToSubmitBeeName`) | P2 |  |
| BNH-14 | GetBeeNameSuggestionsHandler | Error Path | Session lacks the beenamegenerator.admin node | session without ScopeAdminBeeNameGenerator | 403 Forbidden, store not called (`msgNoPermissionToGetBeeNameSuggestions`) | P1 |  |
| BNH-15 | GetBeeNameSuggestionsHandler | Edge Case | Empty amount path value | session has the node; amount = "" | 400 Bad Request (empty is coerced to "NAN", fails ParseInt) (`msgInvalidAmountProvided`) | P2 |  |
| BNH-16 | GetBeeNameSuggestionsHandler | Edge Case | Amount path value is "0" | amount = "0" | 400 Bad Request ("0" is coerced to "NAN", fails ParseInt) (`msgInvalidAmountProvided`) | P2 |  |
| BNH-17 | GetBeeNameSuggestionsHandler | Edge Case | Amount is non-numeric | amount = "abc" | 400 Bad Request (`msgInvalidAmountProvided`) | P2 |  |
| BNH-18 | GetBeeNameSuggestionsHandler | Happy Path | Amount is a valid positive integer | amount = "5"; fake.GetBeeNameSuggestions -> ([]string{"a","b"}, nil) | 200 OK, body decodes to BeeNameSuggestions{Suggestions:["a","b"]} | P1 |  |
| BNH-19 | GetBeeNameSuggestionsHandler | Error Path | Store returns an error | fake.GetBeeNameSuggestions -> (nil, err) | 500 Internal Server Error (`msgFailedToGetBeeNameSuggestions`) | P2 |  |
| BNH-20 | AcceptBeeNameSuggestionHandler | Error Path | Session lacks the beenamegenerator.admin node | session without ScopeAdminBeeNameGenerator | 403 Forbidden, store not called (`msgNoPermissionToAcceptBeeNameSuggestions`) | P1 |  |
| BNH-21 | AcceptBeeNameSuggestionHandler | Edge Case | Empty name path value | session has the node; name = "" | 400 Bad Request, store not called, `detail` is `msgInvalidName` | P2 |  |
| BNH-22 | AcceptBeeNameSuggestionHandler | Happy Path | Valid name, authorized session | fake.AcceptBeeNameSuggestion -> (name, nil) | 200 OK, body decodes to BeeName{Name:name} | P1 |  |
| BNH-23 | AcceptBeeNameSuggestionHandler | Error Path | Store returns an error | fake.AcceptBeeNameSuggestion -> ("", err) | 500 Internal Server Error (`msgFailedToAcceptBeeNameSuggestion`) | P2 |  |
| BNH-24 | RejectBeeNameSuggestionHandler | Error Path | Session lacks the beenamegenerator.admin node | session without ScopeAdminBeeNameGenerator | 403 Forbidden, store not called (`msgNoPermissionToRejectBeeNameSuggestions`) | P1 |  |
| BNH-25 | RejectBeeNameSuggestionHandler | Edge Case | Empty name path value | session has the node; name = "" | 400 Bad Request, store not called, `detail` is `msgInvalidName` | P2 |  |
| BNH-26 | RejectBeeNameSuggestionHandler | Happy Path | Valid name, authorized session | fake.RejectBeeNameSuggestion -> (name, nil) | 204 No Content | P1 |  |
| BNH-27 | RejectBeeNameSuggestionHandler | Error Path | Store returns an error | fake.RejectBeeNameSuggestion -> ("", err) | 500 Internal Server Error (`msgFailedToRejectBeeNameSuggestion`) | P2 |  |
| BNH-28 | UploadBeeNameHandler | Error Path | Store reports the name already exists | fake.UploadBeeName -> ("", ErrBeeNameExists) | 409 Conflict, detail is "That bee name already exists" | P2 |  |
| BNH-29 | SubmitBeeNameHandler | Error Path | Store reports the suggestion already exists | fake.SubmitBeeName -> ("", ErrBeeNameSuggestionExists) | 409 Conflict, detail is "That bee name has already been suggested" | P2 |  |
| BNH-30 | AcceptBeeNameSuggestionHandler | Error Path | Store reports the name already exists | fake.AcceptBeeNameSuggestion -> ("", ErrBeeNameExists) | 409 Conflict, detail is "That bee name already exists" | P2 |  |
| BNH-31 | GetBeeNameHandler | Happy Path | the request's ctx reaches the store | fake.GetBeeName records the ctx it receives; request ctx carries a marker value | The recorded ctx holds the marker value | P1 | |

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| BNS-02 | GetBeeName | Error Path | DB connection unreachable | store.db points at a closed local port | Returns ("", non-nil connection error) | P2 |  |
| BNS-03 | GetBeeName | Happy Path | A row exists in bee_name | seeded via UploadBeeName (requires live DB) | Returns (name, nil) for one of the seeded names | P1 |  |
| BNS-04 | GetBeeName | Edge Case | bee_name has no matching row (empty/only-other-rows result) | table filtered to nothing scannable (requires live DB) | Returns ("", `ErrBeeNameNotFound`) | P2 | Achieved by clearing bee_name first - safe since this module owns that table exclusively, and avoids relying on test execution order. |
| BNS-05 | UploadBeeName | Error Path | DB connection unreachable | store.db points at a closed local port | Returns ("", non-nil connection error) | P2 |  |
| BNS-06 | UploadBeeName | Happy Path | Valid name | live store (requires live DB) | Returns (name, nil); row is queryable back via GetBeeName | P1 |  |
| BNS-07 | DeleteBeeName | Error Path | DB connection unreachable | store.db points at a closed local port | Returns ("", non-nil connection error) | P2 |  |
| BNS-08 | DeleteBeeName | Happy Path | Name previously uploaded | seeded via UploadBeeName (requires live DB) | Returns (name, nil); row no longer present | P1 |  |
| BNS-09 | DeleteBeeName | Edge Case | Name does not exist | live store, unused name (requires live DB) | Returns (name, nil) - DELETE matching zero rows is not an error | P3 |  |
| BNS-10 | SubmitBeeName | Error Path | DB connection unreachable | store.db points at a closed local port | Returns ("", non-nil connection error) | P2 |  |
| BNS-11 | SubmitBeeName | Happy Path | Valid name | live store (requires live DB) | Returns (name, nil); row appears in bee_name_suggestion | P1 |  |
| BNS-12 | GetBeeNameSuggestions | Error Path | DB connection unreachable | store.db points at a closed local port | Returns ([]string{}, non-nil connection error) | P2 |  |
| BNS-13 | GetBeeNameSuggestions | Happy Path | N suggestions seeded, amount <= N | seeded via SubmitBeeName (requires live DB) | Returns (slice of length == amount, nil), all elements from the seeded set | P1 | bee_name_suggestion is cleared first so the count assertion isn't skewed by rows left by other tests; safe since this module owns that table exclusively. |
| BNS-14 | GetBeeNameSuggestions | Edge Case | No rows match (empty suggestion table for a fresh unique batch) | live store, unqueried unique names (requires live DB) | Returns ([]string{}, nil) | P2 | Achieved by clearing bee_name_suggestion first - safe since this module owns that table exclusively, and avoids relying on test execution order. |
| BNS-23 | GetBeeNameSuggestions | Error Path | `rows.Err()` returns non-nil after the scan loop (a genuine iteration error, not zero matching rows) | live store, row iteration fails mid-scan (requires live DB) | Returns (nil, that error) | P2 | Iteration failure is injected by truncating the server response on the wire |
| BNS-24 | UploadBeeName | Error Path | Whitespace-only name | live store, name = `"  \t "` on a database created from `db/bee_name_generator.sql` | Returns ("", non-nil error) with SQLSTATE 23514 (CHECK violation) from the `bee_name` constraint; no row inserted | P2 | Handlers only reject the empty string, so the constraint is the only guard |
| BNS-25 | SubmitBeeName | Error Path | Whitespace-only name | live store, name = `"  \t "` on a database created from `db/bee_name_generator.sql` | Returns ("", non-nil error) with SQLSTATE 23514 (CHECK violation) from the `bee_name_suggestion` constraint; no row inserted | P2 | |
| BNS-26 | AcceptBeeNameSuggestion | Error Path | Whitespace-only name | live store, name = `"  \t "` on a database created from `db/bee_name_generator.sql` | Returns ("", non-nil error) with SQLSTATE 23514 (CHECK violation) from the `bee_name` constraint; no row inserted | P2 | |
| BNS-15 | AcceptBeeNameSuggestion | Error Path | DB connection unreachable (insert fails) | store.db points at a closed local port | Returns ("", non-nil connection error); no partial state possible since insert itself fails | P2 |  |
| BNS-16 | AcceptBeeNameSuggestion | Happy Path | Suggestion previously submitted | seeded via SubmitBeeName (requires live DB) | Returns (name, nil); name now in bee_name, no longer in bee_name_suggestion | P1 |  |
| BNS-17 | AcceptBeeNameSuggestion | Concurrency Invariant | Two goroutines concurrently accept the same freshly-submitted suggestion name | one suggestion seeded via SubmitBeeName; store shared across goroutines; bee_name.name is a PRIMARY KEY (requires live DB) | Exactly one of the two calls succeeds (the other fails on the PRIMARY KEY conflict); after both complete, the name is absent from bee_name_suggestion and present exactly once in bee_name (looped across trials - see test file comment) | P0 | Looped over 20 trials: a single trial can't reliably distinguish the PRIMARY KEY guard from a race that occasionally lets both inserts through. |
| BNS-18 | RejectBeeNameSuggestion | Error Path | DB connection unreachable | store.db points at a closed local port | Returns ("", non-nil connection error) | P2 |  |
| BNS-19 | RejectBeeNameSuggestion | Happy Path | Suggestion previously submitted | seeded via SubmitBeeName (requires live DB) | Returns (name, nil); name no longer in bee_name_suggestion | P1 |  |
| BNS-20 | RejectBeeNameSuggestion | Edge Case | Name does not exist in suggestions | live store, unused name (requires live DB) | Returns (name, nil) - DELETE matching zero rows is not an error | P3 |  |
| BNS-21 | UploadBeeName | Concurrency Invariant | Two goroutines concurrently upload the same name | store shared across goroutines; bee_name.name is a PRIMARY KEY (requires live DB) | Exactly one of the two calls succeeds (the other fails on the PRIMARY KEY conflict); after both complete, the name is present exactly once in bee_name (looped across trials - see test file comment) | P0 | Looped over 20 trials: a single trial can't reliably distinguish the PRIMARY KEY guard from a race that occasionally lets both inserts through. |
| BNS-22 | SubmitBeeName | Concurrency Invariant | Two goroutines concurrently submit the same name | store shared across goroutines; bee_name_suggestion.name is a PRIMARY KEY (requires live DB) | Exactly one of the two calls succeeds (the other fails on the PRIMARY KEY conflict); after both complete, the name is present exactly once in bee_name_suggestion (looped across trials - see test file comment) | P0 | Looped over 20 trials: a single trial can't reliably distinguish the PRIMARY KEY guard from a race that occasionally lets both inserts through. |
