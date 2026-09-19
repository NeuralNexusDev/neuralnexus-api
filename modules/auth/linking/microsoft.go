package linking

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"net/http"
	"os"
	"time"
)

// -------------- Global Variables --------------

//goland:noinspection GoSnakeCaseUsage
var (
	MICROSOFT_CLIENT_ID     = os.Getenv("MICROSOFT_CLIENT_ID")
	MICROSOFT_CLIENT_SECRET = os.Getenv("MICROSOFT_CLIENT_SECRET")
	MICROSOFT_REDIRECT_URI  = os.Getenv("MICROSOFT_REDIRECT_URI")
	MicrosoftConfig         = &oauth2.Config{
		ClientID:     MICROSOFT_CLIENT_ID,
		ClientSecret: MICROSOFT_CLIENT_SECRET,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize",
			TokenURL: "https://login.microsoftonline.com/consumers/oauth2/v2.0/token",
		},
		Scopes:      []string{"XboxLive.signin", "offline_access"},
		RedirectURL: MICROSOFT_REDIRECT_URI,
	}
)

var microsoftHTTPClient = &http.Client{Timeout: 10 * time.Second}

// Endpoint URLs as vars, not inline literals, so tests can point them at a
// local httptest.Server instead of the real Xbox Live/Minecraft services.
var (
	xboxLiveAuthenticateURL = "https://user.auth.xboxlive.com/user/authenticate"
	xstsAuthorizeURL        = "https://xsts.auth.xboxlive.com/xsts/authorize"
	minecraftLoginURL       = "https://api.minecraftservices.com/authentication/login_with_xbox"
	minecraftProfileURL     = "https://api.minecraftservices.com/minecraft/profile"
)

// XErr codes returned by xsts.auth.xboxlive.com/xsts/authorize when the
// Microsoft account can't be authorized against Xbox Live. Undocumented by
// Microsoft, but well established from the Minecraft launcher community
// (see https://minecraft.wiki/w/Mojang_API#Authenticate_with_XSTS).
var (
	ErrNoXboxAccount             = errors.New("this Microsoft account does not have an Xbox account")
	ErrXboxLiveUnavailable       = errors.New("Xbox Live is not available for this account's country/region")
	ErrAdultVerificationRequired = errors.New("adult verification is required on the Xbox homepage")
	ErrAgeVerificationRequired   = errors.New("age verification is required on the Xbox homepage")
	ErrAccountIsChild            = errors.New("this account belongs to a minor and must be added to a family by an adult")
)

func xstsErrForCode(code int64) error {
	switch code {
	case 2148916233:
		return ErrNoXboxAccount
	case 2148916235:
		return ErrXboxLiveUnavailable
	case 2148916236:
		return ErrAdultVerificationRequired
	case 2148916237:
		return ErrAgeVerificationRequired
	case 2148916238:
		return ErrAccountIsChild
	default:
		return fmt.Errorf("xbox XSTS authentication error, code: %d", code)
	}
}

// -------------- Structs --------------

// XboxLiveData is a verified Xbox Live identity (XUID + gamertag), sourced
// from the XSTS token claims.
type XboxLiveData struct {
	XUID     string `json:"xuid" validate:"required"`
	Gamertag string `json:"gamertag" validate:"required"`
}

// GetID returns the platform ID
func (x *XboxLiveData) GetID() string {
	return x.XUID
}

// GetEmail returns the platform email
func (x *XboxLiveData) GetEmail() string {
	return ""
}

// GetUsername returns the platform username
func (x *XboxLiveData) GetUsername() string {
	return x.Gamertag
}

// GetData returns the platform data
func (x *XboxLiveData) GetData() string {
	data, _ := json.Marshal(x)
	return string(data)
}

// CreateLinkedAccount creates a linked account
func (x *XboxLiveData) CreateLinkedAccount(userID string) *auth.LinkedAccount {
	return auth.NewLinkedAccount(userID, auth.PlatformXboxLive, x.Gamertag, x.XUID, x)
}

// -------------- XBL/XSTS/Minecraft chain --------------

