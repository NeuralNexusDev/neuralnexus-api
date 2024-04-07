package beenamegenerator

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// -------------- Globals --------------
const (
	SERVER_URL string = "https://api.neuralnexus.dev/api/v1/bee-name-generator"
)

// -------------- Structs --------------

// NameResponse struct (extends Response)
type NameResponse struct {
	Name string `json:"name" xml:"name"`
}

// NewNameResponse - Create a new NameResponse
func NewNameResponse(name string) NameResponse {
	return NameResponse{
		Name: name,
	}
}

// AmountResponse struct (extends Response)
type AmountResponse struct {
	Amount int64 `json:"amount" xml:"amount"`
}

// NewAmountResponse - Create a new AmountResponse
func NewAmountResponse(amount int64) AmountResponse {
	return AmountResponse{
		Amount: amount,
	}
}

// SuggestionsResponse struct
type SuggestionsResponse struct {
	Suggestions []string `json:"suggestions" xml:"suggestions"`
}

// NewSuggestionsResponse - Create a new SuggestionsResponse
func NewSuggestionsResponse(suggestions []string) SuggestionsResponse {
	return SuggestionsResponse{
		Suggestions: suggestions,
	}
}

// -------------- Functions --------------

// getBeeName returns a random bee name from the database
func getBeeName() database.Response[string] {
	db := database.GetDB("bee_name_generator")
	defer db.Close()

	var beeName string
	err := db.QueryRow(context.Background(), "SELECT name FROM bee_name ORDER BY random() LIMIT 1").Scan(&beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to get bee name")
	}
	return database.SuccessResponse(beeName)
}

// uploadBeeName uploads a bee name to the database
func uploadBeeName(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")
	defer db.Close()

	_, err := db.Exec(context.Background(), "INSERT INTO bee_name (name) VALUES ($1)", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to upload bee name")
	}
	return database.SuccessResponse(beeName)
}

// deleteBeeName deletes a bee name from the database
func deleteBeeName(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")
	defer db.Close()

	_, err := db.Exec(context.Background(), "DELETE FROM bee_name WHERE name = $1", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to delete bee name")
	}
	return database.SuccessResponse(beeName)
}

// submitBeeName submits a bee name to the suggestion database
func submitBeeName(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")
	defer db.Close()

	_, err := db.Exec(context.Background(), "INSERT INTO bee_name_suggestion (name) VALUES ($1)", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to submit bee name")
	}
	return database.SuccessResponse(beeName)
}

// getBeeNameSuggestions returns a list of bee name suggestions
func getBeeNameSuggestions(amount int64) database.Response[[]string] {
	db := database.GetDB("bee_name_generator")
	defer db.Close()

	var beeNames []string
	rows, err := db.Query(context.Background(), "SELECT name FROM bee_name_suggestion ORDER BY random() LIMIT $1", amount)
	if err != nil {
		return database.ErrorResponse[[]string]("Failed to get bee name suggestions")
	}
	defer rows.Close()

	for rows.Next() {
		var beeName string
		err := rows.Scan(&beeName)
		if err != nil {
			return database.ErrorResponse[[]string]("Failed to get bee name suggestions")
		}
		beeNames = append(beeNames, beeName)
	}

	if len(beeNames) == 0 {
		return database.ErrorResponse[[]string]("No bee name suggestions found")
	}
	return database.SuccessResponse(beeNames)
}

// acceptBeeNameSuggestion accepts a bee name suggestion
func acceptBeeNameSuggestion(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")
	defer db.Close()

	_, err := db.Exec(context.Background(), "INSERT INTO bee_name (name) VALUES ($1)", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to accept bee name suggestion")
	}

	_, err = db.Exec(context.Background(), "DELETE FROM bee_name_suggestion WHERE name = $1", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to accept bee name suggestion")
	}
	return database.SuccessResponse(beeName)
}

// rejectBeeNameSuggestion rejects a bee name suggestion
func rejectBeeNameSuggestion(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")
	defer db.Close()

	_, err := db.Exec(context.Background(), "DELETE FROM bee_name_suggestion WHERE name = $1", beeName)
	if err != nil {
		return database.ErrorResponse[string]("Failed to reject bee name suggestion")
	}
	return database.SuccessResponse(beeName)
}

