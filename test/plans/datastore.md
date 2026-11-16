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
| DSH-12 | CreateDataStore | Happy Path | A session with the data store admin permission creates a store | Session of user 2 with the data store admin permission; the service accepts the store | Status 200; the body is a store owned by user 2 with a generated, non-empty store id | P1 |  |
| DSH-13 | CreateDataStore | Error Path | A session without the data store admin permission creates a store | Session with no permissions | Status 403 with detail `msgNoPermissionToCreateDatastore` | P1 |  |
| DSH-14 | CreateDataStore | Error Path | The service fails | Session with the data store admin permission; the service returns an unexpected error | Status 500 with detail `msgFailedToCreateDatastore` | P1 |  |
| DSH-15 | UpdateDataStore | Error Path | A session without the data store admin permission updates a store | Session with no permissions; valid body | Status 403 with detail `msgNoPermissionToUpdateDatastore` | P1 |  |
| DSH-16 | DeleteDataStore | Error Path | A session without the data store admin permission deletes a store | Session with no permissions; valid body | Status 403 with detail `msgNoPermissionToDeleteDatastore` | P1 |  |
| DSH-17 | ReadDataStore | Error Path | The request body is not valid JSON | Body `{` | Status 400 | P1 |  |
| DSH-18 | UpdateDataStore | Error Path | The request body is not valid JSON | Session with the data store admin permission; body `{` | Status 400 | P1 |  |
| DSH-19 | DeleteDataStore | Error Path | The request body is not valid JSON | Session with the data store admin permission; body `{` | Status 400 | P1 |  |
| DSH-20 | UpdateDataStore | Happy Path | A store is updated | Session with the data store admin permission; body names store 1 and owner 2; the service accepts the update | Status 200; the body is store 1 owned by user 2 | P1 |  |
