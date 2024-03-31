package beenamegenerator

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
)

// -------------- Globals --------------
var (
	BNG_AUTH_TOKEN = os.Getenv("BNG_AUTH_TOKEN")

	SERVER_URL string = "https://api.neuralnexus.dev/api/v1/bee-name-generator"

	unauthorizedResponse = Response{
		Success: false,
		Message: "Unauthorized",
		Error:   "",
	}
)

// -------------- Structs --------------

// General API response struct
type Response struct {
	Success bool   `json:"success,omitempty"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// NameResponse struct (extends Response)
type NameResponse struct {
	Response
	Name string `json:"name"`
}

// Creates a new NameResponse struct
func NewNameResponse(name string, success bool, message, error string) NameResponse {
	return NameResponse{
		Response: Response{
			Success: success,
			Message: message,
			Error:   error,
		},
		Name: name,
	}
}

// Creates a new successful or failed NameResponse struct
func NewSuccessfulNameResponse(name string) NameResponse {
	return NewNameResponse(name, true, "", "")
}

// Creates a new failed NameResponse struct
func NewFailedNameResponse(message, error string) NameResponse {
	return NewNameResponse("", false, message, error)
}

// AmountResponse struct (extends Response)
type AmountResponse struct {
	Response
	Amount int64 `json:"amount"`
}

// SuggestionsResponse struct
type SuggestionsResponse struct {
	Response
	Suggestions []string `json:"suggestions"`
}

// -------------- Functions --------------

// getBeeName returns a random bee name from the database
func getBeeName() database.Response[string] {
	db := database.GetDB("bee_name_generator")
	var beeName string

	err := db.QueryRow(context.Background(), "SELECT name FROM bee_name ORDER BY random() LIMIT 1").Scan(&beeName)
	if err != nil {
		return database.Response[string]{
			Success: false,
			Message: "Failed to get bee name: " + err.Error(),
		}
	}
	defer db.Close()

	return database.Response[string]{
		Success: true,
		Data:    beeName,
	}
}

// uploadBeeName uploads a bee name to the database
func uploadBeeName(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")

	_, err := db.Exec(context.Background(), "INSERT INTO bee_name (name) VALUES ($1)", beeName)
	if err != nil {
		return database.Response[string]{
			Success: false,
			Message: "Failed to upload bee name: " + err.Error(),
		}
	}
	defer db.Close()

	return database.Response[string]{
		Success: true,
		Data:    beeName,
	}
}

// deleteBeeName deletes a bee name from the database
func deleteBeeName(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")

	_, err := db.Exec(context.Background(), "DELETE FROM bee_name WHERE name = $1", beeName)
	if err != nil {
		return database.Response[string]{
			Success: false,
			Message: "Failed to delete bee name: " + err.Error(),
		}
	}
	defer db.Close()

	return database.Response[string]{
		Success: true,
		Data:    beeName,
	}
}

// submitBeeName submits a bee name to the suggestion database
func submitBeeName(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")

	_, err := db.Exec(context.Background(), "INSERT INTO bee_name_suggestion (name) VALUES ($1)", beeName)
	if err != nil {
		return database.Response[string]{
			Success: false,
			Message: "Failed to submit bee name: " + err.Error(),
		}
	}
	defer db.Close()

	return database.Response[string]{
		Success: true,
		Data:    beeName,
	}
}

// getBeeNameSuggestions returns a list of bee name suggestions
func getBeeNameSuggestions(amount int64) database.Response[[]string] {
	db := database.GetDB("bee_name_generator")
	var beeNames []string

	rows, err := db.Query(context.Background(), "SELECT name FROM bee_name_suggestion ORDER BY random() LIMIT $1", amount)
	if err != nil {
		return database.Response[[]string]{
			Success: false,
			Message: "Failed to get bee name suggestions: " + err.Error(),
		}
	}
	defer rows.Close()

	for rows.Next() {
		var beeName string
		err := rows.Scan(&beeName)
		if err != nil {
			return database.Response[[]string]{
				Success: false,
				Message: "Failed to get bee name suggestions: " + err.Error(),
			}
		}
		beeNames = append(beeNames, beeName)
	}

	if len(beeNames) == 0 {
		return database.Response[[]string]{
			Success: false,
			Message: "No bee name suggestions found",
		}
	}

	return database.Response[[]string]{
		Success: true,
		Data:    beeNames,
	}
}

// acceptBeeNameSuggestion accepts a bee name suggestion
func acceptBeeNameSuggestion(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")

	_, err := db.Exec(context.Background(), "INSERT INTO bee_name (name) VALUES ($1)", beeName)
	if err != nil {
		return database.Response[string]{
			Success: false,
			Message: "Failed to accept bee name suggestion: " + err.Error(),
		}
	}
	_, err = db.Exec(context.Background(), "DELETE FROM bee_name_suggestion WHERE name = $1", beeName)
	if err != nil {
		return database.Response[string]{
			Success: false,
			Message: "Failed to accept bee name suggestion: " + err.Error(),
		}
	}
	defer db.Close()

	return database.Response[string]{
		Success: true,
		Data:    beeName,
	}
}

// rejectBeeNameSuggestion rejects a bee name suggestion
func rejectBeeNameSuggestion(beeName string) database.Response[string] {
	db := database.GetDB("bee_name_generator")

	_, err := db.Exec(context.Background(), "DELETE FROM bee_name_suggestion WHERE name = $1", beeName)
	if err != nil {
		return database.Response[string]{
			Success: false,
			Message: "Failed to reject bee name suggestion: " + err.Error(),
		}
	}
	defer db.Close()

	return database.Response[string]{
		Success: true,
		Data:    beeName,
	}
}

// -------------- Routes --------------

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	mux.HandleFunc("GET /bee-name-generator", GetRoot)
	mux.HandleFunc("GET /bee-name-generator/name", GetBeeNameHandler)
	mux.HandleFunc("POST /bee-name-generator/name", UploadBeeNameHandler)
	mux.HandleFunc("POST /bee-name-generator/name/{name}", UploadBeeNameHandler)
	mux.HandleFunc("DELETE /bee-name-generator/name", DeleteBeeNameHandler)
	mux.HandleFunc("DELETE /bee-name-generator/name/{name}", DeleteBeeNameHandler)
	mux.HandleFunc("POST /bee-name-generator/suggestion", SubmitBeeNameHandler)
	mux.HandleFunc("POST /bee-name-generator/suggestion/{name}", SubmitBeeNameHandler)
	mux.HandleFunc("GET /bee-name-generator/suggestion", GetBeeNameSuggestionsHandler)
	mux.HandleFunc("GET /bee-name-generator/suggestion/{amount}", GetBeeNameSuggestionsHandler)
	mux.HandleFunc("PUT /bee-name-generator/suggestion", AcceptBeeNameSuggestionHandler)
	mux.HandleFunc("PUT /bee-name-generator/suggestion/{name}", AcceptBeeNameSuggestionHandler)
	mux.HandleFunc("DELETE /bee-name-generator/suggestion", RejectBeeNameSuggestionHandler)
	mux.HandleFunc("DELETE /bee-name-generator/suggestion/{name}", RejectBeeNameSuggestionHandler)
	return mux
}

// GetRoot get a simple docs/examples page
func GetRoot(w http.ResponseWriter, r *http.Request) {
	// Read the html file
	html, err := os.ReadFile("static/beenamegenerator/templates/index.html")
	if err != nil {
		http.Error(w, "Failed to read index.html: "+err.Error(), 500)
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
		http.Error(w, "Failed to get bee name: "+beeName.Message, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"name":"` + beeName.Data + `"}`))
}

