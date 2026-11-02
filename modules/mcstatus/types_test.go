package mcstatus

import (
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/mcstatuspb"
	"github.com/ZeroErrors/go-bedrockping"
	"github.com/dreamscached/minequery/v2"
	"github.com/google/uuid"
)

// tyStubChat17 is a minimal fmt.Stringer standing in for minequery's
// unexported Chat17 implementation, which cannot be constructed from outside
// that package. minequery.Status17.Description only requires satisfying
// minequery.Chat17 (interface{ fmt.Stringer }).
type tyStubChat17 string

func (c tyStubChat17) String() string { return string(c) }

func tySmallImage() image.Image {
	return image.NewRGBA(image.Rect(0, 0, 2, 2))
}

func TestMOTDToName(t *testing.T) {
	t.Run("CST-07_NewlineReplacedWithSpace", func(t *testing.T) {
		if got := MOTDToName("Line1\nLine2"); got != "Line1 Line2" {
			t.Errorf("expected %q, got %q", "Line1 Line2", got)
		}
	})

	t.Run("CST-08_ColorCodesStripped", func(t *testing.T) {
		if got := MOTDToName("§aRed§r Text"); got != "Red Text" {
			t.Errorf("expected %q, got %q", "Red Text", got)
		}
	})

	t.Run("CST-09_ResultIsTrimmed", func(t *testing.T) {
		if got := MOTDToName("  \nHello\n  "); got != "Hello" {
			t.Errorf("expected %q, got %q", "Hello", got)
		}
	})
}

func TestImgToBase64(t *testing.T) {
	t.Run("CST-11_ValidImageEncodedAsDataURI", func(t *testing.T) {
		img := tySmallImage()
		got := ImgToBase64(img)

		const prefix = "data:image/png;base64,"
		if !strings.HasPrefix(got, prefix) {
			t.Fatalf("expected prefix %q, got %q", prefix, got)
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(got, prefix))
		if err != nil {
			t.Fatalf("payload did not decode as base64: %v", err)
		}
		png2, err := png.Decode(strings.NewReader(string(decoded)))
		if err != nil {
			t.Fatalf("payload did not decode as PNG: %v", err)
		}
		if png2.Bounds() != img.Bounds() {
			t.Fatalf("expected bounds %v, got %v", img.Bounds(), png2.Bounds())
		}
	})

	t.Run("CST-12_NilImageReturnsEmptyString", func(t *testing.T) {
		if got := ImgToBase64(nil); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})
}

func TestLoadImgFromFS(t *testing.T) {
	t.Run("CST-14_MissingFileReturnsError", func(t *testing.T) {
		dir := t.TempDir()

		got, err := LoadImgFromFS(os.DirFS(dir), "does-not-exist.png")
		if err == nil {
			t.Fatal("expected a non-nil error")
		}
		if got != nil {
			t.Errorf("expected nil image, got %v", got)
		}
	})

	t.Run("CST-15_InvalidImageDataReturnsError", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "garbage.png")
		if err := os.WriteFile(path, []byte("not an image"), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		got, err := LoadImgFromFS(os.DirFS(dir), "garbage.png")
		if err == nil {
			t.Fatal("expected a non-nil decode error")
		}
		if got != nil {
			t.Errorf("expected nil image, got %v", got)
		}
	})
}

