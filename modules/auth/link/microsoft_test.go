package link

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

func readJSONBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func msConfig(mods ...func(*config.Microsoft)) config.Microsoft {
	return testutil.NewConfig(func(c *config.Config) {
		for _, mod := range mods {
			mod(&c.Microsoft)
		}
	}).Microsoft
}

func TestMS04XboxLiveDataGetData(t *testing.T) {
	x := &XboxLiveData{XUID: "x1", Gamertag: "Tag"}

	t.Run("MS-04_GetData", func(t *testing.T) {
		data := x.GetData()
		if data == "" {
			t.Fatal("GetData() returned empty string")
		}
		want := `{"xuid":"x1","gamertag":"Tag"}`
		if data != want {
			t.Errorf("GetData() = %q, want %q", data, want)
		}
	})
}

func TestMS09MicrosoftUserDataGetData(t *testing.T) {
	m := &MicrosoftUserData{Sub: "sub1", Name: "Alice", Email: "a@b.com"}

	t.Run("MS-09_GetData", func(t *testing.T) {
		want := `{"sub":"sub1","name":"Alice","email":"a@b.com"}`
		if got := m.GetData(); got != want {
			t.Errorf("GetData() = %q, want %q", got, want)
		}
	})
}

func TestMS16XstsErrForCodeUnrecognizedCode(t *testing.T) {
	t.Run("MS-16_UnrecognizedCode", func(t *testing.T) {
		got := xstsErrForCode(testutil.LogContext(context.Background()), 999)
		if !errors.Is(got, ErrXSTSAuthentication) {
			t.Errorf("xstsErrForCode(testutil.LogContext(context.Background()), 999) = %v, want it to wrap %v", got, ErrXSTSAuthentication)
		}
	})
}

func TestMS17to21GetMicrosoftUser(t *testing.T) {
	t.Run("MS-17_Success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.Header.Get("Authorization"); got != "Bearer tok" {
				t.Errorf("Authorization header = %q, want %q", got, "Bearer tok")
			}
			_, _ = w.Write([]byte(`{"sub":"s1","name":"n","email":"e"}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.MicrosoftUserInfoURL = ts.URL })

		got, err := GetMicrosoftUser(testutil.LogContext(context.Background()), cfg, &OAuthToken{AccessToken: "tok"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Sub != "s1" || got.Name != "n" || got.Email != "e" {
			t.Errorf("GetMicrosoftUser() = %+v, want sub=s1 name=n email=e", got)
		}
	})

	t.Run("MS-18_NetworkError", func(t *testing.T) {
		cfg := msConfig(func(m *config.Microsoft) { m.MicrosoftUserInfoURL = "http://127.0.0.1:1" })
		_, err := GetMicrosoftUser(testutil.LogContext(context.Background()), cfg, &OAuthToken{AccessToken: "tok"})
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("MS-19_NonOKStatus", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.MicrosoftUserInfoURL = ts.URL })

		_, err := GetMicrosoftUser(testutil.LogContext(context.Background()), cfg, &OAuthToken{AccessToken: "tok"})
		if !errors.Is(err, ErrMicrosoftUserinfoLookup) {
			t.Fatalf("error = %v, want %v", err, ErrMicrosoftUserinfoLookup)
		}
	})

	t.Run("MS-20_MalformedJSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{not-json`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.MicrosoftUserInfoURL = ts.URL })

		_, err := GetMicrosoftUser(testutil.LogContext(context.Background()), cfg, &OAuthToken{AccessToken: "tok"})
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("MS-21_MissingSub", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"name":"n"}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.MicrosoftUserInfoURL = ts.URL })

		_, err := GetMicrosoftUser(testutil.LogContext(context.Background()), cfg, &OAuthToken{AccessToken: "tok"})
		if !errors.Is(err, ErrUserinfoMissingSub) {
			t.Fatalf("GetMicrosoftUser() error = %v, want %v", err, ErrUserinfoMissingSub)
		}
	})
}

