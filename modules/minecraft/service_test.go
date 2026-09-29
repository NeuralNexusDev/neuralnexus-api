package minecraft

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/redis/go-redis/v9"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
)

type mcMockStore struct {
	getPlayerByUUID           func(id string) (*Player, error)
	getPlayerByName           func(name string) (*Player, error)
	getProfileByUUID          func(id string) (*Profile, error)
	upsertPlayer              func(player *Player, updateProfile bool) error
	upsertTextures            func(value *TexturesValue) error
	upsertTextureHash         func(hash string) error
	getPlayerFromCache        func(key string) (*Player, error)
	setPlayerInCache          func(player *Player) error
	getProfileFromCache       func(id string) (*Profile, error)
	setProfileInCache         func(profile *Profile) error
	getSignedProfileFromCache func(id string) (*Player, error)
	setSignedProfileInCache   func(player *Player) error
	isTextureInS3             func(hash string) (bool, error)
	putTextureInS3            func(hash string, body io.ReadCloser) error
	getGeyserPlayerByGamertag func(gamertag string) (*GeyserPlayer, error)
	getGeyserPlayerByXUID     func(xuid int64) (*GeyserPlayer, error)
	upsertGeyserPlayer        func(player *GeyserPlayer) error
	getGeyserSkin             func(xuid int64) (*GeyserSkin, error)
	getGeyserSkinByHash       func(hash string) (*GeyserSkin, error)
	upsertGeyserSkin          func(xuid int64, skin *GeyserSkin) error
	isGeyserTextureInS3       func(hash string) (bool, error)
	putGeyserTextureInS3      func(hash string, body io.ReadCloser) error
}

var _ Store = (*mcMockStore)(nil)

func (m *mcMockStore) GetPlayerByUUID(id string) (*Player, error) {
	if m.getPlayerByUUID != nil {
		return m.getPlayerByUUID(id)
	}
	return nil, ErrPlayerNotFound
}
func (m *mcMockStore) GetPlayerByName(name string) (*Player, error) {
	if m.getPlayerByName != nil {
		return m.getPlayerByName(name)
	}
	return nil, ErrPlayerNotFound
}
func (m *mcMockStore) GetProfileByUUID(id string) (*Profile, error) {
	if m.getProfileByUUID != nil {
		return m.getProfileByUUID(id)
	}
	return nil, ErrPlayerNotFound
}
func (m *mcMockStore) UpsertPlayer(player *Player, updateProfile bool) error {
	if m.upsertPlayer != nil {
		return m.upsertPlayer(player, updateProfile)
	}
	return nil
}
func (m *mcMockStore) UpsertTextures(value *TexturesValue) error {
	if m.upsertTextures != nil {
		return m.upsertTextures(value)
	}
	return nil
}
func (m *mcMockStore) UpsertTextureHash(hash string) error {
	if m.upsertTextureHash != nil {
		return m.upsertTextureHash(hash)
	}
	return nil
}
func (m *mcMockStore) GetPlayerFromCache(key string) (*Player, error) {
	if m.getPlayerFromCache != nil {
		return m.getPlayerFromCache(key)
	}
	return nil, redis.Nil
}
func (m *mcMockStore) SetPlayerInCache(player *Player) error {
	if m.setPlayerInCache != nil {
		return m.setPlayerInCache(player)
	}
	return nil
}
func (m *mcMockStore) GetProfileFromCache(id string) (*Profile, error) {
	if m.getProfileFromCache != nil {
		return m.getProfileFromCache(id)
	}
	return nil, redis.Nil
}
func (m *mcMockStore) SetProfileInCache(profile *Profile) error {
	if m.setProfileInCache != nil {
		return m.setProfileInCache(profile)
	}
	return nil
}
func (m *mcMockStore) GetSignedProfileFromCache(id string) (*Player, error) {
	if m.getSignedProfileFromCache != nil {
		return m.getSignedProfileFromCache(id)
	}
	return nil, redis.Nil
}
func (m *mcMockStore) SetSignedProfileInCache(player *Player) error {
	if m.setSignedProfileInCache != nil {
		return m.setSignedProfileInCache(player)
	}
	return nil
}
func (m *mcMockStore) IsTextureInS3(hash string) (bool, error) {
	if m.isTextureInS3 != nil {
		return m.isTextureInS3(hash)
	}
	return false, nil
}
func (m *mcMockStore) PutTextureInS3(hash string, body io.ReadCloser) error {
	if m.putTextureInS3 != nil {
		return m.putTextureInS3(hash, body)
	}
	return nil
}
func (m *mcMockStore) GetGeyserPlayerByGamertag(gamertag string) (*GeyserPlayer, error) {
	if m.getGeyserPlayerByGamertag != nil {
		return m.getGeyserPlayerByGamertag(gamertag)
	}
	return nil, ErrPlayerNotFound
}
func (m *mcMockStore) GetGeyserPlayerByXUID(xuid int64) (*GeyserPlayer, error) {
	if m.getGeyserPlayerByXUID != nil {
		return m.getGeyserPlayerByXUID(xuid)
	}
	return nil, ErrPlayerNotFound
}
func (m *mcMockStore) UpsertGeyserPlayer(player *GeyserPlayer) error {
	if m.upsertGeyserPlayer != nil {
		return m.upsertGeyserPlayer(player)
	}
	return nil
}
func (m *mcMockStore) GetGeyserSkin(xuid int64) (*GeyserSkin, error) {
	if m.getGeyserSkin != nil {
		return m.getGeyserSkin(xuid)
	}
	return nil, ErrSkinNotFound
}
func (m *mcMockStore) GetGeyserSkinByHash(hash string) (*GeyserSkin, error) {
	if m.getGeyserSkinByHash != nil {
		return m.getGeyserSkinByHash(hash)
	}
	return nil, nil
}
func (m *mcMockStore) UpsertGeyserSkin(xuid int64, skin *GeyserSkin) error {
	if m.upsertGeyserSkin != nil {
		return m.upsertGeyserSkin(xuid, skin)
	}
	return nil
}
func (m *mcMockStore) IsGeyserTextureInS3(hash string) (bool, error) {
	if m.isGeyserTextureInS3 != nil {
		return m.isGeyserTextureInS3(hash)
	}
	return false, nil
}
func (m *mcMockStore) PutGeyserTextureInS3(hash string, body io.ReadCloser) error {
	if m.putGeyserTextureInS3 != nil {
		return m.putGeyserTextureInS3(hash, body)
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
		store:                store,
		client:               srv.Client(),
		lookupByName:         srv.URL + "/byname/",
		lookupByUUID:         srv.URL + "/byuuid/",
		lookupBulk:           srv.URL + "/bulk",
		lookupProfile:        srv.URL + "/profile/",
		lookupTexture:        srv.URL + "/texture/",
		geyserXUIDLookup:     srv.URL + "/xuid/",
		geyserSkinLookup:     srv.URL + "/skin/",
		geyserGamertagLookup: srv.URL + "/gamertag/",
		nnTextureUrl:         srv.URL + "/cdn/",
		nnGeyserTextureUrl:   srv.URL + "/cdn/geyser/",
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
		svc := NewService(&mcMockStore{}, nil, "http://cdn/")
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
		svc := NewService(&mcMockStore{}, client, "http://cdn/")
		s := svc.(*service)
		if s.client != client {
			t.Error("expected the given client to be retained")
		}
		if s.nnGeyserTextureUrl != "http://cdn/geyser/" {
			t.Errorf("nnGeyserTextureUrl = %q, want %q", s.nnGeyserTextureUrl, "http://cdn/geyser/")
		}
	})
}

