package linking

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
)

// withServer points the given endpoint URL var at a test server for the
// duration of the test, restoring the original value afterward so other
// tests in this package aren't affected by leftover overrides.
func withServer(t *testing.T, urlVar *string, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	original := *urlVar
	*urlVar = server.URL
	t.Cleanup(func() { *urlVar = original })
	return server
}

// -------------- xstsErrForCode --------------

func TestXstsErrForCode(t *testing.T) {
	tests := []struct {
		code int64
		want error
	}{
		{2148916233, ErrNoXboxAccount},
		{2148916235, ErrXboxLiveUnavailable},
		{2148916236, ErrAdultVerificationRequired},
		{2148916237, ErrAgeVerificationRequired},
		{2148916238, ErrAccountIsChild},
	}
	for _, tt := range tests {
		if got := xstsErrForCode(tt.code); got != tt.want {
			t.Errorf("xstsErrForCode(%d) = %v, want %v", tt.code, got, tt.want)
		}
	}

	if err := xstsErrForCode(999999); err == nil {
		t.Error("expected a non-nil fallback error for an unrecognized XErr code")
	}
}

// -------------- MicrosoftConfig / MicrosoftLoginConfig scope separation --------------

// TestMicrosoftConfigsHaveDisjointScopes pins the security property
// MicrosoftLoginConfig exists for: its OIDC scopes must never end up on
// MicrosoftConfig (whose access token authenticates against Xbox Live), and
// vice versa - XboxLive.signin has no business being requested for a plain
// "sign in with Microsoft" login.
func TestMicrosoftConfigsHaveDisjointScopes(t *testing.T) {
	contains := func(scopes []string, want string) bool {
		for _, s := range scopes {
			if s == want {
				return true
			}
		}
		return false
	}

	if !contains(MicrosoftConfig.Scopes, "XboxLive.signin") {
		t.Error("expected MicrosoftConfig to request XboxLive.signin")
	}
	for _, oidcScope := range []string{"openid", "profile", "email"} {
		if contains(MicrosoftConfig.Scopes, oidcScope) {
			t.Errorf("MicrosoftConfig must not request the OIDC scope %q - that belongs to MicrosoftLoginConfig only", oidcScope)
		}
	}

	for _, oidcScope := range []string{"openid", "profile", "email"} {
		if !contains(MicrosoftLoginConfig.Scopes, oidcScope) {
			t.Errorf("expected MicrosoftLoginConfig to request the OIDC scope %q", oidcScope)
		}
	}
	if contains(MicrosoftLoginConfig.Scopes, "XboxLive.signin") {
		t.Error("MicrosoftLoginConfig must not request XboxLive.signin - that belongs to MicrosoftConfig only")
	}
}

// -------------- xblAuthenticate --------------

func TestXblAuthenticateSuccess(t *testing.T) {
	var gotBody xblAuthRequest
	withServer(t, &xboxLiveAuthenticateURL, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(xblAuthResponse{Token: "xbl-token-123"})
	})

	token, err := xblAuthenticate("ms-access-token")
	if err != nil {
		t.Fatalf("xblAuthenticate returned error: %v", err)
	}
	if token != "xbl-token-123" {
		t.Errorf("expected xbl-token-123, got %q", token)
	}
	if gotBody.Properties.RpsTicket != "d=ms-access-token" {
		t.Errorf("expected RpsTicket to be \"d=ms-access-token\", got %q", gotBody.Properties.RpsTicket)
	}
	if gotBody.RelyingParty != "http://auth.xboxlive.com" {
		t.Errorf("expected the http (not https) relying party, got %q", gotBody.RelyingParty)
	}
}

func TestXblAuthenticateNonOKStatus(t *testing.T) {
	withServer(t, &xboxLiveAuthenticateURL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	if _, err := xblAuthenticate("bad-token"); err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
}

func TestXblAuthenticateMissingToken(t *testing.T) {
	withServer(t, &xboxLiveAuthenticateURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(xblAuthResponse{})
	})

	if _, err := xblAuthenticate("some-token"); err == nil {
		t.Fatal("expected an error when the response is missing Token")
	}
}

// -------------- xstsAuthorize --------------

func TestXstsAuthorizeSuccess(t *testing.T) {
	withServer(t, &xstsAuthorizeURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"Token": "xsts-token-abc",
			"DisplayClaims": map[string]interface{}{
				"xui": []map[string]string{
					{"uhs": "user-hash-1", "xid": "1234567890", "gtg": "CoolGamertag"},
				},
			},
		})
	})

	xstsToken, uhs, xuid, gamertag, err := xstsAuthorize("xbl-token")
	if err != nil {
		t.Fatalf("xstsAuthorize returned error: %v", err)
	}
	if xstsToken != "xsts-token-abc" || uhs != "user-hash-1" || xuid != "1234567890" || gamertag != "CoolGamertag" {
		t.Errorf("unexpected result: token=%q uhs=%q xuid=%q gamertag=%q", xstsToken, uhs, xuid, gamertag)
	}
}

