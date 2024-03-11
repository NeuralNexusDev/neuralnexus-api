package cct_turtle

import (
	"github.com/labstack/echo/v4"
)

// -------------- Globals --------------

// -------------- Structs --------------

// -------------- Functions --------------

// -------------- Handlers --------------

// GetTurtleCode - Get the turtle code
func GetTurtleCode(c echo.Context) error {
	return c.File("static/cct_turtle/startup.lua")
}
