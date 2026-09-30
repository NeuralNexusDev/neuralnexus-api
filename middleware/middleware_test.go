package mw

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"golang.org/x/crypto/ed25519"
)

type mwFakeSessionSvc struct {
	readJWTFunc   func(token string) (*auth.Session, error)
	deleteFunc    func(id string) error
	deletedIDs    []string
	readJWTCalled bool
}

var _ auth.SessionService = (*mwFakeSessionSvc)(nil)

func (f *mwFakeSessionSvc) AddSession(*auth.Session) error { return nil }
func (f *mwFakeSessionSvc) GetSession(string) (*auth.Session, error) {
	return nil, errors.New("not implemented")
}
func (f *mwFakeSessionSvc) UpdateSession(*auth.Session) error       { return nil }
func (f *mwFakeSessionSvc) CreateJWT(*auth.Session) (string, error) { return "", nil }

func (f *mwFakeSessionSvc) DeleteSession(id string) error {
	f.deletedIDs = append(f.deletedIDs, id)
	if f.deleteFunc != nil {
		return f.deleteFunc(id)
	}
	return nil
}

func (f *mwFakeSessionSvc) ReadJWT(token string) (*auth.Session, error) {
	f.readJWTCalled = true
	if f.readJWTFunc != nil {
		return f.readJWTFunc(token)
	}
	return nil, errors.New("no ReadJWT configured")
}

type mwFakeRateLimitSvc struct {
	incrErr     error
	getLimit    int
	getErr      error
	getErrLimit int

	incrCalls []string
	getCalls  []string
}

var _ auth.RateLimitService = (*mwFakeRateLimitSvc)(nil)

func (f *mwFakeRateLimitSvc) IncrRateLimit(key string) error {
	f.incrCalls = append(f.incrCalls, key)
	return f.incrErr
}

func (f *mwFakeRateLimitSvc) GetRateLimit(key string) (int, error) {
	f.getCalls = append(f.getCalls, key)
	if f.getErr != nil {
		return f.getErrLimit, f.getErr
	}
	return f.getLimit, nil
}

func (f *mwFakeRateLimitSvc) SetRateLimit(string, int) error { return nil }

type mwSyncedRateLimitSvc struct {
	mu     sync.Mutex
	counts map[string]int
}

var _ auth.RateLimitService = (*mwSyncedRateLimitSvc)(nil)

func (f *mwSyncedRateLimitSvc) IncrRateLimit(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts == nil {
		f.counts = make(map[string]int)
	}
	f.counts[key]++
	return nil
}

func (f *mwSyncedRateLimitSvc) GetRateLimit(key string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[key], nil
}

func (f *mwSyncedRateLimitSvc) SetRateLimit(key string, limit int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts == nil {
		f.counts = make(map[string]int)
	}
	f.counts[key] = limit
	return nil
}

// mwBaseCtx pre-populates RemoteAddrKey/RequestIDKey, which LogRequest
// type-asserts without an ok-check and would otherwise panic on.
func mwBaseCtx() context.Context {
	ctx := context.WithValue(context.Background(), RemoteAddrKey, "127.0.0.1")
	ctx = context.WithValue(ctx, RequestIDKey, 1)
	return ctx
}

func mwCaptureLog(t *testing.T) func() string {
	t.Helper()
	var buf bytes.Buffer
	prevOut := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return buf.String
}

func TestLogRequest(t *testing.T) {
	t.Run("MW-01_SessionInContextLogsUserID", func(t *testing.T) {
		getLog := mwCaptureLog(t)
		ctx := mwBaseCtx()
		ctx = context.WithValue(ctx, SessionKey, &auth.Session{ID: "s1", UserID: "u1"})

		LogRequest(ctx, "hello", "world")

		out := getLog()
		if !strings.Contains(out, "u1") {
			t.Errorf("expected log output to contain the session's user ID, got: %q", out)
		}
		if !strings.Contains(out, "1 u1 127.0.0.1 hello world") {
			t.Errorf("expected log output to contain the formatted line, got: %q", out)
		}
	})

	t.Run("MW-02_NoSessionLogsNA", func(t *testing.T) {
		getLog := mwCaptureLog(t)
		ctx := mwBaseCtx()

		LogRequest(ctx, "msg")

		out := getLog()
		if !strings.Contains(out, "N/A") {
			t.Errorf("expected log output to contain N/A for a missing session, got: %q", out)
		}
	})

	t.Run("MW-03_NoMessageArgsDoesNotPanic", func(t *testing.T) {
		getLog := mwCaptureLog(t)
		ctx := mwBaseCtx()

		LogRequest(ctx)

		out := getLog()
		if !strings.Contains(out, "1 N/A 127.0.0.1") {
			t.Errorf("expected log output with an empty joined message, got: %q", out)
		}
	})

	t.Run("MW-04_MissingRemoteAddrPanics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected LogRequest to panic when RemoteAddrKey is missing from the context")
			}
		}()

		ctx := context.WithValue(context.Background(), RequestIDKey, 1)
		LogRequest(ctx, "should not get here")
	})
}

