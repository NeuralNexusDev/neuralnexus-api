package linking

import (
	"context"
	"errors"
	"fmt"
	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/twitch"
	"golang.org/x/oauth2"
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
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry.Unix(),
		ExpiresIn:    token.ExpiresIn,
		Scope:        scopes,
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
		AccessToken:  newToken.AccessToken,
		TokenType:    newToken.TokenType,
		RefreshToken: newToken.RefreshToken,
		Expiry:       newToken.Expiry.Unix(),
		ExpiresIn:    newToken.ExpiresIn,
		Scope:        scopes,
	}

	return scopedToken, nil
}

// ProcessOAuthLogin processes the OAuth2 code and returns a session
func ProcessOAuthLogin(as auth.AccountService, las auth.LinkAccountStore, ss auth.SessionService, code string, state *OAuthState) (*auth.Session, error) {
	var err error
	var config *oauth2.Config
	switch state.Platform {
	case auth.PlatformDiscord:
		config = discordConfig
	case auth.PlatformTwitch:
		config = twitch.Config
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
		user, err = twitch.GetUser(token)
	default:
		return nil, errors.New("invalid platform")
	}
	if err != nil {
		return nil, err
	}

	a, err := resolveOrCreateAccountForPlatformUser(as, las, state.Platform, user)
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

// resolveOrCreateAccountForPlatformUser resolves the auth.Account linked to
// the given platform user, creating a new placeholder account and linking it
// if none exists yet. If AddLinkedAccountToDB fails for any reason, the
// placeholder account created above is now orphaned and is cleaned up before
// deciding how to handle the error: on auth.ErrAlreadyLinked (a concurrent
// request won the race to link this exact platform account first) the
// winner's linked account/account are re-fetched and returned instead of
// treating it as a hard failure; any other error is returned as-is (wrapped
// together with a cleanup failure, if the cleanup itself also failed).
func resolveOrCreateAccountForPlatformUser(as auth.AccountService, las auth.LinkAccountStore, platform auth.Platform, user auth.PlatformData) (*auth.Account, error) {
	la, err := las.GetLinkedAccountByPlatformID(platform, user.GetID())
	if err == nil {
		return as.GetAccountByID(la.UserID)
	}
	if !errors.Is(err, auth.ErrNotFound) {
		return nil, err
	}

	a, err := auth.NewPasswordLessAccount(user.GetUsername(), user.GetEmail())
	if err != nil {
		return nil, err
	}
	if err := as.AddAccount(a); err != nil {
		return nil, err
	}

	la = auth.NewLinkedAccount(a.UserID, platform, user.GetUsername(), user.GetID(), user)
	if err := las.AddLinkedAccountToDB(la); err != nil {
		// Whatever went wrong, the account created above is now orphaned -
		// clean it up before deciding how to handle err.
		if delErr := as.DeleteAccount(a.UserID); delErr != nil {
			return nil, fmt.Errorf("failed to link account (%w) and failed to clean up the orphaned placeholder account: %w", err, delErr)
		}
		if !errors.Is(err, auth.ErrAlreadyLinked) {
			return nil, err
		}
		// Lost the race: use the winner's account instead.
		la, err = las.GetLinkedAccountByPlatformID(platform, user.GetID())
		if err != nil {
			return nil, err
		}
		a, err = as.GetAccountByID(la.UserID)
		if err != nil {
			return nil, err
		}
	}
	return a, nil
}

// ProcessOAuthLink links an account to an existing user
func ProcessOAuthLink(r *http.Request, las auth.LinkAccountStore, code string, state *OAuthState) (*auth.Session, error) {
	// Get session from request context first: there's no point exchanging
	// the OAuth code or calling out to the platform's API for a session
	// that's missing or already expired.
	session, ok := r.Context().Value(mw.SessionKey).(*auth.Session)
	if !ok || session == nil {
		return nil, errors.New("session not found")
	}
	if !session.IsValid() {
		return nil, errors.New("session expired")
	}

	var err error
	var config *oauth2.Config
	switch state.Platform {
	case auth.PlatformDiscord:
		config = discordConfig
	case auth.PlatformTwitch:
		config = twitch.Config
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
		user, err = twitch.GetUser(token)
	default:
		return nil, errors.New("invalid platform")
	}
	if err != nil {
		return nil, err
	}

	return linkPlatformUserToSession(las, session, state.Platform, user)
}

// linkPlatformUserToSession links the given platform identity to the
// session's account. If it's already linked to a different account, that's
// returned as an error; if it's already linked to this same account, linking
// is a no-op.
func linkPlatformUserToSession(las auth.LinkAccountStore, session *auth.Session, platform auth.Platform, user auth.PlatformData) (*auth.Session, error) {
	la, err := las.GetLinkedAccountByPlatformID(platform, user.GetID())
	switch {
	case err == nil:
		if session.UserID == la.UserID {
			return session, nil
		}
		return nil, errors.New("this platform account is already linked to a different account; log in with it directly if you want to use that account, or unlink it there first")
	case !errors.Is(err, auth.ErrNotFound):
		return nil, err
	}

	// Link account
	la = auth.NewLinkedAccount(session.UserID, platform, user.GetUsername(), user.GetID(), user)
	if err := las.AddLinkedAccountToDB(la); err != nil {
		return nil, errors.New("failed to link account")
	}

	return session, nil
}
