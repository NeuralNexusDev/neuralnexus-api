package auth

import (
	"bytes"
	"testing"

	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
)

func TestHashPasswordAndValidateUser(t *testing.T) {
	user := &Account{}
	if err := user.HashPassword("correct horse battery staple"); err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	if !user.ValidateUser("correct horse battery staple") {
		t.Error("ValidateUser rejected the correct password")
	}
	if user.ValidateUser("wrong password") {
		t.Error("ValidateUser accepted an incorrect password")
	}
	if user.ValidateUser("Correct Horse Battery Staple") {
		t.Error("ValidateUser should be case-sensitive")
	}
}

func TestHashPasswordAndValidateUserEmptyPassword(t *testing.T) {
	user := &Account{}
	if err := user.HashPassword(""); err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	if !user.ValidateUser("") {
		t.Error("ValidateUser rejected the correct (empty) password")
	}
	if user.ValidateUser("not empty") {
		t.Error("ValidateUser accepted an incorrect password against an empty-password hash")
	}
}

func TestValidateUserWithoutHash(t *testing.T) {
	user := &Account{}
	if user.ValidateUser("anything") {
		t.Error("ValidateUser should reject when HashedSecret/Salt are nil")
	}

	user2 := &Account{Salt: []byte("somesalt")}
	if user2.ValidateUser("anything") {
		t.Error("ValidateUser should reject when HashedSecret is nil")
	}

	user3 := &Account{HashedSecret: []byte("somehash")}
	if user3.ValidateUser("anything") {
		t.Error("ValidateUser should reject when Salt is nil")
	}
}

func TestHashPasswordUsesUniqueSalts(t *testing.T) {
	user1 := &Account{}
	user2 := &Account{}
	if err := user1.HashPassword("same password"); err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if err := user2.HashPassword("same password"); err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	if bytes.Equal(user1.Salt, user2.Salt) {
		t.Error("two calls to HashPassword produced the same salt")
	}
	if bytes.Equal(user1.HashedSecret, user2.HashedSecret) {
		t.Error("two calls to HashPassword with the same password produced the same hash (salt not being applied)")
	}
}

func TestIDKeyWithSecretPepperAffectsOutput(t *testing.T) {
	password := []byte("password")
	salt := []byte("0123456789abcdef")

	withPepperA := IDKeyWithSecret(password, salt, []byte("pepper-a"), 3, 64*1024, 4, 32)
	withPepperARepeat := IDKeyWithSecret(password, salt, []byte("pepper-a"), 3, 64*1024, 4, 32)
	withPepperB := IDKeyWithSecret(password, salt, []byte("pepper-b"), 3, 64*1024, 4, 32)
	withoutPepper := IDKeyWithSecret(password, salt, nil, 3, 64*1024, 4, 32)

	if !bytes.Equal(withPepperA, withPepperARepeat) {
		t.Error("identical inputs produced different derived keys")
	}
	if bytes.Equal(withPepperA, withPepperB) {
		t.Error("different peppers produced the same derived key")
	}
	if bytes.Equal(withPepperA, withoutPepper) {
		t.Error("a pepper should change the derived key relative to no pepper")
	}
}

func TestAccountAddRole(t *testing.T) {
	user := &Account{}
	user.AddRole("owner")
	user.AddRole("system")

	if len(user.Roles) != 2 || user.Roles[0] != "owner" || user.Roles[1] != "system" {
		t.Errorf("unexpected roles after AddRole: %v", user.Roles)
	}
}

func TestAccountRemoveRole(t *testing.T) {
	user := &Account{Roles: []string{"owner", "system", "guest"}}
	user.RemoveRole("system")

	if len(user.Roles) != 2 || user.Roles[0] != "owner" || user.Roles[1] != "guest" {
		t.Errorf("unexpected roles after RemoveRole: %v", user.Roles)
	}
}

func TestAccountRemoveRoleNotPresent(t *testing.T) {
	user := &Account{Roles: []string{"owner"}}
	user.RemoveRole("nonexistent")

	if len(user.Roles) != 1 || user.Roles[0] != "owner" {
		t.Errorf("RemoveRole with a missing role should be a no-op, got: %v", user.Roles)
	}
}

func TestAccountNewSessionExpandsRolePermissions(t *testing.T) {
	user := &Account{UserID: "123", Roles: []string{perms.RoleOwner.Name}}

	session, err := user.NewSession(1234567890)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}

	if session.UserID != user.UserID {
		t.Errorf("expected UserID %q, got %q", user.UserID, session.UserID)
	}
	if len(session.Permissions) != len(perms.RoleOwner.Permissions) {
		t.Fatalf("expected %d permissions, got %d: %v", len(perms.RoleOwner.Permissions), len(session.Permissions), session.Permissions)
	}
	for i, p := range perms.RoleOwner.Permissions {
		want := p.Name + "|" + p.Value
		if session.Permissions[i] != want {
			t.Errorf("permission %d: expected %q, got %q", i, want, session.Permissions[i])
		}
	}
}

func TestAccountNewSessionSkipsUnknownRole(t *testing.T) {
	user := &Account{UserID: "123", Roles: []string{"not-a-real-role"}}

	session, err := user.NewSession(1234567890)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if len(session.Permissions) != 0 {
		t.Errorf("expected no permissions for an unknown role, got: %v", session.Permissions)
	}
}
