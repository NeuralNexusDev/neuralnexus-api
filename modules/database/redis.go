package database

import (
	"context"
	"log"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/redis/go-redis/v9"
)

func GetRedis(cfg config.Redis) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Address,
		Username: cfg.Username,
		Password: cfg.Password,
		DB:       0,
	})

	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		log.Fatal("Unable to create client:", err)
		return nil
	}
	return client
}
