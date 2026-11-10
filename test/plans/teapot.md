# Test plan: teapot

## teapot.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| TP-01 | HandleTeapot | Happy Path | No `Accept` header set on the request | Request built with no `Accept` header | Status 418; `Content-Type: application/problem+json`; the JSON body has a `detail` of `You requested a cup of coffee, but I'm a teapot.` | P1 |  |
| TP-02 | HandleTeapot | Edge Case | `Accept: application/xml` (exact match) | Request has `Accept` header set to exactly `"application/xml"` | Status 418; `Content-Type: application/problem+xml`; the XML body has a `detail` of `You requested a cup of coffee, but I'm a teapot.` | P1 |  |
| TP-03 | HandleTeapot | Edge Case | `Accept: application/x-protobuf` (exact match) | Request has `Accept` header set to exactly `"application/x-protobuf"` | Status 418; `Content-Type: application/problem+x-protobuf`; the protobuf body has a `detail` of `You requested a cup of coffee, but I'm a teapot.` | P1 |  |
