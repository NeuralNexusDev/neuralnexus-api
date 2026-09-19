package minecraft

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goccy/go-json"
)

// --- Mock Service ---

type mockService struct {
	player             *Player
	players            []*Player
	profile            *Profile
	geyserPlayer       *GeyserPlayer
	geyserSkin         *GeyserSkin
	geyserProfile      *GeyserProfile
	err                error
	textureBody        []byte
	textureContentType string
}

func (m *mockService) GetMojangPlayerByName(_ string) (*Player, error) {
	return m.player, m.err
}

func (m *mockService) GetMojangPlayerByUUID(_ string) (*Player, error) {
	return m.player, m.err
}

func (m *mockService) GetMojangPlayersByNames(_ []string) ([]*Player, error) {
	return m.players, m.err
}

func (m *mockService) GetMojangProfile(_ string, _ bool) (*Player, error) {
	return m.player, m.err
}

func (m *mockService) GetProfile(_ string) (*Profile, error) {
	return m.profile, m.err
}

func (m *mockService) GetProfileByName(_ string) (*Profile, error) {
	return m.profile, m.err
}

func (m *mockService) GetGeyserXUID(_ string) (*GeyserPlayer, error) {
	return m.geyserPlayer, m.err
}

func (m *mockService) GetGeyserSkin(_ int64) (*GeyserSkin, error) {
	return m.geyserSkin, m.err
}

func (m *mockService) GetGeyserProfile(_ int64) (*GeyserProfile, error) {
	return m.geyserProfile, m.err
}

func (m *mockService) GetGeyserProfileByGamertag(_ string) (*GeyserProfile, error) {
	return m.geyserProfile, m.err
}

func (m *mockService) GetTextureContent(_ string) (*TextureResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	body := m.textureBody
	if body == nil {
		body = []byte("mock-texture-data")
	}
	contentType := m.textureContentType
	if contentType == "" {
		contentType = "image/png"
	}
	return &TextureResult{Body: io.NopCloser(bytes.NewReader(body)), ContentType: contentType}, nil
}

func (m *mockService) GetGeyserTextureContent(_ string) (*TextureResult, error) {
	return m.GetTextureContent("")
}

// --- Tests ---