func TestCreateStack(t *testing.T) {
	t.Run("MW-05_ExecutionOrderMatchesDeclarationOrder", func(t *testing.T) {
		var order []string
		record := func(name string) Middleware {
			return func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					order = append(order, name)
					next.ServeHTTP(w, r)
				})
			}
		}

		stack := CreateStack(record("first"), record("second"), record("third"))
		final := stack(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "final")
		}))

		final.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

		want := []string{"first", "second", "third", "final"}
		if len(order) != len(want) {
			t.Fatalf("expected order %v, got %v", want, order)
		}
		for i := range want {
			if order[i] != want[i] {
				t.Errorf("expected order %v, got %v", want, order)
				break
			}
		}
	})

	t.Run("MW-06_EmptyStackIsPassthrough", func(t *testing.T) {
		stack := CreateStack()
		nextCalls := 0
		final := stack(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalls++
		}))

		final.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

		if nextCalls != 1 {
			t.Errorf("final handler called %d times, want 1", nextCalls)
		}
	})
}

func TestWrappedWriterWriteHeader(t *testing.T) {
	t.Run("MW-07_SetsStatusCodeAndForwards", func(t *testing.T) {
		rec := httptest.NewRecorder()
		ww := &WrappedWriter{ResponseWriter: rec, statusCode: http.StatusOK}

		ww.WriteHeader(http.StatusTeapot)

		if ww.statusCode != http.StatusTeapot {
			t.Errorf("expected wrapped statusCode %d, got %d", http.StatusTeapot, ww.statusCode)
		}
		if rec.Code != http.StatusTeapot {
			t.Errorf("expected underlying recorder code %d, got %d", http.StatusTeapot, rec.Code)
		}
	})
}

func mwRunIP(r *http.Request) *http.Request {
	var got *http.Request
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r
	})
	IPMiddleware(next).ServeHTTP(httptest.NewRecorder(), r)
	return got
}

func TestIPMiddleware(t *testing.T) {
	t.Run("MW-08_CFConnectingIPSetsRemoteAddr", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.1:1234"
		r.Header.Set(CFConnectingIPHeader, "1.2.3.4")

		got := mwRunIP(r)

		if got.RemoteAddr != "1.2.3.4" {
			t.Errorf("expected RemoteAddr %q, got %q", "1.2.3.4", got.RemoteAddr)
		}
		if v, _ := got.Context().Value(RemoteAddrKey).(string); v != "1.2.3.4" {
			t.Errorf("expected context RemoteAddrKey %q, got %q", "1.2.3.4", v)
		}
	})

	t.Run("MW-09_XForwardedForSingleIPSetsRemoteAddr", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.1:1234"
		r.Header.Set(XForwardedForHeader, "5.6.7.8")

		got := mwRunIP(r)

		if got.RemoteAddr != "5.6.7.8" {
			t.Errorf("expected RemoteAddr %q, got %q", "5.6.7.8", got.RemoteAddr)
		}
	})

	t.Run("MW-10_XForwardedForChainUsesLeftmostTrimmedIP", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.1:1234"
		r.Header.Set(XForwardedForHeader, "9.9.9.9, 10.10.10.10")

		got := mwRunIP(r)

		if got.RemoteAddr != "9.9.9.9" {
			t.Errorf("expected leftmost trimmed IP %q, got %q", "9.9.9.9", got.RemoteAddr)
		}
	})

	t.Run("MW-11_CFConnectingIPTakesPrecedenceOverXForwardedFor", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.1:1234"
		r.Header.Set(CFConnectingIPHeader, "1.1.1.1")
		r.Header.Set(XForwardedForHeader, "2.2.2.2")

		got := mwRunIP(r)

		if got.RemoteAddr != "1.1.1.1" {
			t.Errorf("expected CF-Connecting-IP to win, got %q", got.RemoteAddr)
		}
	})

	t.Run("MW-12_BlankLeftmostXForwardedForLeavesRemoteAddrUnchanged", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.1:1234"
		r.Header.Set(XForwardedForHeader, " ,3.3.3.3")

		got := mwRunIP(r)

		if got.RemoteAddr != "10.0.0.1:1234" {
			t.Errorf("expected RemoteAddr to be left unchanged, got %q", got.RemoteAddr)
		}
	})

	t.Run("MW-13_NoHeadersLeavesRemoteAddrUnchanged", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.1:1234"

		got := mwRunIP(r)

		if got.RemoteAddr != "10.0.0.1:1234" {
			t.Errorf("expected RemoteAddr to be left unchanged, got %q", got.RemoteAddr)
		}
		if v, _ := got.Context().Value(RemoteAddrKey).(string); v != "10.0.0.1:1234" {
			t.Errorf("expected context RemoteAddrKey to match the original RemoteAddr, got %q", v)
		}
	})
}

