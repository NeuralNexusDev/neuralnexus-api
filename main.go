package main

import (
	"log"
	"os"
)

func main() {
	ip := os.Getenv("IP_ADDRESS")
	if ip == "" {
		ip = "0.0.0.0"
	}
	port := os.Getenv("REST_PORT")
	if port == "" {
		port = "8080"
	}

	server := NewAPIServer(ip + ":" + port)
	log.Fatal(server.Run())
}
