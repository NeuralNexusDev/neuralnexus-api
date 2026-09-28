package minecraft

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
)

// mcEncodeTextures base64+JSON-encodes a TexturesValue the way a Mojang
// TEXTURES property value is expected to look.
func mcEncodeTextures(t *testing.T, value TexturesValue) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("failed to marshal TexturesValue fixture: %v", err)
	}
	return base64.StdEncoding.EncodeToString(data)
}

func TestTY01to03_Player_MarshalJSON(t *testing.T) {
	t.Run("TY-01_NilProfileActionsOmitted", func(t *testing.T) {
		p := &Player{ID: "id", Name: "name", ProfileActions: nil}
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if strings.Contains(string(data), "profileActions") {
			t.Errorf("expected no profileActions key, got %s", data)
		}
	})

	t.Run("TY-02_EmptyNonNilProfileActionsPresent", func(t *testing.T) {
		p := &Player{ID: "id", Name: "name", ProfileActions: []string{}}
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if !strings.Contains(string(data), `"profileActions":[]`) {
			t.Errorf(`expected "profileActions":[], got %s`, data)
		}
	})

	t.Run("TY-03_PopulatedProfileActions", func(t *testing.T) {
		p := &Player{ID: "id", Name: "name", ProfileActions: []string{"a"}}
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if !strings.Contains(string(data), `"profileActions":["a"]`) {
			t.Errorf(`expected "profileActions":["a"], got %s`, data)
		}
	})
}

func TestTY04to09_Player_ParseProperties(t *testing.T) {
	t.Run("TY-04_ValidTexturesProperty", func(t *testing.T) {
		want := TexturesValue{ProfileID: "abc", ProfileName: "Steve", Textures: Textures{SKIN: &Texture{URL: "http://x/y"}}}
		p := &Player{Properties: []Property{{Name: TEXTURES, Value: mcEncodeTextures(t, want)}}}

		got := p.ParseProperties()
		if got == nil {
			t.Fatal("ParseProperties() = nil, want decoded value")
		}
		if got.ProfileID != want.ProfileID || got.ProfileName != want.ProfileName {
			t.Errorf("ParseProperties() = %+v, want %+v", got, want)
		}
	})

	t.Run("TY-05_NoProperties", func(t *testing.T) {
		p := &Player{Properties: nil}
		if got := p.ParseProperties(); got != nil {
			t.Errorf("ParseProperties() = %+v, want nil", got)
		}
	})

	t.Run("TY-06_UnknownPropertyName", func(t *testing.T) {
		p := &Player{Properties: []Property{{Name: "other", Value: "irrelevant"}}}
		if got := p.ParseProperties(); got != nil {
			t.Errorf("ParseProperties() = %+v, want nil", got)
		}
	})

	t.Run("TY-07_InvalidBase64", func(t *testing.T) {
		p := &Player{Properties: []Property{{Name: TEXTURES, Value: "not-base64!!"}}}
		if got := p.ParseProperties(); got != nil {
			t.Errorf("ParseProperties() = %+v, want nil", got)
		}
	})

	t.Run("TY-08_ValidBase64InvalidJSON", func(t *testing.T) {
		bad := base64.StdEncoding.EncodeToString([]byte("not json"))
		p := &Player{Properties: []Property{{Name: TEXTURES, Value: bad}}}
		if got := p.ParseProperties(); got != nil {
			t.Errorf("ParseProperties() = %+v, want nil", got)
		}
	})

	t.Run("TY-09_FirstInvalidSecondValid", func(t *testing.T) {
		want := TexturesValue{ProfileID: "second"}
		p := &Player{Properties: []Property{
			{Name: TEXTURES, Value: "not-base64!!"},
			{Name: TEXTURES, Value: mcEncodeTextures(t, want)},
		}}
		got := p.ParseProperties()
		if got == nil || got.ProfileID != "second" {
			t.Errorf("ParseProperties() = %+v, want ProfileID=second", got)
		}
	})
}

func TestTY10to11_Player_IsStale(t *testing.T) {
	t.Run("TY-10_Fresh", func(t *testing.T) {
		p := &Player{LastSeen: time.Now().UnixMilli()}
		if p.IsStale() {
			t.Error("IsStale() = true, want false for a fresh entry")
		}
	})
	t.Run("TY-11_Stale", func(t *testing.T) {
		p := &Player{LastSeen: time.Now().Add(-25 * time.Hour).UnixMilli()}
		if !p.IsStale() {
			t.Error("IsStale() = false, want true for an entry older than the staleness threshold")
		}
	})
}

