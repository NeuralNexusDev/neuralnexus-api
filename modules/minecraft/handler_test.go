package minecraft

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/goccy/go-json"

	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

type mcMockService struct {
	getMojangPlayerByName   func(name string) (*MojangPlayer, error)
	getMojangPlayerByUUID   func(id string) (*MojangPlayer, error)
	getMojangPlayersByNames func(names []string) ([]*MojangPlayer, error)
	getMojangProfile        func(id string, signed bool) (*MojangPlayer, error)
	getProfile              func(id string) (*MojangProfile, error)
	getProfileByName        func(name string) (*MojangProfile, error)
	getTextureContent       func(hash string) (*TextureResult, error)
	getGeyserXUID           func(gamertag string) (*GeyserPlayer, error)
	getGeyserPlayerByXUID   func(xuid int64) (*GeyserPlayer, error)
	getGeyserPlayerByUUID   func(id string) (*GeyserPlayer, error)
	getGeyserSkin           func(xuid int64) (*GeyserSkin, error)
	getGeyserProfile        func(id string) (*GeyserProfile, error)
	getGeyserProfileByXUID  func(xuid int64) (*GeyserProfile, error)
	getGeyserProfileByName  func(gamertag string) (*GeyserProfile, error)
}

var _ Service = (*mcMockService)(nil)
var _ MojangService = (*mcMockService)(nil)
var _ GeyserService = (*mcMockService)(nil)
var _ TextureService = (*mcMockService)(nil)

func (m *mcMockService) Mojang() MojangService   { return m }
func (m *mcMockService) Geyser() GeyserService   { return m }
func (m *mcMockService) Texture() TextureService { return m }

func (m *mcMockService) GetMojangPlayerByName(_ context.Context, name string) (*MojangPlayer, error) {
	return m.getMojangPlayerByName(name)
}
func (m *mcMockService) GetMojangPlayerByUUID(_ context.Context, id string) (*MojangPlayer, error) {
	return m.getMojangPlayerByUUID(id)
}
func (m *mcMockService) GetMojangPlayersByNames(_ context.Context, names []string) ([]*MojangPlayer, error) {
	return m.getMojangPlayersByNames(names)
}
func (m *mcMockService) GetMojangProfile(_ context.Context, id string, signed bool) (*MojangPlayer, error) {
	return m.getMojangProfile(id, signed)
}
func (m *mcMockService) GetProfile(_ context.Context, id string) (*MojangProfile, error) {
	return m.getProfile(id)
}
func (m *mcMockService) GetProfileByName(_ context.Context, name string) (*MojangProfile, error) {
	return m.getProfileByName(name)
}
func (m *mcMockService) GetTextureContent(_ context.Context, hash string) (*TextureResult, error) {
	return m.getTextureContent(hash)
}
func (m *mcMockService) GetGeyserPlayerByGamertag(_ context.Context, gamertag string) (*GeyserPlayer, error) {
	return m.getGeyserXUID(gamertag)
}
func (m *mcMockService) GetGeyserPlayerByXUID(_ context.Context, xuid int64) (*GeyserPlayer, error) {
	return m.getGeyserPlayerByXUID(xuid)
}
func (m *mcMockService) GetGeyserPlayerByUUID(_ context.Context, id string) (*GeyserPlayer, error) {
	return m.getGeyserPlayerByUUID(id)
}
func (m *mcMockService) GetGeyserSkin(_ context.Context, xuid int64) (*GeyserSkin, error) {
	return m.getGeyserSkin(xuid)
}
func (m *mcMockService) GetGeyserProfile(_ context.Context, id string) (*GeyserProfile, error) {
	return m.getGeyserProfile(id)
}
func (m *mcMockService) GetGeyserProfileByXUID(_ context.Context, xuid int64) (*GeyserProfile, error) {
	return m.getGeyserProfileByXUID(xuid)
}
func (m *mcMockService) GetGeyserProfileByName(_ context.Context, gamertag string) (*GeyserProfile, error) {
	return m.getGeyserProfileByName(gamertag)
}

func mcRequest(t *testing.T, method, target string, body io.Reader, pathValues map[string]string) *http.Request {
	t.Helper()
	r := testutil.NewRequest(method, target, body)
	for k, v := range pathValues {
		r.SetPathValue(k, v)
	}
	return r
}