func TestSV03to13and128_GetMojangPlayerByName(t *testing.T) {
	t.Run("SV-03_CacheHit", func(t *testing.T) {
		store := &mcMockStore{getPlayerFromCache: func(string) (*Player, error) { return &Player{ID: "cached"}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangPlayerByName("Steve")
		if err != nil || got.ID != "cached" {
			t.Errorf("got (%+v, %v), want cached player", got, err)
		}
	})

	t.Run("SV-04_CacheErrorNotNil", func(t *testing.T) {
		store := &mcMockStore{getPlayerFromCache: func(string) (*Player, error) { return nil, testerrors.ErrBoom }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		_, err := s.GetMojangPlayerByName("Steve")
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})

	t.Run("SV-05_DBFreshEntry", func(t *testing.T) {
		var cached *Player
		store := &mcMockStore{
			getPlayerByName: func(string) (*Player, error) { return &Player{ID: "db", LastSeen: mcNow()}, nil },
			setPlayerInCache: func(p *Player) error {
				cached = p
				return nil
			},
		}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangPlayerByName("Steve")
		if err != nil || got.ID != "db" {
			t.Errorf("got (%+v, %v), want db player", got, err)
		}
		if cached == nil || cached.ID != "db" {
			t.Error("expected the DB player to be cached")
		}
	})

	t.Run("SV-06_DBFreshSetCacheFails", func(t *testing.T) {
		store := &mcMockStore{
			getPlayerByName:  func(string) (*Player, error) { return &Player{ID: "db", LastSeen: mcNow()}, nil },
			setPlayerInCache: func(*Player) error { return testerrors.ErrBoom },
		}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		_, err := s.GetMojangPlayerByName("Steve")
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})

	t.Run("SV-07_MojangFetchSuccess", func(t *testing.T) {
		var upserted, cached bool
		store := &mcMockStore{
			upsertPlayer:     func(*Player, bool) error { upserted = true; return nil },
			setPlayerInCache: func(*Player) error { cached = true; return nil },
		}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "fetched", Name: "Steve"}))
		})
		got, err := s.GetMojangPlayerByName("Steve")
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
		_, err := s.GetMojangPlayerByName("Steve")
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want ErrPlayerNotFound", err)
		}
	})

	t.Run("SV-09_MojangServerError", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.GetMojangPlayerByName("Steve")
		if !errors.Is(err, ErrMojangAPI) {
			t.Fatalf("error = %v, want %v", err, ErrMojangAPI)
		}
	})

	t.Run("SV-10_MojangBodyUndecodable", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "{not json")
		})
		_, err := s.GetMojangPlayerByName("Steve")
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("SV-11_UpsertPlayerFails", func(t *testing.T) {
		store := &mcMockStore{upsertPlayer: func(*Player, bool) error { return testerrors.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "fetched"}))
		})
		_, err := s.GetMojangPlayerByName("Steve")
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})

	t.Run("SV-12_SetPlayerInCacheFails", func(t *testing.T) {
		store := &mcMockStore{setPlayerInCache: func(*Player) error { return testerrors.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "fetched"}))
		})
		_, err := s.GetMojangPlayerByName("Steve")
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})

	t.Run("SV-13_NetworkError", func(t *testing.T) {
		s := mcNewUnreachableService(&mcMockStore{})
		_, err := s.GetMojangPlayerByName("Steve")
		if err == nil {
			t.Fatal("expected a network error")
		}
	})

	t.Run("SV-128_MojangServerErrorMessage", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.GetMojangPlayerByName("Steve")
		want := "mojang API error: 500 Internal Server Error"
		if err == nil || err.Error() != want {
			t.Errorf("error = %v, want %q", err, want)
		}
	})
}

func TestSV14to18_GetMojangPlayerByUUID(t *testing.T) {
	t.Run("SV-14_CacheHit", func(t *testing.T) {
		store := &mcMockStore{getPlayerFromCache: func(string) (*Player, error) { return &Player{ID: "cached"}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangPlayerByUUID("id")
		if err != nil || got.ID != "cached" {
			t.Errorf("got (%+v, %v), want cached player", got, err)
		}
	})

	t.Run("SV-15_CacheErrorNotNil", func(t *testing.T) {
		store := &mcMockStore{getPlayerFromCache: func(string) (*Player, error) { return nil, testerrors.ErrBoom }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		_, err := s.GetMojangPlayerByUUID("id")
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})

	t.Run("SV-16_DBFreshEntry", func(t *testing.T) {
		store := &mcMockStore{getPlayerByUUID: func(string) (*Player, error) { return &Player{ID: "db", LastSeen: mcNow()}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangPlayerByUUID("id")
		if err != nil || got.ID != "db" {
			t.Errorf("got (%+v, %v), want db player", got, err)
		}
	})

	t.Run("SV-17_MojangFetchSuccess", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "fetched"}))
		})
		got, err := s.GetMojangPlayerByUUID("id")
		if err != nil || got.ID != "fetched" {
			t.Errorf("got (%+v, %v), want fetched player", got, err)
		}
	})

	t.Run("SV-18_MojangNotFound", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		_, err := s.GetMojangPlayerByUUID("id")
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want ErrPlayerNotFound", err)
		}
	})
}

