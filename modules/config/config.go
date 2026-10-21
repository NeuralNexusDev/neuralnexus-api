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

type Config struct {
	ApiUrl  string
	SiteUrl string
	Auth    Auth
	Discord Discord
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
	}
}