// UploadBeeNameHandler Upload a bee name (authenticated)
func UploadBeeNameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+BNG_AUTH_TOKEN {
		http.Error(w, "Unauthorized", 401)
		return
	}

	beeName := r.PathValue("name")
	if beeName == "" {
		var nameResponse NameResponse
		err := json.NewDecoder(r.Body).Decode(&nameResponse)
		if err == nil {
			beeName = nameResponse.Name
		}
	}
	if beeName == "" {
		http.Error(w, "Invalid name", 400)
		return
	}
	upload := uploadBeeName(beeName)
	if !upload.Success {
		http.Error(w, "Failed to upload bee name: "+upload.Message, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"name":"` + beeName + `"}`))
}

// DeleteBeeName Delete a bee name (authenticated)
func DeleteBeeNameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+BNG_AUTH_TOKEN {
		http.Error(w, "Unauthorized", 401)
		return
	}

	beeName := r.PathValue("name")
	if beeName == "" {
		var nameResponse NameResponse
		err := json.NewDecoder(r.Body).Decode(&nameResponse)
		if err == nil {
			beeName = nameResponse.Name
		}
	}
	if beeName == "" {
		http.Error(w, "Invalid name", 400)
		return
	}
	delete := deleteBeeName(beeName)
	if !delete.Success {
		http.Error(w, "Failed to delete bee name: "+delete.Message, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"name":"` + beeName + `"}`))
}

