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
	EventSubTypeNotification     = "notification"
	EventSubTypeVerification     = "webhook_callback_verification"
	EventSubStatusVersionRemoved = "version_removed"
)

// eventSubNotification Outlines the structure of the EventSub notification
type eventSubNotification struct {
	Subscription helix.EventSubSubscription `json:"subscription"`
	Challenge    string                     `json:"challenge"`
	Event        json.RawMessage            `json:"event"`
}

// validateEventSubNotification validates the EventSub notification
func validateEventSubNotification(header http.Header, body []byte) (*eventSubNotification, error) {
	if !helix.VerifyEventSubNotification(EVENTSUB_SECRET, header, string(body)) {
		return nil, errors.New("invalid signature")
	}
	var vals eventSubNotification
	err := json.NewDecoder(bytes.NewReader(body)).Decode(&vals)
	if err != nil {
		return nil, err
	}
	if vals.Subscription.CreatedAt.Before(time.Now().Add(-10 * time.Minute)) {
		return nil, errors.New("notification is too old")
	}
	return &vals, nil
}

// HandleEventSub handles the EventSub notifications
func HandleEventSub(eventsub EventSubService, tokens auth.OAuthTokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(EVENTSUB_MESSAGE_TYPE) == "" {
			mw.LogRequest(r.Context(), "EventSub message type not set")
			responses.BadRequest(w, r, "")
			return
		}

		var err error
		var body []byte
		body, err = io.ReadAll(r.Body)
		if err != nil {
			mw.LogRequest(r.Context(), "Failed to read EventSub body:", err.Error())
			responses.BadRequest(w, r, "")
			return
		}
		defer r.Body.Close()

		vals, err := validateEventSubNotification(r.Header, body)
		if err != nil {
			mw.LogRequest(r.Context(), "Failed to validate EventSub notification:", err.Error())
			responses.BadRequest(w, r, "")
			return
		}

		var userId = vals.Subscription.Condition.BroadcasterUserID
		var messageType = strings.ToLower(r.Header.Get(EVENTSUB_MESSAGE_TYPE))
		switch messageType {
		case EventSubTypeRevocation:
			err = handleRevocation(r.Context(), userId, eventsub, tokens, *vals)
		case EventSubTypeVerification:
			err = handleVerification(w, r.Context(), userId, eventsub, *vals)
		case EventSubTypeNotification:
			err = handleNotification(r.Context(), userId, eventsub, tokens, *vals)
		default:
			mw.LogRequest(r.Context(), userId, "Unexpected EventSub message type:", messageType)
			responses.BadRequest(w, r, "")
			return
		}
		if err != nil {
			mw.LogRequest(r.Context(), userId, "Failed to handle EventSub event:", err.Error())
			responses.InternalServerError(w, r, "Failed to handle EventSub event")
			return
		}
		responses.NoContent(w, r)
	}
}

// handleRevocation handles the EventSub revocation notifications
func handleRevocation(ctx context.Context, userId string, eventsub EventSubService, tokens auth.OAuthTokenStore, vals eventSubNotification) error {
	var err error
	switch vals.Subscription.Status {
	case helix.EventSubStatusAuthorizationRevoked:
		mw.LogRequest(ctx, userId, "EventSub authorization revoked")
		err = tokens.DeleteOAuthToken(vals.Subscription.Condition.BroadcasterUserID, auth.PlatformTwitch)
		if err != nil {
			mw.LogRequest(ctx, userId, "Failed to delete OAuth token:", err.Error())
			return errors.New("failed to delete OAuth token")
		}
		fallthrough
	case helix.EventSubStatusUserRemoved,
		helix.EventSubStatusNotificationFailuresExceeded,
		EventSubStatusVersionRemoved:
		mw.LogRequest(ctx, userId, "EventSub subscription removed")
		err = eventsub.RevokeEventSubSubscription(vals.Subscription.ID, vals.Subscription.Status)
		if err != nil {
			mw.LogRequest(ctx, userId, "Failed to revoke EventSub subscription:", err.Error())
			return errors.New("failed to revoke EventSub subscription")
		}
	default:
		mw.LogRequest(ctx, userId, "EventSub unknown revocation status:", vals.Subscription.Status)
		return errors.New("unknown EventSub revocation status")
	}
	return nil
}

// handleVerification handles the EventSub verification challenge
func handleVerification(w http.ResponseWriter, ctx context.Context, userId string, eventsub EventSubService, vals eventSubNotification) error {
	if vals.Challenge == "" || vals.Subscription.Status != helix.EventSubStatusPending {
		mw.LogRequest(ctx, userId, "EventSub unknown verification status:", vals.Subscription.Status)
		return errors.New("unknown EventSub verification status")
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(vals.Challenge))
	mw.LogRequest(ctx, userId, "EventSub challenge received, responding with challenge")

	var err = eventsub.UpdateEventSubSubscriptionStatus(vals.Subscription.ID, helix.EventSubStatusEnabled)
	if err != nil {
		mw.LogRequest(ctx, userId, "Failed to update EventSub subscription:", err.Error())
	}
	return nil
}

// handleNotification handles the EventSub notifications
func handleNotification(ctx context.Context, userId string, eventsub EventSubService, tokens auth.OAuthTokenStore, vals eventSubNotification) error {
	var err error
	mw.LogRequest(ctx, userId, "EventSub notification type:", vals.Subscription.Type)
	switch vals.Subscription.Type {
	case helix.EventSubTypeChannelChatMessage:
		err = handleChannelChatMessage(ctx, userId, eventsub, tokens, vals)
	default:
		mw.LogRequest(ctx, userId, "EventSub unknown notification type:", vals.Subscription.Type)
		return errors.New("unknown EventSub notification type")
	}
	if err != nil {
		mw.LogRequest(ctx, userId, "Failed to handle EventSub notification:", err.Error())
		return errors.New("failed to handle EventSub notification")
	}
	return nil
}

// handleChannelChatMessage handles the EventSub chat message notifications
func handleChannelChatMessage(ctx context.Context, userId string, eventsub EventSubService, tokens auth.OAuthTokenStore, vals eventSubNotification) error {
	var err error
	var chatEvent helix.EventSubChannelChatMessageEvent
	err = json.NewDecoder(bytes.NewReader(vals.Event)).Decode(&chatEvent)
	if err != nil {
		mw.LogRequest(ctx, userId, "Failed to decode EventSub chat message event:", err.Error())
		return errors.New("failed to decode EventSub chat message event")
	}
	log.Printf("User %s sent a chat message in channel %s: %s\n", chatEvent.ChatterUserID, chatEvent.BroadcasterUserID, chatEvent.Message.Text)
	return nil
}
