package twitch

import (
	"errors"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/goccy/go-json"
	"github.com/nicklaw5/helix/v2"
	"golang.org/x/oauth2"
	"os"
)

//goland:noinspection GoSnakeCaseUsage
var (
	USER_ID       = os.Getenv("TWITCH_USER_ID")
	CLIENT_ID     = os.Getenv("TWITCH_CLIENT_ID")
	CLIENT_SECRET = os.Getenv("TWITCH_CLIENT_SECRET")
	REDIRECT_URI  = os.Getenv("TWITCH_REDIRECT_URI")
	Config        = &oauth2.Config{
		ClientID:     CLIENT_ID,
		ClientSecret: CLIENT_SECRET,
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://id.twitch.tv/oauth2/authorize",
			TokenURL:  "https://id.twitch.tv/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
		RedirectURL: REDIRECT_URI,
	}
)

// Data struct
type Data struct {
	*helix.User
}

// GetID returns the platform ID
func (t *Data) GetID() string {
	return t.ID
}

// GetEmail returns the platform email
func (t *Data) GetEmail() string {
	return t.Email
}

// GetUsername returns the platform username
func (t *Data) GetUsername() string {
	return t.Login
}

// GetData returns the platform data
func (t *Data) GetData() string {
	data, _ := json.Marshal(t)
	return string(data)
}

// CreateLinkedAccount creates a linked account
func (t *Data) CreateLinkedAccount(userID string) *auth.LinkedAccount {
	return auth.NewLinkedAccount(userID, auth.PlatformTwitch, t.Login, t.ID, t)
}

// GetUser returns the Twitch user data
func GetUser(token *auth.OAuthToken) (*Data, error) {
	client, err := helix.NewClient(&helix.Options{
		ClientID:        CLIENT_ID,
		UserAccessToken: token.AccessToken,
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
	return &Data{user}, nil
}
