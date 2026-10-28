# Test plan: datastore-numbers

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| NDS-01 | Create | Error Path | Second insert for a store that already has a number row | Live database; account and datastore row inserted; first `Create` succeeded | Returns `ErrNumberExists` (unique violation `23505`) | P1 |  |
| NDS-02 | Create | Error Path | Store id that has no `datastores` row | Live database; account row inserted | Returns `ErrNumberStoreOrUserNotFound` (foreign key violation `23503`) | P1 |  |
| NDS-03 | Create | Error Path | User id that has no `accounts` row | Live database; datastore row inserted | Returns `ErrNumberStoreOrUserNotFound` (foreign key violation `23503`) | P1 |  |
| NDS-04 | Update | Error Path | Update of a number row that does not exist | Live database; datastore exists, no number row | Returns `0, ErrNumberNotFound` because no row was affected | P1 |  |
| NDS-05 | Delete | Error Path | Delete of a number row that does not exist | Live database; datastore exists, no number row | Returns `ErrNumberNotFound` because no row was affected | P1 |  |
| NDS-06 | Delete | Happy Path | Delete of an existing number row | Live database; number row created | `Delete` returns nil; a following `Read` returns `ErrNumberNotFound` | P1 |  |
| NDS-07 | Create, Read, Update, Delete | Error Path | Store id `"abc"` is not a number | Live database; account row inserted | Each of the four methods returns `ErrNumberInvalidID` (invalid text representation `22P02`) | P1 |  |
|  | translateNumberErr | Edge Case | Error that is not a `*pgconn.PgError`, or a `PgError` with another code |  | Returns the error unchanged | P3 | Blank ID: no test; every database error reachable from the four methods in the live tests is a `PgError`, and reaching another one needs a broken connection |

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| NDH-01 | CreateNumberHandler | Error Path | Service returns `ErrNumberExists` | Fake service returns the error; valid body; session holds `numberstore.admin` | 409 with detail `A number already exists for this data store` | P1 |  |
| NDH-02 | CreateNumberHandler | Error Path | Service returns `ErrNumberStoreOrUserNotFound` | Fake service returns the error; valid body; session holds `numberstore.admin` | 404 with detail `Data store or user not found` | P1 |  |
| NDH-03 | CreateNumberHandler | Error Path | Service returns an unexpected error | Fake service returns the error; valid body; session holds `numberstore.admin` | 500 with detail `Failed to create numberstore` | P1 |  |
| NDH-04 | ReadNumberHandler | Error Path | Service returns `ErrNumberNotFound` | Fake service returns the error; valid body; session holds `numberstore.admin` | 404 with detail `Number not found` | P1 |  |
| NDH-05 | ReadNumberHandler | Error Path | Service returns an unexpected error | Fake service returns the error; valid body; session holds `numberstore.admin` | 500 with detail `Failed to read numberstore` | P1 |  |
| NDH-06 | UpdateNumberHandler | Error Path | Service returns `ErrNumberNotFound` | Fake service returns the error; valid body; session holds `numberstore.admin` | 404 with detail `Number not found` | P1 |  |
| NDH-07 | UpdateNumberHandler | Error Path | Service returns an unexpected error | Fake service returns the error; valid body; session holds `numberstore.admin` | 500 with detail `Failed to update numberstore` | P1 |  |
| NDH-08 | DeleteNumberHandler | Error Path | Service returns `ErrNumberNotFound` | Fake service returns the error; valid body; session holds `numberstore.admin` | 404 with detail `Number not found` | P1 |  |
| NDH-09 | DeleteNumberHandler | Error Path | Service returns an unexpected error | Fake service returns the error; valid body; session holds `numberstore.admin` | 500 with detail `Failed to delete numberstore` | P1 |  |
| NDH-10 | CreateNumberHandler, ReadNumberHandler, UpdateNumberHandler, DeleteNumberHandler | Error Path | Service returns `ErrNumberInvalidID` | Fake service returns the error; valid body; session holds `numberstore.admin` | Each handler answers 400 with detail `Data store and user ids must be numbers` | P1 |  |
