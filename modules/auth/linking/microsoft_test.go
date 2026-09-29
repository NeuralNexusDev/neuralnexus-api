package linking

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/google/uuid"
)

func readJSONBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func msSetURL(t *testing.T, target *string, value string) {
	t.Helper()
	original := *target
	*target = value
	t.Cleanup(func() {
		*target = original
	})
}

func TestMS01to04XboxLiveDataAccessors(t *testing.T) {
	x := &XboxLiveData{XUID: "x1", Gamertag: "Tag"}

	t.Run("MS-01_GetID", func(t *testing.T) {
		if got := x.GetID(); got != "x1" {
			t.Errorf("GetID() = %q, want %q", got, "x1")
		}
	})
	t.Run("MS-02_GetEmail", func(t *testing.T) {
		if got := x.GetEmail(); got != "" {
			t.Errorf("GetEmail() = %q, want empty string", got)
		}
	})
	t.Run("MS-03_GetUsername", func(t *testing.T) {
		if got := x.GetUsername(); got != "Tag" {
			t.Errorf("GetUsername() = %q, want %q", got, "Tag")
		}
	})
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

func TestMS05XboxLiveDataCreateLinkedAccount(t *testing.T) {
	t.Run("MS-05_CreateLinkedAccount", func(t *testing.T) {
		x := &XboxLiveData{XUID: "x1", Gamertag: "Tag"}
		la := x.CreateLinkedAccount("user-1")
		if la.UserID != "user-1" || la.Platform != auth.PlatformXboxLive || la.PlatformID != "x1" || la.PlatformUsername != "Tag" {
			t.Errorf("CreateLinkedAccount() = %+v, want UserID=user-1 Platform=xboxlive PlatformID=x1 PlatformUsername=Tag", la)
		}
	})
}

func TestMS06to09MicrosoftUserDataAccessors(t *testing.T) {
	m := &MicrosoftUserData{Sub: "sub1", Name: "Alice", Email: "a@b.com"}

	t.Run("MS-06_GetID", func(t *testing.T) {
		if got := m.GetID(); got != "sub1" {
			t.Errorf("GetID() = %q, want %q", got, "sub1")
		}
	})
	t.Run("MS-07_GetEmail", func(t *testing.T) {
		if got := m.GetEmail(); got != "a@b.com" {
			t.Errorf("GetEmail() = %q, want %q", got, "a@b.com")
		}
	})
	t.Run("MS-08_GetUsername", func(t *testing.T) {
		if got := m.GetUsername(); got != "Alice" {
			t.Errorf("GetUsername() = %q, want %q", got, "Alice")
		}
	})
	t.Run("MS-09_GetData", func(t *testing.T) {
		want := `{"sub":"sub1","name":"Alice","email":"a@b.com"}`
		if got := m.GetData(); got != want {
			t.Errorf("GetData() = %q, want %q", got, want)
		}
	})
}

func TestMS10MicrosoftUserDataCreateLinkedAccount(t *testing.T) {
	t.Run("MS-10_CreateLinkedAccount", func(t *testing.T) {
		m := &MicrosoftUserData{Sub: "sub1", Name: "Alice"}
		la := m.CreateLinkedAccount("user-1")
		if la.UserID != "user-1" || la.Platform != auth.PlatformMicrosoft || la.PlatformID != "sub1" || la.PlatformUsername != "Alice" {
			t.Errorf("CreateLinkedAccount() = %+v, want UserID=user-1 Platform=microsoft PlatformID=sub1 PlatformUsername=Alice", la)
		}
	})
}

func TestMS11to16XstsErrForCode(t *testing.T) {
	tests := []struct {
		name string
		code int64
		want error
	}{
		{"MS-11_NoXboxAccount", 2148916233, ErrNoXboxAccount},
		{"MS-12_XboxLiveUnavailable", 2148916235, ErrXboxLiveUnavailable},
		{"MS-13_AdultVerificationRequired", 2148916236, ErrAdultVerificationRequired},
		{"MS-14_AgeVerificationRequired", 2148916237, ErrAgeVerificationRequired},
		{"MS-15_AccountIsChild", 2148916238, ErrAccountIsChild},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := xstsErrForCode(tc.code); got != tc.want {
				t.Errorf("xstsErrForCode(%d) = %v, want %v", tc.code, got, tc.want)
			}
		})
	}

	t.Run("MS-16_UnrecognizedCode", func(t *testing.T) {
		got := xstsErrForCode(999)
		want := "xbox XSTS authentication error, code: 999"
		if got == nil || got.Error() != want {
			t.Errorf("xstsErrForCode(999) = %v, want %q", got, want)
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
		msSetURL(t, &microsoftUserInfoURL, ts.URL)

		got, err := GetMicrosoftUser(&auth.OAuthToken{AccessToken: "tok"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Sub != "s1" || got.Name != "n" || got.Email != "e" {
			t.Errorf("GetMicrosoftUser() = %+v, want sub=s1 name=n email=e", got)
		}
	})

	t.Run("MS-18_NetworkError", func(t *testing.T) {
		msSetURL(t, &microsoftUserInfoURL, "http://127.0.0.1:1")
		_, err := GetMicrosoftUser(&auth.OAuthToken{AccessToken: "tok"})
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("MS-19_NonOKStatus", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer ts.Close()
		msSetURL(t, &microsoftUserInfoURL, ts.URL)

		_, err := GetMicrosoftUser(&auth.OAuthToken{AccessToken: "tok"})
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("MS-20_MalformedJSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{not-json`))
		}))
		defer ts.Close()
		msSetURL(t, &microsoftUserInfoURL, ts.URL)

		_, err := GetMicrosoftUser(&auth.OAuthToken{AccessToken: "tok"})
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("MS-21_MissingSub", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"name":"n"}`))
		}))
		defer ts.Close()
		msSetURL(t, &microsoftUserInfoURL, ts.URL)

		_, err := GetMicrosoftUser(&auth.OAuthToken{AccessToken: "tok"})
		if !errors.Is(err, errUserinfoMissingSub) {
			t.Fatalf("GetMicrosoftUser() error = %v, want %v", err, errUserinfoMissingSub)
		}
	})
}

func TestMS22to26XblAuthenticate(t *testing.T) {
	t.Run("MS-22_Success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":"t"}`))
		}))
		defer ts.Close()
		msSetURL(t, &xboxLiveAuthenticateURL, ts.URL)

		got, err := xblAuthenticate("ms-token")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "t" {
			t.Errorf("xblAuthenticate() = %q, want %q", got, "t")
		}
	})

	t.Run("MS-23_NetworkError", func(t *testing.T) {
		msSetURL(t, &xboxLiveAuthenticateURL, "http://127.0.0.1:1")
		_, err := xblAuthenticate("ms-token")
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("MS-24_NonOKStatus", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer ts.Close()
		msSetURL(t, &xboxLiveAuthenticateURL, ts.URL)

		_, err := xblAuthenticate("ms-token")
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("MS-25_MalformedJSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{not-json`))
		}))
		defer ts.Close()
		msSetURL(t, &xboxLiveAuthenticateURL, ts.URL)

		_, err := xblAuthenticate("ms-token")
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("MS-26_MissingToken", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}))
		defer ts.Close()
		msSetURL(t, &xboxLiveAuthenticateURL, ts.URL)

		_, err := xblAuthenticate("ms-token")
		if !errors.Is(err, errXboxLiveMissingToken) {
			t.Fatalf("xblAuthenticate() error = %v, want %v", err, errXboxLiveMissingToken)
		}
	})
}

