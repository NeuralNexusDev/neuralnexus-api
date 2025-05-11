package linking

import (
	"context"
	"errors"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"golang.org/x/oauth2"
	"time"
)

// -------------- Structs --------------

// OAuthState used with the OAuth state URL parameter
type OAuthState struct {
	Platform    auth.Platform `json:"platform"`
	Nonce       string        `json:"nonce"`
	RedirectURI string        `json:"redirect_uri"`
}

// ScopedToken OAuth2 token with scope
type ScopedToken struct {
	*oauth2.Token
	Scope []string `json:"scope"`
}

// -------------- Functions --------------

// ExtCodeForToken exchanges the code for an access token and returns a ScopedToken
func ExtCodeForToken(config *oauth2.Config, code string) (*ScopedToken, error) {
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
	} else if rawScopes, ok := token.Extra("scope").(string); ok {
		scopes = []string{rawScopes}
	} else {
		return nil, errors.New("failed to get scope from token")
	}

	var scopedToken = &ScopedToken{
		Token: token,
		Scope: scopes,
	}

	return scopedToken, nil
}

// ProcessOAuth processes the OAuth2 code and returns a session
func ProcessOAuth(store auth.Store, code string, state *OAuthState) (*auth.Session, error) {
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
	var token *ScopedToken
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
		err = errors.New("invalid platform")
	}
	if err != nil {
		return nil, err
	}

	var a *auth.Account
	// Check if platform account is linked to an account
	la, err := store.LinkAccount().GetLinkedAccountByPlatformID(state.Platform, user.GetID())
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
	a, _ = store.Account().GetAccountByEmail(user.GetEmail())
	if a == nil {
		// Create account
		a, err = auth.NewPasswordLessAccount(user.GetUsername(), user.GetEmail())
		if err != nil {
			return nil, err
		}
		err = store.Account().AddAccountToDB(a)
		if err != nil {
			return nil, err
		}
	}

	// Link account
	la = auth.NewLinkedAccount(a.UserID, state.Platform, user.GetUsername(), user.GetID(), user)
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
