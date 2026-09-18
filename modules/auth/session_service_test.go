package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// mockSessionStore implements SessionStore for unit testing sessionService.
type mockSessionStore struct {
	db    map[string]*Session
	cache map[string]*Session

	addToDBErr         error
	updateInDBErr      error
	deleteInDBErr      error
	addToCacheErr      error
	getFromCacheErr    error
	deleteFromCacheErr error

	addToCacheCalled bool
}

var errMockNotFound = errors.New("mock: not found")

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{
		db:    make(map[string]*Session),
		cache: make(map[string]*Session),
	}
}

func (m *mockSessionStore) AddSessionToDB(session *Session) error {
	if m.addToDBErr != nil {
		return m.addToDBErr
	}
	m.db[session.ID] = session
	return nil
}

func (m *mockSessionStore) GetSessionFromDB(id string) (*Session, error) {
	if s, ok := m.db[id]; ok {
		return s, nil
	}
	return nil, errMockNotFound
}

func (m *mockSessionStore) UpdateSessionInDB(session *Session) error {
	if m.updateInDBErr != nil {
		return m.updateInDBErr
	}
	m.db[session.ID] = session
	return nil
}

func (m *mockSessionStore) DeleteSessionInDB(id string) error {
	if m.deleteInDBErr != nil {
		return m.deleteInDBErr
	}
	delete(m.db, id)
	return nil
}

func (m *mockSessionStore) AddSessionToCache(session *Session) error {
	m.addToCacheCalled = true
	if m.addToCacheErr != nil {
		return m.addToCacheErr
	}
	m.cache[session.ID] = session
	return nil
}

func (m *mockSessionStore) GetSessionFromCache(id string) (*Session, error) {
	if m.getFromCacheErr != nil {
		return nil, m.getFromCacheErr
	}
	if s, ok := m.cache[id]; ok {
		return s, nil
	}
	return nil, errMockNotFound
}

func (m *mockSessionStore) DeleteSessionFromCache(id string) error {
	if m.deleteFromCacheErr != nil {
		return m.deleteFromCacheErr
	}
	delete(m.cache, id)
	return nil
}

func TestSessionServiceAddSessionCacheFailureIsFailOpen(t *testing.T) {
	store := newMockSessionStore()
	store.addToCacheErr = errors.New("cache down")
	svc := &sessionService{store: store}

	session := &Session{ID: "s1", UserID: "u1"}
	if err := svc.AddSession(session); err != nil {
		t.Errorf("AddSession should fail open on a cache error, got: %v", err)
	}
	if _, ok := store.db[session.ID]; !ok {
		t.Error("session was not persisted to the DB")
	}
}

func TestSessionServiceAddSessionDBFailurePropagates(t *testing.T) {
	store := newMockSessionStore()
	store.addToDBErr = errors.New("db down")
	svc := &sessionService{store: store}

	if err := svc.AddSession(&Session{ID: "s1"}); err == nil {
		t.Error("expected AddSession to propagate a DB error")
	}
	if store.addToCacheCalled {
		t.Error("AddSession should not attempt to cache a session that failed to persist")
	}
}

func TestSessionServiceGetSessionCacheHit(t *testing.T) {
	store := newMockSessionStore()
	want := &Session{ID: "s1", UserID: "u1"}
	store.cache[want.ID] = want
	svc := &sessionService{store: store}

	got, err := svc.GetSession("s1")
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if got != want {
		t.Error("expected the cached session to be returned without touching the DB")
	}
}

func TestSessionServiceGetSessionCacheMissFallsBackToDB(t *testing.T) {
	store := newMockSessionStore()
	want := &Session{ID: "s1", UserID: "u1"}
	store.db[want.ID] = want
	svc := &sessionService{store: store}

	got, err := svc.GetSession("s1")
	if err != nil {
		t.Fatalf("GetSession returned error: %v", err)
	}
	if got != want {
		t.Error("expected the DB session to be returned on a cache miss")
	}
	if _, ok := store.cache[want.ID]; !ok {
		t.Error("expected GetSession to repopulate the cache after a cache miss")
	}
}

