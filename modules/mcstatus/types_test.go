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

func TestNewServerStatus(t *testing.T) {
	t.Run("TY-01_AllFieldsMapped", func(t *testing.T) {
		players := []*mcstatuspb.Player{{Name: "Steve", Uuid: "uuid-1"}}
		icon := tySmallImage()
		raw := "raw-marker"

		status := NewServerStatus("play.example.com", 25565, "My Server", "Welcome!", "world", 20, 5, players, "1.20.1", "favicon-data", ServerTypeJava, raw, icon)

		if status.Host != "play.example.com" {
			t.Errorf("Host: expected %q, got %q", "play.example.com", status.Host)
		}
		if status.Port != 25565 {
			t.Errorf("Port: expected 25565, got %d", status.Port)
		}
		if status.Name != "My Server" {
			t.Errorf("Name: expected %q, got %q", "My Server", status.Name)
		}
		if status.Motd != "Welcome!" {
			t.Errorf("Motd: expected %q, got %q", "Welcome!", status.Motd)
		}
		if status.Map != "world" {
			t.Errorf("Map: expected %q, got %q", "world", status.Map)
		}
		if status.MaxPlayers != 20 {
			t.Errorf("MaxPlayers: expected 20, got %d", status.MaxPlayers)
		}
		if status.NumPlayers != 5 {
			t.Errorf("NumPlayers: expected 5, got %d", status.NumPlayers)
		}
		if len(status.Players) != 1 || status.Players[0].Name != "Steve" {
			t.Errorf("Players: expected [{Steve uuid-1}], got %+v", status.Players)
		}
		if status.Version != "1.20.1" {
			t.Errorf("Version: expected %q, got %q", "1.20.1", status.Version)
		}
		if status.Favicon != "favicon-data" {
			t.Errorf("Favicon: expected %q, got %q", "favicon-data", status.Favicon)
		}
		if status.ServerType != ServerTypeJava {
			t.Errorf("ServerType (wrapper): expected %q, got %q", ServerTypeJava, status.ServerType)
		}
		if status.ServerStatus.ServerType != mcstatuspb.ServerType_JAVA {
			t.Errorf("ServerType (proto enum): expected JAVA, got %v", status.ServerStatus.ServerType)
		}
		if status.Raw != raw {
			t.Errorf("Raw: expected %v, got %v", raw, status.Raw)
		}
		if status.Icon != icon {
			t.Errorf("Icon: expected the same image instance")
		}
	})

	t.Run("TY-02_BedrockServerTypeMapsToBedrockEnum", func(t *testing.T) {
		status := NewServerStatus("", 0, "", "", "", 0, 0, nil, "", "", ServerTypeBedrock, nil, nil)

		if status.ServerType != ServerTypeBedrock {
			t.Errorf("ServerType (wrapper): expected %q, got %q", ServerTypeBedrock, status.ServerType)
		}
		if status.ServerStatus.ServerType != mcstatuspb.ServerType_BEDROCK {
			t.Errorf("ServerType (proto enum): expected BEDROCK, got %v", status.ServerStatus.ServerType)
		}
	})

	t.Run("TY-03_NilPlayersPassedThrough", func(t *testing.T) {
		status := NewServerStatus("", 0, "", "", "", 0, 0, nil, "", "", ServerTypeJava, nil, nil)

		if status.Players != nil {
			t.Errorf("expected nil Players, got %+v", status.Players)
		}
	})

	t.Run("TY-04_NilIconPassedThrough", func(t *testing.T) {
		status := NewServerStatus("", 0, "", "", "", 0, 0, nil, "", "", ServerTypeJava, nil, nil)

		if status.Icon != nil {
			t.Errorf("expected nil Icon, got %v", status.Icon)
		}
	})

	t.Run("TY-05_UnknownServerTypeDefaultsProtoEnumToZeroValue", func(t *testing.T) {
		status := NewServerStatus("", 0, "", "", "", 0, 0, nil, "", "", ServerType("unknown"), nil, nil)

		if status.ServerType != ServerType("unknown") {
			t.Errorf("ServerType (wrapper): expected %q, got %q", "unknown", status.ServerType)
		}
		if status.ServerStatus.ServerType != mcstatuspb.ServerType_JAVA {
			t.Errorf("ServerType (proto enum): expected the zero value (JAVA), got %v", status.ServerStatus.ServerType)
		}
	})
}

