package minecraft

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
)

// GetMojangPlayerByNameHandler - Get a player by name
func GetMojangPlayerByNameHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			responses.BadRequest(w, r, "Invalid name")
			return
		}

		player, err := s.GetMojangPlayerByName(name)
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

// GetMojangPlayerByUUIDHandler - Get a player by UUID
func GetMojangPlayerByUUIDHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("uuid")
		if _, err := uuid.Parse(id); err != nil {
			responses.BadRequest(w, r, "Not a valid UUID: "+id)
			return
		}

		player, err := s.GetMojangPlayerByUUID(id)
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

// GetMojangPlayersByNamesHandler - Get players by name in batch (max 10)
func GetMojangPlayersByNamesHandler(s Service) http.HandlerFunc {
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

		players, err := s.GetMojangPlayersByNames(names)
		if err != nil {
			log.Println("Failed to get players by names:\n\t", err)
			responses.InternalServerError(w, r, "Failed to get players")
			return
		}
		responses.StructOK(w, r, players)
	}
}

// GetMojangProfileHandler - Get a player's full profile from their UUID
func GetMojangProfileHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("uuid")
		if _, err := uuid.Parse(id); err != nil {
			responses.BadRequest(w, r, "Not a valid UUID: "+id)
			return
		}

		signed := r.URL.Query().Get("unsigned") == "false"

		player, err := s.GetMojangProfile(id, signed)
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

// GetProfileHandler - Get a player's profile with textures decoded
func GetProfileHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("uuid")
		if _, err := uuid.Parse(id); err != nil {
			responses.BadRequest(w, r, "Not a valid UUID: "+id)
			return
		}

		profile, err := s.GetProfile(id)
		if err != nil {
			if errors.Is(err, ErrPlayerNotFound) {
				responses.NoContent(w, r)
				return
			}
			log.Println("Failed to get player profile:\n\t", err)
			responses.InternalServerError(w, r, "Failed to get player profile")
			return
		}
		responses.StructOK(w, r, profile)
	}
}

// GetGeyserXUIDHandler - Look up a Bedrock player's XUID and derived UUID by gamertag
func GetGeyserXUIDHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		gamertag := r.PathValue("gamertag")
		if gamertag == "" {
			responses.BadRequest(w, r, "Invalid gamertag")
			return
		}

		player, err := s.GetGeyserXUID(gamertag)
		if err != nil {
			if errors.Is(err, ErrPlayerNotFound) {
				responses.NotFound(w, r, ErrPlayerNotFound.Error())
				return
			}
			if errors.Is(err, ErrInvalidGeyserRequest) {
				responses.BadRequest(w, r, "Invalid gamertag")
				return
			}
			log.Println("Failed to get Geyser XUID:\n\t", err)
			responses.InternalServerError(w, r, "Failed to get Geyser XUID")
			return
		}
		responses.StructOK(w, r, player)
	}
}

// GetGeyserSkinHandler - Get a Bedrock player's most recently converted skin by XUID
func GetGeyserSkinHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		xuid, err := strconv.ParseInt(r.PathValue("xuid"), 10, 64)
		if err != nil {
			responses.BadRequest(w, r, "Invalid xuid")
			return
		}

		skin, err := s.GetGeyserSkin(xuid)
		if err != nil {
			if errors.Is(err, ErrSkinNotFound) {
				responses.NoContent(w, r)
				return
			}
			if errors.Is(err, ErrInvalidGeyserRequest) {
				responses.BadRequest(w, r, "Invalid xuid")
				return
			}
			log.Println("Failed to get Geyser skin:\n\t", err)
			responses.InternalServerError(w, r, "Failed to get Geyser skin")
			return
		}
		responses.StructOK(w, r, skin)
	}
}

// GetTextureHandler - Serve a texture's bytes, fetching from the backend exactly once
func GetTextureHandler(s Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hash := r.PathValue("hash")
		if hash == "" {
			responses.BadRequest(w, r, "Invalid hash")
			return
		}

		result, err := s.GetTextureContent(hash)
		if err != nil {
			if errors.Is(err, ErrTextureNotFound) {
				responses.NotFound(w, r, "Texture not found")
				return
			}
			responses.BadGateway(w, r, "Failed to get texture")
			log.Println("Failed to get texture:\n\t", err)
			return
		}
		defer result.Body.Close()

		w.Header().Set("Content-Type", result.ContentType)
		_, _ = io.Copy(w, result.Body)
	}
}