func TestHandler_GetMojangPlayerByNameHandler_OK(t *testing.T) {
	svc := &mockService{player: &Player{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}}
	handler := GetMojangPlayerByNameHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/mojang/lookup/name/jeb_", nil)
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

func TestHandler_GetMojangPlayerByNameHandler_NotFound(t *testing.T) {
	svc := &mockService{err: ErrPlayerNotFound}
	handler := GetMojangPlayerByNameHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/mojang/lookup/name/nonexistent", nil)
	r.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetMojangPlayerByNameHandler_InternalError(t *testing.T) {
	svc := &mockService{err: errors.New("db error")}
	handler := GetMojangPlayerByNameHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/mojang/lookup/name/jeb_", nil)
	r.SetPathValue("name", "jeb_")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandler_GetMojangPlayerByNameHandler_OmitsProfileActionsWhenNil(t *testing.T) {
	svc := &mockService{player: &Player{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}}
	handler := GetMojangPlayerByNameHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/mojang/lookup/name/jeb_", nil)
	r.SetPathValue("name", "jeb_")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := raw["profileActions"]; ok {
		t.Errorf("expected profileActions key to be absent, got %s", w.Body.String())
	}
}

func TestHandler_GetMojangPlayerByUUIDHandler_OK(t *testing.T) {
	svc := &mockService{player: &Player{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}}
	handler := GetMojangPlayerByUUIDHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/mojang/lookup/853c80ef3c3749fdaa49938b674adae6", nil)
	r.SetPathValue("uuid", "853c80ef3c3749fdaa49938b674adae6")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_GetMojangPlayerByUUIDHandler_OmitsProfileActionsWhenNil(t *testing.T) {
	svc := &mockService{player: &Player{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}}
	handler := GetMojangPlayerByUUIDHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/mojang/lookup/853c80ef3c3749fdaa49938b674adae6", nil)
	r.SetPathValue("uuid", "853c80ef3c3749fdaa49938b674adae6")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := raw["profileActions"]; ok {
		t.Errorf("expected profileActions key to be absent, got %s", w.Body.String())
	}
}

func TestHandler_GetMojangPlayerByUUIDHandler_InvalidUUID(t *testing.T) {
	svc := &mockService{}
	handler := GetMojangPlayerByUUIDHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/mojang/lookup/not-a-uuid", nil)
	r.SetPathValue("uuid", "not-a-uuid")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetMojangPlayerByUUIDHandler_NotFound(t *testing.T) {
	svc := &mockService{err: ErrPlayerNotFound}
	handler := GetMojangPlayerByUUIDHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/mojang/lookup/853c80ef3c3749fdaa49938b674adae6", nil)
	r.SetPathValue("uuid", "853c80ef3c3749fdaa49938b674adae6")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetMojangPlayersByNamesHandler_OK(t *testing.T) {
	svc := &mockService{players: []*Player{
		{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"},
		{ID: "069a79f444e94726a5befca90e38aaf5", Name: "Notch"},
	}}
	handler := GetMojangPlayersByNamesHandler(svc)

	body, _ := json.Marshal([]string{"jeb_", "Notch"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mc/mojang/lookup/bulk/byname", strings.NewReader(string(body)))
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

func TestHandler_GetMojangPlayersByNamesHandler_InvalidBody(t *testing.T) {
	svc := &mockService{}
	handler := GetMojangPlayersByNamesHandler(svc)

	r := httptest.NewRequest(http.MethodPost, "/api/v1/mc/mojang/lookup/bulk/byname", strings.NewReader("not json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetMojangPlayersByNamesHandler_Empty(t *testing.T) {
	svc := &mockService{}
	handler := GetMojangPlayersByNamesHandler(svc)

	body, _ := json.Marshal([]string{})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mc/mojang/lookup/bulk/byname", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetMojangPlayersByNamesHandler_TooMany(t *testing.T) {
	svc := &mockService{}
	handler := GetMojangPlayersByNamesHandler(svc)

	names := make([]string, 11)
	body, _ := json.Marshal(names)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mc/mojang/lookup/bulk/byname", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetMojangProfileHandler_OK(t *testing.T) {
	svc := &mockService{player: &Player{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}}
	handler := GetMojangProfileHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/mojang/profile/853c80ef3c3749fdaa49938b674adae6", nil)
	r.SetPathValue("uuid", "853c80ef3c3749fdaa49938b674adae6")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_GetMojangProfileHandler_ProfileActionsPresentAsEmptyArray(t *testing.T) {
	svc := &mockService{player: &Player{
		ID:             "853c80ef3c3749fdaa49938b674adae6",
		Name:           "jeb_",
		ProfileActions: []string{},
	}}
	handler := GetMojangProfileHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/mojang/profile/853c80ef3c3749fdaa49938b674adae6", nil)
	r.SetPathValue("uuid", "853c80ef3c3749fdaa49938b674adae6")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	action, ok := raw["profileActions"]
	if !ok {
		t.Fatalf("expected profileActions key to be present, got %s", w.Body.String())
	}
	if string(action) != "[]" {
		t.Errorf("expected profileActions to serialize as [], got %s", action)
	}
}

func TestHandler_GetMojangProfileHandler_Signed(t *testing.T) {
	svc := &mockService{player: &Player{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}}
	handler := GetMojangProfileHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/mojang/profile/853c80ef3c3749fdaa49938b674adae6?unsigned=false", nil)
	r.SetPathValue("uuid", "853c80ef3c3749fdaa49938b674adae6")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_GetMojangProfileHandler_NotFound(t *testing.T) {
	svc := &mockService{err: ErrPlayerNotFound}
	handler := GetMojangProfileHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/mojang/profile/853c80ef3c3749fdaa49938b674adae6", nil)
	r.SetPathValue("uuid", "853c80ef3c3749fdaa49938b674adae6")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestHandler_GetProfileHandler_OK(t *testing.T) {
	svc := &mockService{profile: &Profile{
		ID:   "853c80ef3c3749fdaa49938b674adae6",
		Name: "jeb_",
		Textures: &TexturesValue{
			ProfileID:   "853c80ef3c3749fdaa49938b674adae6",
			ProfileName: "jeb_",
			Textures: Textures{
				SKIN: &Texture{URL: "http://textures.minecraft.net/texture/abc123hash"},
			},
		},
	}}
	handler := GetProfileHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/853c80ef3c3749fdaa49938b674adae6", nil)
	r.SetPathValue("uuid", "853c80ef3c3749fdaa49938b674adae6")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := raw["properties"]; ok {
		t.Errorf("expected no raw base64 properties in the decoded response, got %s", w.Body.String())
	}
	textures, ok := raw["textures"]
	if !ok {
		t.Fatalf("expected textures to be present as decoded JSON, got %s", w.Body.String())
	}
	var decoded TexturesValue
	if err := json.Unmarshal(textures, &decoded); err != nil {
		t.Fatalf("expected textures to already be JSON, not a base64 string: %v", err)
	}
	if decoded.Textures.SKIN == nil || decoded.Textures.SKIN.URL != "http://textures.minecraft.net/texture/abc123hash" {
		t.Errorf("expected decoded skin URL, got %+v", decoded.Textures.SKIN)
	}
}

func TestHandler_GetProfileHandler_InvalidUUID(t *testing.T) {
	svc := &mockService{}
	handler := GetProfileHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/not-a-uuid", nil)
	r.SetPathValue("uuid", "not-a-uuid")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
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

func TestHandler_GetTextureHandler_OK(t *testing.T) {
	svc := &mockService{textureBody: []byte("mock-texture-data"), textureContentType: "image/png"}
	handler := GetTextureHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/texture/abc123hash", nil)
	r.SetPathValue("hash", "abc123hash")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "image/png" {
		t.Errorf("expected image/png, got %s", w.Header().Get("Content-Type"))
	}
	if w.Body.String() != "mock-texture-data" {
		t.Errorf("expected body mock-texture-data, got %s", w.Body.String())
	}
}

func TestHandler_GetTextureHandler_ServiceError(t *testing.T) {
	svc := &mockService{err: errors.New("upstream issue")}
	handler := GetTextureHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/texture/abc123hash", nil)
	r.SetPathValue("hash", "abc123hash")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}

func TestHandler_GetTextureHandler_NotFound(t *testing.T) {
	svc := &mockService{err: ErrTextureNotFound}
	handler := GetTextureHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/texture/abc123hash", nil)
	r.SetPathValue("hash", "abc123hash")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetMojangPlayersByNamesHandler_WrongMediaType(t *testing.T) {
	svc := &mockService{}
	handler := GetMojangPlayersByNamesHandler(svc)

	body, _ := json.Marshal([]string{"jeb_"})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mc/mojang/lookup/bulk/byname", strings.NewReader(string(body)))
	// Intentionally omitting or setting wrong Content-Type
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("expected 415, got %d", w.Code)
	}
}

func TestHandler_GetMojangPlayersByNamesHandler_EmptyNameInBatch(t *testing.T) {
	svc := &mockService{}
	handler := GetMojangPlayersByNamesHandler(svc)

	body, _ := json.Marshal([]string{"jeb_", ""})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/mc/mojang/lookup/bulk/byname", strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetTextureHandler_EmptyHash(t *testing.T) {
	svc := &mockService{}
	handler := GetTextureHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/texture/", nil)
	r.SetPathValue("hash", "") // Empty hash
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetGeyserXUIDHandler_OK(t *testing.T) {
	svc := &mockService{geyserPlayer: &GeyserPlayer{
		Gamertag: "Notch",
		XUID:     2535457445285308,
		UUID:     "00000000-0000-0000-0009-01fc305e8dbc",
	}}
	handler := GetGeyserXUIDHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/geyser/xuid/Notch", nil)
	r.SetPathValue("gamertag", "Notch")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var got GeyserPlayer
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got.Gamertag != "Notch" {
		t.Errorf("expected Notch, got %s", got.Gamertag)
	}
	if got.UUID != "00000000-0000-0000-0009-01fc305e8dbc" {
		t.Errorf("unexpected UUID: %s", got.UUID)
	}
}

func TestHandler_GetGeyserXUIDHandler_EmptyGamertag(t *testing.T) {
	svc := &mockService{}
	handler := GetGeyserXUIDHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/geyser/xuid/", nil)
	r.SetPathValue("gamertag", "")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetGeyserXUIDHandler_NotFound(t *testing.T) {
	svc := &mockService{err: ErrPlayerNotFound}
	handler := GetGeyserXUIDHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/geyser/xuid/nonexistent", nil)
	r.SetPathValue("gamertag", "nonexistent")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetGeyserXUIDHandler_InternalError(t *testing.T) {
	svc := &mockService{err: errors.New("geyser API error: 500")}
	handler := GetGeyserXUIDHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/geyser/xuid/Notch", nil)
	r.SetPathValue("gamertag", "Notch")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandler_GetGeyserXUIDHandler_InvalidGamertag(t *testing.T) {
	svc := &mockService{err: ErrInvalidGeyserRequest}
	handler := GetGeyserXUIDHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/geyser/xuid/this-gamertag-is-way-too-long", nil)
	r.SetPathValue("gamertag", "this-gamertag-is-way-too-long")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetGeyserSkinHandler_OK(t *testing.T) {
	svc := &mockService{geyserSkin: &GeyserSkin{
		Hash:      "abc123",
		IsSteve:   true,
		TextureID: "def456",
		Value:     "base64value",
	}}
	handler := GetGeyserSkinHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/geyser/skin/2535457445285308", nil)
	r.SetPathValue("xuid", "2535457445285308")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var got GeyserSkin
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got.Hash != "abc123" {
		t.Errorf("expected abc123, got %s", got.Hash)
	}
}

func TestHandler_GetGeyserSkinHandler_InvalidXUID(t *testing.T) {
	svc := &mockService{}
	handler := GetGeyserSkinHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/geyser/skin/not-a-number", nil)
	r.SetPathValue("xuid", "not-a-number")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetGeyserSkinHandler_NotFound(t *testing.T) {
	svc := &mockService{err: ErrSkinNotFound}
	handler := GetGeyserSkinHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/geyser/skin/2535457445285308", nil)
	r.SetPathValue("xuid", "2535457445285308")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestHandler_GetGeyserSkinHandler_InternalError(t *testing.T) {
	svc := &mockService{err: errors.New("geyser API error: 500")}
	handler := GetGeyserSkinHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/geyser/skin/2535457445285308", nil)
	r.SetPathValue("xuid", "2535457445285308")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandler_GetGeyserSkinHandler_UpstreamRejectedXUID(t *testing.T) {
	svc := &mockService{err: ErrInvalidGeyserRequest}
	handler := GetGeyserSkinHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/geyser/skin/2535457445285308", nil)
	r.SetPathValue("xuid", "2535457445285308")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetProfileByNameHandler_OK(t *testing.T) {
	svc := &mockService{profile: &Profile{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}}
	handler := GetProfileByNameHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/name/jeb_", nil)
	r.SetPathValue("name", "jeb_")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got Profile
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got.Name != "jeb_" {
		t.Errorf("expected jeb_, got %s", got.Name)
	}
}

func TestHandler_GetProfileByNameHandler_EmptyName(t *testing.T) {
	svc := &mockService{}
	handler := GetProfileByNameHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/name/", nil)
	r.SetPathValue("name", "")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetProfileByNameHandler_NotFound(t *testing.T) {
	svc := &mockService{err: ErrPlayerNotFound}
	handler := GetProfileByNameHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/name/nonexistent", nil)
	r.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestHandler_GetGeyserProfileHandler_OK(t *testing.T) {
	svc := &mockService{geyserProfile: &GeyserProfile{
		UUID:     "00000000-0000-0000-0009-01fc305e8dbc",
		XUID:     2535457445285308,
		Gamertag: "Notch",
	}}
	handler := GetGeyserProfileHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/bedrock/00000000-0000-0000-0009-01fc305e8dbc", nil)
	r.SetPathValue("uuid", "00000000-0000-0000-0009-01fc305e8dbc")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got GeyserProfile
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got.Gamertag != "Notch" {
		t.Errorf("expected Notch, got %s", got.Gamertag)
	}
}

func TestHandler_GetGeyserProfileHandler_InvalidUUID(t *testing.T) {
	svc := &mockService{}
	handler := GetGeyserProfileHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/bedrock/not-a-uuid", nil)
	r.SetPathValue("uuid", "not-a-uuid")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetGeyserProfileHandler_RealJavaUUIDRejected(t *testing.T) {
	svc := &mockService{}
	handler := GetGeyserProfileHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/bedrock/853c80ef-3c37-49fd-aa49-938b674adae6", nil)
	r.SetPathValue("uuid", "853c80ef-3c37-49fd-aa49-938b674adae6")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for a UUID that isn't a derived Bedrock UUID, got %d", w.Code)
	}
}

func TestHandler_GetGeyserProfileHandler_NotFound(t *testing.T) {
	svc := &mockService{err: ErrPlayerNotFound}
	handler := GetGeyserProfileHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/bedrock/00000000-0000-0000-0009-01fc305e8dbc", nil)
	r.SetPathValue("uuid", "00000000-0000-0000-0009-01fc305e8dbc")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetGeyserProfileByNameHandler_OK(t *testing.T) {
	svc := &mockService{geyserProfile: &GeyserProfile{Gamertag: "Notch", XUID: 2535457445285308}}
	handler := GetGeyserProfileByNameHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/bedrock/name/Notch", nil)
	r.SetPathValue("name", "Notch")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var got GeyserProfile
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if got.Gamertag != "Notch" {
		t.Errorf("expected Notch, got %s", got.Gamertag)
	}
}

func TestHandler_GetGeyserProfileByNameHandler_EmptyGamertag(t *testing.T) {
	svc := &mockService{}
	handler := GetGeyserProfileByNameHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/bedrock/name/", nil)
	r.SetPathValue("name", "")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetGeyserProfileByNameHandler_NotFound(t *testing.T) {
	svc := &mockService{err: ErrPlayerNotFound}
	handler := GetGeyserProfileByNameHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/profile/bedrock/name/nonexistent", nil)
	r.SetPathValue("name", "nonexistent")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetGeyserTextureHandler_OK(t *testing.T) {
	svc := &mockService{textureBody: []byte("mock-geyser-texture-data"), textureContentType: "image/png"}
	handler := GetGeyserTextureHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/texture/geyser/abc123hash", nil)
	r.SetPathValue("hash", "abc123hash")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("Content-Type") != "image/png" {
		t.Errorf("expected image/png, got %s", w.Header().Get("Content-Type"))
	}
	if w.Body.String() != "mock-geyser-texture-data" {
		t.Errorf("expected body mock-geyser-texture-data, got %s", w.Body.String())
	}
}

func TestHandler_GetGeyserTextureHandler_EmptyHash(t *testing.T) {
	svc := &mockService{}
	handler := GetGeyserTextureHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/texture/geyser/", nil)
	r.SetPathValue("hash", "")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandler_GetGeyserTextureHandler_NotFound(t *testing.T) {
	svc := &mockService{err: ErrTextureNotFound}
	handler := GetGeyserTextureHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/texture/geyser/abc123hash", nil)
	r.SetPathValue("hash", "abc123hash")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandler_GetGeyserTextureHandler_ServiceError(t *testing.T) {
	svc := &mockService{err: errors.New("upstream issue")}
	handler := GetGeyserTextureHandler(svc)

	r := httptest.NewRequest(http.MethodGet, "/api/v1/mc/texture/geyser/abc123hash", nil)
	r.SetPathValue("hash", "abc123hash")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", w.Code)
	}
}