func TestXstsAuthorizeXErrTranslated(t *testing.T) {
	withServer(t, &xstsAuthorizeURL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"XErr":    2148916233,
			"Message": "",
		})
	})

	_, _, _, _, err := xstsAuthorize("xbl-token")
	if err != ErrNoXboxAccount {
		t.Errorf("expected ErrNoXboxAccount, got: %v", err)
	}
}

func TestXstsAuthorizeMissingClaims(t *testing.T) {
	withServer(t, &xstsAuthorizeURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"Token": "xsts-token"})
	})

	_, _, _, _, err := xstsAuthorize("xbl-token")
	if err == nil {
		t.Fatal("expected an error when DisplayClaims/uhs is missing from an otherwise-200 response")
	}
}

// TestXstsAuthorizeMissingXidOrGtg is a regression test from the review
// pipeline: the original validation only checked Token and Uhs were
// non-empty, never Xid/Gtg - so a response with uhs but no xid/gtg would
// silently produce an XboxLiveData{XUID: "", Gamertag: ""}, and a second
// such account would incorrectly resolve to the first under the
// (platform, platform_id) UNIQUE constraint instead of erroring.
func TestXstsAuthorizeMissingXidOrGtg(t *testing.T) {
	withServer(t, &xstsAuthorizeURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"Token": "xsts-token",
			"DisplayClaims": map[string]interface{}{
				"xui": []map[string]string{{"uhs": "user-hash-1"}},
			},
		})
	})

	_, _, _, _, err := xstsAuthorize("xbl-token")
	if err == nil {
		t.Fatal("expected an error when xid/gtg are missing even though uhs is present")
	}
}

// -------------- minecraftLoginWithXbox --------------

func TestMinecraftLoginWithXboxSuccess(t *testing.T) {
	var gotBody mcLoginWithXboxRequest
	withServer(t, &minecraftLoginURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(mcLoginWithXboxResponse{AccessToken: "mc-access-token"})
	})

	token, err := minecraftLoginWithXbox("user-hash", "xsts-token")
	if err != nil {
		t.Fatalf("minecraftLoginWithXbox returned error: %v", err)
	}
	if token != "mc-access-token" {
		t.Errorf("expected mc-access-token, got %q", token)
	}
	if gotBody.IdentityToken != "XBL3.0 x=user-hash;xsts-token" {
		t.Errorf("unexpected identityToken: %q", gotBody.IdentityToken)
	}
}

func TestMinecraftLoginWithXboxFailure(t *testing.T) {
	withServer(t, &minecraftLoginURL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	if _, err := minecraftLoginWithXbox("uhs", "xsts"); err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
}

// -------------- getMinecraftProfile --------------

func TestGetMinecraftProfileOwned(t *testing.T) {
	withServer(t, &minecraftProfileURL, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer mc-token" {
			t.Errorf("expected Authorization: Bearer mc-token, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   "069a79f444e94726a5befca90e38aaf6",
			"name": "Notch",
			"skins": []map[string]string{
				{"id": "6a1b2c3d-4e5f-4a1b-8c2d-3e4f5a6b7c8d", "state": "ACTIVE", "url": "http://textures.minecraft.net/texture/abc", "variant": "CLASSIC", "alias": "DEFAULT"},
			},
			"capes": []map[string]string{},
		})
	})

	profile, err := getMinecraftProfile("mc-token")
	if err != nil {
		t.Fatalf("getMinecraftProfile returned error: %v", err)
	}
	if profile == nil {
		t.Fatal("expected a non-nil profile for an account that owns Java")
	}
	if profile.ID.String() != "069a79f4-44e9-4726-a5be-fca90e38aaf6" {
		t.Errorf("expected the raw undashed Mojang ID to be dash-inserted, got %q", profile.ID.String())
	}
	if profile.Username != "Notch" {
		t.Errorf("expected Username \"Notch\" (from the response's \"name\" field), got %q", profile.Username)
	}
	if len(profile.Skins) != 1 || profile.Skins[0].ID.String() != "6a1b2c3d-4e5f-4a1b-8c2d-3e4f5a6b7c8d" {
		t.Errorf("expected skins to round-trip, got %+v", profile.Skins)
	}
}

func TestGetMinecraftProfileNotFound(t *testing.T) {
	withServer(t, &minecraftProfileURL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"path":         "/minecraft/profile",
			"errorType":    "NOT_FOUND",
			"error":        "NOT_FOUND",
			"errorMessage": "The server has not found anything matching the request URI",
		})
	})

	profile, err := getMinecraftProfile("mc-token")
	if err != nil {
		t.Fatalf("expected no error for a Bedrock-only account (no Java profile), got: %v", err)
	}
	if profile != nil {
		t.Errorf("expected a nil profile when the account doesn't own Java, got: %+v", profile)
	}
}

