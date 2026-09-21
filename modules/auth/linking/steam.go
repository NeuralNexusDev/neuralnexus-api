package linking

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/goccy/go-json"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
)

// -------------- Global Variables --------------

//goland:noinspection GoSnakeCaseUsage
var STEAM_API_KEY = os.Getenv("STEAM_API_KEY")

var steamHTTPClient = &http.Client{Timeout: 10 * time.Second}

// Endpoint URLs as vars, not inline literals, so tests can point them at a
// local httptest.Server instead of the real Steam services.
var (
	steamOpenIDLoginURL   = "https://steamcommunity.com/openid/login"
	steamPlayerSummaryURL = "https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v2/"
)

// steamClaimedIDPattern matches the identity URL Steam returns in
// openid.claimed_id, capturing the numeric SteamID64.
var steamClaimedIDPattern = regexp.MustCompile(`^https://steamcommunity\.com/openid/id/(\d+)$`)

// -------------- Structs --------------

// SteamData struct
type SteamData struct {
	SteamID64   string `json:"steamid"`
	PersonaName string `json:"personaname"`
	ProfileURL  string `json:"profileurl"`
	AvatarURL   string `json:"avatarfull"`
}

// GetID returns the platform ID
func (s *SteamData) GetID() string {
	return s.SteamID64
}

// GetUsername returns the platform username
func (s *SteamData) GetUsername() string {
	return s.PersonaName
}

// GetEmail returns the platform email - Steam never exposes one
func (s *SteamData) GetEmail() string {
	return ""
}

// GetData returns the platform data
func (s *SteamData) GetData() string {
	data, _ := json.Marshal(s)
	return string(data)
}

// CreateLinkedAccount creates a linked account
func (s *SteamData) CreateLinkedAccount(userID string) *auth.LinkedAccount {
	return auth.NewLinkedAccount(userID, auth.PlatformSteam, s.PersonaName, s.SteamID64, s)
}

// -------------- Functions --------------

// VerifySteamOpenIDCallback verifies an OpenID 2.0 assertion by POSTing it
// back to Steam's own endpoint, returning the caller's SteamID64.
func VerifySteamOpenIDCallback(query url.Values) (string, error) {
	if query.Get("openid.mode") != "id_res" {
		return "", errors.New("unexpected openid.mode")
	}

	matches := steamClaimedIDPattern.FindStringSubmatch(query.Get("openid.claimed_id"))
	if matches == nil {
		return "", errors.New("invalid or missing openid.claimed_id")
	}
	steamID64 := matches[1]

	// check_authentication only confirms the signature is valid over
	// whatever fields openid.signed lists - it says nothing about whether
	// claimed_id was one of them, so that has to be checked separately.
	if !slices.Contains(strings.Split(query.Get("openid.signed"), ","), "claimed_id") {
		return "", errors.New("openid.signed does not cover claimed_id")
	}

	checkValues := url.Values{}
	for k, v := range query {
		checkValues[k] = v
	}
	checkValues.Set("openid.mode", "check_authentication")

	req, err := http.NewRequest(http.MethodPost, steamOpenIDLoginURL, strings.NewReader(checkValues.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := steamHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("steam openid check_authentication error: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if !strings.Contains(string(body), "is_valid:true") {
		return "", errors.New("steam rejected the openid assertion")
	}

	return steamID64, nil
}

// GetSteamUser fetches the caller's public profile via the Steam Web API,
// for a persona name/avatar beyond the bare SteamID64 OpenID provides.
func GetSteamUser(steamID64 string) (*SteamData, error) {
	if STEAM_API_KEY == "" {
		return nil, errors.New("STEAM_API_KEY is not set")
	}

	reqURL := steamPlayerSummaryURL + "?key=" + url.QueryEscape(STEAM_API_KEY) + "&steamids=" + url.QueryEscape(steamID64)
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := steamHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("steam player summary lookup error: %s", resp.Status)
	}

	var parsed struct {
		Response struct {
			Players []SteamData `json:"players"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.Response.Players) == 0 {
		return nil, errors.New("steam player summary response contained no players")
	}
	return &parsed.Response.Players[0], nil
}

// ProcessSteamLogin resolves or creates an account for the given,
// already-verified Steam identity and returns a new session for it.
func ProcessSteamLogin(as auth.AccountService, las auth.LinkAccountStore, ss auth.SessionService, user *SteamData) (*auth.Session, error) {
	a, err := resolveOrCreateAccountForPlatformUser(as, las, auth.PlatformSteam, user)
	if err != nil {
		return nil, err
	}
	session, err := a.NewSession(time.Now().Add(time.Hour * 24).Unix())
	if err != nil {
		return nil, err
	}
	if err = ss.AddSession(session); err != nil {
		return nil, err
	}
	return session, nil
}

// ProcessSteamLink links the given, already-verified Steam identity to the
// session already in r's context.
func ProcessSteamLink(r *http.Request, las auth.LinkAccountStore, user *SteamData) (*auth.Session, error) {
	session, ok := r.Context().Value(mw.SessionKey).(*auth.Session)
	if !ok || session == nil {
		return nil, errors.New("session not found")
	}
	if !session.IsValid() {
		return nil, errors.New("session expired")
	}
	if err := linkPlatformUserToSession(las, session.UserID, auth.PlatformSteam, user); err != nil {
		return nil, err
	}
	return session, nil
}