func TestGetPing17Status(t *testing.T) {
	t.Run("CST-16_AllFieldsMapped", func(t *testing.T) {
		icon := tySmallImage()
		expectedFavicon := ImgToBase64(icon)
		playerUUID := uuid.New()
		s := &minequery.Status17{
			VersionName:   "1.20.1",
			OnlinePlayers: 5,
			MaxPlayers:    20,
			SamplePlayers: []minequery.PlayerEntry17{{Nickname: "Steve", UUID: playerUUID}},
			Description:   tyStubChat17("Hello\nWorld"),
			Icon:          icon,
		}

		result := GetPing17Status(s)

		if result.Name != "Hello World" {
			t.Errorf("Name: expected %q, got %q", "Hello World", result.Name)
		}
		if result.Motd != "Hello\\nWorld" {
			t.Errorf("Motd: expected %q, got %q", "Hello\\nWorld", result.Motd)
		}
		if result.MaxPlayers != 20 {
			t.Errorf("MaxPlayers: expected 20, got %d", result.MaxPlayers)
		}
		if result.NumPlayers != 5 {
			t.Errorf("NumPlayers: expected 5, got %d", result.NumPlayers)
		}
		if len(result.Players) != 1 || result.Players[0].Name != "Steve" || result.Players[0].Uuid != playerUUID.String() {
			t.Errorf("Players: expected [{Steve %s}], got %+v", playerUUID.String(), result.Players)
		}
		if result.Version != "1.20.1" {
			t.Errorf("Version: expected %q, got %q", "1.20.1", result.Version)
		}
		if result.Favicon != expectedFavicon {
			t.Errorf("Favicon: expected %q, got %q", expectedFavicon, result.Favicon)
		}
		if result.ServerType != ServerTypeJava {
			t.Errorf("ServerType: expected %q, got %q", ServerTypeJava, result.ServerType)
		}
		if result.Legacy {
			t.Errorf("Legacy: expected false")
		}
		if result.Raw != s {
			t.Errorf("Raw: expected the original *Status17 pointer")
		}
		if result.Icon != icon {
			t.Errorf("Icon: expected the original icon instance")
		}
		if s.Icon != nil {
			t.Errorf("expected the input Status17.Icon to be cleared (nilled) after conversion, got %v", s.Icon)
		}
	})

	t.Run("CST-17_EmptySamplePlayersYieldsEmptySlice", func(t *testing.T) {
		s := &minequery.Status17{
			Description:   tyStubChat17(""),
			SamplePlayers: nil,
		}

		result := GetPing17Status(s)

		if result.Players == nil {
			t.Fatal("expected a non-nil empty slice, got nil")
		}
		if len(result.Players) != 0 {
			t.Fatalf("expected 0 players, got %d", len(result.Players))
		}
	})
}

func TestGetPing16Status(t *testing.T) {
	t.Run("CST-18_AllFieldsMapped", func(t *testing.T) {
		s := &minequery.Status16{
			MOTD:          "Hi\nThere",
			OnlinePlayers: 3,
			MaxPlayers:    10,
		}

		result := GetPing16Status(s)

		if result.Name != "Hi There" {
			t.Errorf("Name: expected %q, got %q", "Hi There", result.Name)
		}
		if result.Motd != "Hi\\nThere" {
			t.Errorf("Motd: expected %q, got %q", "Hi\\nThere", result.Motd)
		}
		if result.MaxPlayers != 10 {
			t.Errorf("MaxPlayers: expected 10, got %d", result.MaxPlayers)
		}
		if result.NumPlayers != 3 {
			t.Errorf("NumPlayers: expected 3, got %d", result.NumPlayers)
		}
		if len(result.Players) != 0 {
			t.Errorf("Players: expected empty slice, got %+v", result.Players)
		}
		if result.Version != "1.6" {
			t.Errorf("Version: expected %q, got %q", "1.6", result.Version)
		}
		if result.Favicon != "" {
			t.Errorf("Favicon: expected empty string, got %q", result.Favicon)
		}
		if result.ServerType != ServerTypeJava {
			t.Errorf("ServerType: expected %q, got %q", ServerTypeJava, result.ServerType)
		}
		if result.Icon != nil {
			t.Errorf("Icon: expected nil, got %v", result.Icon)
		}
		if !result.Legacy {
			t.Errorf("Legacy: expected true")
		}
		if result.Raw != s {
			t.Errorf("Raw: expected the original *Status16 pointer")
		}
	})
}

