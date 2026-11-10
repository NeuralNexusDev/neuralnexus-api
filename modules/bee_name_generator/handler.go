package beenamegenerator

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

const (
	msgFailedToGetBeeName                     = "Failed to get bee name"
	msgNoPermissionToUploadBeeNames           = "You do not have permission to upload bee names"
	msgFailedToUploadBeeName                  = "Failed to upload bee name"
	msgNoPermissionToDeleteBeeNames           = "You do not have permission to delete bee names"
	msgFailedToDeleteBeeName                  = "Failed to delete bee name"
	msgFailedToSubmitBeeName                  = "Failed to submit bee name"
	msgNoPermissionToGetBeeNameSuggestions    = "You do not have permission to get bee name suggestions"
	msgInvalidAmountProvided                  = "Invalid amount provided"
	msgFailedToGetBeeNameSuggestions          = "Failed to get bee name suggestions"
	msgNoPermissionToAcceptBeeNameSuggestions = "You do not have permission to accept bee name suggestions"
	msgFailedToAcceptBeeNameSuggestion        = "Failed to accept bee name suggestion"
	msgNoPermissionToRejectBeeNameSuggestions = "You do not have permission to reject bee name suggestions"
	msgFailedToRejectBeeNameSuggestion        = "Failed to reject bee name suggestion"
	msgInvalidName                            = "Invalid name"
	msgBeeNameNotFound                        = "Bee name not found"
	msgBeeNameSuggestionNotFound              = "Bee name suggestion not found"
)

func validBeeName(name string) bool {
	if name == "" || strings.Contains(name, "\x00") || !utf8.ValidString(name) {
		return false
	}
	return strings.TrimSpace(name) != ""
}

