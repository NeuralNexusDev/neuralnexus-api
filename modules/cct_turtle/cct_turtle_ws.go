package cct_turtle

import (
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
	}
}
