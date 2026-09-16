package minecraft

import (
	"errors"
	"log"
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
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
				responses.NotFound(w, r, ErrPlayerNotFound.Error())
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

		// TODO: UUID validation
		player, err := s.GetPlayerByUUID(raw)
		if err != nil {
			if errors.Is(err, ErrPlayerNotFound) {
				responses.NotFound(w, r, ErrPlayerNotFound.Error())
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
		if r.Header.Get("Content-Type") != "application/json" {
			responses.UnsupportedMediaType(w, r, "Request must be of type application/json")
			return
		}

		var names []string
		if err := json.NewDecoder(r.Body).Decode(&names); err != nil {
			responses.BadRequest(w, r, "Invalid request body")
			return
		}

		if len(names) == 0 || len(names) > 10 {
			responses.BadRequest(w, r, "size must be between 1 and 10")
			return
		}

		for _, name := range names {
			if name == "" {
				responses.BadRequest(w, r, "Invalid profile name")
				return
			}
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
		_, err := uuid.Parse(id)
		if err != nil {
			responses.BadRequest(w, r, "Not a valid UUID: "+id)
			return
		}

		signed := r.URL.Query().Get("unsigned") == "false"

		player, err := s.GetProfile(id, signed)
		if err != nil {
			if errors.Is(err, ErrPlayerNotFound) {
				responses.NoContent(w, r)
				return
			}
			log.Println("Failed to get player profile:\n\t", err)
			responses.InternalServerError(w, r, "Failed to get player profile")
			return
		}
		responses.StructOK(w, r, player)
	}
}