func TestSessionServiceGetSessionCacheMissAndDBMiss(t *testing.T) {
	store := newMockSessionStore()
	svc := &sessionService{store: store}

	if _, err := svc.GetSession("nonexistent"); err == nil {
		t.Error("expected GetSession to return an error when neither cache nor DB has the session")
	}
}

func TestSessionServiceGetSessionRepopulateCacheFailureIsFailOpen(t *testing.T) {
	store := newMockSessionStore()
	want := &Session{ID: "s1", UserID: "u1"}
	store.db[want.ID] = want
	store.addToCacheErr = errors.New("cache down")
	svc := &sessionService{store: store}

	got, err := svc.GetSession("s1")
	if err != nil {
		t.Fatalf("GetSession should fail open when repopulating the cache fails, got: %v", err)
	}
	if got != want {
		t.Error("expected the DB session to still be returned")
	}
}

func TestSessionServiceUpdateSessionDBFailurePropagates(t *testing.T) {
	store := newMockSessionStore()
	store.updateInDBErr = errors.New("db down")
	svc := &sessionService{store: store}

	if err := svc.UpdateSession(&Session{ID: "s1"}); err == nil {
		t.Error("expected UpdateSession to propagate a DB error")
	}
}

func TestSessionServiceUpdateSessionCacheFailureIsFailOpen(t *testing.T) {
	store := newMockSessionStore()
	store.addToCacheErr = errors.New("cache down")
	svc := &sessionService{store: store}

	if err := svc.UpdateSession(&Session{ID: "s1"}); err != nil {
		t.Errorf("UpdateSession should fail open on a cache error, got: %v", err)
	}
}

func TestSessionServiceDeleteSessionSuccess(t *testing.T) {
	store := newMockSessionStore()
	store.db["s1"] = &Session{ID: "s1"}
	store.cache["s1"] = &Session{ID: "s1"}
	svc := &sessionService{store: store}

	if err := svc.DeleteSession("s1"); err != nil {
		t.Fatalf("DeleteSession returned error: %v", err)
	}
	if _, ok := store.db["s1"]; ok {
		t.Error("session was not removed from the DB")
	}
	if _, ok := store.cache["s1"]; ok {
		t.Error("session was not removed from the cache")
	}
}

func TestSessionServiceDeleteSessionDBFailurePropagates(t *testing.T) {
	store := newMockSessionStore()
	store.db["s1"] = &Session{ID: "s1"}
	store.cache["s1"] = &Session{ID: "s1"}
	store.deleteInDBErr = errors.New("db down")
	svc := &sessionService{store: store}

	if err := svc.DeleteSession("s1"); err == nil {
		t.Error("expected DeleteSession to propagate a DB error")
	}
	if _, ok := store.cache["s1"]; !ok {
		t.Error("cache should not be touched when the DB delete itself fails")
	}
}

// A stale cache entry after a "successful" delete would keep a revoked
// session valid (via ReadJWT's cache-first lookup) until it expires, so this
// must fail closed rather than swallow the cache error like the writes do.
func TestSessionServiceDeleteSessionCacheFailurePropagates(t *testing.T) {
	store := newMockSessionStore()
	store.db["s1"] = &Session{ID: "s1"}
	store.deleteFromCacheErr = errors.New("cache down")
	svc := &sessionService{store: store}

	if err := svc.DeleteSession("s1"); err == nil {
		t.Error("expected DeleteSession to propagate a cache-eviction error rather than swallow it")
	}
	if _, ok := store.db["s1"]; ok {
		t.Error("expected the DB delete to have gone through before the cache-eviction error surfaced")
	}
}

func TestSessionServiceReadJWTValidTokenUpdatesLastUsedAt(t *testing.T) {
	store := newMockSessionStore()
	session := &Session{
		ID:         "s1",
		UserID:     "u1",
		IssuedAt:   time.Now().Add(-time.Hour).Unix(),
		LastUsedAt: time.Now().Add(-time.Hour).Unix(),
		ExpiresAt:  time.Now().Add(time.Hour).Unix(),
	}
	store.db[session.ID] = session
	svc := &sessionService{store: store}

	token, err := svc.CreateJWT(session)
	if err != nil {
		t.Fatalf("CreateJWT returned error: %v", err)
	}

	before := session.LastUsedAt

	got, err := svc.ReadJWT(token)
	if err != nil {
		t.Fatalf("ReadJWT returned error: %v", err)
	}
	if got.UserID != "u1" {
		t.Errorf("expected UserID u1, got %s", got.UserID)
	}
	if got.LastUsedAt <= before {
		t.Error("expected ReadJWT to bump LastUsedAt")
	}
}

