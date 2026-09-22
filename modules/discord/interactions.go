package discord

import (
	"bytes"
	"log"
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/goccy/go-json"
)

type InteractionType int

const (
	INTERACTION_PING                 InteractionType = 1
	APPLICATION_COMMAND              InteractionType = 2
	MESSAGE_COMPONENT                InteractionType = 3
	APPLICATION_COMMAND_AUTOCOMPLETE InteractionType = 4
	MODAL_SUBMIT                     InteractionType = 5
)

// https://docs.discord.com/developers/interactions/receiving-and-responding#interaction-object-interaction-type

//id	snowflake	ID of the interaction
//application_id	snowflake	ID of the application this interaction is for
//type	interaction type	Type of interaction
//data?*	interaction data	Interaction data payload
//guild?	partial guild object	Guild that the interaction was sent from
//guild_id?	snowflake	Guild that the interaction was sent from
//channel?	partial channel object	Channel that the interaction was sent from
//channel_id?	snowflake	Channel that the interaction was sent from
//member?**	guild member object	Guild member data for the invoking user, including permissions
//user?	user object	User object for the invoking user, if invoked in a DM
//token	string	Continuation token for responding to the interaction
//version	integer	Read-only property, always 1
//message?	message object	For components or modals triggered by components, the message they were attached to
//app_permissions***	string	Bitwise set of permissions the app has in the source location of the interaction
//locale?****	string	Selected language of the invoking user
//guild_locale?	string	Guild’s preferred locale, if invoked in a guild
//entitlements	array of entitlement objects	For monetized apps, any entitlements for the invoking user, representing access to premium SKUs
//authorizing_integration_owners	dictionary with keys of application integration types	Mapping of installation contexts that the interaction was authorized for to related user or guild IDs. See Authorizing Integration Owners Object for details
//context?	interaction context type	Context where the interaction was triggered from
//attachment_size_limit	integer	Attachment size limit in bytes

type Interaction struct {
	Type InteractionType `json:"type"`
}

func HandleDiscordInteraction() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, verified := verifyPayload(w, r)
		if !verified {
			return // Already responded
		}

		if r.Header.Get(ContentType) != ApplicationJSON {
			responses.UnsupportedMediaType(w, r, "Request must be of type application/json")
			return
		}

		var interaction Interaction
		if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&interaction); err != nil {
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
