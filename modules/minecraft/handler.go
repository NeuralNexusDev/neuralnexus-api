package minecraft

import (
	"io"
	"net/http"
	"strconv"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/goccy/go-json"
	"github.com/google/uuid"
)

const (
	msgInvalidName               = "Invalid name"
	msgFailedToGetPlayer         = "Failed to get player"
	msgNotAValidUUID             = "Not a valid UUID: "
	msgFailedToGetPlayerProfile  = "Failed to get player profile"
	msgInvalidGamertag           = "Invalid gamertag"
	msgInvalidXuid               = "Invalid xuid"
	msgFailedToGetGeyserProfile  = "Failed to get Geyser profile"
	msgInvalidHash               = "Invalid hash"
	msgTextureNotFound           = "Texture not found"
	msgPlayerNotFound            = "player not found"
	msgFailedToGetTexture        = "Failed to get texture"
	msgRequestMustBeJSON         = "Request must be of type application/json"
	msgInvalidRequestBody        = "Invalid request body"
	msgInvalidSize               = "size must be between 1 and 10"
	msgInvalidProfileName        = "Invalid profile name"
	msgFailedToGetPlayers        = "Failed to get players"
	msgFailedToGetGeyserXUID     = "Failed to get Geyser XUID"
	msgFailedToGetGeyserGamertag = "Failed to get Geyser Gamertag"
	msgFailedToGetGeyserSkin     = "Failed to get Geyser skin"
	msgNotAValidBedrockUUID      = "Not a valid Bedrock UUID: "
	logFailedToGetPlayerProfile  = "Failed to get player profile:\n\t"
)

