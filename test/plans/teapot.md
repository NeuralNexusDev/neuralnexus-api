# Test plan: teapot

## teapot.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| TP-01 | HandleTeapot | Happy Path | No `Accept` header set on the request | Request built with no `Accept` header | Status `http.StatusTeapot`; `Content-Type: application/problem+json`; JSON body decodes into `responses.Problem` with `Detail` equal to `wantDetail` | P1 |  |
| TP-02 | HandleTeapot | Edge Case | `Accept: application/xml` (exact match) | Request has `Accept` header set to exactly `"application/xml"` | Status `http.StatusTeapot`; `Content-Type: application/problem+xml`; XML body decodes into `responses.Problem` with `Detail` equal to `wantDetail` | P1 |  |
| TP-03 | HandleTeapot | Edge Case | `Accept: application/x-protobuf` (exact match) | Request has `Accept` header set to exactly `"application/x-protobuf"` | Status `http.StatusTeapot`; `Content-Type: application/problem+x-protobuf`; body decodes via `problempb.Problem` with `Detail` equal to `wantDetail` | P1 |  |
