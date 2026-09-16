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