// Handler serves the bee name generator routes.
type Handler interface {
	GetBeeName(w http.ResponseWriter, r *http.Request)
	UploadBeeName(w http.ResponseWriter, r *http.Request)
	DeleteBeeName(w http.ResponseWriter, r *http.Request)
	SubmitBeeName(w http.ResponseWriter, r *http.Request)
	GetBeeNameSuggestions(w http.ResponseWriter, r *http.Request)
	AcceptBeeNameSuggestion(w http.ResponseWriter, r *http.Request)
	RejectBeeNameSuggestion(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	s BNGStore
}

// NewHandler returns a Handler that serves the bee name generator routes from the given store.
func NewHandler(s BNGStore) Handler {
	return &handler{s}
}

// GetBeeName Get a bee name
func (h *handler) GetBeeName(w http.ResponseWriter, r *http.Request) {
	beeName, err := h.s.GetBeeName(r.Context())
	if err != nil {
		reqctx.Log(r.Context(), "Failed to get bee name:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToGetBeeName)
		return
	}
	responses.StructOK(w, r, NewBeeName(beeName))
}

// UploadBeeName Upload a bee name
func (h *handler) UploadBeeName(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(reqctx.SessionKey).(*session.Session)
	if !session.HasPermission(perms.ScopeAdminBeeNameGenerator) {
		responses.Forbidden(w, r, msgNoPermissionToUploadBeeNames)
		return
	}

	beeName := r.PathValue("name")
	if !validBeeName(beeName) {
		responses.BadRequest(w, r, msgInvalidName)
		return
	}

	_, err := h.s.UploadBeeName(r.Context(), beeName)
	if err == ErrBeeNameExists {
		responses.Conflict(w, r, "That bee name already exists")
		return
	}
	if err != nil {
		reqctx.Log(r.Context(), "Failed to upload bee name:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToUploadBeeName)
		return
	}
	responses.StructOK(w, r, NewBeeName(beeName))
}

// DeleteBeeName Delete a bee name
func (h *handler) DeleteBeeName(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(reqctx.SessionKey).(*session.Session)
	if !session.HasPermission(perms.ScopeAdminBeeNameGenerator) {
		responses.Forbidden(w, r, msgNoPermissionToDeleteBeeNames)
		return
	}

	beeName := r.PathValue("name")
	if !validBeeName(beeName) {
		responses.BadRequest(w, r, msgInvalidName)
		return
	}

	_, err := h.s.DeleteBeeName(r.Context(), beeName)
	switch err {
	case nil:
		responses.NoContent(w, r)
	case ErrBeeNameNotFound:
		responses.NotFound(w, r, msgBeeNameNotFound)
	default:
		reqctx.Log(r.Context(), "Failed to delete bee name:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToDeleteBeeName)
	}
}

// SubmitBeeName Submit a bee name suggestion
func (h *handler) SubmitBeeName(w http.ResponseWriter, r *http.Request) {
	beeName := r.PathValue("name")
	if !validBeeName(beeName) {
		responses.BadRequest(w, r, msgInvalidName)
		return
	}

	_, err := h.s.SubmitBeeName(r.Context(), beeName)
	if err == ErrBeeNameSuggestionExists {
		responses.Conflict(w, r, "That bee name has already been suggested")
		return
	}
	if err != nil {
		reqctx.Log(r.Context(), "Failed to submit bee name:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToSubmitBeeName)
		return
	}
	responses.StructOK(w, r, NewBeeName(beeName))
}

// GetBeeNameSuggestions Get a list of bee name suggestions
func (h *handler) GetBeeNameSuggestions(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(reqctx.SessionKey).(*session.Session)
	if !session.HasPermission(perms.ScopeAdminBeeNameGenerator) {
		responses.Forbidden(w, r, msgNoPermissionToGetBeeNameSuggestions)
		return
	}

	amountInt, err := strconv.ParseInt(r.PathValue("amount"), 10, 64)
	if err != nil || amountInt < 1 {
		responses.BadRequest(w, r, msgInvalidAmountProvided)
		return
	}

	suggestions, err := h.s.GetBeeNameSuggestions(r.Context(), amountInt)
	if err != nil {
		reqctx.Log(r.Context(), "Failed to get bee name suggestions:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToGetBeeNameSuggestions)
		return
	}
	responses.StructOK(w, r, NewBeeNameSuggestions(suggestions))
}

// AcceptBeeNameSuggestion Accept a bee name suggestion
func (h *handler) AcceptBeeNameSuggestion(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(reqctx.SessionKey).(*session.Session)
	if !session.HasPermission(perms.ScopeAdminBeeNameGenerator) {
		responses.Forbidden(w, r, msgNoPermissionToAcceptBeeNameSuggestions)
		return
	}

	beeName := r.PathValue("name")
	if !validBeeName(beeName) {
		responses.BadRequest(w, r, msgInvalidName)
		return
	}

	_, err := h.s.AcceptBeeNameSuggestion(r.Context(), beeName)
	if err == ErrBeeNameExists {
		responses.Conflict(w, r, "That bee name already exists")
		return
	}
	if err == ErrBeeNameSuggestionNotFound {
		responses.NotFound(w, r, msgBeeNameSuggestionNotFound)
		return
	}
	if err != nil {
		reqctx.Log(r.Context(), "Failed to accept bee name suggestion:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToAcceptBeeNameSuggestion)
		return
	}
	responses.StructOK(w, r, NewBeeName(beeName))
}

// RejectBeeNameSuggestion Reject a bee name suggestion
func (h *handler) RejectBeeNameSuggestion(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(reqctx.SessionKey).(*session.Session)
	if !session.HasPermission(perms.ScopeAdminBeeNameGenerator) {
		responses.Forbidden(w, r, msgNoPermissionToRejectBeeNameSuggestions)
		return
	}

	beeName := r.PathValue("name")
	if !validBeeName(beeName) {
		responses.BadRequest(w, r, msgInvalidName)
		return
	}

	_, err := h.s.RejectBeeNameSuggestion(r.Context(), beeName)
	switch err {
	case nil:
		responses.NoContent(w, r)
	case ErrBeeNameSuggestionNotFound:
		responses.NotFound(w, r, msgBeeNameSuggestionNotFound)
	default:
		reqctx.Log(r.Context(), "Failed to reject bee name suggestion:\n\t", err.Error())
		responses.InternalServerError(w, r, msgFailedToRejectBeeNameSuggestion)
	}
}