func TestSessionServiceReadJWTMalformedToken(t *testing.T) {
	store := newMockSessionStore()
	svc := &sessionService{store: store}

	if _, err := svc.ReadJWT("not.a.jwt"); err == nil {
		t.Error("expected ReadJWT to reject a malformed token")
	}
}

func TestSessionServiceReadJWTWrongSigningKey(t *testing.T) {
	store := newMockSessionStore()
	session := &Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	store.db[session.ID] = session

	claims := SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   session.UserID,
			Audience:  validAudiences,
			ID:        session.ID,
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("not-the-real-secret"))
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}

	svc := &sessionService{store: store}
	if _, err := svc.ReadJWT(token); err == nil {
		t.Error("expected ReadJWT to reject a token signed with the wrong key")
	}
}

// Regression guard for algorithm-confusion attacks: ReadJWT restricts
// parsing to HS256 via jwt.WithValidMethods, so a token using any other
// algorithm - including "none" - must be rejected before signature/key
// verification even comes into play.
func TestSessionServiceReadJWTRejectsAlgNone(t *testing.T) {
	store := newMockSessionStore()
	session := &Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	store.db[session.ID] = session

	claims := SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   session.UserID,
			Audience:  validAudiences,
			ID:        session.ID,
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}

	svc := &sessionService{store: store}
	if _, err := svc.ReadJWT(token); err == nil {
		t.Error("expected ReadJWT to reject a token signed with alg \"none\"")
	}
}

func TestSessionServiceReadJWTExpiredToken(t *testing.T) {
	store := newMockSessionStore()
	session := &Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	store.db[session.ID] = session

	claims := SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Subject:   session.UserID,
			Audience:  validAudiences,
			ID:        session.ID,
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(JWT_SECRET)
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}

	svc := &sessionService{store: store}
	if _, err := svc.ReadJWT(token); err == nil {
		t.Error("expected ReadJWT to reject an expired token")
	}
}

func TestSessionServiceReadJWTRevokedSessionRejected(t *testing.T) {
	store := newMockSessionStore()
	session := &Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	svc := &sessionService{store: store}

	// Sign a token for a session that was never (or is no longer) in the store.
	token, err := svc.CreateJWT(session)
	if err != nil {
		t.Fatalf("CreateJWT returned error: %v", err)
	}

	if _, err := svc.ReadJWT(token); err == nil {
		t.Error("expected ReadJWT to reject a token whose session no longer exists in the store")
	}
}