type xblAuthRequest struct {
	Properties   xblAuthProperties `json:"Properties"`
	RelyingParty string            `json:"RelyingParty"`
	TokenType    string            `json:"TokenType"`
}

type xblAuthProperties struct {
	AuthMethod string `json:"AuthMethod"`
	SiteName   string `json:"SiteName"`
	RpsTicket  string `json:"RpsTicket"`
}

type displayClaims struct {
	Xui []struct {
		Uhs string `json:"uhs"`
		Xid string `json:"xid,omitempty"`
		Gtg string `json:"gtg,omitempty"`
	} `json:"xui"`
}

type xblAuthResponse struct {
	Token         string        `json:"Token"`
	DisplayClaims displayClaims `json:"DisplayClaims"`
}

// xblAuthenticate exchanges a Microsoft OAuth access token for an Xbox Live
// (XBL) user token via the RPS ticket exchange.
func xblAuthenticate(msAccessToken string) (string, error) {
	reqBody := xblAuthRequest{
		Properties: xblAuthProperties{
			AuthMethod: "RPS",
			SiteName:   "user.auth.xboxlive.com",
			RpsTicket:  "d=" + msAccessToken,
		},
		// Has to be http, not https - Xbox Live only recognizes this exact
		// relying party string.
		RelyingParty: "http://auth.xboxlive.com",
		TokenType:    "JWT",
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, xboxLiveAuthenticateURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("x-xbl-contract-version", "1")

	resp, err := microsoftHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("xbox live authentication error: %s", resp.Status)
	}

	var xblResp xblAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&xblResp); err != nil {
		return "", err
	}
	if xblResp.Token == "" {
		return "", errors.New("xbox live authentication response missing Token")
	}
	return xblResp.Token, nil
}

type xstsAuthRequest struct {
	Properties   xstsAuthProperties `json:"Properties"`
	RelyingParty string             `json:"RelyingParty"`
	TokenType    string             `json:"TokenType"`
}

type xstsAuthProperties struct {
	SandboxId  string   `json:"SandboxId"`
	UserTokens []string `json:"UserTokens"`
}

type xstsAuthResponse struct {
	Token         string        `json:"Token"`
	DisplayClaims displayClaims `json:"DisplayClaims"`
	XErr          int64         `json:"XErr,omitempty"`
}

// xstsAuthorize exchanges an XBL user token for an XSTS token scoped to
// Minecraft Services, returning that token alongside the caller's Xbox Live
// identity (XUID, gamertag, user hash) straight from the DisplayClaims - the
// same response this pulls the Minecraft login credential from.
func xstsAuthorize(xblToken string) (xstsToken, userHash, xuid, gamertag string, err error) {
	reqBody := xstsAuthRequest{
		Properties: xstsAuthProperties{
			SandboxId:  "RETAIL",
			UserTokens: []string{xblToken},
		},
		RelyingParty: "rp://api.minecraftservices.com/",
		TokenType:    "JWT",
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", "", "", err
	}

	req, err := http.NewRequest(http.MethodPost, xstsAuthorizeURL, bytes.NewReader(body))
	if err != nil {
		return "", "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := microsoftHTTPClient.Do(req)
	if err != nil {
		return "", "", "", "", err
	}
	defer resp.Body.Close()

	var xstsResp xstsAuthResponse
	// Decode first: an XErr failure still needs the body read to tell which
	// one (see xstsErrForCode). decodeErr is only surfaced after the status
	// checks below, so a non-JSON error page on a genuine 5xx still reports
	// as a clean status error.
	decodeErr := json.NewDecoder(resp.Body).Decode(&xstsResp)

	if xstsResp.XErr != 0 {
		return "", "", "", "", xstsErrForCode(xstsResp.XErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", "", "", fmt.Errorf("xsts authorization error: %s", resp.Status)
	}
	if decodeErr != nil {
		return "", "", "", "", decodeErr
	}
	if xstsResp.Token == "" || len(xstsResp.DisplayClaims.Xui) == 0 {
		return "", "", "", "", errors.New("xsts authorization response missing Token or DisplayClaims")
	}

	claims := xstsResp.DisplayClaims.Xui[0]
	if claims.Uhs == "" || claims.Xid == "" || claims.Gtg == "" {
		return "", "", "", "", errors.New("xsts authorization response missing uhs, xid, or gtg in DisplayClaims")
	}
	return xstsResp.Token, claims.Uhs, claims.Xid, claims.Gtg, nil
}

type mcLoginWithXboxRequest struct {
	IdentityToken string `json:"identityToken"`
}

type mcLoginWithXboxResponse struct {
	AccessToken string `json:"access_token"`
}

// minecraftLoginWithXbox exchanges an XSTS token for a Minecraft Services
// access token.
func minecraftLoginWithXbox(userHash, xstsToken string) (string, error) {
	reqBody := mcLoginWithXboxRequest{
		IdentityToken: fmt.Sprintf("XBL3.0 x=%s;%s", userHash, xstsToken),
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, minecraftLoginURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := microsoftHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("minecraft login-with-xbox error: %s", resp.Status)
	}

	var loginResp mcLoginWithXboxResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", err
	}
	if loginResp.AccessToken == "" {
		return "", errors.New("minecraft login-with-xbox response missing access_token")
	}
	return loginResp.AccessToken, nil
}

type mcProfileResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Skins []Skin `json:"skins"`
	Capes []Cape `json:"capes"`
	Error string `json:"error"`
}

