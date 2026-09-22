package discord

import (
	"log"
	"net/http"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/goccy/go-json"
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

const ContentType string = "Content-Type"

const ApplicationJSON string = "application/json"

// https://docs.discord.com/developers/events/webhook-events#application-authorized

func HandleDiscordWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(ContentType) != ApplicationJSON {
			responses.UnsupportedMediaType(w, r, "Request must be of type application/json")
			return
		}
		defer r.Body.Close()

		var event WebhookEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
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
