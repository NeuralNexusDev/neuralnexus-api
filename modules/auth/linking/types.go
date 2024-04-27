package accountlinking

import (
	"time"
)

// -------------- Structs --------------

// LinkedAccount struct
type LinkedAccount struct {
	UserID           string      `db:"user_id" validate:"required"`
	Platform         string      `db:"platform" validate:"required"`
	PlatformUsername string      `db:"platform_username" validate:"required_without=PlatformID"`
	PlatformID       string      `db:"platform_id" validate:"required_without=PlatformUsername"`
	Data             interface{} `db:"data" validate:"required"`
	DataUpdatedAt    time.Time   `db:"updated_at"`
	CreatedAt        time.Time   `db:"created_at"`
}

// NewLinkedAccount creates a new linked account
func NewLinkedAccount(userID string, platform, platformUsername, platformID string, data Data) *LinkedAccount {
	return &LinkedAccount{
		UserID:           userID,
		Platform:         platform,
		PlatformUsername: platformUsername,
		PlatformID:       platformID,
		Data:             data,
	}
}

// Data interface
type Data interface {
	PlatformID() string
	PlatformUsername() string
	PlatformData() string
	CreateLinkedAccount(string) *LinkedAccount
}

// -------------- Enums --------------

var (
	PlatformDiscord   = "discord"
	PlatformMinecraft = "minecraft"
	PlatformTwitch    = "twitch"
)
