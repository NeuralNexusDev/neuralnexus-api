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
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    T      `json:"data,omitempty"`
}

// -------------- Functions --------------
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
