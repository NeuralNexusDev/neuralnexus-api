package auth

import (
	"testing"
	"time"

	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
)

func TestSessionIsValid(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt int64
		want      bool
	}{
		{"never expires when zero", 0, true},
		{"valid when in the future", time.Now().Add(time.Hour).Unix(), true},
		{"invalid when in the past", time.Now().Add(-time.Hour).Unix(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{ExpiresAt: tt.expiresAt}
			if got := s.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSessionHasPermission(t *testing.T) {
	s := &Session{Permissions: []string{"users|*", "ratelimit|1000"}}

	if !s.HasPermission(perms.Scope{Name: "users", Value: "*"}) {
		t.Error("expected HasPermission to find a matching permission")
	}
	if s.HasPermission(perms.Scope{Name: "users", Value: "other"}) {
		t.Error("HasPermission matched on name alone, ignoring value")
	}
	if s.HasPermission(perms.Scope{Name: "missing", Value: "*"}) {
		t.Error("HasPermission matched a permission the session doesn't have")
	}
}