func mwRunSession(svc auth.SessionService, r *http.Request) (rec *httptest.ResponseRecorder, nextCalls int, gotSession *auth.Session) {
	rec = httptest.NewRecorder()
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		nextCalls++
		gotSession, _ = r.Context().Value(SessionKey).(*auth.Session)
	})
	SessionMiddleware(svc)(next).ServeHTTP(rec, r)
	return rec, nextCalls, gotSession
}

func mwSessionRequest(authHeader, cookieValue string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(mwBaseCtx())
	if authHeader != "" {
		r.Header.Set(AuthHeader, authHeader)
	}
	if cookieValue != "" {
		r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookieValue})
	}
	return r
}

func TestSessionMiddleware(t *testing.T) {
	t.Run("MW-14_NoHeaderNoCookiePassesThrough", func(t *testing.T) {
		svc := &mwFakeSessionSvc{}
		rec, nextCalls, gotSession := mwRunSession(svc, mwSessionRequest("", ""))

		if nextCalls != 1 {
			t.Fatalf("next called %d times, want 1", nextCalls)
		}
		if gotSession != nil {
			t.Errorf("expected no session in context, got %+v", gotSession)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("MW-15_MalformedAuthorizationHeaderRejected", func(t *testing.T) {
		for _, header := range []string{"Basic abc123", "Bearer"} {
			t.Run(header, func(t *testing.T) {
				svc := &mwFakeSessionSvc{}
				rec, nextCalls, _ := mwRunSession(svc, mwSessionRequest(header, ""))

				if nextCalls != 0 {
					t.Errorf("next called %d times, want 0", nextCalls)
				}
				if rec.Code != http.StatusUnauthorized {
					t.Errorf("expected status 401, got %d", rec.Code)
				}
				if svc.readJWTCalled {
					t.Error("expected ReadJWT not to be called for a malformed header")
				}
			})
		}
	})

	t.Run("MW-16_ValidBearerTokenSetsContextAndCallsNext", func(t *testing.T) {
		session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
		svc := &mwFakeSessionSvc{readJWTFunc: func(string) (*auth.Session, error) { return session, nil }}
		rec, nextCalls, gotSession := mwRunSession(svc, mwSessionRequest("Bearer validtoken", ""))

		if nextCalls != 1 {
			t.Fatalf("next called %d times, want 1", nextCalls)
		}
		if gotSession == nil || gotSession.UserID != "u1" {
			t.Errorf("expected session in context, got %+v", gotSession)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("MW-17_ReadJWTErrorRejected", func(t *testing.T) {
		svc := &mwFakeSessionSvc{readJWTFunc: func(string) (*auth.Session, error) { return nil, testerrors.ErrBoom }}
		rec, nextCalls, _ := mwRunSession(svc, mwSessionRequest("Bearer sometoken", ""))

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("MW-52_BearerPrefixWithTrailingSpaceReachesReadJWTWithEmptyToken", func(t *testing.T) {
		var gotToken string
		var tokenSeen bool
		svc := &mwFakeSessionSvc{readJWTFunc: func(token string) (*auth.Session, error) {
			tokenSeen = true
			gotToken = token
			return nil, testerrors.ErrBoom
		}}
		rec, nextCalls, _ := mwRunSession(svc, mwSessionRequest("Bearer ", ""))

		if !tokenSeen {
			t.Fatal("expected ReadJWT to be called for a bare 'Bearer ' prefix")
		}
		if gotToken != "" {
			t.Errorf("expected ReadJWT to be called with an empty token, got %q", gotToken)
		}
		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("MW-18_ExpiredSessionRejectedAndDeleted", func(t *testing.T) {
		session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
		svc := &mwFakeSessionSvc{readJWTFunc: func(string) (*auth.Session, error) { return session, nil }}
		rec, nextCalls, _ := mwRunSession(svc, mwSessionRequest("Bearer expiredtoken", ""))

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
		if len(svc.deletedIDs) != 1 || svc.deletedIDs[0] != "s1" {
			t.Errorf("expected DeleteSession(%q), got %v", "s1", svc.deletedIDs)
		}
	})

	t.Run("MW-19_ExpiredSessionDeleteErrorStillUnauthorized", func(t *testing.T) {
		readLog := mwCaptureLog(t)
		session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
		svc := &mwFakeSessionSvc{
			readJWTFunc: func(string) (*auth.Session, error) { return session, nil },
			deleteFunc:  func(string) error { return testerrors.ErrCacheDown },
		}
		rec, nextCalls, _ := mwRunSession(svc, mwSessionRequest("Bearer expiredtoken", ""))

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401 regardless of the DeleteSession error, got %d", rec.Code)
		}
		if got := strings.Count(readLog(), logErrorDeletingSession); got != 1 {
			t.Errorf("log contains %q %d times, want 1", logErrorDeletingSession, got)
		}
	})

	t.Run("MW-20_ValidCookieSessionSetsContextAndCallsNext", func(t *testing.T) {
		session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
		svc := &mwFakeSessionSvc{readJWTFunc: func(string) (*auth.Session, error) { return session, nil }}
		rec, nextCalls, gotSession := mwRunSession(svc, mwSessionRequest("", "cookietoken"))

		if nextCalls != 1 {
			t.Fatalf("next called %d times, want 1", nextCalls)
		}
		if gotSession == nil || gotSession.UserID != "u1" {
			t.Errorf("expected session in context, got %+v", gotSession)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("MW-21_CookieReadJWTErrorFailsOpen", func(t *testing.T) {
		readLog := mwCaptureLog(t)
		svc := &mwFakeSessionSvc{readJWTFunc: func(string) (*auth.Session, error) { return nil, testerrors.ErrBoom }}
		rec, nextCalls, gotSession := mwRunSession(svc, mwSessionRequest("", "badtoken"))

		if nextCalls != 1 {
			t.Fatalf("next called %d times, want 1", nextCalls)
		}
		if gotSession != nil {
			t.Errorf("expected no session in context, got %+v", gotSession)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 (fail open), got %d", rec.Code)
		}
		if got := strings.Count(readLog(), logErrorReadingJWTFromCookie); got != 1 {
			t.Errorf("log contains %q %d times, want 1", logErrorReadingJWTFromCookie, got)
		}
	})

	t.Run("MW-22_CookieExpiredSessionFailsOpenAndDeletes", func(t *testing.T) {
		session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
		svc := &mwFakeSessionSvc{readJWTFunc: func(string) (*auth.Session, error) { return session, nil }}
		rec, nextCalls, gotSession := mwRunSession(svc, mwSessionRequest("", "expiredtoken"))

		if nextCalls != 1 {
			t.Fatalf("next called %d times, want 1", nextCalls)
		}
		if gotSession != nil {
			t.Errorf("expected no session in context, got %+v", gotSession)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 (fail open), got %d", rec.Code)
		}
		if len(svc.deletedIDs) != 1 || svc.deletedIDs[0] != "s1" {
			t.Errorf("expected DeleteSession(%q), got %v", "s1", svc.deletedIDs)
		}
	})

	t.Run("MW-23_CookieExpiredSessionDeleteErrorStillFailsOpen", func(t *testing.T) {
		readLog := mwCaptureLog(t)
		session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
		svc := &mwFakeSessionSvc{
			readJWTFunc: func(string) (*auth.Session, error) { return session, nil },
			deleteFunc:  func(string) error { return testerrors.ErrCacheDown },
		}
		rec, nextCalls, gotSession := mwRunSession(svc, mwSessionRequest("", "expiredtoken"))

		if nextCalls != 1 {
			t.Fatalf("next called %d times, want 1", nextCalls)
		}
		if gotSession != nil {
			t.Errorf("expected no session in context, got %+v", gotSession)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 (fail open), got %d", rec.Code)
		}
		if got := strings.Count(readLog(), logErrorDeletingSession); got != 1 {
			t.Errorf("log contains %q %d times, want 1", logErrorDeletingSession, got)
		}
	})
}

func mwRunRateLimit(svc auth.RateLimitService, prefix string, sessionLimit, ipLimit int, r *http.Request) (rec *httptest.ResponseRecorder, nextCalls int) {
	rec = httptest.NewRecorder()
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalls++
	})
	RateLimitMiddleware(svc, prefix, sessionLimit, ipLimit)(next).ServeHTTP(rec, r)
	return rec, nextCalls
}

func mwRequireDetail(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var p responses.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to decode problem body %q: %v", rec.Body.String(), err)
	}
	if p.Detail != want {
		t.Fatalf("detail = %q, want %q", p.Detail, want)
	}
}

func mwRateLimitRequest(session *auth.Session, remoteAddr string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remoteAddr
	ctx := mwBaseCtx()
	if session != nil {
		ctx = context.WithValue(ctx, SessionKey, session)
	}
	return r.WithContext(ctx)
}

func TestRateLimitMiddleware(t *testing.T) {
	t.Run("MW-24_SessionUnderLimitCallsNext", func(t *testing.T) {
		svc := &mwFakeRateLimitSvc{getLimit: 3}
		r := mwRateLimitRequest(&auth.Session{ID: "s1", UserID: "u1"}, "1.2.3.4:5678")

		rec, nextCalls := mwRunRateLimit(svc, "rl", 5, 5, r)

		if nextCalls != 1 {
			t.Errorf("next called %d times, want 1 (when under the session limit)", nextCalls)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
		if len(svc.incrCalls) != 1 || svc.incrCalls[0] != "rl:u1" {
			t.Errorf("expected IncrRateLimit(%q), got %v", "rl:u1", svc.incrCalls)
		}
	})

	t.Run("MW-25_SessionOverLimitRejected", func(t *testing.T) {
		svc := &mwFakeRateLimitSvc{getLimit: 10}
		r := mwRateLimitRequest(&auth.Session{ID: "s1", UserID: "u1"}, "1.2.3.4:5678")

		rec, nextCalls := mwRunRateLimit(svc, "rl", 5, 5, r)

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0 (when over the session limit)", nextCalls)
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("expected status 429, got %d", rec.Code)
		}
		mwRequireDetail(t, rec, msgRateLimited)
		if rec.Header().Get("Retry-After") == "" {
			t.Error("expected a Retry-After header on a 429 response")
		}
	})

	t.Run("MW-26_SessionIncrErrorFailsOpen", func(t *testing.T) {
		readLog := mwCaptureLog(t)
		svc := &mwFakeRateLimitSvc{incrErr: testerrors.ErrRedisDown, getLimit: 10}
		r := mwRateLimitRequest(&auth.Session{ID: "s1", UserID: "u1"}, "1.2.3.4:5678")

		rec, nextCalls := mwRunRateLimit(svc, "rl", 5, 5, r)

		if nextCalls != 1 {
			t.Errorf("next called %d times, want 1", nextCalls)
		}
		if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
			t.Errorf("expected the middleware to write nothing, got status %d body %q", rec.Code, rec.Body.String())
		}
		if len(svc.getCalls) != 0 {
			t.Errorf("GetRateLimit calls = %v, want 0", svc.getCalls)
		}
		if !strings.Contains(readLog(), logErrorIncrementingRateLimit) {
			t.Errorf("expected the log to contain %q", logErrorIncrementingRateLimit)
		}
	})

	t.Run("MW-27_SessionGetErrorFailsOpen", func(t *testing.T) {
		readLog := mwCaptureLog(t)
		svc := &mwFakeRateLimitSvc{getErr: testerrors.ErrRedisDown, getErrLimit: 10}
		r := mwRateLimitRequest(&auth.Session{ID: "s1", UserID: "u1"}, "1.2.3.4:5678")

		rec, nextCalls := mwRunRateLimit(svc, "rl", 5, 5, r)

		if nextCalls != 1 {
			t.Errorf("next called %d times, want 1", nextCalls)
		}
		if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
			t.Errorf("expected the middleware to write nothing, got status %d body %q", rec.Code, rec.Body.String())
		}
		if len(svc.getCalls) != 1 {
			t.Errorf("GetRateLimit calls = %v, want 1", svc.getCalls)
		}
		if !strings.Contains(readLog(), logErrorGettingRateLimit) {
			t.Errorf("expected the log to contain %q", logErrorGettingRateLimit)
		}
	})

	t.Run("MW-28_NoSessionUnderIPLimitCallsNext", func(t *testing.T) {
		svc := &mwFakeRateLimitSvc{getLimit: 1}
		r := mwRateLimitRequest(nil, "9.8.7.6:1234")

		rec, nextCalls := mwRunRateLimit(svc, "rl", 5, 5, r)

		if nextCalls != 1 {
			t.Errorf("next called %d times, want 1 (when under the IP limit)", nextCalls)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
		if len(svc.incrCalls) != 1 || svc.incrCalls[0] != "rl:9.8.7.6" {
			t.Errorf("expected IncrRateLimit(%q), got %v", "rl:9.8.7.6", svc.incrCalls)
		}
	})

	t.Run("MW-29_NoSessionOverIPLimitRejected", func(t *testing.T) {
		svc := &mwFakeRateLimitSvc{getLimit: 10}
		r := mwRateLimitRequest(nil, "9.8.7.6:1234")

		rec, nextCalls := mwRunRateLimit(svc, "rl", 5, 5, r)

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0 (when over the IP limit)", nextCalls)
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("expected status 429, got %d", rec.Code)
		}
		mwRequireDetail(t, rec, msgRateLimited)
	})

	t.Run("MW-30_NoSessionRemoteAddrWithoutPortUsesRawValue", func(t *testing.T) {
		svc := &mwFakeRateLimitSvc{getLimit: 1}
		r := mwRateLimitRequest(nil, "9.8.7.6")

		_, nextCalls := mwRunRateLimit(svc, "rl", 5, 5, r)

		if nextCalls != 1 {
			t.Errorf("next called %d times, want 1 (when under the IP limit)", nextCalls)
		}
		if len(svc.incrCalls) != 1 || svc.incrCalls[0] != "rl:9.8.7.6" {
			t.Errorf("expected the raw RemoteAddr to be used as the key, got %v", svc.incrCalls)
		}
	})

	t.Run("MW-31_NoSessionIncrErrorFailsOpen", func(t *testing.T) {
		readLog := mwCaptureLog(t)
		svc := &mwFakeRateLimitSvc{incrErr: testerrors.ErrRedisDown, getLimit: 1}
		r := mwRateLimitRequest(nil, "9.8.7.6:1234")

		rec, nextCalls := mwRunRateLimit(svc, "rl", 5, 5, r)

		if nextCalls != 1 {
			t.Errorf("next called %d times, want 1", nextCalls)
		}
		if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
			t.Errorf("expected the middleware to write nothing, got status %d body %q", rec.Code, rec.Body.String())
		}
		if len(svc.getCalls) != 0 {
			t.Errorf("GetRateLimit calls = %v, want 0", svc.getCalls)
		}
		if !strings.Contains(readLog(), logErrorIncrementingRateLimit) {
			t.Errorf("expected the log to contain %q", logErrorIncrementingRateLimit)
		}
	})

	t.Run("MW-54_NoSessionIncrErrorFailsOpenEvenOverLimit", func(t *testing.T) {
		readLog := mwCaptureLog(t)
		svc := &mwFakeRateLimitSvc{incrErr: testerrors.ErrRedisDown, getLimit: 10}
		r := mwRateLimitRequest(nil, "9.8.7.6:1234")

		rec, nextCalls := mwRunRateLimit(svc, "rl", 5, 5, r)

		if nextCalls != 1 {
			t.Errorf("next called %d times, want 1", nextCalls)
		}
		if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
			t.Errorf("expected the middleware to write nothing, got status %d body %q", rec.Code, rec.Body.String())
		}
		if len(svc.getCalls) != 0 {
			t.Errorf("GetRateLimit calls = %v, want 0", svc.getCalls)
		}
		if !strings.Contains(readLog(), logErrorIncrementingRateLimit) {
			t.Errorf("expected the log to contain %q", logErrorIncrementingRateLimit)
		}
	})

	t.Run("MW-32_NoSessionGetErrorFailsOpen", func(t *testing.T) {
		readLog := mwCaptureLog(t)
		svc := &mwFakeRateLimitSvc{getErr: testerrors.ErrRedisDown, getErrLimit: 10}
		r := mwRateLimitRequest(nil, "9.8.7.6:1234")

		rec, nextCalls := mwRunRateLimit(svc, "rl", 5, 5, r)

		if nextCalls != 1 {
			t.Errorf("next called %d times, want 1", nextCalls)
		}
		if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
			t.Errorf("expected the middleware to write nothing, got status %d body %q", rec.Code, rec.Body.String())
		}
		if len(svc.getCalls) != 1 {
			t.Errorf("GetRateLimit calls = %v, want 1", svc.getCalls)
		}
		if !strings.Contains(readLog(), logErrorGettingRateLimit) {
			t.Errorf("expected the log to contain %q", logErrorGettingRateLimit)
		}
	})

	t.Run("MW-53_ConcurrentSessionRequestsNeverExceedLimit", func(t *testing.T) {
		const (
			trials      = 20
			concurrency = 30
			limit       = 5
		)
		for trial := 0; trial < trials; trial++ {
			svc := &mwSyncedRateLimitSvc{}
			session := &auth.Session{ID: "s1", UserID: "concurrent-user"}

			var passed int32
			var wg sync.WaitGroup
			start := make(chan struct{})
			wg.Add(concurrency)
			for i := 0; i < concurrency; i++ {
				go func() {
					defer wg.Done()
					<-start
					r := mwRateLimitRequest(session, "1.2.3.4:5678")
					_, nextCalls := mwRunRateLimit(svc, "rl-concurrent", limit, limit, r)
					if nextCalls > 1 {
						t.Errorf("next called %d times for one request, want at most 1", nextCalls)
					}
					if nextCalls > 0 {
						atomic.AddInt32(&passed, 1)
					}
				}()
			}
			close(start)
			wg.Wait()

			if int(passed) > limit {
				t.Fatalf("trial %d: expected at most %d concurrent requests to pass, got %d", trial, limit, passed)
			}
		}
	})
}

func mwRunRequestID(r *http.Request) *http.Request {
	var got *http.Request
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r
	})
	RequestIDMiddleware(next).ServeHTTP(httptest.NewRecorder(), r)
	return got
}

func TestRequestIDMiddleware(t *testing.T) {
	t.Run("MW-33_NoHeaderGeneratesIDFromTime", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		got := mwRunRequestID(r)

		id, ok := got.Context().Value(RequestIDKey).(int)
		if !ok || id == 0 {
			t.Errorf("expected a non-zero generated request ID in context, got %v (ok=%v)", id, ok)
		}
		if got.Header.Get(XRequestIDHeader) == "" {
			t.Error("expected the generated request ID to be written back onto the request header")
		}
	})

	t.Run("MW-34_HeaderPresentWithValidIntegerUsesIt", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set(XRequestIDHeader, "42")

		got := mwRunRequestID(r)

		id, ok := got.Context().Value(RequestIDKey).(int)
		if !ok || id != 42 {
			t.Errorf("expected request ID 42 in context, got %v (ok=%v)", id, ok)
		}
	})

	t.Run("MW-35_HeaderPresentButNonNumericDefaultsToZero", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set(XRequestIDHeader, "not-a-number")

		got := mwRunRequestID(r)

		id, ok := got.Context().Value(RequestIDKey).(int)
		if !ok || id != 0 {
			t.Errorf("expected request ID to default to 0 for a non-numeric header, got %v (ok=%v)", id, ok)
		}
	})
}