func TestGetMinecraftProfileServerError(t *testing.T) {
	withServer(t, &minecraftProfileURL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{})
	})

	if _, err := getMinecraftProfile("mc-token"); err == nil {
		t.Fatal("expected an error to propagate for a genuine server failure, not be treated as \"no profile\"")
	}
}

func TestGetMinecraftProfileInvalidUUID(t *testing.T) {
	withServer(t, &minecraftProfileURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "not-a-uuid", "name": "Someone"})
	})

	if _, err := getMinecraftProfile("mc-token"); err == nil {
		t.Fatal("expected an error for a malformed profile ID")
	}
}

// -------------- GetXboxAndMinecraftUser (full chain) --------------

// newFullChainServer wires up all four Xbox Live/Minecraft endpoints behind
// one test server, with the Minecraft profile response controlled by
// hasJavaProfile so both the Java-owner and Bedrock-only paths can be
// exercised end to end through GetXboxAndMinecraftUser, and also serves a
// fake Microsoft OAuth2 token endpoint so ExtCodeForToken/ProcessOAuthLogin/
// ProcessOAuthLink can be driven all the way from an authorization code,
// without hitting the real Microsoft identity platform.
func newFullChainServer(t *testing.T, hasJavaProfile bool) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ms-token", func(w http.ResponseWriter, r *http.Request) {
		// golang.org/x/oauth2 decides how to parse the token response by
		// its Content-Type header - without this set explicitly, Go's
		// content sniffing doesn't recognize plain JSON bytes as
		// "application/json" and the client fails with a misleading
		// "server response missing access_token".
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "ms-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"scope":        "XboxLive.signin offline_access",
		})
	})
	mux.HandleFunc("/xbl-authenticate", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(xblAuthResponse{Token: "xbl-token"})
	})
	mux.HandleFunc("/xsts-authorize", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"Token": "xsts-token",
			"DisplayClaims": map[string]interface{}{
				"xui": []map[string]string{{"uhs": "uhs-1", "xid": "9999", "gtg": "TestGamer"}},
			},
		})
	})
	mux.HandleFunc("/mc-login", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(mcLoginWithXboxResponse{AccessToken: "mc-token"})
	})
	mux.HandleFunc("/mc-profile", func(w http.ResponseWriter, r *http.Request) {
		if !hasJavaProfile {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "NOT_FOUND"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    "069a79f444e94726a5befca90e38aaf6",
			"name":  "Notch",
			"skins": []map[string]string{},
			"capes": []map[string]string{},
		})
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	for _, override := range []*string{&xboxLiveAuthenticateURL, &xstsAuthorizeURL, &minecraftLoginURL, &minecraftProfileURL} {
		original := *override
		t.Cleanup(func() { *override = original })
	}
	xboxLiveAuthenticateURL = server.URL + "/xbl-authenticate"
	xstsAuthorizeURL = server.URL + "/xsts-authorize"
	minecraftLoginURL = server.URL + "/mc-login"
	minecraftProfileURL = server.URL + "/mc-profile"

	// MicrosoftConfig is a *oauth2.Config, so its Endpoint fields are
	// directly mutable - no extra var indirection needed, unlike the
	// XBL/XSTS/MC endpoints above which started life as inline literals.
	originalTokenURL := MicrosoftConfig.Endpoint.TokenURL
	t.Cleanup(func() { MicrosoftConfig.Endpoint.TokenURL = originalTokenURL })
	MicrosoftConfig.Endpoint.TokenURL = server.URL + "/ms-token"
}

func TestGetXboxAndMinecraftUserOwnsJava(t *testing.T) {
	newFullChainServer(t, true)

	xbox, java, err := GetXboxAndMinecraftUser(&auth.OAuthToken{AccessToken: "ms-token"})
	if err != nil {
		t.Fatalf("GetXboxAndMinecraftUser returned error: %v", err)
	}
	if xbox == nil || xbox.XUID != "9999" || xbox.Gamertag != "TestGamer" {
		t.Errorf("unexpected xbox identity: %+v", xbox)
	}
	if java == nil || java.Username != "Notch" {
		t.Errorf("expected a Java profile for an account that owns it, got: %+v", java)
	}
}

func TestGetXboxAndMinecraftUserBedrockOnly(t *testing.T) {
	newFullChainServer(t, false)

	xbox, java, err := GetXboxAndMinecraftUser(&auth.OAuthToken{AccessToken: "ms-token"})
	if err != nil {
		t.Fatalf("expected no error for a Bedrock-only account, got: %v", err)
	}
	if xbox == nil || xbox.XUID != "9999" {
		t.Errorf("expected the Xbox identity to still be populated, got: %+v", xbox)
	}
	if java != nil {
		t.Errorf("expected a nil Java profile for an account that doesn't own Java, got: %+v", java)
	}
}

