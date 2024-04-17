package mcstatus

import gss "github.com/NeuralNexusDev/neuralnexus-api/modules/game_server_status"

var (
	SERVER_URL     string = "https://api.neuralnexus.dev/api/v1/mcstatus"
	defaultIcon, _        = loadImgFromFile("public/mcstatus/icons/default.png")
	bedrockIcon, _        = loadImgFromFile("public/mcstatus/icons/bedrock.png")
)

// ServerInfo contains the server info
type ServerInfo struct {
	Address     string
	Port        int
	EnableQuery bool
	QueryPort   int
	IsBedrock   bool
}

// NewOfflineServerStatus - Create a new empty ServerInfo
func NewOfflineServerStatus(isBedrock bool) StausResponse {
	status := StausResponse{
		Name:          "Server Offline",
		Map:           "",
		MaxPlayers:    0,
		OnlinePlayers: 0,
		Players:       []gss.Player{},
		Connect:       "",
		Version:       "",
	}

	if isBedrock {
		status.Favicon = imgToBase64(bedrockIcon)
		status.ServerType = "bedrock"
	} else {
		status.Favicon = imgToBase64(defaultIcon)
		status.ServerType = "java"
	}
	return status
}

// General status response
type StausResponse struct {
	Name          string       `json:"name" xml:"name"`
	Map           string       `json:"map" xml:"map"`
	MaxPlayers    int          `json:"maxplayers" xml:"maxplayers"`
	OnlinePlayers int          `json:"onlineplayers" xml:"onlineplayers"`
	Players       []gss.Player `json:"players" xml:"players"`
	Connect       string       `json:"connect" xml:"connect"`
	Version       string       `json:"version" xml:"version"`
	Favicon       string       `json:"favicon" xml:"favicon"`
	ServerType    string       `json:"server_type" xml:"server_type"`
}

// Error response
type ErrorResponse struct {
	Succes bool   `json:"success" xml:"success"`
	Error  string `json:"error" xml:"error"`
}
