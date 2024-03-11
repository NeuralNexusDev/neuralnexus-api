package main

import (
	"neuralnexus-api/modules/beenamegenerator"
	"neuralnexus-api/modules/cct_turtle"
	"neuralnexus-api/modules/mcstatus"
	"neuralnexus-api/modules/projects"
	"neuralnexus-api/modules/switchboard"
	"os"

	"github.com/labstack/echo/v4"
)

// -------------- Main --------------
func main() {
	// Get IP from env
	ip := os.Getenv("IP_ADDRESS")
	if ip == "" {
		ip = "0.0.0.0"
	}

	// Get port from env
	port := os.Getenv("REST_PORT")
	if port == "" {
		port = "8080"
	}

	e := echo.New()

	// -------------- Routes --------------

	// -------------- Bee Name Generator --------------
	e.GET("/api/v1/bee-name-generator", beenamegenerator.GetRoot)

	// Get a bee name
	e.GET("/api/v1/bee-name-generator/name", beenamegenerator.GetBeeNameHandler)

	// Upload a bee name
	e.POST("/api/v1/bee-name-generator/name", beenamegenerator.UploadBeeNameHandler)
	e.POST("/api/v1/bee-name-generator/name/:name", beenamegenerator.UploadBeeNameHandler)

	// Delete a bee name
	e.DELETE("/api/v1/bee-name-generator/name", beenamegenerator.DeleteBeeNameHandler)
	e.DELETE("/api/v1/bee-name-generator/name/:name", beenamegenerator.DeleteBeeNameHandler)

	// Submit a bee name
	e.POST("/api/v1/bee-name-generator/suggestion", beenamegenerator.SubmitBeeNameHandler)
	e.POST("/api/v1/bee-name-generator/suggestion/:name", beenamegenerator.SubmitBeeNameHandler)

	// Get bee name suggestions
	e.GET("/api/v1/bee-name-generator/suggestion", beenamegenerator.GetBeeNameSuggestionsHandler)
	e.GET("/api/v1/bee-name-generator/suggestion/:amount", beenamegenerator.GetBeeNameSuggestionsHandler)

	// Accept a bee name suggestion
	e.PUT("/api/v1/bee-name-generator/suggestion", beenamegenerator.AcceptBeeNameSuggestionHandler)
	e.PUT("/api/v1/bee-name-generator/suggestion/:name", beenamegenerator.AcceptBeeNameSuggestionHandler)

	// Reject a bee name suggestion
	e.DELETE("/api/v1/bee-name-generator/suggestion", beenamegenerator.RejectBeeNameSuggestionHandler)
	e.DELETE("/api/v1/bee-name-generator/suggestion/:name", beenamegenerator.RejectBeeNameSuggestionHandler)

	// -------------- MC Status --------------
	e.GET("/api/v1/mcstatus", mcstatus.GetRoot)
	e.GET("/api/v1/mcstatus/:address", mcstatus.GetServerStatus)
	e.GET("/api/v1/mcstatus/icon/:address", mcstatus.GetIcon)

	// -------------- Projects --------------
	e.GET("/api/v1/projects/releases/:group/:project", projects.GetReleasesHandler)

	// -------------- Switchboard --------------
	// e.GET("/ws/v1/switchboard/relay", switchboard.WebSocketRelayHandler)
	e.GET("/websocket/:id", switchboard.WebSocketRelayHandler)

	// -------------- CCT Turtle --------------
	e.GET("/api/v1/cct-turtle/startup.lua", cct_turtle.GetTurtleCode)

	e.Logger.Fatal(e.Start(ip + ":" + port))

	// Connect to MongoDB
	// uri := os.Getenv("MONGODB_URI")
	// if uri == "" {
	// log.Fatal("You must set your 'MONGODB_URI' environment variable. See\n\t https://www.mongodb.com/docs/drivers/go/current/usage-examples/#environment-variable")
	// }
	// var err error
	// database.MongoClient, err = mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	// if err != nil {
	// panic(err)
	// }

	// Get JWT signing key from env
	// authentication.JwtKey = os.Getenv("JWT_KEY")

	// Create router
	// var router *gin.Engine = gin.Default()

	// Routes
	// Economy DB
	// router.GET("/:userID/currencies", economy_db.GetCurrencies)
	// router.GET("/:userID/currencies/:currencyID", economy_db.GetCurrency)
	// router.POST("/:userID/currencies/:currencyID/:ammount", economy_db.UpdateCurrency)
	// router.GET("/:userID/owned", economy_db.GetOwnedCurrencies)
	// router.PUT("/:userID/owned/:currencyID", economy_db.CreateCurrency)

	// router.Run(ip + ":" + port)
}

// -------------- Structs --------------

// -------------- Enums --------------

// -------------- Functions --------------

// -------------- Handlers --------------