// getMinecraftProfile fetches the caller's Minecraft: Java Edition profile.
// A nil profile with a nil error means the account simply doesn't own Java -
// not a failure.
func getMinecraftProfile(mcAccessToken string) (*MinecraftData, error) {
	req, err := http.NewRequest(http.MethodGet, minecraftProfileURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+mcAccessToken)

	resp, err := microsoftHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var profile mcProfileResponse
	// Decode first, same reasoning as xstsAuthorize: "doesn't own Java" is
	// only visible in the decoded body, so decodeErr is surfaced after the
	// status checks below.
	decodeErr := json.NewDecoder(resp.Body).Decode(&profile)

	if resp.StatusCode == http.StatusNotFound || profile.Error == "NOT_FOUND" {
		return nil, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("minecraft profile lookup error: %s", resp.Status)
	}
	if decodeErr != nil {
		return nil, decodeErr
	}

	id, err := uuid.Parse(profile.ID)
	if err != nil {
		return nil, fmt.Errorf("minecraft profile returned an invalid UUID %q: %w", profile.ID, err)
	}
	return &MinecraftData{
		ID:       id,
		Username: profile.Name,
		Skins:    profile.Skins,
		Capes:    profile.Capes,
	}, nil
}

// GetXboxAndMinecraftUser exchanges a Microsoft OAuth access token for the
// caller's Xbox Live identity and, if they own it, their Minecraft: Java
// Edition profile. Three outcomes:
//   - (xbox, java, nil): full success. java is nil if the account doesn't
//     own Java Edition - that's not a failure.
//   - (xbox, nil, err): Xbox Live succeeded but a later Minecraft Services
//     step failed. xbox is still a valid identity - callers must check it
//     before discarding a good login over a failed Java-ownership check.
//   - (nil, nil, err): Xbox Live authentication itself failed.
func GetXboxAndMinecraftUser(token *auth.OAuthToken) (xbox *XboxLiveData, java *MinecraftData, err error) {
	xblToken, err := xblAuthenticate(token.AccessToken)
	if err != nil {
		return nil, nil, err
	}

	xstsToken, userHash, xuid, gamertag, err := xstsAuthorize(xblToken)
	if err != nil {
		return nil, nil, err
	}
	xbox = &XboxLiveData{XUID: xuid, Gamertag: gamertag}

	mcAccessToken, err := minecraftLoginWithXbox(userHash, xstsToken)
	if err != nil {
		return xbox, nil, err
	}

	java, err = getMinecraftProfile(mcAccessToken)
	if err != nil {
		return xbox, nil, err
	}
	return xbox, java, nil
}
