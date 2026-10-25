package config

import (
	"encoding/hex"
	"log"
	"os"

	"golang.org/x/crypto/ed25519"
)

var (
	msgJWTSecretUnset  = "JWT_SECRET environment variable must be set"
	msgSiteAPIURLUnset = "NN_SITE_URL and NN_API_URL environment variables must be set"
)

func checkEnvVar(varName string) string {
	envVar := os.Getenv(varName)
	if envVar == "" {
		log.Fatal(varName + " is not set")
	}
	return envVar
}

type Config struct {
	ApiUrl  string
	SiteUrl string
	Auth    Auth
	Discord Discord
	Twitch  Twitch
}

type Auth struct {
	Session Session
}

type Session struct {
	JwtSecret      []byte
	ValidAudiences []string
}

type Discord struct {
	PublicKey ed25519.PublicKey
}

type Twitch struct {
	UserId         string
	ClientId       string
	ClientSecret   string
	RedirectUri    string
	EventSubUri    string
	EventSubSecret string
	APIBaseUrl     string
}

func FromEnv() *Config {
	// ---------- Session ---------
	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		log.Fatal(msgJWTSecretUnset)
	}

	apiUrl := os.Getenv("NN_API_URL")
	siteUrl := os.Getenv("NN_SITE_URL")
	if siteUrl == "" || apiUrl == "" {
		log.Fatal(msgSiteAPIURLUnset)
	}

	// ---------- Discord ---------
	publicKeyStr := os.Getenv("DISCORD_PUBLIC_KEY")
	publicKey, err := hex.DecodeString(publicKeyStr)
	if publicKeyStr == "" || err != nil || len(publicKey) != ed25519.PublicKeySize {
		log.Fatal("DISCORD_PUBLIC_KEY is not set")
		return nil
	}

	// ---------- Twitch ----------
	userId := checkEnvVar("TWITCH_USER_ID")
	clientId := checkEnvVar("TWITCH_CLIENT_ID")
	clientSecret := checkEnvVar("TWITCH_CLIENT_SECRET")
	redirectUri := checkEnvVar("TWITCH_REDIRECT_URI")
	eventSubUri := checkEnvVar("TWITCH_EVENTSUB_URI")
	eventSubSecret := checkEnvVar("TWITCH_EVENTSUB_SECRET")

	return &Config{
		ApiUrl:  apiUrl,
		SiteUrl: siteUrl,
		Auth: Auth{
			Session: Session{
				JwtSecret:      jwtSecret,
				ValidAudiences: []string{siteUrl, apiUrl},
			},
		},
		Discord: Discord{
			PublicKey: publicKey,
		},
		Twitch: Twitch{
			UserId:         userId,
			ClientId:       clientId,
			ClientSecret:   clientSecret,
			RedirectUri:    redirectUri,
			EventSubUri:    eventSubUri,
			EventSubSecret: eventSubSecret,
		},
	}
}
