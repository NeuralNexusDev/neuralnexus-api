package main

import (
	"neuralnexus-api/modules/beenamegenerator"
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

	// Bee Name Generator
	// e.GET("/api/v1/bee-name-generator", beenamegenerator.GetBeeNameHandler)
	e.GET("/api/v1/bee-name-generator/name", beenamegenerator.GetBeeNameHandler)
	e.POST("/api/v1/bee-name-generator/name", beenamegenerator.Not)

	// MC Status
	e.GET("/api/v1/mcstatus", mcstatus.GetRoot)
	e.GET("/api/v1/mcstatus/:address", mcstatus.GetServerStatus)
	e.GET("/api/v1/mcstatus/icon/:address", mcstatus.GetIcon)

	// Projects
	e.GET("/api/v1/projects/releases/:group/:project", projects.GetReleasesHandler)

	// Switchboard
	// e.GET("/ws/v1/switchboard/relay", switchboard.WebSocketRelayHandler)
	e.GET("/websocket/:id", switchboard.WebSocketRelayHandler)

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