func TestMS22to26XblAuthenticate(t *testing.T) {
	t.Run("MS-22_Success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":"t"}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.XboxLiveAuthenticateURL = ts.URL })

		got, err := xblAuthenticate(testutil.LogContext(context.Background()), cfg, "ms-token")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "t" {
			t.Errorf("xblAuthenticate() = %q, want %q", got, "t")
		}
	})

	t.Run("MS-23_NetworkError", func(t *testing.T) {
		cfg := msConfig(func(m *config.Microsoft) { m.XboxLiveAuthenticateURL = "http://127.0.0.1:1" })
		_, err := xblAuthenticate(testutil.LogContext(context.Background()), cfg, "ms-token")
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("MS-24_NonOKStatus", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.XboxLiveAuthenticateURL = ts.URL })

		_, err := xblAuthenticate(testutil.LogContext(context.Background()), cfg, "ms-token")
		if !errors.Is(err, ErrXboxLiveAuthentication) {
			t.Fatalf("error = %v, want %v", err, ErrXboxLiveAuthentication)
		}
	})

	t.Run("MS-25_MalformedJSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{not-json`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.XboxLiveAuthenticateURL = ts.URL })

		_, err := xblAuthenticate(testutil.LogContext(context.Background()), cfg, "ms-token")
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("MS-26_MissingToken", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.XboxLiveAuthenticateURL = ts.URL })

		_, err := xblAuthenticate(testutil.LogContext(context.Background()), cfg, "ms-token")
		if !errors.Is(err, ErrXboxLiveMissingToken) {
			t.Fatalf("xblAuthenticate() error = %v, want %v", err, ErrXboxLiveMissingToken)
		}
	})
}

func TestMS27to34XstsAuthorize(t *testing.T) {
	t.Run("MS-27_SuccessXboxLiveRP", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":"xsts-t","DisplayClaims":{"xui":[{"uhs":"hash1","xid":"xid1","gtg":"Tag"}]}}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.XstsAuthorizeURL = ts.URL })

		token, uhs, xuid, gamertag, err := xstsAuthorize(testutil.LogContext(context.Background()), cfg, "xbl-token", XstsXboxLiveRelyingParty)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != "xsts-t" || uhs != "hash1" || xuid != "xid1" || gamertag != "Tag" {
			t.Errorf("xstsAuthorize() = (%q,%q,%q,%q), want (xsts-t,hash1,xid1,Tag)", token, uhs, xuid, gamertag)
		}
	})

	t.Run("MS-28_SuccessMinecraftRPUhsOnly", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":"xsts-t","DisplayClaims":{"xui":[{"uhs":"hash1"}]}}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.XstsAuthorizeURL = ts.URL })

		token, uhs, xuid, gamertag, err := xstsAuthorize(testutil.LogContext(context.Background()), cfg, "xbl-token", XstsMinecraftRelyingParty)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != "xsts-t" || uhs != "hash1" || xuid != "" || gamertag != "" {
			t.Errorf("xstsAuthorize() = (%q,%q,%q,%q), want (xsts-t,hash1,\"\",\"\")", token, uhs, xuid, gamertag)
		}
	})

	t.Run("MS-29_XErrTranslated", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"XErr":2148916233}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.XstsAuthorizeURL = ts.URL })

		_, _, _, _, err := xstsAuthorize(testutil.LogContext(context.Background()), cfg, "xbl-token", XstsXboxLiveRelyingParty)
		if err != ErrNoXboxAccount {
			t.Fatalf("xstsAuthorize() error = %v, want ErrNoXboxAccount", err)
		}
	})

	t.Run("MS-30_NonOKStatus", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.XstsAuthorizeURL = ts.URL })

		_, _, _, _, err := xstsAuthorize(testutil.LogContext(context.Background()), cfg, "xbl-token", XstsXboxLiveRelyingParty)
		if !errors.Is(err, ErrXSTSAuthorization) {
			t.Fatalf("error = %v, want %v", err, ErrXSTSAuthorization)
		}
	})

	t.Run("MS-31_MalformedJSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{not-json`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.XstsAuthorizeURL = ts.URL })

		_, _, _, _, err := xstsAuthorize(testutil.LogContext(context.Background()), cfg, "xbl-token", XstsXboxLiveRelyingParty)
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("MS-32_MissingTokenOrClaims", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":""}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.XstsAuthorizeURL = ts.URL })

		_, _, _, _, err := xstsAuthorize(testutil.LogContext(context.Background()), cfg, "xbl-token", XstsXboxLiveRelyingParty)
		if !errors.Is(err, ErrXSTSMissingTokenOrClaims) {
			t.Fatalf("xstsAuthorize() error = %v, want %v", err, ErrXSTSMissingTokenOrClaims)
		}
	})

	t.Run("MS-33_MissingUhs", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":"xsts-t","DisplayClaims":{"xui":[{"xid":"xid1","gtg":"Tag"}]}}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.XstsAuthorizeURL = ts.URL })

		_, _, _, _, err := xstsAuthorize(testutil.LogContext(context.Background()), cfg, "xbl-token", XstsXboxLiveRelyingParty)
		if !errors.Is(err, ErrXSTSMissingUHS) {
			t.Fatalf("xstsAuthorize() error = %v, want %v", err, ErrXSTSMissingUHS)
		}
	})

	t.Run("MS-34_NetworkError", func(t *testing.T) {
		cfg := msConfig(func(m *config.Microsoft) { m.XstsAuthorizeURL = "http://127.0.0.1:1" })
		_, _, _, _, err := xstsAuthorize(testutil.LogContext(context.Background()), cfg, "xbl-token", XstsXboxLiveRelyingParty)
		if err == nil {
			t.Fatal("expected an error")
		}
	})
}

