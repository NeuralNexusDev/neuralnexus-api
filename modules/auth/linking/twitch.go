package accountlinking

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	sess "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/goccy/go-json"
)

// -------------- Global Variables --------------
var (
	TWITCH_CLIENT_ID     = os.Getenv("TWITCH_CLIENT_ID")
	TWITCH_CLIENT_SECRET = os.Getenv("TWITCH_CLIENT_SECRET")
	TWITCH_REDIRECT_URI  = os.Getenv("TWITCH_REDIRECT_URI")
	TWITCH_API_ENDPOINT  = "https://api.twitch.tv/helix"
)

// -------------- Structs --------------

// TwitchTokenResponse struct
type TwitchTokenResponse struct {
	AccessToken string   `json:"access_token"`
	ExpiresIn   int      `json:"expires_in"`
	RereshToken string   `json:"refresh_token"`
	Scope       []string `json:"scope"`
	TokenType   string   `json:"token_type"`
}

// TwitchData struct
type TwitchData struct {
	ID              string `json:"id" validate:"required"`
	Login           string `json:"login" validate:"required"`
	DisplayName     string `json:"display_name" validate:"required"`
	Type            string `json:"type,omitempty"`
	BroadcasterType string `json:"broadcaster_type,omitempty"`
	Description     string `json:"description,omitempty"`
	ProfileImageURL string `json:"profile_image_url,omitempty"`
	OfflineImageURL string `json:"offline_image_url,omitempty"`
	ViewCount       int    `json:"view_count,omitempty"`
	Email           string `json:"email,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
}

// PlatformID returns the platform ID
func (t TwitchData) PlatformID() string {
	return t.ID
}

// PlatformUsername returns the platform username
func (t TwitchData) PlatformUsername() string {
	return t.Login
}

// PlatformData returns the platform data
func (t TwitchData) PlatformData() string {
	data, _ := json.Marshal(t)
	return string(data)
}

// CreateLinkedAccount creates a linked account
func (t TwitchData) CreateLinkedAccount(userID string) *LinkedAccount {
	return NewLinkedAccount(userID, PlatformTwitch, t.Login, t.ID, t)
}

// -------------- Functions --------------

// TwitchExtCodeForToken returns the Twitch extension code for token
func TwitchExtCodeForToken(code string) (*TwitchTokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", TWITCH_CLIENT_ID)
	data.Set("client_secret", TWITCH_CLIENT_SECRET)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", TWITCH_REDIRECT_URI)

	req, err := http.NewRequest("POST", "https://id.twitch.tv/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body := make(map[string]interface{})
		json.NewDecoder(resp.Body).Decode(&body)
		log.Println(body)
		return nil, errors.New("failed to exchange code for access token")
	}

	var token TwitchTokenResponse
	err = json.NewDecoder(resp.Body).Decode(&token)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// TwitchRefreshToken refreshes an access token
func TwitchRefreshToken(refreshToken string) (*TwitchTokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", TWITCH_CLIENT_ID)
	data.Set("client_secret", TWITCH_CLIENT_SECRET)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequest("POST", TWITCH_API_ENDPOINT+"/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to refresh access token")
	}

	var token TwitchTokenResponse
	err = json.NewDecoder(resp.Body).Decode(&token)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// TwitchRevokeToken revokes an access token
func TwitchRevokeToken(accessToken string) error {
	data := url.Values{}
	data.Set("client_id", TWITCH_CLIENT_ID)
	data.Set("token", accessToken)

	req, err := http.NewRequest("POST", TWITCH_API_ENDPOINT+"/oauth2/revoke", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to revoke access token")
	}
	return nil
}

// GetTwitchUser returns the user data
func GetTwitchUser(accessToken string) (*TwitchData, error) {
	req, err := http.NewRequest("GET", TWITCH_API_ENDPOINT+"/users", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Client-ID", TWITCH_CLIENT_ID)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to get user data")
	}

	var data struct {
		Data []TwitchData `json:"data"`
	}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}
	return &data.Data[0], nil
}

// TwitchOAuth process the Twitch OAuth flow
func TwitchOAuth(as auth.AccountStore, ss sess.SessionStore, las LinkAccountStore, code, state string) (*sess.Session, error) {
	var a *auth.Account
	// TODO: Sign the state so it can't be tampered with/impersonated
	if state != "" && false { // TEMPORARILY DISABLED UNTIL STATE IS SIGNED
	}

	token, err := TwitchExtCodeForToken(code)
	if err != nil {
		log.Println("Failed to exchange code for token")
		return nil, err
	}

	user, err := GetTwitchUser(token.AccessToken)
	if err != nil {
		log.Println("Failed to get user from Twitch API")
		return nil, err
	}

	// Check if platform account is linked to an account
	la, err := las.GetLinkedAccountByPlatformID(PlatformTwitch, user.ID)
	if err == nil {
		// If the account IDs don't match, default to OAuth as the source of truth
		if a == nil || a.UserID != la.UserID {
			a, err = as.GetAccountByID(la.UserID)
			if err != nil {
				return nil, err
			}
			session, err := a.NewSession(time.Now().Add(time.Hour * 24).Unix())
			if err != nil {
				return nil, err
			}
			ss.AddSessionToCache(session)
			defer ss.AddSessionToDB(session)
			return session, nil
		} else if a.UserID == la.UserID {
			session, err := a.NewSession(time.Now().Add(time.Hour * 24).Unix())
			if err != nil {
				return nil, err
			}
			ss.AddSessionToCache(session)
			defer ss.AddSessionToDB(session)
			return session, nil
		}
	}

	// Check if the email is already in use -- simple account merging
	a, _ = as.GetAccountByEmail(user.Email)
	if a == nil {
		// Create account
		a, err = auth.NewPasswordLessAccount(user.Login, user.Email)
		if err != nil {
			return nil, err
		}
		a, err = as.AddAccountToDB(a)
		if err != nil {
			return nil, err
		}
	}

	// Link account
	la = NewLinkedAccount(a.UserID, PlatformTwitch, user.Login, user.ID, user)
	_, err = las.AddLinkedAccountToDB(la)
	if err != nil {
		return nil, errors.New("failed to link account")
	}
	session, err := a.NewSession(time.Now().Add(time.Hour * 24).Unix())
	if err != nil {
		return nil, err
	}
	ss.AddSessionToCache(session)
	defer ss.AddSessionToDB(session)
	return session, nil
}