func TestHD01to04_GetMojangPlayerByNameHandler(t *testing.T) {
	t.Run("HD-02_OK", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayerByName: func(name string) (*MojangPlayer, error) {
			return &MojangPlayer{ID: "id", Name: name}, nil
		}}
		h := NewHandler(svc).GetMojangPlayerByName
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/name/Steve", nil, map[string]string{"name": "Steve"}))
		testutil.ExpectStatus(t, w, http.StatusOK)
		var got MojangPlayer
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.Name != "Steve" {
			t.Errorf("body = %s, want a player named Steve", w.Body.String())
		}
	})

	t.Run("HD-03_NotFound", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayerByName: func(string) (*MojangPlayer, error) { return nil, ErrPlayerNotFound }}
		h := NewHandler(svc).GetMojangPlayerByName
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/name/x", nil, map[string]string{"name": "x"}))
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgPlayerNotFound)
	})

	t.Run("HD-04_InternalError", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayerByName: func(string) (*MojangPlayer, error) { return nil, testutil.ErrBoom }}
		h := NewHandler(svc).GetMojangPlayerByName
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/name/x", nil, map[string]string{"name": "x"}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetPlayer)
	})
}

func TestHD05to08_GetMojangPlayerByUUIDHandler(t *testing.T) {
	validUUID := "550e8400-e29b-41d4-a716-446655440000"

	t.Run("HD-05_InvalidUUID", func(t *testing.T) {
		h := NewHandler(&mcMockService{}).GetMojangPlayerByUUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/uuid/not-a-uuid", nil, map[string]string{"uuid": "not-a-uuid"}))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgNotAValidUUID+"not-a-uuid")
	})

	t.Run("HD-06_OK", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayerByUUID: func(id string) (*MojangPlayer, error) {
			return &MojangPlayer{ID: id}, nil
		}}
		h := NewHandler(svc).GetMojangPlayerByUUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/uuid/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		testutil.ExpectStatus(t, w, http.StatusOK)
	})

	t.Run("HD-07_NotFound", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayerByUUID: func(string) (*MojangPlayer, error) { return nil, ErrPlayerNotFound }}
		h := NewHandler(svc).GetMojangPlayerByUUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/uuid/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgPlayerNotFound)
	})

	t.Run("HD-08_InternalError", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayerByUUID: func(string) (*MojangPlayer, error) { return nil, testutil.ErrBoom }}
		h := NewHandler(svc).GetMojangPlayerByUUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/uuid/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetPlayer)
	})
}

func TestHD09to15_GetMojangPlayersByNamesHandler(t *testing.T) {
	post := func(t *testing.T, body string, contentType string) *http.Request {
		r := testutil.NewRequest(http.MethodPost, "/names", strings.NewReader(body))
		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}
		return r
	}

	t.Run("HD-09_WrongMediaType", func(t *testing.T) {
		h := NewHandler(&mcMockService{}).GetMojangPlayersByNames
		w := httptest.NewRecorder()
		h(w, post(t, `["a"]`, ""))
		testutil.ExpectStatus(t, w, http.StatusUnsupportedMediaType)
		testutil.ExpectDetail(t, w, msgRequestMustBeJSON)
	})

	t.Run("HD-10_InvalidBody", func(t *testing.T) {
		h := NewHandler(&mcMockService{}).GetMojangPlayersByNames
		w := httptest.NewRecorder()
		h(w, post(t, `{not json`, "application/json"))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidRequestBody)
	})

	t.Run("HD-11_Empty", func(t *testing.T) {
		h := NewHandler(&mcMockService{}).GetMojangPlayersByNames
		w := httptest.NewRecorder()
		h(w, post(t, `[]`, "application/json"))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidSize)
	})

	t.Run("HD-12_TooMany", func(t *testing.T) {
		h := NewHandler(&mcMockService{}).GetMojangPlayersByNames
		w := httptest.NewRecorder()
		names := `["a","b","c","d","e","f","g","h","i","j","k"]`
		h(w, post(t, names, "application/json"))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidSize)
	})

	t.Run("HD-13_EmptyNameInBatch", func(t *testing.T) {
		h := NewHandler(&mcMockService{}).GetMojangPlayersByNames
		w := httptest.NewRecorder()
		h(w, post(t, `["a",""]`, "application/json"))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidProfileName)
	})

	t.Run("HD-14_OK", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayersByNames: func(names []string) ([]*MojangPlayer, error) {
			out := make([]*MojangPlayer, len(names))
			for i, n := range names {
				out[i] = &MojangPlayer{Name: n}
			}
			return out, nil
		}}
		h := NewHandler(svc).GetMojangPlayersByNames
		w := httptest.NewRecorder()
		h(w, post(t, `["a","b"]`, "application/json"))
		testutil.ExpectStatus(t, w, http.StatusOK)
		var got []MojangPlayer
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || len(got) != 2 {
			t.Errorf("body = %s, want 2 players", w.Body.String())
		}
	})

	t.Run("HD-15_ServiceError", func(t *testing.T) {
		svc := &mcMockService{getMojangPlayersByNames: func([]string) ([]*MojangPlayer, error) { return nil, testutil.ErrBoom }}
		h := NewHandler(svc).GetMojangPlayersByNames
		w := httptest.NewRecorder()
		h(w, post(t, `["a"]`, "application/json"))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetPlayers)
	})
}

