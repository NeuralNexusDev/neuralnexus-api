package config

import (
	"log"
	"os"
)

var (
	msgJWTSecretUnset  = "JWT_SECRET environment variable must be set"
	msgSiteAPIURLUnset = "NN_SITE_URL and NN_API_URL environment variables must be set"
)

type Config struct {
	ApiUrl  string
	SiteUrl string
	Auth    Auth
}

type Auth struct {
	Session Session
}

type Session struct {
	JwtSecret      []byte
	ValidAudiences []string
}

func FromEnv() *Config {
	jwtSecret := []byte(os.Getenv("JWT_SECRET"))
	if len(jwtSecret) == 0 {
		log.Fatal(msgJWTSecretUnset)
	}

	apiUrl := os.Getenv("NN_API_URL")
	siteUrl := os.Getenv("NN_SITE_URL")
	if siteUrl == "" || apiUrl == "" {
		log.Fatal(msgSiteAPIURLUnset)
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
	}
}
