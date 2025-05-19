package twitch

import (
	"bytes"
	"errors"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/goccy/go-json"
	"github.com/nicklaw5/helix/v2"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// channel:bot -- send/listen to chat events
// moderator:read:chatters -- Get a list of users in the chat room.
// channel:manage:redemptions -- Manage Channel Points custom rewards and their redemptions on a channel.

//goland:noinspection GoSnakeCaseUsage
var (
	EVENTSUB_URI    = os.Getenv("TWITCH_EVENTSUB_URI")
	EVENTSUB_SECRET = os.Getenv("TWITCH_EVENTSUB_SECRET")
)

//goland:noinspection GoSnakeCaseUsage
const (
	EVENTSUB_MESSAGE_TYPE       = "twitch-eventsub-message-type"
	EventSubTypeRevocation      = "revocation"
	WebhookCallbackVerification = "webhook_callback_verification"
)

type eventSubNotification struct {
	Subscription helix.EventSubSubscription `json:"subscription"`
	Challenge    string                     `json:"challenge"`
	Event        json.RawMessage            `json:"event"`
}

// validateEventSubNotification validates the EventSub notification
func validateEventSubNotification(w http.ResponseWriter, r *http.Request, body []byte) (*eventSubNotification, error) {
	if !helix.VerifyEventSubNotification(EVENTSUB_SECRET, r.Header, string(body)) {
		return nil, errors.New("invalid signature")
	}
	var vals eventSubNotification
	err := json.NewDecoder(bytes.NewReader(body)).Decode(&vals)
	if err != nil {
		return nil, err
	}
	if vals.Subscription.CreatedAt.Before(time.Now().Add(-10 * time.Minute)) {
		log.Println("EventSub notification is older than 10 minutes")
		return nil, errors.New("notification is too old")
	}
	messageType := strings.ToLower(r.Header.Get(EVENTSUB_MESSAGE_TYPE))
	if vals.Challenge != "" && messageType == WebhookCallbackVerification {
		w.Write([]byte(vals.Challenge))
		return nil, nil
	}
	return &vals, nil
}

func HandleEventSub(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println("failed to read EventSub body:", err)
		return
	}
	defer r.Body.Close()

	vals, err := validateEventSubNotification(w, r, body)
	if err != nil {
		log.Println("failed to validate EventSub notification:", err)
		w.WriteHeader(403)
		return
	}
	if vals == nil {
		log.Println("EventSub challenge received, responding with challenge")
		return
	}

	messageType := strings.ToLower(r.Header.Get(EVENTSUB_MESSAGE_TYPE))
	switch messageType {
	case EventSubTypeRevocation:
		log.Println("EventSub revocation received")
		// TODO: Handle revocation
	}

	//var followEvent helix.EventSubChannelFollowEvent
	//err = json.NewDecoder(bytes.NewReader(vals.Event)).Decode(&followEvent)

	responses.NoContent(w, r)
}
