package testutil

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// FakeResponse returns an HTTP response with the given status and body.
func FakeResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// SwapTransport replaces http.DefaultTransport with rt until the test ends.
func SwapTransport(t *testing.T, rt http.RoundTripper) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = rt
	t.Cleanup(func() {
		http.DefaultTransport = orig
	})
}
