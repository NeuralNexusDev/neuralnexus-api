# Test plan: bee_name_generator
Mode: FRESH

## types.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority |
|----|----------|---------------|----------|---------------|------------------|----------|
| TY-01 | NewBeeName | Accessor | Normal, non-empty name | name = "Buzzy" | Returns &bngpb.BeeName{Name: "Buzzy"} | P3 |
| TY-02 | NewBeeName | Edge Case | Empty string name | name = "" | Returns &bngpb.BeeName{Name: ""} (non-nil pointer, zero-value field) | P3 |
| TY-03 | NewBeeNameSuggestions | Accessor | Normal, populated slice | suggestions = []string{"a","b"} | Returns &bngpb.BeeNameSuggestions{Suggestions: []string{"a","b"}} | P3 |
| TY-04 | NewBeeNameSuggestions | Edge Case | Nil slice | suggestions = nil | Returns &bngpb.BeeNameSuggestions{Suggestions: nil} (non-nil pointer) | P3 |

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority |
|----|----------|---------------|----------|---------------|------------------|----------|
| HD-01 | GetBeeNameHandler | Happy Path | Store returns a name | fake.GetBeeName -> ("Buzzy", nil) | 200 OK, body decodes to BeeName{Name:"Buzzy"} | P1 |
| HD-02 | GetBeeNameHandler | Error Path | Store returns an error | fake.GetBeeName -> ("", err) | 500 Internal Server Error | P2 |
| HD-03 | UploadBeeNameHandler | Error Path | Session lacks the admin scope | session without ScopeAdminBeeNameGenerator | 403 Forbidden, store not called | P1 |
| HD-04 | UploadBeeNameHandler | Edge Case | Empty name path value | session has scope; name = "" | 400 Bad Request, store not called | P2 |
| HD-05 | UploadBeeNameHandler | Happy Path | Valid name, authorized session | fake.UploadBeeName -> (name, nil) | 200 OK, body decodes to BeeName{Name:name} | P1 |
| HD-06 | UploadBeeNameHandler | Error Path | Store returns an error | fake.UploadBeeName -> ("", err) | 500 Internal Server Error | P2 |
| HD-07 | DeleteBeeNameHandler | Error Path | Session lacks the admin scope | session without ScopeAdminBeeNameGenerator | 403 Forbidden, store not called | P1 |
| HD-08 | DeleteBeeNameHandler | Edge Case | Empty name path value | session has scope; name = "" | 400 Bad Request, store not called | P2 |
| HD-09 | DeleteBeeNameHandler | Happy Path | Valid name, authorized session | fake.DeleteBeeName -> (name, nil) | 204 No Content | P1 |
| HD-10 | DeleteBeeNameHandler | Error Path | Store returns an error | fake.DeleteBeeName -> ("", err) | 500 Internal Server Error | P2 |
| HD-11 | SubmitBeeNameHandler | Edge Case | Empty name path value | name = "" | 400 Bad Request, store not called | P2 |
| HD-12 | SubmitBeeNameHandler | Happy Path | Valid name (no auth required) | fake.SubmitBeeName -> (name, nil) | 200 OK, body decodes to BeeName{Name:name} | P1 |
| HD-13 | SubmitBeeNameHandler | Error Path | Store returns an error | fake.SubmitBeeName -> ("", err) | 500 Internal Server Error | P2 |
| HD-14 | GetBeeNameSuggestionsHandler | Error Path | Session lacks the admin scope | session without ScopeAdminBeeNameGenerator | 403 Forbidden, store not called | P1 |
| HD-15 | GetBeeNameSuggestionsHandler | Edge Case | Empty amount path value | session has scope; amount = "" | 400 Bad Request (empty is coerced to "NAN", fails ParseInt) | P2 |
| HD-16 | GetBeeNameSuggestionsHandler | Edge Case | Amount path value is "0" | amount = "0" | 400 Bad Request ("0" is coerced to "NAN", fails ParseInt) | P2 |
| HD-17 | GetBeeNameSuggestionsHandler | Edge Case | Amount is non-numeric | amount = "abc" | 400 Bad Request | P2 |
| HD-18 | GetBeeNameSuggestionsHandler | Happy Path | Amount is a valid positive integer | amount = "5"; fake.GetBeeNameSuggestions -> ([]string{"a","b"}, nil) | 200 OK, body decodes to BeeNameSuggestions{Suggestions:["a","b"]} | P1 |
| HD-19 | GetBeeNameSuggestionsHandler | Error Path | Store returns an error | fake.GetBeeNameSuggestions -> (nil, err) | 500 Internal Server Error | P2 |
| HD-20 | AcceptBeeNameSuggestionHandler | Error Path | Session lacks the admin scope | session without ScopeAdminBeeNameGenerator | 403 Forbidden, store not called | P1 |
| HD-21 | AcceptBeeNameSuggestionHandler | Edge Case | Empty name path value | session has scope; name = "" | 400 Bad Request, store not called | P2 |
| HD-22 | AcceptBeeNameSuggestionHandler | Happy Path | Valid name, authorized session | fake.AcceptBeeNameSuggestion -> (name, nil) | 200 OK, body decodes to BeeName{Name:name} | P1 |
| HD-23 | AcceptBeeNameSuggestionHandler | Error Path | Store returns an error | fake.AcceptBeeNameSuggestion -> ("", err) | 500 Internal Server Error | P2 |
| HD-24 | RejectBeeNameSuggestionHandler | Error Path | Session lacks the admin scope | session without ScopeAdminBeeNameGenerator | 403 Forbidden, store not called | P1 |
| HD-25 | RejectBeeNameSuggestionHandler | Edge Case | Empty name path value | session has scope; name = "" | 400 Bad Request, store not called | P2 |
| HD-26 | RejectBeeNameSuggestionHandler | Happy Path | Valid name, authorized session | fake.RejectBeeNameSuggestion -> (name, nil) | 204 No Content | P1 |
| HD-27 | RejectBeeNameSuggestionHandler | Error Path | Store returns an error | fake.RejectBeeNameSuggestion -> ("", err) | 500 Internal Server Error | P2 |

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority |
|----|----------|---------------|----------|---------------|------------------|----------|
| ST-01 | NewStore | Accessor | Wraps the given pool | db = *pgxpool.Pool | Returned BNGStore is a *store whose db field == the given pool | P3 |
| ST-02 | GetBeeName | Error Path | DB connection unreachable | store.db points at a closed local port | Returns ("", non-nil connection error) | P2 |
| ST-03 | GetBeeName | Happy Path (live) | A row exists in bee_name | seeded via UploadBeeName | Returns (name, nil) for one of the seeded names | P1 |
| ST-04 | GetBeeName | Edge Case (live) | bee_name has no matching row (empty/only-other-rows result) | table filtered to nothing scannable | Returns ("", non-nil error, e.g. pgx.ErrNoRows) | P2 |
| ST-05 | UploadBeeName | Error Path | DB connection unreachable | store.db points at a closed local port | Returns ("", non-nil connection error) | P2 |
| ST-06 | UploadBeeName | Happy Path (live) | Valid name | live store | Returns (name, nil); row is queryable back via GetBeeName | P1 |
| ST-07 | DeleteBeeName | Error Path | DB connection unreachable | store.db points at a closed local port | Returns ("", non-nil connection error) | P2 |
| ST-08 | DeleteBeeName | Happy Path (live) | Name previously uploaded | seeded via UploadBeeName | Returns (name, nil); row no longer present | P1 |
| ST-09 | DeleteBeeName | Edge Case (live) | Name does not exist | live store, unused name | Returns (name, nil) - DELETE matching zero rows is not an error | P3 |
| ST-10 | SubmitBeeName | Error Path | DB connection unreachable | store.db points at a closed local port | Returns ("", non-nil connection error) | P2 |
| ST-11 | SubmitBeeName | Happy Path (live) | Valid name | live store | Returns (name, nil); row appears in bee_name_suggestion | P1 |
| ST-12 | GetBeeNameSuggestions | Error Path | DB connection unreachable | store.db points at a closed local port | Returns ([]string{}, non-nil connection error) | P2 |
| ST-13 | GetBeeNameSuggestions | Happy Path (live) | N suggestions seeded, amount <= N | seeded via SubmitBeeName | Returns (slice of length == amount, nil), all elements from the seeded set | P1 |
| ST-14 | GetBeeNameSuggestions | Edge Case (live) | No rows match (empty suggestion table for a fresh unique batch) | live store, unqueried unique names | Returns ([]string{}, nil) | P2 |
| ST-15 | AcceptBeeNameSuggestion | Error Path | DB connection unreachable (insert fails) | store.db points at a closed local port | Returns ("", non-nil connection error); no partial state possible since insert itself fails | P2 |
| ST-16 | AcceptBeeNameSuggestion | Happy Path (live) | Suggestion previously submitted | seeded via SubmitBeeName | Returns (name, nil); name now in bee_name, no longer in bee_name_suggestion | P1 |
| ST-17 | AcceptBeeNameSuggestion | Concurrency Invariant | Two goroutines concurrently accept the same freshly-submitted suggestion name | one suggestion seeded via SubmitBeeName; store shared across goroutines | Both calls return without deadlock/panic; after both complete, the name is absent from bee_name_suggestion and present at least once in bee_name (looped across trials - see test file comment) | P0 |
| ST-18 | RejectBeeNameSuggestion | Error Path | DB connection unreachable | store.db points at a closed local port | Returns ("", non-nil connection error) | P2 |
| ST-19 | RejectBeeNameSuggestion | Happy Path (live) | Suggestion previously submitted | seeded via SubmitBeeName | Returns (name, nil); name no longer in bee_name_suggestion | P1 |
| ST-20 | RejectBeeNameSuggestion | Edge Case (live) | Name does not exist in suggestions | live store, unused name | Returns (name, nil) - DELETE matching zero rows is not an error | P3 |

