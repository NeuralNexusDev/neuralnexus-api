package mcstatus

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"regexp"
	"strings"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/mcstatuspb"
	"github.com/ZeroErrors/go-bedrockping"
	"github.com/dreamscached/minequery/v2"
)

// MCServerStatus - Minecraft Status response
type MCServerStatus struct {
	mcstatuspb.ServerStatus
	ServerType ServerType  `json:"server_type" xml:"server_type"`
	Raw        interface{} `json:"raw,omitempty" xml:"raw,omitempty"`
	Icon       image.Image `json:"-" xml:"-"`
}

// ServerType - Server type enum
type ServerType string

// ServerType constants
const (
	ServerTypeJava    ServerType = "java"
	ServerTypeBedrock ServerType = "bedrock"
)

// NewServerStatus - Create a new server status
func NewServerStatus(host string, port int, name string, motd string, mapName string, maxPlayers int, numPlayers int, players []*mcstatuspb.Player, version string, favicon string, serverType ServerType, raw interface{}, icon image.Image) *MCServerStatus {
	return &MCServerStatus{
		ServerStatus: mcstatuspb.ServerStatus{
			Host:       host,
			Port:       int32(port),
			Name:       name,
			Motd:       motd,
			Map:        mapName,
			MaxPlayers: int32(maxPlayers),
			NumPlayers: int32(numPlayers),
			Players:    players,
			Version:    version,
			Favicon:    favicon,
			ServerType: mcstatuspb.ServerType(mcstatuspb.ServerType_value[strings.ToUpper(string(serverType))]),
		},
		ServerType: serverType,
		Raw:        raw,
		Icon:       icon,
	}
}

// MOTDToName - Convert MOTD to name
func MOTDToName(motd string) string {
	name := strings.ReplaceAll(motd, "\n", " ")
	name = regexp.MustCompile("\u00a7[a-f0-9k-or]").ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	return name
}

// ImgToBase64 - Convert image.Image to base64 string
func ImgToBase64(i image.Image) string {
	if i == nil {
		return ""
	}
	var buff bytes.Buffer
	png.Encode(&buff, i)
	var encodedString string = base64.StdEncoding.EncodeToString(buff.Bytes())
	return "data:image/png;base64," + encodedString
}

// LoadImgFromFile - Load image from file
func LoadImgFromFile(path string) (image.Image, error) {
	// Open the file
	iconFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer iconFile.Close()

	// Decode the image
	img, _, err := image.Decode(iconFile)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// GetPing17Status - Get the Java server status
func GetPing17Status(s *minequery.Status17) *MCServerStatus {
	icon := s.Icon
	s.Icon = nil
	// TODO: Parse the motd and keep data
	motd := s.Description.String()

	playerList := []*mcstatuspb.Player{}
	for _, player := range s.SamplePlayers {
		playerList = append(playerList, &mcstatuspb.Player{
			Name: player.Nickname,
			Uuid: player.UUID.String(),
		})
	}

	return NewServerStatus(
		"", 0,
		MOTDToName(motd),
		strings.ReplaceAll(motd, "\n", "\\n"),
		"",
		s.MaxPlayers,
		s.OnlinePlayers,
		playerList,
		s.VersionName,
		ImgToBase64(icon),
		ServerTypeJava,
		s,
		icon,
	)
}

// GetPing16Status - Get the Java server status
func GetPing16Status(s *minequery.Status16) *MCServerStatus {
	motd := s.MOTD
	return NewServerStatus(
		"", 0,
		MOTDToName(motd),
		strings.ReplaceAll(motd, "\n", "\\n"),
		"",
		s.MaxPlayers,
		s.OnlinePlayers,
		[]*mcstatuspb.Player{},
		"1.6",
		"",
		ServerTypeJava,
		s,
		nil,
	)
}

// GetPing14Status - Get the Java server status
func GetPing14Status(s *minequery.Status14) *MCServerStatus {
	motd := s.MOTD
	return NewServerStatus(
		"", 0,
		MOTDToName(motd),
		strings.ReplaceAll(motd, "\n", "\\n"),
		"",
		s.MaxPlayers,
		s.OnlinePlayers,
		[]*mcstatuspb.Player{},
		"1.4-1.5",
		"",
		ServerTypeJava,
		s,
		nil,
	)
}

// GetBeta18Status - Get the Java server status
func GetBeta18Status(s *minequery.StatusBeta18) *MCServerStatus {
	motd := s.MOTD
	return NewServerStatus(
		"", 0,
		MOTDToName(motd),
		strings.ReplaceAll(motd, "\n", "\\n"),
		"",
		s.MaxPlayers,
		s.OnlinePlayers,
		[]*mcstatuspb.Player{},
		"b1.8-1.3",
		"",
		ServerTypeJava,
		s,
		nil,
	)
}

// GetQueryStatus - Get the Java server status
func GetQueryStatus(s *minequery.FullQueryStatus) *MCServerStatus {
	motd := s.MOTD
	playerList := []*mcstatuspb.Player{}
	for _, player := range s.SamplePlayers {
		playerList = append(playerList, &mcstatuspb.Player{
			Name: player,
		})
	}

	return NewServerStatus(
		"", 0,
		MOTDToName(motd),
		strings.ReplaceAll(motd, "\n", "\\n"),
		"",
		s.MaxPlayers,
		s.OnlinePlayers,
		playerList,
		s.Version,
		"",
		ServerTypeJava,
		s,
		nil,
	)
}

// GetBedrockStatus - Get the Bedrock server status
func GetBedrockStatus(s bedrockping.Response) *MCServerStatus {
	motd := s.ServerName
	if len(s.Extra) > 1 {
		motd += "\\n" + s.Extra[1]
	}

	var mapName string = ""
	if len(s.Extra) > 2 {
		mapName = s.Extra[2]
	}

	return NewServerStatus(
		"", 0,
		MOTDToName(motd),
		strings.ReplaceAll(motd, "\n", "\\n"),
		mapName,
		s.MaxPlayers,
		s.PlayerCount,
		[]*mcstatuspb.Player{},
		s.MCPEVersion,
		"",
		ServerTypeJava,
		s,
		nil,
	)
}
