package switchboard

import (
	"log"
	"net/http"

	"github.com/goccy/go-json"
	"github.com/gorilla/websocket"
)

// -------------- Globals --------------
var (
	upgrader = websocket.Upgrader{}
)

// WebSocketRelayHandler relays switchboard messages
func WebSocketRelayHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer ws.Close()

	for {
		// Write
		packet := &Message{
			Version:     1,
			Origin:      "server",
			Dest:        "client",
			MessageID:   "1",
			MessageType: "message",
			Encrypted:   true,
			EncScheme:   "AES",
			Content:     "Hello, client!",
		}
		var packetBuffer []byte
		if packet.Encrypted {
			packetBuffer, err = EncryptMessage(packet, "dgwjgsemfouvauxc")
			if err != nil {
				log.Println(err.Error())
			}
		} else {
			packetBuffer, err = json.Marshal(packet)
			if err != nil {
				log.Println(err.Error())
			}
		}

		err = ws.WriteMessage(websocket.BinaryMessage, packetBuffer)
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

		packet, err = DecryptMessage(msg, "dgwjgsemfouvauxc")
		if err != nil {
			log.Println(err.Error())
		}
		log.Println(packet.Content)
	}
}
