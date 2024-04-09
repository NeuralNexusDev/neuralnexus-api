package account_linking

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"

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

// AccessTokenResponse struct
type AccessTokenResponse struct {
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

// ExchangeCodeForAccessToken exchanges a code for an access token
func ExchangeCodeForAccessToken(code string) (*AccessTokenResponse, error) {
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

	var token AccessTokenResponse
	err = json.NewDecoder(resp.Body).Decode(&token)
	if err != nil {
		return nil, err
	}

	return &token, nil
}

// RefreshAccessToken refreshes an access token
func RefreshAccessToken(refreshToken string) (*AccessTokenResponse, error) {
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

	var token AccessTokenResponse
	err = json.NewDecoder(resp.Body).Decode(&token)
	if err != nil {
		return nil, err
	}

	return &token, nil
}

// RevokeAccessToken revokes an access token
func RevokeAccessToken(accessToken string) error {
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
		return nil, errors.New("failed to get Discord user")
	}

	var user DiscordData
	err = json.NewDecoder(resp.Body).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}
