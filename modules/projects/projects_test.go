package projects

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/goccy/go-json"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

var pjCtx = context.Background()

func testGitHub(token string) config.GitHub {
	return testutil.NewConfig(func(cfg *config.Config) {
		cfg.GitHub = config.GitHub{APIEndpoint: "https://api.github.com", Token: token}
	}).GitHub
}

func TestGetReleases(t *testing.T) {
	t.Run("PJ-01_HappyPath_DecodesReleases", func(t *testing.T) {
		cfg := testGitHub("test-token")
		rt := &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, `[{"tag_name":"v1.20.1","html_url":"https://example/1"}]`)}
		testutil.SwapTransport(t, rt)

		releases, err := getReleases(pjCtx, cfg, "group", "project")
		if err != nil {
			t.Fatalf("getReleases() error = %v, want nil", err)
		}
		want := []Release{{TagName: "v1.20.1", URL: "https://example/1"}}
		if !reflect.DeepEqual(releases, want) {
			t.Errorf("getReleases() = %+v, want %+v", releases, want)
		}

		if rt.LastReq == nil {
			t.Fatal("expected an outbound request to be built, got none")
		}
		if got := rt.LastReq.URL.String(); got != "https://api.github.com/repos/group/project/releases" {
			t.Errorf("request URL = %q, want %q", got, "https://api.github.com/repos/group/project/releases")
		}
		if got := rt.LastReq.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
		}
		if got := rt.LastReq.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept header = %q, want %q", got, "application/vnd.github+json")
		}
	})

	t.Run("PJ-03_ErrorPath_InvalidRequestURL", func(t *testing.T) {
		cfg := testGitHub("test-token")
		cfg.APIEndpoint = "http://bad host"

		releases, err := getReleases(pjCtx, cfg, "group", "project")
		if err == nil {
			t.Fatal("getReleases() error = nil, want non-nil")
		}
		if releases != nil {
			t.Errorf("getReleases() releases = %+v, want nil", releases)
		}
	})

	t.Run("PJ-04_ErrorPath_TransportError", func(t *testing.T) {
		cfg := testGitHub("test-token")
		wantErr := testutil.ErrTransportFailed
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Err: wantErr})

		releases, err := getReleases(pjCtx, cfg, "group", "project")
		if !errors.Is(err, wantErr) {
			t.Fatalf("getReleases() error = %v, want it to wrap %v", err, wantErr)
		}
		if releases != nil {
			t.Errorf("getReleases() releases = %+v, want nil", releases)
		}
	})

	t.Run("PJ-05_ErrorPath_InvalidJSONBody", func(t *testing.T) {
		cfg := testGitHub("test-token")
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, "not json")})

		releases, err := getReleases(pjCtx, cfg, "group", "project")
		if err == nil {
			t.Fatal("getReleases() error = nil, want non-nil (decode error)")
		}
		if releases != nil {
			t.Errorf("getReleases() releases = %+v, want nil", releases)
		}
	})

	t.Run("PJ-06_EdgeCase_EmptyReleaseArray", func(t *testing.T) {
		cfg := testGitHub("test-token")
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, "[]")})

		releases, err := getReleases(pjCtx, cfg, "group", "project")
		if err != nil {
			t.Fatalf("getReleases() error = %v, want nil", err)
		}
		if len(releases) != 0 {
			t.Errorf("getReleases() = %+v, want empty slice", releases)
		}
	})

	t.Run("PJ-16_EdgeCase_CtxReachesOutboundRequest", func(t *testing.T) {
		type ctxKey struct{}
		cfg := testGitHub("test-token")
		rt := &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, "[]")}
		testutil.SwapTransport(t, rt)

		ctx := context.WithValue(pjCtx, ctxKey{}, "marker")
		if _, err := getReleases(ctx, cfg, "group", "project"); err != nil {
			t.Fatalf("getReleases() error = %v, want nil", err)
		}
		if rt.LastReq == nil {
			t.Fatal("expected an outbound GitHub request, got none")
		}
		if got := rt.LastReq.Context().Value(ctxKey{}); got != "marker" {
			t.Errorf("outbound request ctx value = %v, want marker", got)
		}
	})

	logCtx := testutil.LogContext(pjCtx)

	t.Run("PJ-17_NotFoundIsErrReleasesNotFound", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusNotFound, `{"message":"Not Found"}`)})

		releases, err := getReleases(logCtx, testGitHub("test-token"), "group", "project")
		if !errors.Is(err, ErrReleasesNotFound) {
			t.Errorf("getReleases() error = %v, want %v", err, ErrReleasesNotFound)
		}
		if releases != nil {
			t.Errorf("getReleases() releases = %+v, want nil", releases)
		}
	})

	t.Run("PJ-18_ForbiddenAndTooManyAreRateLimited", func(t *testing.T) {
		for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests} {
			testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(status, `{"message":"rate limit"}`)})

			releases, err := getReleases(logCtx, testGitHub("test-token"), "group", "project")
			if !errors.Is(err, ErrGitHubRateLimited) {
				t.Errorf("status %d: error = %v, want %v", status, err, ErrGitHubRateLimited)
			}
			if releases != nil {
				t.Errorf("status %d: releases = %+v, want nil", status, releases)
			}
		}
	})

	t.Run("PJ-19_OtherStatusIsErrGitHubStatus", func(t *testing.T) {
		for _, status := range []int{http.StatusInternalServerError, http.StatusUnauthorized} {
			testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(status, `{"message":"nope"}`)})

			releases, err := getReleases(logCtx, testGitHub("test-token"), "group", "project")
			if !errors.Is(err, ErrGitHubStatus) {
				t.Errorf("status %d: error = %v, want %v", status, err, ErrGitHubStatus)
			}
			if releases != nil {
				t.Errorf("status %d: releases = %+v, want nil", status, releases)
			}
		}
	})

	t.Run("PJ-20_PathIsEscaped", func(t *testing.T) {
		rt := &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, "[]")}
		testutil.SwapTransport(t, rt)

		if _, err := getReleases(logCtx, testGitHub("test-token"), "a/../b", "p q"); err != nil {
			t.Fatalf("getReleases() error = %v, want nil", err)
		}
		if got, want := rt.LastReq.URL.EscapedPath(), "/repos/a%2F..%2Fb/p%20q/releases"; got != want {
			t.Errorf("request path = %q, want %q", got, want)
		}
	})
}

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
			if !reflect.DeepEqual(versionMap, wantCombined) {
				t.Errorf("result[%q] = %+v, want combined %+v", version, versionMap, wantCombined)
			}
		}

		promos, ok := result["promos"].(map[string]string)
		if !ok {
			t.Fatalf("promos has type %T, want map[string]string", result["promos"])
		}
		for _, version := range forgeModVersions {
			if got := promos[version+"-latest"]; got != releases[0].URL {
				t.Errorf("promos[%q] = %q, want %q", version+"-latest", got, releases[0].URL)
			}
		}
	})

	t.Run("PJ-09_EdgeCase_EmptyReleases", func(t *testing.T) {
		result := ConvertToFMLFormat("https://github.com/g/p/releases", []Release{})

		if got := result["homepage"]; got != "https://github.com/g/p/releases" {
			t.Errorf("homepage = %v, want %v", got, "https://github.com/g/p/releases")
		}
		promos, ok := result["promos"].(map[string]string)
		if !ok {
			t.Fatalf("promos has type %T, want map[string]string", result["promos"])
		}
		if len(promos) != 0 {
			t.Errorf("promos = %+v, want empty", promos)
		}
		for _, version := range forgeModVersions {
			versionMap, ok := result[version].(map[string]string)
			if !ok {
				t.Fatalf("result[%q] has type %T, want map[string]string", version, result[version])
			}
			if len(versionMap) != 0 {
				t.Errorf("result[%q] = %+v, want empty", version, versionMap)
			}
		}
	})

	t.Run("PJ-10_ErrorPath_TagMissingVPrefix", func(t *testing.T) {
		releases := []Release{{TagName: "1.20.1", URL: "https://example/1"}}
		result := ConvertToFMLFormat("https://github.com/g/p/releases", releases)

		wantReleaseMap := map[string]string{"1.20.1": "https://example/1"}
		for _, version := range forgeModVersions {
			versionMap, ok := result[version].(map[string]string)
			if !ok {
				t.Fatalf("result[%q] has type %T, want map[string]string", version, result[version])
			}
			if !reflect.DeepEqual(versionMap, wantReleaseMap) {
				t.Errorf("result[%q] = %+v, want %+v", version, versionMap, wantReleaseMap)
			}
		}
	})
}

