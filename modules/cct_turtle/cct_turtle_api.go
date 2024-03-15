package cct_turtle

import (
	"encoding/json"
	"time"

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

// GetTurtleUpdatingCode - Get the turtle updating code
func GetTurtleUpdatingCode(c echo.Context) error {
	return c.File("static/cct_turtle/updating_startup.lua")
}

// GetTurtleStatus - Get the turtle status
func GetTurtleStatus(c echo.Context) error {
	return c.JSON(200, Turtle{
		Label:    "Turtle",
		ID:       1,
		Fuel:     "100",
		Position: "0, 0, 0",
		Facing:   "North",
	})
}

// TurtleHelper - The turtle helper
func TurtleHelper(c echo.Context, function string) error {
	label := c.Param("label")
	if label == "" {
		label = c.QueryParam("label")
	}

	Queue.AddNewInstruction(label, function)
	Queue.SendInstruction(label)
	var retries int = 0
	for !Queue.GetStatus(label) && retries < 100 {
		time.Sleep(30 * time.Millisecond)
		retries++
	}
	var status TurtleStatus
	err := json.Unmarshal([]byte(Queue.GetResponse(label)), &status)
	if err != nil {
		return c.JSON(500, err.Error())
	}
	Queue.RemoveInstruction(label, 0)
	return c.JSON(200, status)
}

// MoveTurtleForward - Move the turtle forward
func MoveTurtleForward(c echo.Context) error {
	return TurtleHelper(c, "turtle.forward()")
}

// MoveTurtleBackward - Move the turtle backward
func MoveTurtleBackward(c echo.Context) error {
	return TurtleHelper(c, "turtle.back()")
}

// MoveTurtleUp - Move the turtle up
func MoveTurtleUp(c echo.Context) error {
	return TurtleHelper(c, "turtle.up()")
}

// MoveTurtleDown - Move the turtle down
func MoveTurtleDown(c echo.Context) error {
	return TurtleHelper(c, "turtle.down()")
}

// TurnTurtleLeft - Turn the turtle left
func TurnTurtleLeft(c echo.Context) error {
	return TurtleHelper(c, "turtle.turnLeft()")
}

// TurnTurtleRight - Turn the turtle right
func TurnTurtleRight(c echo.Context) error {
	return TurtleHelper(c, "turtle.turnRight()")
}

// DigTurtle - Dig with the turtle
func DigTurtle(c echo.Context) error {
	return TurtleHelper(c, "turtle.dig()")
}

// DigTurtleUp - Dig up with the turtle
func DigTurtleUp(c echo.Context) error {
	return TurtleHelper(c, "turtle.digUp()")
}

// DigTurtleDown - Dig down with the turtle
func DigTurtleDown(c echo.Context) error {
	return TurtleHelper(c, "turtle.digDown()")
}
