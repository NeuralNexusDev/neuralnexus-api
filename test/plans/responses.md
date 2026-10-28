# Test plan: responses

## response.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RR-01 | TooManyRequests | Edge Case | Retry-After is sent while the local time zone is not UTC | `time.Local` set to a fixed non-UTC zone; retryAfter of 60 | Status `http.StatusTooManyRequests`; the `Retry-After` header ends in `GMT`, parses with `http.ParseTime` and is about 60 seconds after the call | P2 |  |
| RR-02 | SendProblemStruct | Happy Path | a struct embedding a Problem plus `host` and `port` members is sent with no `Accept` header | NotFound problem with host `"example.com"` and port 25565 | Status 404, `Content-Type: application/problem+json`, and the JSON body carries `status`, `title`, `detail` together with `host` and `port` | P1 |  |
| RR-03 | SendProblemStruct | Edge Case | `Accept: application/xml` | same struct | `Content-Type: application/problem+xml` and the XML body has a `Problem` root element carrying the `host` and `port` elements | P2 |  |
| RR-04 | SendProblemStruct | Edge Case | `Accept: application/x-protobuf` | same struct | Status 404, `Content-Type: application/problem+x-protobuf`, and the body decodes as the plain Problem message with its status and detail | P2 |  |
| RR-05 | DecodeStruct | Edge Case | JSON body with `ContentLength` -1 | `{"a":1}` into a struct | Decodes with no error and no panic | P1 |  |
| RR-06 | DecodeStruct | Edge Case | Protobuf body with `ContentLength` -1 | Marshalled `wrapperspb.StringValue`, content type `application/x-protobuf` | Decodes with no error and the value intact | P1 |  |
| RR-07 | DecodeStruct | Edge Case | Protobuf body delivered one byte per read | Same body behind `iotest.OneByteReader` | Decodes whole | P1 |  |
| RR-08 | DecodeStruct | Error Path | `ContentLength` of 1<<40, for JSON, XML and protobuf | Small body, huge declared length | Panics with a non-empty string message in each case | P1 |  |
| RR-09 | DecodeStruct | Error Path | No declared length and a body of `maxBodyBytes`+1 bytes | `ContentLength` -1 | Panics with a non-empty string message | P1 |  |
| RR-10 | DecodeStruct | Edge Case | Body of exactly `maxBodyBytes` bytes | JSON string value sized to the limit | Decodes with no error | P2 |  |
| RR-11 | DecodeStruct | Error Path | The body read fails | Body from `iotest.ErrReader` | Returns an error satisfying `errors.Is` for the read error; no panic | P1 |  |
| RR-12 | DecodeStruct | Error Path | JSON `null` into a pointer target | `var x *T`; bodies `null` and `{}` | `null` panics with a non-empty string message; `{}` decodes with `x` non-nil | P1 |  |
| RR-13 | DecodeStruct | Error Path | Protobuf content type into a type that is neither `proto.Message` nor `ProtoEncoder` | Plain struct | Panics with a non-empty string message | P1 |  |
| RR-14 | DecodeStruct | Happy Path | Ordinary JSON and XML bodies, and malformed JSON | Small valid bodies; `{"a":` | Valid bodies decode with no error; malformed JSON returns an error and does not panic | P1 |  |
| RR-15 | DecodeStruct | Error Path | `data` is nil | `DecodeStruct[T](r, nil)` | Panics with a non-empty string message | P2 |  |