type msRoundTripFunc func(*http.Request) (*http.Response, error)

func (f msRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type msBodyFunc func([]byte) (int, error)

func (f msBodyFunc) Read(p []byte) (int, error) { return f(p) }
func (msBodyFunc) Close() error                 { return nil }

func msWithBody(t *testing.T, body io.Reader, status int) {
	t.Helper()
	prev := microsoftHTTPClient.Transport
	microsoftHTTPClient.Transport = msRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Status: http.StatusText(status), Body: io.NopCloser(body), Header: http.Header{}}, nil
	})
	t.Cleanup(func() { microsoftHTTPClient.Transport = prev })
}

func TestMS35to39MinecraftLoginWithXbox(t *testing.T) {
	t.Run("MS-35_Success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"access_token":"m"}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.MinecraftLoginURL = ts.URL })

		got, err := minecraftLoginWithXbox(testutil.LogContext(context.Background()), cfg, "hash1", "xsts-t")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "m" {
			t.Errorf("minecraftLoginWithXbox() = %q, want %q", got, "m")
		}
	})

	t.Run("MS-36_NetworkError", func(t *testing.T) {
		cfg := msConfig(func(m *config.Microsoft) { m.MinecraftLoginURL = "http://127.0.0.1:1" })
		_, err := minecraftLoginWithXbox(testutil.LogContext(context.Background()), cfg, "hash1", "xsts-t")
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("MS-37_NonOKStatus", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`forbidden`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.MinecraftLoginURL = ts.URL })

		_, err := minecraftLoginWithXbox(testutil.LogContext(context.Background()), cfg, "hash1", "xsts-t")
		if !errors.Is(err, ErrMinecraftLoginWithXbox) {
			t.Fatalf("error = %v, want %v", err, ErrMinecraftLoginWithXbox)
		}
	})

	t.Run("MS-59_NonOKLoginBodyReadErrorStillLoginFailed", func(t *testing.T) {
		sent := false
		msWithBody(t, msBodyFunc(func(p []byte) (int, error) {
			if sent {
				return 0, io.ErrUnexpectedEOF
			}
			sent = true
			return copy(p, "partial"), nil
		}), http.StatusForbidden)
		cfg := msConfig(func(m *config.Microsoft) { m.MinecraftLoginURL = "http://login.test" })

		_, err := minecraftLoginWithXbox(testutil.LogContext(context.Background()), cfg, "hash1", "xsts-t")
		if !errors.Is(err, ErrMinecraftLoginWithXbox) {
			t.Fatalf("error = %v, want %v", err, ErrMinecraftLoginWithXbox)
		}
	})

	t.Run("MS-60_NonOKLoginBodyIsBounded", func(t *testing.T) {
		const total = 64 << 20
		read := 0
		msWithBody(t, msBodyFunc(func(p []byte) (int, error) {
			if read >= total {
				return 0, io.EOF
			}
			read += len(p)
			return len(p), nil
		}), http.StatusForbidden)
		cfg := msConfig(func(m *config.Microsoft) { m.MinecraftLoginURL = "http://login.test" })

		_, err := minecraftLoginWithXbox(testutil.LogContext(context.Background()), cfg, "hash1", "xsts-t")
		if !errors.Is(err, ErrMinecraftLoginWithXbox) {
			t.Fatalf("error = %v, want %v", err, ErrMinecraftLoginWithXbox)
		}
		if read < maxErrorBodyBytes || read > 2*maxErrorBodyBytes {
			t.Errorf("read %d bytes of the error body, want about %d", read, maxErrorBodyBytes)
		}
	})

	t.Run("MS-38_MalformedJSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{not-json`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.MinecraftLoginURL = ts.URL })

		_, err := minecraftLoginWithXbox(testutil.LogContext(context.Background()), cfg, "hash1", "xsts-t")
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("MS-39_MissingAccessToken", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.MinecraftLoginURL = ts.URL })

		_, err := minecraftLoginWithXbox(testutil.LogContext(context.Background()), cfg, "hash1", "xsts-t")
		if !errors.Is(err, ErrLoginMissingAccessToken) {
			t.Fatalf("minecraftLoginWithXbox() error = %v, want %v", err, ErrLoginMissingAccessToken)
		}
	})
}

