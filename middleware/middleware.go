package mw

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/ratelimit"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
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

const (
	AuthHeader           = "Authorization"
	XRequestIDHeader     = "X-Request-ID"
	XForwardedForHeader  = "X-Forwarded-For"
	CFConnectingIPHeader = "CF-Connecting-IP"
	XSignatureEd25519    = "X-Signature-Ed25519"
	XSignatureTimestamp  = "X-Signature-Timestamp"
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
		ctx = context.WithValue(ctx, reqctx.RemoteAddrKey, r.RemoteAddr)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// SessionMiddleware - Read the session from the request
func SessionMiddleware(service session.Service) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if authHeader := r.Header.Get(AuthHeader); authHeader != "" {
				authStrings := strings.Split(authHeader, "Bearer ")
				if len(authStrings) != 2 {
					responses.Unauthorized(w, r, "")
					return
				}
				s, err := service.ReadJWT(r.Context(), authStrings[1])
				if err != nil {
					reqctx.Log(r.Context(), "Error reading JWT:\n\t", err.Error())
					responses.Unauthorized(w, r, "")
					return
				}

				if !s.IsValid() {
					responses.Unauthorized(w, r, "")
					if delErr := service.DeleteSession(context.WithoutCancel(r.Context()), s.ID); delErr != nil {
						reqctx.Log(r.Context(), logErrorDeletingSession, delErr.Error())
					}
					return
				}

				ctx := context.WithValue(r.Context(), reqctx.SessionKey, s)
				ctx = context.WithValue(ctx, reqctx.UserIDKey, s.UserID)
				r = r.WithContext(ctx)
			} else if cookie, err := r.Cookie(session.CookieName); err == nil {
				s, jwtErr := service.ReadJWT(r.Context(), cookie.Value)
				if jwtErr != nil {
					reqctx.Log(r.Context(), logErrorReadingJWTFromCookie, jwtErr.Error())
				} else if !s.IsValid() {
					if delErr := service.DeleteSession(context.WithoutCancel(r.Context()), s.ID); delErr != nil {
						reqctx.Log(r.Context(), logErrorDeletingSession, delErr.Error())
					}
				} else {
					ctx := context.WithValue(r.Context(), reqctx.SessionKey, s)
					ctx = context.WithValue(ctx, reqctx.UserIDKey, s.UserID)
					r = r.WithContext(ctx)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitMiddleware - Rate limit requests
func RateLimitMiddleware(cfg config.RateLimit, service ratelimit.Service, prefix string, sessionLimit int, ipLimit int) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, ok := r.Context().Value(reqctx.SessionKey).(*session.Session)
			if ok && s != nil {
				err := service.IncrRateLimit(r.Context(), prefix+":"+s.UserID)
				if err != nil {
					reqctx.Log(r.Context(), logErrorIncrementingRateLimit, err.Error())
					next.ServeHTTP(w, r)
					return
				}
				limit, err := service.GetRateLimit(r.Context(), prefix+":"+s.UserID)
				if err != nil {
					reqctx.Log(r.Context(), logErrorGettingRateLimit, err.Error())
					next.ServeHTTP(w, r)
					return
				}
				if limit > sessionLimit {
					responses.TooManyRequests(w, r, cfg.RetryAfter, msgRateLimited)
					return
				}
			} else {
				ip, _, err := net.SplitHostPort(r.RemoteAddr)
				if err != nil {
					ip = r.RemoteAddr
				}
				err = service.IncrRateLimit(r.Context(), prefix+":"+ip)
				if err != nil {
					reqctx.Log(r.Context(), logErrorIncrementingRateLimit, err.Error())
					next.ServeHTTP(w, r)
					return
				}
				limit, err := service.GetRateLimit(r.Context(), prefix+":"+ip)
				if err != nil {
					reqctx.Log(r.Context(), logErrorGettingRateLimit, err.Error())
					next.ServeHTTP(w, r)
					return
				}
				if limit > ipLimit {
					responses.TooManyRequests(w, r, cfg.RetryAfter, msgRateLimited)
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
			reqctx.Log(r.Context(), fmt.Sprintf("panic serving %s %s: %v\n%s", r.Method, r.URL.Path, recovered, debug.Stack()))
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
		ctx = context.WithValue(ctx, reqctx.RequestIDKey, requestId)
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

		reqctx.Log(r.Context(), fmt.Sprintf("%d %s %s %s",
			wrapped.statusCode,
			r.Method,
			r.URL.Path,
			time.Since(start),
		))
	})
}

// Auth - Authenticate requests
func Auth(service session.Service) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s, ok := r.Context().Value(reqctx.SessionKey).(*session.Session)
			if !ok || s == nil {
				responses.Unauthorized(w, r, "")
				return
			}

			if !s.IsValid() {
				responses.Unauthorized(w, r, "")
				err := service.DeleteSession(context.WithoutCancel(r.Context()), s.ID)
				if err != nil {
					reqctx.Log(r.Context(), logErrorDeletingSession, err.Error())
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
		s, ok := r.Context().Value(reqctx.SessionKey).(*session.Session)
		if !ok || s == nil {
			responses.Unauthorized(w, r, "")
			return
		}
		r.SetPathValue("user_id", s.UserID)
		next.ServeHTTP(w, r)
	})
}

// VerifyEd25519Middleware verifies the body's signature, generally used by Discord
// https://discord.com/developers/docs/interactions/receiving-and-responding#security-and-authorization
func VerifyEd25519Middleware(cfg *config.Config) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			signatureStr := r.Header.Get(XSignatureEd25519)
			signature, err := hex.DecodeString(signatureStr)
			timestamp := r.Header.Get(XSignatureTimestamp)
			if signatureStr == "" || timestamp == "" {
				if err != nil {
					reqctx.Log(r.Context(), "Error decoding signature:\n\t", err.Error())
				}
				responses.Unauthorized(w, r, msgInvalidSignature)
				return
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				reqctx.Log(r.Context(), "Error reading body:\n\t", err.Error())
				responses.Unauthorized(w, r, msgInvalidSignature)
				return
			}

			var buffer bytes.Buffer
			buffer.WriteString(timestamp)
			buffer.Write(bodyBytes)
			if !ed25519.Verify(cfg.Discord.PublicKey, buffer.Bytes(), signature) {
				responses.Unauthorized(w, r, msgInvalidSignature)
				return
			}

			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			next.ServeHTTP(w, r)
		})
	}
}
