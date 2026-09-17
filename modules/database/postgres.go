package database

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// -------------- Functions --------------

// GetDB - Get a connection pool to the database
func GetDB(database string) *pgxpool.Pool {
	PgPool, err := pgxpool.New(context.Background(), database)
	if err != nil {
		log.Fatal("Unable to create connection pool:", err)
		return nil
	}
	return PgPool
}
