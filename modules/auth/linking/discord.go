package linking

import (
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/bwmarrin/discordgo"
	"github.com/goccy/go-json"
	"golang.org/x/oauth2"
	"os"
)

// -------------- Global Variables --------------

//goland:noinspection GoSnakeCaseUsage
var (
	DISCORD_CLIENT_ID     = os.Getenv("DISCORD_CLIENT_ID")
	DISCORD_CLIENT_SECRET = os.Getenv("DISCORD_CLIENT_SECRET")
	DISCORD_REDIRECT_URI  = os.Getenv("DISCORD_REDIRECT_URI")
	discordConfig         = &oauth2.Config{
		ClientID:     DISCORD_CLIENT_ID,
		ClientSecret: DISCORD_CLIENT_SECRET,
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://discord.com/oauth2/authorize",
			TokenURL:  "https://discord.com/api/oauth2/token",
			AuthStyle: oauth2.AuthStyleInHeader,
		},
		RedirectURL: DISCORD_REDIRECT_URI,
	}
)

// -------------- Structs --------------

// DiscordData struct
type DiscordData struct {
	*discordgo.User
}

// GetID returns the platform ID
func (d *DiscordData) GetID() string {
	return d.ID
}

// GetUsername returns the platform username
func (d *DiscordData) GetUsername() string {
	return d.Username
}

// GetEmail returns the platform email
func (d *DiscordData) GetEmail() string {
	return d.Email
}

// GetData returns the platform data
func (d *DiscordData) GetData() string {
	data, _ := json.Marshal(d)
	return string(data)
}

// CreateLinkedAccount creates a linked account
func (d *DiscordData) CreateLinkedAccount(userID string) *auth.LinkedAccount {
	return auth.NewLinkedAccount(userID, auth.PlatformDiscord, d.Username, d.ID, d)
}

// -------------- Functions --------------

// GetDiscordUser gets a Discord user with the given access token
func GetDiscordUser(token *auth.OAuthToken) (*DiscordData, error) {
	discord, err := discordgo.New("Bearer " + token.AccessToken)
	if err != nil {
		return nil, err
	}
	user, err := discord.User("@me")
	if err != nil {
		return nil, err
	}
	return &DiscordData{User: user}, nil
}
