package gss

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Service - Game Server Status service
type Service struct{}

// NewService - Create new Game Server Status service
func NewService() *Service {
	return &Service{}
}

// NormalizeGameQResponse - Normalize GameQ response
func (s *Service) NormalizeGameQResponse(response *GameQResponse) *GameServerStatus {
	return &GameServerStatus{
		HostName:   response.HostName,
		MapName:    response.MapName,
		MaxPlayers: response.MaxPlayers,
		NumPlayers: response.NumPlayers,
		Players:    response.Players,
	}
}

// NormalizeGameDigResponse - Normalize GameDig response
func (s *Service) NormalizeGameDigResponse(response *GameDigResponse) *GameServerStatus {
	players := make([]string, 0, len(response.Players))
	for _, player := range response.Players {
		players = append(players, player.Name)
	}

	return &GameServerStatus{
		HostName:   response.Name,
		MapName:    response.Map,
		MaxPlayers: response.MaxPlayers,
		NumPlayers: response.NumPlayers,
		Players:    players,
	}
}

// QueryGameQ - Query GameQ REST API
func (s *Service) QueryGameQ(game string, host string, port int) (*GameQResponse, error) {
	var response map[string]GameQResponse

	// Query GameQ REST API
	url := fmt.Sprintf("http://0.0.0.0:3024/GssGameq.php/%s?host=%s&port=%d", game, host, port)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(string(body))
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	for _, v := range response {
		return &v, nil
	}
	return nil, errors.New("no response")
}

// QueryGameDig - Query GameDig REST API
func (s *Service) QueryGameDig(game string, host string, port int) (*GameDigResponse, error) {
	var response GameDigResponse

	// Query GameDig REST API
	url := fmt.Sprintf("http://0.0.0.0:3025/%s?host=%s&port=%d", game, host, port)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, errors.New(string(body))
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}
