package minecraft

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goccy/go-json"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

var mcCtx = testutil.LogContext(context.Background())

type mcMockStore struct {
	getPlayerByUUID           func(id string) (*MojangPlayer, error)
	getPlayerByName           func(name string) (*MojangPlayer, error)
	getProfileByUUID          func(id string) (*MojangProfile, error)
	upsertPlayer              func(player *MojangPlayer, updateProfile bool) error
	getTextures               func(table, id, name string) (*TexturesValue, error)
	upsertTextures            func(table string, value *TexturesValue) error
	upsertTextureHash         func(hash string) error
	getPlayerFromCache        func(key string) (*MojangPlayer, error)
	setPlayerInCache          func(player *MojangPlayer) error
	getProfileFromCache       func(id string) (*MojangProfile, error)
	setProfileInCache         func(profile *MojangProfile) error
	getSignedProfileFromCache func(id string) (*MojangPlayer, error)
	setSignedProfileInCache   func(player *MojangPlayer) error
	isTextureInS3             func(hash string) (bool, error)
	putTextureInS3            func(hash string, body io.ReadCloser) error
	getGeyserPlayerByGamertag func(gamertag string) (*GeyserPlayer, error)
	getGeyserPlayerByUUID     func(uuid string) (*GeyserPlayer, error)
	getGeyserPlayerByXUID     func(xuid int64) (*GeyserPlayer, error)
	upsertGeyserPlayer        func(player *GeyserPlayer) error
}

var _ Store = (*mcMockStore)(nil)

func (m *mcMockStore) GetPlayerByUUID(_ context.Context, id string) (*MojangPlayer, error) {
	if m.getPlayerByUUID != nil {
		return m.getPlayerByUUID(id)
	}
	return nil, ErrPlayerNotFound
}
func (m *mcMockStore) GetPlayerByName(_ context.Context, name string) (*MojangPlayer, error) {
	if m.getPlayerByName != nil {
		return m.getPlayerByName(name)
	}
	return nil, ErrPlayerNotFound
}
func (m *mcMockStore) GetProfileByUUID(_ context.Context, id string) (*MojangProfile, error) {
	if m.getProfileByUUID != nil {
		return m.getProfileByUUID(id)
	}
	return nil, ErrPlayerNotFound
}
func (m *mcMockStore) UpsertPlayer(_ context.Context, player *MojangPlayer, updateProfile bool) error {
	if m.upsertPlayer != nil {
		return m.upsertPlayer(player, updateProfile)
	}
	return nil
}
func (m *mcMockStore) GetTextures(_ context.Context, table, id, name string) (*TexturesValue, error) {
	if m.getTextures != nil {
		return m.getTextures(table, id, name)
	}
	return nil, nil
}
func (m *mcMockStore) UpsertTextures(_ context.Context, table string, value *TexturesValue) error {
	if m.upsertTextures != nil {
		return m.upsertTextures(table, value)
	}
	return nil
}
func (m *mcMockStore) UpsertTextureHash(_ context.Context, hash string) error {
	if m.upsertTextureHash != nil {
		return m.upsertTextureHash(hash)
	}
	return nil
}
func (m *mcMockStore) GetPlayerFromCache(_ context.Context, key string) (*MojangPlayer, error) {
	if m.getPlayerFromCache != nil {
		return m.getPlayerFromCache(key)
	}
	return nil, ErrCacheMiss
}
func (m *mcMockStore) SetPlayerInCache(_ context.Context, player *MojangPlayer) error {
	if m.setPlayerInCache != nil {
		return m.setPlayerInCache(player)
	}
	return nil
}
func (m *mcMockStore) GetProfileFromCache(_ context.Context, id string) (*MojangProfile, error) {
	if m.getProfileFromCache != nil {
		return m.getProfileFromCache(id)
	}
	return nil, ErrCacheMiss
}
func (m *mcMockStore) SetProfileInCache(_ context.Context, profile *MojangProfile) error {
	if m.setProfileInCache != nil {
		return m.setProfileInCache(profile)
	}
	return nil
}
func (m *mcMockStore) GetSignedProfileFromCache(_ context.Context, id string) (*MojangPlayer, error) {
	if m.getSignedProfileFromCache != nil {
		return m.getSignedProfileFromCache(id)
	}
	return nil, ErrCacheMiss
}
func (m *mcMockStore) SetSignedProfileInCache(_ context.Context, player *MojangPlayer) error {
	if m.setSignedProfileInCache != nil {
		return m.setSignedProfileInCache(player)
	}
	return nil
}
func (m *mcMockStore) IsTextureInS3(_ context.Context, hash string) (bool, error) {
	if m.isTextureInS3 != nil {
		return m.isTextureInS3(hash)
	}
	return false, nil
}
func (m *mcMockStore) PutTextureInS3(_ context.Context, hash string, body io.ReadCloser) error {
	if m.putTextureInS3 != nil {
		return m.putTextureInS3(hash, body)
	}
	return nil
}
func (m *mcMockStore) GetGeyserPlayerByGamertag(_ context.Context, gamertag string) (*GeyserPlayer, error) {
	if m.getGeyserPlayerByGamertag != nil {
		return m.getGeyserPlayerByGamertag(gamertag)
	}
	return nil, ErrPlayerNotFound
}
func (m *mcMockStore) GetGeyserPlayerByUUID(_ context.Context, uuid string) (*GeyserPlayer, error) {
	if m.getGeyserPlayerByUUID != nil {
		return m.getGeyserPlayerByUUID(uuid)
	}
	return nil, ErrPlayerNotFound
}
func (m *mcMockStore) GetGeyserPlayerByXUID(_ context.Context, xuid int64) (*GeyserPlayer, error) {
	if m.getGeyserPlayerByXUID != nil {
		return m.getGeyserPlayerByXUID(xuid)
	}
	return nil, ErrPlayerNotFound
}
func (m *mcMockStore) UpsertGeyserPlayer(_ context.Context, player *GeyserPlayer) error {
	if m.upsertGeyserPlayer != nil {
		return m.upsertGeyserPlayer(player)
	}
	return nil
}

func mcNewService(t *testing.T, store Store, handler http.HandlerFunc) *service {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return mcServiceForServer(store, srv)
}

func mcNewUnreachableService(store Store) *service {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()
	return mcServiceForServer(store, srv)
}

func mcServiceForServer(store Store, srv *httptest.Server) *service {
	return &service{
		store:  store,
		client: srv.Client(),
		cfg: config.Minecraft{
			LookupByName:         srv.URL + "/byname/",
			LookupByUUID:         srv.URL + "/byuuid/",
			LookupBulk:           srv.URL + "/bulk",
			LookupProfile:        srv.URL + "/profile/",
			LookupTexture:        srv.URL + "/texture/",
			GeyserXUIDLookup:     srv.URL + "/xuid/",
			GeyserSkinLookup:     srv.URL + "/skin/",
			GeyserGamertagLookup: srv.URL + "/gamertag/",
		},
		s3TextureUrl: srv.URL + "/cdn/",
	}
}

func mcMustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal fixture: %v", err)
	}
	return string(data)
}

// mcNow returns the current time as a millisecond Unix timestamp, matching
// how *IsStale fields are stored.
func mcNow() int64 { return time.Now().UnixMilli() }

