package cct_turtle

import (
	"log"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

// -------------- Globals --------------
var (
	upgrader = websocket.Upgrader{}
)

// -------------- Functions --------------

// -------------- Handlers --------------

func WebSocketTurtleHandler(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	for {
		// Write
		message := "Hello, world!"
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