func TestTY22to23_Profile_IsStale(t *testing.T) {
	t.Run("TY-22_Fresh", func(t *testing.T) {
		p := &Profile{LastSeen: time.Now().UnixMilli()}
		if p.IsStale() {
			t.Error("IsStale() = true, want false for a fresh entry")
		}
	})
	t.Run("TY-23_Stale", func(t *testing.T) {
		p := &Profile{LastSeen: time.Now().Add(-25 * time.Hour).UnixMilli()}
		if !p.IsStale() {
			t.Error("IsStale() = false, want true for an entry older than the staleness threshold")
		}
	})
}

func TestTY26to27_GeyserPlayer_IsStale(t *testing.T) {
	t.Run("TY-26_Fresh", func(t *testing.T) {
		p := &GeyserPlayer{LastSeen: time.Now().UnixMilli()}
		if p.IsStale() {
			t.Error("IsStale() = true, want false for a fresh entry")
		}
	})
	t.Run("TY-27_Stale", func(t *testing.T) {
		p := &GeyserPlayer{LastSeen: time.Now().Add(-25 * time.Hour).UnixMilli()}
		if !p.IsStale() {
			t.Error("IsStale() = false, want true for an entry older than the staleness threshold")
		}
	})
}

func TestTY28to29_GeyserSkin_IsStale(t *testing.T) {
	t.Run("TY-28_Fresh", func(t *testing.T) {
		s := &GeyserSkin{LastSeen: time.Now().UnixMilli()}
		if s.IsStale() {
			t.Error("IsStale() = true, want false for a fresh entry")
		}
	})
	t.Run("TY-29_Stale", func(t *testing.T) {
		s := &GeyserSkin{LastSeen: time.Now().Add(-25 * time.Hour).UnixMilli()}
		if !s.IsStale() {
			t.Error("IsStale() = false, want true for an entry older than the staleness threshold")
		}
	})
}

func TestTY12to13_Player_ToProfile(t *testing.T) {
	t.Run("TY-12_WithTextures", func(t *testing.T) {
		tex := TexturesValue{ProfileID: "abc"}
		p := &Player{
			ID: "id", Name: "name", Legacy: true, Demo: true,
			ProfileActions: []string{"a"},
			Properties:     []Property{{Name: TEXTURES, Value: mcEncodeTextures(t, tex)}},
		}
		got := p.ToProfile()
		if got.ID != p.ID || got.Name != p.Name || got.Legacy != p.Legacy || got.Demo != p.Demo {
			t.Errorf("ToProfile() = %+v, fields do not mirror Player", got)
		}
		if len(got.ProfileActions) != 1 || got.ProfileActions[0] != "a" {
			t.Errorf("ToProfile().ProfileActions = %+v, want [a]", got.ProfileActions)
		}
		if got.Textures == nil || got.Textures.ProfileID != "abc" {
			t.Errorf("ToProfile().Textures = %+v, want decoded from Properties", got.Textures)
		}
	})

	t.Run("TY-13_NoProperties", func(t *testing.T) {
		p := &Player{ID: "id", Name: "name"}
		got := p.ToProfile()
		if got.Textures != nil {
			t.Errorf("ToProfile().Textures = %+v, want nil", got.Textures)
		}
	})
}

func TestTY14to15_Property_String(t *testing.T) {
	t.Run("TY-14_WithSignature", func(t *testing.T) {
		p := &Property{Name: TEXTURES, Value: "val", Signature: "sig"}
		got := p.String()
		if !strings.Contains(got, "sig") {
			t.Errorf("String() = %q, want it to include the signature", got)
		}
	})

	t.Run("TY-15_WithoutSignature", func(t *testing.T) {
		p := &Property{Name: TEXTURES, Value: "val", Signature: ""}
		got := p.String()
		if strings.Contains(got, ", }") || strings.Contains(got, "sig") {
			t.Errorf("String() = %q, want no signature segment", got)
		}
	})
}

