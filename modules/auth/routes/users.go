package authroutes

import (
	"errors"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"net/http"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/twitch"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// GetUserHandler - Get a user
func GetUserHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		userID := r.PathValue("user_id")
		// Locked down to self-lookup and admins for now. Cross-user lookup
		// with the target's consent is a planned feature (for account-link
		// integrations) but isn't implemented yet - don't open this up
		// generally until that consent mechanism actually exists.
		if session.UserID != userID && !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, "You do not have permission to get this user")
			return
		}
		user, err := service.GetUser(userID)
		if err != nil {
			responses.NotFound(w, r, "User not found")
			return
		}
		responses.StructOK(w, r, user)
	}
}

// GetUserFromPlatformHandler - Get a user from a platform
func GetUserFromPlatformHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, "You do not have permission to get users")
			return
		}
		platform := auth.Platform(r.PathValue("platform"))
		platformID := r.PathValue("platform_id")
		user, err := service.GetUserFromPlatform(platform, platformID)
		if err != nil {
			responses.NotFound(w, r, "User not found")
			return
		}
		responses.StructOK(w, r, user)
	}
}

// GetUserPermissionsHandler - Get a user's permissions
func GetUserPermissionsHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		userID := r.PathValue("user_id")
		if session.UserID != userID && !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, "You do not have permission to get user permissions")
			return
		}
		permissions, err := service.GetUserPermissions(userID)
		if err != nil {
			responses.NotFound(w, r, "User not found")
			return
		}
		responses.StructOK(w, r, permissions)
	}
}

// UpdateUserHandler - Update a user
func UpdateUserHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, "You do not have permission to update users")
			return
		}
		userID := r.PathValue("user_id")
		var user auth.Account
		err := responses.DecodeStruct(r, &user)
		if err != nil {
			responses.BadRequest(w, r, "Invalid request body")
			return
		}
		user.UserID = userID
		err = service.UpdateUser(&user)
		if err != nil {
			responses.BadRequest(w, r, "Failed to update user")
			return
		}
		responses.StructOK(w, r, user)
	}
}

// UpdateUserFromPlatformHandler - Update a user from a platform
func UpdateUserFromPlatformHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, "You do not have permission to update users")
			return
		}
		platform := auth.Platform(r.PathValue("platform"))
		platformID := r.PathValue("platform_id")

		var data auth.PlatformData
		switch platform {
		case auth.PlatformDiscord:
			var d linking.DiscordData
			if err := responses.DecodeStruct(r, &d); err != nil {
				responses.BadRequest(w, r, "Invalid request body")
				return
			}
			data = &d
		case auth.PlatformMinecraft:
			var d linking.MinecraftData
			if err := responses.DecodeStruct(r, &d); err != nil {
				responses.BadRequest(w, r, "Invalid request body")
				return
			}
			data = &d
		case auth.PlatformTwitch:
			var d twitch.Data
			if err := responses.DecodeStruct(r, &d); err != nil {
				responses.BadRequest(w, r, "Invalid request body")
				return
			}
			data = &d
		case auth.PlatformXboxLive:
			var d linking.XboxLiveData
			if err := responses.DecodeStruct(r, &d); err != nil {
				responses.BadRequest(w, r, "Invalid request body")
				return
			}
			data = &d
		case auth.PlatformMicrosoft:
			var d linking.MicrosoftUserData
			if err := responses.DecodeStruct(r, &d); err != nil {
				responses.BadRequest(w, r, "Invalid request body")
				return
			}
			data = &d
		default:
			responses.BadRequest(w, r, "Unsupported platform")
			return
		}

		user, err := service.UpdateUserFromPlatform(platform, platformID, data)
		if err != nil {
			responses.BadRequest(w, r, "Failed to update user")
			return
		}
		responses.StructOK(w, r, user)
	}
}

// DeleteUserHandler - Delete a user
func DeleteUserHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, "You do not have permission to delete users")
			return
		}
		userID := r.PathValue("user_id")
		err := service.DeleteUser(userID)
		if err != nil {
			responses.BadRequest(w, r, "Failed to delete user")
			return
		}
		responses.NoContent(w, r)
	}
}

