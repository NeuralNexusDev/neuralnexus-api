# Test plan: responses

## response.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RR-01 | TooManyRequests | Edge Case | Retry-After is sent while the local time zone is not UTC | `time.Local` set to a fixed non-UTC zone; retryAfter of 60 | Status `http.StatusTooManyRequests`; the `Retry-After` header ends in `GMT`, parses with `http.ParseTime` and is about 60 seconds after the call | P2 | The plan covers only this function. |
