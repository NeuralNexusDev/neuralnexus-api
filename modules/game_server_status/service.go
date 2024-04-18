package gss

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/mcstatus"
)

// GSSService - Game Server Status service
type GSSService interface {
	QueryGameQ(game string, host string, port int) (*GameQResponse, error)
	QueryGameDig(game string, host string, port int) (*GameDigResponse, error)
	QueryGameServer(game string, host string, port int) (*GameServerStatus, error)
}

// service - Game Server Status service implementation
type service struct{}

// NewService - Create new Game Server Status service
func NewService() *service {
	return &service{}
}

// QueryGameQ - Query GameQ REST API
func (s *service) QueryGameQ(game string, host string, port int) (*GameQResponse, error) {
	var response map[string]GameQResponse
	url := fmt.Sprintf("http://172.16.1.180:3024/GssGameq.php/%s?host=%s&port=%d", game, host, port)
	resp, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return nil, errors.New("failed to query GameQ API")
	}
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
			return nil, errors.New("failed to read response body")
		}
		log.Println(string(body))
		return nil, errors.New("failed to query GameQ API")
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Println(err)
		return nil, errors.New("failed to decode response body")
	}

	for _, v := range response {
		return &v, nil
	}
	return nil, errors.New("no response from GameQ API")
}

// QueryGameDig - Query GameDig REST API
func (s *service) QueryGameDig(game string, host string, port int) (*GameDigResponse, error) {
	var response GameDigResponse
	url := fmt.Sprintf("http://172.16.1.180:3025/%s?host=%s&port=%d", game, host, port)
	resp, err := http.Get(url)
	if err != nil {
		log.Println(err)
		return nil, errors.New("failed to query GameDig API")
	}
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
			return nil, errors.New("failed to read response body")
		}
		log.Println(string(body))
		return nil, errors.New("failed to query GameDig API")
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		log.Println(err)
		return nil, errors.New("failed to decode response body")
	}

	return &response, nil
}

// QueryGameServer - Query game server status
func (s *service) QueryGameServer(game string, host string, port int) (*GameServerStatus, error) {
	for _, v := range MinecraftList {
		if v == game {
			isBedrock := game != "minecraft"
			response, err := mcstatus.NewService().GetServerStatus(host, port, isBedrock, true)
			if err != nil {
				return nil, err
			}
			return (*mcstatusResponse)(response).Normalize(), nil
		}
	}
	for _, v := range GameQList {
		if v == game {
			response, err := s.QueryGameQ(game, host, port)
			if err != nil {
				return nil, err
			}
			if !response.Online {
				return nil, errors.New("server is offline")
			}
			return response.Normalize(), nil
		}
	}
	for _, v := range GameDigList {
		if v == game {
			response, err := s.QueryGameDig(game, host, port)
			if err != nil {
				return nil, err
			}
			return response.Normalize(), nil
		}
	}
	return nil, errors.New("game not supported")
}
