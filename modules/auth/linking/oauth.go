package linking

import (
	"context"
	"errors"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"golang.org/x/oauth2"
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
