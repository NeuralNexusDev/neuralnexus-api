package minecraft

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goccy/go-json"
)

type mcMockService struct {
	getMojangPlayerByName      func(name string) (*Player, error)
	getMojangPlayerByUUID      func(id string) (*Player, error)
	getMojangPlayersByNames    func(names []string) ([]*Player, error)
	getMojangProfile           func(id string, signed bool) (*Player, error)
	getProfile                 func(id string) (*Profile, error)
	getProfileByName           func(name string) (*Profile, error)
	getTextureContent          func(hash string) (*TextureResult, error)
	getGeyserXUID              func(gamertag string) (*GeyserPlayer, error)
	getGeyserSkin              func(xuid int64) (*GeyserSkin, error)
	getGeyserProfile           func(xuid int64) (*GeyserProfile, error)
	getGeyserProfileByGamertag func(gamertag string) (*GeyserProfile, error)
	getGeyserTextureContent    func(hash string) (*TextureResult, error)
}

var _ Service = (*mcMockService)(nil)

func (m *mcMockService) GetMojangPlayerByName(name string) (*Player, error) {
	return m.getMojangPlayerByName(name)
}
func (m *mcMockService) GetMojangPlayerByUUID(id string) (*Player, error) {
	return m.getMojangPlayerByUUID(id)
}
func (m *mcMockService) GetMojangPlayersByNames(names []string) ([]*Player, error) {
	return m.getMojangPlayersByNames(names)
}
func (m *mcMockService) GetMojangProfile(id string, signed bool) (*Player, error) {
	return m.getMojangProfile(id, signed)
}
func (m *mcMockService) GetProfile(id string) (*Profile, error) { return m.getProfile(id) }
func (m *mcMockService) GetProfileByName(name string) (*Profile, error) {
	return m.getProfileByName(name)
}
func (m *mcMockService) GetTextureContent(hash string) (*TextureResult, error) {
	return m.getTextureContent(hash)
}
func (m *mcMockService) GetGeyserXUID(gamertag string) (*GeyserPlayer, error) {
	return m.getGeyserXUID(gamertag)
}
func (m *mcMockService) GetGeyserSkin(xuid int64) (*GeyserSkin, error) {
	return m.getGeyserSkin(xuid)
}
func (m *mcMockService) GetGeyserProfile(xuid int64) (*GeyserProfile, error) {
	return m.getGeyserProfile(xuid)
}
func (m *mcMockService) GetGeyserProfileByGamertag(gamertag string) (*GeyserProfile, error) {
	return m.getGeyserProfileByGamertag(gamertag)
}
func (m *mcMockService) GetGeyserTextureContent(hash string) (*TextureResult, error) {
	return m.getGeyserTextureContent(hash)
}

func mcRequest(t *testing.T, method, target string, body io.Reader, pathValues map[string]string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, target, body)
	for k, v := range pathValues {
		r.SetPathValue(k, v)
	}
	return r
}

func mcRequireStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, want, w.Body.String())
	}
}

func TestHD01to04_GetMojangPlayerByNameHandler(t *testing.T) {
	t.Run("HD-01_EmptyName", func(t *testing.T) {
		h := GetMojangPlayerByNameHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/name/", nil, map[string]string{"name": ""}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-02_OK", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayerByName: func(name string) (*Player, error) {
			return &Player{ID: "id", Name: name}, nil
		}}
		h := GetMojangPlayerByNameHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/name/Steve", nil, map[string]string{"name": "Steve"}))
		mcRequireStatus(t, w, http.StatusOK)
		var got Player
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.Name != "Steve" {
			t.Errorf("body = %s, want a player named Steve", w.Body.String())
		}
	})

	t.Run("HD-03_NotFound", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayerByName: func(string) (*Player, error) { return nil, ErrPlayerNotFound }}
		h := GetMojangPlayerByNameHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/name/x", nil, map[string]string{"name": "x"}))
		mcRequireStatus(t, w, http.StatusNotFound)
	})

	t.Run("HD-04_InternalError", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayerByName: func(string) (*Player, error) { return nil, errors.New("boom") }}
		h := GetMojangPlayerByNameHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/name/x", nil, map[string]string{"name": "x"}))
		mcRequireStatus(t, w, http.StatusInternalServerError)
	})
}