func TestHD16to20_GetMojangProfileHandler(t *testing.T) {
	validUUID := "550e8400-e29b-41d4-a716-446655440000"

	t.Run("HD-16_InvalidUUID", func(t *testing.T) {
		h := NewHandler(&mcMockService{}).GetMojangProfile
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/profile/bad", nil, map[string]string{"uuid": "bad"}))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgNotAValidUUID+"bad")
	})

	t.Run("HD-17_DefaultQuerySignedFalse", func(t *testing.T) {
		var gotSigned bool
		svc := &mcMockService{getMojangProfile: func(id string, signed bool) (*MojangPlayer, error) {
			gotSigned = signed
			return &MojangPlayer{ID: id}, nil
		}}
		h := NewHandler(svc).GetMojangProfile
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/profile/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		testutil.ExpectStatus(t, w, http.StatusOK)
		if gotSigned {
			t.Error("signed = true, want false when 'unsigned' query param is absent")
		}
	})

	t.Run("HD-18_UnsignedFalseMeansSignedTrue", func(t *testing.T) {
		var gotSigned bool
		svc := &mcMockService{getMojangProfile: func(id string, signed bool) (*MojangPlayer, error) {
			gotSigned = signed
			return &MojangPlayer{ID: id}, nil
		}}
		h := NewHandler(svc).GetMojangProfile
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/profile/"+validUUID+"?unsigned=false", nil, map[string]string{"uuid": validUUID}))
		testutil.ExpectStatus(t, w, http.StatusOK)
		if !gotSigned {
			t.Error("signed = false, want true when query is unsigned=false")
		}
	})

	t.Run("HD-19_NotFound", func(t *testing.T) {
		svc := &mcMockService{getMojangProfile: func(string, bool) (*MojangPlayer, error) { return nil, ErrPlayerNotFound }}
		h := NewHandler(svc).GetMojangProfile
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/profile/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		testutil.ExpectStatus(t, w, http.StatusNoContent)
	})

	t.Run("HD-20_InternalError", func(t *testing.T) {
		svc := &mcMockService{getMojangProfile: func(string, bool) (*MojangPlayer, error) { return nil, testutil.ErrBoom }}
		h := NewHandler(svc).GetMojangProfile
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/profile/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetPlayerProfile)
	})
}

func TestHD21to24_GetProfileHandler(t *testing.T) {
	validUUID := "550e8400-e29b-41d4-a716-446655440000"

	t.Run("HD-21_InvalidUUID", func(t *testing.T) {
		h := NewHandler(&mcMockService{}).GetProfile
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/p/bad", nil, map[string]string{"uuid": "bad"}))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgNotAValidUUID+"bad")
	})

	t.Run("HD-22_OK", func(t *testing.T) {
		svc := &mcMockService{getProfile: func(id string) (*MojangProfile, error) { return &MojangProfile{ID: id}, nil }}
		h := NewHandler(svc).GetProfile
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/p/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		testutil.ExpectStatus(t, w, http.StatusOK)
	})

	t.Run("HD-23_NotFound", func(t *testing.T) {
		svc := &mcMockService{getProfile: func(string) (*MojangProfile, error) { return nil, ErrPlayerNotFound }}
		h := NewHandler(svc).GetProfile
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/p/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgPlayerNotFound)
	})

	t.Run("HD-24_InternalError", func(t *testing.T) {
		svc := &mcMockService{getProfile: func(string) (*MojangProfile, error) { return nil, testutil.ErrBoom }}
		h := NewHandler(svc).GetProfile
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/p/"+validUUID, nil, map[string]string{"uuid": validUUID}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetPlayerProfile)
	})
}

