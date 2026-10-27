package testutil_test

import (
	"reflect"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

func TestNewConfig_Defaults(t *testing.T) {
	want := &config.Config{
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

	got := testutil.NewConfig()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NewConfig() = %+v, want %+v", got, want)
	}
}

func TestNewConfig_OneModifier(t *testing.T) {
	cfg := testutil.NewConfig(func(c *config.Config) {
		c.Auth.Session.JwtSecret = []byte("other")
	})

	if string(cfg.Auth.Session.JwtSecret) != "other" {
		t.Errorf("JwtSecret = %q, want %q", cfg.Auth.Session.JwtSecret, "other")
	}
	if string(cfg.Auth.Account.Pepper) != "test-pepper" {
		t.Errorf("Pepper = %q, want %q", cfg.Auth.Account.Pepper, "test-pepper")
	}
	if cfg.ApiUrl != "https://api.neuralnexus.test" {
		t.Errorf("ApiUrl = %q, want the default", cfg.ApiUrl)
	}
	if cfg.Snowflake.NodeId != 1 || cfg.Snowflake.WorkerId != 1 {
		t.Errorf("Snowflake = %+v, want node 1 worker 1", cfg.Snowflake)
	}
}

func TestNewConfig_ModifiersApplyInOrder(t *testing.T) {
	cfg := testutil.NewConfig(
		func(c *config.Config) { c.SiteUrl = "https://first.test" },
		func(c *config.Config) { c.SiteUrl = c.SiteUrl + "/second" },
	)

	if cfg.SiteUrl != "https://first.test/second" {
		t.Errorf("SiteUrl = %q, want %q", cfg.SiteUrl, "https://first.test/second")
	}
}

func TestNewConfig_ReturnsFreshConfig(t *testing.T) {
	first := testutil.NewConfig(func(c *config.Config) { c.Auth.Session.ValidAudiences[0] = "changed" })
	second := testutil.NewConfig()

	if second.Auth.Session.ValidAudiences[0] != "https://neuralnexus.test" {
		t.Errorf("second config saw a change made to the first: %v", first.Auth.Session.ValidAudiences)
	}
}
