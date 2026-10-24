package testutil

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/goccy/go-json"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// RequireRedirect fails unless w is a 303 to wantTarget and returns the parsed Location.
func RequireRedirect(t *testing.T, w *httptest.ResponseRecorder, wantTarget string) *url.URL {
	t.Helper()
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}
	location := w.Header().Get("Location")
	u, err := url.Parse(location)
	if err != nil {
		t.Fatalf("failed to parse Location %q: %v", location, err)
	}
	target := (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String()
	if target != wantTarget {
		t.Errorf("expected redirect target %q, got %q (full Location %q)", wantTarget, target, location)
	}
	return u
}

// FindCookie returns the response cookie named name, or nil.
func FindCookie(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// ExpectStatus fails unless w has the status want.
func ExpectStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("expected %d, got %d: %s", want, w.Code, w.Body.String())
	}
}

// ExpectDetail fails unless the problem body of w has the detail want.
func ExpectDetail(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var p responses.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to decode problem body %q: %v", w.Body.String(), err)
	}
	if p.Detail != want {
		t.Fatalf("detail = %q, want %q", p.Detail, want)
	}
}

// DecodeJSON decodes body into a generic map and fails the test on invalid JSON.
func DecodeJSON(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("failed to decode JSON body %q: %v", body, err)
	}
	return m
}
