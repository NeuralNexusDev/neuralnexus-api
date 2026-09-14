package minecraft

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/redis/go-redis/v9"
)

// --- Mock Store ---

type mockStore struct {
	cache       map[string]*MCPlayer
	db          map[string]*MCPlayer
	upsertErr   error
	cacheSetErr error
}

func newMockStore() *mockStore {
	return &mockStore{
		cache: make(map[string]*MCPlayer),
		db:    make(map[string]*MCPlayer),
	}
}

func (m *mockStore) GetPlayerFromCache(key string) (*MCPlayer, error) {
	if p, ok := m.cache[key]; ok {
		return p, nil
	}
	return nil, redis.Nil
}

func (m *mockStore) SetPlayerInCache(player *MCPlayer) error {
	if m.cacheSetErr != nil {
		return m.cacheSetErr
	}
	m.cache[player.ID] = player
	m.cache[player.Name] = player
	return nil
}

func (m *mockStore) GetPlayerByUUID(id string) (*MCPlayer, error) {
	if p, ok := m.db[id]; ok {
		return p, nil
	}
	return nil, errors.New("not found")
}

func (m *mockStore) GetPlayerByName(name string) (*MCPlayer, error) {
	for _, p := range m.db {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockStore) UpsertPlayer(player *MCPlayer) error {
	if m.upsertErr != nil {
		return m.upsertErr
	}
	m.db[player.ID] = player
	return nil
}

// --- Helpers ---

func newTestServer(status int, body interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		if body != nil {
			//goland:noinspection GoUnhandledErrorResult
			json.NewEncoder(w).Encode(body)
		}
	}))
}

func newTestService(store Store, server *httptest.Server) Service {
	return &service{
		store:  store,
		client: server.Client(),
	}
}

// --- Tests ---

func TestService_GetPlayerByName_CacheHit(t *testing.T) {
	store := newMockStore()
	player := &MCPlayer{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}
	store.cache["jeb_"] = player

	// Server should never be called on a cache hit
	server := newTestServer(http.StatusInternalServerError, nil)
	defer server.Close()

	svc := newTestService(store, server)
	got, err := svc.GetPlayerByName("jeb_")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "jeb_" {
		t.Errorf("expected jeb_, got %s", got.Name)
	}
}

func TestService_GetPlayerByName_CacheMiss_MojangHit(t *testing.T) {
	store := newMockStore()
	mojangResponse := MCPlayer{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}

	server := newTestServer(http.StatusOK, mojangResponse)
	defer server.Close()

	// Point the URL constants at the test server
	svc := &service{
		store:        store,
		client:       server.Client(),
		lookupByName: server.URL + "/",
	}

	got, err := svc.GetPlayerByName("jeb_")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "jeb_" {
		t.Errorf("expected jeb_, got %s", got.Name)
	}
	// Verify upserted to DB
	if _, ok := store.db[mojangResponse.ID]; !ok {
		t.Error("expected player to be upserted to DB")
	}
	// Verify cached
	if _, ok := store.cache[mojangResponse.Name]; !ok {
		t.Error("expected player to be cached by name")
	}
}

func TestService_GetPlayerByName_NotFound(t *testing.T) {
	store := newMockStore()
	server := newTestServer(http.StatusNotFound, nil)
	defer server.Close()

	svc := &service{store: store, client: server.Client(), lookupByName: server.URL + "/"}
	_, err := svc.GetPlayerByName("nonexistent")
	if !errors.Is(err, ErrPlayerNotFound) {
		t.Errorf("expected ErrPlayerNotFound, got %v", err)
	}
}

func TestService_GetPlayerByUUID_CacheHit(t *testing.T) {
	store := newMockStore()
	player := &MCPlayer{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}
	store.cache[player.ID] = player

	server := newTestServer(http.StatusInternalServerError, nil)
	defer server.Close()

	svc := newTestService(store, server)
	got, err := svc.GetPlayerByUUID(player.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != player.ID {
		t.Errorf("expected %s, got %s", player.ID, got.ID)
	}
}

func TestService_GetPlayerByUUID_CacheMiss_MojangHit(t *testing.T) {
	store := newMockStore()
	mojangResponse := MCPlayer{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}

	server := newTestServer(http.StatusOK, mojangResponse)
	defer server.Close()

	svc := &service{store: store, client: server.Client(), lookupByUUID: server.URL + "/"}
	got, err := svc.GetPlayerByUUID(mojangResponse.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != mojangResponse.ID {
		t.Errorf("expected %s, got %s", mojangResponse.ID, got.ID)
	}
}

func TestService_GetPlayersByNames_AllCacheHits(t *testing.T) {
	store := newMockStore()
	store.cache["jeb_"] = &MCPlayer{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}
	store.cache["Notch"] = &MCPlayer{ID: "069a79f444e94726a5befca90e38aaf5", Name: "Notch"}

	server := newTestServer(http.StatusInternalServerError, nil)
	defer server.Close()

	svc := newTestService(store, server)
	got, err := svc.GetPlayersByNames([]string{"jeb_", "Notch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 players, got %d", len(got))
	}
}

func TestService_GetPlayersByNames_TooMany(t *testing.T) {
	store := newMockStore()
	server := newTestServer(http.StatusOK, nil)
	defer server.Close()

	svc := newTestService(store, server)
	names := make([]string, 11)
	_, err := svc.GetPlayersByNames(names)
	if err == nil {
		t.Error("expected error for >10 names")
	}
}

func TestService_GetPlayersByNames_Empty(t *testing.T) {
	store := newMockStore()
	server := newTestServer(http.StatusOK, nil)
	defer server.Close()

	svc := newTestService(store, server)
	_, err := svc.GetPlayersByNames([]string{})
	if err == nil {
		t.Error("expected error for empty names")
	}
}
