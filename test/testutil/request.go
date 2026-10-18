package testutil

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
)

const (
	// RequestID is the request ID that LogContext and NewRequest set.
	RequestID = 1
	// RemoteAddr is the remote address that LogContext and NewRequest set.
	RemoteAddr = "127.0.0.1"
)

// LogContext returns ctx with the remote address and request ID that reqctx.Log reads.
func LogContext(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, reqctx.RemoteAddrKey, RemoteAddr)
	return context.WithValue(ctx, reqctx.RequestIDKey, RequestID)
}

// NewRequest builds a request whose context carries the keys reqctx.Log reads.
func NewRequest(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	return r.WithContext(LogContext(r.Context()))
}
