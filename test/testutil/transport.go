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

// FakeRoundTripper is an http.RoundTripper that answers every request with Resp, or with Err when Err is set.
type FakeRoundTripper struct {
	Resp    *http.Response
	Err     error
	LastReq *http.Request
}

// RoundTrip records req in LastReq, then returns Err if set and Resp otherwise.
func (f *FakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	f.LastReq = req
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Resp, nil
}
