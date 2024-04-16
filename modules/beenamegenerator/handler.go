package beenamegenerator

import (
	"net/http"
	"strconv"

	"github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux, authedMux *http.ServeMux) (*http.ServeMux, *http.ServeMux) {
	store := NewStore(database.GetDB("bee_name_generator"))
	mux.HandleFunc("GET /api/v1/bee-name-generator/name", GetBeeNameHandler(store))
	authedMux.HandleFunc("POST /api/v1/bee-name-generator/name/{name}", UploadBeeNameHandler(store))
	authedMux.HandleFunc("DELETE /api/v1/bee-name-generator/name/{name}", DeleteBeeNameHandler(store))
	mux.HandleFunc("POST /api/v1/bee-name-generator/suggestion/{name}", SubmitBeeNameHandler(store))
	authedMux.HandleFunc("GET /api/v1/bee-name-generator/suggestion", GetBeeNameSuggestionsHandler(store))
	authedMux.HandleFunc("GET /api/v1/bee-name-generator/suggestion/{amount}", GetBeeNameSuggestionsHandler(store))
	authedMux.HandleFunc("PUT /api/v1/bee-name-generator/suggestion/{name}", AcceptBeeNameSuggestionHandler(store))
	authedMux.HandleFunc("DELETE /api/v1/bee-name-generator/suggestion/{name}", RejectBeeNameSuggestionHandler(store))
	return mux, authedMux
}

// GetBeeNameHandler
func GetBeeNameHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		beeName := s.GetBeeName()
		if !beeName.Success {
			responses.SendAndEncodeInternalServerError(w, r, "Failed to get bee name")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, NewNameResponse(beeName.Data))
	}
}

// UploadBeeNameHandler Upload a bee name
func UploadBeeNameHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(middleware.SessionKey).(auth.Session)
		if !session.HasPermission(auth.ScopeAdminBeeNameGenerator) {
			responses.SendAndEncodeForbidden(w, r, "You do not have permission to upload bee names")
			return
		}

		beeName := r.PathValue("name")
		if beeName == "" {
			responses.SendAndEncodeBadRequest(w, r, "Invalid name")
			return
		}

		upload := s.UploadBeeName(beeName)
		if !upload.Success {
			responses.SendAndEncodeInternalServerError(w, r, "Failed to upload bee name")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, NewNameResponse(beeName))
	}
}

// DeleteBeeName Delete a bee name
func DeleteBeeNameHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(middleware.SessionKey).(auth.Session)
		if !session.HasPermission(auth.ScopeAdminBeeNameGenerator) {
			responses.SendAndEncodeForbidden(w, r, "You do not have permission to delete bee names")
			return
		}

		beeName := r.PathValue("name")
		if beeName == "" {
			responses.SendAndEncodeBadRequest(w, r, "Invalid name")
			return
		}

		delete := s.DeleteBeeName(beeName)
		if !delete.Success {
			responses.SendAndEncodeInternalServerError(w, r, "Failed to delete bee name")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, NewNameResponse(beeName))
	}
}

// SubmitBeeNameHandler Submit a bee name suggestion
func SubmitBeeNameHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		beeName := r.PathValue("name")
		if beeName == "" {
			responses.SendAndEncodeBadRequest(w, r, "Invalid name")
			return
		}

		submit := s.SubmitBeeName(beeName)
		if !submit.Success {
			responses.SendAndEncodeInternalServerError(w, r, "Failed to submit bee name")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, NewNameResponse(beeName))
	}
}

// GetBeeNameSuggestions Get a list of bee name suggestions
func GetBeeNameSuggestionsHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(middleware.SessionKey).(auth.Session)
		if !session.HasPermission(auth.ScopeAdminBeeNameGenerator) {
			responses.SendAndEncodeForbidden(w, r, "You do not have permission to get bee name suggestions")
			return
		}

		amount := r.PathValue("amount")
		if amount == "" || amount == "0" {
			amount = "NAN"
		}
		amountInt, err := strconv.ParseInt(amount, 10, 64)
		if err != nil {
			responses.SendAndEncodeBadRequest(w, r, "Invalid amount provided")
			return
		}

		suggestions := s.GetBeeNameSuggestions(amountInt)
		if !suggestions.Success {
			responses.SendAndEncodeInternalServerError(w, r, "Failed to get bee name suggestions")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, NewSuggestionsResponse(suggestions.Data))
	}
}

// AcceptBeeNameSuggestionHandler Accept a bee name suggestion
func AcceptBeeNameSuggestionHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(middleware.SessionKey).(auth.Session)
		if !session.HasPermission(auth.ScopeAdminBeeNameGenerator) {
			responses.SendAndEncodeForbidden(w, r, "You do not have permission to accept bee name suggestions")
			return
		}

		beeName := r.PathValue("name")
		if beeName == "" {
			responses.SendAndEncodeBadRequest(w, r, "Invalid name")
			return
		}

		accept := s.AcceptBeeNameSuggestion(beeName)
		if !accept.Success {
			responses.SendAndEncodeInternalServerError(w, r, "Failed to accept bee name suggestion")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, NewNameResponse(beeName))
	}
}

// RejectBeeNameSuggestionHandler Reject a bee name suggestion
func RejectBeeNameSuggestionHandler(s BNGStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(middleware.SessionKey).(auth.Session)
		if !session.HasPermission(auth.ScopeAdminBeeNameGenerator) {
			responses.SendAndEncodeForbidden(w, r, "You do not have permission to reject bee name suggestions")
			return
		}

		beeName := r.PathValue("name")
		if beeName == "" {
			responses.SendAndEncodeBadRequest(w, r, "Invalid name")
			return
		}

		reject := s.RejectBeeNameSuggestion(beeName)
		if !reject.Success {
			responses.SendAndEncodeInternalServerError(w, r, "Failed to reject bee name suggestion")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, NewNameResponse(beeName))
	}
}
