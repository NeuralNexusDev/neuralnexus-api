package beenamegenerator

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/labstack/echo/v4"
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
	Success bool   `json:"success"`
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

// -------------- Handlers --------------

// GetRoot get a simple docs/examples page
func GetRoot(c echo.Context) error {
	// Read the html file
	html, err := os.ReadFile("static/beenamegenerator/templates/index.html")
	if err != nil {
		return c.String(500, "Failed to read index.html: "+err.Error())
	}

	// Replace the server url
	htmlString := string(html)
	htmlString = strings.ReplaceAll(htmlString, "{{SERVER_URL}}", SERVER_URL)

	// Serve the html
	c.Request().Header.Set("Content-Type", "text/html")
	return c.HTML(http.StatusOK, htmlString)
}

// GetBeeNameHandler
func GetBeeNameHandler(c echo.Context) error {
	beeName := getBeeName()
	if !beeName.Success {
		return c.JSON(500, NewFailedNameResponse("Failed to get bee name", beeName.Message))
	}
	return c.JSON(200, NewSuccessfulNameResponse(beeName.Data))
}

// UploadBeeNameHandler Upload a bee name (authenticated)
func UploadBeeNameHandler(c echo.Context) error {
	if c.Request().Header.Get("Authorization") != "Bearer "+BNG_AUTH_TOKEN {
		return c.JSON(401, unauthorizedResponse)
	}

	beeName := c.Param("name")
	if beeName == "" {
		beeName = c.QueryParam("name")
	}
	upload := uploadBeeName(beeName)
	if !upload.Success {
		return c.JSON(500, NewFailedNameResponse("Failed to upload bee name", upload.Message))
	}
	return c.JSON(200, NewSuccessfulNameResponse(beeName))
}

// DeleteBeeName Delete a bee name (authenticated)
func DeleteBeeNameHandler(c echo.Context) error {
	if c.Request().Header.Get("Authorization") != "Bearer "+BNG_AUTH_TOKEN {
		return c.JSON(401, unauthorizedResponse)
	}

	beeName := c.Param("name")
	if beeName == "" {
		beeName = c.QueryParam("name")
	}
	delete := deleteBeeName(beeName)
	if !delete.Success {
		return c.JSON(500, delete)
	}
	return c.JSON(200, delete)
}

// SubmitBeeNameHandler Submit a bee name suggestion
func SubmitBeeNameHandler(c echo.Context) error {
	beeName := c.Param("name")
	if beeName == "" {
		beeName = c.QueryParam("name")
	}

	submit := submitBeeName(beeName)
	if !submit.Success {
		return c.JSON(500, submit)
	}
	return c.JSON(200, submit)
}

// GetBeeNameSuggestions Get a list of bee name suggestions (authenticated)
func GetBeeNameSuggestionsHandler(c echo.Context) error {
	if c.Request().Header.Get("Authorization") != "Bearer "+BNG_AUTH_TOKEN {
		return c.JSON(401, unauthorizedResponse)
	}

	amount := c.Param("amount")
	if amount == "" {
		amount = c.QueryParam("amount")
	}
	if amount == "" {
		amount = "1"
	}
	amountInt, err := strconv.ParseInt(amount, 10, 64)
	if err != nil {
		return c.JSON(400, SuggestionsResponse{
			Response: Response{
				Success: false,
				Message: "Invalid amount",
				Error:   err.Error(),
			},
			Suggestions: []string{},
		})
	}

	suggestions := getBeeNameSuggestions(amountInt)
	if !suggestions.Success {
		return c.JSON(500, SuggestionsResponse{
			Response: Response{
				Success: false,
				Message: "Failed to get bee name suggestions",
				Error:   suggestions.Message,
			},
			Suggestions: []string{},
		})
	}

	return c.JSON(200, SuggestionsResponse{
		Response:    Response{Success: true, Message: "", Error: ""},
		Suggestions: suggestions.Data,
	})
}

// AcceptBeeNameSuggestionHandler Accept a bee name suggestion (authenticated)
func AcceptBeeNameSuggestionHandler(c echo.Context) error {
	if c.Request().Header.Get("Authorization") != "Bearer "+BNG_AUTH_TOKEN {
		return c.JSON(401, unauthorizedResponse)
	}

	beeName := c.Param("name")
	if beeName == "" {
		beeName = c.QueryParam("name")
	}

	accept := acceptBeeNameSuggestion(beeName)
	if !accept.Success {
		return c.JSON(500, NewFailedNameResponse("Failed to accept bee name suggestion", accept.Message))
	}
	return c.JSON(200, NewSuccessfulNameResponse(beeName))
}

// RejectBeeNameSuggestionHandler Reject a bee name suggestion (authenticated)
func RejectBeeNameSuggestionHandler(c echo.Context) error {
	if c.Request().Header.Get("Authorization") != "Bearer "+BNG_AUTH_TOKEN {
		return c.JSON(401, "Unauthorized")
	}

	beeName := c.Param("name")
	if beeName == "" {
		beeName = c.QueryParam("name")
	}

	reject := rejectBeeNameSuggestion(beeName)
	if !reject.Success {
		return c.JSON(500, NewFailedNameResponse("Failed to reject bee name suggestion", reject.Message))
	}
	return c.JSON(200, NewSuccessfulNameResponse(beeName))
}