func mcMustProperty(t *testing.T, tv TexturesValue) Property {
	t.Helper()
	prop, err := tv.ToProperty()
	if err != nil {
		t.Fatalf("failed to build property fixture: %v", err)
	}
	return *prop
}

func TestSV01to02_NewService(t *testing.T) {
	t.Run("SV-01_NilClientUsesDefault", func(t *testing.T) {
		svc := NewService(config.Minecraft{}, &mcMockStore{}, nil, "http://cdn/")
		s, ok := svc.(*service)
		if !ok {
			t.Fatalf("NewService() returned %T, want *service", svc)
		}
		if s.client != http.DefaultClient {
			t.Error("expected http.DefaultClient when client is nil")
		}
	})

	t.Run("SV-02_ProvidedClientRetained", func(t *testing.T) {
		client := &http.Client{}
		svc := NewService(config.Minecraft{}, &mcMockStore{}, client, "http://cdn/")
		s := svc.(*service)
		if s.client != client {
			t.Error("expected the given client to be retained")
		}
		if s.s3TextureUrl != "http://cdn/" {
			t.Errorf("s3TextureUrl = %q, want %q", s.s3TextureUrl, "http://cdn/")
		}
	})
}

func TestSV03to13_GetMojangPlayerByName(t *testing.T) {
	t.Run("SV-03_CacheHit", func(t *testing.T) {
		store := &mcMockStore{getPlayerFromCache: func(string) (*MojangPlayer, error) { return &MojangPlayer{ID: "cached"}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangPlayerByName(mcCtx, "Steve")
		if err != nil || got.ID != "cached" {
			t.Errorf("got (%+v, %v), want cached player", got, err)
		}
	})

	t.Run("SV-04_CacheErrorNotNil", func(t *testing.T) {
		store := &mcMockStore{getPlayerFromCache: func(string) (*MojangPlayer, error) { return nil, testutil.ErrBoom }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		_, err := s.GetMojangPlayerByName(mcCtx, "Steve")
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})

	t.Run("SV-05_DBFreshEntry", func(t *testing.T) {
		var cached *MojangPlayer
		store := &mcMockStore{
			getPlayerByName: func(string) (*MojangPlayer, error) { return &MojangPlayer{ID: "db", LastSeen: mcNow()}, nil },
			setPlayerInCache: func(p *MojangPlayer) error {
				cached = p
				return nil
			},
		}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangPlayerByName(mcCtx, "Steve")
		if err != nil || got.ID != "db" {
			t.Errorf("got (%+v, %v), want db player", got, err)
		}
		if cached == nil || cached.ID != "db" {
			t.Error("expected the DB player to be cached")
		}
	})

	t.Run("SV-06_DBFreshSetCacheFails", func(t *testing.T) {
		store := &mcMockStore{
			getPlayerByName:  func(string) (*MojangPlayer, error) { return &MojangPlayer{ID: "db", LastSeen: mcNow()}, nil },
			setPlayerInCache: func(*MojangPlayer) error { return testutil.ErrBoom },
		}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		_, err := s.GetMojangPlayerByName(mcCtx, "Steve")
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})

	t.Run("SV-07_MojangFetchSuccess", func(t *testing.T) {
		var upserted, cached bool
		store := &mcMockStore{
			upsertPlayer:     func(*MojangPlayer, bool) error { upserted = true; return nil },
			setPlayerInCache: func(*MojangPlayer) error { cached = true; return nil },
		}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "fetched", Name: "Steve"}))
		})
		got, err := s.GetMojangPlayerByName(mcCtx, "Steve")
		if err != nil || got.ID != "fetched" {
			t.Errorf("got (%+v, %v), want fetched player", got, err)
		}
		if !upserted || !cached {
			t.Errorf("upserted=%v cached=%v, want both true", upserted, cached)
		}
	})

	t.Run("SV-08_MojangNotFound", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		_, err := s.GetMojangPlayerByName(mcCtx, "Steve")
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want ErrPlayerNotFound", err)
		}
	})

	t.Run("SV-09_MojangServerError", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.GetMojangPlayerByName(mcCtx, "Steve")
		if !errors.Is(err, ErrMojangAPI) {
			t.Fatalf("error = %v, want %v", err, ErrMojangAPI)
		}
	})

	t.Run("SV-10_MojangBodyUndecodable", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "{not json")
		})
		_, err := s.GetMojangPlayerByName(mcCtx, "Steve")
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("SV-11_UpsertPlayerFails", func(t *testing.T) {
		store := &mcMockStore{upsertPlayer: func(*MojangPlayer, bool) error { return testutil.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "fetched"}))
		})
		_, err := s.GetMojangPlayerByName(mcCtx, "Steve")
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})

	t.Run("SV-12_SetPlayerInCacheFails", func(t *testing.T) {
		store := &mcMockStore{setPlayerInCache: func(*MojangPlayer) error { return testutil.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "fetched"}))
		})
		_, err := s.GetMojangPlayerByName(mcCtx, "Steve")
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})

	t.Run("SV-13_NetworkError", func(t *testing.T) {
		s := mcNewUnreachableService(&mcMockStore{})
		_, err := s.GetMojangPlayerByName(mcCtx, "Steve")
		if err == nil {
			t.Fatal("expected a network error")
		}
	})
}

func TestSV14to18_GetMojangPlayerByUUID(t *testing.T) {
	t.Run("SV-14_CacheHit", func(t *testing.T) {
		store := &mcMockStore{getPlayerFromCache: func(string) (*MojangPlayer, error) { return &MojangPlayer{ID: "cached"}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangPlayerByUUID(mcCtx, "id")
		if err != nil || got.ID != "cached" {
			t.Errorf("got (%+v, %v), want cached player", got, err)
		}
	})

	t.Run("SV-15_CacheErrorNotNil", func(t *testing.T) {
		store := &mcMockStore{getPlayerFromCache: func(string) (*MojangPlayer, error) { return nil, testutil.ErrBoom }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		_, err := s.GetMojangPlayerByUUID(mcCtx, "id")
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})

	t.Run("SV-16_DBFreshEntry", func(t *testing.T) {
		store := &mcMockStore{getPlayerByUUID: func(string) (*MojangPlayer, error) { return &MojangPlayer{ID: "db", LastSeen: mcNow()}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangPlayerByUUID(mcCtx, "id")
		if err != nil || got.ID != "db" {
			t.Errorf("got (%+v, %v), want db player", got, err)
		}
	})

	t.Run("SV-17_MojangFetchSuccess", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "fetched"}))
		})
		got, err := s.GetMojangPlayerByUUID(mcCtx, "id")
		if err != nil || got.ID != "fetched" {
			t.Errorf("got (%+v, %v), want fetched player", got, err)
		}
	})

	t.Run("SV-18_MojangNotFound", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		_, err := s.GetMojangPlayerByUUID(mcCtx, "id")
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want ErrPlayerNotFound", err)
		}
	})
}

