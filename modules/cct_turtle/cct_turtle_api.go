package cct_turtle

import (
	"github.com/labstack/echo/v4"
)

// -------------- Globals --------------

// -------------- Structs --------------

// TurtleStatus - The status of the turtle
type TurtleStatus struct {
	Turtle    Turtle    `json:"turtle"`
	Blocks    Blocks    `json:"blocks"`
	Inventory Inventory `json:"inventory"`
}

// Turtle - The turtle
type Turtle struct {
	Label    string `json:"label"`
	ID       int    `json:"id"`
	Fuel     string `json:"fuel"`
	Position string `json:"position"`
	Facing   string `json:"facing"`
}

// Blocks - The blocks
type Blocks struct {
	Up    Block `json:"up"`
	Front Block `json:"front"`
	Down  Block `json:"down"`
}

// Block - The block
type Block struct {
	Name     string `json:"name"`
	Metadata string `json:"metadata"`
	State    string `json:"state"`
}

// Inventory - The inventory
type Inventory []Item

// Item - An item
type Item struct {
	Name   string `json:"name"`
	Damage string `json:"damage"`
	Count  string `json:"count"`
}

// -------------- Functions --------------

// -------------- Handlers --------------

// GetTurtleCode - Get the turtle code
func GetTurtleCode(c echo.Context) error {
	return c.File("static/cct_turtle/startup.lua")
}
