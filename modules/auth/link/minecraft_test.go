package link

import (
	"testing"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
)

func TestMC04MinecraftDataGetData(t *testing.T) {
	id := uuid.New()
	m := &MinecraftData{
		ID:       id,
		Username: "Steve",
		Skins:    []Skin{{ID: uuid.New(), State: "ACTIVE", URL: "https://example.test/skin.png", Variant: "classic", Alias: "default"}},
		Capes:    []Cape{{}},
	}

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