func TestMS40to46GetMinecraftProfile(t *testing.T) {
	validID := uuid.New()

	t.Run("MS-40_Success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%q,"name":"Steve","skins":[],"capes":[]}`, validID.String())))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.MinecraftProfileURL = ts.URL })

		got, err := getMinecraftProfile(testutil.LogContext(context.Background()), cfg, "mc-token")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || got.ID != validID || got.Username != "Steve" {
			t.Errorf("getMinecraftProfile() = %+v, want id=%s username=Steve", got, validID)
		}
	})

	t.Run("MS-41_NotFoundStatus", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.MinecraftProfileURL = ts.URL })

		got, err := getMinecraftProfile(testutil.LogContext(context.Background()), cfg, "mc-token")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("getMinecraftProfile() = %+v, want nil (no Java ownership)", got)
		}
	})

	t.Run("MS-42_NotFoundErrorBody", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"error":"NOT_FOUND"}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.MinecraftProfileURL = ts.URL })

		got, err := getMinecraftProfile(testutil.LogContext(context.Background()), cfg, "mc-token")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("getMinecraftProfile() = %+v, want nil (no Java ownership)", got)
		}
	})

	t.Run("MS-43_NetworkError", func(t *testing.T) {
		cfg := msConfig(func(m *config.Microsoft) { m.MinecraftProfileURL = "http://127.0.0.1:1" })
		_, err := getMinecraftProfile(testutil.LogContext(context.Background()), cfg, "mc-token")
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("MS-44_ServerError", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.MinecraftProfileURL = ts.URL })

		_, err := getMinecraftProfile(testutil.LogContext(context.Background()), cfg, "mc-token")
		if !errors.Is(err, ErrMinecraftProfileLookup) {
			t.Fatalf("error = %v, want %v", err, ErrMinecraftProfileLookup)
		}
	})

	t.Run("MS-45_MalformedJSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{not-json`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.MinecraftProfileURL = ts.URL })

		_, err := getMinecraftProfile(testutil.LogContext(context.Background()), cfg, "mc-token")
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("MS-46_InvalidUUID", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"id":"not-a-uuid","name":"Steve"}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.MinecraftProfileURL = ts.URL })

		_, err := getMinecraftProfile(testutil.LogContext(context.Background()), cfg, "mc-token")
		if err == nil {
			t.Fatal("expected a UUID-parse error")
		}
	})
}

func TestMS47to49AuthenticateXboxLiveIdentity(t *testing.T) {
	t.Run("MS-47_Success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":"xsts-t","DisplayClaims":{"xui":[{"uhs":"hash1","xid":"xid1","gtg":"Tag"}]}}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.XstsAuthorizeURL = ts.URL })

		got, err := authenticateXboxLiveIdentity(testutil.LogContext(context.Background()), cfg, "xbl-token")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.XUID != "xid1" || got.Gamertag != "Tag" {
			t.Errorf("authenticateXboxLiveIdentity() = %+v, want XUID=xid1 Gamertag=Tag", got)
		}
	})

	t.Run("MS-48_XstsAuthorizeFails", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.XstsAuthorizeURL = ts.URL })

		_, err := authenticateXboxLiveIdentity(testutil.LogContext(context.Background()), cfg, "xbl-token")
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("MS-49_MissingXidOrGtg", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":"xsts-t","DisplayClaims":{"xui":[{"uhs":"hash1"}]}}`))
		}))
		defer ts.Close()
		cfg := msConfig(func(m *config.Microsoft) { m.XstsAuthorizeURL = ts.URL })

		_, err := authenticateXboxLiveIdentity(testutil.LogContext(context.Background()), cfg, "xbl-token")
		if !errors.Is(err, ErrXSTSMissingXIDOrGTG) {
			t.Fatalf("authenticateXboxLiveIdentity() error = %v, want %v", err, ErrXSTSMissingXIDOrGTG)
		}
	})
}

