# Test plan: datastore

## store.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| DSS-01 | DeleteDataStore | Error Path | Delete a store id that has no row | Live schema, no store with that id | Returns `ErrDataStoreNotFound` | P1 |  |
| DSS-02 | DeleteDataStore | Happy Path | Delete an existing store | Store created for the seeded owner | Returns nil, and `GetDataStore` for the same id then returns `ErrDataStoreNotFound` | P1 |  |
| DSS-03 | DeleteDataStore | Error Path | Delete a store that a `datastore_numbers` row references | Store and number row exist | Returns `ErrDataStoreInUse` | P1 |  |
| DSS-04 | UpdateDataStore | Error Path | Update a store to an owner id with no account | Store exists; owner id has no `accounts` row | Returns `ErrDataStoreOwnerNotFound` | P1 |  |
| DSS-05 | CreateNewDataStore | Error Path | Create a store for an owner id with no account | Owner id has no `accounts` row | Returns `ErrDataStoreOwnerNotFound` | P1 |  |
| DSS-06 | GetDataStore, UpdateDataStore, DeleteDataStore, CreateNewDataStore | Edge Case | Store id `"abc"` is not a number | Live schema | Each of the four calls returns `ErrDataStoreInvalidID` | P1 |  |

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| DSH-01 | ReadDataStoreHandler | Error Path | Service returns `ErrDataStoreNotFound` | Fake service | Status 404 | P1 |  |
| DSH-02 | ReadDataStoreHandler | Error Path | Service returns an unexpected error | Fake service | Status 500 | P1 |  |
| DSH-03 | ReadDataStoreHandler | Happy Path | Service returns the store | Fake service | Status 200 | P1 |  |
| DSH-04 | UpdateDataStoreHandler | Error Path | Service returns `ErrDataStoreNotFound` | Session holds `datastore.admin` | Status 404 | P1 |  |
| DSH-05 | UpdateDataStoreHandler | Error Path | Service returns `ErrDataStoreOwnerNotFound` | Session holds `datastore.admin` | Status 404 | P1 |  |
| DSH-06 | UpdateDataStoreHandler | Error Path | Service returns an unexpected error | Session holds `datastore.admin` | Status 500 | P1 |  |
| DSH-07 | DeleteDataStoreHandler | Error Path | Service returns `ErrDataStoreNotFound` | Session holds `datastore.admin` | Status 404 | P1 |  |
| DSH-08 | DeleteDataStoreHandler | Error Path | Service returns `ErrDataStoreInUse` | Session holds `datastore.admin` | Status 409 | P1 |  |
| DSH-09 | DeleteDataStoreHandler | Error Path | Service returns an unexpected error | Session holds `datastore.admin` | Status 500 | P1 |  |
| DSH-10 | DeleteDataStoreHandler | Happy Path | Service returns nil | Session holds `datastore.admin` | Status 204 | P1 |  |
| DSH-11 | ReadDataStoreHandler, UpdateDataStoreHandler, DeleteDataStoreHandler | Error Path | Service returns `ErrDataStoreInvalidID` | Session holds `datastore.admin` | Each handler answers status 400 | P1 |  |
