package mw

import (
	"context"
	"fmt"
	"log"
	"net"
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
	// RemoteAddrKey - Key for remote address in context
	RemoteAddrKey
)

const (
	AuthHeader           = "Authorization"
	SessionCookieName    = "session"
	XRequestIDHeader     = "X-Request-ID"
	XForwardedForHeader  = "X-Forwarded-For"
	CFConnectingIPHeader = "CF-Connecting-IP"

	RetryAfter = 60
)

func LogRequest(ctx context.Context, message ...string) {
	remoteAddr := ctx.Value(RemoteAddrKey).(string)
	requestId := ctx.Value(RequestIDKey).(int)
	session, ok := ctx.Value(SessionKey).(*auth.Session)
	if !ok {
		session = nil
	}
	userId := "N/A"
	if session != nil {
		userId = session.UserID
	}
	log.Printf("%d %s %s %s",
		requestId,
		userId,
		remoteAddr,
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

// IPMiddleware - Update the remote address based on headers
func IPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfConnectingIP := r.Header.Get(CFConnectingIPHeader)
		forwardedFor := r.Header.Get(XForwardedForHeader)
		if cfConnectingIP != "" {
			r.RemoteAddr = cfConnectingIP
		} else if forwardedFor != "" {
			// X-Forwarded-For can be a comma-separated proxy chain
			// ("client, proxy1, proxy2, ..."); per RFC 7239/XFF convention
			// the leftmost entry is the original client, so use only that -
			// otherwise a multi-hop value falls through to RateLimitMiddleware
			// and collides every hop into one rate-limit key. If that leftmost
			// entry is empty/whitespace (a malformed or leading-comma header),
			// leave RemoteAddr as the real socket-level address rather than
			// overwriting it with "", which would collapse unrelated clients
			// onto the same empty-string rate-limit bucket.
			clientIP, _, _ := strings.Cut(forwardedFor, ",")
			if clientIP = strings.TrimSpace(clientIP); clientIP != "" {
				r.RemoteAddr = clientIP
			}
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, RemoteAddrKey, r.RemoteAddr)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// SessionMiddleware - Read the session from the request, either from a
// "Bearer <token>" Authorization header (bots/integrations) or, failing
// that, the session cookie (browser frontend, which can't attach a custom
// header of its own to a cookie it can't read - see SessionCookieName).
// The header takes priority when both are present.
func SessionMiddleware(service auth.SessionService) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var token string
			hasCredential := false
			if authHeader := r.Header.Get(AuthHeader); authHeader != "" {
				authStrings := strings.Split(authHeader, "Bearer ")
				if len(authStrings) != 2 {
					responses.Unauthorized(w, r, "")
					return
				}
				token = authStrings[1]
				hasCredential = true
			} else if cookie, err := r.Cookie(SessionCookieName); err == nil {
				token = cookie.Value
				hasCredential = true
			}

			if hasCredential {
				session, err := service.ReadJWT(token)
				if err != nil {
					LogRequest(r.Context(), "Error reading JWT:\n\t", err.Error())
					responses.Unauthorized(w, r, "")
					return
				}

				if !session.IsValid() {
					responses.Unauthorized(w, r, "")
					err = service.DeleteSession(session.ID)
					if err != nil {
						LogRequest(r.Context(), "Error deleting session:\n\t", err.Error())
					}
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

// RateLimitMiddleware - Rate limit requests
func RateLimitMiddleware(service auth.RateLimitService, prefix string, sessionLimit int, ipLimit int) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := r.Context().Value(SessionKey).(*auth.Session)
			if ok && session != nil {
				err := service.IncrRateLimit(prefix + ":" + session.UserID)
				if err != nil {
					LogRequest(r.Context(), "Error incrementing rate limit:\n\t", err.Error())
				}
				limit, err := service.GetRateLimit(prefix + ":" + session.UserID)
				if err != nil {
					LogRequest(r.Context(), "Error getting rate limit:\n\t", err.Error())
				}
				if limit > sessionLimit {
					responses.TooManyRequests(w, r, RetryAfter, "You have been rate limited. Please try again later.")
					return
				}
			} else {
				// net.SplitHostPort (not strings.Split on ":") because an
				// IPv6 address contains colons of its own - splitting on the
				// first one truncates it to its first hextet, colliding
				// unrelated IPv6 clients into the same rate-limit bucket.
				// RemoteAddr may also arrive with no port at all (IPMiddleware
				// can set it to a bare IP from a header), so fall back to the
				// raw value when there's nothing to split.
				ip, _, err := net.SplitHostPort(r.RemoteAddr)
				if err != nil {
					ip = r.RemoteAddr
				}
				err = service.IncrRateLimit(prefix + ":" + ip)
				if err != nil {
					LogRequest(r.Context(), "Error incrementing rate limit:\n\t", err.Error())
					return
				}
				limit, err := service.GetRateLimit(prefix + ":" + ip)
				if err != nil {
					LogRequest(r.Context(), "Error getting rate limit:\n\t", err.Error())
					return
				}
				if limit > ipLimit {
					responses.TooManyRequests(w, r, RetryAfter, "You have been rate limited. Please try again later.")
					return
				}
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

// RequestLoggerMiddleware - Log all requests
func RequestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &WrappedWriter{w, http.StatusOK}

		next.ServeHTTP(wrapped, r)

		LogRequest(r.Context(), fmt.Sprintf("%d %s %s %s",
			wrapped.statusCode,
			r.Method,
			r.URL.Path,
			time.Since(start),
		))
	})
}

// Auth - Authenticate requests
func Auth(service auth.SessionService) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := r.Context().Value(SessionKey).(*auth.Session)
			if !ok || session == nil {
				responses.Unauthorized(w, r, "")
				return
			}

			if !session.IsValid() {
				responses.Unauthorized(w, r, "")
				err := service.DeleteSession(session.ID)
				if err != nil {
					LogRequest(r.Context(), "Error deleting session:\n\t", err.Error())
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SelfUserID rewrites the "user_id" path value to the caller's own session
// user ID, so a literal "me" route (e.g. "GET /users/me" registered
// alongside "GET /users/{user_id}" - the literal takes precedence per
// net/http.ServeMux's matching rules) can reuse a {user_id}-shaped handler
// unchanged. Must run behind Auth (or another middleware that guarantees a
// session), but checks for one anyway rather than assuming it.
func SelfUserID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := r.Context().Value(SessionKey).(*auth.Session)
		if !ok || session == nil {
			responses.Unauthorized(w, r, "")
			return
		}
		r.SetPathValue("user_id", session.UserID)
		next.ServeHTTP(w, r)
	})
}
