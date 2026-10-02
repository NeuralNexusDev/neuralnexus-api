# Test plan: responses

## response.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RR-01 | TooManyRequests | Edge Case | Retry-After is sent while the local time zone is not UTC | `time.Local` set to a fixed non-UTC zone; retryAfter of 60 | Status `http.StatusTooManyRequests`; the `Retry-After` header ends in `GMT`, parses with `http.ParseTime` and is about 60 seconds after the call | P2 |  |
| RR-02 | SendProblemStruct | Happy Path | a struct embedding a Problem plus `host` and `port` members is sent with no `Accept` header | NotFound problem with host `"example.com"` and port 25565 | Status 404, `Content-Type: application/problem+json`, and the JSON body carries `status`, `title`, `detail` together with `host` and `port` | P1 |  |
| RR-03 | SendProblemStruct | Edge Case | `Accept: application/xml` | same struct | `Content-Type: application/problem+xml` and the XML body carries the `host` and `port` elements | P2 |  |
