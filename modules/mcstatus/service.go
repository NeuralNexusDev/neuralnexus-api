package mcstatus

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ZeroErrors/go-bedrockping"
	"github.com/dreamscached/minequery/v2"
)

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

// getServerInfo - Get params
func getServerInfo(r *http.Request) ServerInfo {
	is_bedrock, err := strconv.ParseBool(r.URL.Query().Get("is_bedrock"))
	if err != nil {
		is_bedrock = false
	}
	enable_query, err := strconv.ParseBool(r.URL.Query().Get("enable_query"))
	if err != nil {
		enable_query = false
	}
	portString := r.URL.Query().Get("port")
	address := r.PathValue("address")

	if strings.Contains(address, ":") {
		var split []string = strings.Split(address, ":")
		address = split[0]
		portString = split[1]
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		if is_bedrock {
			port = 19132
		} else {
			port = 25565
		}
	}

	query_port, err := strconv.Atoi(r.URL.Query().Get("query_port"))
	if err != nil {
		query_port = port
	}

	return ServerInfo{
		Address:     address,
		Port:        port,
		EnableQuery: enable_query,
		QueryPort:   query_port,
		IsBedrock:   is_bedrock,
	}
}

// Get Bedrock server status
func (si ServerInfo) BedrockServerStatus() (StausResponse, image.Image, error) {
	// Build the connect string
	connect := si.Address + ":" + fmt.Sprint(si.Port)

	// Get the server status
	respB, err := bedrockping.Query(connect, 5*time.Second, 150*time.Millisecond)
	if err != nil {
		return NewOfflineServerStatus(true), defaultIcon, err
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
		Favicon:       NewOfflineServerStatus(true).Favicon,
		ServerType:    "bedrock",
	}, bedrockIcon, nil
}

// Get Java server status
func (si *ServerInfo) JavaServerStatus() (StausResponse, image.Image, error) {
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
		Connect:       si.Address + ":" + fmt.Sprint(si.Port),
		Version:       "",
		Favicon:       "",
		ServerType:    "java",
	}

	// Now the glorious if else chain
	var icon image.Image

	resp17, err := pinger.Ping17(si.Address, si.Port)
	if err != nil {
		resp16, err := pinger.Ping16(si.Address, si.Port)
		if err != nil {
			resp14, err := pinger.Ping14(si.Address, si.Port)
			if err != nil {
				resp15, err := pinger.PingBeta18(si.Address, si.Port)
				if err != nil {
					return NewOfflineServerStatus(false), defaultIcon, err
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

	if si.EnableQuery {
		respQuery, err := pinger.QueryFull(si.Address, si.QueryPort)
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

// Parse players from Ping17 response
func parsePlayers17(players []minequery.PlayerEntry17) []Player {
	var playerList []Player
	for _, player := range players {
		playerList = append(playerList, Player{
			Name: player.Nickname,
			ID:   player.UUID.String(),
		})
	}
	return playerList
}

// Parse players from Query response
func parsePlayersQuery(players []string) []Player {
	var playerList []Player = []Player{}
	for _, player := range players {
		playerList = append(playerList, Player{
			Name: player,
			ID:   "",
		})
	}
	return playerList
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

// Sevrer status
func (s *Service) ServerStatus(serverInfo ServerInfo) (StausResponse, image.Image, error) {
	if serverInfo.IsBedrock {
		return serverInfo.BedrockServerStatus()
	}
	serverStatus, icon, err := serverInfo.JavaServerStatus()
	if err != nil {
		BServerStatus, Bicon, err := serverInfo.BedrockServerStatus()
		if err == nil {
			return BServerStatus, Bicon, nil
		}
	}
	return serverStatus, icon, nil
}
