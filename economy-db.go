package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// -------------- Global Variables --------------
var mongoClient *mongo.Client

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
	mongoClient, err = mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		panic(err)
	}

	// Create router
	var router *gin.Engine = gin.Default()

	// Routes
	router.GET("/:userID/currencies", getCurrencies)
	router.GET("/:userID/currencies/:currencyID", getCurrency)
	router.POST("/:userID/currencies/:currencyID/:ammount", updateCurrency)
	router.GET("/:userID/owned", getOwnedCurrencies)
	router.PUT("/:userID/owned/:currencyID", createCurrency)

	router.Run(ip + ":" + port)
}

// -------------- Structs --------------

type User struct {
	ID         string         `json:"_id"`
	UserID     string         `json:"userID"`
	Currencies map[string]int `json:"currencies"`
	Owned      []string       `json:"owned"`
}

// -------------- Enums --------------

// -------------- Functions --------------

// Get User from DB
func getUserFromDB(userID string) (User, error) {
	var result bson.M
	coll := mongoClient.Database("economy-db").Collection("currencies")
	err := coll.FindOne(context.TODO(), bson.D{{Key: "userID", Value: userID}}).Decode(&result)
	if err == mongo.ErrNoDocuments {
		return User{}, errors.New("user not found")
	} else if err != nil {
		fmt.Println(err.Error())
		return User{}, errors.New("internal server error")
	}

	var user User
	bsonBytes, _ := bson.Marshal(result)
	err = bson.Unmarshal(bsonBytes, &user)
	if err != nil {
		fmt.Println(err.Error())
		return User{}, errors.New("internal server error")
	}

	return user, nil
}

// Get User's currencies from DB
func getCurrenciesFromDB(userID string) ([]string, error) {
	user, err := getUserFromDB(userID)
	if err != nil {
		return []string{}, err
	}

	// Get the currency IDs into an array
	var currencyIDs []string
	for currencyID := range user.Currencies {
		currencyIDs = append(currencyIDs, currencyID)
	}
	return currencyIDs, nil
}

// Get User's ammount of a currency from DB
func getCurrencyFromDB(userID string, currencyID string) (int, error) {
	user, err := getUserFromDB(userID)
	if err != nil {
		return 0, err
	}

	// Get the currency ammount
	currencyAmmount := user.Currencies[currencyID]
	return currencyAmmount, nil
}

// Update User's ammount of a currency in DB
func updateCurrencyInDB(userID string, currencyID string, currencyAmmount int) error {
	user, err := getUserFromDB(userID)
	if err != nil {
		return err
	}
	user.Currencies[currencyID] += currencyAmmount

	// Update the currency ammount
	filter := bson.D{{Key: "userID", Value: userID}}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "currencies", Value: user.Currencies}}}}
	coll := mongoClient.Database("economy-db").Collection("currencies")
	updateResult, err := coll.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		fmt.Println(err.Error())
		return errors.New("internal server error")
	}
	fmt.Printf("Matched %v documents and updated %v documents.\n", updateResult.MatchedCount, updateResult.ModifiedCount)
	return nil
}

// Get User's owned currencies from DB
func getOwnedCurrenciesFromDB(userID string) ([]string, error) {
	user, err := getUserFromDB(userID)
	if err != nil {
		return []string{}, err
	}
	return user.Owned, nil
}

// Create new Currency for User in DB
func createCurrencyInDB(userID string, currencyID string) error {
	user, err := getUserFromDB(userID)
	if err != nil {
		return err
	}

	// Check if the currency already exists
	for _, ownedCurrency := range user.Owned {
		if ownedCurrency == currencyID {
			return errors.New("currency already exists")
		}
	}

	// Add the currency to the user's owned currencies
	user.Owned = append(user.Owned, currencyID)

	// Update the user's owned currencies
	filter := bson.D{{Key: "userID", Value: userID}}
	update := bson.D{{Key: "$set", Value: bson.D{{Key: "owned", Value: user.Owned}}}}
	coll := mongoClient.Database("economy-db").Collection("currencies")
	updateResult, err := coll.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		fmt.Println(err.Error())
		return errors.New("internal server error")
	}
	fmt.Printf("Matched %v documents and updated %v documents.\n", updateResult.MatchedCount, updateResult.ModifiedCount)
	return nil
}