func TestMS27to34XstsAuthorize(t *testing.T) {
	t.Run("MS-27_SuccessXboxLiveRP", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":"xsts-t","DisplayClaims":{"xui":[{"uhs":"hash1","xid":"xid1","gtg":"Tag"}]}}`))
		}))
		defer ts.Close()
		msSetURL(t, &xstsAuthorizeURL, ts.URL)

		token, uhs, xuid, gamertag, err := xstsAuthorize("xbl-token", xstsXboxLiveRelyingParty)
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
		msSetURL(t, &xstsAuthorizeURL, ts.URL)

		token, uhs, xuid, gamertag, err := xstsAuthorize("xbl-token", xstsMinecraftRelyingParty)
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
		msSetURL(t, &xstsAuthorizeURL, ts.URL)

		_, _, _, _, err := xstsAuthorize("xbl-token", xstsXboxLiveRelyingParty)
		if err != ErrNoXboxAccount {
			t.Fatalf("xstsAuthorize() error = %v, want ErrNoXboxAccount", err)
		}
	})

	t.Run("MS-30_NonOKStatus", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()
		msSetURL(t, &xstsAuthorizeURL, ts.URL)

		_, _, _, _, err := xstsAuthorize("xbl-token", xstsXboxLiveRelyingParty)
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("MS-31_MalformedJSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{not-json`))
		}))
		defer ts.Close()
		msSetURL(t, &xstsAuthorizeURL, ts.URL)

		_, _, _, _, err := xstsAuthorize("xbl-token", xstsXboxLiveRelyingParty)
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("MS-32_MissingTokenOrClaims", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":""}`))
		}))
		defer ts.Close()
		msSetURL(t, &xstsAuthorizeURL, ts.URL)

		_, _, _, _, err := xstsAuthorize("xbl-token", xstsXboxLiveRelyingParty)
		if !errors.Is(err, errXSTSMissingTokenOrClaims) {
			t.Fatalf("xstsAuthorize() error = %v, want %v", err, errXSTSMissingTokenOrClaims)
		}
	})

	t.Run("MS-33_MissingUhs", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":"xsts-t","DisplayClaims":{"xui":[{"xid":"xid1","gtg":"Tag"}]}}`))
		}))
		defer ts.Close()
		msSetURL(t, &xstsAuthorizeURL, ts.URL)

		_, _, _, _, err := xstsAuthorize("xbl-token", xstsXboxLiveRelyingParty)
		if !errors.Is(err, errXSTSMissingUHS) {
			t.Fatalf("xstsAuthorize() error = %v, want %v", err, errXSTSMissingUHS)
		}
	})

	t.Run("MS-34_NetworkError", func(t *testing.T) {
		msSetURL(t, &xstsAuthorizeURL, "http://127.0.0.1:1")
		_, _, _, _, err := xstsAuthorize("xbl-token", xstsXboxLiveRelyingParty)
		if err == nil {
			t.Fatal("expected an error")
		}
	})
}

