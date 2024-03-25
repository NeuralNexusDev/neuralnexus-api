package economy

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// -------------- Structs --------------

type EcoUser struct {
	ID         string         `json:"_id"`        // Internal ID
	UserID     string         `json:"userID"`     // User ID
	Currencies map[string]int `json:"currencies"` // Currencies
	Owned      []string       `json:"owned"`      // Owned currencies
}

// -------------- Enums --------------

// -------------- Functions --------------

// Get User from DB
func getUserFromDB(userID string) (EcoUser, error) {
	var result bson.M
	coll := database.MongoClient.Database("economy-db").Collection("currencies")
	err := coll.FindOne(context.TODO(), bson.D{{Key: "userID", Value: userID}}).Decode(&result)
	if err == mongo.ErrNoDocuments {
		return EcoUser{}, errors.New("user not found")
	} else if err != nil {
		fmt.Println(err.Error())
		return EcoUser{}, errors.New("internal server error")
	}

	var user EcoUser
	bsonBytes, _ := bson.Marshal(result)
	err = bson.Unmarshal(bsonBytes, &user)
	if err != nil {
		fmt.Println(err.Error())
		return EcoUser{}, errors.New("internal server error")
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
	coll := database.MongoClient.Database("economy-db").Collection("currencies")
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
	coll := database.MongoClient.Database("economy-db").Collection("currencies")
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
func GetCurrencies(c *gin.Context) {
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
func GetCurrency(c *gin.Context) {
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
func UpdateCurrency(c *gin.Context) {
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
func GetOwnedCurrencies(c *gin.Context) {
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
func CreateCurrency(c *gin.Context) {
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
