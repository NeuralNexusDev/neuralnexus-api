package database

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

// -------------- Globals --------------
var DATABASE_URL = os.Getenv("DATABASE_URL")

// -------------- Structs --------------

// Generic response struct
type Response[T any] struct {
	Success bool
	Message string
	Data    T
}

// SuccessResponse - Create a new success response
func SuccessResponse[T any](data T) Response[T] {
	return Response[T]{
		Success: true,
		Data:    data,
	}
}

// ErrorResponse - Create a new error response
func ErrorResponse[T any](message string, err error) Response[T] {
	log.Println("[Error]: " + message + ":\n\t" + err.Error())
	return Response[T]{
		Success: false,
		Message: message,
	}
}

// -------------- Functions --------------

// GetDB - Get a connection pool to the database
func GetDB(database string) *pgxpool.Pool {
	if DATABASE_URL == "" {
		log.Println("DATABASE_URL is not set")
		return nil
	}

	PgPool, err := pgxpool.New(context.Background(), DATABASE_URL+"/"+database)
	if err != nil {
		log.Println("Unable to create connection pool:", err)
		return nil
	}
	return PgPool
}
