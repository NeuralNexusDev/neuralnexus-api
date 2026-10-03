package beenamegenerator

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
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
)

// GetBeeNameHandler Get a bee name
func GetBeeNameHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		beeName, err := s.GetBeeName()
		if err != nil {
			log.Println("Failed to get bee name:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToGetBeeName)
			return
		}
		responses.StructOK(w, r, NewBeeName(beeName))
	}
}

// UploadBeeNameHandler Upload a bee name
func UploadBeeNameHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminBeeNameGenerator) {
			responses.Forbidden(w, r, msgNoPermissionToUploadBeeNames)
			return
		}

		beeName := r.PathValue("name")
		if beeName == "" {
			responses.BadRequest(w, r, msgInvalidName)
			return
		}

		_, err := s.UploadBeeName(beeName)
		if errors.Is(err, ErrBeeNameExists) {
			responses.Conflict(w, r, "That bee name already exists")
			return
		}
		if err != nil {
			log.Println("Failed to upload bee name:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToUploadBeeName)
			return
		}
		responses.StructOK(w, r, NewBeeName(beeName))
	}
}

// DeleteBeeNameHandler Delete a bee name
func DeleteBeeNameHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminBeeNameGenerator) {
			responses.Forbidden(w, r, msgNoPermissionToDeleteBeeNames)
			return
		}

		beeName := r.PathValue("name")
		if beeName == "" {
			responses.BadRequest(w, r, msgInvalidName)
			return
		}

		_, err := s.DeleteBeeName(beeName)
		if err != nil {
			log.Println("Failed to delete bee name:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToDeleteBeeName)
			return
		}
		responses.NoContent(w, r)
	}
}

// SubmitBeeNameHandler Submit a bee name suggestion
func SubmitBeeNameHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		beeName := r.PathValue("name")
		if beeName == "" {
			responses.BadRequest(w, r, msgInvalidName)
			return
		}

		_, err := s.SubmitBeeName(beeName)
		if errors.Is(err, ErrBeeNameSuggestionExists) {
			responses.Conflict(w, r, "That bee name has already been suggested")
			return
		}
		if err != nil {
			log.Println("Failed to submit bee name:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToSubmitBeeName)
			return
		}
		responses.StructOK(w, r, NewBeeName(beeName))
	}
}

// GetBeeNameSuggestionsHandler Get a list of bee name suggestions
func GetBeeNameSuggestionsHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminBeeNameGenerator) {
			responses.Forbidden(w, r, msgNoPermissionToGetBeeNameSuggestions)
			return
		}

		amount := r.PathValue("amount")
		if amount == "" || amount == "0" {
			amount = "NAN"
		}
		amountInt, err := strconv.ParseInt(amount, 10, 64)
		if err != nil {
			responses.BadRequest(w, r, msgInvalidAmountProvided)
			return
		}

		suggestions, err := s.GetBeeNameSuggestions(amountInt)
		if err != nil {
			log.Println("Failed to get bee name suggestions:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToGetBeeNameSuggestions)
			return
		}
		responses.StructOK(w, r, NewBeeNameSuggestions(suggestions))
	}
}

// AcceptBeeNameSuggestionHandler Accept a bee name suggestion
func AcceptBeeNameSuggestionHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminBeeNameGenerator) {
			responses.Forbidden(w, r, msgNoPermissionToAcceptBeeNameSuggestions)
			return
		}

		beeName := r.PathValue("name")
		if beeName == "" {
			responses.BadRequest(w, r, msgInvalidName)
			return
		}

		_, err := s.AcceptBeeNameSuggestion(beeName)
		if errors.Is(err, ErrBeeNameExists) {
			responses.Conflict(w, r, "That bee name already exists")
			return
		}
		if err != nil {
			log.Println("Failed to accept bee name suggestion:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToAcceptBeeNameSuggestion)
			return
		}
		responses.StructOK(w, r, NewBeeName(beeName))
	}
}

// RejectBeeNameSuggestionHandler Reject a bee name suggestion
func RejectBeeNameSuggestionHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminBeeNameGenerator) {
			responses.Forbidden(w, r, msgNoPermissionToRejectBeeNameSuggestions)
			return
		}

		beeName := r.PathValue("name")
		if beeName == "" {
			responses.BadRequest(w, r, msgInvalidName)
			return
		}

		_, err := s.RejectBeeNameSuggestion(beeName)
		if err != nil {
			responses.InternalServerError(w, r, msgFailedToRejectBeeNameSuggestion)
			return
		}
		responses.NoContent(w, r)
	}
}
