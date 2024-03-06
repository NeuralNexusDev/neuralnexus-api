package beenamegenerator

import (
	"context"
	"neuralnexus-api/modules/database"

	"github.com/labstack/echo/v4"
)

// -------------- Globals --------------

// -------------- Structs --------------

// -------------- Functions --------------

// getBeeName returns a random bee name from the database
func getBeeName() database.Response[string] {
	db := database.GetDB("bee_name_generator")
	var beeName string

	err := db.QueryRow(context.Background(), "SELECT name FROM bee_name ORDER BY random() LIMIT 1").Scan(&beeName)
	if err != nil {
		return database.Response[string]{
			Success: false,
			Message: "Failed to get bee name: " + err.Error(),
		}
	}

	return database.Response[string]{
		Success: true,
		Data:    beeName,
	}
}

// -------------- Handlers --------------

// GetBeeNameHandler
func GetBeeNameHandler(c echo.Context) error {
	beeName := getBeeName()
	if !beeName.Success {
		return c.JSON(500, beeName)
	}
	return c.JSON(200, beeName)
}
