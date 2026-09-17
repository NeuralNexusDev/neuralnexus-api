package minecraft

import (
	"encoding/base64"
	"testing"

	"github.com/goccy/go-json"
)

func encodedTextures(t *testing.T, value TexturesValue) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("failed to marshal textures: %v", err)
	}
	return base64.StdEncoding.EncodeToString(data)
}

func TestTypes_ParseProperties_ValidTextures(t *testing.T) {
	value := TexturesValue{
		Timestamp:   1234567890,
		ProfileID:   "853c80ef3c3749fdaa49938b674adae6",
		ProfileName: "jeb_",
		Textures: Textures{
			SKIN: &Texture{URL: "http://textures.minecraft.net/texture/abc123"},
		},
	}

	player := &Player{
		Properties: []Property{
			{Name: TEXTURES, Value: encodedTextures(t, value)},
		},
	}

	got := player.ParseProperties()
	if got == nil {
		t.Fatal("expected non-nil TexturesValue")
	}
	if got.ProfileName != "jeb_" {
		t.Errorf("expected jeb_, got %s", got.ProfileName)
	}
	if got.Textures.SKIN == nil {
		t.Fatal("expected non-nil SKIN")
	}
	if got.Textures.SKIN.Hash() != "abc123" {
		t.Errorf("expected abc123, got %s", got.Textures.SKIN.Hash())
	}
}

func TestTypes_ParseProperties_NoProperties(t *testing.T) {
	player := &Player{}
	if got := player.ParseProperties(); got != nil {
		t.Error("expected nil for player with no properties")
	}
}

func TestTypes_ParseProperties_UnknownProperty(t *testing.T) {
	player := &Player{
		Properties: []Property{
			{Name: "unknown", Value: "somevalue"},
		},
	}
	if got := player.ParseProperties(); got != nil {
		t.Error("expected nil for unknown property")
	}
}

func TestTypes_ParseProperties_WithCape(t *testing.T) {
	value := TexturesValue{
		Textures: Textures{
			SKIN: &Texture{URL: "http://textures.minecraft.net/texture/skin123"},
			CAPE: &Texture{URL: "http://textures.minecraft.net/texture/cape456"},
		},
	}

	player := &Player{
		Properties: []Property{
			{Name: TEXTURES, Value: encodedTextures(t, value)},
		},
	}

	got := player.ParseProperties()
	if got == nil {
		t.Fatal("expected non-nil TexturesValue")
	}
	if got.Textures.CAPE == nil {
		t.Fatal("expected non-nil CAPE")
	}
	if got.Textures.CAPE.Hash() != "cape456" {
		t.Errorf("expected cape456, got %s", got.Textures.CAPE.Hash())
	}
}

func TestTypes_ParseProperties_SlimModel(t *testing.T) {
	value := TexturesValue{
		Textures: Textures{
			SKIN: &Texture{
				URL:      "http://textures.minecraft.net/texture/skin123",
				Metadata: &Metadata{Model: SLIM},
			},
		},
	}

	player := &Player{
		Properties: []Property{
			{Name: TEXTURES, Value: encodedTextures(t, value)},
		},
	}

	got := player.ParseProperties()
	if got == nil {
		t.Fatal("expected non-nil TexturesValue")
	}
	if got.Textures.SKIN.Metadata == nil {
		t.Fatal("expected non-nil Metadata")
	}
	if got.Textures.SKIN.Metadata.Model != SLIM {
		t.Errorf("expected slim, got %s", got.Textures.SKIN.Metadata.Model)
	}
}

func TestTypes_TextureHash(t *testing.T) {
	tex := &Texture{URL: "http://textures.minecraft.net/texture/abc123"}
	if tex.Hash() != "abc123" {
		t.Errorf("expected abc123, got %s", tex.Hash())
	}
}

func TestTypes_TextureHash_EmptyURL(t *testing.T) {
	tex := &Texture{URL: ""}
	if tex.Hash() != "" {
		t.Errorf("expected empty hash, got %s", tex.Hash())
	}
}

func TestTypes_TextureHash_TrailingSlash(t *testing.T) {
	tex := &Texture{URL: "http://textures.minecraft.net/texture/"}
	if tex.Hash() != "" {
		t.Errorf("expected empty hash for trailing slash, got %s", tex.Hash())
	}
}

func TestTypes_ParseProperties_MultipleProperties(t *testing.T) {
	value := TexturesValue{
		Textures: Textures{
			SKIN: &Texture{URL: "http://textures.minecraft.net/texture/abc123"},
		},
	}

	player := &Player{
		Properties: []Property{
			{Name: "some_other_prop", Value: "ignored"},
			{Name: TEXTURES, Value: encodedTextures(t, value)},
		},
	}

	got := player.ParseProperties()
	if got == nil {
		t.Fatal("expected non-nil TexturesValue when textures is not the first property")
	}
	if got.Textures.SKIN.Hash() != "abc123" {
		t.Errorf("expected abc123, got %s", got.Textures.SKIN.Hash())
	}
}

func TestTypes_ParseProperties_InvalidBase64(t *testing.T) {
	player := &Player{
		Properties: []Property{
			{Name: TEXTURES, Value: "not-valid-base64!@#$"},
		},
	}

	got := player.ParseProperties()
	if got != nil {
		t.Error("expected nil when property value contains invalid base64")
	}
}

func TestTypes_MarshalJSON_ProfileActionsNil_KeyOmitted(t *testing.T) {
	player := &Player{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}

	data, err := json.Marshal(player)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if _, ok := raw["profileActions"]; ok {
		t.Errorf("expected profileActions key to be omitted, got %s", data)
	}
}