func TestMOTDToName(t *testing.T) {
	t.Run("TY-06_PlainMOTDUnchanged", func(t *testing.T) {
		if got := MOTDToName("Hello World"); got != "Hello World" {
			t.Errorf("expected %q, got %q", "Hello World", got)
		}
	})

	t.Run("TY-07_NewlineReplacedWithSpace", func(t *testing.T) {
		if got := MOTDToName("Line1\nLine2"); got != "Line1 Line2" {
			t.Errorf("expected %q, got %q", "Line1 Line2", got)
		}
	})

	t.Run("TY-08_ColorCodesStripped", func(t *testing.T) {
		if got := MOTDToName("§aRed§r Text"); got != "Red Text" {
			t.Errorf("expected %q, got %q", "Red Text", got)
		}
	})

	t.Run("TY-09_ResultIsTrimmed", func(t *testing.T) {
		if got := MOTDToName("  \nHello\n  "); got != "Hello" {
			t.Errorf("expected %q, got %q", "Hello", got)
		}
	})

	t.Run("TY-10_EmptyStringReturnsEmpty", func(t *testing.T) {
		if got := MOTDToName(""); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})
}

func TestImgToBase64(t *testing.T) {
	t.Run("TY-11_ValidImageEncodedAsDataURI", func(t *testing.T) {
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

	t.Run("TY-12_NilImageReturnsEmptyString", func(t *testing.T) {
		if got := ImgToBase64(nil); got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})
}

func TestLoadImgFromFile(t *testing.T) {
	t.Run("TY-13_ValidPNGFileDecodes", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "icon.png")
		img := tySmallImage()
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatalf("failed to encode test PNG: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("failed to close test file: %v", err)
		}

		got, err := LoadImgFromFile(path)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if got.Bounds() != img.Bounds() {
			t.Fatalf("expected bounds %v, got %v", img.Bounds(), got.Bounds())
		}
	})

	t.Run("TY-14_MissingFileReturnsError", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "does-not-exist.png")

		got, err := LoadImgFromFile(path)
		if err == nil {
			t.Fatal("expected a non-nil error")
		}
		if got != nil {
			t.Errorf("expected nil image, got %v", got)
		}
	})

	t.Run("TY-15_InvalidImageDataReturnsError", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "garbage.png")
		if err := os.WriteFile(path, []byte("not an image"), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		got, err := LoadImgFromFile(path)
		if err == nil {
			t.Fatal("expected a non-nil decode error")
		}
		if got != nil {
			t.Errorf("expected nil image, got %v", got)
		}
	})
}

func TestGetPing17Status(t *testing.T) {
	t.Run("TY-16_AllFieldsMapped", func(t *testing.T) {
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

	t.Run("TY-17_EmptySamplePlayersYieldsEmptySlice", func(t *testing.T) {
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
	t.Run("TY-18_AllFieldsMapped", func(t *testing.T) {
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
	t.Run("TY-19_AllFieldsMapped", func(t *testing.T) {
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
	t.Run("TY-20_AllFieldsMapped", func(t *testing.T) {
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
	t.Run("TY-21_AllFieldsMapped", func(t *testing.T) {
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

	t.Run("TY-22_EmptySamplePlayersYieldsEmptySlice", func(t *testing.T) {
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
	t.Run("TY-23_ExtraWithMapNamePresent", func(t *testing.T) {
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

	t.Run("TY-24_NoExtraOmitsSecondLineAndMap", func(t *testing.T) {
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

	t.Run("TY-25_ExtraLengthTwoOmitsMap", func(t *testing.T) {
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
}
