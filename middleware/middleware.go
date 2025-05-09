package mw

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// Middleware - Middleware type
type Middleware func(http.Handler) http.Handler

// key - Type for context keys
type key int

const (
	// SessionKey - Key for session in context
	SessionKey key = iota
	// RequestIDKey - Key for request ID in context
	RequestIDKey
)

const (
	AuthHeader           = "Authorization"
	XRequestIDHeader     = "X-Request-ID"
	XForwardedForHeader  = "X-Forwarded-For"
	CFConnectingIPHeader = "CF-Connecting-IP"
)

func LogRequest(r *http.Request, message ...string) {
	requestId := r.Context().Value(RequestIDKey).(int)
	session, ok := r.Context().Value(SessionKey).(*auth.Session)
	userId := "N/A"
	if ok && session != nil {
		userId = session.UserID
	}
	log.Printf("%d %s %s %s",
		requestId,
		userId,
		r.RemoteAddr,
		strings.Join(message, " "),
	)
}

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

// SessionMiddleware - Read the session from the request
func SessionMiddleware(service auth.SessionService) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get(AuthHeader)
			if authHeader != "" {
				authStrings := strings.Split(authHeader, "Bearer ")
				if len(authStrings) != 2 {
					responses.Unauthorized(w, r, "")
					return
				}

				session, err := service.GetSession(authStrings[1])
				if err != nil {
					LogRequest(r, "Error getting session:\n\t", err.Error())
					responses.Unauthorized(w, r, "")
					return
				}

				if !session.IsValid() {
					responses.Unauthorized(w, r, "")
					err = service.DeleteSession(session.ID)
					if err != nil {
						LogRequest(r, "Error deleting session:\n\t", err.Error())
					}
					return
				}

				session.LastUsedAt = time.Now().Unix()
				err = service.UpdateSession(session)
				if err != nil {
					responses.InternalServerError(w, r, "Error updating session")
					LogRequest(r, "Error updating session:\n\t", err.Error())
					return
				}

				ctx := r.Context()
				ctx = context.WithValue(ctx, SessionKey, session)
				r = r.WithContext(ctx)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequestIDMiddleware - Set the request ID in the context
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestIdStr := r.Header.Get(XRequestIDHeader)
		var requestId int
		if requestIdStr == "" {
			requestId = int(time.Now().UnixNano())
			r.Header.Set(XRequestIDHeader, strconv.Itoa(requestId))
		} else {
			requestId, _ = strconv.Atoi(requestIdStr)
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, RequestIDKey, requestId)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// IPMiddleware - Update the remote address based on headers
func IPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfConnectingIP := r.Header.Get(CFConnectingIPHeader)
		forwardedFor := r.Header.Get(XForwardedForHeader)
		if cfConnectingIP != "" {
			r.RemoteAddr = cfConnectingIP
		} else if forwardedFor != "" {
			r.RemoteAddr = forwardedFor
		}

		next.ServeHTTP(w, r)
	})
}

// RequestLoggerMiddleware - Log all requests
func RequestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &WrappedWriter{w, http.StatusOK}

		next.ServeHTTP(wrapped, r)

		LogRequest(r, fmt.Sprintf("%d %s %s %s",
			wrapped.statusCode,
			r.Method,
			r.URL.Path,
			time.Since(start),
		))
	})
}

// Auth - Authenticate requests
func Auth(service auth.SessionService, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get(AuthHeader)
		if authHeader == "" {
			responses.Unauthorized(w, r, "")
			return
		}

		//authStrings := strings.Split(authHeader, "Bearer ")
		//if len(authStrings) != 2 {
		//	responses.Unauthorized(w, r, "")
		//	return
		//}
		//
		//session, err := service.GetSession(authStrings[1])
		//if err != nil {
		//	log.Println("Error getting session:\n\t", err)
		//	responses.Unauthorized(w, r, "")
		//	return
		//}
		//
		//if !session.IsValid() {
		//	responses.Unauthorized(w, r, "")
		//	service.DeleteSession(session.ID)
		//	return
		//}
		//
		//session.LastUsedAt = time.Now().Unix()
		//service.UpdateSession(session)
		//
		//ctx := r.Context()
		//ctx = context.WithValue(ctx, SessionKey, session)
		//next.ServeHTTP(w, r.WithContext(ctx))

		next.ServeHTTP(w, r)
	}
}