type Handler interface {
	GetMojangPlayerByName(w http.ResponseWriter, r *http.Request)
	GetMojangPlayerByUUID(w http.ResponseWriter, r *http.Request)
	GetMojangPlayersByNames(w http.ResponseWriter, r *http.Request)
	GetMojangProfile(w http.ResponseWriter, r *http.Request)

	GetProfile(w http.ResponseWriter, r *http.Request)
	GetProfileByName(w http.ResponseWriter, r *http.Request)

	GetGeyserPlayerByXUID(w http.ResponseWriter, r *http.Request)
	GetGeyserPlayerByGamertag(w http.ResponseWriter, r *http.Request)
	GetGeyserSkin(w http.ResponseWriter, r *http.Request)

	GetGeyserProfile(w http.ResponseWriter, r *http.Request)
	GetGeyserProfileByXUID(w http.ResponseWriter, r *http.Request)
	GetGeyserProfileByName(w http.ResponseWriter, r *http.Request)

	GetTexture(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	s Service
}

func NewHandler(mcs Service) Handler {
	return &handler{mcs}
}

// GetMojangPlayerByName - Get a player by name
func (h *handler) GetMojangPlayerByName(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		responses.BadRequest(w, r, msgInvalidName)
		return
	}

	player, err := h.s.Mojang().GetMojangPlayerByName(r.Context(), name)
	switch err {
	case ErrPlayerNotFound:
		responses.NotFound(w, r, msgPlayerNotFound)
	case nil:
		responses.StructOK(w, r, player)
	default:
		reqctx.Log(r.Context(), "Failed to get player by name:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToGetPlayer)
	}
}

// GetMojangPlayerByUUID - Get a player by UUID
func (h *handler) GetMojangPlayerByUUID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("uuid")
	if _, err := uuid.Parse(id); err != nil {
		responses.BadRequest(w, r, msgNotAValidUUID+id)
		return
	}

	player, err := h.s.Mojang().GetMojangPlayerByUUID(r.Context(), id)
	switch err {
	case ErrPlayerNotFound:
		responses.NotFound(w, r, msgPlayerNotFound)
	case nil:
		responses.StructOK(w, r, player)
	default:
		reqctx.Log(r.Context(), "Failed to get player by UUID:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToGetPlayer)
	}
}

// GetMojangPlayersByNames - Get players by name in batch (max 10)
func (h *handler) GetMojangPlayersByNames(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "application/json" {
		responses.UnsupportedMediaType(w, r, msgRequestMustBeJSON)
		return
	}

	var names []string
	if err := json.NewDecoder(r.Body).Decode(&names); err != nil {
		responses.BadRequest(w, r, msgInvalidRequestBody)
		return
	}

	if len(names) == 0 || len(names) > 10 {
		responses.BadRequest(w, r, msgInvalidSize)
		return
	}

	for _, name := range names {
		if name == "" {
			responses.BadRequest(w, r, msgInvalidProfileName)
			return
		}
	}

	players, err := h.s.Mojang().GetMojangPlayersByNames(r.Context(), names)
	if err != nil {
		reqctx.Log(r.Context(), "Failed to get players by names:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToGetPlayers)
		return
	}
	responses.StructOK(w, r, players)
}

// GetMojangProfile - Get a player's full profile from their UUID
func (h *handler) GetMojangProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("uuid")
	if _, err := uuid.Parse(id); err != nil {
		responses.BadRequest(w, r, msgNotAValidUUID+id)
		return
	}

	signed := r.URL.Query().Get("unsigned") == "false"

	player, err := h.s.Mojang().GetMojangProfile(r.Context(), id, signed)
	switch err {
	case ErrPlayerNotFound:
		responses.NoContent(w, r)
	case nil:
		responses.StructOK(w, r, player)
	default:
		reqctx.Log(r.Context(), logFailedToGetPlayerProfile, err.Error())
		responses.InternalServerError(w, r, msgFailedToGetPlayerProfile)
	}
}

// GetProfile - Get a player's profile with textures decoded
func (h *handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("uuid")
	if _, err := uuid.Parse(id); err != nil {
		responses.BadRequest(w, r, msgNotAValidUUID+id)
		return
	}

	profile, err := h.s.Mojang().GetProfile(r.Context(), id)
	switch err {
	case ErrPlayerNotFound:
		responses.NoContent(w, r)
	case nil:
		responses.StructOK(w, r, profile)
	default:
		reqctx.Log(r.Context(), logFailedToGetPlayerProfile, err.Error())
		responses.InternalServerError(w, r, msgFailedToGetPlayerProfile)
	}
}

// GetProfileByName - Get a player's profile by name
func (h *handler) GetProfileByName(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		responses.BadRequest(w, r, msgInvalidName)
		return
	}

	profile, err := h.s.Mojang().GetProfileByName(r.Context(), name)
	switch err {
	case ErrPlayerNotFound:
		responses.NoContent(w, r)
	case nil:
		responses.StructOK(w, r, profile)
	default:
		reqctx.Log(r.Context(), "Failed to get player profile by name:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToGetPlayerProfile)
	}
}

// GetGeyserPlayerByXUID - Look up a Bedrock player by xuid
func (h *handler) GetGeyserPlayerByXUID(w http.ResponseWriter, r *http.Request) {
	xuidStr := r.PathValue("xuid")
	var xuid int64
	if id, err := uuidToXUID(xuidStr); err == nil {
		xuid = id
	} else {
		id, err := strconv.ParseInt(xuidStr, 10, 64)
		xuid = id
		if err != nil {
			responses.BadRequest(w, r, msgInvalidXuid)
			return
		}
	}

	player, err := h.s.Geyser().GetGeyserPlayerByXUID(r.Context(), xuid)
	switch err {
	case ErrPlayerNotFound:
		responses.NotFound(w, r, msgPlayerNotFound)
	case ErrInvalidGeyserRequest:
		responses.BadRequest(w, r, msgInvalidGamertag)
	case nil:
		responses.StructOK(w, r, player)
	default:
		reqctx.Log(r.Context(), "Failed to get Geyser Gamertag:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToGetGeyserGamertag)
	}
}

// GetGeyserPlayerByGamertag - Look up a Bedrock player by gamertag
func (h *handler) GetGeyserPlayerByGamertag(w http.ResponseWriter, r *http.Request) {
	gamertag := r.PathValue("gamertag")
	if gamertag == "" {
		responses.BadRequest(w, r, msgInvalidGamertag)
		return
	}

	player, err := h.s.Geyser().GetGeyserPlayerByGamertag(r.Context(), gamertag)
	switch err {
	case ErrPlayerNotFound:
		responses.NotFound(w, r, msgPlayerNotFound)
	case ErrInvalidGeyserRequest:
		responses.BadRequest(w, r, msgInvalidGamertag)
	case nil:
		responses.StructOK(w, r, player)
	default:
		reqctx.Log(r.Context(), "Failed to get Geyser XUID:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToGetGeyserXUID)
	}
}

// GetGeyserSkin - Get a Bedrock player's skin by XUID
func (h *handler) GetGeyserSkin(w http.ResponseWriter, r *http.Request) {
	xuid, err := strconv.ParseInt(r.PathValue("xuid"), 10, 64)
	if err != nil {
		responses.BadRequest(w, r, msgInvalidXuid)
		return
	}

	skin, err := h.s.Geyser().GetGeyserSkin(r.Context(), xuid)
	switch err {
	case ErrSkinNotFound:
		responses.NoContent(w, r)
	case ErrInvalidGeyserRequest:
		responses.BadRequest(w, r, msgInvalidXuid)
	case nil:
		responses.StructOK(w, r, skin)
	default:
		reqctx.Log(r.Context(), "Failed to get Geyser skin:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToGetGeyserSkin)
	}
}

// GetGeyserProfile - Get a Bedrock player's full profile by UUID
func (h *handler) GetGeyserProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("uuid")
	if _, err := uuidToXUID(id); err != nil {
		responses.BadRequest(w, r, msgNotAValidBedrockUUID+id)
		return
	}

	profile, err := h.s.Geyser().GetGeyserProfile(r.Context(), id)
	switch err {
	case ErrPlayerNotFound:
		responses.NotFound(w, r, msgPlayerNotFound)
	case ErrInvalidGeyserRequest:
		responses.BadRequest(w, r, msgInvalidXuid)
	case nil:
		responses.StructOK(w, r, profile)
	default:
		reqctx.Log(r.Context(), "Failed to get Geyser profile:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToGetGeyserProfile)
	}
}

// GetGeyserProfileByXUID - Get a Bedrock player's full profile by gamertag
func (h *handler) GetGeyserProfileByXUID(w http.ResponseWriter, r *http.Request) {
	xuidStr := r.PathValue("xuid")
	var xuid int64
	if id, err := uuidToXUID(xuidStr); err == nil {
		xuid = id
	} else {
		id, err := strconv.ParseInt(xuidStr, 10, 64)
		xuid = id
		if err != nil {
			responses.BadRequest(w, r, msgInvalidXuid)
			return
		}
	}

	profile, err := h.s.Geyser().GetGeyserProfileByXUID(r.Context(), xuid)
	switch err {
	case ErrPlayerNotFound:
		responses.NotFound(w, r, msgPlayerNotFound)
	case ErrInvalidGeyserRequest:
		responses.BadRequest(w, r, msgInvalidXuid)
	case nil:
		responses.StructOK(w, r, profile)
	default:
		reqctx.Log(r.Context(), "Failed to get Geyser profile by XUID:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToGetGeyserProfile)
	}
}

// GetGeyserProfileByName - Get a Bedrock player's full profile by gamertag
func (h *handler) GetGeyserProfileByName(w http.ResponseWriter, r *http.Request) {
	gamertag := r.PathValue("name")
	if gamertag == "" {
		responses.BadRequest(w, r, msgInvalidGamertag)
		return
	}

	profile, err := h.s.Geyser().GetGeyserProfileByName(r.Context(), gamertag)
	switch err {
	case ErrPlayerNotFound:
		responses.NotFound(w, r, msgPlayerNotFound)
	case ErrInvalidGeyserRequest:
		responses.BadRequest(w, r, msgInvalidGamertag)
	case nil:
		responses.StructOK(w, r, profile)
	default:
		reqctx.Log(r.Context(), "Failed to get Geyser profile by name:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToGetGeyserProfile)
	}
}

// GetTexture - Serve a texture's bytes, fetching from the backend exactly once
func (h *handler) GetTexture(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	if hash == "" {
		responses.BadRequest(w, r, msgInvalidHash)
		return
	}

	result, err := h.s.Texture().GetTextureContent(r.Context(), hash)
	if err != nil {
		if err == ErrTextureNotFound {
			responses.NotFound(w, r, msgTextureNotFound)
			return
		}
		responses.BadGateway(w, r, msgFailedToGetTexture)
		reqctx.Log(r.Context(), "Failed to get texture:\n\t", err.Error())
		return
	}
	defer result.Body.Close()

	w.Header().Set("Content-Type", result.ContentType)
	_, _ = io.Copy(w, result.Body)
}