func TestTypes_MarshalJSON_ProfileActionsEmptyNotNil_KeyPresentAsEmptyArray(t *testing.T) {
	player := &Player{
		ID:             "853c80ef3c3749fdaa49938b674adae6",
		Name:           "jeb_",
		ProfileActions: []string{},
	}

	data, err := json.Marshal(player)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	action, ok := raw["profileActions"]
	if !ok {
		t.Fatalf("expected profileActions key to be present, got %s", data)
	}
	if string(action) != "[]" {
		t.Errorf("expected profileActions to serialize as [], got %s", action)
	}
}

func TestTypes_MarshalJSON_ProfileActionsPopulated(t *testing.T) {
	player := &Player{
		ID:             "853c80ef3c3749fdaa49938b674adae6",
		Name:           "jeb_",
		ProfileActions: []string{"FORCED_NAME_CHANGE"},
	}

	data, err := json.Marshal(player)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got Player
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(got.ProfileActions) != 1 || got.ProfileActions[0] != "FORCED_NAME_CHANGE" {
		t.Errorf("expected [FORCED_NAME_CHANGE], got %v", got.ProfileActions)
	}
}

func TestTypes_Profile_MarshalJSON_ProfileActionsOmittedWhenEmpty(t *testing.T) {
	profile := &Profile{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}

	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if _, ok := raw["profileActions"]; ok {
		t.Errorf("expected profileActions to be omitted when empty, got %s", data)
	}
	if _, ok := raw["firstSeen"]; ok {
		t.Errorf("expected firstSeen to never be exposed in JSON, got %s", data)
	}
	if _, ok := raw["lastSeen"]; ok {
		t.Errorf("expected lastSeen to never be exposed in JSON, got %s", data)
	}
}

func TestTypes_Profile_MarshalJSON_ProfileActionsPresentWhenPopulated(t *testing.T) {
	profile := &Profile{
		ID:             "853c80ef3c3749fdaa49938b674adae6",
		Name:           "jeb_",
		ProfileActions: []string{"FORCED_NAME_CHANGE"},
	}

	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got Profile
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(got.ProfileActions) != 1 || got.ProfileActions[0] != "FORCED_NAME_CHANGE" {
		t.Errorf("expected [FORCED_NAME_CHANGE], got %v", got.ProfileActions)
	}
}

func TestTypes_Profile_ToPlayer_EncodesTexturesAsUnsignedProperty(t *testing.T) {
	profile := &Profile{
		ID:   "853c80ef3c3749fdaa49938b674adae6",
		Name: "jeb_",
		Textures: &TexturesValue{
			ProfileID:   "853c80ef3c3749fdaa49938b674adae6",
			ProfileName: "jeb_",
			Textures:    Textures{SKIN: &Texture{URL: "http://textures.minecraft.net/texture/abc123"}},
		},
	}

	player, err := profile.ToPlayer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(player.Properties) != 1 || player.Properties[0].Name != TEXTURES {
		t.Fatalf("expected a single textures property, got %v", player.Properties)
	}
	if player.Properties[0].Signature != "" {
		t.Error("expected no signature on a locally-encoded property")
	}

	decoded := player.ParseProperties()
	if decoded == nil || decoded.Textures.SKIN == nil || decoded.Textures.SKIN.Hash() != "abc123" {
		t.Errorf("expected the encoded property to decode back to the original textures, got %+v", decoded)
	}
}

func TestTypes_Profile_ToPlayer_NoTextures(t *testing.T) {
	profile := &Profile{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}

	player, err := profile.ToPlayer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(player.Properties) != 0 {
		t.Errorf("expected no properties when the profile has no textures, got %v", player.Properties)
	}
}

func TestTypes_Profile_WithTextureURL_RewritesSkinAndCape(t *testing.T) {
	profile := &Profile{
		ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_",
		Textures: &TexturesValue{
			Textures: Textures{
				SKIN: &Texture{URL: "http://textures.minecraft.net/texture/skin123", Metadata: &Metadata{Model: SLIM}},
				CAPE: &Texture{URL: "http://textures.minecraft.net/texture/cape456"},
			},
		},
	}

	got := profile.WithTextureURL("https://cdn.example.com/texture/")

	if got.Textures.Textures.SKIN.URL != "https://cdn.example.com/texture/skin123" {
		t.Errorf("expected rewritten skin URL, got %s", got.Textures.Textures.SKIN.URL)
	}
	if got.Textures.Textures.SKIN.Metadata == nil || got.Textures.Textures.SKIN.Metadata.Model != SLIM {
		t.Error("expected slim model to be preserved")
	}
	if got.Textures.Textures.CAPE.URL != "https://cdn.example.com/texture/cape456" {
		t.Errorf("expected rewritten cape URL, got %s", got.Textures.Textures.CAPE.URL)
	}
	if profile.Textures.Textures.SKIN.URL != "http://textures.minecraft.net/texture/skin123" {
		t.Error("expected the original profile to be left untouched")
	}
}

func TestTypes_Profile_WithTextureURL_NoTextures(t *testing.T) {
	profile := &Profile{ID: "853c80ef3c3749fdaa49938b674adae6", Name: "jeb_"}

	got := profile.WithTextureURL("https://cdn.example.com/texture/")
	if got.Textures != nil {
		t.Errorf("expected no textures, got %+v", got.Textures)
	}
}
