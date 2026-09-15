package minecraft

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goccy/go-json"
)

// --- Mock Service ---

type mockService struct {
	player  *Player
	players []*Player
	err     error
}

func (m *mockService) GetPlayerByName(_ string) (*Player, error) {
	return m.player, m.err
}

func (m *mockService) GetPlayerByUUID(_ string) (*Player, error) {
	return m.player, m.err
}

func (m *mockService) GetPlayersByNames(_ []string) ([]*Player, error) {
	return m.players, m.err
}

func (m *mockService) GetProfile(_ string, _ bool) (*Player, error) {
	return m.player, m.err
}

// --- Tests ---

func TestHandler_GetPlayerByNameHandler_OK(t *testing.T) {
	svc := &mockService{player: &Player{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}}
	handler := GetPlayerByNameHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/lookup/name/jeb_", nil)
	r.SetPathValue("name", "jeb_")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var got Player
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got.Name != "jeb_" {
		t.Errorf("expected jeb_, got %s", got.Name)
	}
}

func TestHandler_GetPlayerByNameHandler_NotFound(t *testing.T) {
	svc := &mockService{err: ErrPlayerNotFound}
	handler := GetPlayerByNameHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/lookup/name/nonexistent", nil)
	r.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetPlayerByNameHandler_InternalError(t *testing.T) {
	svc := &mockService{err: errors.New("db error")}
	handler := GetPlayerByNameHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/lookup/name/jeb_", nil)
	r.SetPathValue("name", "jeb_")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandler_GetPlayerByUUIDHandler_OK(t *testing.T) {
	svc := &mockService{player: &Player{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}}
	handler := GetPlayerByUUIDHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/lookup/853c80ef3c3749fdaa49938b674adae6", nil)
	r.SetPathValue("uuid", "853c80ef3c3749fdaa49938b674adae6")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_GetPlayerByUUIDHandler_NotFound(t *testing.T) {
	svc := &mockService{err: ErrPlayerNotFound}
	handler := GetPlayerByUUIDHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/lookup/853c80ef3c3749fdaa49938b674adae6", nil)
	r.SetPathValue("uuid", "853c80ef3c3749fdaa49938b674adae6")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetPlayersByNamesHandler_OK(t *testing.T) {
	svc := &mockService{players: []*Player{
		{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"},
		{ID: "069a79f444e94726a5befca90e38aaf5", Name: "Notch"},
	}}
	handler := GetPlayersByNamesHandler(svc)

	body, _ := json.Marshal([]string{"jeb_", "Notch"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mc/profile/lookup/bulk/byname", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var got []*Player
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 players, got %d", len(got))
	}
}

func TestHandler_GetPlayersByNamesHandler_InvalidBody(t *testing.T) {
	svc := &mockService{}
	handler := GetPlayersByNamesHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/mc/profile/lookup/bulk/byname", strings.NewReader("not json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetPlayersByNamesHandler_Empty(t *testing.T) {
	svc := &mockService{}
	handler := GetPlayersByNamesHandler(svc)

	body, _ := json.Marshal([]string{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mc/profile/lookup/bulk/byname", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetPlayersByNamesHandler_TooMany(t *testing.T) {
	svc := &mockService{}
	handler := GetPlayersByNamesHandler(svc)

	names := make([]string, 11)
	body, _ := json.Marshal(names)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mc/profile/lookup/bulk/byname", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetProfileHandler_OK(t *testing.T) {
	svc := &mockService{player: &Player{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}}
	handler := GetProfileHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/853c80ef3c3749fdaa49938b674adae6", nil)
	r.SetPathValue("uuid", "853c80ef3c3749fdaa49938b674adae6")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_GetProfileHandler_Signed(t *testing.T) {
	svc := &mockService{player: &Player{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}}
	handler := GetProfileHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/853c80ef3c3749fdaa49938b674adae6?unsigned=false", nil)
	r.SetPathValue("uuid", "853c80ef3c3749fdaa49938b674adae6")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_GetProfileHandler_NotFound(t *testing.T) {
	svc := &mockService{err: ErrPlayerNotFound}
	handler := GetProfileHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/853c80ef3c3749fdaa49938b674adae6", nil)
	r.SetPathValue("uuid", "853c80ef3c3749fdaa49938b674adae6")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}
