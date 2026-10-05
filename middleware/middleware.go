package mw

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"golang.org/x/crypto/ed25519"
)

const (
	logErrorDeletingSession       = "Error deleting session:\n\t"
	logErrorReadingJWTFromCookie  = "Error reading JWT from cookie:\n\t"
	logErrorIncrementingRateLimit = "Error incrementing rate limit:\n\t"
	logErrorGettingRateLimit      = "Error getting rate limit:\n\t"
)

const (
	msgRateLimited      = "You have been rate limited. Please try again later."
	msgInvalidSignature = "Invalid signature"
	msgInternalError    = "An internal server error occurred."
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
	XSignatureEd25519    = "X-Signature-Ed25519"
	XSignatureTimestamp  = "X-Signature-Timestamp"

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
	started    bool
}

// WriteHeader - Write the header
func (w *WrappedWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.started = true
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write - Write the body
func (w *WrappedWriter) Write(b []byte) (int, error) {
	w.started = true
	return w.ResponseWriter.Write(b)
}

// Unwrap - Let http.ResponseController reach the underlying writer
func (w *WrappedWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// IPMiddleware - Update the remote address based on headers
func IPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfConnectingIP := r.Header.Get(CFConnectingIPHeader)
		forwardedFor := r.Header.Get(XForwardedForHeader)
		if cfConnectingIP != "" {
			r.RemoteAddr = cfConnectingIP
		} else if forwardedFor != "" {
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

// SessionMiddleware - Read the session from the request
func SessionMiddleware(service auth.SessionService) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authHeader := r.Header.Get(AuthHeader); authHeader != "" {
				authStrings := strings.Split(authHeader, "Bearer ")
				if len(authStrings) != 2 {
					responses.Unauthorized(w, r, "")
					return
				}
				session, err := service.ReadJWT(authStrings[1])
				if err != nil {
					LogRequest(r.Context(), "Error reading JWT:\n\t", err.Error())
					responses.Unauthorized(w, r, "")
					return
				}

				if !session.IsValid() {
					responses.Unauthorized(w, r, "")
					if delErr := service.DeleteSession(session.ID); delErr != nil {
						LogRequest(r.Context(), logErrorDeletingSession, delErr.Error())
					}
					return
				}

				ctx := context.WithValue(r.Context(), SessionKey, session)
				r = r.WithContext(ctx)
			} else if cookie, err := r.Cookie(SessionCookieName); err == nil {
				session, jwtErr := service.ReadJWT(cookie.Value)
				if jwtErr != nil {
					LogRequest(r.Context(), logErrorReadingJWTFromCookie, jwtErr.Error())
				} else if !session.IsValid() {
					if delErr := service.DeleteSession(session.ID); delErr != nil {
						LogRequest(r.Context(), logErrorDeletingSession, delErr.Error())
					}
				} else {
					ctx := context.WithValue(r.Context(), SessionKey, session)
					r = r.WithContext(ctx)
				}
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
					LogRequest(r.Context(), logErrorIncrementingRateLimit, err.Error())
					next.ServeHTTP(w, r)
					return
				}
				limit, err := service.GetRateLimit(prefix + ":" + session.UserID)
				if err != nil {
					LogRequest(r.Context(), logErrorGettingRateLimit, err.Error())
					next.ServeHTTP(w, r)
					return
				}
				if limit > sessionLimit {
					responses.TooManyRequests(w, r, RetryAfter, msgRateLimited)
					return
				}
			} else {
				ip, _, err := net.SplitHostPort(r.RemoteAddr)
				if err != nil {
					ip = r.RemoteAddr
				}
				err = service.IncrRateLimit(prefix + ":" + ip)
				if err != nil {
					LogRequest(r.Context(), logErrorIncrementingRateLimit, err.Error())
					next.ServeHTTP(w, r)
					return
				}
				limit, err := service.GetRateLimit(prefix + ":" + ip)
				if err != nil {
					LogRequest(r.Context(), logErrorGettingRateLimit, err.Error())
					next.ServeHTTP(w, r)
					return
				}
				if limit > ipLimit {
					responses.TooManyRequests(w, r, RetryAfter, msgRateLimited)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RecoveryMiddleware - Log a panic from a handler and answer 500, unless the handler has already started the response
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tracked := &WrappedWriter{ResponseWriter: w, statusCode: http.StatusOK}
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			if recovered == http.ErrAbortHandler {
				panic(recovered)
			}
			log.Printf("panic serving %s %s: request_id=%v %v\n%s", r.Method, r.URL.Path, r.Context().Value(RequestIDKey), recovered, debug.Stack())
			if !tracked.started {
				responses.InternalServerError(tracked, r, msgInternalError)
			}
		}()
		next.ServeHTTP(tracked, r)
	})
}

// NoSniffMiddleware - Stop browsers from guessing a response's content type
func NoSniffMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
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
		wrapped := &WrappedWriter{ResponseWriter: w, statusCode: http.StatusOK}

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
					LogRequest(r.Context(), logErrorDeletingSession, err.Error())
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SelfUserID rewrites the "user_id" path value to the caller's own session
// user ID, so a {user_id}-shaped handler can be reused for a "me" route.
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

// VerifyEd25519Middleware verifies the body's signature, generally used by Discord
// https://discord.com/developers/docs/interactions/receiving-and-responding#security-and-authorization
func VerifyEd25519Middleware(publicKey ed25519.PublicKey) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			signatureStr := r.Header.Get(XSignatureEd25519)
			signature, err := hex.DecodeString(signatureStr)
			timestamp := r.Header.Get(XSignatureTimestamp)
			if signatureStr == "" || timestamp == "" {
				if err != nil {
					LogRequest(r.Context(), "Error decoding signature:\n\t", err.Error())
				}
				responses.Unauthorized(w, r, msgInvalidSignature)
				return
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				LogRequest(r.Context(), "Error reading body:\n\t", err.Error())
				responses.Unauthorized(w, r, msgInvalidSignature)
				return
			}

			var buffer bytes.Buffer
			buffer.WriteString(timestamp)
			buffer.Write(bodyBytes)
			if !ed25519.Verify(publicKey, buffer.Bytes(), signature) {
				responses.Unauthorized(w, r, msgInvalidSignature)
				return
			}

			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			next.ServeHTTP(w, r)
		})
	}
}