func TestHD25to28_GetProfileByNameHandler(t *testing.T) {
	t.Run("HD-26_OK", func(t *testing.T) {
		svc := &mcMockService{getProfileByName: func(name string) (*MojangProfile, error) { return &MojangProfile{Name: name}, nil }}
		h := NewHandler(svc).GetProfileByName
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/pn/Steve", nil, map[string]string{"name": "Steve"}))
		testutil.ExpectStatus(t, w, http.StatusOK)
	})

	t.Run("HD-27_NotFound", func(t *testing.T) {
		svc := &mcMockService{getProfileByName: func(string) (*MojangProfile, error) { return nil, ErrPlayerNotFound }}
		h := NewHandler(svc).GetProfileByName
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/pn/x", nil, map[string]string{"name": "x"}))
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgPlayerNotFound)
	})

	t.Run("HD-28_InternalError", func(t *testing.T) {
		svc := &mcMockService{getProfileByName: func(string) (*MojangProfile, error) { return nil, testutil.ErrBoom }}
		h := NewHandler(svc).GetProfileByName
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/pn/x", nil, map[string]string{"name": "x"}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetPlayerProfile)
	})
}

func TestHD29to33_GetGeyserXUIDHandler(t *testing.T) {
	t.Run("HD-30_OK", func(t *testing.T) {
		svc := &mcMockService{getGeyserXUID: func(g string) (*GeyserPlayer, error) { return &GeyserPlayer{Name: g}, nil }}
		h := NewHandler(svc).GetGeyserPlayerByGamertag
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/g/Notch", nil, map[string]string{"gamertag": "Notch"}))
		testutil.ExpectStatus(t, w, http.StatusOK)
	})

	t.Run("HD-31_NotFound", func(t *testing.T) {
		svc := &mcMockService{getGeyserXUID: func(string) (*GeyserPlayer, error) { return nil, ErrPlayerNotFound }}
		h := NewHandler(svc).GetGeyserPlayerByGamertag
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/g/x", nil, map[string]string{"gamertag": "x"}))
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgPlayerNotFound)
	})

	t.Run("HD-32_InvalidGeyserRequest", func(t *testing.T) {
		svc := &mcMockService{getGeyserXUID: func(string) (*GeyserPlayer, error) { return nil, ErrInvalidGeyserRequest }}
		h := NewHandler(svc).GetGeyserPlayerByGamertag
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/g/x", nil, map[string]string{"gamertag": "x"}))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidGamertag)
	})

	t.Run("HD-33_InternalError", func(t *testing.T) {
		svc := &mcMockService{getGeyserXUID: func(string) (*GeyserPlayer, error) { return nil, testutil.ErrBoom }}
		h := NewHandler(svc).GetGeyserPlayerByGamertag
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/g/x", nil, map[string]string{"gamertag": "x"}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetGeyserXUID)
	})
}

func TestHD34to38_GetGeyserSkinHandler(t *testing.T) {
	t.Run("HD-34_NonNumericXUID", func(t *testing.T) {
		h := NewHandler(&mcMockService{}).GetGeyserSkin
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/s/abc", nil, map[string]string{"xuid": "abc"}))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidXuid)
	})

	t.Run("HD-35_OK", func(t *testing.T) {
		svc := &mcMockService{getGeyserSkin: func(xuid int64) (*GeyserSkin, error) { return &GeyserSkin{Hash: "h"}, nil }}
		h := NewHandler(svc).GetGeyserSkin
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/s/1", nil, map[string]string{"xuid": "1"}))
		testutil.ExpectStatus(t, w, http.StatusOK)
	})

	t.Run("HD-36_NotFound", func(t *testing.T) {
		svc := &mcMockService{getGeyserSkin: func(int64) (*GeyserSkin, error) { return nil, ErrSkinNotFound }}
		h := NewHandler(svc).GetGeyserSkin
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/s/1", nil, map[string]string{"xuid": "1"}))
		testutil.ExpectStatus(t, w, http.StatusNoContent)
	})

	t.Run("HD-37_InvalidGeyserRequest", func(t *testing.T) {
		svc := &mcMockService{getGeyserSkin: func(int64) (*GeyserSkin, error) { return nil, ErrInvalidGeyserRequest }}
		h := NewHandler(svc).GetGeyserSkin
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/s/1", nil, map[string]string{"xuid": "1"}))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidXuid)
	})

	t.Run("HD-38_InternalError", func(t *testing.T) {
		svc := &mcMockService{getGeyserSkin: func(int64) (*GeyserSkin, error) { return nil, testutil.ErrBoom }}
		h := NewHandler(svc).GetGeyserSkin
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/s/1", nil, map[string]string{"xuid": "1"}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetGeyserSkin)
	})
}