func TestSV19to29_GetMojangPlayersByNames(t *testing.T) {
	t.Run("SV-19_Empty", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, nil)
		_, err := s.GetMojangPlayersByNames(mcCtx, nil)
		if !errors.Is(err, ErrNoNamesProvided) {
			t.Fatalf("error = %v, want %v", err, ErrNoNamesProvided)
		}
	})

	t.Run("SV-20_TooMany", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, nil)
		names := make([]string, 11)
		for i := range names {
			names[i] = fmt.Sprintf("n%d", i)
		}
		_, err := s.GetMojangPlayersByNames(mcCtx, names)
		if !errors.Is(err, ErrBatchLimit) {
			t.Fatalf("error = %v, want %v", err, ErrBatchLimit)
		}
	})

	t.Run("SV-21_AllCacheHits", func(t *testing.T) {
		store := &mcMockStore{getPlayerFromCache: func(key string) (*MojangPlayer, error) { return &MojangPlayer{ID: key}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangPlayersByNames(mcCtx, []string{"a", "b"})
		if err != nil || len(got) != 2 {
			t.Errorf("got (%v, %v), want 2 players", got, err)
		}
	})

	t.Run("SV-22_AllDBFresh", func(t *testing.T) {
		store := &mcMockStore{getPlayerByName: func(name string) (*MojangPlayer, error) { return &MojangPlayer{ID: name, LastSeen: mcNow()}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangPlayersByNames(mcCtx, []string{"a", "b"})
		if err != nil || len(got) != 2 {
			t.Errorf("got (%v, %v), want 2 players", got, err)
		}
	})

	t.Run("SV-23_DBFreshSetCacheFails", func(t *testing.T) {
		store := &mcMockStore{
			getPlayerByName:  func(name string) (*MojangPlayer, error) { return &MojangPlayer{ID: name, LastSeen: mcNow()}, nil },
			setPlayerInCache: func(*MojangPlayer) error { return testutil.ErrBoom },
		}
		s := mcNewService(t, store, nil)
		_, err := s.GetMojangPlayersByNames(mcCtx, []string{"a"})
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})

	t.Run("SV-24_MissesFetchedFromMojang", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, []MojangPlayer{{ID: "fetched1"}, {ID: "fetched2"}}))
		})
		got, err := s.GetMojangPlayersByNames(mcCtx, []string{"a", "b"})
		if err != nil || len(got) != 2 {
			t.Errorf("got (%v, %v), want 2 fetched players", got, err)
		}
	})

	t.Run("SV-25_BulkNetworkError", func(t *testing.T) {
		s := mcNewUnreachableService(&mcMockStore{})
		_, err := s.GetMojangPlayersByNames(mcCtx, []string{"a"})
		if err == nil {
			t.Fatal("expected a network error")
		}
	})

	t.Run("SV-26_BulkServerError", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.GetMojangPlayersByNames(mcCtx, []string{"a"})
		if !errors.Is(err, ErrMojangAPI) {
			t.Fatalf("error = %v, want %v", err, ErrMojangAPI)
		}
	})

	t.Run("SV-27_BulkBodyUndecodable", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "{not json")
		})
		_, err := s.GetMojangPlayersByNames(mcCtx, []string{"a"})
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("SV-28_UpsertFailsOnFetched", func(t *testing.T) {
		store := &mcMockStore{upsertPlayer: func(*MojangPlayer, bool) error { return testutil.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, []MojangPlayer{{ID: "fetched"}}))
		})
		_, err := s.GetMojangPlayersByNames(mcCtx, []string{"a"})
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})

	t.Run("SV-29_SetCacheFailsOnFetched", func(t *testing.T) {
		store := &mcMockStore{setPlayerInCache: func(*MojangPlayer) error { return testutil.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, []MojangPlayer{{ID: "fetched"}}))
		})
		_, err := s.GetMojangPlayersByNames(mcCtx, []string{"a"})
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})
}

func TestSV30to35_GetMojangProfile(t *testing.T) {
	t.Run("SV-30_SignedCacheHit", func(t *testing.T) {
		store := &mcMockStore{getSignedProfileFromCache: func(string) (*MojangPlayer, error) { return &MojangPlayer{ID: "cached"}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangProfile(mcCtx, "id", true)
		if err != nil || got.ID != "cached" {
			t.Errorf("got (%+v, %v), want cached player", got, err)
		}
	})

	t.Run("SV-31_SignedCacheErrorNotNil", func(t *testing.T) {
		store := &mcMockStore{getSignedProfileFromCache: func(string) (*MojangPlayer, error) { return nil, testutil.ErrBoom }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		_, err := s.GetMojangProfile(mcCtx, "id", true)
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})

	t.Run("SV-32_SignedCacheMissFetchSucceeds", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "fetched", ProfileActions: []string{}}))
		})
		got, err := s.GetMojangProfile(mcCtx, "id", true)
		if err != nil || got.ID != "fetched" {
			t.Errorf("got (%+v, %v), want fetched player", got, err)
		}
	})

	t.Run("SV-33_SignedFetchFails", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
		_, err := s.GetMojangProfile(mcCtx, "id", true)
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want ErrPlayerNotFound", err)
		}
	})

	t.Run("SV-34_UnsignedResolvesAndConverts", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*MojangProfile, error) {
			return &MojangProfile{ID: "id", Name: "Steve"}, nil
		}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangProfile(mcCtx, "id", false)
		if err != nil || got.ID != "id" {
			t.Errorf("got (%+v, %v), want converted player", got, err)
		}
	})

	t.Run("SV-35_UnsignedResolveFails", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*MojangProfile, error) { return nil, testutil.ErrBoom }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		_, err := s.GetMojangProfile(mcCtx, "id", false)
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})
}

func TestSV36to39_GetProfile(t *testing.T) {
	t.Run("SV-36_RewritesSkinAndCapeURLs", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*MojangProfile, error) {
			return &MojangProfile{ID: "id", Textures: &TexturesValue{Textures: Textures{
				SKIN: &Texture{URL: "http://mojang/texture/skinhash"},
				CAPE: &Texture{URL: "http://mojang/texture/capehash"},
			}}}, nil
		}}
		s := mcNewService(t, store, nil)
		got, err := s.GetProfile(mcCtx, "id")
		if err != nil {
			t.Fatalf("GetProfile() error = %v", err)
		}
		wantSkin := s.s3TextureUrl + "texture/skinhash"
		wantCape := s.s3TextureUrl + "texture/capehash"
		if got.Textures.Textures.SKIN.URL != wantSkin {
			t.Errorf("SKIN.URL = %q, want %q", got.Textures.Textures.SKIN.URL, wantSkin)
		}
		if got.Textures.Textures.CAPE.URL != wantCape {
			t.Errorf("CAPE.URL = %q, want %q", got.Textures.Textures.CAPE.URL, wantCape)
		}
	})

	t.Run("SV-37_NilTextures", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*MojangProfile, error) { return &MojangProfile{ID: "id"}, nil }}
		s := mcNewService(t, store, nil)
		got, err := s.GetProfile(mcCtx, "id")
		if err != nil || got.Textures != nil {
			t.Errorf("got (%+v, %v), want Textures nil, no error", got, err)
		}
	})

	t.Run("SV-38_ResolveProfileFails", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*MojangProfile, error) { return nil, testutil.ErrBoom }}
		s := mcNewService(t, store, nil)
		_, err := s.GetProfile(mcCtx, "id")
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})

	t.Run("SV-39_OnlySkinRewritten", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*MojangProfile, error) {
			return &MojangProfile{ID: "id", Textures: &TexturesValue{Textures: Textures{
				SKIN: &Texture{URL: "http://mojang/texture/skinhash"},
			}}}, nil
		}}
		s := mcNewService(t, store, nil)
		got, err := s.GetProfile(mcCtx, "id")
		if err != nil {
			t.Fatalf("GetProfile() error = %v", err)
		}
		if got.Textures.Textures.CAPE != nil {
			t.Errorf("CAPE = %+v, want nil", got.Textures.Textures.CAPE)
		}
	})
}