// -------------- Routes --------------

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux, authedMux *http.ServeMux) (*http.ServeMux, *http.ServeMux) {
	mux.HandleFunc("GET /bee-name-generator", GetRoot)
	mux.HandleFunc("GET /bee-name-generator/name", GetBeeNameHandler)
	authedMux.HandleFunc("POST /bee-name-generator/name", UploadBeeNameHandler)
	authedMux.HandleFunc("POST /bee-name-generator/name/{name}", UploadBeeNameHandler)
	authedMux.HandleFunc("DELETE /bee-name-generator/name", DeleteBeeNameHandler)
	authedMux.HandleFunc("DELETE /bee-name-generator/name/{name}", DeleteBeeNameHandler)
	mux.HandleFunc("POST /bee-name-generator/suggestion", SubmitBeeNameHandler)
	mux.HandleFunc("POST /bee-name-generator/suggestion/{name}", SubmitBeeNameHandler)
	authedMux.HandleFunc("GET /bee-name-generator/suggestion", GetBeeNameSuggestionsHandler)
	authedMux.HandleFunc("GET /bee-name-generator/suggestion/{amount}", GetBeeNameSuggestionsHandler)
	authedMux.HandleFunc("PUT /bee-name-generator/suggestion", AcceptBeeNameSuggestionHandler)
	authedMux.HandleFunc("PUT /bee-name-generator/suggestion/{name}", AcceptBeeNameSuggestionHandler)
	authedMux.HandleFunc("DELETE /bee-name-generator/suggestion", RejectBeeNameSuggestionHandler)
	authedMux.HandleFunc("DELETE /bee-name-generator/suggestion/{name}", RejectBeeNameSuggestionHandler)
	return mux, authedMux
}

// TODO: Deprecate this in favor of the actual web page
// GetRoot get a simple docs/examples page
func GetRoot(w http.ResponseWriter, r *http.Request) {
	// Read the html file
	html, err := os.ReadFile("static/beenamegenerator/templates/index.html")
	if err != nil {
		log.Println("Failed to read index.html: " + err.Error())
		responses.SendAndEncodeInternalServerError(w, r, "Failed to read index.html")
		return
	}

	// Replace the server url
	htmlString := string(html)
	htmlString = strings.ReplaceAll(htmlString, "{{SERVER_URL}}", SERVER_URL)

	// Serve the html
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(htmlString))
}

// GetBeeNameHandler
func GetBeeNameHandler(w http.ResponseWriter, r *http.Request) {
	beeName := getBeeName()
	if !beeName.Success {
		responses.SendAndEncodeInternalServerError(w, r, "Failed to get bee name")
		return
	}
	responses.SendAndEncodeStruct(w, r, http.StatusOK, NewNameResponse(beeName.Data))
}

// UploadBeeNameHandler Upload a bee name (authenticated)
func UploadBeeNameHandler(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(middleware.SessionKey).(auth.Session)
	if !session.HasPermission(auth.ScopeAdminBeeNameGenerator) {
		responses.SendAndEncodeForbidden(w, r, "You do not have permission to upload bee names")
		return
	}

	beeName := r.PathValue("name")
	if beeName == "" {
		var nameResponse NameResponse
		err := responses.DecodeStruct(r, &nameResponse)
		if err == nil {
			beeName = nameResponse.Name
		}
	}
	if beeName == "" {
		responses.SendAndEncodeBadRequest(w, r, "Invalid name")
		return
	}

	upload := uploadBeeName(beeName)
	if !upload.Success {
		responses.SendAndEncodeInternalServerError(w, r, "Failed to upload bee name")
		return
	}
	responses.SendAndEncodeStruct(w, r, http.StatusOK, NewNameResponse(beeName))
}

// DeleteBeeName Delete a bee name (authenticated)
func DeleteBeeNameHandler(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(middleware.SessionKey).(auth.Session)
	if !session.HasPermission(auth.ScopeAdminBeeNameGenerator) {
		responses.SendAndEncodeForbidden(w, r, "You do not have permission to delete bee names")
		return
	}

	beeName := r.PathValue("name")
	if beeName == "" {
		var nameResponse NameResponse
		err := responses.DecodeStruct(r, &nameResponse)
		if err == nil {
			beeName = nameResponse.Name
		}
	}
	if beeName == "" {
		responses.SendAndEncodeBadRequest(w, r, "Invalid name")
		return
	}

	delete := deleteBeeName(beeName)
	if !delete.Success {
		responses.SendAndEncodeInternalServerError(w, r, "Failed to delete bee name")
		return
	}
	responses.SendAndEncodeStruct(w, r, http.StatusOK, NewNameResponse(beeName))
}

