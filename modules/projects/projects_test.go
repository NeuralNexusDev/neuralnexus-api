package projects

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/goccy/go-json"
)

// -------------- Shared helpers --------------

// fakeRoundTripper is a test-only http.RoundTripper that returns a canned
// response or a canned error, without making any real network call. It is
// swapped in for http.DefaultTransport (the getReleases seam: it builds a
// plain &http.Client{} with no Transport set, which falls back to
// http.DefaultTransport at call time).
type fakeRoundTripper struct {
	resp    *http.Response
	err     error
	lastReq *http.Request
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

// fakeResponse builds a minimal *http.Response suitable for a RoundTripper
// to return.
func fakeResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// swapTransport replaces the package-level http.DefaultTransport for the
// duration of a subtest and restores the original afterward.
func swapTransport(t *testing.T, rt http.RoundTripper) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = rt
	t.Cleanup(func() {
		http.DefaultTransport = orig
	})
}

// swapGithubToken temporarily reassigns the package-level githubToken
// variable for the duration of a subtest and restores it afterward. This is
// used instead of t.Setenv because githubToken is read once at package-init
// time from the environment, not per-call.
func swapGithubToken(t *testing.T, val string) {
	t.Helper()
	orig := githubToken
	githubToken = val
	t.Cleanup(func() {
		githubToken = orig
	})
}

// -------------- getReleases --------------

func TestGetReleases(t *testing.T) {
	t.Run("PJ-01_HappyPath_DecodesReleases", func(t *testing.T) {
		swapGithubToken(t, "test-token")
		rt := &fakeRoundTripper{resp: fakeResponse(http.StatusOK, `[{"tag_name":"v1.20.1","html_url":"https://example/1"}]`)}
		swapTransport(t, rt)

		releases, err := getReleases("group", "project")
		if err != nil {
			t.Fatalf("getReleases() error = %v, want nil", err)
		}
		want := []Release{{TagName: "v1.20.1", URL: "https://example/1"}}
		if !reflect.DeepEqual(releases, want) {
			t.Errorf("getReleases() = %+v, want %+v", releases, want)
		}

		if rt.lastReq == nil {
			t.Fatal("expected an outbound request to be built, got none")
		}
		if got := rt.lastReq.URL.String(); got != "https://api.github.com/repos/group/project/releases" {
			t.Errorf("request URL = %q, want %q", got, "https://api.github.com/repos/group/project/releases")
		}
		if got := rt.lastReq.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
		}
		if got := rt.lastReq.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept header = %q, want %q", got, "application/vnd.github+json")
		}
	})

	t.Run("PJ-02_ErrorPath_TokenNotSet", func(t *testing.T) {
		swapGithubToken(t, "")

		releases, err := getReleases("group", "project")
		if err == nil {
			t.Fatal("getReleases() error = nil, want non-nil")
		}
		if err.Error() != "GITHUB_TOKEN is not set" {
			t.Errorf("getReleases() error = %q, want %q", err.Error(), "GITHUB_TOKEN is not set")
		}
		if releases != nil {
			t.Errorf("getReleases() releases = %+v, want nil", releases)
		}
	})

	t.Run("PJ-03_ErrorPath_InvalidRequestURL", func(t *testing.T) {
		swapGithubToken(t, "test-token")
		// A newline in the path component is an invalid URL control
		// character, so http.NewRequest fails before any network call is
		// attempted (no transport swap needed).

		releases, err := getReleases("a\nb", "project")
		if err == nil {
			t.Fatal("getReleases() error = nil, want non-nil")
		}
		if releases != nil {
			t.Errorf("getReleases() releases = %+v, want nil", releases)
		}
	})

	t.Run("PJ-04_ErrorPath_TransportError", func(t *testing.T) {
		swapGithubToken(t, "test-token")
		wantErr := errors.New("simulated transport failure")
		swapTransport(t, &fakeRoundTripper{err: wantErr})

		releases, err := getReleases("group", "project")
		if err == nil {
			t.Fatal("getReleases() error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), wantErr.Error()) {
			t.Errorf("getReleases() error = %q, want it to contain %q", err.Error(), wantErr.Error())
		}
		if releases != nil {
			t.Errorf("getReleases() releases = %+v, want nil", releases)
		}
	})

	t.Run("PJ-05_ErrorPath_InvalidJSONBody", func(t *testing.T) {
		swapGithubToken(t, "test-token")
		swapTransport(t, &fakeRoundTripper{resp: fakeResponse(http.StatusOK, "not json")})

		releases, err := getReleases("group", "project")
		if err == nil {
			t.Fatal("getReleases() error = nil, want non-nil (decode error)")
		}
		if releases != nil {
			t.Errorf("getReleases() releases = %+v, want nil", releases)
		}
	})

	t.Run("PJ-06_EdgeCase_EmptyReleaseArray", func(t *testing.T) {
		swapGithubToken(t, "test-token")
		swapTransport(t, &fakeRoundTripper{resp: fakeResponse(http.StatusOK, "[]")})

		releases, err := getReleases("group", "project")
		if err != nil {
			t.Fatalf("getReleases() error = %v, want nil", err)
		}
		if len(releases) != 0 {
			t.Errorf("getReleases() = %+v, want empty slice", releases)
		}
	})
}