func TestHD39to43_GetGeyserProfileHandler(t *testing.T) {
	bedrockUUID := xuidToUUID(42)

	t.Run("HD-39_NotDerivedBedrockUUID", func(t *testing.T) {
		h := NewHandler(&mcMockService{}).GetGeyserProfile
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gp/x", nil, map[string]string{"uuid": "not-a-uuid"}))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgNotAValidBedrockUUID+"not-a-uuid")
	})

	t.Run("HD-40_OK", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfile: func(id string) (*GeyserProfile, error) {
			return &GeyserProfile{ID: id}, nil
		}}
		h := NewHandler(svc).GetGeyserProfile
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gp/"+bedrockUUID, nil, map[string]string{"uuid": bedrockUUID}))
		testutil.ExpectStatus(t, w, http.StatusOK)
	})

	t.Run("HD-41_NotFound", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfile: func(string) (*GeyserProfile, error) { return nil, ErrPlayerNotFound }}
		h := NewHandler(svc).GetGeyserProfile
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gp/"+bedrockUUID, nil, map[string]string{"uuid": bedrockUUID}))
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgPlayerNotFound)
	})

	t.Run("HD-42_InvalidGeyserRequest", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfile: func(string) (*GeyserProfile, error) { return nil, ErrInvalidGeyserRequest }}
		h := NewHandler(svc).GetGeyserProfile
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gp/"+bedrockUUID, nil, map[string]string{"uuid": bedrockUUID}))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidXuid)
	})

	t.Run("HD-43_InternalError", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfile: func(string) (*GeyserProfile, error) { return nil, testutil.ErrBoom }}
		h := NewHandler(svc).GetGeyserProfile
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gp/"+bedrockUUID, nil, map[string]string{"uuid": bedrockUUID}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetGeyserProfile)
	})
}

func TestHD44to48_GetGeyserProfileByNameHandler(t *testing.T) {
	t.Run("HD-45_OK", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfileByName: func(g string) (*GeyserProfile, error) {
			return &GeyserProfile{Name: g}, nil
		}}
		h := NewHandler(svc).GetGeyserProfileByName
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpn/Notch", nil, map[string]string{"name": "Notch"}))
		testutil.ExpectStatus(t, w, http.StatusOK)
	})

	t.Run("HD-46_NotFound", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfileByName: func(string) (*GeyserProfile, error) { return nil, ErrPlayerNotFound }}
		h := NewHandler(svc).GetGeyserProfileByName
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpn/x", nil, map[string]string{"name": "x"}))
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgPlayerNotFound)
	})

	t.Run("HD-47_InvalidGeyserRequest", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfileByName: func(string) (*GeyserProfile, error) { return nil, ErrInvalidGeyserRequest }}
		h := NewHandler(svc).GetGeyserProfileByName
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpn/x", nil, map[string]string{"name": "x"}))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidGamertag)
	})

	t.Run("HD-48_InternalError", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfileByName: func(string) (*GeyserProfile, error) { return nil, testutil.ErrBoom }}
		h := NewHandler(svc).GetGeyserProfileByName
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpn/x", nil, map[string]string{"name": "x"}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetGeyserProfile)
	})
}

