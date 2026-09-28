# Test plan: teapot
Mode: FRESH

## teapot.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| TP-01 | HandleTeapot | Happy Path | No `Accept` header set on the request | Request built with no `Accept` header | Status 418; `Content-Type: application/problem+json`; JSON body decodes with `type="about:blank"`, `status=418`, `title="I'm a teapot"`, `detail="You requested a cup of coffee, but I'm a teapot."`, `instance="https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/418"` | P1 |  |
| TP-02 | HandleTeapot | Edge Case | `Accept: application/xml` (exact match) | Request has `Accept` header set to exactly `"application/xml"` | Status 418; `Content-Type: application/problem+xml`; XML body unmarshals with the same five field values as TP-01 | P1 |  |
| TP-03 | HandleTeapot | Edge Case | `Accept: application/x-protobuf` (exact match) | Request has `Accept` header set to exactly `"application/x-protobuf"` | Status 418; `Content-Type: application/problem+x-protobuf`; body is valid protobuf that unmarshals via `problempb.Problem` with the same five field values as TP-01 | P1 |  |
| TP-04 | HandleTeapot | Edge Case | `Accept` header present but not one of the two special-cased values (e.g. `"text/plain"`) | Request has `Accept: "text/plain"` | Falls back to the default branch: status 418, `Content-Type: application/problem+json`, same body as TP-01 | P2 |  |
| TP-05 | HandleTeapot | Edge Case | `Accept` header matches a special-cased value except for letter case (`"APPLICATION/XML"`) | Request has `Accept: "APPLICATION/XML"` | The switch match is an exact, case-sensitive string compare, so this does **not** hit the XML branch: falls back to `Content-Type: application/problem+json`, same body as TP-01 | P2 |  |
| TP-06 | HandleTeapot | Edge Case | `Accept` header holds a compound/quality-weighted value (e.g. `"application/xml, application/json;q=0.9"`) instead of a single bare media type | Request has that compound `Accept` header | No case matches the full string exactly, so it falls back to `Content-Type: application/problem+json`, same body as TP-01 | P3 |  |
| TP-07 | HandleTeapot | Edge Case | `Accept` header explicitly set to the empty string | Request has `Accept` header present with value `""` | Falls back to `Content-Type: application/problem+json`, same body as TP-01 | P3 |  |

## Function inventory self-check
- [x] HandleTeapot — covered by TP-01, TP-02, TP-03, TP-04, TP-05, TP-06, TP-07
