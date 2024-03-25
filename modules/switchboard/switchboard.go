package switchboard

import (
	"encoding/json"
	"log"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/encryption"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

// -------------- Globals --------------
var (
	upgrader = websocket.Upgrader{}
)

// -------------- Structs --------------

type Message struct {
	Sender             MessageSender     `json:"sender"`
	Channel            MessageType       `json:"channel"`
	Message            string            `json:"message"`
	TimeStamp          int64             `json:"timestamp"`
	PlaceHolderMessage string            `json:"placeHolderMessage"`
	PlaceHolders       map[string]string `json:"placeholders"`
	IsRemote           bool              `json:"isRemote"`
	IsGlobal           bool              `json:"isGlobal"`
}

type MessageSender struct {
	Name        string       `json:"name"`
	Prefix      string       `json:"prefix"`
	Suffix      string       `json:"suffix"`
	DisplayName string       `json:"displayName"`
	UUID        string       `json:"uuid"`
	Server      SimpleServer `json:"server"`
}

type SimpleServer struct {
	Name string `json:"name"`
}

// -------------- Enums --------------

// MessageType
type MessageType string

var (
	MessageTypeMap = map[string]string{
		"PLAYER_ADVANCEMENT_FINISHED": "tc:p_adv_fin",
		"PLAYER_DEATH":                "tc:p_death",
		"PLAYER_LOGIN":                "tc:p_login",
		"PLAYER_LOGOUT":               "tc:p_logout",
		"PLAYER_MESSAGE":              "tc:p_msg",
		"SERVER_STARTED":              "tc:s_start",
		"SERVER_STOPPED":              "tc:s_stop",
		"CUSTOM":                      "tc:custom",
	}

	MessageTypeMapReverse = map[string]string{
		"tc:p_adv_fin": "PLAYER_ADVANCEMENT_FINISHED",
		"tc:p_death":   "PLAYER_DEATH",
		"tc:p_login":   "PLAYER_LOGIN",
		"tc:p_logout":  "PLAYER_LOGOUT",
		"tc:p_msg":     "PLAYER_MESSAGE",
		"tc:s_start":   "SERVER_STARTED",
		"tc:s_stop":    "SERVER_STOPPED",
		"tc:custom":    "CUSTOM",
	}
)

func (mt MessageType) String() string {
	if val, ok := MessageTypeMapReverse[string(mt)]; ok {
		return val
	}
	return "CUSTOM"
}

// -------------- Functions --------------

// EncryptMessage encrypts a message
func EncryptMessage(message Message, key string) ([]byte, error) {
	// Convert message to JSON
	messageJSON, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}

	// Encrypt message
	encryptedMessage, err := encryption.EncryptAES(messageJSON, key)
	if err != nil {
		return nil, err
	}

	return encryptedMessage, nil
}

// DecryptMessage decrypts a message
func DecryptMessage(encryptedMessage []byte, key string) (Message, error) {
	// Decrypt message
	decryptedMessage, err := encryption.DecryptAES(encryptedMessage, key)

	if err != nil {
		return Message{}, err
	}

	// Take off the padding at the front of the byte array
	// Thanks google common for having nice byte buffers that add padding,
	// In which case I forgot about it and it took me 2 hours to figure out why the message was not decrypting
	// TODO: Convert Java-side to just plain ol byte arrays with no fancy helper classes
	decryptedMessage = decryptedMessage[2:]

	// Convert message from JSON
	var message Message
	err = json.Unmarshal(decryptedMessage, &message)
	if err != nil {
		return Message{}, err
	}

	return message, nil
}

// -------------- Handlers --------------

// WebSocketRelayHandler relays switchboard messages
func WebSocketRelayHandler(c echo.Context) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	for {
		// Write
		message := Message{
			Sender: MessageSender{
				Name:        "NeuralNexus",
				Prefix:      "§7",
				Suffix:      "§r",
				DisplayName: "NeuralNexus",
				UUID:        "00000000-0000-0000-0000-000000000000",
				Server: SimpleServer{
					Name: "NeuralNexus",
				},
			},
			Channel: MessageType("PLAYER_MESSAGE"),
			Message: "Hello, world!",
		}
		encryptedMessage, err := EncryptMessage(message, "dgwjgsemfouvauxc")
		if err != nil {
			log.Println(err.Error())
		}

		err = ws.WriteMessage(websocket.BinaryMessage, encryptedMessage)
		if err != nil {
			log.Println(err.Error())
		}

		// Read
		msgType, msg, err := ws.ReadMessage()

		if err != nil {
			log.Println(err.Error())
		} else if msgType != websocket.BinaryMessage {
			log.Println("Message type is not binary")
		}

		message, err = DecryptMessage(msg, "dgwjgsemfouvauxc")
		if err != nil {
			log.Println(err.Error())
		}
		log.Println(message.Message)
	}
}