func TestRequestLoggerMiddleware(t *testing.T) {
	t.Run("MW-36_LogsStatusMethodAndPath", func(t *testing.T) {
		getLog := mwCaptureLog(t)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		})
		handler := RequestLoggerMiddleware(next)

		r := httptest.NewRequest(http.MethodPost, "/things", nil).WithContext(mwBaseCtx())
		handler.ServeHTTP(httptest.NewRecorder(), r)

		out := getLog()
		wantFragment := fmt.Sprintf("%d %s %s", http.StatusCreated, http.MethodPost, "/things")
		if !strings.Contains(out, wantFragment) {
			t.Errorf("expected log output to contain %q, got: %q", wantFragment, out)
		}
	})

	t.Run("MW-37_NoExplicitWriteHeaderLogsDefaultOK", func(t *testing.T) {
		getLog := mwCaptureLog(t)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("body without an explicit status"))
		})
		handler := RequestLoggerMiddleware(next)

		r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(mwBaseCtx())
		handler.ServeHTTP(httptest.NewRecorder(), r)

		out := getLog()
		wantFragment := fmt.Sprintf("%d %s", http.StatusOK, http.MethodGet)
		if !strings.Contains(out, wantFragment) {
			t.Errorf("expected log output to reflect the default status 200, got: %q", out)
		}
	})
}