func TestMS50to51GetXboxUser(t *testing.T) {
	t.Run("MS-50_Success", func(t *testing.T) {
		xblTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":"xbl-t"}`))
		}))
		defer xblTS.Close()
		xstsTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":"xsts-t","DisplayClaims":{"xui":[{"uhs":"hash1","xid":"xid1","gtg":"Tag"}]}}`))
		}))
		defer xstsTS.Close()
		cfg := msConfig(func(m *config.Microsoft) {
			m.XboxLiveAuthenticateURL = xblTS.URL
			m.XstsAuthorizeURL = xstsTS.URL
		})

		got, err := GetXboxUser(testutil.LogContext(context.Background()), cfg, &OAuthToken{AccessToken: "ms-tok"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.XUID != "xid1" || got.Gamertag != "Tag" {
			t.Errorf("GetXboxUser() = %+v, want XUID=xid1 Gamertag=Tag", got)
		}
	})

	t.Run("MS-51_XblAuthenticateFails", func(t *testing.T) {
		xstsCalled := false
		xstsTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			xstsCalled = true
			_, _ = w.Write([]byte(`{}`))
		}))
		defer xstsTS.Close()
		cfg := msConfig(func(m *config.Microsoft) {
			m.XboxLiveAuthenticateURL = "http://127.0.0.1:1"
			m.XstsAuthorizeURL = xstsTS.URL
		})

		_, err := GetXboxUser(testutil.LogContext(context.Background()), cfg, &OAuthToken{AccessToken: "ms-tok"})
		if err == nil {
			t.Fatal("expected an error")
		}
		if xstsCalled {
			t.Error("xsts authorize endpoint should never be called once xblAuthenticate fails")
		}
	})
}

type msFullChainServers struct {
	xbl, xsts, mcLogin, mcProfile *httptest.Server
	cfg                           config.Microsoft
}

func (s *msFullChainServers) close() {
	s.xbl.Close()
	s.xsts.Close()
	s.mcLogin.Close()
	s.mcProfile.Close()
}

