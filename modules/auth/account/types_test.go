package account

import (
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

func TestTY01to02NewAccount(t *testing.T) {
	t.Run("ACT-01_Success", func(t *testing.T) {
		a := NewAccount(testutil.NewConfig().Auth.Account.Pepper, "alice", "a@b.com", "hunter2")
		if a.UserID == "" {
			t.Error("expected a non-empty UserID")
		}
		if a.Username != "alice" {
			t.Errorf("Username = %q, want %q", a.Username, "alice")
		}
		if a.Email == nil || *a.Email != "a@b.com" {
			t.Errorf("Email = %v, want pointer to %q", a.Email, "a@b.com")
		}
		if len(a.HashedSecret) == 0 || len(a.Salt) == 0 {
			t.Errorf("expected HashedSecret/Salt to be populated, got %d/%d bytes", len(a.HashedSecret), len(a.Salt))
		}
	})

	t.Run("ACT-02_EmptyEmailStaysNil", func(t *testing.T) {
		a := NewAccount(testutil.NewConfig().Auth.Account.Pepper, "alice", "", "hunter2")
		if a.Email != nil {
			t.Errorf("Email = %v, want nil for an empty email argument", a.Email)
		}
	})
}

func TestTY04to05NewPasswordLessAccount(t *testing.T) {
	t.Run("ACT-04_Success", func(t *testing.T) {
		a := NewPasswordLessAccount("alice", "a@b.com")
		if a.UserID == "" || a.Username != "alice" {
			t.Errorf("got %+v, want UserID set and Username=alice", a)
		}
		if a.Email == nil || *a.Email != "a@b.com" {
			t.Errorf("Email = %v, want pointer to %q", a.Email, "a@b.com")
		}
		if a.HashedSecret != nil || a.Salt != nil {
			t.Errorf("expected no password material, got HashedSecret=%v Salt=%v", a.HashedSecret, a.Salt)
		}
	})

	t.Run("ACT-05_EmptyEmailStaysNil", func(t *testing.T) {
		a := NewPasswordLessAccount("alice", "")
		if a.Email != nil {
			t.Errorf("Email = %v, want nil", a.Email)
		}
	})
}

func TestTY06NewIDOnlyAccount(t *testing.T) {
	t.Run("ACT-06_OnlyUserIDSet", func(t *testing.T) {
		a := NewIDOnlyAccount()
		if a.UserID == "" {
			t.Error("expected a non-empty UserID")
		}
		if a.Username != "" || a.Email != nil || a.HashedSecret != nil || a.Salt != nil {
			t.Errorf("expected every other field to stay zero-valued, got %+v", a)
		}
	})
}

func TestTY07to08IDKeyWithSecret(t *testing.T) {
	password := []byte("hunter2")
	salt := []byte("0123456789abcdef")

	t.Run("ACT-07_DeterministicForSameInputs", func(t *testing.T) {
		secret := []byte("pepper")
		got1 := IDKeyWithSecret(password, salt, secret, 3, 64*1024, 4, 32)
		got2 := IDKeyWithSecret(password, salt, secret, 3, 64*1024, 4, 32)
		if string(got1) != string(got2) {
			t.Errorf("expected identical output for identical inputs, got %x and %x", got1, got2)
		}
	})

	t.Run("ACT-08_DifferentSecretYieldsDifferentOutput", func(t *testing.T) {
		out1 := IDKeyWithSecret(password, salt, []byte("pepper-a"), 3, 64*1024, 4, 32)
		out2 := IDKeyWithSecret(password, salt, []byte("pepper-b"), 3, 64*1024, 4, 32)
		if string(out1) == string(out2) {
			t.Error("expected different secrets to produce different output, got identical digests")
		}
	})
}

func TestTY09to10HashPassword(t *testing.T) {
	t.Run("ACT-09_Success", func(t *testing.T) {
		u := &Account{}
		u.HashPassword(testutil.NewConfig().Auth.Account.Pepper, "hunter2")
		if len(u.HashedSecret) != 32 {
			t.Errorf("HashedSecret length = %d, want 32", len(u.HashedSecret))
		}
		if len(u.Salt) != 16 {
			t.Errorf("Salt length = %d, want 16", len(u.Salt))
		}
	})

	t.Run("ACT-10_DifferentCallsUseDifferentSalts", func(t *testing.T) {
		u1, u2 := &Account{}, &Account{}
		u1.HashPassword(testutil.NewConfig().Auth.Account.Pepper, "hunter2")
		u2.HashPassword(testutil.NewConfig().Auth.Account.Pepper, "hunter2")
		if string(u1.Salt) == string(u2.Salt) {
			t.Error("expected two calls to produce different random salts")
		}
		if string(u1.HashedSecret) == string(u2.HashedSecret) {
			t.Error("expected two calls (with different salts) to produce different hashed secrets")
		}
	})
}

func TestTY12to14ValidateUser(t *testing.T) {
	t.Run("ACT-12_CorrectPassword", func(t *testing.T) {
		u := passwordAccount("", "", "correct-horse")
		if !u.ValidateUser(testutil.NewConfig().Auth.Account.Pepper, "correct-horse") {
			t.Error("ValidateUser() = false for the correct password, want true")
		}
	})

	t.Run("ACT-13_WrongPassword", func(t *testing.T) {
		u := passwordAccount("", "", "correct-horse")
		if u.ValidateUser(testutil.NewConfig().Auth.Account.Pepper, "wrong-password") {
			t.Error("ValidateUser() = true for an incorrect password, want false")
		}
	})

	t.Run("ACT-14_NeverHashedDoesNotPanic", func(t *testing.T) {
		u := &Account{}
		if u.ValidateUser(testutil.NewConfig().Auth.Account.Pepper, "anything") {
			t.Error("ValidateUser() = true for an account with no password set, want false")
		}
	})
}

func TestTY19to21RemoveRole(t *testing.T) {
	t.Run("ACT-19_RemovesPresentRole", func(t *testing.T) {
		a := &Account{Roles: []string{"a", "b", "c"}}
		a.RemoveRole("b")
		if len(a.Roles) != 2 || a.Roles[0] != "a" || a.Roles[1] != "c" {
			t.Errorf("Roles = %v, want [a c]", a.Roles)
		}
	})

	t.Run("ACT-20_RoleNotPresent", func(t *testing.T) {
		a := &Account{Roles: []string{"a", "b", "c"}}
		a.RemoveRole("z")
		if len(a.Roles) != 3 {
			t.Errorf("Roles = %v, want unchanged [a b c]", a.Roles)
		}
	})

	t.Run("ACT-21_EmptyRoles", func(t *testing.T) {
		a := &Account{}
		a.RemoveRole("z")
		if len(a.Roles) != 0 {
			t.Errorf("Roles = %v, want to remain empty", a.Roles)
		}
	})
}

func passwordAccount(username, email, password string) *Account {
	salt := []byte("0123456789abcdef")
	a := &Account{UserID: "u1", Username: username, Salt: salt, HashedSecret: IDKeyWithSecret([]byte(password), salt, testutil.NewConfig().Auth.Account.Pepper, 3, 64*1024, 4, 32)}
	if email != "" {
		a.Email = &email
	}
	return a
}
