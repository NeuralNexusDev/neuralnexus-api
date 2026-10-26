package config

import (
	"encoding/hex"
	"log"
	"os"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/oauth2"
)

func checkEnvVar(varName string) string {
	envVar := os.Getenv(varName)
	if envVar == "" {
		log.Fatal(varName + " is not set")
	}
	return envVar
}

type Config struct {
	ApiUrl    string
	SiteUrl   string
	Redis     Redis
	Auth      Auth
	Discord   Discord
	Microsoft Microsoft
	Steam     Steam
	Twitch    Twitch
}

type Redis struct {
	Address  string
	Username string
	Password string
}

type Auth struct {
	Account Account
	Session Session
}

type Account struct {
	Pepper []byte
}

type Session struct {
	JwtSecret      []byte
	ValidAudiences []string
}

type Discord struct {
	ClientId     string
	ClientSecret string
	RedirectUri  string
	PublicKey    ed25519.PublicKey
	OAuth2Config *oauth2.Config
}

type Microsoft struct {
	ClientId          string
	ClientSecret      string
	RedirectUri       string
	OAuth2Config      *oauth2.Config
	LoginOAuth2Config *oauth2.Config
}

type Steam struct {
	APIKey string
}

type Twitch struct {
	ClientId       string
	ClientSecret   string
	RedirectUri    string
	UserId         string
	EventSubUri    string
	EventSubSecret string
	APIBaseUrl     string
	OAuth2Config   *oauth2.Config
}

func FromEnv() *Config {
	apiUrl := checkEnvVar("NN_API_URL")
	siteUrl := checkEnvVar("NN_SITE_URL")

	// ---------- Discord ---------
	discordClientId := checkEnvVar("DISCORD_CLIENT_ID")
	discordClientSecret := checkEnvVar("DISCORD_CLIENT_SECRET")
	discordRedirectUri := checkEnvVar("DISCORD_REDIRECT_URI")
	publicKey, err := hex.DecodeString(checkEnvVar("DISCORD_PUBLIC_KEY"))
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		log.Fatal("DISCORD_PUBLIC_KEY is not the correct length")
		return nil
	}

	// ---------- Microsoft ---------
	microsoftClientId := checkEnvVar("MICROSOFT_CLIENT_ID")
	microsoftClientSecret := checkEnvVar("MICROSOFT_CLIENT_SECRET")
	microsoftRedirectUri := checkEnvVar("MICROSOFT_REDIRECT_URI")

	// ---------- Twitch ----------
	twitchClientId := checkEnvVar("TWITCH_CLIENT_ID")
	twitchClientSecret := checkEnvVar("TWITCH_CLIENT_SECRET")
	twitchRedirectUri := checkEnvVar("TWITCH_REDIRECT_URI")

	return &Config{
		ApiUrl:  apiUrl,
		SiteUrl: siteUrl,
		Redis: Redis{
			Address:  checkEnvVar("REDIS_ADDRESS"),
			Username: checkEnvVar("REDIS_USERNAME"),
			Password: checkEnvVar("REDIS_PASSWORD"),
		},
		Auth: Auth{
			Account: Account{
				Pepper: []byte(checkEnvVar("PEPPER")),
			},
			Session: Session{
				JwtSecret:      []byte(checkEnvVar("JWT_SECRET")),
				ValidAudiences: []string{siteUrl, apiUrl},
			},
		},
		Discord: Discord{
			ClientId:     discordClientId,
			ClientSecret: discordClientSecret,
			RedirectUri:  discordRedirectUri,
			PublicKey:    publicKey,
			OAuth2Config: &oauth2.Config{
				ClientID:     discordClientId,
				ClientSecret: discordClientSecret,
				RedirectURL:  discordRedirectUri,
				Endpoint: oauth2.Endpoint{
					AuthURL:   "https://discord.com/oauth2/authorize",
					TokenURL:  "https://discord.com/api/oauth2/token",
					AuthStyle: oauth2.AuthStyleInHeader,
				},
			},
		},
		Microsoft: Microsoft{
			ClientId:     microsoftClientId,
			ClientSecret: microsoftClientSecret,
			RedirectUri:  microsoftRedirectUri,
			OAuth2Config: &oauth2.Config{
				ClientID:     microsoftClientId,
				ClientSecret: microsoftClientSecret,
				RedirectURL:  microsoftRedirectUri,
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize",
					TokenURL: "https://login.microsoftonline.com/consumers/oauth2/v2.0/token",
				},
				Scopes: []string{"XboxLive.signin", "offline_access"},
			},
			LoginOAuth2Config: &oauth2.Config{
				ClientID:     microsoftClientId,
				ClientSecret: microsoftClientSecret,
				RedirectURL:  microsoftRedirectUri,
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize",
					TokenURL: "https://login.microsoftonline.com/consumers/oauth2/v2.0/token",
				},
				Scopes: []string{"openid", "profile", "email", "offline_access"},
			},
		},
		Steam: Steam{
			APIKey: checkEnvVar("STEAM_API_KEY"),
		},
		Twitch: Twitch{
			ClientId:       twitchClientId,
			ClientSecret:   twitchClientSecret,
			RedirectUri:    twitchRedirectUri,
			UserId:         checkEnvVar("TWITCH_USER_ID"),
			EventSubUri:    checkEnvVar("TWITCH_EVENTSUB_URI"),
			EventSubSecret: checkEnvVar("TWITCH_EVENTSUB_SECRET"),
			APIBaseUrl:     "",
			OAuth2Config: &oauth2.Config{
				ClientID:     twitchClientId,
				ClientSecret: twitchClientSecret,
				RedirectURL:  twitchRedirectUri,
				Endpoint: oauth2.Endpoint{
					AuthURL:   "https://id.twitch.tv/oauth2/authorize",
					TokenURL:  "https://id.twitch.tv/oauth2/token",
					AuthStyle: oauth2.AuthStyleInParams,
				},
			},
		},
	}
}
