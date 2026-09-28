# Test plan: projects
Mode: FRESH

## projects.go

| ID | Function | Scenario Type | Scenario | Precondition | Expected Result | Priority |
|----|----------|---------------|----------|---------------|------------------|----------|
| PJ-01 | getReleases | Happy Path | Token set, GitHub responds 200 with a valid JSON release array | `githubToken` non-empty; `http.DefaultTransport` swapped for a RoundTripper returning a 200 response whose body is a valid `[]Release` JSON array | Returns the decoded `[]Release` slice matching the fixture, `nil` error | P1 |
| PJ-02 | getReleases | Error Path | `GITHUB_TOKEN` not set | `githubToken` reassigned to `""` | Returns `nil` and an error with message `"GITHUB_TOKEN is not set"`, before any HTTP call is attempted | P1 |
| PJ-03 | getReleases | Error Path | `http.NewRequest` fails because `group`/`project` contain an invalid URL control character (e.g. `"a\nb"`) | `githubToken` non-empty; no transport swap needed since the request is never sent | Returns `nil` and a non-nil error (URL parse/`net/url` error), and no network call occurs | P2 |
| PJ-04 | getReleases | Error Path | `client.Do` fails at the transport level | `githubToken` non-empty; `http.DefaultTransport` swapped for a RoundTripper returning a canned error | Returns `nil` and an error equal to (wrapping) the canned transport error | P2 |
| PJ-05 | getReleases | Error Path | GitHub responds 200 but body is not valid JSON | `githubToken` non-empty; `http.DefaultTransport` swapped for a RoundTripper returning a 200 response with body `"not json"` | Returns `nil` and a non-nil JSON decode error | P2 |
| PJ-06 | getReleases | Edge Case | GitHub responds 200 with an empty JSON array `[]` | `githubToken` non-empty; `http.DefaultTransport` swapped for a RoundTripper returning a 200 response with body `"[]"` | Returns a non-nil, zero-length `[]Release` slice and `nil` error | P3 |
| PJ-07 | ConvertToFMLFormat | Happy Path | Single release with a well-formed `"vX.Y"`-style tag | `releases = []Release{{TagName: "v1.20.1", URL: "https://example/1"}}` | Returned map has `homepage` equal to input URL, `promos["<version>-latest"]` and `["<version>-recommended"]` equal to `releases[0].URL` for every entry in `forgeModVersions`, and every `forgeModVersions` key maps to `map[string]string{"1.20.1": "https://example/1"}` | P1 |
| PJ-08 | ConvertToFMLFormat | Edge Case | Multiple releases with distinct tags | `releases` holds two releases, e.g. `v1.20.1` and `v1.19.4`, each with its own URL | The combined `releaseMap` (containing both version→URL entries) is assigned identically under every `forgeModVersions` key in the result (the function does not filter per compatible version), and `promos` entries all point at `releases[0].URL` | P2 |
| PJ-09 | ConvertToFMLFormat | Error Path | A release's `TagName` contains no `"v"` at all (e.g. `"1.20.1"`) | `releases = []Release{{TagName: "1.20.1", URL: "https://example/1"}}` | Current behavior: the function panics (`strings.Split(...)[1]`: index out of range) — see SOURCE BUGS in report | P1 |
| PJ-10 | ConvertToFMLFormat | Edge Case | `releases` is an empty slice | `releases = []Release{}` | Current behavior: the function panics (`releases[0]` on empty slice: index out of range) — see SOURCE BUGS in report | P1 |
| PJ-11 | GetReleasesHandler | Happy Path | Request with no `format` query param, upstream succeeds | Path values `group`/`project` set; `githubToken` non-empty; `http.DefaultTransport` swapped to return a valid release array | 200 status, `Content-Type: application/json`, body is the JSON-encoded `[]Release` from `getReleases` | P1 |
| PJ-12 | GetReleasesHandler | Happy Path | Request with `format=fml`, upstream succeeds with one well-formed release | Path values `group`/`project` set; `?format=fml`; `githubToken` non-empty; transport swapped to return one release with a `"vX.Y"` tag | 200 status, `Content-Type: application/json`, body is the JSON-encoded map from `ConvertToFMLFormat` (contains `homepage`, `promos`, and per-version keys) | P1 |
| PJ-13 | GetReleasesHandler | Error Path | `getReleases` returns an error | `githubToken` reassigned to `""` so `getReleases` fails fast | 500 status, body equals the error message text (`"GITHUB_TOKEN is not set\n"` as written by `http.Error`) | P2 |

## Function inventory self-check
- [x] getReleases — covered by PJ-01, PJ-02, PJ-03, PJ-04, PJ-05, PJ-06
- [x] ConvertToFMLFormat — covered by PJ-07, PJ-08, PJ-09, PJ-10
- [x] GetReleasesHandler — covered by PJ-11, PJ-12, PJ-13
