package linking

import (
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
)

func TestMC01to04MinecraftDataAccessors(t *testing.T) {
	id := uuid.New()
	m := &MinecraftData{
		ID:       id,
		Username: "Steve",
		Skins:    []Skin{{ID: uuid.New(), State: "ACTIVE", URL: "https://example.test/skin.png", Variant: "classic", Alias: "default"}},
		Capes:    []Cape{{}},
	}

	t.Run("MC-01_GetID", func(t *testing.T) {
		if got := m.GetID(); got != id.String() {
			t.Errorf("GetID() = %q, want %q", got, id.String())
		}
	})

	t.Run("MC-02_GetEmail", func(t *testing.T) {
		if got := m.GetEmail(); got != "" {
			t.Errorf("GetEmail() = %q, want empty string", got)
		}
	})

	t.Run("MC-03_GetUsername", func(t *testing.T) {
		if got := m.GetUsername(); got != "Steve" {
			t.Errorf("GetUsername() = %q, want %q", got, "Steve")
		}
	})

	t.Run("MC-04_GetData", func(t *testing.T) {
		data := m.GetData()
		var roundTripped MinecraftData
		if err := json.Unmarshal([]byte(data), &roundTripped); err != nil {
			t.Fatalf("GetData() did not round-trip as JSON: %v", err)
		}
		if roundTripped.ID != m.ID || roundTripped.Username != m.Username || len(roundTripped.Skins) != len(m.Skins) {
			t.Errorf("GetData() round-trip mismatch: got %+v, want equivalent of %+v", roundTripped, *m)
		}
	})
}

func TestMC05CreateLinkedAccount(t *testing.T) {
	t.Run("MC-05_CreateLinkedAccount", func(t *testing.T) {
		id := uuid.New()
		m := &MinecraftData{ID: id, Username: "Steve"}

		la := m.CreateLinkedAccount("user-1")

		if la.UserID != "user-1" {
			t.Errorf("UserID = %q, want %q", la.UserID, "user-1")
		}
		if la.Platform != auth.PlatformMinecraft {
			t.Errorf("Platform = %q, want %q", la.Platform, auth.PlatformMinecraft)
		}
		if la.PlatformUsername != "Steve" {
			t.Errorf("PlatformUsername = %q, want %q", la.PlatformUsername, "Steve")
		}
		if la.PlatformID != id.String() {
			t.Errorf("PlatformID = %q, want %q", la.PlatformID, id.String())
		}
		if !la.Verified || !la.LoginEnabled {
			t.Errorf("expected a freshly created linked account to be Verified and LoginEnabled, got Verified=%v LoginEnabled=%v", la.Verified, la.LoginEnabled)
		}
	})
}
