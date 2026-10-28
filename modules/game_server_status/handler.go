package gss

import (
	"net/http"
	"strconv"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/mcstatus"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

const (
	msgInvalidHost             = "Invalid host"
	msgInvalidPort             = "Invalid port"
	msgGameQQueryFailed        = "failed to query GameQ API"
	msgGameDigQueryFailed      = "failed to query GameDig API"
	msgReadBodyFailed          = "failed to read response body"
	msgDecodeBodyFailed        = "failed to decode response body"
	msgNoGameQResponse         = "no response from GameQ API"
	msgServerOffline           = "server is offline"
	msgGameUnsupported         = "this game is not supported, or the given query type doesn't support this game"
	msgJavaStatusFailed        = "failed to get java server status"
	msgBedrockStatusFailed     = "failed to get bedrock server status"
	msgQueryFailed             = "Failed to query game server"
	msgUpstreamQueryFailed     = "Failed to reach the game server query backend"
	logUnableToQueryGameServer = "[Error]: Unable to query game server:\n\t"
)

// GameServerStatusHandler - Get the game server status
func GameServerStatusHandler(s GSSService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		game := r.PathValue("game")
		host := r.URL.Query().Get("host")
		if host == "" {
			responses.BadRequest(w, r, msgInvalidHost)
			return
		}
		port, err := strconv.Atoi(r.URL.Query().Get("port"))
		if err != nil {
			responses.BadRequest(w, r, msgInvalidPort)
			return
		}

		queryType := ParseQueryType(r.URL.Query().Get("query_type"))
		status, err := s.QueryGameServer(r.Context(), game, host, port, queryType)
		if err != nil {
			reqctx.Log(r.Context(), logUnableToQueryGameServer, err.Error())
			switch err {
			case ErrServerOffline:
				responses.NotFound(w, r, msgServerOffline)
			case ErrGameUnsupported:
				responses.BadRequest(w, r, msgGameUnsupported)
			case ErrGameQQuery:
				responses.BadGateway(w, r, msgGameQQueryFailed)
			case ErrGameDigQuery:
				responses.BadGateway(w, r, msgGameDigQueryFailed)
			case ErrReadBody:
				responses.BadGateway(w, r, msgReadBodyFailed)
			case ErrDecodeBody:
				responses.BadGateway(w, r, msgDecodeBodyFailed)
			case ErrNoGameQResponse:
				responses.BadGateway(w, r, msgNoGameQResponse)
			case mcstatus.ErrJavaStatus:
				responses.BadGateway(w, r, msgJavaStatusFailed)
			case mcstatus.ErrBedrockStatus:
				responses.BadGateway(w, r, msgBedrockStatusFailed)
			default:
				responses.InternalServerError(w, r, msgQueryFailed)
			}
			return
		}
		returnRaw := r.URL.Query().Get("raw") == "true"
		if !returnRaw {
			status.Raw = nil
		}
		responses.StructOK(w, r, status)
	}
}

// SimpleGameServerStatus - Get the simple game server status
func SimpleGameServerStatus(s GSSService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		game := r.PathValue("game")
		host := r.URL.Query().Get("host")
		if host == "" {
			responses.BadRequest(w, r, msgInvalidHost)
			return
		}
		port, err := strconv.Atoi(r.URL.Query().Get("port"))
		if err != nil {
			responses.BadRequest(w, r, msgInvalidPort)
			return
		}

		plain := func(code int, text string) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(code)
			w.Write([]byte(text))
		}
		queryType := ParseQueryType(r.URL.Query().Get("query_type"))
		_, err = s.QueryGameServer(r.Context(), game, host, port, queryType)
		switch err {
		case nil:
			plain(http.StatusOK, "Online")
		case ErrServerOffline, mcstatus.ErrJavaStatus, mcstatus.ErrBedrockStatus:
			plain(http.StatusNotFound, "Offline")
		case ErrGameUnsupported:
			responses.BadRequest(w, r, msgGameUnsupported)
		case ErrGameQQuery, ErrGameDigQuery, ErrReadBody, ErrDecodeBody, ErrNoGameQResponse:
			reqctx.Log(r.Context(), "Unable to query game server:\n\t", err.Error())
			responses.BadGateway(w, r, msgUpstreamQueryFailed)
		default:
			reqctx.Log(r.Context(), "Unable to query game server:\n\t", err.Error())
			responses.InternalServerError(w, r, msgQueryFailed)
		}
	}
}
