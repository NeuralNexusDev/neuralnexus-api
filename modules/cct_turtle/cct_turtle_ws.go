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

	// websocketMap - A map of websockets
	websocketMap = make(map[string]*websocket.Conn)

	// Queue - A queue of instructions and their labels
	Queue = InstructionQueue{
		queue: make(map[string][]Instruction),
	}
)

// -------------- Structs --------------

// Instruction - An instruction
type Instruction struct {
	Label    string `json:"label"`
	Func     string `json:"func"`
	Status   bool   `json:"status"`
	Response string `json:"response"`
}

// NewInstruction - Create a new instruction
func NewInstruction(label string, function string) Instruction {
	return Instruction{
		Label:    label,
		Func:     function,
		Status:   false,
		Response: "",
	}
}

// InstructionQueue - A queue of instructions and their labels
type InstructionQueue struct {
	queue map[string][]Instruction
}

// AddNewInstruction - Add a new instruction to the queue
func (iq *InstructionQueue) AddNewInstruction(label string, function string) {
	iq.AddInstruction(label, NewInstruction(label, function))
}

// AddInstruction - Add an instruction to the queue
func (iq *InstructionQueue) AddInstruction(label string, instruction Instruction) {
	// If the queue doesn't exist, create it
	if _, ok := iq.queue[label]; !ok {
		iq.queue[label] = make([]Instruction, 0)
	}

	// Add the instruction to the queue
	iq.queue[label] = append(iq.queue[label], instruction)
}

// GetInstruction - Get the next instruction from the queue
func (iq *InstructionQueue) GetInstruction(label string) Instruction {
	// If the queue doesn't exist, return an empty instruction
	if _, ok := iq.queue[label]; !ok {
		return Instruction{}
	}

	// Get the instruction from the queue
	if len(iq.queue[label]) == 0 {
		return Instruction{}
	}
	instruction := iq.queue[label][0]

	// Remove the instruction from the queue
	iq.queue[label] = iq.queue[label][1:]

	return instruction
}

// RemoveInstruction - Remove an instruction from the queue
func (iq *InstructionQueue) RemoveInstruction(label string, index int) {
	// If the queue doesn't exist, return
	if _, ok := iq.queue[label]; !ok {
		return
	}

	// Remove the instruction from the queue
	if index < 0 || index >= len(iq.queue[label]) {
		return
	} else if len(iq.queue[label]) == 1 {
		iq.queue[label] = make([]Instruction, 0)
		return
	}
	iq.queue[label] = append(iq.queue[label][:index], iq.queue[label][index+1:]...)
}

// GetStatus - Get the status of the instruction
func (iq *InstructionQueue) GetStatus(label string) bool {
	// If the queue doesn't exist, return false
	if _, ok := iq.queue[label]; !ok {
		return false
	}

	// Get the status of the instruction
	if len(iq.queue[label]) == 0 {
		return false
	}
	return iq.queue[label][0].Status
}

// SetStatus - Set the status of the instruction
func (iq *InstructionQueue) SetStatus(label string, status bool) {
	// If the queue doesn't exist, return
	if _, ok := iq.queue[label]; !ok {
		return
	}

	// Set the status of the instruction
	if len(iq.queue[label]) == 0 {
		return
	}

	iq.queue[label][0].Status = status
}

// GetResponse - Get the response of the instruction
func (iq *InstructionQueue) GetResponse(label string) string {
	// If the queue doesn't exist, return an empty string
	if _, ok := iq.queue[label]; !ok {
		return ""
	}

	// Get the response of the instruction
	if len(iq.queue[label]) == 0 {
		return ""
	}
	return iq.queue[label][0].Response
}

// SetResponse - Set the response of the instruction
func (iq *InstructionQueue) SetResponse(label string, response string) {
	// If the queue doesn't exist, return
	if _, ok := iq.queue[label]; !ok {
		return
	}

	// Set the response of the instruction
	if len(iq.queue[label]) == 0 {
		return
	}

	iq.queue[label][0].Response = response
}

// SendInstruction - Send an instruction to the turtle
func (iq *InstructionQueue) SendInstruction(label string) {
	i := iq.GetInstruction(label)
	ws := GetWebSocket(i.Label)

	// TODO: Depricate this structure
	var instrJSON []byte = []byte("{\"label\":\"" + i.Label + "\",\"func\":\"" + i.Func + "\"}")

	if ws != nil {
		err := ws.WriteMessage(websocket.TextMessage, instrJSON)
		if err != nil {
			log.Println(err.Error())
		}
	}
}

// -------------- Functions --------------

// AddWebSocket - Add a websocket to the map
func AddWebSocket(label string, ws *websocket.Conn) {
	websocketMap[label] = ws
}

// GetWebSocket - Get a websocket from the map
func GetWebSocket(label string) *websocket.Conn {
	return websocketMap[label]
}

// RemoveWebSocket - Remove a websocket from the map
func RemoveWebSocket(label string) {
	delete(websocketMap, label)
}

// -------------- Handlers --------------

func WebSocketTurtleHandler(c echo.Context) error {
	label := c.Param("label")
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

	AddWebSocket(label, ws)

	for {
		// Read
		msgType, msg, err := ws.ReadMessage()
		if err != nil {
			log.Println(err.Error())
			return err
		} else if msgType != websocket.TextMessage {
			log.Println("Message type is not text")
		}

		// Get the instruction from the queue
		Queue.SetStatus(label, true)
		Queue.SetResponse(label, string(msg))

		// Print the message
		log.Println(string(msg))
	}
}
