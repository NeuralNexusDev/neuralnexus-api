package discord

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
