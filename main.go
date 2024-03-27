package main

import (
	"log"
	"net/http"
	"os"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/beenamegenerator"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/cct_turtle"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/mcstatus"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/projects"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/switchboard"
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

	// -------------- CCT Turtle --------------
	e.GET("/ws/v1/cct-turtle/:label", cct_turtle.WebSocketTurtleHandler)
	e.GET("/api/v1/cct-turtle/startup.lua", cct_turtle.GetTurtleCode)
	e.GET("/api/v1/cct-turtle/updating_startup.lua", cct_turtle.GetTurtleUpdatingCode)
	// e.GET("/api/v1/cct-turtle/status", cct_turtle.GetTurtleStatus)
	// e.GET("/api/v1/cct-turtle/status/:label", cct_turtle.GetTurtleStatus)
	e.GET("/api/v1/cct-turtle/forward", cct_turtle.MoveTurtleForward)
	e.GET("/api/v1/cct-turtle/forward/:label", cct_turtle.MoveTurtleForward)
	e.GET("/api/v1/cct-turtle/back", cct_turtle.MoveTurtleBackward)
	e.GET("/api/v1/cct-turtle/back/:label", cct_turtle.MoveTurtleBackward)
	e.GET("/api/v1/cct-turtle/up", cct_turtle.MoveTurtleUp)
	e.GET("/api/v1/cct-turtle/up/:label", cct_turtle.MoveTurtleUp)
	e.GET("/api/v1/cct-turtle/down", cct_turtle.MoveTurtleDown)
	e.GET("/api/v1/cct-turtle/down/:label", cct_turtle.MoveTurtleDown)
	e.GET("/api/v1/cct-turtle/left", cct_turtle.TurnTurtleLeft)
	e.GET("/api/v1/cct-turtle/left/:label", cct_turtle.TurnTurtleLeft)
	e.GET("/api/v1/cct-turtle/right", cct_turtle.TurnTurtleRight)
	e.GET("/api/v1/cct-turtle/right/:label", cct_turtle.TurnTurtleRight)
	e.GET("/api/v1/cct-turtle/dig", cct_turtle.DigTurtle)
	e.GET("/api/v1/cct-turtle/dig/:label", cct_turtle.DigTurtle)
	e.GET("/api/v1/cct-turtle/dig-up", cct_turtle.DigTurtleUp)
	e.GET("/api/v1/cct-turtle/dig-up/:label", cct_turtle.DigTurtleUp)
	e.GET("/api/v1/cct-turtle/dig-down", cct_turtle.DigTurtleDown)
	e.GET("/api/v1/cct-turtle/dig-down/:label", cct_turtle.DigTurtleDown)

	// e.Logger.Fatal(e.Start(ip + ":" + port))

	router := http.NewServeMux()

	// -------------- Bee Name Generator --------------
	router.HandleFunc("GET /api/v1/bee-name-generator", beenamegenerator.GetRoot)
	router.HandleFunc("GET /api/v1/bee-name-generator/name", beenamegenerator.GetBeeNameHandler)
	router.HandleFunc("POST /api/v1/bee-name-generator/name", beenamegenerator.UploadBeeNameHandler)
	router.HandleFunc("POST /api/v1/bee-name-generator/name/{name}", beenamegenerator.UploadBeeNameHandler)
	router.HandleFunc("DELETE /api/v1/bee-name-generator/name", beenamegenerator.DeleteBeeNameHandler)
	router.HandleFunc("DELETE /api/v1/bee-name-generator/name/{name}", beenamegenerator.DeleteBeeNameHandler)
	router.HandleFunc("POST /api/v1/bee-name-generator/suggestion", beenamegenerator.SubmitBeeNameHandler)
	router.HandleFunc("POST /api/v1/bee-name-generator/suggestion/{name}", beenamegenerator.SubmitBeeNameHandler)
	router.HandleFunc("GET /api/v1/bee-name-generator/suggestion", beenamegenerator.GetBeeNameSuggestionsHandler)
	router.HandleFunc("GET /api/v1/bee-name-generator/suggestion/{amount}", beenamegenerator.GetBeeNameSuggestionsHandler)
	router.HandleFunc("PUT /api/v1/bee-name-generator/suggestion", beenamegenerator.AcceptBeeNameSuggestionHandler)
	router.HandleFunc("PUT /api/v1/bee-name-generator/suggestion/{name}", beenamegenerator.AcceptBeeNameSuggestionHandler)
	router.HandleFunc("DELETE /api/v1/bee-name-generator/suggestion", beenamegenerator.RejectBeeNameSuggestionHandler)
	router.HandleFunc("DELETE /api/v1/bee-name-generator/suggestion/{name}", beenamegenerator.RejectBeeNameSuggestionHandler)

	// -------------- MC Status --------------
	router.HandleFunc("GET /api/v1/mcstatus", mcstatus.GetRoot)
	router.HandleFunc("GET /api/v1/mcstatus/{address}", mcstatus.GetServerStatus)
	router.HandleFunc("GET /api/v1/mcstatus/icon/{address}", mcstatus.GetIcon)

	// -------------- Projects --------------
	router.HandleFunc("GET /api/v1/projects/releases/{group}/{project}", projects.GetReleasesHandler)

	// -------------- Switchboard --------------
	// router.HandleFunc("GET /ws/v1/switchboard/relay", switchboard.WebSocketRelayHandler)
	router.HandleFunc("GET /websocket/:id", switchboard.WebSocketRelayHandler)

	server := http.Server{
		Addr:    ip + ":" + port,
		Handler: router,
	}
	log.Fatal(server.ListenAndServe())

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