func TestSV19to29_GetMojangPlayersByNames(t *testing.T) {
	t.Run("SV-19_Empty", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, nil)
		_, err := s.GetMojangPlayersByNames(nil)
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
		_, err := s.GetMojangPlayersByNames(names)
		if !errors.Is(err, ErrBatchLimit) {
			t.Fatalf("error = %v, want %v", err, ErrBatchLimit)
		}
	})

	t.Run("SV-21_AllCacheHits", func(t *testing.T) {
		store := &mcMockStore{getPlayerFromCache: func(key string) (*Player, error) { return &Player{ID: key}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangPlayersByNames([]string{"a", "b"})
		if err != nil || len(got) != 2 {
			t.Errorf("got (%v, %v), want 2 players", got, err)
		}
	})

	t.Run("SV-22_AllDBFresh", func(t *testing.T) {
		store := &mcMockStore{getPlayerByName: func(name string) (*Player, error) { return &Player{ID: name, LastSeen: mcNow()}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangPlayersByNames([]string{"a", "b"})
		if err != nil || len(got) != 2 {
			t.Errorf("got (%v, %v), want 2 players", got, err)
		}
	})

	t.Run("SV-23_DBFreshSetCacheFails", func(t *testing.T) {
		store := &mcMockStore{
			getPlayerByName:  func(name string) (*Player, error) { return &Player{ID: name, LastSeen: mcNow()}, nil },
			setPlayerInCache: func(*Player) error { return testerrors.ErrBoom },
		}
		s := mcNewService(t, store, nil)
		_, err := s.GetMojangPlayersByNames([]string{"a"})
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})

	t.Run("SV-24_MissesFetchedFromMojang", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, []Player{{ID: "fetched1"}, {ID: "fetched2"}}))
		})
		got, err := s.GetMojangPlayersByNames([]string{"a", "b"})
		if err != nil || len(got) != 2 {
			t.Errorf("got (%v, %v), want 2 fetched players", got, err)
		}
	})

	t.Run("SV-25_BulkNetworkError", func(t *testing.T) {
		s := mcNewUnreachableService(&mcMockStore{})
		_, err := s.GetMojangPlayersByNames([]string{"a"})
		if err == nil {
			t.Fatal("expected a network error")
		}
	})

	t.Run("SV-26_BulkServerError", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.GetMojangPlayersByNames([]string{"a"})
		if !errors.Is(err, ErrMojangAPI) {
			t.Fatalf("error = %v, want %v", err, ErrMojangAPI)
		}
	})

	t.Run("SV-27_BulkBodyUndecodable", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "{not json")
		})
		_, err := s.GetMojangPlayersByNames([]string{"a"})
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("SV-28_UpsertFailsOnFetched", func(t *testing.T) {
		store := &mcMockStore{upsertPlayer: func(*Player, bool) error { return testerrors.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, []Player{{ID: "fetched"}}))
		})
		_, err := s.GetMojangPlayersByNames([]string{"a"})
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})

	t.Run("SV-29_SetCacheFailsOnFetched", func(t *testing.T) {
		store := &mcMockStore{setPlayerInCache: func(*Player) error { return testerrors.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, []Player{{ID: "fetched"}}))
		})
		_, err := s.GetMojangPlayersByNames([]string{"a"})
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})
}

func TestSV30to35_GetMojangProfile(t *testing.T) {
	t.Run("SV-30_SignedCacheHit", func(t *testing.T) {
		store := &mcMockStore{getSignedProfileFromCache: func(string) (*Player, error) { return &Player{ID: "cached"}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangProfile("id", true)
		if err != nil || got.ID != "cached" {
			t.Errorf("got (%+v, %v), want cached player", got, err)
		}
	})

	t.Run("SV-31_SignedCacheErrorNotNil", func(t *testing.T) {
		store := &mcMockStore{getSignedProfileFromCache: func(string) (*Player, error) { return nil, testerrors.ErrBoom }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		_, err := s.GetMojangProfile("id", true)
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})

	t.Run("SV-32_SignedCacheMissFetchSucceeds", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "fetched", ProfileActions: []string{}}))
		})
		got, err := s.GetMojangProfile("id", true)
		if err != nil || got.ID != "fetched" {
			t.Errorf("got (%+v, %v), want fetched player", got, err)
		}
	})

	t.Run("SV-33_SignedFetchFails", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
		_, err := s.GetMojangProfile("id", true)
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want ErrPlayerNotFound", err)
		}
	})

	t.Run("SV-34_UnsignedResolvesAndConverts", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*Profile, error) {
			return &Profile{ID: "id", Name: "Steve"}, nil
		}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.GetMojangProfile("id", false)
		if err != nil || got.ID != "id" {
			t.Errorf("got (%+v, %v), want converted player", got, err)
		}
	})

	t.Run("SV-35_UnsignedResolveFails", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*Profile, error) { return nil, testerrors.ErrBoom }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		_, err := s.GetMojangProfile("id", false)
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})
}

func TestSV36to39_GetProfile(t *testing.T) {
	t.Run("SV-36_RewritesSkinAndCapeURLs", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*Profile, error) {
			return &Profile{ID: "id", Textures: &TexturesValue{Textures: Textures{
				SKIN: &Texture{URL: "http://mojang/texture/skinhash"},
				CAPE: &Texture{URL: "http://mojang/texture/capehash"},
			}}}, nil
		}}
		s := mcNewService(t, store, nil)
		got, err := s.GetProfile("id")
		if err != nil {
			t.Fatalf("GetProfile() error = %v", err)
		}
		wantSkin := s.nnTextureUrl + "skinhash"
		wantCape := s.nnTextureUrl + "capehash"
		if got.Textures.Textures.SKIN.URL != wantSkin {
			t.Errorf("SKIN.URL = %q, want %q", got.Textures.Textures.SKIN.URL, wantSkin)
		}
		if got.Textures.Textures.CAPE.URL != wantCape {
			t.Errorf("CAPE.URL = %q, want %q", got.Textures.Textures.CAPE.URL, wantCape)
		}
	})

	t.Run("SV-37_NilTextures", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*Profile, error) { return &Profile{ID: "id"}, nil }}
		s := mcNewService(t, store, nil)
		got, err := s.GetProfile("id")
		if err != nil || got.Textures != nil {
			t.Errorf("got (%+v, %v), want Textures nil, no error", got, err)
		}
	})

	t.Run("SV-38_ResolveProfileFails", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*Profile, error) { return nil, testerrors.ErrBoom }}
		s := mcNewService(t, store, nil)
		_, err := s.GetProfile("id")
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})

	t.Run("SV-39_OnlySkinRewritten", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*Profile, error) {
			return &Profile{ID: "id", Textures: &TexturesValue{Textures: Textures{
				SKIN: &Texture{URL: "http://mojang/texture/skinhash"},
			}}}, nil
		}}
		s := mcNewService(t, store, nil)
		got, err := s.GetProfile("id")
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
			getPlayerFromCache:  func(string) (*Player, error) { return &Player{ID: "id"}, nil },
			getProfileFromCache: func(string) (*Profile, error) { return &Profile{ID: "id"}, nil },
		}
		s := mcNewService(t, store, nil)
		got, err := s.GetProfileByName("Steve")
		if err != nil || got.ID != "id" {
			t.Errorf("got (%+v, %v), want resolved profile", got, err)
		}
	})

	t.Run("SV-41_NameLookupFails", func(t *testing.T) {
		store := &mcMockStore{getPlayerFromCache: func(string) (*Player, error) { return nil, redis.Nil }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		_, err := s.GetProfileByName("Steve")
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want ErrPlayerNotFound", err)
		}
	})

	t.Run("SV-42_ProfileLookupFailsAfterName", func(t *testing.T) {
		store := &mcMockStore{
			getPlayerFromCache:  func(string) (*Player, error) { return &Player{ID: "id"}, nil },
			getProfileFromCache: func(string) (*Profile, error) { return nil, testerrors.ErrBoom },
		}
		s := mcNewService(t, store, nil)
		_, err := s.GetProfileByName("Steve")
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})
}

