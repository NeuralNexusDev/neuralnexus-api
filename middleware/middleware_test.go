package mw

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
)

// mockSessionService implements auth.SessionService for unit testing middleware.
type mockSessionService struct {
	readJWTFunc       func(token string) (*auth.Session, error)
	readJWTCalled     bool
	deleteSessionFunc func(id string) error
	deletedIDs        []string
}

var _ auth.SessionService = (*mockSessionService)(nil)

func (m *mockSessionService) AddSession(*auth.Session) error { return nil }
func (m *mockSessionService) GetSession(string) (*auth.Session, error) {
	return nil, errors.New("not implemented")
}
func (m *mockSessionService) UpdateSession(*auth.Session) error       { return nil }
func (m *mockSessionService) CreateJWT(*auth.Session) (string, error) { return "", nil }

func (m *mockSessionService) DeleteSession(id string) error {
	m.deletedIDs = append(m.deletedIDs, id)
	if m.deleteSessionFunc != nil {
		return m.deleteSessionFunc(id)
	}
	return nil
}

func (m *mockSessionService) ReadJWT(token string) (*auth.Session, error) {
	m.readJWTCalled = true
	return m.readJWTFunc(token)
}

// newTestRequest builds a request carrying the context values LogRequest
// requires (RemoteAddrKey/RequestIDKey), since some of SessionMiddleware's
// error paths call it and it type-asserts those without an ok-check.
func newTestRequest(authHeader string) *http.Request {
	return newTestRequestWithCookie(authHeader, "")
}

// newTestRequestWithCookie is newTestRequest plus an optional session cookie,
// for exercising SessionMiddleware's cookie fallback.
func newTestRequestWithCookie(authHeader, cookieValue string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if authHeader != "" {
		r.Header.Set(AuthHeader, authHeader)
	}
	if cookieValue != "" {
		r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookieValue})
	}
	ctx := context.WithValue(r.Context(), RemoteAddrKey, "127.0.0.1")
	ctx = context.WithValue(ctx, RequestIDKey, 1)
	return r.WithContext(ctx)
}

func runSessionMiddleware(svc auth.SessionService, r *http.Request) (w *httptest.ResponseRecorder, nextCalled bool, gotSession *auth.Session) {
	w = httptest.NewRecorder()
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		nextCalled = true
		gotSession, _ = r.Context().Value(SessionKey).(*auth.Session)
	})
	SessionMiddleware(svc)(next).ServeHTTP(w, r)
	return w, nextCalled, gotSession
}

