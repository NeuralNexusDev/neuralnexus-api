package linking

import (
	"context"
	"errors"
	"golang.org/x/oauth2"
)

// -------------- Structs --------------

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
