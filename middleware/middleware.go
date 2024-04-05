package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/google/uuid"
)

// WrappedWriter - Wrapper for http.ResponseWriter
type WrappedWriter struct {
	http.ResponseWriter
	statusCode int
}

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

// RequestLoggerMiddleware - Log all requests
func RequestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := WrappedWriter{w, http.StatusOK}
		next.ServeHTTP(wrapped, r)
		log.Printf("%s %d %s %s %s", r.RemoteAddr, wrapped.statusCode, r.Method, r.URL.Path, time.Since(start))
	})
}

// AuthMiddleware - Authenticate requests
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		authStrings := strings.Split(authHeader, "Bearer ")
		if len(authStrings) != 2 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		sessionID, err := uuid.Parse(authStrings[1])
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		session := auth.GetSession(sessionID)
		if !session.Success {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if !session.Data.IsValid() {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, SessionKey, session.Data)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