func TestSV40to42_GetProfileByName(t *testing.T) {
	t.Run("SV-40_ResolvesNameThenProfile", func(t *testing.T) {
		store := &mcMockStore{
			getPlayerFromCache:  func(string) (*MojangPlayer, error) { return &MojangPlayer{ID: "id"}, nil },
			getProfileFromCache: func(string) (*MojangProfile, error) { return &MojangProfile{ID: "id"}, nil },
		}
		s := mcNewService(t, store, nil)
		got, err := s.GetProfileByName(mcCtx, "Steve")
		if err != nil || got.ID != "id" {
			t.Errorf("got (%+v, %v), want resolved profile", got, err)
		}
	})

	t.Run("SV-41_NameLookupFails", func(t *testing.T) {
		store := &mcMockStore{getPlayerFromCache: func(string) (*MojangPlayer, error) { return nil, ErrCacheMiss }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		_, err := s.GetProfileByName(mcCtx, "Steve")
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want ErrPlayerNotFound", err)
		}
	})

	t.Run("SV-42_ProfileLookupFailsAfterName", func(t *testing.T) {
		store := &mcMockStore{
			getPlayerFromCache:  func(string) (*MojangPlayer, error) { return &MojangPlayer{ID: "id"}, nil },
			getProfileFromCache: func(string) (*MojangProfile, error) { return nil, testutil.ErrBoom },
		}
		s := mcNewService(t, store, nil)
		_, err := s.GetProfileByName(mcCtx, "Steve")
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})
}

func TestSV43to50_ResolveProfile(t *testing.T) {
	t.Run("SV-43_CacheHit", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*MojangProfile, error) { return &MojangProfile{ID: "cached"}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.resolveProfile(mcCtx, "id")
		if err != nil || got.ID != "cached" {
			t.Errorf("got (%+v, %v), want cached profile", got, err)
		}
	})

	t.Run("SV-44_CacheErrorNotNil", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*MojangProfile, error) { return nil, testutil.ErrBoom }}
		s := mcNewService(t, store, nil)
		_, err := s.resolveProfile(mcCtx, "id")
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})

	t.Run("SV-45_DBProfileFresh", func(t *testing.T) {
		store := &mcMockStore{getProfileByUUID: func(string) (*MojangProfile, error) {
			return &MojangProfile{ID: "db", ProfileActions: []string{"a"}, LastSeen: mcNow()}, nil
		}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.resolveProfile(mcCtx, "id")
		if err != nil || got.ID != "db" {
			t.Errorf("got (%+v, %v), want db profile", got, err)
		}
	})

	t.Run("SV-46_NilProfileActionsNormalized", func(t *testing.T) {
		store := &mcMockStore{getProfileByUUID: func(string) (*MojangProfile, error) {
			return &MojangProfile{ID: "db", ProfileActions: nil, LastSeen: mcNow()}, nil
		}}
		s := mcNewService(t, store, nil)
		got, err := s.resolveProfile(mcCtx, "id")
		if err != nil || got.ProfileActions == nil || len(got.ProfileActions) != 0 {
			t.Errorf("got (%+v, %v), want a non-nil empty ProfileActions", got, err)
		}
	})

	t.Run("SV-47_SetCacheFails", func(t *testing.T) {
		store := &mcMockStore{
			getProfileByUUID:  func(string) (*MojangProfile, error) { return &MojangProfile{ID: "db", LastSeen: mcNow()}, nil },
			setProfileInCache: func(*MojangProfile) error { return testutil.ErrBoom },
		}
		s := mcNewService(t, store, nil)
		_, err := s.resolveProfile(mcCtx, "id")
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})

	t.Run("SV-48_DBErrorNotPlayerNotFoundFallsThrough", func(t *testing.T) {
		store := &mcMockStore{getProfileByUUID: func(string) (*MojangProfile, error) { return nil, testutil.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "fetched", ProfileActions: []string{}}))
		})
		got, err := s.resolveProfile(mcCtx, "id")
		if err != nil || got.ID != "fetched" {
			t.Errorf("got (%+v, %v), want fetched profile after DB error", got, err)
		}
	})

	t.Run("SV-49_DBMissFetchesFromMojang", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "fetched", ProfileActions: []string{}}))
		})
		got, err := s.resolveProfile(mcCtx, "id")
		if err != nil || got.ID != "fetched" {
			t.Errorf("got (%+v, %v), want fetched profile", got, err)
		}
	})

	t.Run("SV-50_FetchFails", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.resolveProfile(mcCtx, "id")
		if err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestSV51to63_FetchProfileFromMojang(t *testing.T) {
	t.Run("SV-51_UnsignedSuccessStoresTextures", func(t *testing.T) {
		var hashesStored []string
		var texturesStored bool
		var cachedProfile bool
		tex := TexturesValue{Textures: Textures{SKIN: &Texture{URL: "http://mojang/texture/skinhash"}, CAPE: &Texture{URL: "http://mojang/texture/capehash"}}}
		store := &mcMockStore{
			upsertTextureHash: func(hash string) error { hashesStored = append(hashesStored, hash); return nil },
			upsertTextures:    func(string, *TexturesValue) error { texturesStored = true; return nil },
			setProfileInCache: func(*MojangProfile) error { cachedProfile = true; return nil },
		}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "id", Properties: []Property{mcMustProperty(t, tex)}}))
		})
		player, profile, err := s.fetchProfileFromMojang(mcCtx, "id", false)
		if err != nil {
			t.Fatalf("fetchProfileFromMojang() error = %v", err)
		}
		if player.ID != "id" || profile.ID != "id" {
			t.Errorf("player/profile = %+v / %+v, want id=id", player, profile)
		}
		if len(hashesStored) != 2 || !texturesStored || !cachedProfile {
			t.Errorf("hashesStored=%v texturesStored=%v cachedProfile=%v", hashesStored, texturesStored, cachedProfile)
		}
	})

	t.Run("SV-52_SignedUsesUnsignedFalseQueryAndSignedCache", func(t *testing.T) {
		var gotQuery, gotPath string
		var signedCached bool
		s := mcNewService(t, &mcMockStore{setSignedProfileInCache: func(*MojangPlayer) error { signedCached = true; return nil }},
			func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.RawQuery
				gotPath = r.URL.Path
				io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "id"}))
			})
		_, _, err := s.fetchProfileFromMojang(mcCtx, "id", true)
		if err != nil {
			t.Fatalf("fetchProfileFromMojang() error = %v", err)
		}
		if gotQuery != "unsigned=false" {
			t.Errorf("query = %q, want unsigned=false", gotQuery)
		}
		if !strings.Contains(gotPath, "/profile/") {
			t.Errorf("path = %q, want the profile endpoint", gotPath)
		}
		if !signedCached {
			t.Error("expected SetSignedProfileInCache to be called")
		}
	})

	t.Run("SV-53_NetworkError", func(t *testing.T) {
		s := mcNewUnreachableService(&mcMockStore{})
		_, _, err := s.fetchProfileFromMojang(mcCtx, "id", false)
		if err == nil {
			t.Fatal("expected a network error")
		}
	})

	t.Run("SV-54_NoContentIsNotFound", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
		_, _, err := s.fetchProfileFromMojang(mcCtx, "id", false)
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want ErrPlayerNotFound", err)
		}
	})

	t.Run("SV-55_ServerError", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, _, err := s.fetchProfileFromMojang(mcCtx, "id", false)
		if !errors.Is(err, ErrMojangAPI) {
			t.Fatalf("error = %v, want %v", err, ErrMojangAPI)
		}
	})

	t.Run("SV-56_BodyUndecodable", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "{not json")
		})
		_, _, err := s.fetchProfileFromMojang(mcCtx, "id", false)
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("SV-57_NilProfileActionsNormalized", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "id", ProfileActions: nil}))
		})
		_, profile, err := s.fetchProfileFromMojang(mcCtx, "id", false)
		if err != nil || profile.ProfileActions == nil {
			t.Errorf("got (%+v, %v), want non-nil ProfileActions", profile, err)
		}
	})

	t.Run("SV-58_UpsertPlayerFails", func(t *testing.T) {
		store := &mcMockStore{upsertPlayer: func(*MojangPlayer, bool) error { return testutil.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "id"}))
		})
		_, _, err := s.fetchProfileFromMojang(mcCtx, "id", false)
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})

	t.Run("SV-59_NoTexturesSkipsStorage", func(t *testing.T) {
		called := false
		store := &mcMockStore{upsertTextures: func(string, *TexturesValue) error { called = true; return nil }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "id"}))
		})
		_, _, err := s.fetchProfileFromMojang(mcCtx, "id", false)
		if err != nil || called {
			t.Errorf("err=%v called=%v, want nil error and no texture storage", err, called)
		}
	})

	t.Run("SV-60_UpsertTextureHashFailsIsNotFatal", func(t *testing.T) {
		tex := TexturesValue{Textures: Textures{SKIN: &Texture{URL: "http://mojang/texture/skinhash"}}}
		store := &mcMockStore{upsertTextureHash: func(string) error { return testutil.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "id", Properties: []Property{mcMustProperty(t, tex)}}))
		})
		_, _, err := s.fetchProfileFromMojang(mcCtx, "id", false)
		if err != nil {
			t.Errorf("err = %v, want nil (hash failure should be logged, not fatal)", err)
		}
	})

	t.Run("SV-61_UpsertTexturesFailsIsNotFatal", func(t *testing.T) {
		tex := TexturesValue{Textures: Textures{SKIN: &Texture{URL: "http://mojang/texture/skinhash"}}}
		store := &mcMockStore{upsertTextures: func(string, *TexturesValue) error { return testutil.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "id", Properties: []Property{mcMustProperty(t, tex)}}))
		})
		_, _, err := s.fetchProfileFromMojang(mcCtx, "id", false)
		if err != nil {
			t.Errorf("err = %v, want nil (texture storage failure should be logged, not fatal)", err)
		}
	})

	t.Run("SV-62_SignedSetCacheFails", func(t *testing.T) {
		store := &mcMockStore{setSignedProfileInCache: func(*MojangPlayer) error { return testutil.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "id"}))
		})
		_, _, err := s.fetchProfileFromMojang(mcCtx, "id", true)
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})

	t.Run("SV-63_UnsignedSetCacheFails", func(t *testing.T) {
		store := &mcMockStore{setProfileInCache: func(*MojangProfile) error { return testutil.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, MojangPlayer{ID: "id"}))
		})
		_, _, err := s.fetchProfileFromMojang(mcCtx, "id", false)
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})
}