func TestSV43to50_ResolveProfile(t *testing.T) {
	t.Run("SV-43_CacheHit", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*Profile, error) { return &Profile{ID: "cached"}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.resolveProfile("id")
		if err != nil || got.ID != "cached" {
			t.Errorf("got (%+v, %v), want cached profile", got, err)
		}
	})

	t.Run("SV-44_CacheErrorNotNil", func(t *testing.T) {
		store := &mcMockStore{getProfileFromCache: func(string) (*Profile, error) { return nil, testerrors.ErrBoom }}
		s := mcNewService(t, store, nil)
		_, err := s.resolveProfile("id")
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})

	t.Run("SV-45_DBProfileFresh", func(t *testing.T) {
		store := &mcMockStore{getProfileByUUID: func(string) (*Profile, error) {
			return &Profile{ID: "db", ProfileActions: []string{"a"}, LastSeen: mcNow()}, nil
		}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Mojang") })
		got, err := s.resolveProfile("id")
		if err != nil || got.ID != "db" {
			t.Errorf("got (%+v, %v), want db profile", got, err)
		}
	})

	t.Run("SV-46_NilProfileActionsNormalized", func(t *testing.T) {
		store := &mcMockStore{getProfileByUUID: func(string) (*Profile, error) {
			return &Profile{ID: "db", ProfileActions: nil, LastSeen: mcNow()}, nil
		}}
		s := mcNewService(t, store, nil)
		got, err := s.resolveProfile("id")
		if err != nil || got.ProfileActions == nil || len(got.ProfileActions) != 0 {
			t.Errorf("got (%+v, %v), want a non-nil empty ProfileActions", got, err)
		}
	})

	t.Run("SV-47_SetCacheFails", func(t *testing.T) {
		store := &mcMockStore{
			getProfileByUUID:  func(string) (*Profile, error) { return &Profile{ID: "db", LastSeen: mcNow()}, nil },
			setProfileInCache: func(*Profile) error { return testerrors.ErrBoom },
		}
		s := mcNewService(t, store, nil)
		_, err := s.resolveProfile("id")
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})

	t.Run("SV-48_DBErrorNotPlayerNotFoundFallsThrough", func(t *testing.T) {
		store := &mcMockStore{getProfileByUUID: func(string) (*Profile, error) { return nil, testerrors.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "fetched", ProfileActions: []string{}}))
		})
		got, err := s.resolveProfile("id")
		if err != nil || got.ID != "fetched" {
			t.Errorf("got (%+v, %v), want fetched profile after DB error", got, err)
		}
	})

	t.Run("SV-49_DBMissFetchesFromMojang", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "fetched", ProfileActions: []string{}}))
		})
		got, err := s.resolveProfile("id")
		if err != nil || got.ID != "fetched" {
			t.Errorf("got (%+v, %v), want fetched profile", got, err)
		}
	})

	t.Run("SV-50_FetchFails", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.resolveProfile("id")
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
			upsertTextures:    func(*TexturesValue) error { texturesStored = true; return nil },
			setProfileInCache: func(*Profile) error { cachedProfile = true; return nil },
		}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "id", Properties: []Property{mcMustProperty(t, tex)}}))
		})
		player, profile, err := s.fetchProfileFromMojang("id", false)
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
		s := mcNewService(t, &mcMockStore{setSignedProfileInCache: func(*Player) error { signedCached = true; return nil }},
			func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.RawQuery
				gotPath = r.URL.Path
				io.WriteString(w, mcMustJSON(t, Player{ID: "id"}))
			})
		_, _, err := s.fetchProfileFromMojang("id", true)
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
		_, _, err := s.fetchProfileFromMojang("id", false)
		if err == nil {
			t.Fatal("expected a network error")
		}
	})

	t.Run("SV-54_NoContentIsNotFound", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
		_, _, err := s.fetchProfileFromMojang("id", false)
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want ErrPlayerNotFound", err)
		}
	})

	t.Run("SV-55_ServerError", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, _, err := s.fetchProfileFromMojang("id", false)
		if !errors.Is(err, ErrMojangAPI) {
			t.Fatalf("error = %v, want %v", err, ErrMojangAPI)
		}
	})

	t.Run("SV-56_BodyUndecodable", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "{not json")
		})
		_, _, err := s.fetchProfileFromMojang("id", false)
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("SV-57_NilProfileActionsNormalized", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "id", ProfileActions: nil}))
		})
		_, profile, err := s.fetchProfileFromMojang("id", false)
		if err != nil || profile.ProfileActions == nil {
			t.Errorf("got (%+v, %v), want non-nil ProfileActions", profile, err)
		}
	})

	t.Run("SV-58_UpsertPlayerFails", func(t *testing.T) {
		store := &mcMockStore{upsertPlayer: func(*Player, bool) error { return testerrors.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "id"}))
		})
		_, _, err := s.fetchProfileFromMojang("id", false)
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})

	t.Run("SV-59_NoTexturesSkipsStorage", func(t *testing.T) {
		called := false
		store := &mcMockStore{upsertTextures: func(*TexturesValue) error { called = true; return nil }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "id"}))
		})
		_, _, err := s.fetchProfileFromMojang("id", false)
		if err != nil || called {
			t.Errorf("err=%v called=%v, want nil error and no texture storage", err, called)
		}
	})

	t.Run("SV-60_UpsertTextureHashFailsIsNotFatal", func(t *testing.T) {
		tex := TexturesValue{Textures: Textures{SKIN: &Texture{URL: "http://mojang/texture/skinhash"}}}
		store := &mcMockStore{upsertTextureHash: func(string) error { return testerrors.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "id", Properties: []Property{mcMustProperty(t, tex)}}))
		})
		_, _, err := s.fetchProfileFromMojang("id", false)
		if err != nil {
			t.Errorf("err = %v, want nil (hash failure should be logged, not fatal)", err)
		}
	})

	t.Run("SV-61_UpsertTexturesFailsIsNotFatal", func(t *testing.T) {
		tex := TexturesValue{Textures: Textures{SKIN: &Texture{URL: "http://mojang/texture/skinhash"}}}
		store := &mcMockStore{upsertTextures: func(*TexturesValue) error { return testerrors.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "id", Properties: []Property{mcMustProperty(t, tex)}}))
		})
		_, _, err := s.fetchProfileFromMojang("id", false)
		if err != nil {
			t.Errorf("err = %v, want nil (texture storage failure should be logged, not fatal)", err)
		}
	})

	t.Run("SV-62_SignedSetCacheFails", func(t *testing.T) {
		store := &mcMockStore{setSignedProfileInCache: func(*Player) error { return testerrors.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "id"}))
		})
		_, _, err := s.fetchProfileFromMojang("id", true)
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})

	t.Run("SV-63_UnsignedSetCacheFails", func(t *testing.T) {
		store := &mcMockStore{setProfileInCache: func(*Profile) error { return testerrors.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, Player{ID: "id"}))
		})
		_, _, err := s.fetchProfileFromMojang("id", false)
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})
}

