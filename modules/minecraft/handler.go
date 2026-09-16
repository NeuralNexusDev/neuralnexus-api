package minecraft

import (
	"errors"
	"io"
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
		id := r.PathValue("uuid")
		if _, err := uuid.Parse(id); err != nil {
			responses.BadRequest(w, r, "Not a valid UUID: "+id)
			return
		}

		player, err := s.GetPlayerByUUID(id)
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
		if _, err := uuid.Parse(id); err != nil {
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

// GetTextureHandler - Pass-through the texture URL to the S3 bucket
func GetTextureHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hash := r.PathValue("hash")
		if hash == "" {
			responses.BadRequest(w, r, "Invalid hash")
			return
		}

		targetURL, err := s.GetTexture(hash, false)
		if err != nil {
			responses.BadGateway(w, r, "Failed to get texture")
			log.Println("Failed to get texture:\n\t", err)
			return
		}

		// Fetch the file via standard HTTP client
		// TODO: Consider if this needs to be replaced for testing
		resp, err := http.Get(targetURL)
		if err != nil {
			responses.BadGateway(w, r, "Failed to reach storage backend")
			log.Println("Failed to reach storage backend:\n\t", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			responses.NotFound(w, r, "Texture not found")
			return
		}

		// Forward the Content-Type from S3 (or default to image/png)
		if ct := resp.Header.Get("Content-Type"); ct != "" {
			w.Header().Set("Content-Type", ct)
		} else {
			w.Header().Set("Content-Type", "image/png")
		}

		// Pass through status code and stream the bytes
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}