// -------------- ConvertToFMLFormat --------------

func TestConvertToFMLFormat(t *testing.T) {
	t.Run("PJ-07_HappyPath_SingleRelease", func(t *testing.T) {
		releases := []Release{{TagName: "v1.20.1", URL: "https://example/1"}}
		result := ConvertToFMLFormat("https://github.com/g/p/releases", releases)

		if got := result["homepage"]; got != "https://github.com/g/p/releases" {
			t.Errorf("homepage = %v, want %v", got, "https://github.com/g/p/releases")
		}

		promos, ok := result["promos"].(map[string]string)
		if !ok {
			t.Fatalf("promos has type %T, want map[string]string", result["promos"])
		}
		wantReleaseMap := map[string]string{"1.20.1": "https://example/1"}
		for _, version := range forgeModVersions {
			if got := promos[version+"-latest"]; got != releases[0].URL {
				t.Errorf("promos[%q] = %q, want %q", version+"-latest", got, releases[0].URL)
			}
			if got := promos[version+"-recommended"]; got != releases[0].URL {
				t.Errorf("promos[%q] = %q, want %q", version+"-recommended", got, releases[0].URL)
			}
			versionMap, ok := result[version].(map[string]string)
			if !ok {
				t.Fatalf("result[%q] has type %T, want map[string]string", version, result[version])
			}
			if !reflect.DeepEqual(versionMap, wantReleaseMap) {
				t.Errorf("result[%q] = %+v, want %+v", version, versionMap, wantReleaseMap)
			}
		}
	})

	t.Run("PJ-08_EdgeCase_MultipleReleasesCombinedMap", func(t *testing.T) {
		releases := []Release{
			{TagName: "v1.20.1", URL: "https://example/A"},
			{TagName: "v1.19.4", URL: "https://example/B"},
		}
		result := ConvertToFMLFormat("https://github.com/g/p/releases", releases)

		wantCombined := map[string]string{
			"1.20.1": "https://example/A",
			"1.19.4": "https://example/B",
		}
		for _, version := range forgeModVersions {
			versionMap, ok := result[version].(map[string]string)
			if !ok {
				t.Fatalf("result[%q] has type %T, want map[string]string", version, result[version])
			}
			// The function does not filter releaseMap per Forge version: the
			// full combined map is assigned identically under every key.
			if !reflect.DeepEqual(versionMap, wantCombined) {
				t.Errorf("result[%q] = %+v, want combined %+v", version, versionMap, wantCombined)
			}
		}

		promos, ok := result["promos"].(map[string]string)
		if !ok {
			t.Fatalf("promos has type %T, want map[string]string", result["promos"])
		}
		// promos always point at releases[0], the first element of the slice.
		for _, version := range forgeModVersions {
			if got := promos[version+"-latest"]; got != releases[0].URL {
				t.Errorf("promos[%q] = %q, want %q", version+"-latest", got, releases[0].URL)
			}
		}
	})

	t.Run("PJ-09_ErrorPath_TagWithoutV_Panics", func(t *testing.T) {
		// SOURCE BUG: a release TagName with no "v" causes
		// strings.Split(tag, "v")[1] to index out of range. This test
		// documents current behavior; it does not fix the source.
		releases := []Release{{TagName: "1.20.1", URL: "https://example/1"}}

		panicked := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = true
				}
			}()
			ConvertToFMLFormat("https://github.com/g/p/releases", releases)
		}()

		if !panicked {
			t.Fatal("expected ConvertToFMLFormat to panic on a TagName without \"v\" (current behavior); got no panic")
		}
	})

	t.Run("PJ-10_EdgeCase_EmptyReleases_Panics", func(t *testing.T) {
		// SOURCE BUG: an empty releases slice causes releases[0].URL (in the
		// promos loop) to index out of range. This test documents current
		// behavior; it does not fix the source.
		panicked := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = true
				}
			}()
			ConvertToFMLFormat("https://github.com/g/p/releases", []Release{})
		}()

		if !panicked {
			t.Fatal("expected ConvertToFMLFormat to panic on an empty releases slice (current behavior); got no panic")
		}
	})
}