// SubmitBeeNameHandler Submit a bee name suggestion
func SubmitBeeNameHandler(w http.ResponseWriter, r *http.Request) {
	beeName := r.PathValue("name")
	if beeName == "" {
		var nameResponse NameResponse
		err := json.NewDecoder(r.Body).Decode(&nameResponse)
		if err == nil {
			beeName = nameResponse.Name
		}
	}

	if beeName == "" {
		http.Error(w, "Invalid name", 400)
		return
	}
	submit := submitBeeName(beeName)
	if !submit.Success {
		http.Error(w, submit.Message, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"name":"` + beeName + `"}`))
}

// GetBeeNameSuggestions Get a list of bee name suggestions (authenticated)
func GetBeeNameSuggestionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+BNG_AUTH_TOKEN {
		http.Error(w, "Unauthorized", 401)
		return
	}

	amount := r.PathValue("amount")
	if amount == "" {
		var amountResponse AmountResponse
		err := json.NewDecoder(r.Body).Decode(&amountResponse)
		if err == nil {
			amount = strconv.FormatInt(amountResponse.Amount, 10)
		}
	}
	if amount == "" || amount == "0" {
		amount = "1"
	}

	amountInt, err := strconv.ParseInt(amount, 10, 64)
	if err != nil {
		http.Error(w, "Invalid amount", 400)
		return
	}

	suggestions := getBeeNameSuggestions(amountInt)
	if !suggestions.Success {
		http.Error(w, "Failed to get bee name suggestions: "+suggestions.Message, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"suggestions":["` + strings.Join(suggestions.Data, "\",\"") + `"]}`))
}

// AcceptBeeNameSuggestionHandler Accept a bee name suggestion (authenticated)
func AcceptBeeNameSuggestionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+BNG_AUTH_TOKEN {
		http.Error(w, "Unauthorized", 401)
		return
	}

	beeName := r.PathValue("name")
	if beeName == "" {
		var nameResponse NameResponse
		err := json.NewDecoder(r.Body).Decode(&nameResponse)
		if err == nil {
			beeName = nameResponse.Name
		}
	}
	if beeName == "" {
		http.Error(w, "Invalid name", 400)
		return
	}
	accept := acceptBeeNameSuggestion(beeName)
	if !accept.Success {
		http.Error(w, "Failed to accept bee name suggestion: "+accept.Message, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"name":"` + beeName + `"}`))
}

// RejectBeeNameSuggestionHandler Reject a bee name suggestion (authenticated)
func RejectBeeNameSuggestionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+BNG_AUTH_TOKEN {
		http.Error(w, "Unauthorized", 401)
		return
	}

	beeName := r.PathValue("name")
	if beeName == "" {
		var nameResponse NameResponse
		err := json.NewDecoder(r.Body).Decode(&nameResponse)
		if err == nil {
			beeName = nameResponse.Name
		}
	}
	if beeName == "" {
		http.Error(w, "Invalid name", 400)
		return
	}
	reject := rejectBeeNameSuggestion(beeName)
	if !reject.Success {
		http.Error(w, "Failed to reject bee name suggestion: "+reject.Message, 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"name":"` + beeName + `"}`))
}
