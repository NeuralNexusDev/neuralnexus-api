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
	Database  Database
	Redis     Redis
	S3        S3
	Auth      Auth
	Discord   Discord
	GitHub    GitHub
	Microsoft Microsoft
	Steam     Steam
	Twitch    Twitch
}

type Database struct {
	DBURL  string
	DBURL2 string
}

type Redis struct {
	Address  string
	Username string
	Password string
}

type S3 struct {
	APIUrl     string
	MCATexture MCATexture
}

type MCATexture struct {
	Bucket    string
	AccessKey string
	SecretKey string
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
	PublicKey    ed25519.PublicKey
	OAuth2Config *oauth2.Config
}

type GitHub struct {
	Token string
}

type Microsoft struct {
	OAuth2Config      *oauth2.Config
	LoginOAuth2Config *oauth2.Config
}

type Steam struct {
	APIKey string
}

type Twitch struct {
	ClientId       string
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

	return &Config{
		ApiUrl:  apiUrl,
		SiteUrl: siteUrl,
		Database: Database{
			DBURL:  checkEnvVar("DATABASE_URL"),
			DBURL2: checkEnvVar("DATABASE_URL_2"),
		},
		Redis: Redis{
			Address:  checkEnvVar("REDIS_ADDRESS"),
			Username: checkEnvVar("REDIS_USERNAME"),
			Password: checkEnvVar("REDIS_PASSWORD"),
		},
		S3: S3{
			APIUrl: checkEnvVar("S3_API_URL"),
			MCATexture: MCATexture{
				Bucket:    "mca",
				AccessKey: checkEnvVar("S3_ACCESS_KEY_MCA_TEXTURE"),
				SecretKey: checkEnvVar("S3_SECRET_KEY_MCA_TEXTURE"),
			},
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
			PublicKey: publicKey,
			OAuth2Config: &oauth2.Config{
				ClientID:     checkEnvVar("DISCORD_CLIENT_ID"),
				ClientSecret: checkEnvVar("DISCORD_CLIENT_SECRET"),
				RedirectURL:  checkEnvVar("DISCORD_REDIRECT_URI"),
				Endpoint: oauth2.Endpoint{
					AuthURL:   "https://discord.com/oauth2/authorize",
					TokenURL:  "https://discord.com/api/oauth2/token",
					AuthStyle: oauth2.AuthStyleInHeader,
				},
			},
		},
		GitHub: GitHub{
			Token: checkEnvVar("GITHUB_TOKEN"),
		},
		Microsoft: Microsoft{
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
			UserId:         checkEnvVar("TWITCH_USER_ID"),
			EventSubUri:    checkEnvVar("TWITCH_EVENTSUB_URI"),
			EventSubSecret: checkEnvVar("TWITCH_EVENTSUB_SECRET"),
			APIBaseUrl:     "",
			OAuth2Config: &oauth2.Config{
				ClientID:     twitchClientId,
				ClientSecret: checkEnvVar("TWITCH_CLIENT_SECRET"),
				RedirectURL:  checkEnvVar("TWITCH_REDIRECT_URI"),
				Endpoint: oauth2.Endpoint{
					AuthURL:   "https://id.twitch.tv/oauth2/authorize",
					TokenURL:  "https://id.twitch.tv/oauth2/token",
					AuthStyle: oauth2.AuthStyleInParams,
				},
			},
		},
	}
}
