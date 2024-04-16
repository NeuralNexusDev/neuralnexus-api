package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/google/uuid"
)

// Middleware - Middleware type
type Middleware func(http.Handler) http.Handler

// key - Type for context keys
type key int

const (
	// SessionKey - Key for session in context
	SessionKey key = iota
)

// CreateStack - Create a stack of middlewares
func CreateStack(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// WrappedWriter - Wrapper for http.ResponseWriter
type WrappedWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader - Write the header
func (w *WrappedWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// RequestLoggerMiddleware - Log all requests
func RequestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &WrappedWriter{w, http.StatusOK}
		next.ServeHTTP(wrapped, r)

		cfConnectingIP := r.Header.Get("CF-Connecting-IP")
		forwardedFor := r.Header.Get("X-Forwarded-For")
		if cfConnectingIP != "" {
			r.RemoteAddr = cfConnectingIP
		} else if forwardedFor != "" {
			r.RemoteAddr = forwardedFor
		}

		log.Printf("%s %d %s %s %s", r.RemoteAddr, wrapped.statusCode, r.Method, r.URL.Path, time.Since(start))
	})
}

// AuthMiddleware - Authenticate requests
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			responses.SendAndEncodeUnauthorized(w, r, "")
			return
		}

		authStrings := strings.Split(authHeader, "Bearer ")
		if len(authStrings) != 2 {
			responses.SendAndEncodeUnauthorized(w, r, "")
			return
		}

		sessionID, err := uuid.Parse(authStrings[1])
		if err != nil {
			responses.SendAndEncodeUnauthorized(w, r, "")
			return
		}

		session := auth.GetSession(sessionID)
		if !session.Success {
			responses.SendAndEncodeUnauthorized(w, r, "")
			return
		}

		if !session.Data.IsValid() {
			responses.SendAndEncodeUnauthorized(w, r, "")
			auth.DeleteSession(session.Data.ID)
			return
		}

		session.Data.LastUsedAt = time.Now().Unix()
		auth.UpdateSession(session.Data)

		ctx := r.Context()
		ctx = context.WithValue(ctx, SessionKey, session.Data)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