func mwRunAuth(svc auth.SessionService, r *http.Request) (rec *httptest.ResponseRecorder, nextCalls int) {
	rec = httptest.NewRecorder()
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalls++
	})
	Auth(svc)(next).ServeHTTP(rec, r)
	return rec, nextCalls
}

func TestAuth(t *testing.T) {
	t.Run("MW-38_NoSessionInContextRejected", func(t *testing.T) {
		svc := &mwFakeSessionSvc{}
		r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(mwBaseCtx())

		rec, nextCalls := mwRunAuth(svc, r)

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("MW-39_TypedNilSessionInContextRejected", func(t *testing.T) {
		svc := &mwFakeSessionSvc{}
		ctx := context.WithValue(mwBaseCtx(), SessionKey, (*auth.Session)(nil))
		r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

		rec, nextCalls := mwRunAuth(svc, r)

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("MW-40_ExpiredSessionRejectedAndDeleted", func(t *testing.T) {
		svc := &mwFakeSessionSvc{}
		session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
		ctx := context.WithValue(mwBaseCtx(), SessionKey, session)
		r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

		rec, nextCalls := mwRunAuth(svc, r)

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
		if len(svc.deletedIDs) != 1 || svc.deletedIDs[0] != "s1" {
			t.Errorf("expected DeleteSession(%q), got %v", "s1", svc.deletedIDs)
		}
	})

	t.Run("MW-41_ExpiredSessionDeleteErrorStillUnauthorized", func(t *testing.T) {
		readLog := mwCaptureLog(t)
		svc := &mwFakeSessionSvc{deleteFunc: func(string) error { return testerrors.ErrCacheDown }}
		session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
		ctx := context.WithValue(mwBaseCtx(), SessionKey, session)
		r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

		rec, nextCalls := mwRunAuth(svc, r)

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401 regardless of the DeleteSession error, got %d", rec.Code)
		}
		if got := strings.Count(readLog(), logErrorDeletingSession); got != 1 {
			t.Errorf("log contains %q %d times, want 1", logErrorDeletingSession, got)
		}
	})

	t.Run("MW-42_ValidSessionCallsNext", func(t *testing.T) {
		svc := &mwFakeSessionSvc{}
		session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
		ctx := context.WithValue(mwBaseCtx(), SessionKey, session)
		r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

		rec, nextCalls := mwRunAuth(svc, r)

		if nextCalls != 1 {
			t.Errorf("next called %d times, want 1 (for a valid session)", nextCalls)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})
}

func mwRunSelfUserID(r *http.Request) (rec *httptest.ResponseRecorder, nextCalls int, gotUserID string) {
	rec = httptest.NewRecorder()
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		nextCalls++
		gotUserID = r.PathValue("user_id")
	})
	SelfUserID(next).ServeHTTP(rec, r)
	return rec, nextCalls, gotUserID
}

func TestSelfUserID(t *testing.T) {
	t.Run("MW-43_NoSessionInContextRejected", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/users/me", nil)

		rec, nextCalls, gotUserID := mwRunSelfUserID(r)

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
		if gotUserID != "" {
			t.Errorf("expected no path value to be set, got %q", gotUserID)
		}
	})

	t.Run("MW-44_TypedNilSessionInContextRejected", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), SessionKey, (*auth.Session)(nil))
		r := httptest.NewRequest(http.MethodGet, "/users/me", nil).WithContext(ctx)

		rec, nextCalls, _ := mwRunSelfUserID(r)

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("MW-45_ValidSessionSetsPathValueAndCallsNext", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), SessionKey, &auth.Session{ID: "s1", UserID: "u1"})
		r := httptest.NewRequest(http.MethodGet, "/users/me", nil).WithContext(ctx)

		rec, nextCalls, gotUserID := mwRunSelfUserID(r)

		if nextCalls != 1 {
			t.Fatalf("next called %d times, want 1", nextCalls)
		}
		if gotUserID != "u1" {
			t.Errorf("expected user_id path value %q, got %q", "u1", gotUserID)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})
}

