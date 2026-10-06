package discord

import (
	"fmt"
	"net/http"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/bwmarrin/discordgo"
	"github.com/goccy/go-json"
)

const (
	msgRequestMustBeJSON  = "Request must be of type application/json"
	msgInvalidRequestBody = "Invalid request body"
	msgInvalidEventData   = "Invalid event data"
)

func HandleDiscordWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(ContentType) != ApplicationJSON {
			responses.UnsupportedMediaType(w, r, msgRequestMustBeJSON)
			return
		}
		defer r.Body.Close()

		ctx := r.Context()
		var event WebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			mw.LogRequest(ctx, "Invalid Discord webhook request body:\n\t", err.Error())
			responses.BadRequest(w, r, msgInvalidRequestBody)
			return
		}
		if event.Version != 1 {
			mw.LogRequest(ctx,
				fmt.Sprintf("Unknown Discord webhook version, %d. Attempting to process regardless.", event.Version))
		}

		switch event.Type {
		case WebhookTypePing:
			mw.LogRequest(ctx, "Discord webhook PING")
			responses.NoContent(w, r)
			return
		case WebhookTypeEvent:
			switch event.Event.Type {
			case ApplicationAuthorized:
				data := event.Event.ApplicationAuthorizedData()
				switch {
				case data.IntegrationType == nil:
					mw.LogRequest(ctx, "Received authorized event with nil integration type, assuming user login: ", data.User.ID)
					jsonData, err := json.Marshal(data)
					if err != nil {
						fmt.Println(err)
					}
					fmt.Println(string(jsonData))
				case *data.IntegrationType == GuildInstall:
					mw.LogRequest(ctx, fmt.Sprintf("Received authorized event for Discord Guild: %s", data.Guild.ID))
				case *data.IntegrationType == UserInstall:
					mw.LogRequest(ctx, fmt.Sprintf("Received authorized event for Discord User: %s", data.User.ID))
				default:
					mw.LogRequest(ctx, "Invalid integration type")
					responses.BadRequest(w, r, msgInvalidEventData)
					return
				}
			case ApplicationDeauthorized:
				data := event.Event.ApplicationDeauthorizedData()
				mw.LogRequest(ctx, fmt.Sprintf("Received deauthorized event for Discord User: %s", data.User.ID))
			case EntitlementCreate, EntitlementUpdate, EntitlementDelete, QuestUserEnrollment, LobbyMessageCreate,
				LobbyMessageUpdate, LobbyMessageDelete, GameDirectMessageCreate, GameDirectMessageUpdate, GameDirectMessageDelete:
				mw.LogRequest(ctx, fmt.Sprintf("Unhandled Discord webhook event type: %s", event.Event.Type.String()))
			default:
				mw.LogRequest(ctx, fmt.Sprintf("Unknown Discord webhook event type: %s", event.Event.Type.String()))
			}
		default:
			mw.LogRequest(ctx, fmt.Sprintf("Unknown Webhook type: %s", event.Type.String()))
		}
		responses.NoContent(w, r)
	}
}

func HandleDiscordInteraction() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(ContentType) != ApplicationJSON {
			responses.UnsupportedMediaType(w, r, msgRequestMustBeJSON)
			return
		}
		defer r.Body.Close()

		ctx := r.Context()
		var interaction discordgo.Interaction
		if err := json.NewDecoder(r.Body).Decode(&interaction); err != nil {
			responses.BadRequest(w, r, msgInvalidRequestBody)
			return
		}

		switch interaction.Type {
		case discordgo.InteractionPing:
			mw.LogRequest(ctx, "Discord interaction PING")
			responses.SendStruct(w, r, http.StatusOK, discordgo.Interaction{Type: discordgo.InteractionPing})
			return
		case discordgo.InteractionApplicationCommand, discordgo.InteractionMessageComponent,
			discordgo.InteractionApplicationCommandAutocomplete, discordgo.InteractionModalSubmit:
			mw.LogRequest(ctx, fmt.Sprintf("Unhandled Discord Interaction type: %s", interaction.Type.String()))
		default:
			mw.LogRequest(ctx, fmt.Sprintf("Unknown Discord Interaction type: %v", interaction.Type.String()))
		}
		responses.NoContent(w, r)
	}
}