// -------------- Handlers --------------

// Get User's currencies
func getCurrencies(c *gin.Context) {
	userID := c.Param("userID")

	currencies, err := getCurrenciesFromDB(userID)
	if err == nil {
		c.JSON(200, gin.H{
			"success":    true,
			"currencies": currencies,
		})
	} else if err.Error() == "user not found" {
		c.JSON(404, gin.H{
			"success": false,
			"error":   "user not found",
		})
		return
	} else if err.Error() == "internal server error" {
		c.JSON(500, gin.H{
			"success": false,
			"error":   "internal server error",
		})
		return
	}
}

// Get User's ammount of a currency
func getCurrency(c *gin.Context) {
	userID := c.Param("userID")
	currencyID := c.Param("currencyID")

	currencyAmmount, err := getCurrencyFromDB(userID, currencyID)
	if err == nil {
		c.JSON(200, gin.H{
			"success": true,
			"ammount": currencyAmmount,
		})
	} else if err.Error() == "user not found" {
		c.JSON(404, gin.H{
			"success": false,
			"error":   "user not found",
		})
		return
	} else if err.Error() == "internal server error" {
		c.JSON(500, gin.H{
			"success": false,
			"error":   "internal server error",
		})
		return
	}
}

// TODO: Add authentication
// Update User's ammount of a currency
func updateCurrency(c *gin.Context) {
	userID := c.Param("userID")
	currencyID := c.Param("currencyID")
	currencyAmmount, err := strconv.Atoi(c.Param("ammount"))
	if err != nil {
		c.JSON(400, gin.H{
			"success": false,
			"error":   "invalid ammount",
		})
		return
	}

	err = updateCurrencyInDB(userID, currencyID, currencyAmmount)
	if err == nil {
		c.JSON(200, gin.H{
			"success": true,
		})
	} else if err.Error() == "user not found" {
		c.JSON(404, gin.H{
			"success": false,
			"error":   "user not found",
		})
		return
	} else if err.Error() == "internal server error" {
		c.JSON(500, gin.H{
			"success": false,
			"error":   "internal server error",
		})
		return
	}
}

// Get User's owned currencies
func getOwnedCurrencies(c *gin.Context) {
	userID := c.Param("userID")

	ownedCurrencies, err := getOwnedCurrenciesFromDB(userID)
	if err == nil {
		c.JSON(200, gin.H{
			"success":         true,
			"ownedCurrencies": ownedCurrencies,
		})
	} else if err.Error() == "user not found" {
		c.JSON(404, gin.H{
			"success": false,
			"error":   "user not found",
		})
		return
	} else if err.Error() == "internal server error" {
		c.JSON(500, gin.H{
			"success": false,
			"error":   "internal server error",
		})
		return
	}
}

// TODO: Add authentication
// Create new Currency for User
func createCurrency(c *gin.Context) {
	userID := c.Param("userID")
	currencyID := c.Param("currencyID")

	err := createCurrencyInDB(userID, currencyID)
	if err == nil {
		c.JSON(200, gin.H{
			"success": true,
		})
	} else if err.Error() == "user not found" {
		c.JSON(404, gin.H{
			"success": false,
			"error":   "user not found",
		})
		return
	} else if err.Error() == "internal server error" {
		c.JSON(500, gin.H{
			"success": false,
			"error":   "internal server error",
		})
		return
	} else if err.Error() == "currency already exists" {
		c.JSON(400, gin.H{
			"success": false,
			"error":   "currency already exists",
		})
		return
	}
}
