package mcstatus

var (
	SERVER_URL     string = "https://api.neuralnexus.dev/api/v1/mcstatus"
	defaultIcon, _        = loadImgFromFile("public/mcstatus/icons/default.png")
	bedrockIcon, _        = loadImgFromFile("public/mcstatus/icons/bedrock.png")
)

// ServerInfo contains the server info
type ServerInfo struct {
	Address     string `json:"address"`
	Port        int    `json:"port"`
	EnableQuery bool   `json:"enable_query"`
	QueryPort   int    `json:"query_port"`
	IsBedrock   bool   `json:"is_bedrock"`
}

// NewOfflineServerStatus - Create a new empty ServerInfo
func NewOfflineServerStatus(isBedrock bool) StausResponse {
	status := StausResponse{
		Name:          "Server Offline",
		Map:           "",
		MaxPlayers:    0,
		OnlinePlayers: 0,
		Players:       []Player{},
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

// Simple Player definition
type Player struct {
	Name string `json:"name"`
}

// General status response
type StausResponse struct {
	Name          string   `json:"name"`
	Map           string   `json:"map"`
	MaxPlayers    int      `json:"maxplayers"`
	OnlinePlayers int      `json:"onlineplayers"`
	Players       []Player `json:"players"`
	Connect       string   `json:"connect"`
	Version       string   `json:"version"`
	Favicon       string   `json:"favicon"`
	ServerType    string   `json:"server_type"`
}

// Error response
type ErrorResponse struct {
	Succes bool   `json:"success"`
	Error  string `json:"error"`
}
