package main

import (
	"context"
	"log"
	"os"
	"pop-vinyl/modules/database"
	"pop-vinyl/modules/economy_db"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

	// Connect to MongoDB
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		log.Fatal("You must set your 'MONGODB_URI' environment variable. See\n\t https://www.mongodb.com/docs/drivers/go/current/usage-examples/#environment-variable")
	}
	var err error
	database.MongoClient, err = mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		panic(err)
	}

	// Create router
	var router *gin.Engine = gin.Default()

	// Routes
	// Economy DB
	router.GET("/:userID/currencies", economy_db.GetCurrencies)
	router.GET("/:userID/currencies/:currencyID", economy_db.GetCurrency)
	router.POST("/:userID/currencies/:currencyID/:ammount", economy_db.UpdateCurrency)
	router.GET("/:userID/owned", economy_db.GetOwnedCurrencies)
	router.PUT("/:userID/owned/:currencyID", economy_db.CreateCurrency)

	router.Run(ip + ":" + port)
}

// -------------- Structs --------------

// -------------- Enums --------------

// -------------- Functions --------------

// -------------- Handlers --------------