func newMSFullChain(t *testing.T, javaOwned bool, mcLoginFails, mcProfileFails, xstsXboxLiveFails bool) *msFullChainServers {
	t.Helper()
	xbl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Token":"xbl-t"}`))
	}))
	xsts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RelyingParty string `json:"RelyingParty"`
		}
		_ = readJSONBody(r, &body)
		if body.RelyingParty == XstsXboxLiveRelyingParty {
			if xstsXboxLiveFails {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = w.Write([]byte(`{"Token":"xsts-xbl-t","DisplayClaims":{"xui":[{"uhs":"hash1","xid":"xid1","gtg":"Tag"}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"Token":"xsts-mc-t","DisplayClaims":{"xui":[{"uhs":"hash1"}]}}`))
	}))
	mcLogin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if mcLoginFails {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"mc-t"}`))
	}))
	mcProfile := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if mcProfileFails {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !javaOwned {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`{"id":%q,"name":"Steve","skins":[],"capes":[]}`, uuid.New().String())))
	}))

	cfg := msConfig(func(m *config.Microsoft) {
		m.XboxLiveAuthenticateURL = xbl.URL
		m.XstsAuthorizeURL = xsts.URL
		m.MinecraftLoginURL = mcLogin.URL
		m.MinecraftProfileURL = mcProfile.URL
	})

	return &msFullChainServers{xbl: xbl, xsts: xsts, mcLogin: mcLogin, mcProfile: mcProfile, cfg: cfg}
}

func TestMS52to58GetXboxAndMinecraftUser(t *testing.T) {
	t.Run("MS-52_FullSuccessJavaOwned", func(t *testing.T) {
		s := newMSFullChain(t, true, false, false, false)
		defer s.close()
		cfg := s.cfg

		xbox, java, err := GetXboxAndMinecraftUser(testutil.LogContext(context.Background()), cfg, &OAuthToken{AccessToken: "tok"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if xbox == nil || xbox.XUID != "xid1" {
			t.Errorf("xbox = %+v, want XUID=xid1", xbox)
		}
		if java == nil || java.Username != "Steve" {
			t.Errorf("java = %+v, want Username=Steve", java)
		}
	})

	t.Run("MS-53_FullSuccessJavaNotOwned", func(t *testing.T) {
		s := newMSFullChain(t, false, false, false, false)
		defer s.close()
		cfg := s.cfg

		xbox, java, err := GetXboxAndMinecraftUser(testutil.LogContext(context.Background()), cfg, &OAuthToken{AccessToken: "tok"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if xbox == nil {
			t.Error("expected a non-nil xbox identity")
		}
		if java != nil {
			t.Errorf("java = %+v, want nil (not owned)", java)
		}
	})

	t.Run("MS-54_XblAuthenticateFails", func(t *testing.T) {
		s := newMSFullChain(t, true, false, false, false)
		defer s.close()
		cfg := s.cfg
		cfg.XboxLiveAuthenticateURL = "http://127.0.0.1:1"

		xbox, java, err := GetXboxAndMinecraftUser(testutil.LogContext(context.Background()), cfg, &OAuthToken{AccessToken: "tok"})
		if err == nil {
			t.Fatal("expected an error")
		}
		if xbox != nil || java != nil {
			t.Errorf("xbox=%+v java=%+v, want both nil on early failure", xbox, java)
		}
	})

	t.Run("MS-55_MinecraftXstsAuthorizeFails", func(t *testing.T) {
		xbl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":"xbl-t"}`))
		}))
		defer xbl.Close()
		xsts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer xsts.Close()
		cfg := msConfig(func(m *config.Microsoft) {
			m.XboxLiveAuthenticateURL = xbl.URL
			m.XstsAuthorizeURL = xsts.URL
		})

		xbox, java, err := GetXboxAndMinecraftUser(testutil.LogContext(context.Background()), cfg, &OAuthToken{AccessToken: "tok"})
		if err == nil {
			t.Fatal("expected an error")
		}
		if xbox != nil || java != nil {
			t.Errorf("xbox=%+v java=%+v, want both nil", xbox, java)
		}
	})

	t.Run("MS-56_MinecraftLoginFailsXboxStillResolves", func(t *testing.T) {
		s := newMSFullChain(t, true, true, false, false)
		defer s.close()
		cfg := s.cfg

		xbox, java, err := GetXboxAndMinecraftUser(testutil.LogContext(context.Background()), cfg, &OAuthToken{AccessToken: "tok"})
		if err == nil {
			t.Fatal("expected a minecraft-login error")
		}
		if xbox == nil || xbox.XUID != "xid1" {
			t.Errorf("xbox = %+v, want a resolved identity despite the minecraft failure", xbox)
		}
		if java != nil {
			t.Errorf("java = %+v, want nil", java)
		}
	})

	t.Run("MS-57_MinecraftLoginFailsAndXboxLiveAlsoFails", func(t *testing.T) {
		s := newMSFullChain(t, true, true, false, true)
		defer s.close()
		cfg := s.cfg

		xbox, java, err := GetXboxAndMinecraftUser(testutil.LogContext(context.Background()), cfg, &OAuthToken{AccessToken: "tok"})
		if err == nil {
			t.Fatal("expected an error")
		}
		if xbox != nil || java != nil {
			t.Errorf("xbox=%+v java=%+v, want both nil when the Xbox Live identity fetch also fails", xbox, java)
		}
	})

	t.Run("MS-58_MinecraftProfileFailsXboxStillResolves", func(t *testing.T) {
		s := newMSFullChain(t, true, false, true, false)
		defer s.close()
		cfg := s.cfg

		xbox, java, err := GetXboxAndMinecraftUser(testutil.LogContext(context.Background()), cfg, &OAuthToken{AccessToken: "tok"})
		if err == nil {
			t.Fatal("expected a minecraft-profile error")
		}
		if xbox == nil || xbox.XUID != "xid1" {
			t.Errorf("xbox = %+v, want a resolved identity despite the profile failure", xbox)
		}
		if java != nil {
			t.Errorf("java = %+v, want nil", java)
		}
	})
}