func TestSessionServiceReadJWTSubjectMismatch(t *testing.T) {
	store := newMockSessionStore()
	// Sign a token claiming subject u1, but the store has a different user under the same session ID.
	signed := &Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	store.db["s1"] = &Session{ID: "s1", UserID: "u2", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	svc := &sessionService{store: store}

	token, err := svc.CreateJWT(signed)
	if err != nil {
		t.Fatalf("CreateJWT returned error: %v", err)
	}

	if _, err := svc.ReadJWT(token); err == nil {
		t.Error("expected ReadJWT to reject a token whose subject doesn't match the stored session's UserID")
	}
}

func TestSessionServiceReadJWTInvalidAudience(t *testing.T) {
	store := newMockSessionStore()
	// The session must actually exist in the store, so that a passing test
	// can only mean the audience check itself rejected the token - not that
	// it failed downstream (e.g. session-not-found) for an unrelated reason.
	store.db["s1"] = &Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	svc := &sessionService{store: store}

	claims := SessionClaims{
		Scope: nil,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   "u1",
			Audience:  []string{"https://evil.example.com"},
			ID:        "s1",
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(JWT_SECRET)
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}

	if _, err := svc.ReadJWT(token); err == nil {
		t.Error("expected ReadJWT to reject a token with an unrecognized audience")
	}
}

// Regression guard for the audience-bypass bug: the per-entry audience
// validation loop is vacuously true when claims.Audience is empty (no aud
// claim at all skips the loop body entirely), so a hand-signed token with no
// audience used to sail through ReadJWT. ReadJWT must fail closed instead.
func TestSessionServiceReadJWTMissingAudienceRejected(t *testing.T) {
	store := newMockSessionStore()
	// The session must actually exist in the store, so that a passing test
	// can only mean the audience check itself rejected the token - not that
	// it failed downstream (e.g. session-not-found) for an unrelated reason.
	store.db["s1"] = &Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	svc := &sessionService{store: store}

	claims := SessionClaims{
		Scope: nil,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   "u1",
			// Audience intentionally left as its zero value (nil).
			ID: "s1",
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(JWT_SECRET)
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}

	if _, err := svc.ReadJWT(token); err == nil {
		t.Error("expected ReadJWT to reject a token with no audience claim")
	}
}

// Regression guard: if validAudiences itself is ever misconfigured to
// contain an empty string (e.g. NN_SITE_URL/NN_API_URL were left unset in a
// deployment - now prevented by init(), but this check must hold regardless
// of that), a token with an empty-string audience entry must still be
// rejected rather than matching it.
func TestSessionServiceReadJWTEmptyStringAudienceRejected(t *testing.T) {
	store := newMockSessionStore()
	// The session must actually exist in the store, so that a passing test
	// can only mean the audience check itself rejected the token - not that
	// it failed downstream (e.g. session-not-found) for an unrelated reason.
	store.db["s1"] = &Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	svc := &sessionService{store: store}

	originalValidAudiences := validAudiences
	validAudiences = []string{"", "https://example.com"}
	defer func() { validAudiences = originalValidAudiences }()

	claims := SessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   "u1",
			Audience:  []string{""},
			ID:        "s1",
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(JWT_SECRET)
	if err != nil {
		t.Fatalf("failed to sign test token: %v", err)
	}

	if _, err := svc.ReadJWT(token); err == nil {
		t.Error("expected ReadJWT to reject a token with an empty-string audience entry, even when validAudiences itself contains one")
	}
}

// Regression guard: ReadJWT's trailing call to s.UpdateSession(session) (to
// persist the bumped LastUsedAt) must propagate any error it returns rather
// than silently swallow it - e.g. a future "simplify this to fire-and-forget"
// change. UpdateSession returns the DB error directly without wrapping it,
// so it's asserted here with errors.Is rather than just a non-nil check.
func TestSessionServiceReadJWTPropagatesUpdateSessionError(t *testing.T) {
	store := newMockSessionStore()
	session := &Session{
		ID:        "s1",
		UserID:    "u1",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	store.db[session.ID] = session
	svc := &sessionService{store: store}

	token, err := svc.CreateJWT(session)
	if err != nil {
		t.Fatalf("CreateJWT returned error: %v", err)
	}

	wantErr := errors.New("db down")
	store.updateInDBErr = wantErr

	if _, err := svc.ReadJWT(token); !errors.Is(err, wantErr) {
		t.Errorf("expected ReadJWT to propagate the UpdateSession error %v, got: %v", wantErr, err)
	}
}

// Regression guard for the never-expiring session round-trip bug:
// Session.ExpiresAt == 0 means "never expires" (see Session.IsValid), but
// CreateJWT used to encode that as exp=1970-01-01, which ReadJWT would then
// immediately reject as expired. CreateJWT must omit the exp claim instead,
// and ReadJWT must accept the result.
func TestSessionServiceCreateJWTAndReadJWTNeverExpiringSession(t *testing.T) {
	store := newMockSessionStore()
	session := &Session{
		ID:        "s1",
		UserID:    "u1",
		ExpiresAt: 0,
	}
	store.db[session.ID] = session
	svc := &sessionService{store: store}

	token, err := svc.CreateJWT(session)
	if err != nil {
		t.Fatalf("CreateJWT returned error: %v", err)
	}

	got, err := svc.ReadJWT(token)
	if err != nil {
		t.Fatalf("ReadJWT returned error for a never-expiring session token: %v", err)
	}
	if got.UserID != "u1" {
		t.Errorf("expected UserID u1, got %s", got.UserID)
	}
}