func TestSV64to71_GetGeyserXUID(t *testing.T) {
	t.Run("SV-64_DBFresh", func(t *testing.T) {
		store := &mcMockStore{getGeyserPlayerByGamertag: func(string) (*GeyserPlayer, error) {
			return &GeyserPlayer{Gamertag: "Notch", LastSeen: mcNow()}, nil
		}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Geyser") })
		got, err := s.GetGeyserXUID("Notch")
		if err != nil || got.Gamertag != "Notch" {
			t.Errorf("got (%+v, %v), want db player", got, err)
		}
	})

	t.Run("SV-65_FetchSuccess", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"xuid": 42}`)
		})
		got, err := s.GetGeyserXUID("Notch")
		if err != nil || got.XUID != 42 {
			t.Errorf("got (%+v, %v), want XUID=42", got, err)
		}
	})

	t.Run("SV-66_BadRequest", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		})
		_, err := s.GetGeyserXUID("Notch")
		if !errors.Is(err, ErrInvalidGeyserRequest) {
			t.Errorf("err = %v, want ErrInvalidGeyserRequest", err)
		}
	})

	t.Run("SV-67_ServerError", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.GetGeyserXUID("Notch")
		if !errors.Is(err, ErrGeyserAPI) {
			t.Fatalf("error = %v, want %v", err, ErrGeyserAPI)
		}
	})

	t.Run("SV-68_BodyUndecodable", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "{not json")
		})
		_, err := s.GetGeyserXUID("Notch")
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("SV-69_ZeroXUIDIsNotFound", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"xuid": 0}`)
		})
		_, err := s.GetGeyserXUID("Notch")
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want ErrPlayerNotFound", err)
		}
	})

	t.Run("SV-70_UpsertFails", func(t *testing.T) {
		store := &mcMockStore{upsertGeyserPlayer: func(*GeyserPlayer) error { return testerrors.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"xuid": 42}`)
		})
		_, err := s.GetGeyserXUID("Notch")
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
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
		_, err := s.GetGeyserXUID("a b#c")
		if err != nil {
			t.Fatalf("GetGeyserXUID() error = %v", err)
		}
		if strings.Contains(gotEscapedPath, "#") || strings.Contains(gotEscapedPath, " ") {
			t.Errorf("request path = %q, want the gamertag to be escaped", gotEscapedPath)
		}
	})
}

func TestSV72to78_GetGeyserSkin(t *testing.T) {
	t.Run("SV-72_DBFresh", func(t *testing.T) {
		store := &mcMockStore{getGeyserSkin: func(int64) (*GeyserSkin, error) { return &GeyserSkin{Hash: "db", LastSeen: mcNow()}, nil }}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Geyser") })
		got, err := s.GetGeyserSkin(1)
		if err != nil || got.Hash != "db" {
			t.Errorf("got (%+v, %v), want db skin", got, err)
		}
	})

	t.Run("SV-73_FetchSuccess", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, GeyserSkin{Hash: "fetched"}))
		})
		got, err := s.GetGeyserSkin(1)
		if err != nil || got.Hash != "fetched" {
			t.Errorf("got (%+v, %v), want fetched skin", got, err)
		}
	})

	t.Run("SV-74_BadRequest", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		})
		_, err := s.GetGeyserSkin(1)
		if !errors.Is(err, ErrInvalidGeyserRequest) {
			t.Errorf("err = %v, want ErrInvalidGeyserRequest", err)
		}
	})

	t.Run("SV-75_ServerError", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.GetGeyserSkin(1)
		if !errors.Is(err, ErrGeyserAPI) {
			t.Fatalf("error = %v, want %v", err, ErrGeyserAPI)
		}
	})

	t.Run("SV-76_BodyUndecodable", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "{not json")
		})
		_, err := s.GetGeyserSkin(1)
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("SV-77_EmptyHashIsSkinNotFound", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{}`)
		})
		_, err := s.GetGeyserSkin(1)
		if !errors.Is(err, ErrSkinNotFound) {
			t.Errorf("err = %v, want ErrSkinNotFound", err)
		}
	})

	t.Run("SV-78_UpsertFails", func(t *testing.T) {
		store := &mcMockStore{upsertGeyserSkin: func(int64, *GeyserSkin) error { return testerrors.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, mcMustJSON(t, GeyserSkin{Hash: "fetched"}))
		})
		_, err := s.GetGeyserSkin(1)
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})
}

func TestSV79to84_ResolveGeyserPlayerByXUID(t *testing.T) {
	t.Run("SV-79_DBFresh", func(t *testing.T) {
		store := &mcMockStore{getGeyserPlayerByXUID: func(int64) (*GeyserPlayer, error) {
			return &GeyserPlayer{Gamertag: "db", LastSeen: mcNow()}, nil
		}}
		s := mcNewService(t, store, func(http.ResponseWriter, *http.Request) { t.Fatal("should not call Geyser") })
		got, err := s.resolveGeyserPlayerByXUID(1)
		if err != nil || got.Gamertag != "db" {
			t.Errorf("got (%+v, %v), want db player", got, err)
		}
	})

	t.Run("SV-80_FetchSuccess", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"gamertag": "Notch"}`)
		})
		got, err := s.resolveGeyserPlayerByXUID(1)
		if err != nil || got.Gamertag != "Notch" {
			t.Errorf("got (%+v, %v), want fetched player", got, err)
		}
	})

	t.Run("SV-81_BadRequest", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		})
		_, err := s.resolveGeyserPlayerByXUID(1)
		if !errors.Is(err, ErrInvalidGeyserRequest) {
			t.Errorf("err = %v, want ErrInvalidGeyserRequest", err)
		}
	})

	t.Run("SV-82_ServerError", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.resolveGeyserPlayerByXUID(1)
		if !errors.Is(err, ErrGeyserAPI) {
			t.Fatalf("error = %v, want %v", err, ErrGeyserAPI)
		}
	})

	t.Run("SV-83_EmptyGamertagIsNotFound", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{}`)
		})
		_, err := s.resolveGeyserPlayerByXUID(1)
		if !errors.Is(err, ErrPlayerNotFound) {
			t.Errorf("err = %v, want ErrPlayerNotFound", err)
		}
	})

	t.Run("SV-84_UpsertFails", func(t *testing.T) {
		store := &mcMockStore{upsertGeyserPlayer: func(*GeyserPlayer) error { return testerrors.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, `{"gamertag": "Notch"}`)
		})
		_, err := s.resolveGeyserPlayerByXUID(1)
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})
}