func TestTY16to17_TexturesValue_ToProperty(t *testing.T) {
	t.Run("TY-16_Populated", func(t *testing.T) {
		tv := &TexturesValue{ProfileID: "abc", ProfileName: "Steve"}
		prop, err := tv.ToProperty()
		if err != nil {
			t.Fatalf("ToProperty() error = %v", err)
		}
		if prop == nil || prop.Name != TEXTURES {
			t.Fatalf("ToProperty() = %+v, want a TEXTURES property", prop)
		}
		decoded, err := base64.StdEncoding.DecodeString(prop.Value)
		if err != nil {
			t.Fatalf("property value is not valid base64: %v", err)
		}
		var got TexturesValue
		if err := json.Unmarshal(decoded, &got); err != nil {
			t.Fatalf("property value did not decode to JSON: %v", err)
		}
		if got.ProfileID != tv.ProfileID {
			t.Errorf("decoded ProfileID = %q, want %q", got.ProfileID, tv.ProfileID)
		}
	})

	t.Run("TY-17_NilReceiver", func(t *testing.T) {
		var tv *TexturesValue
		prop, err := tv.ToProperty()
		if prop != nil || err != nil {
			t.Errorf("ToProperty() = (%+v, %v), want (nil, nil)", prop, err)
		}
	})
}

