package linking

import (
	"context"
	"errors"
	"github.com/nicklaw5/helix/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/goccy/go-json"
)

// -------------- Global Variables --------------
//
//goland:noinspection GoSnakeCaseUsage
var (
	TWITCH_CLIENT_ID     = os.Getenv("TWITCH_CLIENT_ID")
	TWITCH_CLIENT_SECRET = os.Getenv("TWITCH_CLIENT_SECRET")
	TWITCH_REDIRECT_URI  = os.Getenv("TWITCH_REDIRECT_URI")
	TWITCH_API_ENDPOINT  = "https://api.twitch.tv/helix"
)

// -------------- Structs --------------

// TwitchTokenResponse struct
type TwitchTokenResponse struct {
	*oauth2.Token
	Scope []string `json:"scope"`
}

// TwitchData struct
type TwitchData struct {
	*helix.User
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
func (t TwitchData) CreateLinkedAccount(userID string) *auth.LinkedAccount {
	return auth.NewLinkedAccount(userID, auth.PlatformTwitch, t.Login, t.ID, t)
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

	req, err := http.NewRequest("POST", endpoints.Twitch.TokenURL, strings.NewReader(data.Encode()))
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

func UpdatedTwitchExtCodeForToken(code string) (*TwitchTokenResponse, error) {
	config := &oauth2.Config{
		ClientID:     TWITCH_CLIENT_ID,
		ClientSecret: TWITCH_CLIENT_SECRET,
		Endpoint:     endpoints.Twitch,
		RedirectURL:  TWITCH_REDIRECT_URI,
	}
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, errors.New("failed to exchange code for access token")
	}

	var twitchToken = &TwitchTokenResponse{
		Token: token,
		Scope: token.Extra("scope").([]string),
	}

	return twitchToken, nil
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

func UpdatedGetTwitchUser(accessToken string) (*TwitchData, error) {
	client, err := helix.NewClient(&helix.Options{
		ClientID:        TWITCH_CLIENT_ID,
		UserAccessToken: accessToken,
	})
	if err != nil {
		return nil, err
	}

	users, err := client.GetUsers(&helix.UsersParams{})
	if err != nil {
		return nil, err
	}
	if len(users.Data.Users) == 0 {
		return nil, errors.New("failed to get user data")
	}

	user := &users.Data.Users[0]
	return &TwitchData{user}, nil
}

// TwitchOAuth process the Twitch OAuth flow
func TwitchOAuth(store auth.Store, code string, state auth.OAuthState) (*auth.Session, error) {
	var a *auth.Account
	// TODO: Sign the state so it can't be tampered with/impersonated
	if state.Platform != "" && false { // TEMPORARILY DISABLED UNTIL STATE IS SIGNED
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
	la, err := store.LinkAccount().GetLinkedAccountByPlatformID(auth.PlatformTwitch, user.ID)
	if err == nil {
		// If the account IDs don't match, default to OAuth as the source of truth
		if a == nil || a.UserID != la.UserID {
			a, err = store.Account().GetAccountByID(la.UserID)
			if err != nil {
				return nil, err
			}
			session, err := a.NewSession(time.Now().Add(time.Hour * 24).Unix())
			if err != nil {
				return nil, err
			}
			store.Session().AddSessionToCache(session)
			defer store.Session().AddSessionToDB(session)
			return session, nil
		} else if a.UserID == la.UserID {
			session, err := a.NewSession(time.Now().Add(time.Hour * 24).Unix())
			if err != nil {
				return nil, err
			}
			store.Session().AddSessionToCache(session)
			defer store.Session().AddSessionToDB(session)
			return session, nil
		}
	}

	// Check if the email is already in use -- simple account merging
	a, _ = store.Account().GetAccountByEmail(user.Email)
	if a == nil {
		// Create account
		a, err = auth.NewPasswordLessAccount(user.Login, user.Email)
		if err != nil {
			return nil, err
		}
		err = store.Account().AddAccountToDB(a)
		if err != nil {
			return nil, err
		}
	}

	// Link account
	la = auth.NewLinkedAccount(a.UserID, auth.PlatformTwitch, user.Login, user.ID, user)
	err = store.LinkAccount().AddLinkedAccountToDB(la)
	if err != nil {
		return nil, errors.New("failed to link account")
	}
	session, err := a.NewSession(time.Now().Add(time.Hour * 24).Unix())
	if err != nil {
		return nil, err
	}
	store.Session().AddSessionToCache(session)
	defer store.Session().AddSessionToDB(session)
	return session, nil
}
