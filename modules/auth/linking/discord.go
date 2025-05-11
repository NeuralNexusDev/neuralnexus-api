package linking

import (
	"errors"
	"github.com/bwmarrin/discordgo"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/goccy/go-json"
)

// -------------- Global Variables --------------

//goland:noinspection GoSnakeCaseUsage
var (
	DISCORD_CLIENT_ID     = os.Getenv("DISCORD_CLIENT_ID")
	DISCORD_CLIENT_SECRET = os.Getenv("DISCORD_CLIENT_SECRET")
	DISCORD_REDIRECT_URI  = os.Getenv("DISCORD_REDIRECT_URI")
	DISCORD_API_ENDPOINT  = "https://discord.com/api/v10"
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

// PlatformID returns the platform ID
func (d *DiscordData) PlatformID() string {
	return d.ID
}

// PlatformUsername returns the platform username
func (d *DiscordData) PlatformUsername() string {
	return d.Username
}

// PlatformData returns the platform data
func (d *DiscordData) PlatformData() string {
	data, _ := json.Marshal(d)
	return string(data)
}

// CreateLinkedAccount creates a linked account
func (d *DiscordData) CreateLinkedAccount(userID string) *auth.LinkedAccount {
	return auth.NewLinkedAccount(userID, auth.PlatformDiscord, d.Username, d.ID, d)
}

// -------------- Functions --------------

// OldGetDiscordUser gets a Discord user
func OldGetDiscordUser(accessToken string) (*DiscordData, error) {
	req, err := http.NewRequest("GET", DISCORD_API_ENDPOINT+"/users/@me", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("Failed to get Discord user:", resp.Status)
		return nil, errors.New("failed to get Discord user")
	}

	var data DiscordData
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

// GetDiscordUser gets a Discord user with the given access token
func GetDiscordUser(accessToken string) (*DiscordData, error) {
	discord, err := discordgo.New("Bearer " + accessToken)
	if err != nil {
		return nil, err
	}
	user, err := discord.User("@me")
	if err != nil {
		return nil, err
	}
	return &DiscordData{User: user}, nil
}

// DiscordOAuth process the Discord OAuth flow
func DiscordOAuth(store auth.Store, code string, state auth.OAuthState) (*auth.Session, error) {
	var a *auth.Account
	// TODO: Sign the state so it can't be tampered with/impersonated
	if state.Platform != "" && false { // TEMPORARILY DISABLED UNTIL STATE IS SIGNED
	}

	token, err := ExtCodeForToken(discordConfig, code)
	if err != nil {
		log.Println("Failed to exchange code for token")
		return nil, err
	}

	user, err := GetDiscordUser(token.AccessToken)
	if err != nil {
		log.Println("Failed to get user from Discord API")
		return nil, err
	}

	// Check if platform account is linked to an account
	la, err := store.LinkAccount().GetLinkedAccountByPlatformID(auth.PlatformDiscord, user.ID)
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
	a, _ = store.Account().GetAccountByEmail(user.Email)
	if a == nil {
		// Create account
		a, err = auth.NewPasswordLessAccount(user.Username, user.Email)
		if err != nil {
			return nil, err
		}
		err = store.Account().AddAccountToDB(a)
		if err != nil {
			return nil, err
		}
	}

	// Link account
	la = auth.NewLinkedAccount(a.UserID, auth.PlatformDiscord, user.Username, user.ID, user)
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
