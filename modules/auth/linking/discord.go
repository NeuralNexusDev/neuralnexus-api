package accountlinking

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/google/uuid"
)

// -------------- Global Variables --------------
var (
	DISCORD_CLIENT_ID     = os.Getenv("DISCORD_CLIENT_ID")
	DISCORD_CLIENT_SECRET = os.Getenv("DISCORD_CLIENT_SECRET")
	DISCORD_REDIRECT_URI  = os.Getenv("DISCORD_REDIRECT_URI")
	DISCORD_API_ENDPOINT  = "https://discord.com/api/v10"
)

// -------------- Structs --------------

// DiscordTokenResponse struct
type DiscordTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	RereshToken string `json:"refresh_token"`
	Scope       string `json:"scope"`
}

// DiscordData struct
type DiscordData struct {
	ID               string `json:"id" validate:"required"`
	Username         string `json:"username" validate:"required"`
	Discriminator    string `json:"discriminator" validate:"required"`
	Avatar           string `json:"avatar,omitempty"`
	Bot              bool   `json:"bot,omitempty"`
	System           bool   `json:"system,omitempty"`
	MFAEnabled       bool   `json:"mfa_enabled,omitempty"`
	Banner           string `json:"banner,omitempty"`
	AccentColor      int    `json:"accent_color,omitempty"`
	Locale           string `json:"locale,omitempty"`
	Verified         bool   `json:"verified,omitempty"`
	Email            string `json:"email,omitempty"`
	Flags            int    `json:"flags,omitempty"`
	PremiumType      int    `json:"premium_type,omitempty"`
	PublicFlags      int    `json:"public_flags,omitempty"`
	AvatarDecoration string `json:"avatar_decoration,omitempty"`
}

// PlatformID returns the platform ID
func (d DiscordData) PlatformID() string {
	return d.ID
}

// PlatformUsername returns the platform username
func (d DiscordData) PlatformUsername() string {
	return d.Username
}

// PlatformData returns the platform data
func (d DiscordData) PlatformData() string {
	data, _ := json.Marshal(d)
	return string(data)
}

// CreateLinkedAccount creates a linked account
func (d DiscordData) CreateLinkedAccount(userID uuid.UUID) LinkedAccount {
	return NewLinkedAccount(userID, PlatformDiscord, d.Username, d.ID, d)
}

// -------------- Functions --------------

// DiscordExtCodeForToken exchanges a code for an access token
func DiscordExtCodeForToken(code string) (*DiscordTokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", DISCORD_REDIRECT_URI)

	req, err := http.NewRequest("POST", DISCORD_API_ENDPOINT+"/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(DISCORD_CLIENT_ID, DISCORD_CLIENT_SECRET)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to exchange code for access token")
	}

	var token DiscordTokenResponse
	err = json.NewDecoder(resp.Body).Decode(&token)
	if err != nil {
		return nil, err
	}

	return &token, nil
}

// DiscordRefreshToken refreshes an access token
func DiscordRefreshToken(refreshToken string) (*DiscordTokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequest("POST", DISCORD_API_ENDPOINT+"/oauth2/token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(DISCORD_CLIENT_ID, DISCORD_CLIENT_SECRET)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to refresh access token")
	}

	var token DiscordTokenResponse
	err = json.NewDecoder(resp.Body).Decode(&token)
	if err != nil {
		return nil, err
	}

	return &token, nil
}

// DiscordRevokeToken revokes an access token
func DiscordRevokeToken(accessToken string) error {
	data := url.Values{}
	data.Set("token", accessToken)

	req, err := http.NewRequest("POST", DISCORD_API_ENDPOINT+"/oauth2/token/revoke", strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(DISCORD_CLIENT_ID, DISCORD_CLIENT_SECRET)

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

// GetDiscordUser gets a Discord user
func GetDiscordUser(accessToken string) (*DiscordData, error) {
	req, err := http.NewRequest("GET", DISCORD_API_ENDPOINT+"/users/@me", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("Failed to get Discord user:", resp.Status)
		return nil, errors.New("failed to get Discord user")
	}

	var user DiscordData
	err = json.NewDecoder(resp.Body).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// DiscordOAuth process the Discord OAuth flow
func DiscordOAuth(code, state string) (*auth.Session, error) {
	var a *auth.Account
	// TODO: Sign the state so it can't be tampered with/impersonated
	if state != "" && false { // TEMPORARILY DISABLED
		// Get account by state (which is the user ID)
		id, err := uuid.Parse(state)
		if err != nil {
			log.Println("Failed to parse state as UUID")
			return nil, err
		}
		ad := auth.GetAccountByID(id)
		if !ad.Success {
			return nil, errors.New("failed to get account")
		}
		a = &ad.Data
	}

	token, err := DiscordExtCodeForToken(code)
	if err != nil {
		log.Println("Failed to exchange code for token")
		return nil, err
	}

	user, err := GetDiscordUser(token.AccessToken)
	if err != nil {
		log.Println("Failed to get user from Discord API")
		return nil, err
	}

	// Check if platform account is linked to an account
	lad := GetLinkedAccountByPlatformID(PlatformDiscord, user.ID)
	if lad.Success {
		// If the account IDs don't match, default to OAuth as the source of truth
		if a == nil || a.UserID != lad.Data.UserID {
			ad := auth.GetAccountByID(lad.Data.UserID)
			if !ad.Success {
				return nil, errors.New("failed to get account")
			}
			s := ad.Data.NewSession(time.Now().Add(time.Hour * 24).Unix())
			auth.AddSessionToCache(s)
			defer auth.AddSessionToDB(s)
			return &s, nil
		} else if a.UserID == lad.Data.UserID {
			s := a.NewSession(time.Now().Add(time.Hour * 24).Unix())
			auth.AddSessionToCache(s)
			defer auth.AddSessionToDB(s)
			return &s, nil
		}
	}

	// Check if the email is already in use -- simple account merging
	ad := auth.GetAccountByEmail(user.Email)
	if ad.Success {
		a = &ad.Data
	} else if a == nil {
		// Create account
		act := auth.NewPasswordLessAccount(user.Username, user.Email)
		a = &act
		dbResponse := auth.CreateAccount(*a)
		if !dbResponse.Success {
			return nil, errors.New("failed to create account")
		}
	}

	// Link account
	la := NewLinkedAccount(a.UserID, PlatformDiscord, user.Username, user.ID, user)
	linkAcctData := AddLinkedAccountToDB(la)
	if !linkAcctData.Success {
		return nil, errors.New("failed to link account")
	}
	s := a.NewSession(time.Now().Add(time.Hour * 24).Unix())
	auth.AddSessionToCache(s)
	defer auth.AddSessionToDB(s)
	return &s, nil
}
