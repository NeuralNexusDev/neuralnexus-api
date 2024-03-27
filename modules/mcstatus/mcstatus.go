package mcstatus

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ZeroErrors/go-bedrockping"
	"github.com/dreamscached/minequery/v2"
)

// -------------- Globals --------------
var (
	SERVER_URL string = "https://api.neuralnexus.dev/api/v1/mcstatus"

	defaultIcon, _ = loadImgFromFile("static/mcstatus/icons/default.png")

	offlineJavaResponse StausResponse = StausResponse{
		Name:          "Server Offline",
		Map:           "",
		MaxPlayers:    0,
		OnlinePlayers: 0,
		Players:       []Player{},
		Connect:       "",
		Version:       "",
		Favicon:       imgToBase64(defaultIcon),
		ServerType:    "java",
	}

	bedrockIcon, _ = loadImgFromFile("static/mcstatus/icons/bedrock.png")

	offlineBedrockResponse StausResponse = StausResponse{
		Name:          "Server Offline",
		Map:           "",
		MaxPlayers:    0,
		OnlinePlayers: 0,
		Players:       []Player{},
		Connect:       "",
		Version:       "",
		Favicon:       imgToBase64(bedrockIcon),
		ServerType:    "bedrock",
	}
)

// -------------- Structs --------------

