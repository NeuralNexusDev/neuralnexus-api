package auth

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
)

type tyFakePlatformData struct {
	id, email, username, data string
}

func (f tyFakePlatformData) GetID() string       { return f.id }
func (f tyFakePlatformData) GetEmail() string    { return f.email }
func (f tyFakePlatformData) GetUsername() string { return f.username }
func (f tyFakePlatformData) GetData() string     { return f.data }
func (f tyFakePlatformData) CreateLinkedAccount(_ string) *LinkedAccount {
	return nil
}

func TestTY01to02NewAccount(t *testing.T) {
	t.Run("TY-01_Success", func(t *testing.T) {
		a, err := NewAccount("alice", "a@b.com", "hunter2")
		if err != nil {
			t.Fatalf("NewAccount() unexpected error: %v", err)
		}
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

	t.Run("TY-02_EmptyEmailStaysNil", func(t *testing.T) {
		a, err := NewAccount("alice", "", "hunter2")
		if err != nil {
			t.Fatalf("NewAccount() unexpected error: %v", err)
		}
		if a.Email != nil {
			t.Errorf("Email = %v, want nil for an empty email argument", a.Email)
		}
	})
}

func TestTY04to05NewPasswordLessAccount(t *testing.T) {
	t.Run("TY-04_Success", func(t *testing.T) {
		a, err := NewPasswordLessAccount("alice", "a@b.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
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

	t.Run("TY-05_EmptyEmailStaysNil", func(t *testing.T) {
		a, err := NewPasswordLessAccount("alice", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.Email != nil {
			t.Errorf("Email = %v, want nil", a.Email)
		}
	})
}

func TestTY06NewIDOnlyAccount(t *testing.T) {
	t.Run("TY-06_OnlyUserIDSet", func(t *testing.T) {
		a, err := NewIDOnlyAccount()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
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

	t.Run("TY-07_DeterministicForSameInputs", func(t *testing.T) {
		secret := []byte("pepper")
		got1 := IDKeyWithSecret(password, salt, secret, 3, 64*1024, 4, 32)
		got2 := IDKeyWithSecret(password, salt, secret, 3, 64*1024, 4, 32)
		if string(got1) != string(got2) {
			t.Errorf("expected identical output for identical inputs, got %x and %x", got1, got2)
		}
	})

	t.Run("TY-08_DifferentSecretYieldsDifferentOutput", func(t *testing.T) {
		out1 := IDKeyWithSecret(password, salt, []byte("pepper-a"), 3, 64*1024, 4, 32)
		out2 := IDKeyWithSecret(password, salt, []byte("pepper-b"), 3, 64*1024, 4, 32)
		if string(out1) == string(out2) {
			t.Error("expected different secrets to produce different output, got identical digests")
		}
	})
}

func TestTY09to10HashPassword(t *testing.T) {
	t.Run("TY-09_Success", func(t *testing.T) {
		u := &Account{}
		if err := u.HashPassword("hunter2"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(u.HashedSecret) != 32 {
			t.Errorf("HashedSecret length = %d, want 32", len(u.HashedSecret))
		}
		if len(u.Salt) != 16 {
			t.Errorf("Salt length = %d, want 16", len(u.Salt))
		}
	})

	t.Run("TY-10_DifferentCallsUseDifferentSalts", func(t *testing.T) {
		u1, u2 := &Account{}, &Account{}
		if err := u1.HashPassword("hunter2"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := u2.HashPassword("hunter2"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(u1.Salt) == string(u2.Salt) {
			t.Error("expected two calls to produce different random salts")
		}
		if string(u1.HashedSecret) == string(u2.HashedSecret) {
			t.Error("expected two calls (with different salts) to produce different hashed secrets")
		}
	})
}

func TestTY12to14ValidateUser(t *testing.T) {
	t.Run("TY-12_CorrectPassword", func(t *testing.T) {
		u := &Account{}
		if err := u.HashPassword("correct-horse"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u.ValidateUser("correct-horse") {
			t.Error("ValidateUser() = false for the correct password, want true")
		}
	})

	t.Run("TY-13_WrongPassword", func(t *testing.T) {
		u := &Account{}
		if err := u.HashPassword("correct-horse"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u.ValidateUser("wrong-password") {
			t.Error("ValidateUser() = true for an incorrect password, want false")
		}
	})

	t.Run("TY-14_NeverHashedDoesNotPanic", func(t *testing.T) {
		u := &Account{}
		if u.ValidateUser("anything") {
			t.Error("ValidateUser() = true for an account with no password set, want false")
		}
	})
}

func TestTY15DummyValidateUser(t *testing.T) {
	t.Run("TY-15_DoesNotPanic", func(t *testing.T) {
		DummyValidateUser("whatever")
	})
}

func TestTY17to18AddRole(t *testing.T) {
	t.Run("TY-17_AddsToEmptyRoles", func(t *testing.T) {
		a := &Account{}
		a.AddRole("admin")
		if len(a.Roles) != 1 || a.Roles[0] != "admin" {
			t.Errorf("Roles = %v, want [admin]", a.Roles)
		}
	})

	t.Run("TY-18_DuplicateAllowed", func(t *testing.T) {
		a := &Account{Roles: []string{"admin"}}
		a.AddRole("admin")
		if len(a.Roles) != 2 || a.Roles[0] != "admin" || a.Roles[1] != "admin" {
			t.Errorf("Roles = %v, want [admin admin] (no de-duplication)", a.Roles)
		}
	})
}

func TestTY19to21RemoveRole(t *testing.T) {
	t.Run("TY-19_RemovesPresentRole", func(t *testing.T) {
		a := &Account{Roles: []string{"a", "b", "c"}}
		a.RemoveRole("b")
		if len(a.Roles) != 2 || a.Roles[0] != "a" || a.Roles[1] != "c" {
			t.Errorf("Roles = %v, want [a c]", a.Roles)
		}
	})

	t.Run("TY-20_RoleNotPresent", func(t *testing.T) {
		a := &Account{Roles: []string{"a", "b", "c"}}
		a.RemoveRole("z")
		if len(a.Roles) != 3 {
			t.Errorf("Roles = %v, want unchanged [a b c]", a.Roles)
		}
	})

	t.Run("TY-21_EmptyRoles", func(t *testing.T) {
		a := &Account{}
		a.RemoveRole("z")
		if len(a.Roles) != 0 {
			t.Errorf("Roles = %v, want to remain empty", a.Roles)
		}
	})
}

func TestTY22DefaultAccountSettings(t *testing.T) {
	t.Run("TY-22_DefaultsToPasswordAuthEnabled", func(t *testing.T) {
		got := DefaultAccountSettings("u1")
		want := &AccountSettings{UserID: "u1", PasswordAuthEnabled: true}
		if got.UserID != want.UserID || got.PasswordAuthEnabled != want.PasswordAuthEnabled {
			t.Errorf("DefaultAccountSettings() = %+v, want %+v", got, want)
		}
	})
}

func TestTY23to25AccountNewSession(t *testing.T) {
	t.Run("TY-23_ExpandsRolePermissions", func(t *testing.T) {
		a := &Account{UserID: "u1", Roles: []string{perms.RoleSystem.Name}}
		s, err := a.NewSession(12345)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.UserID != "u1" {
			t.Errorf("UserID = %q, want %q", s.UserID, "u1")
		}
		if s.ExpiresAt != 12345 {
			t.Errorf("ExpiresAt = %d, want 12345", s.ExpiresAt)
		}
		want := make([]string, 0, len(perms.RoleSystem.Permissions))
		for _, p := range perms.RoleSystem.Permissions {
			want = append(want, p.Name+"|"+p.Value)
		}
		if len(s.Permissions) != len(want) {
			t.Fatalf("Permissions = %v, want %v", s.Permissions, want)
		}
		for i := range want {
			if s.Permissions[i] != want[i] {
				t.Errorf("Permissions[%d] = %q, want %q", i, s.Permissions[i], want[i])
			}
		}
	})

	t.Run("TY-24_SkipsUnknownRole", func(t *testing.T) {
		a := &Account{UserID: "u1", Roles: []string{"not-a-real-role"}}
		s, err := a.NewSession(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(s.Permissions) != 0 {
			t.Errorf("Permissions = %v, want empty (unknown role silently skipped)", s.Permissions)
		}
	})

	t.Run("TY-25_NoRoles", func(t *testing.T) {
		a := &Account{UserID: "u1"}
		s, err := a.NewSession(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(s.Permissions) != 0 {
			t.Errorf("Permissions = %v, want empty", s.Permissions)
		}
	})
}

func TestTY26NewLinkedAccount(t *testing.T) {
	t.Run("TY-26_BuildsVerifiedLoginEnabledLink", func(t *testing.T) {
		data := tyFakePlatformData{id: "p1", username: "alice"}
		la := NewLinkedAccount("u1", PlatformDiscord, "alice", "p1", data)

		if la.UserID != "u1" || la.Platform != PlatformDiscord || la.PlatformUsername != "alice" || la.PlatformID != "p1" {
			t.Errorf("got %+v, want UserID=u1 Platform=discord PlatformUsername=alice PlatformID=p1", la)
		}
		if la.Data != PlatformData(data) {
			t.Errorf("Data = %v, want %v", la.Data, data)
		}
		if !la.Verified || !la.LoginEnabled {
			t.Errorf("expected a freshly created link to be Verified and LoginEnabled, got Verified=%v LoginEnabled=%v", la.Verified, la.LoginEnabled)
		}
	})
}

// TestTY27to28InitTypesGo covers types.go's init(). See
// TestSE32to34InitSessionGo in session_test.go for why SE-33/TY-28 re-exec
// the test binary rather than recovering from the log.Fatal in-process.
func TestTY27to28InitTypesGo(t *testing.T) {
	t.Run("TY-27_PackageLoadedUnderRequiredEnv", func(t *testing.T) {
		if len(pepper) == 0 {
			t.Fatal("pepper is empty; init() should already have log.Fatal'd if so, so this file's other tests couldn't be running")
		}
	})

	t.Run("TY-28_MissingPepperFatals", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "-test.run=^$")
		cmd.Env = append(os.Environ(), "PEPPER=")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.Success() {
			t.Fatalf("re-exec with PEPPER unset: got err=%v, want a non-zero exit from init()'s log.Fatal; stderr:\n%s", err, stderr.String())
		}
		if !strings.Contains(stderr.String(), "PEPPER environment variable must be set") {
			t.Errorf("subprocess stderr = %q, want it to contain init()'s PEPPER message", stderr.String())
		}
	})
}