## Function inventory self-check
- [x] NewBeeName — covered by TY-01, TY-02
- [x] NewBeeNameSuggestions — covered by TY-03, TY-04
- [x] GetBeeNameHandler — covered by HD-01, HD-02
- [x] UploadBeeNameHandler — covered by HD-03, HD-04, HD-05, HD-06
- [x] DeleteBeeNameHandler — covered by HD-07, HD-08, HD-09, HD-10
- [x] SubmitBeeNameHandler — covered by HD-11, HD-12, HD-13
- [x] GetBeeNameSuggestionsHandler — covered by HD-14, HD-15, HD-16, HD-17, HD-18, HD-19
- [x] AcceptBeeNameSuggestionHandler — covered by HD-20, HD-21, HD-22, HD-23
- [x] RejectBeeNameSuggestionHandler — covered by HD-24, HD-25, HD-26, HD-27
- [x] NewStore — covered by ST-01
- [x] (*store) GetBeeName — covered by ST-02, ST-03, ST-04
- [x] (*store) UploadBeeName — covered by ST-05, ST-06
- [x] (*store) DeleteBeeName — covered by ST-07, ST-08, ST-09
- [x] (*store) SubmitBeeName — covered by ST-10, ST-11
- [x] (*store) GetBeeNameSuggestions — covered by ST-12, ST-13, ST-14
- [x] (*store) AcceptBeeNameSuggestion — covered by ST-15, ST-16, ST-17
- [x] (*store) RejectBeeNameSuggestion — covered by ST-18, ST-19, ST-20
