package discord

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/goccy/go-json"
)

type WebhookType int

const (
	WebhookTypePing  WebhookType = 0 // PING event sent to verify your Webhook Event URL is active
	WebhookTypeEvent WebhookType = 1 // Webhook event
)

func (t WebhookType) String() string {
	switch t {
	case WebhookTypePing:
		return "Ping"
	case WebhookTypeEvent:
		return "Event"
	default:
		return fmt.Sprintf("WebhookType(%d)", t)
	}
}

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
	Event *WebhookEvent `json:"event"`
}

// WebhookEventType string type for the Discord Webhook event
// https://docs.discord.com/developers/events/webhook-events#event-types
type WebhookEventType string

const (
	ApplicationAuthorized   WebhookEventType = "APPLICATION_AUTHORIZED"     // Sent when an app was authorized by a user to a server or their account
	ApplicationDeauthorized WebhookEventType = "APPLICATION_DEAUTHORIZED"   // Sent when an app was deauthorized by a user
	EntitlementCreate       WebhookEventType = "ENTITLEMENT_CREATE"         // Entitlement was created
	EntitlementUpdate       WebhookEventType = "ENTITLEMENT_UPDATE"         // Entitlement was updated
	EntitlementDelete       WebhookEventType = "ENTITLEMENT_DELETE"         // Entitlement was deleted
	QuestUserEnrollment     WebhookEventType = "QUEST_USER_ENROLLMENT"      // User was added to a Quest (currently unavailable)
	LobbyMessageCreate      WebhookEventType = "LOBBY_MESSAGE_CREATE"       // Sent when a message is created in a lobby
	LobbyMessageUpdate      WebhookEventType = "LOBBY_MESSAGE_UPDATE"       // Sent when a message is updated in a lobby
	LobbyMessageDelete      WebhookEventType = "LOBBY_MESSAGE_DELETE"       // Sent when a message is deleted from a lobby
	GameDirectMessageCreate WebhookEventType = "GAME_DIRECT_MESSAGE_CREATE" // Sent when a direct message is created during an active Social SDK session
	GameDirectMessageUpdate WebhookEventType = "GAME_DIRECT_MESSAGE_UPDATE" // Sent when a direct message is updated during an active Social SDK session
	GameDirectMessageDelete WebhookEventType = "GAME_DIRECT_MESSAGE_DELETE" // Sent when a direct message is deleted during an active Social SDK session
)

func (t WebhookEventType) String() string {
	switch t {
	case ApplicationAuthorized:
		return "ApplicationAuthorized"
	case ApplicationDeauthorized:
		return "ApplicationDeauthorized"
	case EntitlementCreate:
		return "EntitlementCreate"
	case EntitlementUpdate:
		return "EntitlementUpdate"
	case EntitlementDelete:
		return "EntitlementDelete"
	case QuestUserEnrollment:
		return "QuestUserEnrollment"
	case LobbyMessageCreate:
		return "LobbyMessageCreate"
	case LobbyMessageUpdate:
		return "LobbyMessageUpdate"
	case LobbyMessageDelete:
		return "LobbyMessageDelete"
	case GameDirectMessageCreate:
		return "GameDirectMessageCreate"
	case GameDirectMessageUpdate:
		return "GameDirectMessageUpdate"
	case GameDirectMessageDelete:
		return "GameDirectMessageDelete"
	default:
		return string("WebhookEventType(" + t + ")")
	}
}

type WebhookEvent struct {
	Type      WebhookEventType `json:"type"`
	Timestamp time.Time        `json:"timestamp"`
	Data      WebhookEventData `json:"data"`
}

type InstallationContext int

const (
	GuildInstall InstallationContext = 0 // App is installable to guilds/servers
	UserInstall  InstallationContext = 1 // App is installable to users
)

func (c InstallationContext) String() string {
	switch c {
	case GuildInstall:
		return "GuildInstall"
	case UserInstall:
		return "UserInstall"
	default:
		return fmt.Sprintf("InstallationContext(%d)", c)
	}
}

type WebhookEventData interface {
	Type() WebhookEventType
}

// ApplicationAuthorizedWebhookData Websocket Event for application authorizations
// https://docs.discord.com/developers/events/webhook-events#application-authorized
type ApplicationAuthorizedWebhookData struct {
	// InstallationContext for the authorization. Either guild (0) if installed to a server or user (1) if installed to a user’s account
	IntegrationType *InstallationContext `json:"integration_type"`
	// discordgo.User who authorized the app
	User discordgo.User `json:"user"`
	// List of scopes the user authorized
	Scopes []string `json:"scopes"`
	// [Server/Guild](discordgo.Guild) which app was authorized for (when integration type is 0)
	Guild discordgo.Guild `json:"guild"`
}

func (d ApplicationAuthorizedWebhookData) Type() WebhookEventType {
	return ApplicationAuthorized
}

// ApplicationDeauthorizedWebhookData Websocket Event for application Deauthorizations
// https://docs.discord.com/developers/events/webhook-events#application-deauthorized
type ApplicationDeauthorizedWebhookData struct {
	// discordgo.User who deauthorized the app
	User discordgo.User `json:"user"`
}

func (d ApplicationDeauthorizedWebhookData) Type() WebhookEventType {
	return ApplicationDeauthorized
}

type webhookEvent WebhookEvent

type rawWebhookEvent struct {
	webhookEvent
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

func (e *WebhookEvent) UnmarshalJSON(raw []byte) error {
	var tmp rawWebhookEvent
	err := json.Unmarshal(raw, &tmp)
	if err != nil {
		return err
	}

	*e = WebhookEvent(tmp.webhookEvent)

	t, err := time.ParseInLocation(time.RFC3339Nano, tmp.Timestamp+"Z", time.UTC)
	if err != nil {
		return err
	}
	e.Timestamp = t

	switch tmp.Type {
	case ApplicationAuthorized:
		v := ApplicationAuthorizedWebhookData{}
		err = json.Unmarshal(tmp.Data, &v)
		if err != nil {
			return err
		}
		e.Data = v
	case ApplicationDeauthorized:
		v := ApplicationDeauthorizedWebhookData{}
		err = json.Unmarshal(tmp.Data, &v)
		if err != nil {
			return err
		}
		e.Data = v
	}
	return nil
}

func (e *WebhookEvent) ApplicationAuthorizedData() (data ApplicationAuthorizedWebhookData) {
	if e.Type != ApplicationAuthorized {
		panic("ApplicationAuthorizedData called on interaction of type " + e.Type.String())
	}
	return e.Data.(ApplicationAuthorizedWebhookData)
}

func (e *WebhookEvent) ApplicationDeauthorizedData() (data ApplicationDeauthorizedWebhookData) {
	if e.Type != ApplicationDeauthorized {
		panic("ApplicationDeauthorizedData called on interaction of type " + e.Type.String())
	}
	return e.Data.(ApplicationDeauthorizedWebhookData)
}