func TestSV64to71_GetGeyserPlayerByGamertag(t *testing.T) {
	t.Run("SV-64_DBFresh", func(t *testing.T) {
		store := &mcMockStore{getGeyserPlayerByGamertag: func(string) (*GeyserPlayer, error) {
			return &GeyserPlayer{Gamertag: "Notch", LastSeen: mcNow()}, nil
		}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Geyser") })
		got, err := s.GetGeyserPlayerByGamertag(mcCtx, "Notch")
		if err != nil || got.Gamertag != "Notch" {
			t.Errorf("got (%+v, %v), want db player", got, err)
		}
	})

	t.Run("SV-65_FetchSuccess", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"xuid": 42}`)
		})
		got, err := s.GetGeyserPlayerByGamertag(mcCtx, "Notch")
		if err != nil || got.XUID != 42 {
			t.Errorf("got (%+v, %v), want XUID=42", got, err)
		}
	})

	t.Run("SV-66_BadRequest", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		})
		_, err := s.GetGeyserPlayerByGamertag(mcCtx, "Notch")
		if !errors.Is(err, ErrInvalidGeyserRequest) {
			t.Errorf("err = %v, want ErrInvalidGeyserRequest", err)
		}
	})

	t.Run("SV-67_ServerError", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.GetGeyserPlayerByGamertag(mcCtx, "Notch")
		if !errors.Is(err, ErrGeyserAPI) {
			t.Fatalf("error = %v, want %v", err, ErrGeyserAPI)
		}
	})

	t.Run("SV-68_BodyUndecodable", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "{not json")
		})
		_, err := s.GetGeyserPlayerByGamertag(mcCtx, "Notch")
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("SV-69_ZeroXUIDIsNotFound", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"xuid": 0}`)
		})
		_, err := s.GetGeyserPlayerByGamertag(mcCtx, "Notch")
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want ErrPlayerNotFound", err)
		}
	})

	t.Run("SV-70_UpsertFails", func(t *testing.T) {
		store := &mcMockStore{upsertGeyserPlayer: func(*GeyserPlayer) error { return testutil.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"xuid": 42}`)
		})
		_, err := s.GetGeyserPlayerByGamertag(mcCtx, "Notch")
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})

	t.Run("SV-71_GamertagEscaped", func(t *testing.T) {
		// r.URL.Path is already unescaped by net/http; only
		// r.URL.EscapedPath() shows the form PathEscape produced.
		var gotEscapedPath string
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			gotEscapedPath = r.URL.EscapedPath()
			io.WriteString(w, `{"xuid": 42}`)
		})
		_, err := s.GetGeyserPlayerByGamertag(mcCtx, "a b#c")
		if err != nil {
			t.Fatalf("GetGeyserPlayerByGamertag() error = %v", err)
		}
		if strings.Contains(gotEscapedPath, "#") || strings.Contains(gotEscapedPath, " ") {
			t.Errorf("request path = %q, want the gamertag to be escaped", gotEscapedPath)
		}
	})
}

func TestSV79to84_GetGeyserPlayerByXUID(t *testing.T) {
	t.Run("SV-79_DBFresh", func(t *testing.T) {
		store := &mcMockStore{getGeyserPlayerByXUID: func(int64) (*GeyserPlayer, error) {
			return &GeyserPlayer{Gamertag: "db", LastSeen: mcNow()}, nil
		}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Geyser") })
		got, err := s.GetGeyserPlayerByXUID(mcCtx, 1)
		if err != nil || got.Gamertag != "db" {
			t.Errorf("got (%+v, %v), want db player", got, err)
		}
	})

	t.Run("SV-80_FetchSuccess", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"gamertag": "Notch"}`)
		})
		got, err := s.GetGeyserPlayerByXUID(mcCtx, 1)
		if err != nil || got.Gamertag != "Notch" {
			t.Errorf("got (%+v, %v), want fetched player", got, err)
		}
	})

	t.Run("SV-81_BadRequest", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		})
		_, err := s.GetGeyserPlayerByXUID(mcCtx, 1)
		if !errors.Is(err, ErrInvalidGeyserRequest) {
			t.Errorf("err = %v, want ErrInvalidGeyserRequest", err)
		}
	})

	t.Run("SV-82_ServerError", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.GetGeyserPlayerByXUID(mcCtx, 1)
		if !errors.Is(err, ErrGeyserAPI) {
			t.Fatalf("error = %v, want %v", err, ErrGeyserAPI)
		}
	})

	t.Run("SV-83_EmptyGamertagIsNotFound", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{}`)
		})
		_, err := s.GetGeyserPlayerByXUID(mcCtx, 1)
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want ErrPlayerNotFound", err)
		}
	})

	t.Run("SV-84_UpsertFails", func(t *testing.T) {
		store := &mcMockStore{upsertGeyserPlayer: func(*GeyserPlayer) error { return testutil.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"gamertag": "Notch"}`)
		})
		_, err := s.GetGeyserPlayerByXUID(mcCtx, 1)
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})
}

