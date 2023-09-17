package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

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

	var router *gin.Engine = gin.Default()

	// Minecraft Server Status
	router.GET("/", getRoot)
	router.Static("/vendored", "./vendored")

	// Test
	router.GET("/test", getTest)

	router.Run(ip + ":" + port)
}

// -------------- Structs --------------

type PopVynil struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Number   int    `json:"number"`
}

// -------------- Enums --------------

// -------------- Functions --------------

// -------------- Handlers --------------

// Get root route
func getRoot(c *gin.Context) {
	// Read the html file
	html, err := os.ReadFile("templates/index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Return the html
	c.Data(http.StatusOK, "text/html", html)
}

// Returns test HTMX html inject
func getTest(c *gin.Context) {
	// Return the html
	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, "<div><h1>Whoop, Worked!</h1></div>")
}
