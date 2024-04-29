package gss

import (
	"net/http"
	"strconv"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	service := NewService()
	mux.HandleFunc("GET /api/v1/game-server-status/{game}", GameServerStatusHandler(service))
	mux.HandleFunc("GET /api/v1/game-server-status/simple/{game}", SimpleGameServerStatus(service))
	return mux
}

// GameServerStatusHandler - Get the game server status
func GameServerStatusHandler(s GSSService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		game := r.PathValue("game")
		host := r.URL.Query().Get("host")
		if host == "" {
			responses.BadRequest(w, r, "Invalid host")
			return
		}
		port, err := strconv.Atoi(r.URL.Query().Get("port"))
		if err != nil {
			responses.BadRequest(w, r, "Invalid port")
			return
		}

		queryType := ParseQueryType(r.URL.Query().Get("query_type"))
		status, err := s.QueryGameServer(game, host, port, queryType)
		if err != nil {
			responses.NotFound(w, r, err.Error())
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
			responses.BadRequest(w, r, "Invalid host")
			return
		}
		port, err := strconv.Atoi(r.URL.Query().Get("port"))
		if err != nil {
			responses.BadRequest(w, r, "Invalid port")
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