// -------------- GetXboxUser (Xbox-only, no Minecraft Services calls) --------------

// TestGetXboxUserNeverTouchesMinecraftServices is the core regression test
// for the Xbox/Java split: a caller that only wants the Xbox Live identity
// must not trigger the Minecraft login-with-Xbox/profile calls at all, even
// though the account in this test does own Java - proving the split is a
// real skip, not just a discarded result.
func TestGetXboxUserNeverTouchesMinecraftServices(t *testing.T) {
	withServer(t, &xboxLiveAuthenticateURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(xblAuthResponse{Token: "xbl-token"})
	})
	withServer(t, &xstsAuthorizeURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"Token": "xsts-token",
			"DisplayClaims": map[string]interface{}{
				"xui": []map[string]string{{"uhs": "uhs-1", "xid": "9999", "gtg": "TestGamer"}},
			},
		})
	})
	var minecraftServicesCalled bool
	withServer(t, &minecraftLoginURL, func(w http.ResponseWriter, r *http.Request) {
		minecraftServicesCalled = true
		_ = json.NewEncoder(w).Encode(mcLoginWithXboxResponse{AccessToken: "mc-token"})
	})
	withServer(t, &minecraftProfileURL, func(w http.ResponseWriter, r *http.Request) {
		minecraftServicesCalled = true
		w.WriteHeader(http.StatusNotFound)
	})

	xbox, err := GetXboxUser(&auth.OAuthToken{AccessToken: "ms-token"})
	if err != nil {
		t.Fatalf("GetXboxUser returned error: %v", err)
	}
	if xbox == nil || xbox.XUID != "9999" || xbox.Gamertag != "TestGamer" {
		t.Errorf("unexpected xbox identity: %+v", xbox)
	}
	if minecraftServicesCalled {
		t.Error("GetXboxUser must never call Minecraft Services")
	}
}

func TestGetXboxUserXstsErrorPropagates(t *testing.T) {
	withServer(t, &xboxLiveAuthenticateURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(xblAuthResponse{Token: "xbl-token"})
	})
	withServer(t, &xstsAuthorizeURL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"XErr": 2148916233})
	})

	if _, err := GetXboxUser(&auth.OAuthToken{AccessToken: "ms-token"}); err != ErrNoXboxAccount {
		t.Errorf("expected ErrNoXboxAccount, got: %v", err)
	}
}

// -------------- GetMicrosoftUser --------------

func TestGetMicrosoftUserSuccess(t *testing.T) {
	withServer(t, &microsoftUserInfoURL, func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer ms-access-token" {
			t.Errorf("expected Authorization: Bearer ms-access-token, got %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"sub":   "microsoft-oid-123",
			"name":  "Jane Doe",
			"email": "jane@example.com",
		})
	})

	user, err := GetMicrosoftUser(&auth.OAuthToken{AccessToken: "ms-access-token"})
	if err != nil {
		t.Fatalf("GetMicrosoftUser returned error: %v", err)
	}
	if user.GetID() != "microsoft-oid-123" || user.GetUsername() != "Jane Doe" || user.GetEmail() != "jane@example.com" {
		t.Errorf("unexpected user: %+v", user)
	}
}

func TestGetMicrosoftUserMissingSub(t *testing.T) {
	withServer(t, &microsoftUserInfoURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "Jane Doe"})
	})

	if _, err := GetMicrosoftUser(&auth.OAuthToken{AccessToken: "tok"}); err == nil {
		t.Fatal("expected an error when the userinfo response is missing sub")
	}
}

func TestGetMicrosoftUserNonOKStatus(t *testing.T) {
	withServer(t, &microsoftUserInfoURL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	if _, err := GetMicrosoftUser(&auth.OAuthToken{AccessToken: "tok"}); err == nil {
		t.Fatal("expected an error for a non-2xx response")
	}
}

func TestGetXboxAndMinecraftUserXstsErrorPropagatesWithoutXboxIdentity(t *testing.T) {
	withServer(t, &xboxLiveAuthenticateURL, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(xblAuthResponse{Token: "xbl-token"})
	})
	withServer(t, &xstsAuthorizeURL, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"XErr": 2148916233})
	})

	xbox, java, err := GetXboxAndMinecraftUser(&auth.OAuthToken{AccessToken: "ms-token"})
	if err != ErrNoXboxAccount {
		t.Errorf("expected ErrNoXboxAccount, got: %v", err)
	}
	if xbox != nil || java != nil {
		t.Errorf("expected no identities when XSTS authorization itself fails, got xbox=%+v java=%+v", xbox, java)
	}
}
