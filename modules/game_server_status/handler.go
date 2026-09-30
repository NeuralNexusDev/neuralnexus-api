package gss

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/mcstatus"
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
	logUnableToQueryGameServer = "[Error]: Unable to query game server:\n\t"
)

type failureMapping struct {
	err     error
	respond func(http.ResponseWriter, *http.Request, string)
	msg     string
}

var queryFailures = []failureMapping{
	{ErrServerOffline, responses.NotFound, msgServerOffline},
	{ErrGameUnsupported, responses.BadRequest, msgGameUnsupported},
	{ErrGameQQuery, responses.BadGateway, msgGameQQueryFailed},
	{ErrGameDigQuery, responses.BadGateway, msgGameDigQueryFailed},
	{ErrReadBody, responses.BadGateway, msgReadBodyFailed},
	{ErrDecodeBody, responses.BadGateway, msgDecodeBodyFailed},
	{ErrNoGameQResponse, responses.BadGateway, msgNoGameQResponse},
	{mcstatus.ErrJavaStatus, responses.BadGateway, msgJavaStatusFailed},
	{mcstatus.ErrBedrockStatus, responses.BadGateway, msgBedrockStatusFailed},
}

func respondQueryFailure(w http.ResponseWriter, r *http.Request, err error) {
	log.Println(logUnableToQueryGameServer, err)
	for _, m := range queryFailures {
		if errors.Is(err, m.err) {
			m.respond(w, r, m.msg)
			return
		}
	}
	responses.InternalServerError(w, r, msgQueryFailed)
}

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
		status, err := s.QueryGameServer(game, host, port, queryType)
		if err != nil {
			respondQueryFailure(w, r, err)
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

		status := "Online"
		statusCode := http.StatusOK
		queryType := ParseQueryType(r.URL.Query().Get("query_type"))
		_, err = s.QueryGameServer(game, host, port, queryType)
		if err != nil {
			status = "Offline"
			statusCode = http.StatusNotFound
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(statusCode)
		w.Write([]byte(status))
	}
}
