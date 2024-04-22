package gss

import (
	"net/http"
	"strconv"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	service := NewService()
	mux.HandleFunc("/api/v1/game-server-status/{game}", GameServerStatusHandler(service))
	mux.HandleFunc("/api/v1/game-server-status/simple/{game}", SimpleGameServerStatus(service))
	return mux
}

// GameServerStatusHandler - Get the game server status
func GameServerStatusHandler(s GSSService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
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

		var status *GameServerStatus
		query_type := r.URL.Query().Get("query_type")
		if query_type != "" {
			switch query_type {
			case "gameq":
				query, err := s.QueryGameQ(game, host, port)
				if err != nil {
					status = query.Normalize()
				}
			case "gamedig":
				query, err := s.QueryGameDig(game, host, port)
				if err != nil {
					status = query.Normalize()
				}
			}
		}

		if status == nil {
			status, err = s.QueryGameServer(game, host, port)
		}
		if err != nil {
			responses.SendAndEncodeNotFound(w, r, err.Error())
			return
		}
		returnRaw := r.URL.Query().Get("raw") == "true"
		if !returnRaw {
			status.Raw = nil
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, status)
	}
}

// SimpleGameServerStatus - Get the simple game server status
func SimpleGameServerStatus(s GSSService) http.HandlerFunc {
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

		status := "Online"
		statusCode := http.StatusOK
		_, err = s.QueryGameServer(game, host, port)
		if err != nil {
			status = "Offline"
			statusCode = http.StatusNotFound
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(statusCode)
		w.Write([]byte(status))
	}
}