func TestSV85to88_GetGeyserProfile(t *testing.T) {
	t.Run("SV-85_ComposesPlayerAndSkin", func(t *testing.T) {
		store := &mcMockStore{
			getGeyserPlayerByXUID: func(int64) (*GeyserPlayer, error) { return &GeyserPlayer{Gamertag: "Notch", LastSeen: mcNow()}, nil },
			getGeyserSkin:         func(int64) (*GeyserSkin, error) { return &GeyserSkin{Hash: "h", LastSeen: mcNow()}, nil },
		}
		s := mcNewService(t, store, nil)
		got, err := s.GetGeyserProfile(1)
		if err != nil || got.Skin == nil || got.Skin.Hash != "h" {
			t.Errorf("got (%+v, %v), want combined profile with skin", got, err)
		}
	})

	t.Run("SV-86_NoSkinYet", func(t *testing.T) {
		store := &mcMockStore{getGeyserPlayerByXUID: func(int64) (*GeyserPlayer, error) { return &GeyserPlayer{Gamertag: "Notch", LastSeen: mcNow()}, nil }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) })
		got, err := s.GetGeyserProfile(1)
		if err != nil || got.Skin != nil {
			t.Errorf("got (%+v, %v), want Skin nil, no error", got, err)
		}
	})

	t.Run("SV-87_PlayerResolveFails", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.GetGeyserProfile(1)
		if !errors.Is(err, ErrGeyserAPI) {
			t.Fatalf("error = %v, want %v", err, ErrGeyserAPI)
		}
	})

	t.Run("SV-88_SkinFailsWithOtherError", func(t *testing.T) {
		store := &mcMockStore{
			getGeyserPlayerByXUID: func(int64) (*GeyserPlayer, error) { return &GeyserPlayer{Gamertag: "Notch", LastSeen: mcNow()}, nil },
			getGeyserSkin:         func(int64) (*GeyserSkin, error) { return nil, testerrors.ErrBoom },
		}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.GetGeyserProfile(1)
		if err == nil || errors.Is(err, ErrSkinNotFound) {
			t.Errorf("err = %v, want a non-nil error other than ErrSkinNotFound", err)
		}
	})
}

