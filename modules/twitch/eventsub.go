package twitch

import (
	"bytes"
	"context"
	"errors"
	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
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
		mw.LogRequest(r.Context(), "EventSub notification is older than 10 minutes")
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
			mw.LogRequest(r.Context(), "EventSub message type not set")
			responses.BadRequest(w, r, "EventSub message type not set")
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			mw.LogRequest(r.Context(), "Failed to read EventSub body:", err.Error())
			responses.InternalServerError(w, r, "Failed to read EventSub body")
			return
		}
		defer r.Body.Close()

		vals, err := validateEventSubNotification(w, r, body)
		if err != nil {
			mw.LogRequest(r.Context(), "Failed to validate EventSub notification:", err.Error())
			responses.InternalServerError(w, r, "Failed to validate EventSub notification")
			return
		}

		switch vals.Subscription.Status {
		case helix.EventSubStatusPending:
			mw.LogRequest(r.Context(), "EventSub challenge received, responding with challenge")
			err = eventsub.UpdateEventSubSubscriptionStatus(vals.Subscription.ID, helix.EventSubStatusEnabled)
			if err != nil {
				mw.LogRequest(r.Context(), "Failed to update EventSub subscription:", err.Error())
			}
			return
		case helix.EventSubStatusFailed:
			mw.LogRequest(r.Context(), "EventSub verification failed")
			err = eventsub.RevokeEventSubSubscription(vals.Subscription.ID, vals.Subscription.Status)
			if err != nil {
				mw.LogRequest(r.Context(), "Failed to revoke EventSub subscription:", err.Error())
				responses.InternalServerError(w, r, "Failed to revoke EventSub subscription")
				return
			}
			responses.NoContent(w, r)
			return
		}

		messageType := strings.ToLower(r.Header.Get(EVENTSUB_MESSAGE_TYPE))
		switch messageType {
		case EventSubTypeRevocation:
			err = HandleRevocation(r.Context(), eventsub, tokens, *vals)
			if err != nil {
				mw.LogRequest(r.Context(), "Failed to handle EventSub revocation:", err.Error())
				responses.InternalServerError(w, r, "Failed to handle EventSub revocation")
				return
			}
		case helix.EventSubTypeChannelFollow:
			var userID = vals.Subscription.Condition.BroadcasterUserID
			mw.LogRequest(r.Context(), userID, "EventSub channel follow received")
			var followEvent helix.EventSubChannelFollowEvent
			err = json.NewDecoder(bytes.NewReader(vals.Event)).Decode(&followEvent)
			if err != nil {
				mw.LogRequest(r.Context(), userID, "Failed to decode EventSub channel follow event:", err.Error())
				responses.InternalServerError(w, r, "Failed to decode EventSub channel follow event")
				return
			}
			log.Printf("User %s followed channel %s\n", followEvent.UserID, followEvent.BroadcasterUserID)
		}

		responses.NoContent(w, r)
	}
}

// HandleRevocation handles the EventSub revocation notifications
func HandleRevocation(ctx context.Context, eventsub EventSubService, tokens auth.OAuthTokenStore, vals eventSubNotification) error {
	var err error
	var userID = vals.Subscription.Condition.BroadcasterUserID
	mw.LogRequest(ctx, userID, "EventSub revocation received")
	switch vals.Subscription.Status {
	case helix.EventSubStatusAuthorizationRevoked:
		mw.LogRequest(ctx, userID, "EventSub authorization revoked")
		err = tokens.DeleteOAuthToken(vals.Subscription.Condition.BroadcasterUserID, auth.PlatformTwitch)
		if err != nil {
			mw.LogRequest(ctx, userID, "Failed to delete OAuth token:", err.Error())
			return errors.New("failed to delete OAuth token")
		}
		fallthrough
	case helix.EventSubStatusUserRemoved,
		helix.EventSubStatusNotificationFailuresExceeded,
		EventSubStatusVersionRemoved:
		mw.LogRequest(ctx, userID, "EventSub subscription removed")
		err = eventsub.RevokeEventSubSubscription(vals.Subscription.ID, vals.Subscription.Status)
		if err != nil {
			mw.LogRequest(ctx, userID, "Failed to revoke EventSub subscription:", err.Error())
			return errors.New("failed to revoke EventSub subscription")
		}
	default:
		mw.LogRequest(ctx, userID, "EventSub unknown revocation status:", vals.Subscription.Status)
		return errors.New("unknown EventSub revocation status")
	}
	return nil
}
