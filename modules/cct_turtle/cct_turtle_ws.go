package cct_turtle

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
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

type InstructionState string

const (
	Waiting    InstructionState = "waiting"
	InProgress InstructionState = "in_progress"
	Complete   InstructionState = "complete"
)

// Instruction - An instruction
type Instruction struct {
	Label    string           `json:"label"`
	Func     string           `json:"func"`
	State    InstructionState `json:"state"`
	Response string           `json:"response,omitempty"`
}

// NewInstruction - Create a new instruction
func NewInstruction(label string, function string) Instruction {
	return Instruction{
		Label:    label,
		Func:     function,
		State:    Waiting,
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

// GetState - Get the state of the instruction
func (iq *InstructionQueue) GetState(label string) InstructionState {
	// If the queue doesn't exist, return an empty string
	if _, ok := iq.queue[label]; !ok {
		return ""
	}

	// Get the state of the instruction
	if len(iq.queue[label]) == 0 {
		return ""
	}

	return iq.queue[label][0].State
}

// SetState - Set the state of the instruction
func (iq *InstructionQueue) SetState(label string, state InstructionState) {
	// If the queue doesn't exist, return
	if _, ok := iq.queue[label]; !ok {
		return
	}

	// Set the state of the instruction
	if len(iq.queue[label]) == 0 {
		return
	}

	iq.queue[label][0].State = state
}

// GetResponse - Get the response of the instruction
func (iq *InstructionQueue) GetResponse(label string) (TurtleStatus, error) {
	// If the queue doesn't exist, return an empty string
	if _, ok := iq.queue[label]; !ok {
		return TurtleStatus{}, errors.New("queue does not exist")
	}

	// Get the response of the instruction
	if len(iq.queue[label]) == 0 {
		return TurtleStatus{}, errors.New("queue is empty")
	}

	var status TurtleStatus
	err := json.Unmarshal([]byte(iq.queue[label][0].Response), &status)
	if err != nil {
		return TurtleStatus{}, err
	}

	return status, nil
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

func WebSocketTurtleHandler(w http.ResponseWriter, r *http.Request) {
	label := r.PathValue("label")

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer RemoveWebSocket(label)
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
		msgType, msg, err := ws.ReadMessage()
		if err != nil {
			log.Println(err.Error())
			return
		} else if msgType != websocket.TextMessage {
			log.Println("Message type is not text")
		}

		// TODO: update turtle status in DB

		// Get the instruction from the queue
		Queue.SetState(label, Complete)
		Queue.SetResponse(label, string(msg))
	}
}