func TestHD49to52_GetTextureHandler(t *testing.T) {
	t.Run("HD-50_OK", func(t *testing.T) {
		svc := &mcMockService{getTextureContent: func(hash string) (*TextureResult, error) {
			return &TextureResult{Body: io.NopCloser(strings.NewReader("bytes")), ContentType: "image/png"}, nil
		}}
		h := NewHandler(svc).GetTexture
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/t/abc", nil, map[string]string{"hash": "abc"}))
		testutil.ExpectStatus(t, w, http.StatusOK)
		if w.Body.String() != "bytes" {
			t.Errorf("body = %q, want %q", w.Body.String(), "bytes")
		}
		if got := w.Header().Get("Content-Type"); got != "image/png" {
			t.Errorf("Content-Type = %q, want image/png", got)
		}
	})

	t.Run("HD-51_NotFound", func(t *testing.T) {
		svc := &mcMockService{getTextureContent: func(string) (*TextureResult, error) { return nil, ErrTextureNotFound }}
		h := NewHandler(svc).GetTexture
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/t/abc", nil, map[string]string{"hash": "abc"}))
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgTextureNotFound)
	})

	t.Run("HD-52_ServiceError", func(t *testing.T) {
		svc := &mcMockService{getTextureContent: func(string) (*TextureResult, error) { return nil, testutil.ErrBoom }}
		h := NewHandler(svc).GetTexture
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/t/abc", nil, map[string]string{"hash": "abc"}))
		testutil.ExpectStatus(t, w, http.StatusBadGateway)
		testutil.ExpectDetail(t, w, msgFailedToGetTexture)
	})
}

func TestHD59to65_GetGeyserPlayerByXUIDHandler(t *testing.T) {
	t.Run("HD-59_DecimalXUID", func(t *testing.T) {
		var gotXUID int64
		svc := &mcMockService{getGeyserPlayerByXUID: func(xuid int64) (*GeyserPlayer, error) {
			gotXUID = xuid
			return &GeyserPlayer{}, nil
		}}
		h := NewHandler(svc).GetGeyserPlayerByXUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gx/42", nil, map[string]string{"xuid": "42"}))
		testutil.ExpectStatus(t, w, http.StatusOK)
		if gotXUID != 42 {
			t.Errorf("service xuid = %d, want 42", gotXUID)
		}
	})

	t.Run("HD-60_FloodgateUUIDXUID", func(t *testing.T) {
		var gotXUID int64
		svc := &mcMockService{getGeyserPlayerByXUID: func(xuid int64) (*GeyserPlayer, error) {
			gotXUID = xuid
			return &GeyserPlayer{}, nil
		}}
		h := NewHandler(svc).GetGeyserPlayerByXUID
		w := httptest.NewRecorder()
		bedrockUUID := xuidToUUID(42)
		h(w, mcRequest(t, http.MethodGet, "/gx/"+bedrockUUID, nil, map[string]string{"xuid": bedrockUUID}))
		testutil.ExpectStatus(t, w, http.StatusOK)
		if gotXUID != 42 {
			t.Errorf("service xuid = %d, want 42", gotXUID)
		}
	})

	t.Run("HD-61_InvalidXUID", func(t *testing.T) {
		var called bool
		svc := &mcMockService{getGeyserPlayerByXUID: func(int64) (*GeyserPlayer, error) {
			called = true
			return nil, nil
		}}
		h := NewHandler(svc).GetGeyserPlayerByXUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gx/zz", nil, map[string]string{"xuid": "zz"}))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidXuid)
		if called {
			t.Error("service was called for an invalid xuid")
		}
	})

	t.Run("HD-62_NotFound", func(t *testing.T) {
		svc := &mcMockService{getGeyserPlayerByXUID: func(int64) (*GeyserPlayer, error) { return nil, ErrPlayerNotFound }}
		h := NewHandler(svc).GetGeyserPlayerByXUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gx/42", nil, map[string]string{"xuid": "42"}))
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgPlayerNotFound)
	})

	t.Run("HD-63_InvalidGeyserRequest", func(t *testing.T) {
		svc := &mcMockService{getGeyserPlayerByXUID: func(int64) (*GeyserPlayer, error) { return nil, ErrInvalidGeyserRequest }}
		h := NewHandler(svc).GetGeyserPlayerByXUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gx/42", nil, map[string]string{"xuid": "42"}))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-64_OK", func(t *testing.T) {
		svc := &mcMockService{getGeyserPlayerByXUID: func(int64) (*GeyserPlayer, error) {
			return &GeyserPlayer{XUID: 42, Name: "Notch"}, nil
		}}
		h := NewHandler(svc).GetGeyserPlayerByXUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gx/42", nil, map[string]string{"xuid": "42"}))
		testutil.ExpectStatus(t, w, http.StatusOK)
		var got GeyserPlayer
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.XUID != 42 || got.Name != "Notch" {
			t.Errorf("body = %s, want xuid 42 and gamertag Notch", w.Body.String())
		}
	})

	t.Run("HD-65_InternalError", func(t *testing.T) {
		svc := &mcMockService{getGeyserPlayerByXUID: func(int64) (*GeyserPlayer, error) { return nil, testutil.ErrBoom }}
		h := NewHandler(svc).GetGeyserPlayerByXUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gx/42", nil, map[string]string{"xuid": "42"}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetGeyserGamertag)
	})
}

