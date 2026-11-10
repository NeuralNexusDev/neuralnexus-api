# Test plan: responses

## response.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RR-01 | TooManyRequests | Edge Case | Retry-After is sent with a retry delay of 60 seconds | retry delay of 60 seconds | Status 429; the `Retry-After` header is an HTTP date in GMT about 60 seconds after the call | P2 |  |
| RR-06 | DecodeStruct | Edge Case | Protobuf body with no declared length | a protobuf string message sent with content type `application/x-protobuf` | Decodes with no error and the value intact | P1 |  |
| RR-08 | DecodeStruct | Error Path | A declared length of about a terabyte, for JSON, XML and protobuf | Small body, huge declared length | Panics with a non-empty string message in each case | P1 |  |
| RR-09 | DecodeStruct | Error Path | No declared length and a body one byte over the 1 MiB limit | no declared length | Panics with a non-empty string message | P1 |  |
| RR-10 | DecodeStruct | Edge Case | Body of exactly 1 MiB | JSON string value sized to the limit, no declared length | Decodes with no error | P2 |  |
| RR-11 | DecodeStruct | Error Path | The body read fails | The body fails on the first read | Returns an error that wraps the read error; no panic | P1 |  |
| RR-12 | DecodeStruct | Error Path | JSON `null` into a pointer target | a nil pointer target; bodies `null` and `{}` | `null` panics with a non-empty string message; `{}` decodes and the pointer is non-nil | P1 |  |
| RR-13 | DecodeStruct | Error Path | Protobuf content type into a type that cannot be decoded as protobuf | Plain struct | Panics with a non-empty string message | P1 |  |
| RR-14 | DecodeStruct | Happy Path | Ordinary JSON and XML bodies, and malformed JSON | Small valid bodies; `{"a":` | Valid bodies decode with no error; malformed JSON returns an error and does not panic | P1 |  |
| RR-15 | DecodeStruct | Error Path | No target to decode into | a nil target | Panics with a non-empty string message | P2 |  |

## problem.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority | Notes |
|----|----------|---------------|----------|---------------|------------------|----------|-------|
| RR-02 | SendProblemStruct | Happy Path | a struct embedding a Problem plus a host and a port is sent with no `Accept` header | NotFound problem with host `"example.com"` and port 25565 | Status 404, `Content-Type: application/problem+json`, and the JSON body carries `status`, `title`, `detail` together with the host and the port | P1 |  |
| RR-03 | SendProblemStruct | Edge Case | `Accept: application/xml` | same struct | `Content-Type: application/problem+xml` and the XML body has a root element named Problem carrying the host and port elements | P2 |  |
| RR-04 | SendProblemStruct | Edge Case | `Accept: application/x-protobuf` | same struct | Status 404, `Content-Type: application/problem+x-protobuf`, and the body decodes as the plain Problem message with its status and detail | P2 |  |
