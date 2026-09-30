# Test plan: modules/datastore

## handler.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| HD-01 | DeleteDataStoreHandler | Error Path | Session lacks the admin datastore scope | Session with no permissions | 403 Forbidden, `detail` is `msgNoPermissionToDeleteDatastore`; `DSService.Delete` never called | P1 |  |
| HD-02 | DeleteDataStoreHandler | Error Path | Request body is not valid JSON | Admin session, body `not json` | 400 Bad Request; `DSService.Delete` never called | P2 |  |
| HD-03 | DeleteDataStoreHandler | Error Path | `DSService.Delete` fails | Admin session, valid body, service returns `testerrors.ErrDBDown` | 500 Internal Server Error, `detail` is `msgFailedToDeleteDatastore` | P2 |  |
| HD-04 | DeleteDataStoreHandler | Happy Path | `DSService.Delete` succeeds | Admin session, valid body | 204 No Content; `DSService.Delete` called once with the decoded `StoreID` | P1 | The other datastore handlers are not covered by this plan. |
