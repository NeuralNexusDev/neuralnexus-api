package link

import (
	"errors"
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
	TwitchConfig  = &oauth2.Config{
		ClientID:     CLIENT_ID,
		ClientSecret: CLIENT_SECRET,
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://id.twitch.tv/oauth2/authorize",
			TokenURL:  "https://id.twitch.tv/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
		RedirectURL: REDIRECT_URI,
	}
	// APIBaseURL overrides helix's default Twitch API base URL, letting
	// tests point it at a local server. Empty uses helix's own default.
	APIBaseURL string
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
func GetTwitchUser(token *OAuthToken) (*TwitchData, error) {
	client, err := helix.NewClient(&helix.Options{
		ClientID:        CLIENT_ID,
		UserAccessToken: token.AccessToken,
		APIBaseURL:      APIBaseURL,
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