type failingWriter struct{ header http.Header }

func (f *failingWriter) Header() http.Header       { return f.header }
func (f *failingWriter) WriteHeader(int)           {}
func (f *failingWriter) Write([]byte) (int, error) { return 0, testutil.ErrBodyRead }

func TestGetReleasesHandler(t *testing.T) {
	t.Run("PJ-11_HappyPath_DefaultFormat", func(t *testing.T) {
		cfg := testGitHub("test-token")
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, `[{"tag_name":"v1.20.1","html_url":"https://example/1"}]`)})

		req := testutil.NewRequest(http.MethodGet, "/projects/releases/group/project", nil)
		req.SetPathValue("group", "group")
		req.SetPathValue("project", "project")
		rec := httptest.NewRecorder()

		GetReleasesHandler(cfg)(rec, req)

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
		cfg := testGitHub("test-token")
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, `[{"tag_name":"v1.20.1","html_url":"https://example/1"}]`)})

		req := testutil.NewRequest(http.MethodGet, "/projects/releases/group/project?format=fml", nil)
		req.SetPathValue("group", "group")
		req.SetPathValue("project", "project")
		rec := httptest.NewRecorder()

		GetReleasesHandler(cfg)(rec, req)

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

	t.Run("PJ-14_EdgeCase_FMLFormatEmptyUpstream", func(t *testing.T) {
		cfg := testGitHub("test-token")
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, `[]`)})

		req := testutil.NewRequest(http.MethodGet, "/projects/releases/group/project?format=fml", nil)
		req.SetPathValue("group", "group")
		req.SetPathValue("project", "project")
		rec := httptest.NewRecorder()

		GetReleasesHandler(cfg)(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		var body map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if got := body["homepage"]; got != "https://github.com/group/project/releases" {
			t.Errorf("homepage = %v, want %v", got, "https://github.com/group/project/releases")
		}
		promos, ok := body["promos"].(map[string]interface{})
		if !ok || len(promos) != 0 {
			t.Errorf("promos = %v, want an empty object", body["promos"])
		}
	})

	t.Run("PJ-15_ErrorPath_OtherFailureHidesCause", func(t *testing.T) {
		cfg := testGitHub("test-token")
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Err: testutil.ErrTransportFailed})

		req := testutil.NewRequest(http.MethodGet, "/projects/releases/group/project", nil)
		req.SetPathValue("group", "group")
		req.SetPathValue("project", "project")
		rec := httptest.NewRecorder()

		GetReleasesHandler(cfg)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
		if got, want := rec.Body.String(), msgFailedToGetReleases+"\n"; got != want {
			t.Errorf("body = %q, want %q", got, want)
		}
	})

	for _, tc := range []struct {
		id      string
		name    string
		status  int
		want    int
		message string
	}{
		{"PJ-21", "HandlerNotFound", http.StatusNotFound, http.StatusNotFound, "Releases not found"},
		{"PJ-22", "HandlerRateLimited", http.StatusForbidden, http.StatusServiceUnavailable, "GitHub is not accepting requests right now. Try again later"},
		{"PJ-23", "HandlerBadStatus", http.StatusInternalServerError, http.StatusBadGateway, "GitHub returned an unexpected response"},
	} {
		t.Run(tc.id+"_"+tc.name, func(t *testing.T) {
			testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(tc.status, `{}`)})

			req := testutil.NewRequest(http.MethodGet, "/projects/releases/group/project", nil)
			req.SetPathValue("group", "group")
			req.SetPathValue("project", "project")
			rec := httptest.NewRecorder()

			GetReleasesHandler(testGitHub("test-token"))(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if got, want := rec.Body.String(), tc.message+"\n"; got != want {
				t.Errorf("body = %q, want %q", got, want)
			}
		})
	}

	t.Run("PJ-24_EncodeErrorIsLogged", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, `[{"tag_name":"v1","html_url":"u"}]`)})

		for _, target := range []string{"/projects/releases/group/project", "/projects/releases/group/project?format=fml"} {
			req := testutil.NewRequest(http.MethodGet, target, nil)
			req.SetPathValue("group", "group")
			req.SetPathValue("project", "project")

			GetReleasesHandler(testGitHub("test-token"))(&failingWriter{header: make(http.Header)}, req)
		}
	})
}
