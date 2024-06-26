package mw

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	sess "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
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

		cfConnectingIP := r.Header.Get("CF-Connecting-IP")
		forwardedFor := r.Header.Get("X-Forwarded-For")
		if cfConnectingIP != "" {
			r.RemoteAddr = cfConnectingIP
		} else if forwardedFor != "" {
			r.RemoteAddr = forwardedFor
		}

		next.ServeHTTP(wrapped, r)

		log.Printf("%s %d %s %s %s", r.RemoteAddr, wrapped.statusCode, r.Method, r.URL.Path, time.Since(start))
	})
}

// Auth - Authenticate requests
func Auth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		db := database.GetDB("neuralnexus")
		rdb := database.GetRedis()
		sessService := sess.NewSessionService(sess.NewSessionStore(db, rdb))

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			responses.Unauthorized(w, r, "")
			return
		}

		authStrings := strings.Split(authHeader, "Bearer ")
		if len(authStrings) != 2 {
			responses.Unauthorized(w, r, "")
			return
		}

		session, err := sessService.GetSession(authStrings[1])
		if err != nil {
			log.Println("Error getting session:\n\t", err)
			responses.Unauthorized(w, r, "")
			return
		}

		if !session.IsValid() {
			responses.Unauthorized(w, r, "")
			sessService.DeleteSession(session.ID)
			return
		}

		session.LastUsedAt = time.Now().Unix()
		sessService.UpdateSession(session)

		ctx := r.Context()
		ctx = context.WithValue(ctx, SessionKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
