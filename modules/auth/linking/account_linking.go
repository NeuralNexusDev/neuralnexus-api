package accountlinking

import (
	"context"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CREATE TABLE linked_accounts (
//   user_id UUID NOT NULL,
//   platform TEXT NOT NULL,
//   platform_username TEXT NOT NULL,
//   platform_id TEXT NOT NULL,
//   data JSONB NOT NULL,
//   data_updated_at timestamp with time zone default current_timestamp,
//   created_at timestamp with time zone default current_timestamp,
//   FOREIGN KEY (user_id) REFERENCES accounts(user_id),
//   CONSTRAINT linked_accounts_unique UNIQUE (user_id, platform)
// );

// -------------- Structs --------------

// LinkedAccount struct
type LinkedAccount struct {
	UserID           uuid.UUID   `db:"user_id" validate:"required"`
	Platform         string      `db:"platform" validate:"required"`
	PlatformUsername string      `db:"platform_username" validate:"required_without=PlatformID"`
	PlatformID       string      `db:"platform_id" validate:"required_without=PlatformUsername"`
	Data             interface{} `db:"data" validate:"required"`
	DataUpdatedAt    time.Time   `db:"data_updated_at"`
	CreatedAt        time.Time   `db:"created_at"`
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

func AddLinkedAccountToDB(linkedAccount LinkedAccount) database.Response[LinkedAccount] {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	_, err := db.Exec(context.Background(), "INSERT INTO linked_accounts (user_id, platform, platform_username, platform_id, data) VALUES ($1, $2, $3, $4, $5)", linkedAccount.UserID, linkedAccount.Platform, linkedAccount.PlatformUsername, linkedAccount.PlatformID, linkedAccount.Data)
	if err != nil {
		return database.ErrorResponse[LinkedAccount]("Failed to create linked account", err)
	}
	return database.SuccessResponse(linkedAccount)
}

func GetLinkedAccountByPlatformID(platform, platformID string) database.Response[LinkedAccount] {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	rows, err := db.Query(context.Background(), "SELECT * FROM linked_accounts WHERE platform = $1 AND platform_id = $2", platform, platformID)
	if err != nil {
		return database.ErrorResponse[LinkedAccount]("Failed to get linked account", err)
	}

	linkedAccount, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[LinkedAccount])
	if err != nil {
		return database.ErrorResponse[LinkedAccount]("Failed to get linked account", err)
	}
	return database.SuccessResponse(*linkedAccount)
}

func GetLinkedAccountByUserID(userID uuid.UUID, platform string) database.Response[LinkedAccount] {
	db := database.GetDB("neuralnexus")
	defer db.Close()

	rows, err := db.Query(context.Background(), "SELECT * FROM linked_accounts WHERE user_id = $1 AND platform = $2", userID, platform)
	if err != nil {
		return database.ErrorResponse[LinkedAccount]("Failed to get linked account", err)
	}

	linkedAccount, err := pgx.CollectExactlyOneRow(rows, pgx.RowToAddrOfStructByName[LinkedAccount])
	if err != nil {
		return database.ErrorResponse[LinkedAccount]("Failed to get linked account", err)
	}
	return database.SuccessResponse(*linkedAccount)
}

// -------------- Handlers --------------
