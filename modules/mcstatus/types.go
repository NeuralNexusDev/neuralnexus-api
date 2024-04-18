package mcstatus

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"regexp"
	"strings"

	"github.com/ZeroErrors/go-bedrockping"
	"github.com/dreamscached/minequery/v2"
	"github.com/google/uuid"
)

// MCStatusResponse - Minecraft Status response
type MCStatusResponse struct {
	Host       string      `json:"host" xml:"host"`
	Port       int         `json:"port" xml:"port"`
	Name       string      `json:"name" xml:"name"`
	MOTD       string      `json:"motd" xml:"motd"`
	Map        string      `json:"map" xml:"map"`
	MaxPlayers int         `json:"max_players" xml:"max_players"`
	NumPlayers int         `json:"num_players" xml:"num_players"`
	Players    []Player    `json:"players" xml:"players"`
	Version    string      `json:"version" xml:"version"`
	Favicon    string      `json:"favicon" xml:"favicon"`
	ServerType ServerType  `json:"server_type" xml:"server_type"`
	Raw        interface{} `json:"raw,omitempty" xml:"raw,omitempty"`
	Icon       image.Image `json:"-" xml:"-"`
}

// Player - Player info
type Player struct {
	Name string    `json:"name" xml:"name"`
	UUID uuid.UUID `json:"uuid" xml:"uuid"`
}

// ServerType - Server type enum
type ServerType string

// ServerType constants
const (
	ServerTypeJava    ServerType = "java"
	ServerTypeBedrock ServerType = "bedrock"
)

// MOTDToName - Convert MOTD to name
func MOTDToName(motd string) string {
	name := strings.ReplaceAll(motd, "\n", " ")
	name = regexp.MustCompile("\u00a7[a-f0-9k-or]").ReplaceAllString(name, "")
	name = strings.TrimSpace(name)
	return name
}

