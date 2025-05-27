package twitch

import (
	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"io"
	"net/http"
	"os"
	"strings"
)

// channel:bot -- send/listen to chat events
// moderator:read:chatters -- Get a list of users in the chat room.
// channel:manage:redemptions -- Manage Channel Points custom rewards and their redemptions on a channel.

//goland:noinspection GoSnakeCaseUsage
var (
	EVENTSUB_URI    = os.Getenv("TWITCH_EVENTSUB_URI")
	EVENTSUB_SECRET = os.Getenv("TWITCH_EVENTSUB_SECRET")
)

const (
	EventSubMessageType = "twitch-eventsub-message-type"

	EventSubTypeRevocation   = "revocation"
	EventSubTypeVerification = "webhook_callback_verification"
	EventSubTypeNotification = "notification"
)

// HandleEventSub handles the EventSub notifications
func HandleEventSub(eventsub EventSubService, tokens auth.OAuthTokenStore, linked auth.LinkAccountStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(EventSubMessageType) == "" {
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
		var messageType = strings.ToLower(r.Header.Get(EventSubMessageType))
		switch messageType {
		case EventSubTypeRevocation:
			err = handleRevocation(r.Context(), userId, eventsub, tokens, *vals)
		case EventSubTypeVerification:
			err = handleVerification(w, r.Context(), userId, eventsub, *vals)
		case EventSubTypeNotification:
			err = handleNotification(r.Context(), userId, eventsub, tokens, *vals, linked)
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