func TestGetPing14Status(t *testing.T) {
	t.Run("CST-19_AllFieldsMapped", func(t *testing.T) {
		s := &minequery.Status14{
			MOTD:          "Old\nServer",
			OnlinePlayers: 2,
			MaxPlayers:    8,
		}

		result := GetPing14Status(s)

		if result.Name != "Old Server" {
			t.Errorf("Name: expected %q, got %q", "Old Server", result.Name)
		}
		if result.Motd != "Old\\nServer" {
			t.Errorf("Motd: expected %q, got %q", "Old\\nServer", result.Motd)
		}
		if result.MaxPlayers != 8 {
			t.Errorf("MaxPlayers: expected 8, got %d", result.MaxPlayers)
		}
		if result.NumPlayers != 2 {
			t.Errorf("NumPlayers: expected 2, got %d", result.NumPlayers)
		}
		if result.Version != "1.4-1.5" {
			t.Errorf("Version: expected %q, got %q", "1.4-1.5", result.Version)
		}
		if !result.Legacy {
			t.Errorf("Legacy: expected true")
		}
		if result.Raw != s {
			t.Errorf("Raw: expected the original *Status14 pointer")
		}
	})
}

func TestGetBeta18Status(t *testing.T) {
	t.Run("CST-20_AllFieldsMapped", func(t *testing.T) {
		s := &minequery.StatusBeta18{
			MOTD:          "Beta\nServer",
			OnlinePlayers: 1,
			MaxPlayers:    4,
		}

		result := GetBeta18Status(s)

		if result.Name != "Beta Server" {
			t.Errorf("Name: expected %q, got %q", "Beta Server", result.Name)
		}
		if result.Motd != "Beta\\nServer" {
			t.Errorf("Motd: expected %q, got %q", "Beta\\nServer", result.Motd)
		}
		if result.MaxPlayers != 4 {
			t.Errorf("MaxPlayers: expected 4, got %d", result.MaxPlayers)
		}
		if result.NumPlayers != 1 {
			t.Errorf("NumPlayers: expected 1, got %d", result.NumPlayers)
		}
		if result.Version != "b1.8-1.3" {
			t.Errorf("Version: expected %q, got %q", "b1.8-1.3", result.Version)
		}
		if !result.Legacy {
			t.Errorf("Legacy: expected true")
		}
		if result.Raw != s {
			t.Errorf("Raw: expected the original *StatusBeta18 pointer")
		}
	})
}

func TestGetQueryStatus(t *testing.T) {
	t.Run("CST-21_AllFieldsMapped", func(t *testing.T) {
		s := &minequery.FullQueryStatus{
			MOTD:          "Q\nMotd",
			OnlinePlayers: 7,
			MaxPlayers:    15,
			SamplePlayers: []string{"Alice", "Bob"},
			Version:       "1.19.4",
		}

		result := GetQueryStatus(s)

		if result.Name != "Q Motd" {
			t.Errorf("Name: expected %q, got %q", "Q Motd", result.Name)
		}
		if result.Motd != "Q\\nMotd" {
			t.Errorf("Motd: expected %q, got %q", "Q\\nMotd", result.Motd)
		}
		if result.MaxPlayers != 15 {
			t.Errorf("MaxPlayers: expected 15, got %d", result.MaxPlayers)
		}
		if result.NumPlayers != 7 {
			t.Errorf("NumPlayers: expected 7, got %d", result.NumPlayers)
		}
		if len(result.Players) != 2 || result.Players[0].Name != "Alice" || result.Players[0].Uuid != "" || result.Players[1].Name != "Bob" {
			t.Errorf("Players: expected [{Alice } {Bob }], got %+v", result.Players)
		}
		if result.Version != "1.19.4" {
			t.Errorf("Version: expected %q, got %q", "1.19.4", result.Version)
		}
		if result.Favicon != "" {
			t.Errorf("Favicon: expected empty string, got %q", result.Favicon)
		}
		if result.ServerType != ServerTypeJava {
			t.Errorf("ServerType: expected %q, got %q", ServerTypeJava, result.ServerType)
		}
		if result.Legacy {
			t.Errorf("Legacy: expected false")
		}
		if result.Raw != s {
			t.Errorf("Raw: expected the original *FullQueryStatus pointer")
		}
	})

	t.Run("CST-22_EmptySamplePlayersYieldsEmptySlice", func(t *testing.T) {
		s := &minequery.FullQueryStatus{SamplePlayers: nil}

		result := GetQueryStatus(s)

		if result.Players == nil {
			t.Fatal("expected a non-nil empty slice, got nil")
		}
		if len(result.Players) != 0 {
			t.Fatalf("expected 0 players, got %d", len(result.Players))
		}
	})
}

