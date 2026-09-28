# Test plan: middleware
Mode: LEGACY REPLACEMENT

## middleware.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority |
|----|----------|---------------|----------|---------------|------------------|----------|
| MW-01 | LogRequest | Happy Path | Log a request that has an authenticated session in context | `ctx` has `RemoteAddrKey`, `RequestIDKey`, and `SessionKey` set to a `*auth.Session` with `UserID = "u1"` | Log line contains the request ID, `"u1"`, the remote addr, and the joined message | P2 |
| MW-02 | LogRequest | Happy Path | Log a request with no session in context | `ctx` has `RemoteAddrKey`, `RequestIDKey` set; `SessionKey` absent | Log line contains `"N/A"` in place of a user ID | P2 |
| MW-03 | LogRequest | Edge Case | Called with zero variadic message arguments | `ctx` fully populated, `message` omitted | Log line still contains request ID/user/remote-addr with an empty joined-message segment, no panic | P3 |
| MW-04 | LogRequest | Edge Case | Context is missing `RemoteAddrKey` | `ctx` has no value set for `RemoteAddrKey` | Function panics (failed type assertion on `ctx.Value(RemoteAddrKey).(string)`) | P3 |
| MW-05 | CreateStack | Happy Path | Compose several middlewares into one stack | 3 middlewares, each appending its own name to a shared slice before calling `next` | Execution order matches declaration order (first-declared middleware runs first/outermost) | P1 |
| MW-06 | CreateStack | Edge Case | Called with zero middlewares | `middlewares` is empty | Returned `Middleware` is a pure passthrough — wraps `next` unchanged | P3 |
| MW-07 | WrappedWriter.WriteHeader | Accessor | Set a status code through the wrapper | `WrappedWriter` wraps an `httptest.ResponseRecorder` | `w.statusCode` equals the given code, and the underlying recorder's `Code` is also set to it | P2 |
| MW-08 | IPMiddleware | Happy Path | `CF-Connecting-IP` header present | Request has `CF-Connecting-IP: 1.2.3.4`, no `X-Forwarded-For` | `r.RemoteAddr` becomes `"1.2.3.4"`; `RemoteAddrKey` in context matches | P1 |
| MW-09 | IPMiddleware | Happy Path | `X-Forwarded-For` header present with a single IP | Request has `X-Forwarded-For: 5.6.7.8`, no `CF-Connecting-IP` | `r.RemoteAddr` becomes `"5.6.7.8"` | P1 |
| MW-10 | IPMiddleware | Edge Case | `X-Forwarded-For` has multiple comma-separated IPs | `X-Forwarded-For: 9.9.9.9, 10.10.10.10` | `r.RemoteAddr` becomes the first, trimmed IP `"9.9.9.9"` | P2 |
| MW-11 | IPMiddleware | Edge Case | Both `CF-Connecting-IP` and `X-Forwarded-For` present | `CF-Connecting-IP: 1.1.1.1`, `X-Forwarded-For: 2.2.2.2` | `CF-Connecting-IP` takes precedence; `r.RemoteAddr` becomes `"1.1.1.1"` | P2 |
| MW-12 | IPMiddleware | Edge Case | `X-Forwarded-For` present but blank/whitespace-only first segment | `X-Forwarded-For: " ,3.3.3.3"`, no `CF-Connecting-IP` | `r.RemoteAddr` is left unchanged from its original value | P3 |
| MW-13 | IPMiddleware | Edge Case | Neither header present | No `CF-Connecting-IP` or `X-Forwarded-For` | `r.RemoteAddr` unchanged; context `RemoteAddrKey` still set to the original value | P3 |
| MW-14 | SessionMiddleware | Happy Path | No `Authorization` header, no session cookie | Request carries neither | `next` is called; no `*auth.Session` present in the request's context | P1 |
| MW-15 | SessionMiddleware | Error Path | Malformed `Authorization` header (not a single `"Bearer <token>"` split) | `Authorization: Basic xyz` (or bare `"Bearer"`) | `responses.Unauthorized` (401) sent; `next` not called | P1 |
| MW-16 | SessionMiddleware | Happy Path | Valid `Authorization: Bearer <token>`, `ReadJWT` succeeds, session valid | Mock `ReadJWT` returns a valid, non-expired `*auth.Session` | Session placed in context under `SessionKey`; `next` called | P1 |
| MW-17 | SessionMiddleware | Error Path | Valid Bearer format, `ReadJWT` returns an error | Mock `ReadJWT` returns `(nil, err)` | `responses.Unauthorized` (401) sent; `next` not called | P1 |
| MW-18 | SessionMiddleware | Error Path | Valid Bearer format, session expired (`IsValid()` false) | Mock `ReadJWT` returns a session with `ExpiresAt` in the past | `responses.Unauthorized` (401) sent, `service.DeleteSession` called with the session ID; `next` not called | P1 |
| MW-52 | SessionMiddleware | Edge Case | `Authorization` header is exactly `"Bearer "` (trailing space, no token) | `strings.Split(authHeader, "Bearer ")` on `"Bearer "` yields `["", ""]`, length 2 | Header is treated as well-formed and `ReadJWT` is called with an empty string, not rejected up front like a bare `"Bearer"` (no space) is; `ReadJWT` then errors, so `responses.Unauthorized` (401) sent, `next` not called | P3 |
| MW-19 | SessionMiddleware | Edge Case | Header-path expired session, and `DeleteSession` itself errors | As MW-18, but mock `DeleteSession` returns an error | Error is logged; `responses.Unauthorized` (401) is still sent; `next` not called | P2 |
| MW-20 | SessionMiddleware | Happy Path | Valid session cookie | Request carries a `session` cookie; mock `ReadJWT` returns a valid session | Session placed in context; `next` called | P1 |
| MW-21 | SessionMiddleware | Edge Case | Cookie present, `ReadJWT` errors | Mock `ReadJWT` returns `(nil, err)` for the cookie value | Error logged; `next` is still called; no session in context (no 401, unlike the header path) | P2 |
| MW-22 | SessionMiddleware | Edge Case | Cookie present, session expired | Mock `ReadJWT` returns an expired session | `service.DeleteSession` called; `next` still called; no session in context (no 401) | P2 |
| MW-23 | SessionMiddleware | Edge Case | Cookie-path expired session, and `DeleteSession` errors | As MW-22, mock `DeleteSession` returns an error | Error logged; `next` still called; no session in context | P3 |
| MW-24 | RateLimitMiddleware | Happy Path | Session in context, under the session limit | Mock `IncrRateLimit` nil, `GetRateLimit` returns a value `<= sessionLimit` | `next` called; no 429 sent | P1 |
| MW-25 | RateLimitMiddleware | Error Path | Session in context, over the session limit | Mock `GetRateLimit` returns a value `> sessionLimit` | `responses.TooManyRequests` (429) sent with a `Retry-After` header; `next` not called | P1 |
| MW-26 | RateLimitMiddleware | Edge Case | Session in context, `IncrRateLimit` errors | Mock `IncrRateLimit` returns an error | Error logged; `GetRateLimit` is still called and its result still governs whether `next` runs | P2 |
| MW-27 | RateLimitMiddleware | Edge Case | Session in context, `GetRateLimit` errors | Mock `GetRateLimit` returns `(0, err)` | Error logged; `limit` defaults to its zero value; since `0 > sessionLimit` is false for a non-negative limit, `next` is still called | P2 |
| MW-28 | RateLimitMiddleware | Happy Path | No session, valid `host:port` `RemoteAddr`, under the IP limit | `r.RemoteAddr = "1.2.3.4:5678"`; mock returns a value `<= ipLimit` | `next` called; rate-limit keys built from the parsed IP only | P1 |
| MW-29 | RateLimitMiddleware | Error Path | No session, over the IP limit | Mock `GetRateLimit` returns a value `> ipLimit` | `responses.TooManyRequests` (429) sent; `next` not called | P1 |
| MW-30 | RateLimitMiddleware | Edge Case | No session, `RemoteAddr` has no port | `r.RemoteAddr = "1.2.3.4"` (no colon) so `net.SplitHostPort` errors | Falls back to using the raw `RemoteAddr` string as the rate-limit key | P2 |
| MW-31 | RateLimitMiddleware | Edge Case | No session, `IncrRateLimit` errors on the IP branch | Mock `IncrRateLimit` returns an error | Error logged, function returns immediately: `next` is **not** called and no response is explicitly written | P2 |
| MW-32 | RateLimitMiddleware | Edge Case | No session, `GetRateLimit` errors on the IP branch | Mock `GetRateLimit` returns `(0, err)` | Error logged, function returns immediately: `next` is **not** called, no response explicitly written | P2 |
| MW-33 | RequestIDMiddleware | Happy Path | No `X-Request-ID` header | Request has no such header | A generated int ID is placed in context under `RequestIDKey`; the same value is also set back onto `r.Header`'s `X-Request-ID` | P1 |
| MW-34 | RequestIDMiddleware | Happy Path | `X-Request-ID` header present with a valid integer | `X-Request-ID: 42` | Context's `RequestIDKey` is `42` | P1 |
| MW-35 | RequestIDMiddleware | Edge Case | `X-Request-ID` header present but non-numeric | `X-Request-ID: not-a-number` | `strconv.Atoi` error is ignored; context's `RequestIDKey` defaults to `0` | P2 |
| MW-36 | RequestLoggerMiddleware | Happy Path | Downstream handler sets an explicit status code | `next` calls `w.WriteHeader(201)` | The wrapped writer's captured status is `201`; a log line is emitted (via `LogRequest`) containing `201`, the method, and the path | P1 |
| MW-37 | RequestLoggerMiddleware | Edge Case | Downstream handler never calls `WriteHeader` | `next` only writes a body via `Write` | Logged status code is the `WrappedWriter`'s initial default, `http.StatusOK` | P2 |
| MW-38 | Auth | Error Path | No session in context | `SessionKey` absent from context | `responses.Unauthorized` (401) sent; `next` not called | P1 |
| MW-39 | Auth | Edge Case | Context holds a typed-nil `*auth.Session` | `ctx` value at `SessionKey` is `(*auth.Session)(nil)` | `responses.Unauthorized` (401) sent; `next` not called | P2 |
| MW-40 | Auth | Error Path | Session in context is expired | Session with a past `ExpiresAt` | `responses.Unauthorized` (401) sent, `service.DeleteSession` called; `next` not called | P1 |
| MW-41 | Auth | Edge Case | Expired session, `DeleteSession` errors | Mock `DeleteSession` returns an error | Error logged; `responses.Unauthorized` (401) still sent; `next` not called | P2 |
| MW-42 | Auth | Happy Path | Session in context is valid | Non-expired session | `next` called | P1 |
| MW-43 | SelfUserID | Error Path | No session in context | `SessionKey` absent | `responses.Unauthorized` (401) sent; `next` not called; no path value set | P1 |
| MW-44 | SelfUserID | Edge Case | Context holds a typed-nil `*auth.Session` | `ctx` value at `SessionKey` is `(*auth.Session)(nil)` | `responses.Unauthorized` (401) sent; `next` not called | P2 |
| MW-45 | SelfUserID | Happy Path | Valid session in context | Session with `UserID = "u1"` | `r.PathValue("user_id")` becomes `"u1"` before `next` runs; `next` called | P1 |
| MW-46 | VerifyEd25519Middleware | Error Path | Missing `X-Signature-Ed25519` header | No signature header; timestamp header present | `responses.Unauthorized` (401, `"Invalid signature"`); `next` not called | P1 |
| MW-47 | VerifyEd25519Middleware | Error Path | Missing `X-Signature-Timestamp` header | Signature header present; no timestamp header | `responses.Unauthorized` (401, `"Invalid signature"`); `next` not called | P1 |
| MW-48 | VerifyEd25519Middleware | Happy Path | Valid signature and timestamp over the body | Signature computed as `ed25519.Sign(priv, timestamp+body)`; both headers set | `next` called; `r.Body` inside `next` is readable and yields the original body bytes | P0 |
| MW-49 | VerifyEd25519Middleware | Error Path | Headers present but signature verification fails | Signature bytes don't match `timestamp+body` under the public key | `responses.Unauthorized` (401, `"Invalid signature"`); `next` not called | P1 |
| MW-50 | VerifyEd25519Middleware | Edge Case | Signature header present but not valid hex; timestamp present | `X-Signature-Ed25519: not-hex!!` | `hex.DecodeString` fails silently (only logged when a header is also empty, which it isn't here); `ed25519.Verify` is called with an empty signature and returns false; `responses.Unauthorized` (401) sent | P2 |
| MW-51 | VerifyEd25519Middleware | Error Path | `io.ReadAll(r.Body)` fails | Request body is a reader that always returns an error | `responses.Unauthorized` (401, `"Invalid signature"`); `next` not called | P2 |

## Function inventory self-check
- [x] LogRequest — covered by MW-01, MW-02, MW-03, MW-04
- [x] CreateStack — covered by MW-05, MW-06
- [x] WrappedWriter.WriteHeader — covered by MW-07
- [x] IPMiddleware — covered by MW-08, MW-09, MW-10, MW-11, MW-12, MW-13
- [x] SessionMiddleware — covered by MW-14, MW-15, MW-16, MW-17, MW-18, MW-19, MW-20, MW-21, MW-22, MW-23, MW-52
- [x] RateLimitMiddleware — covered by MW-24, MW-25, MW-26, MW-27, MW-28, MW-29, MW-30, MW-31, MW-32
- [x] RequestIDMiddleware — covered by MW-33, MW-34, MW-35
- [x] RequestLoggerMiddleware — covered by MW-36, MW-37
- [x] Auth — covered by MW-38, MW-39, MW-40, MW-41, MW-42
- [x] SelfUserID — covered by MW-43, MW-44, MW-45
- [x] VerifyEd25519Middleware — covered by MW-46, MW-47, MW-48, MW-49, MW-50, MW-51
