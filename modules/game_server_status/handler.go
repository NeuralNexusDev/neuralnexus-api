package gss

import (
	"net/http"
	"strconv"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	service := NewService()
	mux.HandleFunc("/api/v1/game-server-status/{game}", GetGameServerStatusHandler(service))
	return mux
}

// GetGameServerStatusHandler - Get the game server status
func GetGameServerStatusHandler(s GSSService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		game := r.PathValue("game")
		host := r.URL.Query().Get("host")
		if host == "" {
			responses.SendAndEncodeBadRequest(w, r, "Invalid host")
			return
		}
		port, err := strconv.Atoi(r.URL.Query().Get("port"))
		if err != nil {
			responses.SendAndEncodeBadRequest(w, r, "Invalid port")
			return
		}

		returnRaw := r.URL.Query().Get("raw") == "true"
		status, err := s.QueryGameServer(game, host, port)
		if err != nil {
			responses.SendAndEncodeInternalServerError(w, r, err.Error())
			return
		}
		if !returnRaw {
			status.Raw = nil
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, status)
	}
}
