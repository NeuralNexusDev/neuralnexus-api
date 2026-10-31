package link

import (
	"context"
	"errors"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/goccy/go-json"
	"github.com/nicklaw5/helix/v2"
)

// TwitchData struct
type TwitchData struct {
	*helix.User
}

// GetID returns the platform ID
func (t *TwitchData) GetID() string {
	return t.ID
}

// GetEmail returns the platform email
func (t *TwitchData) GetEmail() string {
	return t.Email
}

// GetUsername returns the platform username
func (t *TwitchData) GetUsername() string {
	return t.Login
}

// GetData returns the platform data
func (t *TwitchData) GetData() string {
	data, _ := json.Marshal(t)
	return string(data)
}

// CreateLinkedAccount creates a linked account
func (t *TwitchData) CreateLinkedAccount(userID string) *LinkedAccount {
	return NewLinkedAccount(userID, PlatformTwitch, t.Login, t.ID, t)
}

// GetTwitchUser returns the Twitch user data
func GetTwitchUser(ctx context.Context, cfg config.Twitch, token *OAuthToken) (*TwitchData, error) {
	client, err := helix.NewClientWithContext(ctx, &helix.Options{
		ClientID:        cfg.ClientId,
		UserAccessToken: token.AccessToken,
		APIBaseURL:      cfg.APIBaseUrl,
	})
	if err != nil {
		panic("helix.NewClientWithContext failed although config.FromEnv requires TWITCH_CLIENT_ID: " + err.Error())
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
