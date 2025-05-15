package linking

import (
	"errors"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/goccy/go-json"
	"github.com/nicklaw5/helix/v2"
	"golang.org/x/oauth2"
	"os"
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
func (t *TwitchData) CreateLinkedAccount(userID string) *auth.LinkedAccount {
	return auth.NewLinkedAccount(userID, auth.PlatformTwitch, t.Login, t.ID, t)
}

// -------------- Functions --------------

// GetTwitchUser returns the Twitch user data
func GetTwitchUser(token *auth.OAuthToken) (*TwitchData, error) {
	client, err := helix.NewClient(&helix.Options{
		ClientID:        TWITCH_CLIENT_ID,
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
	return &TwitchData{user}, nil
}
