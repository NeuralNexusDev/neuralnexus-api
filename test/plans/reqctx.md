# Test plan: reqctx

## reqctx.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RQ-01 | Log | Happy Path | Log a request whose context has a user ID | `ctx` has `RemoteAddrKey` `"127.0.0.1"`, `RequestIDKey` 42, `UserIDKey` `"u1"` | Log output is exactly `42 u1 127.0.0.1 hello world` and a newline for messages `"hello", "world"` | P1 |  |
| RQ-02 | Log | Happy Path | Log a request with no user ID in context | As RQ-01 without `UserIDKey` | Log output is exactly `42 N/A 127.0.0.1 hello world` and a newline | P1 |  |
| RQ-03 | Log | Edge Case | Called with zero message arguments | `ctx` has the remote address and request ID | Output is `42 N/A 127.0.0.1 ` with an empty message segment, no panic | P3 |  |
| RQ-04 | Log | Error Path | Remote address missing from context | `ctx` has `RequestIDKey` only | Panics from the unchecked type assertion; nothing is logged | P2 |  |
| RQ-05 | Log | Error Path | Request ID missing from context | `ctx` has `RemoteAddrKey` only | Panics from the unchecked type assertion; nothing is logged | P2 |  |
| RQ-06 | keys | Happy Path | Key constants hold their iota values | None | `SessionKey` is 0, `RequestIDKey` is 1, `RemoteAddrKey` is 2, `UserIDKey` is 3 | P2 |  |