// ImgToBase64 - Convert image.Image to base64 string
func ImgToBase64(i image.Image) string {
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
func GetPing17Status(s *minequery.Status17) *MCStatusResponse {
	icon := s.Icon
	s.Icon = nil
	// TODO: Parse the motd and keep data
	motd := s.Description.String()

	var playerList []Player = []Player{}
	for _, player := range s.SamplePlayers {
		playerList = append(playerList, Player{
			Name: player.Nickname,
			UUID: player.UUID,
		})
	}

	return &MCStatusResponse{
		Name:       MOTDToName(motd),
		MOTD:       strings.ReplaceAll(motd, "\n", "\\n"),
		MaxPlayers: s.MaxPlayers,
		NumPlayers: s.OnlinePlayers,
		Players:    playerList,
		Favicon:    ImgToBase64(icon),
		ServerType: ServerTypeJava,
		Icon:       icon,
		Raw:        s,
	}
}

// GetPing16Status - Get the Java server status
func GetPing16Status(s *minequery.Status16) *MCStatusResponse {
	motd := s.MOTD
	return &MCStatusResponse{
		Name:       MOTDToName(motd),
		MOTD:       strings.ReplaceAll(motd, "\n", "\\n"),
		MaxPlayers: s.MaxPlayers,
		NumPlayers: s.OnlinePlayers,
		Players:    []Player{},
		Favicon:    LegacyIcon,
		ServerType: ServerTypeJava,
		Icon:       ImgLegacyIcon,
		Raw:        s,
	}
}

// GetPing14Status - Get the Java server status
func GetPing14Status(s *minequery.Status14) *MCStatusResponse {
	motd := s.MOTD
	return &MCStatusResponse{
		Name:       MOTDToName(motd),
		MOTD:       strings.ReplaceAll(motd, "\n", "\\n"),
		MaxPlayers: s.MaxPlayers,
		NumPlayers: s.OnlinePlayers,
		Players:    []Player{},
		Favicon:    LegacyIcon,
		ServerType: ServerTypeJava,
		Icon:       ImgLegacyIcon,
		Raw:        s,
	}
}

// GetBeta18Status - Get the Java server status
func GetBeta18Status(s *minequery.StatusBeta18) *MCStatusResponse {
	motd := s.MOTD
	return &MCStatusResponse{
		Name:       MOTDToName(motd),
		MOTD:       strings.ReplaceAll(motd, "\n", "\\n"),
		MaxPlayers: s.MaxPlayers,
		NumPlayers: s.OnlinePlayers,
		Players:    []Player{},
		Favicon:    LegacyIcon,
		ServerType: ServerTypeJava,
		Icon:       ImgLegacyIcon,
		Raw:        s,
	}
}

// GetQueryStatus - Get the Java server status
func GetQueryStatus(s *minequery.FullQueryStatus) *MCStatusResponse {
	motd := s.MOTD
	var playerList []Player = []Player{}
	for _, player := range s.SamplePlayers {
		playerList = append(playerList, Player{
			Name: player,
		})
	}

	return &MCStatusResponse{
		Name:       MOTDToName(motd),
		MOTD:       strings.ReplaceAll(motd, "\n", "\\n"),
		MaxPlayers: s.MaxPlayers,
		NumPlayers: s.OnlinePlayers,
		Players:    playerList,
		Favicon:    LegacyIcon,
		ServerType: ServerTypeJava,
		Icon:       ImgLegacyIcon,
		Raw:        s,
	}
}

// GetBedrockStatus - Get the Bedrock server status
func GetBedrockStatus(s bedrockping.Response) *MCStatusResponse {
	motd := s.ServerName
	if len(s.Extra) > 1 {
		motd += "\\n" + s.Extra[1]
	}

	var mapName string = ""
	if len(s.Extra) > 2 {
		mapName = s.Extra[2]
	}

	return &MCStatusResponse{
		Name:       MOTDToName(motd),
		MOTD:       strings.ReplaceAll(motd, "\n", "\\n"),
		Map:        mapName,
		MaxPlayers: s.MaxPlayers,
		NumPlayers: s.PlayerCount,
		Players:    []Player{},
		Favicon:    BedrockIcon,
		ServerType: ServerTypeBedrock,
		Icon:       ImgBedrockIcon,
		Raw:        s,
	}
}

// DefaultIcon - Default icon
const DefaultIcon = "data:image/png;base64,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"

// LegacyIcon - Legacy icon
const LegacyIcon = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAMAAACdt4HsAAABX1BMVEVHcExIcj1bj05aQSxiRS9dRC9HMyJUfUdeQy1NekFvUDZUg0h4WUFMNiVIMiF4Vjt7WDxmSDJcj05LUDA1JhkqPyRBYjdcQi1XPippo1qPZkc6VzJUhUhraGVSUlL///9hQkVKNCNhRC5bQi1RgEVjRjBSgkZPfURGMSFpSjJONyVaQCtIMyJKdT5VPClYiUpXPipwUDd4VjtOe0JWhklUhEhsTTVMeUFQOCZajExeQi1mSDFLdz9MNSQySyssQiY2JRlTOyhIcTxhllJAYjc6VzFyUjh1VDk9XTReklAvRyhcj042Ui5CLh87KRs+Kx0pPiNGbTuCXUGJYkNlnFZCZzlKOy+OZkd6Vzw0KRt+WT6TaUlTT0tDPCYwMR4wOyM1JRkoNh89QihfRzU3MSI7SCuVa0tsp1tDRitnVERCXDVORDtCQD48NC5kTj1mWlBuY1pra2tsbW1raGZHcEzZxmpuAAAAdXRSTlMA0nGTk3GTGhpyk9JxdNKT0tLSU5SUlNLS0ZOTHtLSGv///////////////////////////////////////////////////////////////////////////////////////////////////////////////wA/Eef4AAAJCUlEQVRYw5yWV5Mb1xGFSVrWcsmyJJO2qyTrZe7knHPABGAGg0HOGZsDl6Ro//8H99DaoCpZlnifNtT9uvuc0xd49uw3z4/Hxz8++/Jz9OK1WX3/4uiLr7+RKUKeTl8ffwni6Pi5bVC0TrgypT//wwiobqapaNupSMkGpQd/DHF0/NqQqUAndDsQRZswDDqgf/8g0HxKGwZhy5lMmyYdBLYuprqe/r4uQHnatg3Z1UeBSwcm6EjogU0bVBDor/+vIzB7WRm6TbmVS8iVSVdZ6Rq0qANFTANK/m0EVDcInZDLrKoLE5QdUG5W0Wlqw290IFKVbP5vLWB2SpYp25jCgeI0QdsBYQLSqGRaDGgTyKVL2L+uxdGL7ysCyslGOXVNN8sqShylMDxhyFU2zfRUpOWqLCuT+DVEXT0rZVembdMFESA/ppjSskvB5ISuu246SwPaKLNMzq4zyn3zCy2Ojt+4lFG6oJNIgHy0SJgUZUyvp7It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"

// BedrockIcon - Bedrock icon
const BedrockIcon = "data:image/png;base64,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"

var (
	ImgDefaultIcon, _ = LoadImgFromFile("public/mcstatus/icons/default.png")
	ImgLegacyIcon, _  = LoadImgFromFile("public/mcstatus/icons/legacy.png")
	ImgBedrockIcon, _ = LoadImgFromFile("public/mcstatus/icons/bedrock.png")
)
