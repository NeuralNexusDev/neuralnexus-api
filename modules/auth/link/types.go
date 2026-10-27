package link

import (
	"time"
)

const (
	HeaderAccept             = "Accept"
	HeaderAuthorization      = "Authorization"
	HeaderContentType        = "Content-Type"
	HeaderXBLContractVersion = "x-xbl-contract-version"
	ContentTypeJSON          = "application/json"
	AuthBearer               = "Bearer "
)

// LinkedAccount struct
type LinkedAccount struct {
	UserID           string      `db:"user_id" validate:"required" json:"user_id" xml:"user_id"`
	Platform         Platform    `db:"platform" validate:"required" json:"platform" xml:"platform"`
	PlatformUsername string      `db:"platform_username" validate:"required_without=GetID" json:"platform_username" xml:"platform_username"`
	PlatformID       string      `db:"platform_id" validate:"required_without=GetUsername" json:"platform_id" xml:"platform_id"`
	Data             interface{} `db:"data" validate:"required" json:"data" xml:"data"`
	Verified         bool        `db:"verified" json:"verified" xml:"verified"`
	LoginEnabled     bool        `db:"login_enabled" json:"login_enabled" xml:"login_enabled"`
	DataUpdatedAt    time.Time   `db:"updated_at" json:"updated_at" xml:"updated_at"`
	CreatedAt        time.Time   `db:"created_at" json:"created_at" xml:"created_at"`
}

// NewLinkedAccount creates a new, verified, login-enabled linked account.
func NewLinkedAccount(userID string, platform Platform, platformUsername, platformID string, data PlatformData) *LinkedAccount {
	return &LinkedAccount{
		UserID:           userID,
		Platform:         platform,
		PlatformUsername: platformUsername,
		PlatformID:       platformID,
		Data:             data,
		Verified:         true,
		LoginEnabled:     true,
	}
}

// PlatformData interface
type PlatformData interface {
	GetID() string
	GetEmail() string
	GetUsername() string
	GetData() string
	CreateLinkedAccount(string) *LinkedAccount
}

// Platform - The external platform a linked account belongs to
type Platform string

var (
	PlatformDiscord   Platform = "discord"
	PlatformMinecraft Platform = "minecraft"
	PlatformTwitch    Platform = "twitch"
	PlatformXboxLive  Platform = "xboxlive"
	PlatformMicrosoft Platform = "microsoft"
	PlatformSteam     Platform = "steam"
)

// OAuthToken OAuth2 token with scope
type OAuthToken struct {
	AccessToken  string    `json:"access_token" db:"access_token"`
	TokenType    string    `json:"token_type,omitempty" db:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty" db:"refresh_token"`
	Expiry       int64     `json:"expiry,omitempty" db:"expiry"`
	ExpiresIn    int64     `json:"expires_in,omitempty" db:"expires_in"`
	UserID       string    `json:"user_id" db:"user_id"`
	Platform     Platform  `json:"platform" db:"platform"`
	Scope        []string  `json:"scope" db:"scope"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}