func TestSessionMiddlewareNoAuthHeaderPassesThrough(t *testing.T) {
	svc := &mockSessionService{}
	w, nextCalled, gotSession := runSessionMiddleware(svc, newTestRequest(""))

	if !nextCalled {
		t.Fatal("expected the request to pass through to the next handler")
	}
	if gotSession != nil {
		t.Error("expected no session in context when no Authorization header is present")
	}
	if svc.readJWTCalled {
		t.Error("ReadJWT should not be called when there's no Authorization header")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSessionMiddlewareMalformedHeaderRejected(t *testing.T) {
	tests := []string{
		"Basic abc123",        // wrong scheme entirely
		"Bearer",              // missing token and separator
		"BearerNoSpace token", // no "Bearer " literal
	}

	for _, header := range tests {
		t.Run(header, func(t *testing.T) {
			svc := &mockSessionService{}
			w, nextCalled, _ := runSessionMiddleware(svc, newTestRequest(header))

			if nextCalled {
				t.Error("expected the request to be rejected before reaching the next handler")
			}
			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", w.Code)
			}
			if svc.readJWTCalled {
				t.Error("ReadJWT should not be called for a malformed Authorization header")
			}
		})
	}
}

// "Bearer " (trailing space, empty token) passes the `len(authStrings) != 2`
// check (it splits into ["", ""]), so unlike the other malformed headers
// above it reaches ReadJWT with an empty token string rather than being
// rejected up front.
func TestSessionMiddlewareBareBearerPrefixReachesReadJWT(t *testing.T) {
	svc := &mockSessionService{
		readJWTFunc: func(token string) (*auth.Session, error) {
			if token != "" {
				t.Errorf("expected ReadJWT to be called with an empty token, got %q", token)
			}
			return nil, errors.New("invalid token")
		},
	}
	w, nextCalled, _ := runSessionMiddleware(svc, newTestRequest("Bearer "))

	if !svc.readJWTCalled {
		t.Error("expected ReadJWT to be called with the empty token")
	}
	if nextCalled {
		t.Error("expected the request to be rejected before reaching the next handler")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestSessionMiddlewareReadJWTErrorRejected(t *testing.T) {
	svc := &mockSessionService{
		readJWTFunc: func(string) (*auth.Session, error) {
			return nil, errors.New("invalid token")
		},
	}
	w, nextCalled, _ := runSessionMiddleware(svc, newTestRequest("Bearer sometoken"))

	if nextCalled {
		t.Error("expected the request to be rejected before reaching the next handler")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestSessionMiddlewareValidSessionSetsContextAndCallsNext(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	svc := &mockSessionService{
		readJWTFunc: func(string) (*auth.Session, error) {
			return session, nil
		},
	}
	w, nextCalled, gotSession := runSessionMiddleware(svc, newTestRequest("Bearer validtoken"))

	if !nextCalled {
		t.Fatal("expected the request to reach the next handler")
	}
	if gotSession == nil || gotSession.UserID != "u1" {
		t.Errorf("expected the session to be set in context, got: %+v", gotSession)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if len(svc.deletedIDs) != 0 {
		t.Error("a valid session should not be deleted")
	}
}

func TestSessionMiddlewareNeverExpiresSessionSetsContextAndCallsNext(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: 0}
	svc := &mockSessionService{
		readJWTFunc: func(string) (*auth.Session, error) {
			return session, nil
		},
	}
	w, nextCalled, gotSession := runSessionMiddleware(svc, newTestRequest("Bearer validtoken"))

	if !nextCalled {
		t.Fatal("expected the request to reach the next handler")
	}
	if gotSession == nil || gotSession.UserID != "u1" {
		t.Errorf("expected the session to be set in context, got: %+v", gotSession)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if len(svc.deletedIDs) != 0 {
		t.Error("a never-expiring session (ExpiresAt == 0) should not be deleted")
	}
}

func TestSessionMiddlewareExpiredSessionRejectedAndDeleted(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
	svc := &mockSessionService{
		readJWTFunc: func(string) (*auth.Session, error) {
			return session, nil
		},
	}
	w, nextCalled, _ := runSessionMiddleware(svc, newTestRequest("Bearer expiredtoken"))

	if nextCalled {
		t.Error("expected an expired session to be rejected before reaching the next handler")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
	if len(svc.deletedIDs) != 1 || svc.deletedIDs[0] != "s1" {
		t.Errorf("expected the expired session to be deleted, deletedIDs: %v", svc.deletedIDs)
	}
}

func TestSessionMiddlewareValidCookieSetsContextAndCallsNext(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	svc := &mockSessionService{
		readJWTFunc: func(token string) (*auth.Session, error) {
			if token != "cookietoken" {
				t.Errorf("expected ReadJWT to be called with the cookie's value, got %q", token)
			}
			return session, nil
		},
	}
	w, nextCalled, gotSession := runSessionMiddleware(svc, newTestRequestWithCookie("", "cookietoken"))

	if !nextCalled {
		t.Fatal("expected the request to reach the next handler")
	}
	if gotSession == nil || gotSession.UserID != "u1" {
		t.Errorf("expected the session to be set in context, got: %+v", gotSession)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSessionMiddlewareInvalidCookieRejected(t *testing.T) {
	svc := &mockSessionService{
		readJWTFunc: func(string) (*auth.Session, error) {
			return nil, errors.New("invalid token")
		},
	}
	w, nextCalled, _ := runSessionMiddleware(svc, newTestRequestWithCookie("", "badtoken"))

	if nextCalled {
		t.Error("expected the request to be rejected before reaching the next handler")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestSessionMiddlewareNoHeaderNoCookiePassesThrough(t *testing.T) {
	svc := &mockSessionService{}
	w, nextCalled, gotSession := runSessionMiddleware(svc, newTestRequestWithCookie("", ""))

	if !nextCalled {
		t.Fatal("expected the request to pass through to the next handler")
	}
	if gotSession != nil {
		t.Error("expected no session in context when neither a header nor a cookie is present")
	}
	if svc.readJWTCalled {
		t.Error("ReadJWT should not be called when there's no header or cookie")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestSessionMiddlewareHeaderTakesPriorityOverCookie pins the documented
// precedence: a bot/integration presenting its own bearer token must not
// have it silently overridden by an unrelated cookie riding along on the
// same request.
func TestSessionMiddlewareHeaderTakesPriorityOverCookie(t *testing.T) {
	svc := &mockSessionService{
		readJWTFunc: func(token string) (*auth.Session, error) {
			if token != "headertoken" {
				t.Errorf("expected ReadJWT to be called with the header's token, got %q", token)
			}
			return &auth.Session{ID: "s1", UserID: "from-header", ExpiresAt: time.Now().Add(time.Hour).Unix()}, nil
		},
	}
	w, nextCalled, gotSession := runSessionMiddleware(svc, newTestRequestWithCookie("Bearer headertoken", "cookietoken"))

	if !nextCalled {
		t.Fatal("expected the request to reach the next handler")
	}
	if gotSession == nil || gotSession.UserID != "from-header" {
		t.Errorf("expected the header's session to win, got: %+v", gotSession)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// mockRateLimitService implements auth.RateLimitService for unit testing
// RateLimitMiddleware.
type mockRateLimitService struct {
	incrErr  error
	getLimit int
	getErr   error

	incrCalls []string
	getCalls  []string
}

var _ auth.RateLimitService = (*mockRateLimitService)(nil)

func (m *mockRateLimitService) IncrRateLimit(key string) error {
	m.incrCalls = append(m.incrCalls, key)
	return m.incrErr
}

func (m *mockRateLimitService) GetRateLimit(key string) (int, error) {
	m.getCalls = append(m.getCalls, key)
	if m.getErr != nil {
		return 0, m.getErr
	}
	return m.getLimit, nil
}

func (m *mockRateLimitService) SetRateLimit(string, int) error { return nil }

func runRateLimitMiddleware(svc auth.RateLimitService, prefix string, sessionLimit, ipLimit int, r *http.Request) (w *httptest.ResponseRecorder, nextCalled bool) {
	w = httptest.NewRecorder()
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	})
	RateLimitMiddleware(svc, prefix, sessionLimit, ipLimit)(next).ServeHTTP(w, r)
	return w, nextCalled
}

func newRateLimitTestRequest(session *auth.Session, remoteAddr string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remoteAddr
	ctx := context.WithValue(r.Context(), RemoteAddrKey, remoteAddr)
	ctx = context.WithValue(ctx, RequestIDKey, 1)
	if session != nil {
		ctx = context.WithValue(ctx, SessionKey, session)
	}
	return r.WithContext(ctx)
}

func TestRateLimitMiddlewareSessionUnderLimitCallsNext(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1"}
	svc := &mockRateLimitService{getLimit: 3}
	r := newRateLimitTestRequest(session, "1.2.3.4:5678")

	w, nextCalled := runRateLimitMiddleware(svc, "rl", 5, 5, r)

	if !nextCalled {
		t.Error("expected next to be called when under the session rate limit")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if len(svc.incrCalls) != 1 || svc.incrCalls[0] != "rl:u1" {
		t.Errorf("expected IncrRateLimit to be called with the session-based key, got: %v", svc.incrCalls)
	}
}

func TestRateLimitMiddlewareSessionOverLimitRejected(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1"}
	svc := &mockRateLimitService{getLimit: 10}
	r := newRateLimitTestRequest(session, "1.2.3.4:5678")

	w, nextCalled := runRateLimitMiddleware(svc, "rl", 5, 5, r)

	if nextCalled {
		t.Error("expected next NOT to be called when over the session rate limit")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w.Code)
	}
}

// The session-based branch fails open: an IncrRateLimit or GetRateLimit
// error is logged but does not block the request, since neither error path
// returns early - execution falls through to the limit check and next().
func TestRateLimitMiddlewareSessionIncrErrorFailsOpen(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1"}
	svc := &mockRateLimitService{incrErr: errors.New("redis down"), getLimit: 1}
	r := newRateLimitTestRequest(session, "1.2.3.4:5678")

	w, nextCalled := runRateLimitMiddleware(svc, "rl", 5, 5, r)

	if !nextCalled {
		t.Error("expected the session-based branch to fail open (call next) on an IncrRateLimit error")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRateLimitMiddlewareSessionGetErrorFailsOpen(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1"}
	svc := &mockRateLimitService{getErr: errors.New("redis down")}
	r := newRateLimitTestRequest(session, "1.2.3.4:5678")

	w, nextCalled := runRateLimitMiddleware(svc, "rl", 5, 5, r)

	if !nextCalled {
		t.Error("expected the session-based branch to fail open (call next) on a GetRateLimit error")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRateLimitMiddlewareNoSessionFallsBackToIPKey(t *testing.T) {
	svc := &mockRateLimitService{getLimit: 1}
	r := newRateLimitTestRequest(nil, "9.8.7.6:1234")

	w, nextCalled := runRateLimitMiddleware(svc, "rl", 5, 5, r)

	if !nextCalled {
		t.Error("expected next to be called when under the IP rate limit")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if len(svc.incrCalls) != 1 || svc.incrCalls[0] != "rl:9.8.7.6" {
		t.Errorf("expected IncrRateLimit to be called with the IP-based key (port stripped), got: %v", svc.incrCalls)
	}
}

// Regression guard: an IPv6 RemoteAddr contains colons of its own, so naively
// splitting on ":" to strip the port truncates it to its first hextet,
// colliding unrelated clients into the same rate-limit bucket.
func TestRateLimitMiddlewareIPv6RemoteAddrKeysDontCollide(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		wantKey    string
	}{
		{"bracketed with port", "[2001:db8::1]:8080", "rl:2001:db8::1"},
		{"bare, no port", "2001:db8::2", "rl:2001:db8::2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockRateLimitService{getLimit: 1}
			r := newRateLimitTestRequest(nil, tt.remoteAddr)

			_, nextCalled := runRateLimitMiddleware(svc, "rl", 5, 5, r)

			if !nextCalled {
				t.Error("expected next to be called when under the IP rate limit")
			}
			if len(svc.incrCalls) != 1 || svc.incrCalls[0] != tt.wantKey {
				t.Errorf("expected IncrRateLimit key %q, got: %v", tt.wantKey, svc.incrCalls)
			}
		})
	}

	// The two addresses above share a first hextet ("2001") - the old
	// strings.Split(addr, ":")[0] logic would key them identically.
	svcA := &mockRateLimitService{getLimit: 1}
	runRateLimitMiddleware(svcA, "rl", 5, 5, newRateLimitTestRequest(nil, "[2001:db8::1]:8080"))
	svcB := &mockRateLimitService{getLimit: 1}
	runRateLimitMiddleware(svcB, "rl", 5, 5, newRateLimitTestRequest(nil, "2001:db8::2"))
	if svcA.incrCalls[0] == svcB.incrCalls[0] {
		t.Errorf("expected distinct IPv6 addresses to produce distinct rate-limit keys, both got %q", svcA.incrCalls[0])
	}
}

func TestRateLimitMiddlewareIPOverLimitRejected(t *testing.T) {
	svc := &mockRateLimitService{getLimit: 10}
	r := newRateLimitTestRequest(nil, "9.8.7.6:1234")

	w, nextCalled := runRateLimitMiddleware(svc, "rl", 5, 5, r)

	if nextCalled {
		t.Error("expected next NOT to be called when over the IP rate limit")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w.Code)
	}
}

// Unlike the session-based branch, the no-session/IP-based branch returns
// immediately when IncrRateLimit errors, so next is never reached. This is
// an intentional asymmetry in the current code, not a design this test is
// endorsing - it just locks in the actual observed behavior.
func TestRateLimitMiddlewareIPIncrErrorReturnsEarlyWithoutCallingNext(t *testing.T) {
	svc := &mockRateLimitService{incrErr: errors.New("redis down"), getLimit: 1}
	r := newRateLimitTestRequest(nil, "9.8.7.6:1234")

	w, nextCalled := runRateLimitMiddleware(svc, "rl", 5, 5, r)

	if nextCalled {
		t.Error("expected the IP-based branch to return early (not call next) on an IncrRateLimit error")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected the default 200 status since no response is written on this early return, got %d", w.Code)
	}
	if len(svc.getCalls) != 0 {
		t.Error("expected GetRateLimit not to be called after an IncrRateLimit error on the IP-based branch")
	}
}

// Same early-return asymmetry as above, but for the GetRateLimit error.
func TestRateLimitMiddlewareIPGetErrorReturnsEarlyWithoutCallingNext(t *testing.T) {
	svc := &mockRateLimitService{getErr: errors.New("redis down")}
	r := newRateLimitTestRequest(nil, "9.8.7.6:1234")

	w, nextCalled := runRateLimitMiddleware(svc, "rl", 5, 5, r)

	if nextCalled {
		t.Error("expected the IP-based branch to return early (not call next) on a GetRateLimit error")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected the default 200 status since no response is written on this early return, got %d", w.Code)
	}
}

func runIPMiddleware(r *http.Request) *http.Request {
	var gotReq *http.Request
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotReq = r
	})
	IPMiddleware(next).ServeHTTP(httptest.NewRecorder(), r)
	return gotReq
}

func TestIPMiddlewareCFConnectingIPTakesPrecedenceOverXForwardedFor(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set(CFConnectingIPHeader, "1.1.1.1")
	r.Header.Set(XForwardedForHeader, "2.2.2.2")

	gotReq := runIPMiddleware(r)

	if gotReq.RemoteAddr != "1.1.1.1" {
		t.Errorf("expected CF-Connecting-IP to take precedence, got RemoteAddr %q", gotReq.RemoteAddr)
	}
}

func TestIPMiddlewareXForwardedForUsedWhenNoCFHeader(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set(XForwardedForHeader, "2.2.2.2")

	gotReq := runIPMiddleware(r)

	if gotReq.RemoteAddr != "2.2.2.2" {
		t.Errorf("expected X-Forwarded-For to be used, got RemoteAddr %q", gotReq.RemoteAddr)
	}
}

func TestIPMiddlewareNoHeadersLeavesRemoteAddrUntouched(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:1234"

	gotReq := runIPMiddleware(r)

	if gotReq.RemoteAddr != "10.0.0.1:1234" {
		t.Errorf("expected RemoteAddr to be left untouched, got %q", gotReq.RemoteAddr)
	}
}

// Regression guard: X-Forwarded-For can be a comma-separated proxy chain
// ("client, proxy1, proxy2, ..."). IPMiddleware must take just the leftmost
// (client) entry, trimmed of whitespace, rather than handing the whole raw
// chain to r.RemoteAddr - otherwise downstream consumers like
// RateLimitMiddleware's net.SplitHostPort fallback can't parse it as a
// single address and fall back to keying on the entire multi-IP string,
// which lets an attacker dodge rate limiting by appending junk after a comma.
func TestIPMiddlewareXForwardedForChainUsesLeftmostEntry(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set(XForwardedForHeader, "203.0.113.5, 70.41.3.18, 150.172.238.178")

	gotReq := runIPMiddleware(r)

	if gotReq.RemoteAddr != "203.0.113.5" {
		t.Errorf("expected RemoteAddr to be the leftmost (client) entry of the X-Forwarded-For chain, got %q", gotReq.RemoteAddr)
	}
}

// Regression guard: a malformed X-Forwarded-For whose leftmost entry is
// empty (a leading comma, or a whitespace-only header) must NOT overwrite
// RemoteAddr with an empty string - that would discard the real socket-level
// peer address and collapse every such request onto the same "prefix:"
// rate-limit bucket downstream.
func TestIPMiddlewareEmptyLeftmostEntryLeavesRemoteAddrUntouched(t *testing.T) {
	tests := []string{", 70.41.3.18", "   ", "\t"}

	for _, xff := range tests {
		t.Run(xff, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = "10.0.0.1:1234"
			r.Header.Set(XForwardedForHeader, xff)

			gotReq := runIPMiddleware(r)

			if gotReq.RemoteAddr != "10.0.0.1:1234" {
				t.Errorf("expected the real RemoteAddr to be preserved when X-Forwarded-For's leftmost entry is empty, got %q", gotReq.RemoteAddr)
			}
		})
	}
}

// End-to-end regression guard: running IPMiddleware then RateLimitMiddleware
// in sequence (as they're chained in production) on a request carrying a
// multi-hop X-Forwarded-For chain must key the rate limit on just the
// client's leftmost IP, not the raw comma-separated chain.
func TestRateLimitMiddlewareAfterIPMiddlewareXForwardedForChainUsesClientIPKey(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set(XForwardedForHeader, "203.0.113.5, 70.41.3.18, 150.172.238.178")
	ctx := context.WithValue(r.Context(), RequestIDKey, 1)
	r = r.WithContext(ctx)

	svc := &mockRateLimitService{getLimit: 1}
	w := httptest.NewRecorder()
	nextCalled := false
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	})

	stack := IPMiddleware(RateLimitMiddleware(svc, "rl", 5, 5)(next))
	stack.ServeHTTP(w, r)

	if !nextCalled {
		t.Error("expected next to be called when under the IP rate limit")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if len(svc.incrCalls) != 1 || svc.incrCalls[0] != "rl:203.0.113.5" {
		t.Errorf("expected IncrRateLimit key %q, got: %v", "rl:203.0.113.5", svc.incrCalls)
	}
}

func TestSessionMiddlewareExpiredSessionDeleteErrorIsLoggedNotFatal(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(-time.Hour).Unix()}
	svc := &mockSessionService{
		readJWTFunc: func(string) (*auth.Session, error) {
			return session, nil
		},
		deleteSessionFunc: func(string) error {
			return errors.New("cache down")
		},
	}
	w, nextCalled, _ := runSessionMiddleware(svc, newTestRequest("Bearer expiredtoken"))

	if nextCalled {
		t.Error("expected an expired session to be rejected before reaching the next handler")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 regardless of the DeleteSession error, got %d", w.Code)
	}
}

func runSelfUserID(r *http.Request) (w *httptest.ResponseRecorder, nextCalled bool, gotUserID string) {
	w = httptest.NewRecorder()
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		nextCalled = true
		gotUserID = r.PathValue("user_id")
	})
	SelfUserID(next).ServeHTTP(w, r)
	return w, nextCalled, gotUserID
}

func TestSelfUserIDSetsPathValueFromSessionAndCallsNext(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1"}
	ctx := context.WithValue(context.Background(), SessionKey, session)
	r := httptest.NewRequest(http.MethodGet, "/users/me", nil).WithContext(ctx)

	w, nextCalled, gotUserID := runSelfUserID(r)

	if !nextCalled {
		t.Fatal("expected the request to reach the next handler")
	}
	if gotUserID != "u1" {
		t.Errorf("expected user_id path value to be set to the session's UserID, got %q", gotUserID)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestSelfUserIDOverridesAnyExistingPathValue guards against a route that
// mistakenly still has a {user_id} wildcard segment (or any other source
// setting one) - the session's own ID must always win for a /me route,
// never something already present on the request.
func TestSelfUserIDOverridesAnyExistingPathValue(t *testing.T) {
	session := &auth.Session{ID: "s1", UserID: "u1"}
	ctx := context.WithValue(context.Background(), SessionKey, session)
	r := httptest.NewRequest(http.MethodGet, "/users/me", nil).WithContext(ctx)
	r.SetPathValue("user_id", "someone-else")

	_, _, gotUserID := runSelfUserID(r)

	if gotUserID != "u1" {
		t.Errorf("expected the session's own UserID to win, got %q", gotUserID)
	}
}

func TestSelfUserIDNoSessionRejected(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/users/me", nil)

	w, nextCalled, _ := runSelfUserID(r)

	if nextCalled {
		t.Error("expected the request to be rejected before reaching the next handler")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 when there's no session in context, got %d", w.Code)
	}
}