func TestSV93to95_GetTextureContent(t *testing.T) {
	t.Run("SV-93_PresentDelegatesToS3", func(t *testing.T) {
		store := &mcMockStore{isTextureInS3: func(string) (bool, error) { return true, nil }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "bytes")
		})
		res, err := s.GetTextureContent(mcCtx, "hash")
		if err != nil {
			t.Fatalf("GetTextureContent() error = %v", err)
		}
		defer res.Body.Close()
		data, _ := io.ReadAll(res.Body)
		if string(data) != "bytes" {
			t.Errorf("body = %q, want bytes", data)
		}
	})

	t.Run("SV-94_AbsentDelegatesToFetch", func(t *testing.T) {
		store := &mcMockStore{isTextureInS3: func(string) (bool, error) { return false, nil }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "bytes")
		})
		res, err := s.GetTextureContent(mcCtx, "hash")
		if err != nil {
			t.Fatalf("GetTextureContent() error = %v", err)
		}
		data, _ := io.ReadAll(res.Body)
		if string(data) != "bytes" {
			t.Errorf("body = %q, want bytes", data)
		}
	})

	t.Run("SV-95_IsTextureInS3Fails", func(t *testing.T) {
		store := &mcMockStore{isTextureInS3: func(string) (bool, error) { return false, testutil.ErrBoom }}
		s := mcNewService(t, store, nil)
		_, err := s.GetTextureContent(mcCtx, "hash")
		if !errors.Is(err, testutil.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testutil.ErrBoom)
		}
	})
}

func TestSV96to100_ServeFromS3(t *testing.T) {
	t.Run("SV-96_OK", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			io.WriteString(w, "bytes")
		})
		res, err := s.serveFromS3(mcCtx, "hash")
		if err != nil {
			t.Fatalf("serveFromS3() error = %v", err)
		}
		defer res.Body.Close()
		if res.ContentType != "image/png" {
			t.Errorf("ContentType = %q, want image/png", res.ContentType)
		}
	})

	t.Run("SV-97_NotFound", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		_, err := s.serveFromS3(mcCtx, "hash")
		if !errors.Is(err, ErrTextureNotFound) {
			t.Errorf("err = %v, want ErrTextureNotFound", err)
		}
	})

	t.Run("SV-98_OtherStatus", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.serveFromS3(mcCtx, "hash")
		if !errors.Is(err, ErrBadStatusS3) {
			t.Fatalf("error = %v, want %v", err, ErrBadStatusS3)
		}
	})

	t.Run("SV-99_NetworkError", func(t *testing.T) {
		s := mcNewUnreachableService(&mcMockStore{})
		_, err := s.serveFromS3(mcCtx, "hash")
		if err == nil {
			t.Fatal("expected a network error")
		}
	})

	t.Run("SV-100_MissingContentTypeDefaults", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			// Nil the header so net/http's automatic sniffing doesn't set one.
			w.Header()["Content-Type"] = nil
			io.WriteString(w, "bytes")
		})
		res, err := s.serveFromS3(mcCtx, "hash")
		if err != nil {
			t.Fatalf("serveFromS3() error = %v", err)
		}
		defer res.Body.Close()
		if res.ContentType != "image/png" {
			t.Errorf("ContentType = %q, want default image/png", res.ContentType)
		}
	})
}

func TestSV102to109_FetchAndArchive(t *testing.T) {
	t.Run("SV-102_OK", func(t *testing.T) {
		var archived bool
		var hashStored string
		store := &mcMockStore{
			putTextureInS3:    func(string, io.ReadCloser) error { archived = true; return nil },
			upsertTextureHash: func(hash string) error { hashStored = hash; return nil },
		}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			io.WriteString(w, "bytes")
		})
		res, err := s.fetchAndArchive(mcCtx, "hash")
		if err != nil {
			t.Fatalf("fetchAndArchive() error = %v", err)
		}
		data, _ := io.ReadAll(res.Body)
		if string(data) != "bytes" || !archived || hashStored != "hash" {
			t.Errorf("data=%q archived=%v hashStored=%q", data, archived, hashStored)
		}
	})

	t.Run("SV-103_NotFound", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		_, err := s.fetchAndArchive(mcCtx, "hash")
		if !errors.Is(err, ErrTextureNotFound) {
			t.Errorf("err = %v, want ErrTextureNotFound", err)
		}
	})

	t.Run("SV-104_OtherStatus", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.fetchAndArchive(mcCtx, "hash")
		if !errors.Is(err, ErrBadStatusRemote) {
			t.Fatalf("error = %v, want %v", err, ErrBadStatusRemote)
		}
	})

	t.Run("SV-105_NetworkError", func(t *testing.T) {
		s := mcNewUnreachableService(&mcMockStore{})
		_, err := s.fetchAndArchive(mcCtx, "hash")
		if err == nil {
			t.Fatal("expected a network error")
		}
	})

	t.Run("SV-106_BodyReadFails", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "1000")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "short")
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, err := hj.Hijack(); err == nil {
					conn.Close()
				}
			}
		})
		_, err := s.fetchAndArchive(mcCtx, "hash")
		if err == nil {
			t.Fatal("expected a body read error from the truncated response")
		}
	})

	t.Run("SV-107_ArchiveFailsStillServesClient", func(t *testing.T) {
		var hashCalled bool
		store := &mcMockStore{
			putTextureInS3:    func(string, io.ReadCloser) error { return testutil.ErrBoom },
			upsertTextureHash: func(string) error { hashCalled = true; return nil },
		}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "bytes")
		})
		res, err := s.fetchAndArchive(mcCtx, "hash")
		if err != nil {
			t.Fatalf("fetchAndArchive() error = %v", err)
		}
		data, _ := io.ReadAll(res.Body)
		if string(data) != "bytes" {
			t.Errorf("body = %q, want bytes despite archive failure", data)
		}
		if hashCalled {
			t.Error("UpsertTextureHash should not be called when PutTextureInS3 fails")
		}
	})

	t.Run("SV-108_HashUpsertFailsStillServesClient", func(t *testing.T) {
		store := &mcMockStore{upsertTextureHash: func(string) error { return testutil.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "bytes")
		})
		res, err := s.fetchAndArchive(mcCtx, "hash")
		if err != nil {
			t.Fatalf("fetchAndArchive() error = %v", err)
		}
		data, _ := io.ReadAll(res.Body)
		if string(data) != "bytes" {
			t.Errorf("body = %q, want bytes despite hash-upsert failure", data)
		}
	})

	t.Run("SV-109_MissingContentTypeDefaults", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			// Nil the header so net/http's automatic sniffing doesn't set one.
			w.Header()["Content-Type"] = nil
			io.WriteString(w, "bytes")
		})
		res, err := s.fetchAndArchive(mcCtx, "hash")
		if err != nil {
			t.Fatalf("fetchAndArchive() error = %v", err)
		}
		if res.ContentType != "image/png" {
			t.Errorf("ContentType = %q, want default image/png", res.ContentType)
		}
	})
}

