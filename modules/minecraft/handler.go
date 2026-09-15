package minecraft

import (
	"errors"
	"log"
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/goccy/go-json"
)

// GetPlayerByNameHandler - Get a player by name
func GetPlayerByNameHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			responses.BadRequest(w, r, "Invalid name")
			return
		}

		player, err := s.GetPlayerByName(name)
		if err != nil {
			if errors.Is(err, ErrPlayerNotFound) {
				responses.NotFound(w, r, "Player not found")
				return
			}
			log.Println("Failed to get player by name:\n\t", err)
			responses.InternalServerError(w, r, "Failed to get player")
			return
		}
		responses.StructOK(w, r, player)
	}
}

// GetPlayerByUUIDHandler - Get a player by UUID
func GetPlayerByUUIDHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := r.PathValue("uuid")
		if raw == "" {
			responses.BadRequest(w, r, "Invalid UUID")
			return
		}

		player, err := s.GetPlayerByUUID(raw)
		if err != nil {
			if errors.Is(err, ErrPlayerNotFound) {
				responses.NotFound(w, r, "Player not found")
				return
			}
			log.Println("Failed to get player by UUID:\n\t", err)
			responses.InternalServerError(w, r, "Failed to get player")
			return
		}
		responses.StructOK(w, r, player)
	}
}

// GetPlayersByNamesHandler - Get players by name in batch (max 10)
func GetPlayersByNamesHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var names []string
		if err := json.NewDecoder(r.Body).Decode(&names); err != nil {
			responses.BadRequest(w, r, "Invalid request body")
			return
		}

		if len(names) == 0 {
			responses.BadRequest(w, r, "No names provided")
			return
		}
		if len(names) > 10 {
			responses.BadRequest(w, r, "Batch lookup is limited to 10 names")
			return
		}

		players, err := s.GetPlayersByNames(names)
		if err != nil {
			log.Println("Failed to get players by names:\n\t", err)
			responses.InternalServerError(w, r, "Failed to get players")
			return
		}
		responses.StructOK(w, r, players)
	}
}

// GetProfileHandler - Get a player's profile from their UUID
func GetProfileHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("uuid")
		if id == "" {
			responses.BadRequest(w, r, "Invalid UUID")
			return
		}

		signed := r.URL.Query().Get("unsigned") == "false"

		player, err := s.GetProfile(id, signed)
		if err != nil {
			if errors.Is(err, ErrPlayerNotFound) {
				responses.NotFound(w, r, "Player not found")
				return
			}
			log.Println("Failed to get player profile:\n\t", err)
			responses.InternalServerError(w, r, "Failed to get player profile")
			return
		}
		responses.StructOK(w, r, player)
	}
}
