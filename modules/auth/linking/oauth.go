package linking

import (
	"context"
	"errors"
	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"time"
)

// -------------- Structs --------------

// Mode describing how to handle the OAuth interaction
type Mode string

const (
	ModeLogin Mode = "login"
	ModeLink  Mode = "link"
)

// OAuthState used with the OAuth state URL parameter
type OAuthState struct {
	Platform    auth.Platform `json:"platform"`
	Nonce       string        `json:"nonce"`
	RedirectURI string        `json:"redirect_uri"`
	Mode        Mode          `json:"mode"`
}

// -------------- Functions --------------

// ExtCodeForToken exchanges the code for an access token and returns a auth.OAuthToken
func ExtCodeForToken(config *oauth2.Config, code string) (*auth.OAuthToken, error) {
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, errors.New("failed to exchange code for access token")
	}

	var scopes []string
	if rawScopes, ok := token.Extra("scope").([]interface{}); ok {
		for _, s := range rawScopes {
			if str, ok := s.(string); ok {
				scopes = append(scopes, str)
			}
		}
		// Discord is special and returns a single string if there's only one scope
	} else if rawScopes, ok := token.Extra("scope").(string); ok {
		scopes = []string{rawScopes}
	} else {
		return nil, errors.New("failed to get scope from token")
	}

	var scopedToken = &auth.OAuthToken{
		Token: token,
		Scope: scopes,
	}

	return scopedToken, nil
}

// RefreshToken refreshes the token and returns a auth.OAuthToken
func RefreshToken(config *oauth2.Config, token *oauth2.Token) (*auth.OAuthToken, error) {
	newToken, err := config.TokenSource(context.Background(), token).Token()
	if err != nil {
		return nil, err
	}
	if newToken == nil {
		return nil, errors.New("failed to refresh token")
	}

	var scopes []string
	if rawScopes, ok := newToken.Extra("scope").([]interface{}); ok {
		for _, s := range rawScopes {
			if str, ok := s.(string); ok {
				scopes = append(scopes, str)
			}
		}
	} else if rawScopes, ok := newToken.Extra("scope").(string); ok {
		scopes = []string{rawScopes}
	} else {
		return nil, errors.New("failed to get scope from token")
	}

	var scopedToken = &auth.OAuthToken{
		Token: newToken,
		Scope: scopes,
	}

	return scopedToken, nil
}

// DeferStoreSession adds a session to the session service and logs an error if it fails
func DeferStoreSession(ss auth.SessionService, session *auth.Session) {
	err := ss.AddSession(session)
	if err != nil {
		log.Println("failed to add session:\n\t", err)
	}
}

// ProcessOAuthLogin processes the OAuth2 code and returns a session
func ProcessOAuthLogin(as auth.AccountService, las auth.LinkAccountStore, ss auth.SessionService, code string, state *OAuthState) (*auth.Session, error) {
	var err error
	var config *oauth2.Config
	switch state.Platform {
	case auth.PlatformDiscord:
		config = discordConfig
	case auth.PlatformTwitch:
		config = twitchConfig
	default:
		return nil, errors.New("invalid platform")
	}
	var token *auth.OAuthToken
	token, err = ExtCodeForToken(config, code)
	if err != nil {
		return nil, err
	}

	var user auth.PlatformData
	switch state.Platform {
	case auth.PlatformDiscord:
		user, err = GetDiscordUser(token)
	case auth.PlatformTwitch:
		user, err = GetTwitchUser(token)
	default:
		return nil, errors.New("invalid platform")
	}
	if err != nil {
		return nil, err
	}

	var a *auth.Account
	var la *auth.LinkedAccount
	var session *auth.Session
	la, err = las.GetLinkedAccountByPlatformID(state.Platform, user.GetID())
	if err != nil {
		a, err = auth.NewPasswordLessAccount(user.GetUsername(), user.GetEmail())
		if err != nil {
			return nil, err
		}
		err = as.AddAccount(a)
		if err != nil {
			return nil, err
		}
	} else {
		a, err = as.GetAccountByID(la.UserID)
		if err != nil {
			return nil, err
		}
	}

	session, err = a.NewSession(time.Now().Add(time.Hour * 24).Unix())
	if err != nil {
		return nil, err
	}

	defer DeferStoreSession(ss, session)
	return session, nil
}

// ProcessOAuthLink links an account to an existing user
func ProcessOAuthLink(r *http.Request, las auth.LinkAccountStore, code string, state *OAuthState) (*auth.Session, error) {
	var err error
	var config *oauth2.Config
	switch state.Platform {
	case auth.PlatformDiscord:
		config = discordConfig
	case auth.PlatformTwitch:
		config = twitchConfig
	default:
		return nil, errors.New("invalid platform")
	}
	var token *auth.OAuthToken
	token, err = ExtCodeForToken(config, code)
	if err != nil {
		return nil, err
	}

	var user auth.PlatformData
	switch state.Platform {
	case auth.PlatformDiscord:
		user, err = GetDiscordUser(token)
	case auth.PlatformTwitch:
		user, err = GetTwitchUser(token)
	default:
		return nil, errors.New("invalid platform")
	}
	if err != nil {
		return nil, err
	}

	// Get session from request context
	session, ok := r.Context().Value(mw.SessionKey).(*auth.Session)
	if !ok || session == nil {
		return nil, errors.New("session not found")
	}
	if session.IsValid() {
		return nil, errors.New("session expired")
	}

	// Check if platform account is linked to an account
	la, err := las.GetLinkedAccountByPlatformID(state.Platform, user.GetID())
	if err == nil {
		// Return an error if the linked account is not the same as the current session
		if session.UserID != la.UserID {
			return nil, errors.New("platform account already linked to another account")
		}
	}

	// Link account
	la = auth.NewLinkedAccount(session.UserID, state.Platform, user.GetUsername(), user.GetID(), user)
	err = las.AddLinkedAccountToDB(la)
	if err != nil {
		return nil, errors.New("failed to link account")
	}

	return session, nil
}