func TestSV128to131_GeyserProfileSkinStepRejection(t *testing.T) {
	skinBadRequest := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadRequest) }

	t.Run("SV-130_ProfileResolveBadRequestStaysInvalidRequest", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, skinBadRequest)
		_, err := s.GetGeyserProfile(mcCtx, 1)
		if !errors.Is(err, ErrInvalidGeyserRequest) {
			t.Errorf("err = %v, want %v", err, ErrInvalidGeyserRequest)
		}
	})

	t.Run("SV-131_ByGamertagLookupBadRequestStaysInvalidRequest", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, skinBadRequest)
		_, err := s.GetGeyserProfileByGamertag(mcCtx, "Notch")
		if !errors.Is(err, ErrInvalidGeyserRequest) {
			t.Errorf("err = %v, want %v", err, ErrInvalidGeyserRequest)
		}
	})
}

type mcCtxKey struct{}

func mcBlockingMojang(started chan<- struct{}) http.HandlerFunc {
	var once sync.Once
	return func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		once.Do(func() { close(started) })
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
			io.WriteString(w, "[]")
		}
	}
}

func mcRequireCancelStopsCall(t *testing.T, started <-chan struct{}, call func(ctx context.Context) error) {
	t.Helper()
	ctx, cancel := context.WithCancel(mcCtx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- call(ctx) }()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("upstream was never called")
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("call kept waiting on the upstream after ctx was cancelled")
	}
}

func TestSV135to137_OutboundCallsCancelWithCtx(t *testing.T) {
	t.Run("SV-135_GetCancelsWithCtx", func(t *testing.T) {
		started := make(chan struct{})
		s := mcNewService(t, &mcMockStore{}, mcBlockingMojang(started))
		mcRequireCancelStopsCall(t, started, func(ctx context.Context) error {
			_, err := s.GetMojangPlayerByName(ctx, "Steve")
			return err
		})
	})

	t.Run("SV-137_BulkPostCancelsWithCtx", func(t *testing.T) {
		started := make(chan struct{})
		s := mcNewService(t, &mcMockStore{}, mcBlockingMojang(started))
		mcRequireCancelStopsCall(t, started, func(ctx context.Context) error {
			_, err := s.GetMojangPlayersByNames(ctx, []string{"a"})
			return err
		})
	})
}

type mcWriteCtxStore struct {
	*mcMockStore
	calls   []string
	errs    []error
	markers []any
}

func (m *mcWriteCtxStore) record(ctx context.Context, call string) {
	m.calls = append(m.calls, call)
	m.errs = append(m.errs, ctx.Err())
	m.markers = append(m.markers, ctx.Value(mcCtxKey{}))
}

func (m *mcWriteCtxStore) UpsertPlayer(ctx context.Context, player *MojangPlayer, updateProfile bool) error {
	m.record(ctx, "UpsertPlayer")
	return nil
}

func (m *mcWriteCtxStore) SetPlayerInCache(ctx context.Context, player *MojangPlayer) error {
	m.record(ctx, "SetPlayerInCache")
	return nil
}

func (m *mcWriteCtxStore) SetProfileInCache(ctx context.Context, profile *MojangProfile) error {
	m.record(ctx, "SetProfileInCache")
	return nil
}

func (m *mcWriteCtxStore) SetSignedProfileInCache(ctx context.Context, player *MojangPlayer) error {
	m.record(ctx, "SetSignedProfileInCache")
	return nil
}

func (m *mcWriteCtxStore) UpsertTextureHash(ctx context.Context, hash string) error {
	m.record(ctx, "UpsertTextureHash")
	return nil
}

func (m *mcWriteCtxStore) UpsertTextures(ctx context.Context, table string, value *TexturesValue) error {
	m.record(ctx, "UpsertTextures")
	return nil
}

func (m *mcWriteCtxStore) PutTextureInS3(ctx context.Context, hash string, body io.ReadCloser) error {
	m.record(ctx, "PutTextureInS3")
	return nil
}

type mcRoundTripFunc func(*http.Request) (*http.Response, error)

func (f mcRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// mcCancelAfterRead cancels once the whole body has been handed to the reader.
type mcCancelAfterRead struct {
	*strings.Reader
	cancel context.CancelFunc
}

func (b mcCancelAfterRead) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if b.Reader.Len() == 0 {
		b.cancel()
	}
	return n, err
}

func (mcCancelAfterRead) Close() error { return nil }

func TestSV136_WritesSurviveCtxCancel(t *testing.T) {
	t.Run("SV-136_WritesSurviveCtxCancel", func(t *testing.T) {
		store := &mcWriteCtxStore{mcMockStore: &mcMockStore{}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) {})
		ctx, cancel := context.WithCancel(context.WithValue(mcCtx, mcCtxKey{}, "marker"))
		defer cancel()
		s.client = &http.Client{Transport: mcRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			body := mcMustJSON(t, MojangPlayer{ID: "fetched", Name: "Steve"})
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       mcCancelAfterRead{Reader: strings.NewReader(body), cancel: cancel},
				Request:    r,
			}, nil
		})}

		if _, err := s.GetMojangPlayerByName(ctx, "Steve"); err != nil {
			t.Fatalf("GetMojangPlayerByName() error = %v", err)
		}
		if ctx.Err() == nil {
			t.Fatal("ctx was not cancelled before the writes")
		}
		if len(store.errs) != 2 {
			t.Fatalf("recorded %d writes, want 2 (upsert and cache set)", len(store.errs))
		}
		for i := range store.errs {
			if store.errs[i] != nil {
				t.Errorf("write %d: ctx.Err() = %v, want nil", i, store.errs[i])
			}
			if store.markers[i] != "marker" {
				t.Errorf("write %d: ctx lost its values", i)
			}
		}
	})
}

// mcMojangBody makes the service's client answer 200 with body and cancel ctx
// once the body has been read.
func mcMojangBody(s *service, body string, cancel context.CancelFunc) {
	s.client = &http.Client{Transport: mcRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       mcCancelAfterRead{Reader: strings.NewReader(body), cancel: cancel},
			Request:    r,
		}, nil
	})}
}

