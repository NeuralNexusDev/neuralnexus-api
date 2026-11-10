# Test plan: datastore

## dataStore

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| DSS-01 | DeleteDataStore | Error Path | Delete a store id that has no row | Live schema, no store with that id | Returns `ErrDataStoreNotFound` | P1 |  |
| DSS-02 | DeleteDataStore | Happy Path | Delete an existing store | Store created for the seeded owner | Returns nil, and getting the same store then returns `ErrDataStoreNotFound` | P1 |  |
| DSS-03 | DeleteDataStore | Error Path | Delete a store that a number row refers to | Store and number row exist | Returns `ErrDataStoreInUse` | P1 |  |
| DSS-04 | UpdateDataStore | Error Path | Update a store to an owner id with no account | Store exists; owner id has no account | Returns `ErrDataStoreOwnerNotFound` | P1 |  |
| DSS-05 | CreateNewDataStore | Error Path | Create a store for an owner id with no account | Owner id has no account | Returns `ErrDataStoreOwnerNotFound` | P1 |  |
| DSS-06 | GetDataStore | Edge Case | Store id `"abc"` is not a number | Live schema | Each of the four calls returns `ErrDataStoreInvalidID` | P1 | Runs for get, update, delete and create |

## handler

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| DSH-01 | ReadDataStore | Error Path | Service returns `ErrDataStoreNotFound` | Service fails with `ErrDataStoreNotFound` | Status 404 | P1 |  |
| DSH-02 | ReadDataStore | Error Path | Service returns an unexpected error | Service fails with an unexpected error | Status 500 | P1 |  |
| DSH-03 | ReadDataStore | Happy Path | Service returns the store | Service finds the store | Status 200 | P1 |  |
| DSH-04 | UpdateDataStore | Error Path | Service returns `ErrDataStoreNotFound` | Session holds `datastore.admin` | Status 404 | P1 |  |
| DSH-05 | UpdateDataStore | Error Path | Service returns `ErrDataStoreOwnerNotFound` | Session holds `datastore.admin` | Status 404 | P1 |  |
| DSH-06 | UpdateDataStore | Error Path | Service returns an unexpected error | Session holds `datastore.admin` | Status 500 | P1 |  |
| DSH-07 | DeleteDataStore | Error Path | Service returns `ErrDataStoreNotFound` | Session holds `datastore.admin` | Status 404 | P1 |  |
| DSH-08 | DeleteDataStore | Error Path | Service returns `ErrDataStoreInUse` | Session holds `datastore.admin` | Status 409 | P1 |  |
| DSH-09 | DeleteDataStore | Error Path | Service returns an unexpected error | Session holds `datastore.admin` | Status 500 | P1 |  |
| DSH-10 | DeleteDataStore | Happy Path | Service returns nil | Session holds `datastore.admin` | Status 204 | P1 |  |
| DSH-11 | ReadDataStore | Error Path | Service returns `ErrDataStoreInvalidID` | Session holds `datastore.admin` | Each handler answers status 400 | P1 | Runs for the read, update and delete handlers |
