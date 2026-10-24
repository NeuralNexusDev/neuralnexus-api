package testutil

import "testing"

func TestPostgresURL(t *testing.T) {
	for _, c := range []struct{ name, env, want string }{
		{"server url", "postgres://u:p@localhost:55432", "postgres://u:p@localhost:55432/archive"},
		{"trailing slash", "postgres://u:p@localhost:55432/", "postgres://u:p@localhost:55432/archive"},
		{"existing database", "postgres://u:p@localhost:55432/other", "postgres://u:p@localhost:55432/archive"},
		{"query kept", "postgres://u:p@localhost:55432?sslmode=disable", "postgres://u:p@localhost:55432/archive?sslmode=disable"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("TEST_POSTGRES_URL", c.env)
			if got := PostgresURL(t, "archive"); got != c.want {
				t.Errorf("PostgresURL() = %q, want %q", got, c.want)
			}
		})
	}
}
