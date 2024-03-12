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
)

// -------------- Structs --------------

// Instruction - An instruction
type Instruction struct {
	Label string `json:"label"`
	Func  string `json:"func"`
}

// InstructionQueue - The instruction queue
type InstructionQueue struct {
	Instructions []Instruction
}

// -------------- Functions --------------

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