func TestGetBedrockStatus(t *testing.T) {
	t.Run("CST-23_ExtraWithMapNamePresent", func(t *testing.T) {
		s := bedrockping.Response{
			ServerName:  "BedrockSrv",
			Extra:       []string{"unused0", "motdLine1", "MyMap"},
			MaxPlayers:  50,
			PlayerCount: 12,
			MCPEVersion: "1.20.10",
		}

		result := GetBedrockStatus(s)

		if result.Motd != "BedrockSrv\\nmotdLine1" {
			t.Errorf("Motd: expected %q, got %q", "BedrockSrv\\nmotdLine1", result.Motd)
		}
		if result.Map != "MyMap" {
			t.Errorf("Map: expected %q, got %q", "MyMap", result.Map)
		}
		if result.MaxPlayers != 50 {
			t.Errorf("MaxPlayers: expected 50, got %d", result.MaxPlayers)
		}
		if result.NumPlayers != 12 {
			t.Errorf("NumPlayers: expected 12, got %d", result.NumPlayers)
		}
		if len(result.Players) != 0 {
			t.Errorf("Players: expected empty slice, got %+v", result.Players)
		}
		if result.Version != "1.20.10" {
			t.Errorf("Version: expected %q, got %q", "1.20.10", result.Version)
		}
		if result.Favicon != "" {
			t.Errorf("Favicon: expected empty string, got %q", result.Favicon)
		}
		if !reflect.DeepEqual(result.Raw, s) {
			t.Errorf("Raw: expected %+v, got %+v", s, result.Raw)
		}
	})

	t.Run("CST-24_NoExtraOmitsSecondLineAndMap", func(t *testing.T) {
		s := bedrockping.Response{
			ServerName:  "BedrockSrv",
			Extra:       nil,
			MaxPlayers:  20,
			PlayerCount: 1,
			MCPEVersion: "1.20.10",
		}

		result := GetBedrockStatus(s)

		if result.Name != "BedrockSrv" {
			t.Errorf("Name: expected %q, got %q", "BedrockSrv", result.Name)
		}
		if result.Motd != "BedrockSrv" {
			t.Errorf("Motd: expected %q, got %q", "BedrockSrv", result.Motd)
		}
		if result.Map != "" {
			t.Errorf("Map: expected empty string, got %q", result.Map)
		}
	})

	t.Run("CST-25_ExtraLengthTwoOmitsMap", func(t *testing.T) {
		s := bedrockping.Response{
			ServerName:  "BedrockSrv",
			Extra:       []string{"unused0", "motdLine1"},
			MaxPlayers:  20,
			PlayerCount: 1,
			MCPEVersion: "1.20.10",
		}

		result := GetBedrockStatus(s)

		if result.Motd != "BedrockSrv\\nmotdLine1" {
			t.Errorf("Motd: expected %q, got %q", "BedrockSrv\\nmotdLine1", result.Motd)
		}
		if result.Map != "" {
			t.Errorf("Map: expected empty string (len(Extra)==2 is not >2), got %q", result.Map)
		}
	})

	t.Run("CST-26_BedrockStatusReportsBedrockType", func(t *testing.T) {
		result := GetBedrockStatus(bedrockping.Response{ServerName: "BedrockSrv"})

		if result.ServerType != ServerTypeBedrock {
			t.Errorf("ServerType: expected %q, got %q", ServerTypeBedrock, result.ServerType)
		}
		if result.ServerStatus.ServerType != mcstatuspb.ServerType_BEDROCK {
			t.Errorf("ServerStatus.ServerType: expected %v, got %v", mcstatuspb.ServerType_BEDROCK, result.ServerStatus.ServerType)
		}
	})
}
