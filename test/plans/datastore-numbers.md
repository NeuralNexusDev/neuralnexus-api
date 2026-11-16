# Test plan: datastore-numbers

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
|  | translateNumberErr | Edge Case | Database error that is not one of the three handled codes, or an error that did not come from the database |  | Returns the error unchanged | P3 | Blank ID: no test; the four methods' live-test errors are all database errors, and another needs a broken connection |

## numberStore

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| NDS-01 | Create | Error Path | Second insert for a store that already has a number row | Live database; account and datastore row inserted; first create succeeded | Returns `ErrNumberExists` (unique violation `23505`) | P1 |  |
| NDS-02 | Create | Error Path | Store id that has no data store | Live database; account row inserted | Returns `ErrNumberStoreOrUserNotFound` (foreign key violation `23503`) | P1 |  |
| NDS-03 | Create | Error Path | User id that has no account | Live database; datastore row inserted | Returns `ErrNumberStoreOrUserNotFound` (foreign key violation `23503`) | P1 |  |
| NDS-04 | Update | Error Path | Update of a number row that does not exist | Live database; datastore exists, no number row | Returns `0, ErrNumberNotFound` because no row was affected | P1 |  |
| NDS-05 | Delete | Error Path | Delete of a number row that does not exist | Live database; datastore exists, no number row | Returns `ErrNumberNotFound` because no row was affected | P1 |  |
| NDS-06 | Delete | Happy Path | Delete of an existing number row | Live database; number row created | The delete returns nil; a following read returns `ErrNumberNotFound` | P1 |  |
| NDS-07 | Create | Error Path | Store id `"abc"` is not a number | Live database; account row inserted | Each of the four methods returns `ErrNumberInvalidID` (the database reports an invalid text representation) | P1 | Runs for create, read, update and delete |

## handler

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| NDH-01 | CreateNumber | Error Path | Service returns `ErrNumberExists` | Service fails with the error; valid body; session holds `numberstore.admin` | 409 with detail `A number already exists for this data store` | P1 |  |
| NDH-02 | CreateNumber | Error Path | Service returns `ErrNumberStoreOrUserNotFound` | Service fails with the error; valid body; session holds `numberstore.admin` | 404 with detail `Data store or user not found` | P1 |  |
| NDH-03 | CreateNumber | Error Path | Service returns an unexpected error | Service fails with the error; valid body; session holds `numberstore.admin` | 500 with detail `Failed to create numberstore` | P1 |  |
| NDH-04 | ReadNumber | Error Path | Service returns `ErrNumberNotFound` | Service fails with the error; valid body; session holds `numberstore.admin` | 404 with detail `Number not found` | P1 |  |
| NDH-05 | ReadNumber | Error Path | Service returns an unexpected error | Service fails with the error; valid body; session holds `numberstore.admin` | 500 with detail `Failed to read numberstore` | P1 |  |
| NDH-06 | UpdateNumber | Error Path | Service returns `ErrNumberNotFound` | Service fails with the error; valid body; session holds `numberstore.admin` | 404 with detail `Number not found` | P1 |  |
| NDH-07 | UpdateNumber | Error Path | Service returns an unexpected error | Service fails with the error; valid body; session holds `numberstore.admin` | 500 with detail `Failed to update numberstore` | P1 |  |
| NDH-08 | DeleteNumber | Error Path | Service returns `ErrNumberNotFound` | Service fails with the error; valid body; session holds `numberstore.admin` | 404 with detail `Number not found` | P1 |  |
| NDH-09 | DeleteNumber | Error Path | Service returns an unexpected error | Service fails with the error; valid body; session holds `numberstore.admin` | 500 with detail `Failed to delete numberstore` | P1 |  |
| NDH-10 | CreateNumber | Error Path | Service returns `ErrNumberInvalidID` | Service fails with the error; valid body; session holds `numberstore.admin` | Each handler answers 400 with detail `Data store and user ids must be numbers` | P1 | Runs for the create, read, update and delete handlers |
| NDH-11 | CreateNumber | Happy Path | A number is created | Session with the number store admin permission; body names store 1, user 2 and value 3; the service accepts it | Status 200; the body is the number for store 1, user 2 and value 3 | P1 |  |
| NDH-12 | CreateNumber | Error Path | A session without the number store admin permission creates a number | Session with no permissions; valid body | Status 403 with detail `msgNoPermissionToCreateNumberstore` | P1 |  |
| NDH-13 | CreateNumber | Error Path | The request body is not valid JSON | Session with the number store admin permission; body `{` | Status 400 | P1 |  |
| NDH-14 | ReadNumber | Happy Path | A number is read | Valid body; the service returns the number | Status 200; the body is the number for store 1, user 2 and value 3 | P1 |  |
| NDH-15 | ReadNumber | Error Path | The request body is not valid JSON | Body `{` | Status 400 | P1 |  |
| NDH-16 | UpdateNumber | Happy Path | A number is updated | Session with the number store admin permission; valid body; the service accepts the update | Status 200; the body is the number for store 1, user 2 and value 3 | P1 |  |
| NDH-17 | UpdateNumber | Error Path | A session without the number store admin permission updates a number | Session with no permissions; valid body | Status 403 with detail `msgNoPermissionToUpdateNumberstore` | P1 |  |
| NDH-18 | UpdateNumber | Error Path | The request body is not valid JSON | Session with the number store admin permission; body `{` | Status 400 | P1 |  |
| NDH-19 | DeleteNumber | Happy Path | A number is deleted | Valid body; the service accepts the delete | Status 200; the body is the deleted number (store 1, user 2, value 3) | P1 |  |
| NDH-20 | DeleteNumber | Error Path | The request body is not valid JSON | Body `{` | Status 400 | P1 |  |
