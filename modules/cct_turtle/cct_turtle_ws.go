package cct_turtle

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

// -------------- Globals --------------
var (
	upgrader = websocket.Upgrader{}

	pongTimeout = 55 * time.Second

	// A map of InstructionQueues
	queue = make(map[string][]Instruction)
)

// -------------- Structs --------------

// Instruction - An instruction
type Instruction struct {
	Label string `json:"label"`
	Func  string `json:"func"`
}

// -------------- Functions --------------

// AddInstruction - Add an instruction to the queue
func AddInstruction(label string, instruction Instruction) {
	// If the queue doesn't exist, create it
	if _, ok := queue[label]; !ok {
		queue[label] = make([]Instruction, 0)
	}

	// Add the instruction to the queue
	queue[label] = append(queue[label], instruction)
}

// GetInstruction - Get the next instruction from the queue
func GetInstruction(label string) Instruction {
	// If the queue doesn't exist, return an empty instruction
	if _, ok := queue[label]; !ok {
		return Instruction{}
	}

	// Get the instruction from the queue
	instruction := queue[label][0]

	// Remove the instruction from the queue
	queue[label] = queue[label][1:]

	return instruction
}

// RemoveInstruction - Remove an instruction from the queue
func RemoveInstruction(label string, index int) {
	// If the queue doesn't exist, return
	if _, ok := queue[label]; !ok {
		return
	}

	// Remove the instruction from the queue
	queue[label] = append(queue[label][:index], queue[label][index+1:]...)
}

// -------------- Handlers --------------

func WebSocketTurtleHandler(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	_ = ws.SetWriteDeadline(time.Now().Add(pongTimeout))
	ws.SetPongHandler(func(string) error {
		err = ws.SetWriteDeadline(time.Now().Add(pongTimeout))
		if err != nil {
			log.Println(err.Error())
		}
		return nil
	})

	for {
		// Write
		message := "{\"label\":\"" + c.Param("id") + "\",\"func\":\"return " + "turtle.forward()" + "\"}"
		err = ws.WriteMessage(websocket.TextMessage, []byte(message))
		if err != nil {
			return err
		}

		// Read
		msgType, msg, err := ws.ReadMessage()
		if err != nil {
			log.Println(err.Error())
		} else if msgType != websocket.TextMessage {
			log.Println("Message type is not text")
		}
		// Print the message
		log.Println(string(msg))
	}
}
