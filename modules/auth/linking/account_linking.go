package account_linking

import "github.com/google/uuid"

// CREATE TABLE linked_accounts (
// 	user_id UUID FOREIGN KEY REFERENCES accounts(user_id),
// 	platform TEXT NOT NULL,
//  platform_username TEXT NOT NULL,
// 	platform_id TEXT NOT NULL,
//  data JSONB NOT NULL,
//  data_updated_at timestamp with time zone default current_timestamp,
// 	created_at timestamp with time zone default current_timestamp,
//  CONSTRAINT linked_accounts_unique UNIQUE (user_id, platform)
// );

// -------------- Structs --------------

// LinkedAccount struct
type LinkedAccount struct {
	UserID           uuid.UUID `db:"user_id" validate:"required"`
	Platform         string    `db:"platform" validate:"required"`
	PlatformUsername string    `db:"platform_username" validate:"required_without=PlatformID"`
	PlatformID       string    `db:"platform_id" validate:"required_without=PlatformUsername"`
	Data             Data      `db:"data" validate:"required"`
	DataUpdatedAt    string    `db:"data_updated_at"`
	CreatedAt        string    `db:"created_at"`
}

// NewLinkedAccount creates a new linked account
func NewLinkedAccount(userID uuid.UUID, platform, platformUsername, platformID string, data Data) LinkedAccount {
	return LinkedAccount{
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
	CreateLinkedAccount(uuid.UUID) LinkedAccount
}

// -------------- Enums --------------

var (
	PlatformDiscord   = "discord"
	PlatformMinecraft = "minecraft"
	PlatformTwitch    = "twitch"
)

// -------------- Functions --------------

// -------------- Handlers --------------
