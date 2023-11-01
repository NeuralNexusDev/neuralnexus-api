package authentication

import (
	"context"
	"errors"
	"fmt"
	"neuralnexus-api/modules/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// -------------- Structs --------------

type Account struct {
	ID           string   `json:"_id"`          // Internal ID
	UserId       string   `json:"userId"`       // User ID
	Username     string   `json:"username"`     // Username
	Email        string   `json:"email"`        // Email
	HashedSecret string   `json:"hashedSecret"` // Hashed secret
	Tokens       []string `json:"tokens"`       // Tokens
}

// -------------- Enums --------------

// -------------- Functions --------------

// Get User from DB
func getUserFromDB(userID string) (Account, error) {
	var result bson.M
	coll := database.MongoClient.Database("authentication-db").Collection("accounts")
	err := coll.FindOne(context.TODO(), bson.D{{Key: "userId", Value: userID}}).Decode(&result)
	if err == mongo.ErrNoDocuments {
		return Account{}, errors.New("user not found")
	} else if err != nil {
		fmt.Println(err.Error())
		return Account{}, errors.New("internal server error")
	}

	var user Account
	bsonBytes, _ := bson.Marshal(result)
	err = bson.Unmarshal(bsonBytes, &user)
	if err != nil {
		fmt.Println(err.Error())
		return Account{}, errors.New("internal server error")
	}

	return user, nil
}

// Validate user
func validateUser(userID string, secret string) (bool, error) {
	user, err := getUserFromDB(userID)
	if err != nil {
		return false, err
	}

	return user.HashedSecret == secret, nil
}

// Validate token
func validateToken(userID string, token string) (bool, error) {
	user, err := getUserFromDB(userID)
	if err != nil {
		return false, err
	}

	for _, t := range user.Tokens {
		if t == token {
			return true, nil
		}
	}

	return false, nil
}

// -------------- Handlers --------------
