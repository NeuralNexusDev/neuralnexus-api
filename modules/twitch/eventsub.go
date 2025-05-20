package twitch

import (
	"bytes"
	"errors"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
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
	EVENTSUB_MESSAGE_TYPE        = "twitch-eventsub-message-type"
	EventSubTypeRevocation       = "revocation"
	EventSubTypeVerification     = "webhook_callback_verification"
	EventSubStatusVersionRemoved = "version_removed"
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
	if messageType == EventSubTypeVerification {
		if vals.Challenge != "" && vals.Subscription.Status == helix.EventSubStatusPending {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(vals.Challenge))
			return &vals, nil
		}
		return nil, errors.New("invalid challenge")
	}
	return &vals, nil
}

// HandleEventSub handles the EventSub notifications
func HandleEventSub(eventsub EventSubService, tokens auth.OAuthTokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(EVENTSUB_MESSAGE_TYPE) == "" {
			log.Println("EventSub message type not set")
			responses.BadRequest(w, r, "EventSub message type not set")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Println("Failed to read EventSub body:", err)
			responses.InternalServerError(w, r, "Failed to read EventSub body")
			return
		}
		defer r.Body.Close()

		vals, err := validateEventSubNotification(w, r, body)
		if err != nil {
			log.Println("Failed to validate EventSub notification:", err)
			responses.InternalServerError(w, r, "Failed to validate EventSub notification")
			return
		}

		switch vals.Subscription.Status {
		case helix.EventSubStatusPending:
			log.Println("EventSub challenge received, responding with challenge")
			err = eventsub.UpdateEventSubSubscriptionStatus(vals.Subscription.ID, helix.EventSubStatusEnabled)
			if err != nil {
				log.Println("Failed to update EventSub subscription:", err)
			}
			return
		case helix.EventSubStatusFailed:
			log.Println("EventSub verification failed")
			err = eventsub.RevokeEventSubSubscription(vals.Subscription.ID, vals.Subscription.Status)
			if err != nil {
				log.Println("Failed to revoke EventSub subscription:", err)
				responses.InternalServerError(w, r, "Failed to revoke EventSub subscription")
				return
			}
			responses.NoContent(w, r)
			return
		}

		messageType := strings.ToLower(r.Header.Get(EVENTSUB_MESSAGE_TYPE))
		switch messageType {
		case EventSubTypeRevocation:
			err = HandleRevocation(eventsub, tokens, *vals)
			if err != nil {
				log.Println("Failed to handle EventSub revocation:", err)
				responses.InternalServerError(w, r, "Failed to handle EventSub revocation")
				return
			}
		case helix.EventSubTypeChannelFollow:
			log.Println("EventSub channel follow received")
			var followEvent helix.EventSubChannelFollowEvent
			err = json.NewDecoder(bytes.NewReader(vals.Event)).Decode(&followEvent)
			if err != nil {
				log.Println("Failed to decode EventSub channel follow event:", err)
				responses.InternalServerError(w, r, "Failed to decode EventSub channel follow event")
				return
			}
			log.Printf("User %s followed channel %s\n", followEvent.UserID, followEvent.BroadcasterUserID)
		}

		responses.NoContent(w, r)
	}
}

// HandleRevocation handles the EventSub revocation notifications
func HandleRevocation(eventsub EventSubService, tokens auth.OAuthTokenStore, vals eventSubNotification) error {
	var err error
	log.Println("EventSub revocation received")
	switch vals.Subscription.Status {
	case helix.EventSubStatusAuthorizationRevoked:
		log.Println("EventSub authorization revoked")
		err = tokens.DeleteOAuthToken(vals.Subscription.Condition.BroadcasterUserID, auth.PlatformTwitch)
		if err != nil {
			log.Println("failed to delete OAuth token:", err)
			return errors.New("failed to delete OAuth token")
		}
		fallthrough
	case helix.EventSubStatusUserRemoved,
		helix.EventSubStatusNotificationFailuresExceeded,
		EventSubStatusVersionRemoved:
		log.Println("EventSub subscription removed")
		err = eventsub.RevokeEventSubSubscription(vals.Subscription.ID, vals.Subscription.Status)
		if err != nil {
			log.Println("failed to revoke EventSub subscription:", err)
			return errors.New("failed to revoke EventSub subscription")
		}
	default:
		log.Println("EventSub unknown revocation status:", vals.Subscription.Status)
		return errors.New("unknown EventSub revocation status")
	}
	return nil
}
