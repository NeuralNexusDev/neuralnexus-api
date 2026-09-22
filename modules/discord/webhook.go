package discord

import (
	"bytes"
	"encoding/hex"
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
	WEBHOOK_PING WebhookType = 0
	Event        WebhookType = 1
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

// https://docs.discord.com/developers/events/webhook-events#application-authorized

func HandleDiscordWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, verified := verifyPayload(w, r)
		if !verified {
			return // Already responded
		}

		if r.Header.Get(ContentType) != ApplicationJSON {
			responses.UnsupportedMediaType(w, r, "Request must be of type application/json")
			return
		}

		var event WebhookEvent
		if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&event); err != nil {
			responses.BadRequest(w, r, "Invalid request body")
			return
		}

		switch event.Type {
		case WEBHOOK_PING:
			responses.NoContent(w, r)
			return
		case Event:
		default:
			responses.BadRequest(w, r, "Unknown Webhook type")
			log.Printf("Unknown Webhook type: %v", event.Type)
			return
		}
	}
}

// TODO: Make this middleware?
func verifyPayload(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	publicKeyStr := os.Getenv("DISCORD_PUBLIC_KEY")
	publicKey, err := hex.DecodeString(publicKeyStr)
	if publicKeyStr == "" || err != nil || len(publicKey) != ed25519.PublicKeySize {
		responses.InternalServerError(w, r, "DISCORD_PUBLIC_KEY environment variable not set")
		return nil, false
	}
	signatureStr := r.Header.Get(XSignatureEd25519)
	signature, err := hex.DecodeString(signatureStr)
	timestamp := r.Header.Get(XSignatureTimestamp)
	if signatureStr == "" || err != nil || timestamp == "" {
		responses.Unauthorized(w, r, "Invalid signature")
		return nil, false
	}
	defer r.Body.Close()

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("Error reading body:\n\t", err)
		responses.Unauthorized(w, r, "Invalid signature")
		return nil, false
	}

	var buffer bytes.Buffer
	buffer.WriteString(timestamp)
	buffer.Write(bodyBytes)

	if !ed25519.Verify(publicKey, buffer.Bytes(), signature) {
		responses.Unauthorized(w, r, "Invalid signature")
		return nil, false
	}
	return bodyBytes, true
}