// SubmitBeeNameHandler Submit a bee name suggestion
func SubmitBeeNameHandler(w http.ResponseWriter, r *http.Request) {
	beeName := r.PathValue("name")
	if beeName == "" {
		var nameResponse NameResponse
		err := responses.DecodeStruct(r, &nameResponse)
		if err == nil {
			beeName = nameResponse.Name
		}
	}
	if beeName == "" {
		responses.SendAndEncodeBadRequest(w, r, "Invalid name")
		return
	}

	submit := submitBeeName(beeName)
	if !submit.Success {
		responses.SendAndEncodeInternalServerError(w, r, "Failed to submit bee name")
		return
	}
	responses.SendAndEncodeStruct(w, r, http.StatusOK, NewNameResponse(beeName))
}

// GetBeeNameSuggestions Get a list of bee name suggestions (authenticated)
func GetBeeNameSuggestionsHandler(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(middleware.SessionKey).(auth.Session)
	if !session.HasPermission(auth.ScopeAdminBeeNameGenerator) {
		responses.SendAndEncodeForbidden(w, r, "You do not have permission to get bee name suggestions")
		return
	}

	amount := r.PathValue("amount")
	if amount == "" {
		var amountResponse AmountResponse
		err := responses.DecodeStruct(r, &amountResponse)
		if err == nil {
			amount = strconv.FormatInt(amountResponse.Amount, 10)
		}
	}
	if amount == "" || amount == "0" {
		amount = "1"
	}
	amountInt, err := strconv.ParseInt(amount, 10, 64)
	if err != nil {
		responses.SendAndEncodeBadRequest(w, r, "Invalid amount provided")
		return
	}

	suggestions := getBeeNameSuggestions(amountInt)
	if !suggestions.Success {
		responses.SendAndEncodeInternalServerError(w, r, "Failed to get bee name suggestions")
		return
	}
	responses.SendAndEncodeStruct(w, r, http.StatusOK, NewSuggestionsResponse(suggestions.Data))
}

// AcceptBeeNameSuggestionHandler Accept a bee name suggestion (authenticated)
func AcceptBeeNameSuggestionHandler(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(middleware.SessionKey).(auth.Session)
	if !session.HasPermission(auth.ScopeAdminBeeNameGenerator) {
		responses.SendAndEncodeForbidden(w, r, "You do not have permission to accept bee name suggestions")
		return
	}

	beeName := r.PathValue("name")
	if beeName == "" {
		var nameResponse NameResponse
		err := responses.DecodeStruct(r, &nameResponse)
		if err == nil {
			beeName = nameResponse.Name
		}
	}
	if beeName == "" {
		responses.SendAndEncodeBadRequest(w, r, "Invalid name")
		return
	}

	accept := acceptBeeNameSuggestion(beeName)
	if !accept.Success {
		responses.SendAndEncodeInternalServerError(w, r, "Failed to accept bee name suggestion")
		return
	}
	responses.SendAndEncodeStruct(w, r, http.StatusOK, NewNameResponse(beeName))
}

// RejectBeeNameSuggestionHandler Reject a bee name suggestion (authenticated)
func RejectBeeNameSuggestionHandler(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(middleware.SessionKey).(auth.Session)
	if !session.HasPermission(auth.ScopeAdminBeeNameGenerator) {
		responses.SendAndEncodeForbidden(w, r, "You do not have permission to reject bee name suggestions")
		return
	}

	beeName := r.PathValue("name")
	if beeName == "" {
		var nameResponse NameResponse
		err := responses.DecodeStruct(r, &nameResponse)
		if err == nil {
			beeName = nameResponse.Name
		}
	}
	if beeName == "" {
		responses.SendAndEncodeBadRequest(w, r, "Invalid name")
		return
	}

	reject := rejectBeeNameSuggestion(beeName)
	if !reject.Success {
		responses.SendAndEncodeInternalServerError(w, r, "Failed to reject bee name suggestion")
		return
	}
	responses.SendAndEncodeStruct(w, r, http.StatusOK, NewNameResponse(beeName))
}
