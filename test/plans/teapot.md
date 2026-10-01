# Test plan: teapot

## teapot.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| TP-01 | HandleTeapot | Happy Path | No `Accept` header set on the request | Request built with no `Accept` header | Status `http.StatusTeapot`; `Content-Type: application/problem+json`; JSON body decodes into `responses.Problem` with `Detail` equal to `wantDetail` | P1 |  |
| TP-02 | HandleTeapot | Edge Case | `Accept: application/xml` (exact match) | Request has `Accept` header set to exactly `"application/xml"` | Status `http.StatusTeapot`; `Content-Type: application/problem+xml`; XML body decodes into `responses.Problem` with `Detail` equal to `wantDetail` | P1 |  |
| TP-03 | HandleTeapot | Edge Case | `Accept: application/x-protobuf` (exact match) | Request has `Accept` header set to exactly `"application/x-protobuf"` | Status `http.StatusTeapot`; `Content-Type: application/problem+x-protobuf`; body decodes via `problempb.Problem` with `Detail` equal to `wantDetail` | P1 |  |
| TP-04 | HandleTeapot | Edge Case | `Accept` header present but not one of the two special-cased values (e.g. `"text/plain"`) | Request has `Accept: "text/plain"` | Falls back to the default branch: status `http.StatusTeapot`, `Content-Type: application/problem+json`, JSON body with `Detail` equal to `wantDetail` | P2 |  |
| TP-05 | HandleTeapot | Edge Case | `Accept` header matches a special-cased value except for letter case (`"APPLICATION/XML"`) | Request has `Accept: "APPLICATION/XML"` | The switch match is an exact, case-sensitive string compare, so this does **not** hit the XML branch: status `http.StatusTeapot`, `Content-Type: application/problem+json`, JSON body with `Detail` equal to `wantDetail` | P2 |  |
| TP-06 | HandleTeapot | Edge Case | `Accept` header holds a compound/quality-weighted value (e.g. `"application/xml, application/json;q=0.9"`) instead of a single bare media type | Request has that compound `Accept` header | No case matches the full string exactly, so it falls back to status `http.StatusTeapot`, `Content-Type: application/problem+json`, JSON body with `Detail` equal to `wantDetail` | P3 |  |
| TP-07 | HandleTeapot | Edge Case | `Accept` header explicitly set to the empty string | Request has `Accept` header present with value `""` | Falls back to status `http.StatusTeapot`, `Content-Type: application/problem+json`, JSON body with `Detail` equal to `wantDetail` | P3 |  |
