package link

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
)

var microsoftHTTPClient = &http.Client{Timeout: 10 * time.Second}

// XSTS relying parties. Xbox Live only ever puts xid/gtg in DisplayClaims
// for the xboxLive one - an authorization scoped to the Minecraft relying
// party legitimately gets uhs alone, so the two need separate XSTS calls.
//
//goland:noinspection HttpUrlsUsage
const (
	XstsMinecraftRelyingParty    = "rp://api.minecraftservices.com/"
	XstsXboxLiveRelyingParty     = "http://xboxlive.com"
	XstsAuthXboxLiveRelyingParty = "http://auth.xboxlive.com"

	maxErrorBodyBytes = 4 << 10
)

// XErr codes returned by xsts.auth.xboxlive.com/xsts/authorize when the
// Microsoft account can't be authorized against Xbox Live. Undocumented by
// Microsoft, but well established from the Minecraft launcher community
// (see https://minecraft.wiki/w/Mojang_API#Authenticate_with_XSTS).
var (
	ErrNoXboxAccount             = errors.New("this Microsoft account does not have an Xbox account")
	ErrXboxLiveUnavailable       = errors.New("xbox Live is not available for this account's country/region")
	ErrAdultVerificationRequired = errors.New("adult verification is required on the Xbox homepage")
	ErrAgeVerificationRequired   = errors.New("age verification is required on the Xbox homepage")
	ErrAccountIsChild            = errors.New("this account belongs to a minor and must be added to a family by an adult")
)

var (
	ErrXSTSAuthentication       = errors.New("xbox XSTS authentication error")
	ErrMicrosoftUserinfoLookup  = errors.New("microsoft userinfo lookup error")
	ErrXboxLiveAuthentication   = errors.New("xbox live authentication error")
	ErrXSTSAuthorization        = errors.New("xsts authorization error")
	ErrMinecraftLoginWithXbox   = errors.New("minecraft login-with-xbox error")
	ErrMinecraftProfileLookup   = errors.New("minecraft profile lookup error")
	ErrUserinfoMissingSub       = errors.New("microsoft userinfo response missing sub")
	ErrXboxLiveMissingToken     = errors.New("xbox live authentication response missing Token")
	ErrXSTSMissingTokenOrClaims = errors.New("xsts authorization response missing Token or DisplayClaims")
	ErrXSTSMissingUHS           = errors.New("xsts authorization response missing uhs in DisplayClaims")
	ErrLoginMissingAccessToken  = errors.New("minecraft login-with-xbox response missing access_token")
	ErrXSTSMissingXIDOrGTG      = errors.New("xsts authorization response missing xid or gtg in DisplayClaims")
)

