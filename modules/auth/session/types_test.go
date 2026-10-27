package session

import (
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
)

func TestSE02to03HasPermission(t *testing.T) {
	scope := perms.Scope{Node: "perm.node"}

	t.Run("SE-02_Match", func(t *testing.T) {
		for _, held := range []string{"perm.node", "perm.node:value", "perm.node:a:b"} {
			s := &Session{Permissions: []string{"other", held}}
			if !s.HasPermission(scope) {
				t.Errorf("HasPermission() = false for %q, want true", held)
			}
		}
	})

	t.Run("SE-03_NoMatch", func(t *testing.T) {
		if (&Session{Permissions: []string{}}).HasPermission(scope) {
			t.Error("HasPermission() = true with no permissions, want false")
		}
		for _, held := range []string{"other", "perm.nodes", "perm.node.sub", "perm.nod", "perm.node2:value", "x:perm.node"} {
			if (&Session{Permissions: []string{held}}).HasPermission(scope) {
				t.Errorf("HasPermission() = true for %q, want false", held)
			}
		}
	})
}

func TestSE35to36HasPermissionValue(t *testing.T) {
	scope := perms.Scope{Node: "perm.list"}

	t.Run("SE-35_ValueMatch", func(t *testing.T) {
		s := &Session{Permissions: []string{"perm.list:a", "perm.list:b:c"}}
		for _, value := range []string{"a", "b:c"} {
			if !s.HasPermissionValue(scope, value) {
				t.Errorf("HasPermissionValue(%q) = false, want true", value)
			}
		}
	})

	t.Run("SE-36_ValueNoMatch", func(t *testing.T) {
		s := &Session{Permissions: []string{"perm.list:a", "perm.list", "other:b"}}
		for _, value := range []string{"b", "", "ab", "perm.list"} {
			if s.HasPermissionValue(scope, value) {
				t.Errorf("HasPermissionValue(%q) = true, want false", value)
			}
		}
	})
}

func TestSE04to06IsValid(t *testing.T) {
	t.Run("SE-04_FutureExpiry", func(t *testing.T) {
		s := &Session{ExpiresAt: time.Now().Add(time.Hour).Unix()}
		if !s.IsValid() {
			t.Error("IsValid() = false, want true")
		}
	})

	t.Run("SE-05_NeverExpires", func(t *testing.T) {
		s := &Session{ExpiresAt: 0}
		if !s.IsValid() {
			t.Error("IsValid() = false, want true")
		}
	})

	t.Run("SE-06_PastExpiry", func(t *testing.T) {
		s := &Session{ExpiresAt: time.Now().Add(-time.Hour).Unix()}
		if s.IsValid() {
			t.Error("IsValid() = true, want false")
		}
	})
}