// ServerInfo contains the server info
type ServerInfo struct {
	Address     string `json:"address"`
	Port        int    `json:"port"`
	EnableQuery bool   `json:"enable_query"`
	QueryPort   int    `json:"query_port"`
	IsBedrock   bool   `json:"is_bedrock"`
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

// -------------- Functions --------------

// Convert image.Image to base64 string
func imgToBase64(i image.Image) string {
	// Buffer to encode image
	var buff bytes.Buffer

	// Encode image to writer
	png.Encode(&buff, i)

	// Encode byte array into base64 string
	var encodedString string = base64.StdEncoding.EncodeToString(buff.Bytes())

	return "data:image/png;base64," + encodedString
}

// Load image from file
func loadImgFromFile(path string) (image.Image, error) {
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

// Get params
func getServerInfo(r *http.Request) ServerInfo {
	// Get is_bedrock from params
	is_bedrock, err := strconv.ParseBool(r.URL.Query().Get("is_bedrock"))
	if err != nil {
		is_bedrock = false
	}

	// Get enable_query from params
	enable_query, err := strconv.ParseBool(r.URL.Query().Get("enable_query"))
	if err != nil {
		enable_query = true
	}

	// Get port from params
	portString := r.URL.Query().Get("port")

	// Get slug from params
	var address string = r.PathValue("address")

	// Check if the address contains a colon anywhere
	if strings.Contains(address, ":") {
		// Split the address into host and port
		var split []string = strings.Split(address, ":")
		address = split[0]
		portString = split[1]
	}

	// Parse the port
	port, err := strconv.Atoi(portString)
	if err != nil {
		if is_bedrock {
			port = 19132
		} else {
			port = 25565
		}
	}

	// Get query port from params
	query_port, err := strconv.Atoi(r.URL.Query().Get("query_port"))
	if err != nil {
		query_port = port
	}

	// Return the server info
	var serverInfo ServerInfo = ServerInfo{
		Address:     address,
		Port:        port,
		EnableQuery: enable_query,
		QueryPort:   query_port,
		IsBedrock:   is_bedrock,
	}

	// Get the request body
	body, err := io.ReadAll(r.Body)
	if err == nil {
		bodyString := string(body)

		// Convert the body to a map
		var bodyMap map[string]interface{}
		err = json.Unmarshal([]byte(bodyString), &bodyMap)
		if err == nil {
			// Get address from body
			if bodyMap["address"] != nil {
				serverInfo.Address = bodyMap["address"].(string)
			}

			// Get port from body
			if bodyMap["port"] != nil {
				serverInfo.Port = int(bodyMap["port"].(float64))
			}

			// Get query_port from body
			if bodyMap["query_port"] != nil {
				serverInfo.QueryPort = int(bodyMap["query_port"].(float64))
			} else {
				serverInfo.QueryPort = port
			}

			// Get is_bedrock from body
			if bodyMap["is_bedrock"] != nil {
				serverInfo.IsBedrock = bodyMap["is_bedrock"].(bool)
			} else {
				is_bedrock = false
			}

			// Get enable_query from body
			if bodyMap["enable_query"] != nil {
				serverInfo.EnableQuery = bodyMap["enable_query"].(bool)
			} else {
				enable_query = true
			}
		}
	}

	// Return the server info
	return serverInfo
}

// -------------- Status --------------

// Get Bedrock server status
func BedrockServerStatus(serverInfo ServerInfo) (StausResponse, image.Image, error) {
	// Build the connect string
	connect := serverInfo.Address + ":" + fmt.Sprint(serverInfo.Port)

	// Get the server status
	respB, err := bedrockping.Query(connect, 5*time.Second, 150*time.Millisecond)
	if err != nil {
		return offlineBedrockResponse, defaultIcon, err
	}

	// Get the server name
	serverName := respB.ServerName
	if len(respB.Extra) > 1 {
		serverName += "\\n" + respB.Extra[1]
	}

	var mapName string = ""
	if len(respB.Extra) > 2 {
		mapName = respB.Extra[2]
	}

	// Create the status response
	return StausResponse{
		Name:          serverName,
		Map:           mapName,
		MaxPlayers:    respB.MaxPlayers,
		OnlinePlayers: respB.PlayerCount,
		Players:       []Player{},
		Connect:       connect,
		Version:       respB.MCPEVersion,
		Favicon:       offlineBedrockResponse.Favicon,
		ServerType:    "bedrock",
	}, bedrockIcon, nil
}

// Parse players from Ping17 response
func parsePlayers17(players []minequery.PlayerEntry17) []Player {
	// Create a new array of players
	var playerList []Player = []Player{}

	// Loop through the players
	for _, player := range players {
		// Append the player to the player list
		playerList = append(playerList, Player{
			Name: player.Nickname,
		})
	}

	return playerList
}

// Parse players from Query response
func parsePlayersQuery(players []string) []Player {
	// Create a new array of players
	var playerList []Player = []Player{}

	// Loop through the players
	for _, player := range players {
		// Append the player to the player list
		playerList = append(playerList, Player{
			Name: player,
		})
	}

	return playerList
}

// Get Java server status
func JavaServerStatus(serverInfo ServerInfo) (StausResponse, image.Image, error) {
	// Create a new pinger
	pinger := minequery.NewPinger(
		minequery.WithTimeout(5*time.Second),
		minequery.WithProtocolVersion16(minequery.Ping16ProtocolVersion162),
		minequery.WithProtocolVersion17(minequery.Ping17ProtocolVersion119),
	)

	// Default struct data
	statusResponse := StausResponse{
		Name:          "",
		Map:           "",
		MaxPlayers:    0,
		OnlinePlayers: 0,
		Players:       []Player{},
		Connect:       serverInfo.Address + ":" + fmt.Sprint(serverInfo.Port),
		Version:       "",
		Favicon:       "",
		ServerType:    "java",
	}

	// Now the glorious if else chain
	var icon image.Image

	resp17, err := pinger.Ping17(serverInfo.Address, serverInfo.Port)
	if err != nil {
		resp16, err := pinger.Ping16(serverInfo.Address, serverInfo.Port)
		if err != nil {
			resp14, err := pinger.Ping14(serverInfo.Address, serverInfo.Port)
			if err != nil {
				resp15, err := pinger.PingBeta18(serverInfo.Address, serverInfo.Port)
				if err != nil {
					return offlineJavaResponse, defaultIcon, err
				}
				statusResponse.Name = resp15.MOTD
				statusResponse.MaxPlayers = resp15.MaxPlayers
				statusResponse.OnlinePlayers = resp15.OnlinePlayers
				statusResponse.Version = "1.5"
			}
			statusResponse.Name = resp14.MOTD
			statusResponse.MaxPlayers = resp14.MaxPlayers
			statusResponse.OnlinePlayers = resp14.OnlinePlayers
			statusResponse.Version = "1.4"
		}
		statusResponse.Name = resp16.MOTD
		statusResponse.MaxPlayers = resp16.MaxPlayers
		statusResponse.OnlinePlayers = resp16.OnlinePlayers
		statusResponse.Version = resp16.ServerVersion
	} else {
		icon = resp17.Icon
		statusResponse.Name = strings.ReplaceAll(resp17.Description.String(), "\n", "\\n")
		statusResponse.MaxPlayers = resp17.MaxPlayers
		statusResponse.OnlinePlayers = resp17.OnlinePlayers
		statusResponse.Players = parsePlayers17(resp17.SamplePlayers)
		statusResponse.Version = resp17.VersionName
		statusResponse.Favicon = imgToBase64(icon)
	}

	if serverInfo.EnableQuery {
		respQuery, err := pinger.QueryFull(serverInfo.Address, serverInfo.QueryPort)
		if err == nil {
			statusResponse.Name = respQuery.MOTD
			statusResponse.MaxPlayers = respQuery.MaxPlayers
			statusResponse.OnlinePlayers = respQuery.OnlinePlayers
			statusResponse.Players = parsePlayersQuery(respQuery.SamplePlayers)
			statusResponse.Version = respQuery.Version
		}
	}

	return statusResponse, icon, nil
}

// Sevrer status
func ServerStatus(serverInfo ServerInfo) (StausResponse, image.Image, error) {
	if serverInfo.IsBedrock {
		return BedrockServerStatus(serverInfo)
	}
	serverStatus, icon, err := JavaServerStatus(serverInfo)
	if err != nil {
		BServerStatus, Bicon, err := BedrockServerStatus(serverInfo)
		if err == nil {
			return BServerStatus, Bicon, nil
		}
	}
	return serverStatus, icon, nil
}

// -------------- Routes --------------

// Route that returns general API info
func GetRoot(w http.ResponseWriter, r *http.Request) {
	// Check the request type
	if strings.Split(r.Header.Get("Content-Type"), ",")[0] == "application/json" {
		GetServerStatus(w, r)
		return
	}

	// Read the html file
	html, err := os.ReadFile("static/mcstatus/templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Replace the server url
	htmlString := string(html)
	htmlString = strings.ReplaceAll(htmlString, "{{SERVER_URL}}", SERVER_URL)

	// Serve the html
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(htmlString))
}

// Route that returns the server icon as a PNG (base64 encoded string didn't work for some reason)
func GetIcon(w http.ResponseWriter, r *http.Request) {
	// Get the query params
	serverInfo := getServerInfo(r)
	serverInfo.EnableQuery = false

	var icon image.Image
	var status int = http.StatusOK

	// Get the server status
	if serverInfo.IsBedrock {
		// Get Bedrock icon
		icon = bedrockIcon
		status = http.StatusNoContent
	} else {
		// Get Java server status
		_, image, err := JavaServerStatus(serverInfo)

		if err == nil {
			// Set the icon as the server icon
			icon = image
		} else {
			// Set the icon as the default icon
			icon = defaultIcon
			status = http.StatusNotFound
		}
	}

	// Set the content type as image/png
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(status)
	png.Encode(w, icon)
}

// Route that returns the server status
func GetServerStatus(w http.ResponseWriter, r *http.Request) {
	// Get the query params
	serverInfo := getServerInfo(r)

	// Disable query if there is no accept header (assume embed)
	if strings.Split(r.Header.Get("Accept"), ",")[0] == "" {
		serverInfo.EnableQuery = false
	}

	var status int = http.StatusOK

	// Get the server status
	resp, _, err := ServerStatus(serverInfo)
	if err != nil {
		status = http.StatusNotFound
	}

	// Check the request type
	if strings.Split(r.Header.Get("Content-Type"), ",")[0] == "application/json" {
		// Serve the json
		w.Header().Set("Content-Type", "application/json")
		if resp.Name == "Server Offline" {
			status = http.StatusNotFound
		}
		json.NewEncoder(w).Encode(resp)
		return
	} else if strings.Split(r.Header.Get("Accept"), ",")[0] == "text/html" {
		html, err := os.ReadFile("static/mcstatus/templates/status.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		// Replace the placeholders
		htmlString := string(html)
		htmlString = strings.ReplaceAll(htmlString, "{{SERVER_URL}}", SERVER_URL)
		htmlString = strings.ReplaceAll(htmlString, "{{ADDRESS}}", resp.Connect)
		htmlString = strings.ReplaceAll(htmlString, "{{MOTD}}", resp.Name)
		htmlString = strings.ReplaceAll(htmlString, "{{FAVICON}}", resp.Favicon)
		htmlString = strings.ReplaceAll(htmlString, "{{ONLINE_PLAYERS}}", fmt.Sprint(resp.OnlinePlayers))
		htmlString = strings.ReplaceAll(htmlString, "{{MAX_PLAYERS}}", fmt.Sprint(resp.MaxPlayers))

		// Serve the html
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(htmlString))
		return
	} else {
		html, err := os.ReadFile("static/mcstatus/templates/embed.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		// Build response address string
		var queryParams []string = []string{}
		if serverInfo.IsBedrock {
			queryParams = append(queryParams, "is_bedrock=true")
		}
		if serverInfo.Port != 25565 {
			queryParams = append(queryParams, "port="+fmt.Sprint(serverInfo.Port))
		}
		if serverInfo.QueryPort != serverInfo.Port {
			queryParams = append(queryParams, "query_port="+fmt.Sprint(serverInfo.QueryPort))
		}
		if !serverInfo.EnableQuery {
			queryParams = append(queryParams, "enable_query=false")
		}

		// Format the query params
		var queryParamsString string = ""
		if len(queryParams) > 0 {
			queryParamsString = "?" + strings.Join(queryParams, "&")
		}

		// Replace the placeholders
		htmlString := string(html)
		htmlString = strings.ReplaceAll(htmlString, "{{SERVER_URL}}", SERVER_URL)
		htmlString = strings.ReplaceAll(htmlString, "{{ADDRESS}}", resp.Connect)
		htmlString = strings.ReplaceAll(htmlString, "{{ONLINE_PLAYERS}}", fmt.Sprint(resp.OnlinePlayers))
		htmlString = strings.ReplaceAll(htmlString, "{{MAX_PLAYERS}}", fmt.Sprint(resp.MaxPlayers))
		htmlString = strings.ReplaceAll(htmlString, "{{MOTD_TEXT1}}", strings.Split(resp.Name, "\\n")[0])
		var motd2String string = ""
		if len(strings.Split(resp.Name, "\\n")) > 1 {
			motd2String = strings.Split(resp.Name, "\\n")[1]
		}
		htmlString = strings.ReplaceAll(htmlString, "{{MOTD_TEXT2}}", motd2String)
		htmlString = strings.ReplaceAll(htmlString, "{{VERSION}}", resp.Version)
		htmlString = strings.ReplaceAll(htmlString, "{{ADDRESS_STR}}", serverInfo.Address+queryParamsString)
		htmlString = strings.ReplaceAll(htmlString, "{{FAVICON}}", resp.Favicon)

		// Serve the html
		w.WriteHeader(status)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(htmlString))
	}
}