func TestHD05to08_GetMojangPlayerByUUIDHandler(t *testing.T) {
	validUUID := "550e8400-e29b-41d4-a716-446655440000"

	t.Run("HD-05_InvalidUUID", func(t *testing.T) {
		h := GetMojangPlayerByUUIDHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/uuid/not-a-uuid", nil, map[string]string{"uuid": "not-a-uuid"}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-06_OK", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayerByUUID: func(id string) (*Player, error) {
			return &Player{ID: id}, nil
		}}
		h := GetMojangPlayerByUUIDHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/uuid/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		mcRequireStatus(t, w, http.StatusOK)
	})

	t.Run("HD-07_NotFound", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayerByUUID: func(string) (*Player, error) { return nil, ErrPlayerNotFound }}
		h := GetMojangPlayerByUUIDHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/uuid/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		mcRequireStatus(t, w, http.StatusNotFound)
	})

	t.Run("HD-08_InternalError", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayerByUUID: func(string) (*Player, error) { return nil, errors.New("boom") }}
		h := GetMojangPlayerByUUIDHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/uuid/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		mcRequireStatus(t, w, http.StatusInternalServerError)
	})
}

func TestHD09to15_GetMojangPlayersByNamesHandler(t *testing.T) {
	post := func(t *testing.T, body string, contentType string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/names", strings.NewReader(body))
		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}
		return r
	}

	t.Run("HD-09_WrongMediaType", func(t *testing.T) {
		h := GetMojangPlayersByNamesHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, post(t, `["a"]`, ""))
		mcRequireStatus(t, w, http.StatusUnsupportedMediaType)
	})

	t.Run("HD-10_InvalidBody", func(t *testing.T) {
		h := GetMojangPlayersByNamesHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, post(t, `{not json`, "application/json"))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-11_Empty", func(t *testing.T) {
		h := GetMojangPlayersByNamesHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, post(t, `[]`, "application/json"))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-12_TooMany", func(t *testing.T) {
		h := GetMojangPlayersByNamesHandler(&mcMockService{})
		w := httptest.NewRecorder()
		names := `["a","b","c","d","e","f","g","h","i","j","k"]`
		h(w, post(t, names, "application/json"))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-13_EmptyNameInBatch", func(t *testing.T) {
		h := GetMojangPlayersByNamesHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, post(t, `["a",""]`, "application/json"))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-14_OK", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayersByNames: func(names []string) ([]*Player, error) {
			out := make([]*Player, len(names))
			for i, n := range names {
				out[i] = &Player{Name: n}
			}
			return out, nil
		}}
		h := GetMojangPlayersByNamesHandler(svc)
		w := httptest.NewRecorder()
		h(w, post(t, `["a","b"]`, "application/json"))
		mcRequireStatus(t, w, http.StatusOK)
		var got []Player
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || len(got) != 2 {
			t.Errorf("body = %s, want 2 players", w.Body.String())
		}
	})

	t.Run("HD-15_ServiceError", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayersByNames: func([]string) ([]*Player, error) { return nil, errors.New("boom") }}
		h := GetMojangPlayersByNamesHandler(svc)
		w := httptest.NewRecorder()
		h(w, post(t, `["a"]`, "application/json"))
		mcRequireStatus(t, w, http.StatusInternalServerError)
	})
}