// -------------- GetReleasesHandler --------------

func TestGetReleasesHandler(t *testing.T) {
	t.Run("PJ-11_HappyPath_DefaultFormat", func(t *testing.T) {
		swapGithubToken(t, "test-token")
		swapTransport(t, &fakeRoundTripper{resp: fakeResponse(http.StatusOK, `[{"tag_name":"v1.20.1","html_url":"https://example/1"}]`)})

		req := httptest.NewRequest(http.MethodGet, "/projects/releases/group/project", nil)
		req.SetPathValue("group", "group")
		req.SetPathValue("project", "project")
		rec := httptest.NewRecorder()

		GetReleasesHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}
		var got []Release
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		want := []Release{{TagName: "v1.20.1", URL: "https://example/1"}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("body = %+v, want %+v", got, want)
		}
	})

	t.Run("PJ-12_HappyPath_FMLFormat", func(t *testing.T) {
		swapGithubToken(t, "test-token")
		swapTransport(t, &fakeRoundTripper{resp: fakeResponse(http.StatusOK, `[{"tag_name":"v1.20.1","html_url":"https://example/1"}]`)})

		req := httptest.NewRequest(http.MethodGet, "/projects/releases/group/project?format=fml", nil)
		req.SetPathValue("group", "group")
		req.SetPathValue("project", "project")
		rec := httptest.NewRecorder()

		GetReleasesHandler(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want %q", got, "application/json")
		}
		var body map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if got := body["homepage"]; got != "https://github.com/group/project/releases" {
			t.Errorf("homepage = %v, want %v", got, "https://github.com/group/project/releases")
		}
		promos, ok := body["promos"].(map[string]interface{})
		if !ok {
			t.Fatalf("promos has type %T, want map[string]interface{}", body["promos"])
		}
		if got := promos["1.20.1-latest"]; got != "https://example/1" {
			t.Errorf("promos[1.20.1-latest] = %v, want %v", got, "https://example/1")
		}
	})

	t.Run("PJ-13_ErrorPath_UpstreamError", func(t *testing.T) {
		swapGithubToken(t, "")

		req := httptest.NewRequest(http.MethodGet, "/projects/releases/group/project", nil)
		req.SetPathValue("group", "group")
		req.SetPathValue("project", "project")
		rec := httptest.NewRecorder()

		GetReleasesHandler(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
		wantBody := "GITHUB_TOKEN is not set\n"
		if got := rec.Body.String(); got != wantBody {
			t.Errorf("body = %q, want %q", got, wantBody)
		}
	})
}
