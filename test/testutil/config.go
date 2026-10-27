package testutil

import "github.com/NeuralNexusDev/neuralnexus-api/modules/config"

// NewConfig returns a Config with fixed test values, then applies mods in order.
func NewConfig(mods ...func(*config.Config)) *config.Config {
	cfg := &config.Config{
		ApiUrl:  "https://api.neuralnexus.test",
		SiteUrl: "https://neuralnexus.test",
		Auth: config.Auth{
			Account: config.Account{Pepper: []byte("test-pepper")},
			Session: config.Session{
				JwtSecret:      []byte("test-jwt-secret-0123456789abcdef"),
				ValidAudiences: []string{"https://neuralnexus.test", "https://api.neuralnexus.test"},
			},
		},
		Snowflake: config.Snowflake{NodeId: 1, WorkerId: 1},
	}
	for _, mod := range mods {
		mod(cfg)
	}
	return cfg
}