func TestHD16to20_GetMojangProfileHandler(t *testing.T) {
	validUUID := "550e8400-e29b-41d4-a716-446655440000"

	t.Run("HD-16_InvalidUUID", func(t *testing.T) {
		h := GetMojangProfileHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/profile/bad", nil, map[string]string{"uuid": "bad"}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-17_DefaultQuerySignedFalse", func(t *testing.T) {
		var gotSigned bool
		svc := &mcMockService{getMojangProfile: func(id string, signed bool) (*Player, error) {
			gotSigned = signed
			return &Player{ID: id}, nil
		}}
		h := GetMojangProfileHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/profile/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		mcRequireStatus(t, w, http.StatusOK)
		if gotSigned {
			t.Error("signed = true, want false when 'unsigned' query param is absent")
		}
	})

	t.Run("HD-18_UnsignedFalseMeansSignedTrue", func(t *testing.T) {
		var gotSigned bool
		svc := &mcMockService{getMojangProfile: func(id string, signed bool) (*Player, error) {
			gotSigned = signed
			return &Player{ID: id}, nil
		}}
		h := GetMojangProfileHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/profile/"+validUUID+"?unsigned=false", nil, map[string]string{"uuid": validUUID}))
		mcRequireStatus(t, w, http.StatusOK)
		if !gotSigned {
			t.Error("signed = false, want true when query is unsigned=false")
		}
	})

	t.Run("HD-19_NotFound", func(t *testing.T) {
		svc := &mcMockService{getMojangProfile: func(string, bool) (*Player, error) { return nil, ErrPlayerNotFound }}
		h := GetMojangProfileHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/profile/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		mcRequireStatus(t, w, http.StatusNoContent)
	})

	t.Run("HD-20_InternalError", func(t *testing.T) {
		svc := &mcMockService{getMojangProfile: func(string, bool) (*Player, error) { return nil, errors.New("boom") }}
		h := GetMojangProfileHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/profile/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		mcRequireStatus(t, w, http.StatusInternalServerError)
	})
}

func TestHD21to24_GetProfileHandler(t *testing.T) {
	validUUID := "550e8400-e29b-41d4-a716-446655440000"

	t.Run("HD-21_InvalidUUID", func(t *testing.T) {
		h := GetProfileHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/p/bad", nil, map[string]string{"uuid": "bad"}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-22_OK", func(t *testing.T) {
		svc := &mcMockService{getProfile: func(id string) (*Profile, error) { return &Profile{ID: id}, nil }}
		h := GetProfileHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/p/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		mcRequireStatus(t, w, http.StatusOK)
	})

	t.Run("HD-23_NotFound", func(t *testing.T) {
		svc := &mcMockService{getProfile: func(string) (*Profile, error) { return nil, ErrPlayerNotFound }}
		h := GetProfileHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/p/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		mcRequireStatus(t, w, http.StatusNoContent)
	})

	t.Run("HD-24_InternalError", func(t *testing.T) {
		svc := &mcMockService{getProfile: func(string) (*Profile, error) { return nil, errors.New("boom") }}
		h := GetProfileHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/p/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		mcRequireStatus(t, w, http.StatusInternalServerError)
	})
}

func TestHD25to28_GetProfileByNameHandler(t *testing.T) {
	t.Run("HD-25_EmptyName", func(t *testing.T) {
		h := GetProfileByNameHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/pn/", nil, map[string]string{"name": ""}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-26_OK", func(t *testing.T) {
		svc := &mcMockService{getProfileByName: func(name string) (*Profile, error) { return &Profile{Name: name}, nil }}
		h := GetProfileByNameHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/pn/Steve", nil, map[string]string{"name": "Steve"}))
		mcRequireStatus(t, w, http.StatusOK)
	})

	t.Run("HD-27_NotFound", func(t *testing.T) {
		svc := &mcMockService{getProfileByName: func(string) (*Profile, error) { return nil, ErrPlayerNotFound }}
		h := GetProfileByNameHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/pn/x", nil, map[string]string{"name": "x"}))
		mcRequireStatus(t, w, http.StatusNoContent)
	})

	t.Run("HD-28_InternalError", func(t *testing.T) {
		svc := &mcMockService{getProfileByName: func(string) (*Profile, error) { return nil, errors.New("boom") }}
		h := GetProfileByNameHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/pn/x", nil, map[string]string{"name": "x"}))
		mcRequireStatus(t, w, http.StatusInternalServerError)
	})
}

func TestHD29to33_GetGeyserXUIDHandler(t *testing.T) {
	t.Run("HD-29_EmptyGamertag", func(t *testing.T) {
		h := GetGeyserXUIDHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/g/", nil, map[string]string{"gamertag": ""}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-30_OK", func(t *testing.T) {
		svc := &mcMockService{getGeyserXUID: func(g string) (*GeyserPlayer, error) { return &GeyserPlayer{Gamertag: g}, nil }}
		h := GetGeyserXUIDHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/g/Notch", nil, map[string]string{"gamertag": "Notch"}))
		mcRequireStatus(t, w, http.StatusOK)
	})

	t.Run("HD-31_NotFound", func(t *testing.T) {
		svc := &mcMockService{getGeyserXUID: func(string) (*GeyserPlayer, error) { return nil, ErrPlayerNotFound }}
		h := GetGeyserXUIDHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/g/x", nil, map[string]string{"gamertag": "x"}))
		mcRequireStatus(t, w, http.StatusNotFound)
	})

	t.Run("HD-32_InvalidGeyserRequest", func(t *testing.T) {
		svc := &mcMockService{getGeyserXUID: func(string) (*GeyserPlayer, error) { return nil, ErrInvalidGeyserRequest }}
		h := GetGeyserXUIDHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/g/x", nil, map[string]string{"gamertag": "x"}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-33_InternalError", func(t *testing.T) {
		svc := &mcMockService{getGeyserXUID: func(string) (*GeyserPlayer, error) { return nil, errors.New("boom") }}
		h := GetGeyserXUIDHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/g/x", nil, map[string]string{"gamertag": "x"}))
		mcRequireStatus(t, w, http.StatusInternalServerError)
	})
}

func TestHD34to38_GetGeyserSkinHandler(t *testing.T) {
	t.Run("HD-34_NonNumericXUID", func(t *testing.T) {
		h := GetGeyserSkinHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/s/abc", nil, map[string]string{"xuid": "abc"}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-35_OK", func(t *testing.T) {
		svc := &mcMockService{getGeyserSkin: func(xuid int64) (*GeyserSkin, error) { return &GeyserSkin{Hash: "h"}, nil }}
		h := GetGeyserSkinHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/s/1", nil, map[string]string{"xuid": "1"}))
		mcRequireStatus(t, w, http.StatusOK)
	})

	t.Run("HD-36_NotFound", func(t *testing.T) {
		svc := &mcMockService{getGeyserSkin: func(int64) (*GeyserSkin, error) { return nil, ErrSkinNotFound }}
		h := GetGeyserSkinHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/s/1", nil, map[string]string{"xuid": "1"}))
		mcRequireStatus(t, w, http.StatusNoContent)
	})

	t.Run("HD-37_InvalidGeyserRequest", func(t *testing.T) {
		svc := &mcMockService{getGeyserSkin: func(int64) (*GeyserSkin, error) { return nil, ErrInvalidGeyserRequest }}
		h := GetGeyserSkinHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/s/1", nil, map[string]string{"xuid": "1"}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-38_InternalError", func(t *testing.T) {
		svc := &mcMockService{getGeyserSkin: func(int64) (*GeyserSkin, error) { return nil, errors.New("boom") }}
		h := GetGeyserSkinHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/s/1", nil, map[string]string{"xuid": "1"}))
		mcRequireStatus(t, w, http.StatusInternalServerError)
	})
}

func TestHD39to43_GetGeyserProfileHandler(t *testing.T) {
	bedrockUUID := xuidToUUID(42)

	t.Run("HD-39_NotDerivedBedrockUUID", func(t *testing.T) {
		h := GetGeyserProfileHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gp/x", nil, map[string]string{"uuid": "not-a-uuid"}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-40_OK", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfile: func(xuid int64) (*GeyserProfile, error) {
			return &GeyserProfile{XUID: xuid}, nil
		}}
		h := GetGeyserProfileHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gp/"+bedrockUUID, nil, map[string]string{"uuid": bedrockUUID}))
		mcRequireStatus(t, w, http.StatusOK)
	})

	t.Run("HD-41_NotFound", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfile: func(int64) (*GeyserProfile, error) { return nil, ErrPlayerNotFound }}
		h := GetGeyserProfileHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gp/"+bedrockUUID, nil, map[string]string{"uuid": bedrockUUID}))
		mcRequireStatus(t, w, http.StatusNotFound)
	})

	t.Run("HD-42_InvalidGeyserRequest", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfile: func(int64) (*GeyserProfile, error) { return nil, ErrInvalidGeyserRequest }}
		h := GetGeyserProfileHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gp/"+bedrockUUID, nil, map[string]string{"uuid": bedrockUUID}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-43_InternalError", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfile: func(int64) (*GeyserProfile, error) { return nil, errors.New("boom") }}
		h := GetGeyserProfileHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gp/"+bedrockUUID, nil, map[string]string{"uuid": bedrockUUID}))
		mcRequireStatus(t, w, http.StatusInternalServerError)
	})
}

func TestHD44to48_GetGeyserProfileByNameHandler(t *testing.T) {
	t.Run("HD-44_EmptyGamertag", func(t *testing.T) {
		h := GetGeyserProfileByNameHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpn/", nil, map[string]string{"name": ""}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-45_OK", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfileByGamertag: func(g string) (*GeyserProfile, error) {
			return &GeyserProfile{Gamertag: g}, nil
		}}
		h := GetGeyserProfileByNameHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpn/Notch", nil, map[string]string{"name": "Notch"}))
		mcRequireStatus(t, w, http.StatusOK)
	})

	t.Run("HD-46_NotFound", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfileByGamertag: func(string) (*GeyserProfile, error) { return nil, ErrPlayerNotFound }}
		h := GetGeyserProfileByNameHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpn/x", nil, map[string]string{"name": "x"}))
		mcRequireStatus(t, w, http.StatusNotFound)
	})

	t.Run("HD-47_InvalidGeyserRequest", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfileByGamertag: func(string) (*GeyserProfile, error) { return nil, ErrInvalidGeyserRequest }}
		h := GetGeyserProfileByNameHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpn/x", nil, map[string]string{"name": "x"}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-48_InternalError", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfileByGamertag: func(string) (*GeyserProfile, error) { return nil, errors.New("boom") }}
		h := GetGeyserProfileByNameHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpn/x", nil, map[string]string{"name": "x"}))
		mcRequireStatus(t, w, http.StatusInternalServerError)
	})
}

