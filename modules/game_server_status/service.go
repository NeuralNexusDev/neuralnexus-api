package gss

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/mcstatus"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/goccy/go-json"
)

var (
	ErrGameQQuery      = errors.New("failed to query GameQ API")
	ErrGameDigQuery    = errors.New("failed to query GameDig API")
	ErrReadBody        = errors.New("failed to read response body")
	ErrDecodeBody      = errors.New("failed to decode response body")
	ErrNoGameQResponse = errors.New("no response from GameQ API")
	ErrServerOffline   = errors.New("server is offline")
	ErrGameUnsupported = errors.New("this game is not supported, or the given query type doesn't support this game")
)

// GSSService - Game Server Status service
type GSSService interface {
	QueryGameQ(ctx context.Context, game string, host string, port int) (*GameQResponse, error)
	QueryGameDig(ctx context.Context, game string, host string, port int) (*GameDigResponse, error)
	QueryGameServer(ctx context.Context, game string, host string, port int, queryType QueryType) (*GameServerStatus, error)
}

// service - Game Server Status service implementation
type service struct {
	cfg      config.GameServerStatus
	mcStatus mcstatus.MCStatusService
}

// NewService - Create new Game Server Status service
func NewService(cfg config.GameServerStatus, mcStatus mcstatus.MCStatusService) GSSService {
	return &service{cfg: cfg, mcStatus: mcStatus}
}

// QueryGameQ - Query GameQ REST API
func (s *service) QueryGameQ(ctx context.Context, game string, host string, port int) (*GameQResponse, error) {
	var response map[string]GameQResponse
	q := url.Values{}
	q.Set("host", host)
	q.Set("port", strconv.Itoa(port))
	reqURL := s.cfg.GameQEndpoint + "/GssGameq.php/" + url.PathEscape(game) + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		reqctx.Log(ctx, err.Error())
		return nil, ErrGameQQuery
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		reqctx.Log(ctx, err.Error())
		return nil, ErrGameQQuery
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			reqctx.Log(ctx, err.Error())
			return nil, ErrReadBody
		}
		reqctx.Log(ctx, string(body))
		return nil, ErrGameQQuery
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		reqctx.Log(ctx, err.Error())
		return nil, ErrDecodeBody
	}

	for _, v := range response {
		return &v, nil
	}
	return nil, ErrNoGameQResponse
}

// QueryGameDig - Query GameDig REST API
func (s *service) QueryGameDig(ctx context.Context, game string, host string, port int) (*GameDigResponse, error) {
	var response GameDigResponse
	q := url.Values{}
	q.Set("host", host)
	q.Set("port", strconv.Itoa(port))
	reqURL := s.cfg.GameDigEndpoint + "/" + url.PathEscape(game) + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		reqctx.Log(ctx, err.Error())
		return nil, ErrGameDigQuery
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		reqctx.Log(ctx, err.Error())
		return nil, ErrGameDigQuery
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			reqctx.Log(ctx, err.Error())
			return nil, ErrReadBody
		}
		reqctx.Log(ctx, string(body))
		return nil, ErrGameDigQuery
	}

	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		reqctx.Log(ctx, err.Error())
		return nil, ErrDecodeBody
	}

	return &response, nil
}

// DetermineOrVerifyQueryType - Determine the query type or check if the query type is valid for the game
func DetermineOrVerifyQueryType(game string, queryType QueryType) (QueryType, bool) {
	for _, v := range MinecraftList {
		if v == game {
			switch queryType {
			case QueryTypeUnknown:
				fallthrough
			case QueryTypeMinecraft:
				return QueryTypeMinecraft, true
			}
			break
		}
	}
	for _, v := range GameQList {
		if v == game {
			switch queryType {
			case QueryTypeUnknown:
				fallthrough
			case QueryTypeGameQ:
				return QueryTypeGameQ, true
			}
			break
		}
	}
	for _, v := range GameDigList {
		if v == game {
			switch queryType {
			case QueryTypeUnknown:
				fallthrough
			case QueryTypeGameDig:
				return QueryTypeGameDig, true
			}
			break
		}
	}
	return QueryTypeUnknown, false
}

// QueryGameServer - Query game server status
func (s *service) QueryGameServer(ctx context.Context, game string, host string, port int, queryType QueryType) (*GameServerStatus, error) {
	queryType, valid := DetermineOrVerifyQueryType(game, queryType)
	if !valid {
		return nil, ErrGameUnsupported
	}
	switch queryType {
	case QueryTypeMinecraft:
		isBedrock := game != "minecraft"
		response, err := s.mcStatus.GetServerStatus(ctx, host, port, isBedrock, true, port)
		if err != nil {
			return nil, err
		}
		return (*mcServerStatus)(response).Normalize(), nil
	case QueryTypeGameQ:
		response, err := s.QueryGameQ(ctx, game, host, port)
		if err != nil {
			return nil, err
		}
		if !response.Online {
			return nil, ErrServerOffline
		}
		return response.Normalize(), nil
	case QueryTypeGameDig:
		response, err := s.QueryGameDig(ctx, game, host, port)
		if err != nil {
			return nil, err
		}
		return response.Normalize(), nil
	default:
		return nil, ErrGameUnsupported
	}
}