type mwErrReader struct{}

func (mwErrReader) Read([]byte) (int, error) { return 0, testerrors.ErrBoom }
func (mwErrReader) Close() error             { return nil }

func mwEd25519Request(t *testing.T, priv ed25519.PrivateKey, timestamp, body string, corruptSig bool) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/interactions", strings.NewReader(body))
	if priv != nil {
		sig := ed25519.Sign(priv, []byte(timestamp+body))
		if corruptSig {
			sig[0] ^= 0xFF
		}
		r.Header.Set(XSignatureEd25519, fmt.Sprintf("%x", sig))
	}
	if timestamp != "" {
		r.Header.Set(XSignatureTimestamp, timestamp)
	}
	return r
}

func TestVerifyEd25519Middleware(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ed25519 key: %v", err)
	}

	run := func(r *http.Request) (rec *httptest.ResponseRecorder, nextCalls int, bodyInNext []byte) {
		rec = httptest.NewRecorder()
		next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			nextCalls++
			bodyInNext, _ = io.ReadAll(r.Body)
		})
		VerifyEd25519Middleware(pub)(next).ServeHTTP(rec, r)
		return rec, nextCalls, bodyInNext
	}

	t.Run("MW-46_MissingSignatureHeaderRejected", func(t *testing.T) {
		r := mwEd25519Request(t, nil, "1234567890", "{}", false)

		rec, nextCalls, _ := run(r)

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
		mwRequireDetail(t, rec, msgInvalidSignature)
	})

	t.Run("MW-47_MissingTimestampHeaderRejected", func(t *testing.T) {
		r := mwEd25519Request(t, priv, "", "{}", false)

		rec, nextCalls, _ := run(r)

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
		mwRequireDetail(t, rec, msgInvalidSignature)
	})

	t.Run("MW-48_ValidSignatureCallsNextWithReadableBody", func(t *testing.T) {
		body := `{"type":1}`
		r := mwEd25519Request(t, priv, "1700000000", body, false)

		rec, nextCalls, bodyInNext := run(r)

		if nextCalls != 1 {
			t.Fatalf("next called %d times, want 1 (for a valid signature)", nextCalls)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
		if string(bodyInNext) != body {
			t.Errorf("expected the body to be readable in next and match the original, got %q", string(bodyInNext))
		}
	})

	t.Run("MW-49_TamperedSignatureRejected", func(t *testing.T) {
		r := mwEd25519Request(t, priv, "1700000000", "{}", true)

		rec, nextCalls, _ := run(r)

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
		mwRequireDetail(t, rec, msgInvalidSignature)
	})

	t.Run("MW-50_NonHexSignatureWithTimestampRejected", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/interactions", strings.NewReader("{}"))
		r.Header.Set(XSignatureEd25519, "not-hex!!")
		r.Header.Set(XSignatureTimestamp, "1700000000")

		rec, nextCalls, _ := run(r)

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("MW-51_BodyReadErrorRejected", func(t *testing.T) {
		sig := ed25519.Sign(priv, []byte("1700000000"))
		r := httptest.NewRequest(http.MethodPost, "/interactions", nil).WithContext(mwBaseCtx())
		r.Body = mwErrReader{}
		r.Header.Set(XSignatureEd25519, fmt.Sprintf("%x", sig))
		r.Header.Set(XSignatureTimestamp, "1700000000")

		rec, nextCalls, _ := run(r)

		if nextCalls != 0 {
			t.Errorf("next called %d times, want 0", nextCalls)
		}
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
		mwRequireDetail(t, rec, msgInvalidSignature)
	})
}