// GetUserLinkedAccountsHandler - List a user's linked platforms
func GetUserLinkedAccountsHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		userID := r.PathValue("user_id")
		if session.UserID != userID && !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, "You do not have permission to view this user's linked accounts")
			return
		}
		links, err := service.GetUserLinkedAccounts(userID)
		if err != nil {
			responses.InternalServerError(w, r, "Failed to get linked accounts")
			return
		}
		responses.StructOK(w, r, links)
	}
}

// UnlinkPlatformHandler - Unlink a platform from a user
func UnlinkPlatformHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		userID := r.PathValue("user_id")
		if session.UserID != userID && !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, "You do not have permission to unlink this user's platforms")
			return
		}
		platform := auth.Platform(r.PathValue("platform"))
		err := service.UnlinkPlatform(userID, platform)
		switch {
		case err == nil:
			responses.NoContent(w, r)
		case errors.Is(err, auth.ErrNotFound):
			responses.NotFound(w, r, "This platform isn't linked to this user")
		case errors.Is(err, auth.ErrWouldLockAccount):
			responses.BadRequest(w, r, "Set a password or link another platform before unlinking your last one")
		default:
			responses.InternalServerError(w, r, "Failed to unlink platform")
		}
	}
}

// SetPlatformLoginEnabledRequest - Body for SetPlatformLoginEnabledHandler
type SetPlatformLoginEnabledRequest struct {
	// LoginEnabled is a pointer so a missing field is rejected instead of
	// silently defaulting to false.
	LoginEnabled *bool `json:"login_enabled" xml:"login_enabled"`
}

// SetPlatformLoginEnabledHandler - Toggle whether a linked platform can log in
func SetPlatformLoginEnabledHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		userID := r.PathValue("user_id")
		if session.UserID != userID && !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, "You do not have permission to update this user's platforms")
			return
		}
		platform := auth.Platform(r.PathValue("platform"))
		var body SetPlatformLoginEnabledRequest
		if err := responses.DecodeStruct(r, &body); err != nil || body.LoginEnabled == nil {
			responses.BadRequest(w, r, "Invalid request body")
			return
		}

		err := service.SetPlatformLoginEnabled(userID, platform, *body.LoginEnabled)
		switch {
		case err == nil:
			responses.NoContent(w, r)
		case errors.Is(err, auth.ErrNotFound):
			responses.NotFound(w, r, "This platform isn't linked to this user")
		case errors.Is(err, auth.ErrWouldLockAccount):
			responses.BadRequest(w, r, "Set a password or link another platform before disabling your last login method")
		case errors.Is(err, auth.ErrLinkedAccountUnverified):
			responses.BadRequest(w, r, "This linked account is unverified and can't be enabled for login")
		default:
			responses.InternalServerError(w, r, "Failed to update platform")
		}
	}
}

// GetAccountSettingsHandler - Get a user's account settings
func GetAccountSettingsHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		userID := r.PathValue("user_id")
		if session.UserID != userID && !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, "You do not have permission to view this user's settings")
			return
		}
		settings, err := service.GetAccountSettings(userID)
		if err != nil {
			responses.InternalServerError(w, r, "Failed to get account settings")
			return
		}
		responses.StructOK(w, r, settings)
	}
}

// UpdateAccountSettingsRequest - Body for UpdateAccountSettingsHandler
type UpdateAccountSettingsRequest struct {
	// PasswordAuthEnabled is a pointer so a missing field is rejected instead
	// of silently defaulting to false.
	PasswordAuthEnabled *bool `json:"password_auth_enabled" xml:"password_auth_enabled"`
}

// UpdateAccountSettingsHandler - Update a user's account settings
func UpdateAccountSettingsHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		userID := r.PathValue("user_id")
		if session.UserID != userID && !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, "You do not have permission to update this user's settings")
			return
		}
		var body UpdateAccountSettingsRequest
		if err := responses.DecodeStruct(r, &body); err != nil || body.PasswordAuthEnabled == nil {
			responses.BadRequest(w, r, "Invalid request body")
			return
		}

		err := service.SetPasswordAuthEnabled(userID, *body.PasswordAuthEnabled)
		switch {
		case err == nil:
			responses.NoContent(w, r)
		case errors.Is(err, auth.ErrWouldLockAccount):
			responses.BadRequest(w, r, "Link and enable another login method before disabling your password")
		case errors.Is(err, auth.ErrNoPasswordSet):
			responses.BadRequest(w, r, "Set a password before enabling password login")
		default:
			responses.InternalServerError(w, r, "Failed to update user settings")
		}
	}
}