func TestHD66to72_GetGeyserProfileByXUIDHandler(t *testing.T) {
	t.Run("HD-66_DecimalXUID", func(t *testing.T) {
		var gotXUID int64
		svc := &mcMockService{getGeyserProfileByXUID: func(xuid int64) (*GeyserProfile, error) {
			gotXUID = xuid
			return &GeyserProfile{}, nil
		}}
		h := NewHandler(svc).GetGeyserProfileByXUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpx/42", nil, map[string]string{"xuid": "42"}))
		testutil.ExpectStatus(t, w, http.StatusOK)
		if gotXUID != 42 {
			t.Errorf("service xuid = %d, want 42", gotXUID)
		}
	})

	t.Run("HD-67_FloodgateUUIDXUID", func(t *testing.T) {
		var gotXUID int64
		svc := &mcMockService{getGeyserProfileByXUID: func(xuid int64) (*GeyserProfile, error) {
			gotXUID = xuid
			return &GeyserProfile{}, nil
		}}
		h := NewHandler(svc).GetGeyserProfileByXUID
		w := httptest.NewRecorder()
		bedrockUUID := xuidToUUID(42)
		h(w, mcRequest(t, http.MethodGet, "/gpx/"+bedrockUUID, nil, map[string]string{"xuid": bedrockUUID}))
		testutil.ExpectStatus(t, w, http.StatusOK)
		if gotXUID != 42 {
			t.Errorf("service xuid = %d, want 42", gotXUID)
		}
	})

	t.Run("HD-68_InvalidXUID", func(t *testing.T) {
		var called bool
		svc := &mcMockService{getGeyserProfileByXUID: func(int64) (*GeyserProfile, error) {
			called = true
			return nil, nil
		}}
		h := NewHandler(svc).GetGeyserProfileByXUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpx/zz", nil, map[string]string{"xuid": "zz"}))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
		testutil.ExpectDetail(t, w, msgInvalidXuid)
		if called {
			t.Error("service was called for an invalid xuid")
		}
	})

	t.Run("HD-69_NotFound", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfileByXUID: func(int64) (*GeyserProfile, error) { return nil, ErrPlayerNotFound }}
		h := NewHandler(svc).GetGeyserProfileByXUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpx/42", nil, map[string]string{"xuid": "42"}))
		testutil.ExpectStatus(t, w, http.StatusNotFound)
		testutil.ExpectDetail(t, w, msgPlayerNotFound)
	})

	t.Run("HD-70_InvalidGeyserRequest", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfileByXUID: func(int64) (*GeyserProfile, error) { return nil, ErrInvalidGeyserRequest }}
		h := NewHandler(svc).GetGeyserProfileByXUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpx/42", nil, map[string]string{"xuid": "42"}))
		testutil.ExpectStatus(t, w, http.StatusBadRequest)
	})

	t.Run("HD-71_OK", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfileByXUID: func(int64) (*GeyserProfile, error) {
			return &GeyserProfile{XUID: 42, Name: "Notch"}, nil
		}}
		h := NewHandler(svc).GetGeyserProfileByXUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpx/42", nil, map[string]string{"xuid": "42"}))
		testutil.ExpectStatus(t, w, http.StatusOK)
		var got GeyserProfile
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.XUID != 42 || got.Name != "Notch" {
			t.Errorf("body = %s, want xuid 42 and gamertag Notch", w.Body.String())
		}
	})

	t.Run("HD-72_InternalError", func(t *testing.T) {
		svc := &mcMockService{getGeyserProfileByXUID: func(int64) (*GeyserProfile, error) { return nil, testutil.ErrBoom }}
		h := NewHandler(svc).GetGeyserProfileByXUID
		w := httptest.NewRecorder()
		h(w, mcRequest(t, http.MethodGet, "/gpx/42", nil, map[string]string{"xuid": "42"}))
		testutil.ExpectStatus(t, w, http.StatusInternalServerError)
		testutil.ExpectDetail(t, w, msgFailedToGetGeyserProfile)
	})
}