func xstsErrForCode(ctx context.Context, code int64) error {
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
		reqctx.Log(ctx, ErrXSTSAuthentication.Error(), "code:", strconv.FormatInt(code, 10))
		return ErrXSTSAuthentication
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
func (x *XboxLiveData) CreateLinkedAccount(userID string) *LinkedAccount {
	return NewLinkedAccount(userID, PlatformXboxLive, x.Gamertag, x.XUID, x)
}

// MicrosoftUserData is a plain Microsoft account identity, sourced from the
// OIDC userinfo endpoint - unrelated to Xbox Live or Minecraft ownership.
type MicrosoftUserData struct {
	Sub   string `json:"sub" validate:"required"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// GetID returns the platform ID
func (m *MicrosoftUserData) GetID() string {
	return m.Sub
}

// GetEmail returns the platform email
func (m *MicrosoftUserData) GetEmail() string {
	return m.Email
}

// GetUsername returns the platform username
func (m *MicrosoftUserData) GetUsername() string {
	return m.Name
}

// GetData returns the platform data
func (m *MicrosoftUserData) GetData() string {
	data, _ := json.Marshal(m)
	return string(data)
}

// CreateLinkedAccount creates a linked account
func (m *MicrosoftUserData) CreateLinkedAccount(userID string) *LinkedAccount {
	return NewLinkedAccount(userID, PlatformMicrosoft, m.Name, m.Sub, m)
}

// GetMicrosoftUser fetches the caller's plain Microsoft account identity from
// the OIDC userinfo endpoint, for a "sign in with Microsoft" login that isn't
// about Xbox Live or Minecraft ownership.
func GetMicrosoftUser(ctx context.Context, cfg config.Microsoft, token *OAuthToken) (*MicrosoftUserData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.MicrosoftUserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(HeaderAuthorization, AuthBearer+token.AccessToken)
	req.Header.Set(HeaderAccept, ContentTypeJSON)

	resp, err := microsoftHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		reqctx.Log(ctx, ErrMicrosoftUserinfoLookup.Error(), resp.Status)
		return nil, ErrMicrosoftUserinfoLookup
	}

	var user MicrosoftUserData
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	if user.Sub == "" {
		return nil, ErrUserinfoMissingSub
	}
	return &user, nil
}

// -------------- XBL/XSTS/Minecraft chain --------------

const (
	SandboxIdRetail = "RETAIL"
	TokenTypeJWT    = "JWT"
)

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

type displayClaim struct {
	Uhs string `json:"Uhs"`
	Xid string `json:"Xid"`
	Gtg string `json:"Gtg"`
}

type displayClaims struct {
	Xui []displayClaim `json:"xui"`
}

type xblAuthResponse struct {
	Token         string        `json:"Token"`
	DisplayClaims displayClaims `json:"DisplayClaims"`
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
	xblAuthResponse
	XErr int64 `json:"XErr,omitempty"`
}

type mcLoginWithXboxRequest struct {
	IdentityToken string `json:"identityToken"`
}

type mcLoginWithXboxResponse struct {
	AccessToken string `json:"access_token"`
}

type mcProfileResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Skins []Skin `json:"skins"`
	Capes []Cape `json:"capes"`
	Error string `json:"error"`
}

// xblAuthenticate exchanges a Microsoft OAuth access token for an Xbox Live
// (XBL) user token via the RPS ticket exchange.
func xblAuthenticate(ctx context.Context, cfg config.Microsoft, msAccessToken string) (string, error) {
	reqBody := xblAuthRequest{
		Properties: xblAuthProperties{
			AuthMethod: "RPS",
			SiteName:   "user.auth.xboxlive.com",
			RpsTicket:  "d=" + msAccessToken,
		},
		RelyingParty: XstsAuthXboxLiveRelyingParty,
		TokenType:    TokenTypeJWT,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.XboxLiveAuthenticateURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set(HeaderAccept, ContentTypeJSON)
	req.Header.Set(HeaderContentType, ContentTypeJSON)
	req.Header.Set(HeaderXBLContractVersion, "1")

	resp, err := microsoftHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		reqctx.Log(ctx, ErrXboxLiveAuthentication.Error(), resp.Status)
		return "", ErrXboxLiveAuthentication
	}

	var xblResp xblAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&xblResp); err != nil {
		return "", err
	}
	if xblResp.Token == "" {
		return "", ErrXboxLiveMissingToken
	}
	return xblResp.Token, nil
}

// xstsAuthorize exchanges an XBL user token for an XSTS token scoped to
// relyingParty, returning that token alongside whatever DisplayClaims Xbox
// Live includes for it. xuid/gamertag are only populated when relyingParty
// is xstsXboxLiveRelyingParty - see the const doc comment above.
func xstsAuthorize(ctx context.Context, cfg config.Microsoft, xblToken, relyingParty string) (xstsToken, userHash, xuid, gamertag string, err error) {
	reqBody := xstsAuthRequest{
		Properties: xstsAuthProperties{
			SandboxId:  SandboxIdRetail,
			UserTokens: []string{xblToken},
		},
		RelyingParty: relyingParty,
		TokenType:    TokenTypeJWT,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.XstsAuthorizeURL, bytes.NewReader(body))
	if err != nil {
		return "", "", "", "", err
	}
	req.Header.Set(HeaderAccept, ContentTypeJSON)
	req.Header.Set(HeaderContentType, ContentTypeJSON)

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
		return "", "", "", "", xstsErrForCode(ctx, xstsResp.XErr)
	}
	if resp.StatusCode != http.StatusOK {
		reqctx.Log(ctx, ErrXSTSAuthorization.Error(), resp.Status)
		return "", "", "", "", ErrXSTSAuthorization
	}
	if decodeErr != nil {
		return "", "", "", "", decodeErr
	}
	if xstsResp.Token == "" || len(xstsResp.DisplayClaims.Xui) == 0 {
		return "", "", "", "", ErrXSTSMissingTokenOrClaims
	}

	claims := xstsResp.DisplayClaims.Xui[0]
	if claims.Uhs == "" {
		return "", "", "", "", ErrXSTSMissingUHS
	}
	return xstsResp.Token, claims.Uhs, claims.Xid, claims.Gtg, nil
}

// minecraftLoginWithXbox exchanges an XSTS token for a Minecraft Services
// access token.
func minecraftLoginWithXbox(ctx context.Context, cfg config.Microsoft, userHash, xstsToken string) (string, error) {
	reqBody := mcLoginWithXboxRequest{
		IdentityToken: fmt.Sprintf("XBL3.0 x=%s;%s", userHash, xstsToken),
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.MinecraftLoginURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set(HeaderAccept, ContentTypeJSON)
	req.Header.Set(HeaderContentType, ContentTypeJSON)

	resp, err := microsoftHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		if readErr != nil {
			reqctx.Log(ctx, "Failed to read the Minecraft login error body:\n\t", readErr.Error())
		}
		reqctx.Log(ctx, ErrMinecraftLoginWithXbox.Error(), resp.Status, string(respBody))
		return "", ErrMinecraftLoginWithXbox
	}

	var loginResp mcLoginWithXboxResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", err
	}
	if loginResp.AccessToken == "" {
		return "", ErrLoginMissingAccessToken
	}
	return loginResp.AccessToken, nil
}

// getMinecraftProfile fetches the caller's Minecraft: Java Edition profile.
// A nil profile with a nil error means the account simply doesn't own Java -
// not a failure.
func getMinecraftProfile(ctx context.Context, cfg config.Microsoft, mcAccessToken string) (*MinecraftData, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.MinecraftProfileURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(HeaderAuthorization, AuthBearer+mcAccessToken)

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
	if resp.StatusCode != http.StatusOK {
		reqctx.Log(ctx, ErrMinecraftProfileLookup.Error(), resp.Status)
		return nil, ErrMinecraftProfileLookup
	}
	if decodeErr != nil {
		return nil, decodeErr
	}

	id, err := uuid.Parse(profile.ID)
	if err != nil {
		reqctx.Log(ctx, "Minecraft profile returned an invalid UUID:", profile.ID, err.Error())
		return nil, err
	}
	return &MinecraftData{
		ID:       id,
		Username: profile.Name,
		Skins:    profile.Skins,
		Capes:    profile.Capes,
	}, nil
}

// authenticateXboxLiveIdentity authorizes xblToken against the Xbox Live
// relying party to get the caller's verified Xbox Live identity (XUID +
// gamertag) - unrelated to Minecraft: Java ownership, and deliberately never
// mixed into the Minecraft-scoped XSTS/login_with_xbox chain below.
func authenticateXboxLiveIdentity(ctx context.Context, cfg config.Microsoft, xblToken string) (*XboxLiveData, error) {
	_, _, xuid, gamertag, err := xstsAuthorize(ctx, cfg, xblToken, XstsXboxLiveRelyingParty)
	if err != nil {
		return nil, err
	}
	if xuid == "" || gamertag == "" {
		return nil, ErrXSTSMissingXIDOrGTG
	}
	return &XboxLiveData{XUID: xuid, Gamertag: gamertag}, nil
}

// GetXboxUser exchanges a Microsoft OAuth access token for the caller's
// verified Xbox Live identity, without touching Minecraft Services (or even
// requesting a Minecraft-scoped XSTS token) at all - for callers that want
// to link Xbox Live without also linking (or checking ownership of)
// Minecraft: Java Edition.
func GetXboxUser(ctx context.Context, cfg config.Microsoft, token *OAuthToken) (*XboxLiveData, error) {
	xblToken, err := xblAuthenticate(ctx, cfg, token.AccessToken)
	if err != nil {
		return nil, err
	}
	return authenticateXboxLiveIdentity(ctx, cfg, xblToken)
}

// GetXboxAndMinecraftUser exchanges a Microsoft OAuth access token for the
// caller's Xbox Live identity and, if they own it, their Minecraft: Java
// Edition profile. Three outcomes:
//   - (xbox, java, nil): full success. java is nil if the account doesn't
//     own Java Edition - that's not a failure.
//   - (xbox, nil, err): Xbox Live succeeded but a later Minecraft Services
//     step failed. xbox is still a valid identity. ProcessOAuthLogin
//     continues without Java when that Xbox identity is already linked, and
//     returns err otherwise. ProcessOAuthLink returns err.
//   - (nil, nil, err): Xbox Live authentication itself failed.
//
// The Minecraft-scoped XSTS token is authorized, consumed by
// minecraftLoginWithXbox, and (if that succeeds) exchanged for a profile
// before the Xbox Live identity is authorized at all, rather than
// interleaving the two XSTS authorizations - a previous version requested
// both up front and login_with_xbox started failing with 403s in prod.
func GetXboxAndMinecraftUser(ctx context.Context, cfg config.Microsoft, token *OAuthToken) (xbox *XboxLiveData, java *MinecraftData, err error) {
	xblToken, err := xblAuthenticate(ctx, cfg, token.AccessToken)
	if err != nil {
		return nil, nil, err
	}

	xstsToken, userHash, _, _, err := xstsAuthorize(ctx, cfg, xblToken, XstsMinecraftRelyingParty)
	if err != nil {
		return nil, nil, err
	}

	mcAccessToken, minecraftErr := minecraftLoginWithXbox(ctx, cfg, userHash, xstsToken)
	if minecraftErr == nil {
		java, minecraftErr = getMinecraftProfile(ctx, cfg, mcAccessToken)
	}

	xbox, err = authenticateXboxLiveIdentity(ctx, cfg, xblToken)
	if err != nil {
		return nil, nil, err
	}
	if minecraftErr != nil {
		return xbox, nil, minecraftErr
	}
	return xbox, java, nil
}
