package authentication

import (
	"context"
	"errors"
	"fmt"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

// -------------- Constants --------------

var JwtKey string

// -------------- Structs --------------

// Account struct
type Account struct {
	ID           string `json:"_id"`          // Internal ID
	UserId       string `json:"userId"`       // User ID
	Username     string `json:"username"`     // Username
	Email        string `json:"email"`        // Email
	HashedSecret string `json:"hashedSecret"` // Hashed secret
}

// -------------- Enums --------------

// -------------- Functions --------------

// Get User from DB
func GetUserFromDB(userID string) (Account, error) {
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

// Save User to DB
func (user *Account) SaveUserToDB() error {
	coll := database.MongoClient.Database("authentication-db").Collection("accounts")
	_, err := coll.InsertOne(context.TODO(), bson.D{
		{Key: "userId", Value: user.UserId},
		{Key: "username", Value: user.Username},
		{Key: "email", Value: user.Email},
		{Key: "hashedSecret", Value: user.HashedSecret},
	})
	if err != nil {
		fmt.Println(err.Error())
		return errors.New("internal server error")
	}

	return nil
}

// Hash password
func (user *Account) HashPassword(password string) error {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		return err
	}
	user.HashedSecret = string(bytes)
	return nil
}

// Validate password
func (user *Account) ValidateUser(password string) error {
	err := bcrypt.CompareHashAndPassword([]byte(user.HashedSecret), []byte(password))
	if err != nil {
		return err
	}
	return nil
}

// Create token
func (user *Account) CreateToken() (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"userId": user.UserId,
	})
	tokenString, err := token.SignedString([]byte(JwtKey))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

// Validate token
func ValidateToken(token string) (bool, error) {
	_, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(JwtKey), nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// -------------- Handlers --------------