func TestHD49to52_GetTextureHandler(t *testing.T) {
	t.Run("HD-49_EmptyHash", func(t *testing.T) {
		h := GetTextureHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/t/", nil, map[string]string{"hash": ""}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-50_OK", func(t *testing.T) {
		svc := &mcMockService{getTextureContent: func(hash string) (*TextureResult, error) {
			return &TextureResult{Body: io.NopCloser(strings.NewReader("bytes")), ContentType: "image/png"}, nil
		}}
		h := GetTextureHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/t/abc", nil, map[string]string{"hash": "abc"}))
		mcRequireStatus(t, w, http.StatusOK)
		if w.Body.String() != "bytes" {
			t.Errorf("body = %q, want %q", w.Body.String(), "bytes")
		}
		if got := w.Header().Get("Content-Type"); got != "image/png" {
			t.Errorf("Content-Type = %q, want image/png", got)
		}
	})

	t.Run("HD-51_NotFound", func(t *testing.T) {
		svc := &mcMockService{getTextureContent: func(string) (*TextureResult, error) { return nil, ErrTextureNotFound }}
		h := GetTextureHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/t/abc", nil, map[string]string{"hash": "abc"}))
		mcRequireStatus(t, w, http.StatusNotFound)
	})

	t.Run("HD-52_ServiceError", func(t *testing.T) {
		svc := &mcMockService{getTextureContent: func(string) (*TextureResult, error) { return nil, errors.New("boom") }}
		h := GetTextureHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/t/abc", nil, map[string]string{"hash": "abc"}))
		mcRequireStatus(t, w, http.StatusBadGateway)
	})
}

func TestHD53to56_GetGeyserTextureHandler(t *testing.T) {
	t.Run("HD-53_EmptyHash", func(t *testing.T) {
		h := GetGeyserTextureHandler(&mcMockService{})
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gt/", nil, map[string]string{"hash": ""}))
		mcRequireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-54_OK", func(t *testing.T) {
		svc := &mcMockService{getGeyserTextureContent: func(hash string) (*TextureResult, error) {
			return &TextureResult{Body: io.NopCloser(strings.NewReader("bytes")), ContentType: "image/png"}, nil
		}}
		h := GetGeyserTextureHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gt/abc", nil, map[string]string{"hash": "abc"}))
		mcRequireStatus(t, w, http.StatusOK)
		if w.Body.String() != "bytes" {
			t.Errorf("body = %q, want %q", w.Body.String(), "bytes")
		}
	})

	t.Run("HD-55_NotFound", func(t *testing.T) {
		svc := &mcMockService{getGeyserTextureContent: func(string) (*TextureResult, error) { return nil, ErrTextureNotFound }}
		h := GetGeyserTextureHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gt/abc", nil, map[string]string{"hash": "abc"}))
		mcRequireStatus(t, w, http.StatusNotFound)
	})

	t.Run("HD-56_ServiceError", func(t *testing.T) {
		svc := &mcMockService{getGeyserTextureContent: func(string) (*TextureResult, error) { return nil, errors.New("boom") }}
		h := GetGeyserTextureHandler(svc)
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gt/abc", nil, map[string]string{"hash": "abc"}))
		mcRequireStatus(t, w, http.StatusBadGateway)
	})
}