func TestMS35to39MinecraftLoginWithXbox(t *testing.T) {
	t.Run("MS-35_Success", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"access_token":"m"}`))
		}))
		defer ts.Close()
		msSetURL(t, &minecraftLoginURL, ts.URL)

		got, err := minecraftLoginWithXbox("hash1", "xsts-t")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "m" {
			t.Errorf("minecraftLoginWithXbox() = %q, want %q", got, "m")
		}
	})

	t.Run("MS-36_NetworkError", func(t *testing.T) {
		msSetURL(t, &minecraftLoginURL, "http://127.0.0.1:1")
		_, err := minecraftLoginWithXbox("hash1", "xsts-t")
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
		msSetURL(t, &minecraftLoginURL, ts.URL)

		_, err := minecraftLoginWithXbox("hash1", "xsts-t")
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("MS-38_MalformedJSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{not-json`))
		}))
		defer ts.Close()
		msSetURL(t, &minecraftLoginURL, ts.URL)

		_, err := minecraftLoginWithXbox("hash1", "xsts-t")
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("MS-39_MissingAccessToken", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{}`))
		}))
		defer ts.Close()
		msSetURL(t, &minecraftLoginURL, ts.URL)

		_, err := minecraftLoginWithXbox("hash1", "xsts-t")
		if !errors.Is(err, errLoginMissingAccessToken) {
			t.Fatalf("minecraftLoginWithXbox() error = %v, want %v", err, errLoginMissingAccessToken)
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
		msSetURL(t, &minecraftProfileURL, ts.URL)

		got, err := getMinecraftProfile("mc-token")
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
		msSetURL(t, &minecraftProfileURL, ts.URL)

		got, err := getMinecraftProfile("mc-token")
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
		msSetURL(t, &minecraftProfileURL, ts.URL)

		got, err := getMinecraftProfile("mc-token")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("getMinecraftProfile() = %+v, want nil (no Java ownership)", got)
		}
	})

	t.Run("MS-43_NetworkError", func(t *testing.T) {
		msSetURL(t, &minecraftProfileURL, "http://127.0.0.1:1")
		_, err := getMinecraftProfile("mc-token")
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("MS-44_ServerError", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()
		msSetURL(t, &minecraftProfileURL, ts.URL)

		_, err := getMinecraftProfile("mc-token")
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("MS-45_MalformedJSON", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{not-json`))
		}))
		defer ts.Close()
		msSetURL(t, &minecraftProfileURL, ts.URL)

		_, err := getMinecraftProfile("mc-token")
		if err == nil {
			t.Fatal("expected a decode error")
		}
	})

	t.Run("MS-46_InvalidUUID", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"id":"not-a-uuid","name":"Steve"}`))
		}))
		defer ts.Close()
		msSetURL(t, &minecraftProfileURL, ts.URL)

		_, err := getMinecraftProfile("mc-token")
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
		msSetURL(t, &xstsAuthorizeURL, ts.URL)

		got, err := authenticateXboxLiveIdentity("xbl-token")
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
		msSetURL(t, &xstsAuthorizeURL, ts.URL)

		_, err := authenticateXboxLiveIdentity("xbl-token")
		if err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("MS-49_MissingXidOrGtg", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"Token":"xsts-t","DisplayClaims":{"xui":[{"uhs":"hash1"}]}}`))
		}))
		defer ts.Close()
		msSetURL(t, &xstsAuthorizeURL, ts.URL)

		_, err := authenticateXboxLiveIdentity("xbl-token")
		if !errors.Is(err, errXSTSMissingXIDOrGTG) {
			t.Fatalf("authenticateXboxLiveIdentity() error = %v, want %v", err, errXSTSMissingXIDOrGTG)
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
		msSetURL(t, &xboxLiveAuthenticateURL, xblTS.URL)
		msSetURL(t, &xstsAuthorizeURL, xstsTS.URL)

		got, err := GetXboxUser(&auth.OAuthToken{AccessToken: "ms-tok"})
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
		msSetURL(t, &xboxLiveAuthenticateURL, "http://127.0.0.1:1")
		msSetURL(t, &xstsAuthorizeURL, xstsTS.URL)

		_, err := GetXboxUser(&auth.OAuthToken{AccessToken: "ms-tok"})
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
		if body.RelyingParty == xstsXboxLiveRelyingParty {
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

	msSetURL(t, &xboxLiveAuthenticateURL, xbl.URL)
	msSetURL(t, &xstsAuthorizeURL, xsts.URL)
	msSetURL(t, &minecraftLoginURL, mcLogin.URL)
	msSetURL(t, &minecraftProfileURL, mcProfile.URL)

	return &msFullChainServers{xbl: xbl, xsts: xsts, mcLogin: mcLogin, mcProfile: mcProfile}
}

func TestMS52to58GetXboxAndMinecraftUser(t *testing.T) {
	t.Run("MS-52_FullSuccessJavaOwned", func(t *testing.T) {
		s := newMSFullChain(t, true, false, false, false)
		defer s.close()

		xbox, java, err := GetXboxAndMinecraftUser(&auth.OAuthToken{AccessToken: "tok"})
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

		xbox, java, err := GetXboxAndMinecraftUser(&auth.OAuthToken{AccessToken: "tok"})
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
		msSetURL(t, &xboxLiveAuthenticateURL, "http://127.0.0.1:1")

		xbox, java, err := GetXboxAndMinecraftUser(&auth.OAuthToken{AccessToken: "tok"})
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
		msSetURL(t, &xboxLiveAuthenticateURL, xbl.URL)
		msSetURL(t, &xstsAuthorizeURL, xsts.URL)

		xbox, java, err := GetXboxAndMinecraftUser(&auth.OAuthToken{AccessToken: "tok"})
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

		xbox, java, err := GetXboxAndMinecraftUser(&auth.OAuthToken{AccessToken: "tok"})
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

		xbox, java, err := GetXboxAndMinecraftUser(&auth.OAuthToken{AccessToken: "tok"})
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

		xbox, java, err := GetXboxAndMinecraftUser(&auth.OAuthToken{AccessToken: "tok"})
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