func mcRequireDetachedWrites(t *testing.T, ctx context.Context, store *mcWriteCtxStore, want ...string) {
	t.Helper()
	if ctx.Err() == nil {
		t.Fatal("ctx was not cancelled before the writes")
	}
	if !slices.Equal(store.calls, want) {
		t.Fatalf("recorded writes = %v, want %v", store.calls, want)
	}
	for i, call := range store.calls {
		if store.errs[i] != nil {
			t.Errorf("%s: ctx.Err() = %v, want nil", call, store.errs[i])
		}
		if store.markers[i] != "marker" {
			t.Errorf("%s: ctx lost its values", call)
		}
	}
}

func TestSV138to146_JavaWritesSurviveCtxCancel(t *testing.T) {
	newCtx := func() (context.Context, context.CancelFunc) {
		return context.WithCancel(context.WithValue(mcCtx, mcCtxKey{}, "marker"))
	}
	skinAndCape := TexturesValue{Textures: Textures{SKIN: &Texture{URL: "http://mojang/texture/skinhash"}, CAPE: &Texture{URL: "http://mojang/texture/capehash"}}}

	t.Run("SV-138_NameDBHitCachesAfterCancel", func(t *testing.T) {
		store := &mcWriteCtxStore{mcMockStore: &mcMockStore{
			getPlayerByName: func(string) (*MojangPlayer, error) {
				return &MojangPlayer{ID: "id", Name: "Steve", LastSeen: mcNow()}, nil
			},
		}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) {})
		ctx, cancel := newCtx()
		cancel()

		if _, err := s.GetMojangPlayerByName(ctx, "Steve"); err != nil {
			t.Fatalf("GetMojangPlayerByName() error = %v", err)
		}
		mcRequireDetachedWrites(t, ctx, store, "SetPlayerInCache")
	})

	t.Run("SV-139_UUIDDBHitCachesAfterCancel", func(t *testing.T) {
		store := &mcWriteCtxStore{mcMockStore: &mcMockStore{
			getPlayerByUUID: func(string) (*MojangPlayer, error) {
				return &MojangPlayer{ID: "id", Name: "Steve", LastSeen: mcNow()}, nil
			},
		}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) {})
		ctx, cancel := newCtx()
		cancel()

		if _, err := s.GetMojangPlayerByUUID(ctx, "id"); err != nil {
			t.Fatalf("GetMojangPlayerByUUID() error = %v", err)
		}
		mcRequireDetachedWrites(t, ctx, store, "SetPlayerInCache")
	})

	t.Run("SV-140_UUIDMojangFetchWritesAfterCancel", func(t *testing.T) {
		store := &mcWriteCtxStore{mcMockStore: &mcMockStore{}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) {})
		ctx, cancel := newCtx()
		defer cancel()
		mcMojangBody(s, mcMustJSON(t, MojangPlayer{ID: "id", Name: "Steve"}), cancel)

		if _, err := s.GetMojangPlayerByUUID(ctx, "id"); err != nil {
			t.Fatalf("GetMojangPlayerByUUID() error = %v", err)
		}
		mcRequireDetachedWrites(t, ctx, store, "UpsertPlayer", "SetPlayerInCache")
	})

	t.Run("SV-141_BulkDBHitCachesAfterCancel", func(t *testing.T) {
		store := &mcWriteCtxStore{mcMockStore: &mcMockStore{
			getPlayerByName: func(string) (*MojangPlayer, error) {
				return &MojangPlayer{ID: "id", Name: "Steve", LastSeen: mcNow()}, nil
			},
		}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) {})
		ctx, cancel := newCtx()
		cancel()

		if _, err := s.GetMojangPlayersByNames(ctx, []string{"Steve"}); err != nil {
			t.Fatalf("GetMojangPlayersByNames() error = %v", err)
		}
		mcRequireDetachedWrites(t, ctx, store, "SetPlayerInCache")
	})

	t.Run("SV-142_BulkMojangFetchWritesAfterCancel", func(t *testing.T) {
		store := &mcWriteCtxStore{mcMockStore: &mcMockStore{}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) {})
		ctx, cancel := newCtx()
		defer cancel()
		mcMojangBody(s, mcMustJSON(t, []MojangPlayer{{ID: "id", Name: "Steve"}}), cancel)

		if _, err := s.GetMojangPlayersByNames(ctx, []string{"Steve"}); err != nil {
			t.Fatalf("GetMojangPlayersByNames() error = %v", err)
		}
		mcRequireDetachedWrites(t, ctx, store, "UpsertPlayer", "SetPlayerInCache")
	})

	t.Run("SV-143_ProfileDBHitCachesAfterCancel", func(t *testing.T) {
		store := &mcWriteCtxStore{mcMockStore: &mcMockStore{
			getProfileByUUID: func(string) (*MojangProfile, error) {
				return &MojangProfile{ID: "id", Name: "Steve", LastSeen: mcNow()}, nil
			},
		}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) {})
		ctx, cancel := newCtx()
		cancel()

		if _, err := s.resolveProfile(ctx, "id"); err != nil {
			t.Fatalf("resolveProfile() error = %v", err)
		}
		mcRequireDetachedWrites(t, ctx, store, "SetProfileInCache")
	})

	t.Run("SV-144_UnsignedProfileFetchWritesAfterCancel", func(t *testing.T) {
		store := &mcWriteCtxStore{mcMockStore: &mcMockStore{}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) {})
		ctx, cancel := newCtx()
		defer cancel()
		mcMojangBody(s, mcMustJSON(t, MojangPlayer{ID: "id", Properties: []Property{mcMustProperty(t, skinAndCape)}}), cancel)

		if _, _, err := s.fetchProfileFromMojang(ctx, "id", false); err != nil {
			t.Fatalf("fetchProfileFromMojang() error = %v", err)
		}
		mcRequireDetachedWrites(t, ctx, store, "UpsertPlayer", "UpsertTextureHash", "UpsertTextureHash", "UpsertTextures", "SetProfileInCache")
	})

	t.Run("SV-145_SignedProfileFetchWritesAfterCancel", func(t *testing.T) {
		store := &mcWriteCtxStore{mcMockStore: &mcMockStore{}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) {})
		ctx, cancel := newCtx()
		defer cancel()
		mcMojangBody(s, mcMustJSON(t, MojangPlayer{ID: "id", Properties: []Property{mcMustProperty(t, skinAndCape)}}), cancel)

		if _, _, err := s.fetchProfileFromMojang(ctx, "id", true); err != nil {
			t.Fatalf("fetchProfileFromMojang() error = %v", err)
		}
		mcRequireDetachedWrites(t, ctx, store, "UpsertPlayer", "UpsertTextureHash", "UpsertTextureHash", "UpsertTextures", "SetSignedProfileInCache")
	})

	t.Run("SV-146_TextureArchiveWritesAfterCancel", func(t *testing.T) {
		store := &mcWriteCtxStore{mcMockStore: &mcMockStore{}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) {})
		ctx, cancel := newCtx()
		defer cancel()
		mcMojangBody(s, "bytes", cancel)

		if _, err := s.fetchAndArchive(ctx, "hash"); err != nil {
			t.Fatalf("fetchAndArchive() error = %v", err)
		}
		mcRequireDetachedWrites(t, ctx, store, "PutTextureInS3", "UpsertTextureHash")
	})
}