func TestTY18to21_Texture_Hash(t *testing.T) {
	cases := []struct {
		id   string
		tex  *Texture
		want string
	}{
		{"TY-18_ValidURL", &Texture{URL: "http://textures.minecraft.net/texture/abc123"}, "abc123"},
		{"TY-19_NilReceiver", nil, ""},
		{"TY-20_NoSlash", &Texture{URL: "abc123"}, ""},
		{"TY-21_TrailingSlash", &Texture{URL: "http://x/"}, ""},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			if got := c.tex.Hash(); got != c.want {
				t.Errorf("Hash() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestTY24to25_Profile_ToPlayer(t *testing.T) {
	t.Run("TY-24_WithTextures", func(t *testing.T) {
		p := &Profile{
			ID: "id", Name: "name",
			Textures: &TexturesValue{Textures: Textures{SKIN: &Texture{URL: "http://x/hash"}}},
		}
		player, err := p.ToPlayer()
		if err != nil {
			t.Fatalf("ToPlayer() error = %v", err)
		}
		if len(player.Properties) != 1 || player.Properties[0].Name != TEXTURES {
			t.Errorf("ToPlayer().Properties = %+v, want exactly one TEXTURES entry", player.Properties)
		}
	})

	t.Run("TY-25_NoTextures", func(t *testing.T) {
		p := &Profile{ID: "id", Name: "name", Textures: nil}
		player, err := p.ToPlayer()
		if err != nil {
			t.Fatalf("ToPlayer() error = %v", err)
		}
		if len(player.Properties) != 0 {
			t.Errorf("ToPlayer().Properties = %+v, want empty", player.Properties)
		}
	})
}

func TestTY30to33_GeyserSkin_SkinURL(t *testing.T) {
	t.Run("TY-30_Valid", func(t *testing.T) {
		tv := TexturesValue{Textures: Textures{SKIN: &Texture{URL: "http://x/hash"}}}
		s := &GeyserSkin{Value: mcEncodeTextures(t, tv)}
		if got := s.SkinURL(); got != "http://x/hash" {
			t.Errorf("SkinURL() = %q, want %q", got, "http://x/hash")
		}
	})

	t.Run("TY-31_NotBase64", func(t *testing.T) {
		s := &GeyserSkin{Value: "not-base64!!"}
		if got := s.SkinURL(); got != "" {
			t.Errorf("SkinURL() = %q, want empty", got)
		}
	})

	t.Run("TY-32_ValidBase64InvalidJSON", func(t *testing.T) {
		s := &GeyserSkin{Value: base64.StdEncoding.EncodeToString([]byte("not json"))}
		if got := s.SkinURL(); got != "" {
			t.Errorf("SkinURL() = %q, want empty", got)
		}
	})

	t.Run("TY-33_NoSkinTexture", func(t *testing.T) {
		tv := TexturesValue{Textures: Textures{}}
		s := &GeyserSkin{Value: mcEncodeTextures(t, tv)}
		if got := s.SkinURL(); got != "" {
			t.Errorf("SkinURL() = %q, want empty", got)
		}
	})
}

func TestTY34to35_XuidToUUID(t *testing.T) {
	t.Run("TY-34_Nonzero", func(t *testing.T) {
		got := xuidToUUID(123456789)
		if _, err := uuid.Parse(got); err != nil {
			t.Fatalf("xuidToUUID() = %q is not a valid UUID: %v", got, err)
		}
		back, err := uuidToXUID(got)
		if err != nil || back != 123456789 {
			t.Errorf("round trip failed: uuidToXUID(%q) = (%d, %v), want (123456789, nil)", got, back, err)
		}
	})

	t.Run("TY-35_Zero", func(t *testing.T) {
		want := "00000000-0000-0000-0000-000000000000"
		if got := xuidToUUID(0); got != want {
			t.Errorf("xuidToUUID(0) = %q, want %q", got, want)
		}
	})
}

func TestTY36to38_UuidToXUID(t *testing.T) {
	t.Run("TY-36_RoundTrip", func(t *testing.T) {
		id := xuidToUUID(42)
		got, err := uuidToXUID(id)
		if err != nil || got != 42 {
			t.Errorf("uuidToXUID(%q) = (%d, %v), want (42, nil)", id, got, err)
		}
	})

	t.Run("TY-37_MalformedUUID", func(t *testing.T) {
		_, err := uuidToXUID("not-a-uuid")
		if err == nil {
			t.Error("uuidToXUID() error = nil, want a parse error")
		}
	})

	t.Run("TY-38_NonDerivedUUID", func(t *testing.T) {
		_, err := uuidToXUID(uuid.New().String())
		if err == nil {
			t.Error("uuidToXUID() error = nil, want 'not a derived Bedrock UUID' error")
		}
	})
}

func TestTY39to43_TexturesRow_Value(t *testing.T) {
	skinHash := "skinhash"
	capeHash := "capehash"

	t.Run("TY-39_SkinAndCapeNoModel", func(t *testing.T) {
		row := &TexturesRow{PlayerId: "id", Skin: &skinHash, Cape: &capeHash, LastSeen: 100}
		got := row.Value("Steve", "http://cdn/")
		if got == nil {
			t.Fatal("Value() = nil, want a decoded TexturesValue")
		}
		if got.Textures.SKIN == nil || got.Textures.SKIN.URL != "http://cdn/skinhash" {
			t.Errorf("SKIN = %+v, want URL http://cdn/skinhash", got.Textures.SKIN)
		}
		if got.Textures.CAPE == nil || got.Textures.CAPE.URL != "http://cdn/capehash" {
			t.Errorf("CAPE = %+v, want URL http://cdn/capehash", got.Textures.CAPE)
		}
		if got.Textures.SKIN.Metadata != nil {
			t.Errorf("SKIN.Metadata = %+v, want nil (no model)", got.Textures.SKIN.Metadata)
		}
	})

	t.Run("TY-40_SlimModel", func(t *testing.T) {
		slim := SLIM
		row := &TexturesRow{PlayerId: "id", Skin: &skinHash, Model: &slim, LastSeen: 100}
		got := row.Value("Steve", "http://cdn/")
		if got.Textures.SKIN.Metadata == nil || got.Textures.SKIN.Metadata.Model != SLIM {
			t.Errorf("SKIN.Metadata = %+v, want Model=SLIM", got.Textures.SKIN.Metadata)
		}
	})

	t.Run("TY-41_NilReceiver", func(t *testing.T) {
		var row *TexturesRow
		if got := row.Value("Steve", "http://cdn/"); got != nil {
			t.Errorf("Value() = %+v, want nil", got)
		}
	})

	t.Run("TY-42_NoSkinOrCape", func(t *testing.T) {
		row := &TexturesRow{PlayerId: "id", LastSeen: 100}
		if got := row.Value("Steve", "http://cdn/"); got != nil {
			t.Errorf("Value() = %+v, want nil", got)
		}
	})

	t.Run("TY-43_OnlySkin", func(t *testing.T) {
		row := &TexturesRow{PlayerId: "id", Skin: &skinHash, LastSeen: 100}
		got := row.Value("Steve", "http://cdn/")
		if got.Textures.CAPE != nil {
			t.Errorf("CAPE = %+v, want nil", got.Textures.CAPE)
		}
	})
}
