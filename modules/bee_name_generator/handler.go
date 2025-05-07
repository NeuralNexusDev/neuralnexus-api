package beenamegenerator

import (
	"log"
	"net/http"
	"strconv"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	store := NewStore(database.GetDB("bee_name_generator"))
	mux.HandleFunc("GET /api/v1/bee-name-generator/name", GetBeeNameHandler(store))
	mux.HandleFunc("POST /api/v1/bee-name-generator/name/{name}", mw.Auth(UploadBeeNameHandler(store)))
	mux.HandleFunc("DELETE /api/v1/bee-name-generator/name/{name}", mw.Auth(DeleteBeeNameHandler(store)))
	mux.HandleFunc("POST /api/v1/bee-name-generator/suggestion/{name}", SubmitBeeNameHandler(store))
	mux.HandleFunc("GET /api/v1/bee-name-generator/suggestion", mw.Auth(GetBeeNameSuggestionsHandler(store)))
	mux.HandleFunc("GET /api/v1/bee-name-generator/suggestion/{amount}", mw.Auth(GetBeeNameSuggestionsHandler(store)))
	mux.HandleFunc("PUT /api/v1/bee-name-generator/suggestion/{name}", mw.Auth(AcceptBeeNameSuggestionHandler(store)))
	mux.HandleFunc("DELETE /api/v1/bee-name-generator/suggestion/{name}", mw.Auth(RejectBeeNameSuggestionHandler(store)))
	return mux
}

// GetBeeNameHandler Get a bee name
func GetBeeNameHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		beeName, err := s.GetBeeName()
		if err != nil {
			log.Println("Failed to get bee name:\n\t", err)
			responses.InternalServerError(w, r, "Failed to get bee name")
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
			responses.Forbidden(w, r, "You do not have permission to upload bee names")
			return
		}

		beeName := r.PathValue("name")
		if beeName == "" {
			responses.BadRequest(w, r, "Invalid name")
			return
		}

		_, err := s.UploadBeeName(beeName)
		if err != nil {
			log.Println("Failed to upload bee name:\n\t", err)
			responses.InternalServerError(w, r, "Failed to upload bee name")
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
			responses.Forbidden(w, r, "You do not have permission to delete bee names")
			return
		}

		beeName := r.PathValue("name")
		if beeName == "" {
			responses.BadRequest(w, r, "Invalid name")
			return
		}

		_, err := s.DeleteBeeName(beeName)
		if err != nil {
			log.Println("Failed to delete bee name:\n\t", err)
			responses.InternalServerError(w, r, "Failed to delete bee name")
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
			responses.BadRequest(w, r, "Invalid name")
			return
		}

		_, err := s.SubmitBeeName(beeName)
		if err != nil {
			log.Println("Failed to submit bee name:\n\t", err)
			responses.InternalServerError(w, r, "Failed to submit bee name")
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
			responses.Forbidden(w, r, "You do not have permission to get bee name suggestions")
			return
		}

		amount := r.PathValue("amount")
		if amount == "" || amount == "0" {
			amount = "NAN"
		}
		amountInt, err := strconv.ParseInt(amount, 10, 64)
		if err != nil {
			responses.BadRequest(w, r, "Invalid amount provided")
			return
		}

		suggestions, err := s.GetBeeNameSuggestions(amountInt)
		if err != nil {
			log.Println("Failed to get bee name suggestions:\n\t", err)
			responses.InternalServerError(w, r, "Failed to get bee name suggestions")
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
			responses.Forbidden(w, r, "You do not have permission to accept bee name suggestions")
			return
		}

		beeName := r.PathValue("name")
		if beeName == "" {
			responses.BadRequest(w, r, "Invalid name")
			return
		}

		_, err := s.AcceptBeeNameSuggestion(beeName)
		if err != nil {
			log.Println("Failed to accept bee name suggestion:\n\t", err)
			responses.InternalServerError(w, r, "Failed to accept bee name suggestion")
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
			responses.Forbidden(w, r, "You do not have permission to reject bee name suggestions")
			return
		}

		beeName := r.PathValue("name")
		if beeName == "" {
			responses.BadRequest(w, r, "Invalid name")
			return
		}

		_, err := s.RejectBeeNameSuggestion(beeName)
		if err != nil {
			responses.InternalServerError(w, r, "Failed to reject bee name suggestion")
			return
		}
		responses.NoContent(w, r)
	}
}
