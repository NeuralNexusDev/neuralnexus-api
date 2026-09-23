package discord

import (
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/goccy/go-json"
)

type WebhookType int

const (
	WebhookPing  WebhookType = 0 // PING event sent to verify your Webhook Event URL is active
	WebhookEvent WebhookType = 1 // Webhook event
)

// WebhookPayload payload delivered to the webhook endpoint
// https://docs.discord.com/developers/events/webhook-events
type WebhookPayload struct {
	// Version scheme for the webhook event. Currently, always 1
	Version int `json:"version"`
	// ID of your app
	ApplicationId string `json:"application_id"`
	// Type of webhook, either 0 for PING or 1 for webhook events
	Type WebhookType `json:"type"`
	// Event data payload
	Event *EventBody `json:"event"`
}

// EventType string type for the Discord Webhook event
// https://docs.discord.com/developers/events/webhook-events#event-types
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
	Type      EventType        `json:"type"`
	Timestamp time.Time        `json:"timestamp"`
	Data      *json.RawMessage `json:"data"`
}

type InstallationContext int

const (
	GUILD_INSTALL InstallationContext = 0 // App is installable to guilds/servers
	USER_INSTALL  InstallationContext = 1 // App is installable to users
)

// ApplicationAuthorizedEvent Websocket Event for application authorizations
// https://docs.discord.com/developers/events/webhook-events#application-authorized
type ApplicationAuthorizedEvent struct {
	// InstallationContext for the authorization. Either guild (0) if installed to a server or user (1) if installed to a user’s account
	IntegrationType *InstallationContext `json:"integration_type"`
	// discordgo.User who authorized the app
	User discordgo.User `json:"user"`
	// List of scopes the user authorized
	Scopes []string `json:"scopes"`
	// [Server/Guild](discordgo.Guild) which app was authorized for (when integration type is 0)
	Guild discordgo.Guild `json:"guild"`
}

// ApplicationDeauthorizedEvent Websocket Event for application Deauthorizations
// https://docs.discord.com/developers/events/webhook-events#application-deauthorized
type ApplicationDeauthorizedEvent struct {
	// discordgo.User who deauthorized the app
	User discordgo.User `json:"user"`
}
