package discord

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/goccy/go-json"
	"golang.org/x/crypto/ed25519"
)

type WebhookType int

const (
	PING  WebhookType = 0
	Event WebhookType = 1
)

type WebhookEvent struct {
	Version       int         `json:"version"`
	ApplicationId string      `json:"application_id"`
	Type          WebhookType `json:"type"`
	Event         *EventBody  `json:"event"`
}

type EventType string

const (
	ApplicationAuthorized   EventType = "APPLICATION_AUTHORIZED"     // Sent when an app was authorized by a user to a server or their account
	ApplicationDeauthorized EventType = "APPLICATION_DEAUTHORIZED"   // Sent when an app was deauthorized by a user
	EntitlementCreate       EventType = "ENTITLEMENT_CREATE"         // Entitlement was created
	EntitlementUpdate       EventType = "ENTITLEMENT_UPDATE"         // Entitlement was updated
	EntitlementDelete       EventType = "ENTITLEMENT_DELETE"         // Entitlement was deleted
	QuestUserEnrollment     EventType = "QUEST_USER_ENROLLMENT"      // User was added to a Quest (currently unavailable)
	LobbyMessageCreate      EventType = "LOBBY_MESSAGE_CREATE"       // Sent when a message is created in a lobby
	LobbyMessageUpdate      EventType = "LOBBY_MESSAGE_UPDATE"       // Sent when a message is updated in a lobby
	LobbyMessageDelete      EventType = "LOBBY_MESSAGE_DELETE"       // Sent when a message is deleted from a lobby
	GameDirectMessageCreate EventType = "GAME_DIRECT_MESSAGE_CREATE" // Sent when a direct message is created during an active Social SDK session
	GameDirectMessageUpdate EventType = "GAME_DIRECT_MESSAGE_UPDATE" // Sent when a direct message is updated during an active Social SDK session
	GameDirectMessageDelete EventType = "GAME_DIRECT_MESSAGE_DELETE" // Sent when a direct message is deleted during an active Social SDK session
)

type EventBody struct {
	Type      EventType    `json:"type"`
	Timestamp time.Time    `json:"timestamp"`
	Data      *interface{} `json:"data"`
}

const (
	ContentType         string = "Content-Type"
	XSignatureEd25519   string = "X-Signature-Ed25519"
	XSignatureTimestamp string = "X-Signature-Timestamp"
)

const ApplicationJSON string = "application/json"

func HandleDiscordWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		signature := r.Header.Get(XSignatureEd25519)
		timestamp := r.Header.Get(XSignatureTimestamp)
		publicKey := os.Getenv("DISCORD_PUBLIC_KEY")
		if publicKey == "" {
			responses.InternalServerError(w, r, "DISCORD_PUBLIC_KEY environment variable not set")
			return
		}
		if signature == "" || timestamp == "" {
			responses.BadRequest(w, r, "Invalid signature")
			return
		}

		var body []byte
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println("Error reading body:\n\t", err)
			responses.BadRequest(w, r, "Invalid signature")
			return
		}

		verified := ed25519.Verify([]byte(publicKey), body, []byte(signature))
		if !verified {
			responses.BadRequest(w, r, "Invalid signature")
			return
		}

		if r.Header.Get(ContentType) != ApplicationJSON {
			responses.UnsupportedMediaType(w, r, "Request must be of type application/json")
			return
		}

		var event WebhookEvent
		if err := json.NewDecoder(bytes.NewReader(body)).Decode(&event); err != nil {
			responses.BadRequest(w, r, "Invalid request body")
			return
		}

		switch event.Type {
		case PING:
			responses.NoContent(w, r)
			return
		case Event:
		default:
		}
	}
}