func TestSV89to92_GetGeyserProfileByGamertag(t *testing.T) {
	t.Run("SV-89_ComposesPlayerAndSkin", func(t *testing.T) {
		store := &mcMockStore{
			getGeyserPlayerByGamertag: func(string) (*GeyserPlayer, error) { return &GeyserPlayer{XUID: 1, LastSeen: mcNow()}, nil },
			getGeyserSkin:             func(int64) (*GeyserSkin, error) { return &GeyserSkin{Hash: "h", LastSeen: mcNow()}, nil },
		}
		s := mcNewService(t, store, nil)
		got, err := s.GetGeyserProfileByGamertag("Notch")
		if err != nil || got.Skin == nil {
			t.Errorf("got (%+v, %v), want combined profile with skin", got, err)
		}
	})

	t.Run("SV-90_NoSkinYet", func(t *testing.T) {
		store := &mcMockStore{getGeyserPlayerByGamertag: func(string) (*GeyserPlayer, error) { return &GeyserPlayer{XUID: 1, LastSeen: mcNow()}, nil }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{}`) })
		got, err := s.GetGeyserProfileByGamertag("Notch")
		if err != nil || got.Skin != nil {
			t.Errorf("got (%+v, %v), want Skin nil", got, err)
		}
	})

	t.Run("SV-91_XUIDLookupFails", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.GetGeyserProfileByGamertag("Notch")
		if !errors.Is(err, ErrGeyserAPI) {
			t.Fatalf("error = %v, want %v", err, ErrGeyserAPI)
		}
	})

	t.Run("SV-92_SkinFailsWithOtherError", func(t *testing.T) {
		store := &mcMockStore{
			getGeyserPlayerByGamertag: func(string) (*GeyserPlayer, error) { return &GeyserPlayer{XUID: 1, LastSeen: mcNow()}, nil },
			getGeyserSkin:             func(int64) (*GeyserSkin, error) { return nil, testerrors.ErrBoom },
		}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.GetGeyserProfileByGamertag("Notch")
		if err == nil || errors.Is(err, ErrSkinNotFound) {
			t.Errorf("err = %v, want a non-nil error other than ErrSkinNotFound", err)
		}
	})
}

func TestSV93to95_GetTextureContent(t *testing.T) {
	t.Run("SV-93_PresentDelegatesToS3", func(t *testing.T) {
		store := &mcMockStore{isTextureInS3: func(string) (bool, error) { return true, nil }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "bytes")
		})
		res, err := s.GetTextureContent("hash")
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
		res, err := s.GetTextureContent("hash")
		if err != nil {
			t.Fatalf("GetTextureContent() error = %v", err)
		}
		data, _ := io.ReadAll(res.Body)
		if string(data) != "bytes" {
			t.Errorf("body = %q, want bytes", data)
		}
	})

	t.Run("SV-95_IsTextureInS3Fails", func(t *testing.T) {
		store := &mcMockStore{isTextureInS3: func(string) (bool, error) { return false, testerrors.ErrBoom }}
		s := mcNewService(t, store, nil)
		_, err := s.GetTextureContent("hash")
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})
}

func TestSV96to100_ServeFromS3(t *testing.T) {
	t.Run("SV-96_OK", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			io.WriteString(w, "bytes")
		})
		res, err := s.serveFromS3("hash")
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
		_, err := s.serveFromS3("hash")
		if !errors.Is(err, ErrTextureNotFound) {
			t.Errorf("err = %v, want ErrTextureNotFound", err)
		}
	})

	t.Run("SV-98_OtherStatus", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.serveFromS3("hash")
		if !errors.Is(err, ErrBadStatusS3) {
			t.Fatalf("error = %v, want %v", err, ErrBadStatusS3)
		}
	})

	t.Run("SV-99_NetworkError", func(t *testing.T) {
		s := mcNewUnreachableService(&mcMockStore{})
		_, err := s.serveFromS3("hash")
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
		res, err := s.serveFromS3("hash")
		if err != nil {
			t.Fatalf("serveFromS3() error = %v", err)
		}
		defer res.Body.Close()
		if res.ContentType != "image/png" {
			t.Errorf("ContentType = %q, want default image/png", res.ContentType)
		}
	})
}

func TestSV101_BytesReadCloser_Close(t *testing.T) {
	t.Run("SV-101_AlwaysNil", func(t *testing.T) {
		var b bytesReadCloser
		if err := b.Close(); err != nil {
			t.Errorf("Close() = %v, want nil", err)
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
		res, err := s.fetchAndArchive("hash")
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
		_, err := s.fetchAndArchive("hash")
		if !errors.Is(err, ErrTextureNotFound) {
			t.Errorf("err = %v, want ErrTextureNotFound", err)
		}
	})

	t.Run("SV-104_OtherStatus", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.fetchAndArchive("hash")
		if !errors.Is(err, ErrBadStatusRemote) {
			t.Fatalf("error = %v, want %v", err, ErrBadStatusRemote)
		}
	})

	t.Run("SV-105_NetworkError", func(t *testing.T) {
		s := mcNewUnreachableService(&mcMockStore{})
		_, err := s.fetchAndArchive("hash")
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
		_, err := s.fetchAndArchive("hash")
		if err == nil {
			t.Fatal("expected a body read error from the truncated response")
		}
	})

	t.Run("SV-107_ArchiveFailsStillServesClient", func(t *testing.T) {
		var hashCalled bool
		store := &mcMockStore{
			putTextureInS3:    func(string, io.ReadCloser) error { return testerrors.ErrBoom },
			upsertTextureHash: func(string) error { hashCalled = true; return nil },
		}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "bytes")
		})
		res, err := s.fetchAndArchive("hash")
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
		store := &mcMockStore{upsertTextureHash: func(string) error { return testerrors.ErrBoom }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "bytes")
		})
		res, err := s.fetchAndArchive("hash")
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
		res, err := s.fetchAndArchive("hash")
		if err != nil {
			t.Fatalf("fetchAndArchive() error = %v", err)
		}
		if res.ContentType != "image/png" {
			t.Errorf("ContentType = %q, want default image/png", res.ContentType)
		}
	})
}

func TestSV110to112_GetGeyserTextureContent(t *testing.T) {
	t.Run("SV-110_PresentDelegatesToS3", func(t *testing.T) {
		store := &mcMockStore{isGeyserTextureInS3: func(string) (bool, error) { return true, nil }}
		s := mcNewService(t, store, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "bytes") })
		res, err := s.GetGeyserTextureContent("hash")
		if err != nil {
			t.Fatalf("GetGeyserTextureContent() error = %v", err)
		}
		data, _ := io.ReadAll(res.Body)
		if string(data) != "bytes" {
			t.Errorf("body = %q, want bytes", data)
		}
	})

	t.Run("SV-111_AbsentDelegatesToFetch", func(t *testing.T) {
		var srv *httptest.Server
		store := &mcMockStore{
			isGeyserTextureInS3: func(string) (bool, error) { return false, nil },
			getGeyserSkinByHash: func(string) (*GeyserSkin, error) {
				tex := TexturesValue{Textures: Textures{SKIN: &Texture{URL: srv.URL + "/skin.png"}}}
				return &GeyserSkin{Value: mcEncodeTextures(t, tex)}, nil
			},
		}
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "bytes")
		}))
		t.Cleanup(srv.Close)
		s := mcServiceForServer(store, srv)
		res, err := s.GetGeyserTextureContent("hash")
		if err != nil {
			t.Fatalf("GetGeyserTextureContent() error = %v", err)
		}
		data, _ := io.ReadAll(res.Body)
		if string(data) != "bytes" {
			t.Errorf("body = %q, want bytes", data)
		}
	})

	t.Run("SV-112_IsGeyserTextureInS3Fails", func(t *testing.T) {
		store := &mcMockStore{isGeyserTextureInS3: func(string) (bool, error) { return false, testerrors.ErrBoom }}
		s := mcNewService(t, store, nil)
		_, err := s.GetGeyserTextureContent("hash")
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})
}

func TestSV113to117_ServeGeyserFromS3(t *testing.T) {
	t.Run("SV-113_OK", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			io.WriteString(w, "bytes")
		})
		res, err := s.serveGeyserFromS3("hash")
		if err != nil {
			t.Fatalf("serveGeyserFromS3() error = %v", err)
		}
		if res.ContentType != "image/png" {
			t.Errorf("ContentType = %q, want image/png", res.ContentType)
		}
	})

	t.Run("SV-114_NotFound", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		_, err := s.serveGeyserFromS3("hash")
		if !errors.Is(err, ErrTextureNotFound) {
			t.Errorf("err = %v, want ErrTextureNotFound", err)
		}
	})

	t.Run("SV-115_OtherStatus", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		})
		_, err := s.serveGeyserFromS3("hash")
		if !errors.Is(err, ErrBadStatusS3) {
			t.Fatalf("error = %v, want %v", err, ErrBadStatusS3)
		}
	})

	t.Run("SV-116_NetworkError", func(t *testing.T) {
		s := mcNewUnreachableService(&mcMockStore{})
		_, err := s.serveGeyserFromS3("hash")
		if err == nil {
			t.Fatal("expected a network error")
		}
	})

	t.Run("SV-117_MissingContentTypeDefaults", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, func(w http.ResponseWriter, r *http.Request) {
			// Nil the header so net/http's automatic sniffing doesn't set one.
			w.Header()["Content-Type"] = nil
			io.WriteString(w, "bytes")
		})
		res, err := s.serveGeyserFromS3("hash")
		if err != nil {
			t.Fatalf("serveGeyserFromS3() error = %v", err)
		}
		if res.ContentType != "image/png" {
			t.Errorf("ContentType = %q, want default image/png", res.ContentType)
		}
	})
}

func TestSV118to127_FetchAndArchiveGeyserTexture(t *testing.T) {
	t.Run("SV-118_OK", func(t *testing.T) {
		skinSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "bytes")
		}))
		t.Cleanup(skinSrv.Close)
		tex := TexturesValue{Textures: Textures{SKIN: &Texture{URL: skinSrv.URL + "/skin.png"}}}
		var archived bool
		store := &mcMockStore{
			getGeyserSkinByHash:  func(string) (*GeyserSkin, error) { return &GeyserSkin{Value: mcEncodeTextures(t, tex)}, nil },
			putGeyserTextureInS3: func(string, io.ReadCloser) error { archived = true; return nil },
		}
		s := mcNewService(t, store, nil)
		res, err := s.fetchAndArchiveGeyserTexture("hash")
		if err != nil {
			t.Fatalf("fetchAndArchiveGeyserTexture() error = %v", err)
		}
		data, _ := io.ReadAll(res.Body)
		if string(data) != "bytes" || !archived {
			t.Errorf("data=%q archived=%v", data, archived)
		}
	})

	t.Run("SV-119_StoreLookupFails", func(t *testing.T) {
		store := &mcMockStore{getGeyserSkinByHash: func(string) (*GeyserSkin, error) { return nil, testerrors.ErrBoom }}
		s := mcNewService(t, store, nil)
		_, err := s.fetchAndArchiveGeyserTexture("hash")
		if !errors.Is(err, testerrors.ErrBoom) {
			t.Errorf("err = %v, want %v", err, testerrors.ErrBoom)
		}
	})

	t.Run("SV-120_SkinNil", func(t *testing.T) {
		s := mcNewService(t, &mcMockStore{}, nil)
		_, err := s.fetchAndArchiveGeyserTexture("hash")
		if !errors.Is(err, ErrTextureNotFound) {
			t.Errorf("err = %v, want ErrTextureNotFound", err)
		}
	})

	t.Run("SV-121_SkinURLUndecodable", func(t *testing.T) {
		store := &mcMockStore{getGeyserSkinByHash: func(string) (*GeyserSkin, error) { return &GeyserSkin{Value: "not-base64!!"}, nil }}
		s := mcNewService(t, store, nil)
		_, err := s.fetchAndArchiveGeyserTexture("hash")
		if !errors.Is(err, ErrTextureNotFound) {
			t.Errorf("err = %v, want ErrTextureNotFound", err)
		}
	})

	t.Run("SV-122_NetworkError", func(t *testing.T) {
		deadSrv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		deadSrv.Close()
		tex := TexturesValue{Textures: Textures{SKIN: &Texture{URL: deadSrv.URL + "/skin.png"}}}
		store := &mcMockStore{getGeyserSkinByHash: func(string) (*GeyserSkin, error) { return &GeyserSkin{Value: mcEncodeTextures(t, tex)}, nil }}
		s := mcNewService(t, store, nil)
		_, err := s.fetchAndArchiveGeyserTexture("hash")
		if err == nil {
			t.Fatal("expected a network error")
		}
	})

	t.Run("SV-123_SkinHostNotFound", func(t *testing.T) {
		skinSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		t.Cleanup(skinSrv.Close)
		tex := TexturesValue{Textures: Textures{SKIN: &Texture{URL: skinSrv.URL + "/skin.png"}}}
		store := &mcMockStore{getGeyserSkinByHash: func(string) (*GeyserSkin, error) { return &GeyserSkin{Value: mcEncodeTextures(t, tex)}, nil }}
		s := mcNewService(t, store, nil)
		_, err := s.fetchAndArchiveGeyserTexture("hash")
		if !errors.Is(err, ErrTextureNotFound) {
			t.Errorf("err = %v, want ErrTextureNotFound", err)
		}
	})

	t.Run("SV-124_SkinHostOtherStatus", func(t *testing.T) {
		skinSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(skinSrv.Close)
		tex := TexturesValue{Textures: Textures{SKIN: &Texture{URL: skinSrv.URL + "/skin.png"}}}
		store := &mcMockStore{getGeyserSkinByHash: func(string) (*GeyserSkin, error) { return &GeyserSkin{Value: mcEncodeTextures(t, tex)}, nil }}
		s := mcNewService(t, store, nil)
		_, err := s.fetchAndArchiveGeyserTexture("hash")
		if !errors.Is(err, ErrBadStatusRemote) {
			t.Fatalf("error = %v, want %v", err, ErrBadStatusRemote)
		}
	})

	t.Run("SV-125_BodyReadFails", func(t *testing.T) {
		skinSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "1000")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "short")
			if hj, ok := w.(http.Hijacker); ok {
				if conn, _, err := hj.Hijack(); err == nil {
					conn.Close()
				}
			}
		}))
		t.Cleanup(skinSrv.Close)
		tex := TexturesValue{Textures: Textures{SKIN: &Texture{URL: skinSrv.URL + "/skin.png"}}}
		store := &mcMockStore{getGeyserSkinByHash: func(string) (*GeyserSkin, error) { return &GeyserSkin{Value: mcEncodeTextures(t, tex)}, nil }}
		s := mcNewService(t, store, nil)
		_, err := s.fetchAndArchiveGeyserTexture("hash")
		if err == nil {
			t.Fatal("expected a body read error")
		}
	})

	t.Run("SV-126_ArchiveFailsStillServesClient", func(t *testing.T) {
		skinSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "bytes")
		}))
		t.Cleanup(skinSrv.Close)
		tex := TexturesValue{Textures: Textures{SKIN: &Texture{URL: skinSrv.URL + "/skin.png"}}}
		store := &mcMockStore{
			getGeyserSkinByHash:  func(string) (*GeyserSkin, error) { return &GeyserSkin{Value: mcEncodeTextures(t, tex)}, nil },
			putGeyserTextureInS3: func(string, io.ReadCloser) error { return testerrors.ErrBoom },
		}
		s := mcNewService(t, store, nil)
		res, err := s.fetchAndArchiveGeyserTexture("hash")
		if err != nil {
			t.Fatalf("fetchAndArchiveGeyserTexture() error = %v", err)
		}
		data, _ := io.ReadAll(res.Body)
		if string(data) != "bytes" {
			t.Errorf("body = %q, want bytes despite archive failure", data)
		}
	})

	t.Run("SV-127_MissingContentTypeDefaults", func(t *testing.T) {
		skinSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Nil the header so net/http's automatic sniffing doesn't set one.
			w.Header()["Content-Type"] = nil
			io.WriteString(w, "bytes")
		}))
		t.Cleanup(skinSrv.Close)
		tex := TexturesValue{Textures: Textures{SKIN: &Texture{URL: skinSrv.URL + "/skin.png"}}}
		store := &mcMockStore{getGeyserSkinByHash: func(string) (*GeyserSkin, error) { return &GeyserSkin{Value: mcEncodeTextures(t, tex)}, nil }}
		s := mcNewService(t, store, nil)
		res, err := s.fetchAndArchiveGeyserTexture("hash")
		if err != nil {
			t.Fatalf("fetchAndArchiveGeyserTexture() error = %v", err)
		}
		if res.ContentType != "image/png" {
			t.Errorf("ContentType = %q, want default image/png", res.ContentType)
		}
	})
}
