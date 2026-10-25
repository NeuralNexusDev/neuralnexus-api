package twitch

import (
	"io"
	"net/http"
	"strings"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/link"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// channel:bot -- send/listen to chat events
// moderator:read:chatters -- Get a list of users in the chat room.
// channel:manage:redemptions -- Manage Channel Points custom rewards and their redemptions on a channel.

const (
	EventSubMessageType = "twitch-eventsub-message-type"

	EventSubTypeRevocation         = "revocation"
	EventSubTypeVerification       = "webhook_callback_verification"
	EventSubTypeNotification       = "notification"
	msgFailedToHandleEventSubEvent = "Failed to handle EventSub event"
)

// HandleEventSub handles the EventSub notifications
func HandleEventSub(cfg config.Twitch, eventsub EventSubService, tokens link.OAuthTokenStore, linked link.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(EventSubMessageType) == "" {
			reqctx.Log(r.Context(), "EventSub message type not set")
			responses.BadRequest(w, r, "")
			return
		}

		var err error
		var body []byte
		body, err = io.ReadAll(r.Body)
		if err != nil {
			reqctx.Log(r.Context(), "Failed to read EventSub body:", err.Error())
			responses.BadRequest(w, r, "")
			return
		}
		defer r.Body.Close()

		vals, err := validateEventSubNotification(cfg.EventSubSecret, r.Header, body)
		if err != nil {
			reqctx.Log(r.Context(), "Failed to validate EventSub notification:", err.Error())
			responses.BadRequest(w, r, "")
			return
		}

		var userId = vals.Subscription.Condition.BroadcasterUserID
		var messageType = strings.ToLower(r.Header.Get(EventSubMessageType))
		switch messageType {
		case EventSubTypeRevocation:
			err = handleRevocation(r.Context(), userId, eventsub, tokens, *vals)
		case EventSubTypeVerification:
			err = handleVerification(w, r.Context(), userId, eventsub, *vals)
		case EventSubTypeNotification:
			err = handleNotification(r.Context(), userId, eventsub, tokens, *vals, linked)
		default:
			reqctx.Log(r.Context(), userId, "Unexpected EventSub message type:", messageType)
			responses.BadRequest(w, r, "")
			return
		}
		if err != nil {
			reqctx.Log(r.Context(), userId, "Failed to handle EventSub event:", err.Error())
			responses.InternalServerError(w, r, msgFailedToHandleEventSubEvent)
			return
		}
		responses.NoContent(w, r)
	}
}
