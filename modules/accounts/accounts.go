package accounts

// -------------- Structs --------------

// DiscordUser struct
type DiscordUser struct {
	Id       string `json:"id"`       // Discord ID
	Username string `json:"username"` // Discord username
	Avatar   string `json:"avatar"`   // Discord avatar
}

// TwitchUser struct
type TwitchUser struct {
	Id          string `json:"id"`           // Twitch ID
	Login       string `json:"login"`        // Twitch username
	DisplayName string `json:"display_name"` // Twitch display name
}

// MinecraftUser struct
type MinecraftUser struct {
	Id       string `json:"id"`       // Minecraft UUID
	Username string `json:"username"` // Minecraft username
	Skin     string `json:"skin"`     // Minecraft skin
}

// User struct
type User struct {
	ID        string        `json:"_id"`       // Internal ID
	UserId    string        `json:"userId"`    // User ID
	Username  string        `json:"username"`  // Username
	Discord   DiscordUser   `json:"discord"`   // DiscordUser
	Twitch    TwitchUser    `json:"twitch"`    // TwitchUser
	Minecraft MinecraftUser `json:"minecraft"` // MinecraftUser
}

// -------------- Enums --------------

// -------------- Functions --------------

// -------------- Handlers --------------
