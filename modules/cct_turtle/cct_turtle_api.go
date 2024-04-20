package cctturtle

import (
	"net/http"
	"time"

	"github.com/goccy/go-json"
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

// -------------- Routes --------------
// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	mux.HandleFunc("GET /api/v1/ws/v1/cct-turtle/{label}", WebSocketTurtleHandler)
	// e.GET("/api/v1/cct-turtle/status", GetTurtleStatus)
	// e.GET("/api/v1/cct-turtle/status/:label", GetTurtleStatus)
	mux.HandleFunc("GET /api/v1/cct-turtle/startup.lua", GetTurtleCode)
	mux.HandleFunc("GET /api/v1/cct-turtle/updating_startup.lua", GetTurtleUpdatingCode)
	mux.HandleFunc("GET /api/v1/cct-turtle/forward", MoveTurtleForward)
	mux.HandleFunc("GET /api/v1/cct-turtle/forward/{label}", MoveTurtleForward)
	mux.HandleFunc("GET /api/v1/cct-turtle/back", MoveTurtleBackward)
	mux.HandleFunc("GET /api/v1/cct-turtle/back/{label}", MoveTurtleBackward)
	mux.HandleFunc("GET /api/v1/cct-turtle/up", MoveTurtleUp)
	mux.HandleFunc("GET /api/v1/cct-turtle/up/{label}", MoveTurtleUp)
	mux.HandleFunc("GET /api/v1/cct-turtle/down", MoveTurtleDown)
	mux.HandleFunc("GET /api/v1/cct-turtle/down/{label}", MoveTurtleDown)
	mux.HandleFunc("GET /api/v1/cct-turtle/left", TurnTurtleLeft)
	mux.HandleFunc("GET /api/v1/cct-turtle/left/{label}", TurnTurtleLeft)
	mux.HandleFunc("GET /api/v1/cct-turtle/right", TurnTurtleRight)
	mux.HandleFunc("GET /api/v1/cct-turtle/right/{label}", TurnTurtleRight)
	mux.HandleFunc("GET /api/v1/cct-turtle/dig", DigTurtle)
	mux.HandleFunc("GET /api/v1/cct-turtle/dig/{label}", DigTurtle)
	mux.HandleFunc("GET /api/v1/cct-turtle/dig-up", DigTurtleUp)
	mux.HandleFunc("GET /api/v1/cct-turtle/dig-up/{label}", DigTurtleUp)
	mux.HandleFunc("GET /api/v1/cct-turtle/dig-down", DigTurtleDown)
	mux.HandleFunc("GET /api/v1/cct-turtle/dig-down/{label}", DigTurtleDown)
	return mux
}

// GetTurtleCode - Get the turtle code
func GetTurtleCode(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "public/cct_turtle/startup.lua")
}

// GetTurtleUpdatingCode - Get the turtle updating code
func GetTurtleUpdatingCode(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "public/cct_turtle/updating_startup.lua")
}

// TODO: pull from DB
// GetTurtleStatus - Get the turtle status
func GetTurtleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	turtleStatus := Turtle{
		Label:    "Turtle",
		ID:       1,
		Fuel:     "100",
		Position: "0, 0, 0",
		Facing:   "North",
	}
	json.NewEncoder(w).Encode(turtleStatus)
}

// TurtleHelper - The turtle helper
func TurtleHelper(w http.ResponseWriter, r *http.Request, function string) {
	label := r.URL.Query().Get("label")
	if label == "" {
		label = r.PathValue("label")
	}

	Queue.AddNewInstruction(label, function)
	Queue.SendInstruction(label)
	var retries int = 0
	for Queue.GetState(label) != Complete && retries < 100 {
		time.Sleep(30 * time.Millisecond)
		retries++
	}
	status, err := Queue.GetResponse(label)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	Queue.RemoveInstruction(label, 0)
	json.NewEncoder(w).Encode(status)
}

// MoveTurtleForward - Move the turtle forward
func MoveTurtleForward(w http.ResponseWriter, r *http.Request) {
	TurtleHelper(w, r, "turtle.forward()")
}

// MoveTurtleBackward - Move the turtle backward
func MoveTurtleBackward(w http.ResponseWriter, r *http.Request) {
	TurtleHelper(w, r, "turtle.back()")
}

// MoveTurtleUp - Move the turtle up
func MoveTurtleUp(w http.ResponseWriter, r *http.Request) {
	TurtleHelper(w, r, "turtle.up()")
}

// MoveTurtleDown - Move the turtle down
func MoveTurtleDown(w http.ResponseWriter, r *http.Request) {
	TurtleHelper(w, r, "turtle.down()")
}

// TurnTurtleLeft - Turn the turtle left
func TurnTurtleLeft(w http.ResponseWriter, r *http.Request) {
	TurtleHelper(w, r, "turtle.turnLeft()")
}

// TurnTurtleRight - Turn the turtle right
func TurnTurtleRight(w http.ResponseWriter, r *http.Request) {
	TurtleHelper(w, r, "turtle.turnRight()")
}

// DigTurtle - Dig with the turtle
func DigTurtle(w http.ResponseWriter, r *http.Request) {
	TurtleHelper(w, r, "turtle.dig()")
}

// DigTurtleUp - Dig up with the turtle
func DigTurtleUp(w http.ResponseWriter, r *http.Request) {
	TurtleHelper(w, r, "turtle.digUp()")
}

// DigTurtleDown - Dig down with the turtle
func DigTurtleDown(w http.ResponseWriter, r *http.Request) {
	TurtleHelper(w, r, "turtle.digDown()")
}
