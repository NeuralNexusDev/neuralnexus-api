package discord

import (
	"fmt"
	"log"
	"net/http"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/goccy/go-json"
)

func HandleDiscordWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(ContentType) != ApplicationJSON {
			responses.UnsupportedMediaType(w, r, "Request must be of type application/json")
			return
		}
		defer r.Body.Close()

		ctx := r.Context()
		var event WebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			mw.LogRequest(ctx, "Invalid Discord webhook request body:\n\t", err.Error())
			responses.BadRequest(w, r, "Invalid request body")
			return
		}
		if event.Version != 1 {
			mw.LogRequest(ctx,
				fmt.Sprintf("Unknown Discord webhook version, %d. Attempting to process regardless.", event.Version))
		}

		switch event.Type {
		case WebhookPing:
			mw.LogRequest(ctx, "Discord webhook PING")
			responses.NoContent(w, r)
			return
		case WebhookEvent:
			if event.Event.Data == nil {
				mw.LogRequest(ctx, fmt.Sprintf("Websocket event %s is missing data", event.Event.Type))
				responses.BadRequest(w, r, "Invalid request body")
				return
			}

			switch event.Event.Type {
			case ApplicationAuthorized:
				var data ApplicationAuthorizedEvent
				if err := json.Unmarshal(*event.Event.Data, &data); err != nil {
					mw.LogRequest(ctx, "Failed to deserialize Application Authorized event data:\n\t", err.Error())
					responses.BadRequest(w, r, "Invalid event data")
					return
				}
				switch {
				case data.IntegrationType == nil:
					mw.LogRequest(ctx, "Failed to deserialize Application Authorized event data")
					responses.BadRequest(w, r, "Invalid event data")
					return
				case GUILD_INSTALL == *data.IntegrationType:
					mw.LogRequest(ctx, fmt.Sprintf("Received authorized event for Discord Guild: %s", data.Guild.ID))
				case USER_INSTALL == *data.IntegrationType:
					mw.LogRequest(ctx, fmt.Sprintf("Received authorized event for Discord User: %s", data.User.ID))
				default:
					mw.LogRequest(ctx, "Invalid integration type")
					responses.BadRequest(w, r, "Invalid event data")
					return
				}
			case ApplicationDeauthorized:
				var data ApplicationAuthorizedEvent
				if err := json.Unmarshal(*event.Event.Data, &data); err != nil {
					mw.LogRequest(ctx, "Failed to deserialize Application Deauthorized event data:\n\t", err.Error())
					responses.BadRequest(w, r, "Invalid event data")
					return
				}
				mw.LogRequest(ctx, fmt.Sprintf("Received deauthorized event for Discord User: %s", data.User.ID))
			default:
				mw.LogRequest(ctx, fmt.Sprintf("Unknown Discord webhook event type: %s", event.Event.Type))
				responses.BadRequest(w, r, "Unknown Discord webhook event type")
				return
			}
		default:
			mw.LogRequest(ctx, fmt.Sprintf("Unknown Webhook type: %v", event.Type))
			responses.BadRequest(w, r, "Unknown Webhook type")
			return
		}
		responses.NoContent(w, r)
	}
}

func HandleDiscordInteraction() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(ContentType) != ApplicationJSON {
			responses.UnsupportedMediaType(w, r, "Request must be of type application/json")
			return
		}
		defer r.Body.Close()

		var interaction Interaction
		if err := json.NewDecoder(r.Body).Decode(&interaction); err != nil {
			responses.BadRequest(w, r, "Invalid request body")
			return
		}

		switch interaction.Type {
		case INTERACTION_PING:
			responses.SendStruct(w, r, http.StatusOK, Interaction{Type: INTERACTION_PING})
			return
		default:
			responses.BadRequest(w, r, "Unknown Interaction")
			log.Printf("Unknown Interaction type: %v", interaction.Type)
			return
		}
	}
}
