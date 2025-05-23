package database

import (
	"context"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

// -------------- Globals --------------

//goland:noinspection GoSnakeCaseUsage
var (
	REDIS_URL      = os.Getenv("REDIS_ADDRESS")
	REDIS_USERNAME = os.Getenv("REDIS_USERNAME")
	REDIS_PASSWORD = os.Getenv("REDIS_PASSWORD")
)

// -------------- Functions --------------

func GetRedis() *redis.Client {
	if REDIS_URL == "" {
		log.Fatal("REDIS_URL is not set")
		return nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     REDIS_URL,
		Username: REDIS_USERNAME,
		Password: REDIS_PASSWORD,
		DB:       0,
	})

	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		log.Fatal("Unable to create client:", err)
		return nil
	}
	return client
}
