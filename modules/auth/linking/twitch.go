package linking

import (
	"context"
	"errors"
	"github.com/nicklaw5/helix/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
	"log"
	"os"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/goccy/go-json"
)

// -------------- Global Variables --------------

//goland:noinspection GoSnakeCaseUsage
var (
	TWITCH_CLIENT_ID     = os.Getenv("TWITCH_CLIENT_ID")
	TWITCH_CLIENT_SECRET = os.Getenv("TWITCH_CLIENT_SECRET")
	TWITCH_REDIRECT_URI  = os.Getenv("TWITCH_REDIRECT_URI")
	twitchConfig         = &oauth2.Config{
		ClientID:     TWITCH_CLIENT_ID,
		ClientSecret: TWITCH_CLIENT_SECRET,
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://id.twitch.tv/oauth2/authorize",
			TokenURL:  "https://id.twitch.tv/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
		RedirectURL: TWITCH_REDIRECT_URI,
	}
)

// -------------- Structs --------------

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
func TwitchExtCodeForToken(code string) (*ScopedToken, error) {
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

	scope, ok := token.Extra("scope").([]interface{})
	if !ok {
		return nil, errors.New("failed to get scope from token")
	}
	var scopes []string
	for _, s := range scope {
		if str, ok := s.(string); ok {
			scopes = append(scopes, str)
		}
	}

	var scopedToken = &ScopedToken{
		Token: token,
		Scope: scopes,
	}

	return scopedToken, nil
}

// GetTwitchUser returns the Twitch user data
func GetTwitchUser(accessToken string) (*TwitchData, error) {
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

	token, err := ExtCodeForToken(twitchConfig, code)
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
