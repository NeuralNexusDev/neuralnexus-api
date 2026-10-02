package authroutes

import (
	"errors"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"log"
	"net/http"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/twitch"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

const (
	msgUserNotFound                             = "User not found"
	msgNoPermissionToUpdateUsers                = "You do not have permission to update users"
	msgInvalidRequestBody                       = "Invalid request body"
	msgFailedToUpdateUser                       = "Failed to update user"
	msgPlatformNotLinked                        = "This platform isn't linked to this user"
	msgNoPermissionToGetUser                    = "You do not have permission to get this user"
	msgNoPermissionToGetUsers                   = "You do not have permission to get users"
	msgNoPermissionToGetUserPermissions         = "You do not have permission to get user permissions"
	msgUnsupportedPlatform                      = "Unsupported platform"
	msgNoPermissionToDeleteUsers                = "You do not have permission to delete users"
	msgFailedToDeleteUser                       = "Failed to delete user"
	msgNoPermissionToViewLinkedAccounts         = "You do not have permission to view this user's linked accounts"
	msgFailedToGetLinkedAccounts                = "Failed to get linked accounts"
	msgNoPermissionToUnlinkPlatforms            = "You do not have permission to unlink this user's platforms"
	msgSetPasswordOrLinkBeforeUnlinking         = "Set a password or link another platform before unlinking your last one"
	msgFailedToUnlinkPlatform                   = "Failed to unlink platform"
	msgNoPermissionToUpdatePlatforms            = "You do not have permission to update this user's platforms"
	msgSetPasswordOrLinkBeforeDisabling         = "Set a password or link another platform before disabling your last login method"
	msgLinkedAccountUnverified                  = "This linked account is unverified and can't be enabled for login"
	msgFailedToUpdatePlatform                   = "Failed to update platform"
	msgNoPermissionToViewSettings               = "You do not have permission to view this user's settings"
	msgFailedToGetAccountSettings               = "Failed to get account settings"
	msgNoPermissionToUpdateSettings             = "You do not have permission to update this user's settings"
	msgEnableLoginMethodBeforeDisablingPassword = "Link and enable another login method before disabling your password"
	msgSetPasswordBeforeEnablingPasswordLogin   = "Set a password before enabling password login"
	msgFailedToUpdateUserSettings               = "Failed to update user settings"
	msgFailedToGetUser                          = "Failed to get user"
	logFailedToGetUser                          = "Failed to get user:\n\t"
	logFailedToDeleteUser                       = "Failed to delete user:\n\t"
	logFailedToUpdateUser                       = "Failed to update user:\n\t"
	msgEmailAlreadyExists                       = "An account with this email already exists"
	msgUsernameAlreadyExists                    = "An account with this username already exists"
	msgInvalidRoleID                            = "Roles must be role IDs"
	msgUnknownRoleID                            = "Roles must be existing roles"
)

func respondUpdateUserFailure(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrNotFound):
		responses.NotFound(w, r, msgUserNotFound)
	case errors.Is(err, auth.ErrEmailAlreadyExists):
		responses.Conflict(w, r, msgEmailAlreadyExists)
	case errors.Is(err, auth.ErrUsernameAlreadyExists):
		responses.Conflict(w, r, msgUsernameAlreadyExists)
	case errors.Is(err, auth.ErrInvalidRoleID):
		responses.BadRequest(w, r, msgInvalidRoleID)
	case errors.Is(err, auth.ErrUnknownRoleID):
		responses.BadRequest(w, r, msgUnknownRoleID)
	default:
		log.Println(logFailedToUpdateUser, err)
		responses.InternalServerError(w, r, msgFailedToUpdateUser)
	}
}

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
			responses.Forbidden(w, r, msgNoPermissionToGetUser)
			return
		}
		user, err := service.GetUser(userID)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				responses.NotFound(w, r, msgUserNotFound)
				return
			}
			log.Println(logFailedToGetUser, err)
			responses.InternalServerError(w, r, msgFailedToGetUser)
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
			responses.Forbidden(w, r, msgNoPermissionToGetUsers)
			return
		}
		platform := auth.Platform(r.PathValue("platform"))
		platformID := r.PathValue("platform_id")
		user, err := service.GetUserFromPlatform(platform, platformID)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				responses.NotFound(w, r, msgUserNotFound)
				return
			}
			log.Println(logFailedToGetUser, err)
			responses.InternalServerError(w, r, msgFailedToGetUser)
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
			responses.Forbidden(w, r, msgNoPermissionToGetUserPermissions)
			return
		}
		permissions, err := service.GetUserPermissions(userID)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				responses.NotFound(w, r, msgUserNotFound)
				return
			}
			log.Println(logFailedToGetUser, err)
			responses.InternalServerError(w, r, msgFailedToGetUser)
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
			responses.Forbidden(w, r, msgNoPermissionToUpdateUsers)
			return
		}
		userID := r.PathValue("user_id")
		var user auth.Account
		err := responses.DecodeStruct(r, &user)
		if err != nil {
			responses.BadRequest(w, r, msgInvalidRequestBody)
			return
		}
		user.UserID = userID
		err = service.UpdateUser(&user)
		if err != nil {
			respondUpdateUserFailure(w, r, err)
			return
		}
		stored, err := service.GetUser(userID)
		if err != nil {
			respondUpdateUserFailure(w, r, err)
			return
		}
		responses.StructOK(w, r, stored)
	}
}

// UpdateUserFromPlatformHandler - Update a user from a platform
func UpdateUserFromPlatformHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, msgNoPermissionToUpdateUsers)
			return
		}
		platform := auth.Platform(r.PathValue("platform"))
		platformID := r.PathValue("platform_id")

		var data auth.PlatformData
		switch platform {
		case auth.PlatformDiscord:
			var d linking.DiscordData
			if err := responses.DecodeStruct(r, &d); err != nil {
				responses.BadRequest(w, r, msgInvalidRequestBody)
				return
			}
			data = &d
		case auth.PlatformMinecraft:
			var d linking.MinecraftData
			if err := responses.DecodeStruct(r, &d); err != nil {
				responses.BadRequest(w, r, msgInvalidRequestBody)
				return
			}
			data = &d
		case auth.PlatformTwitch:
			var d twitch.Data
			if err := responses.DecodeStruct(r, &d); err != nil {
				responses.BadRequest(w, r, msgInvalidRequestBody)
				return
			}
			data = &d
		case auth.PlatformXboxLive:
			var d linking.XboxLiveData
			if err := responses.DecodeStruct(r, &d); err != nil {
				responses.BadRequest(w, r, msgInvalidRequestBody)
				return
			}
			data = &d
		case auth.PlatformMicrosoft:
			var d linking.MicrosoftUserData
			if err := responses.DecodeStruct(r, &d); err != nil {
				responses.BadRequest(w, r, msgInvalidRequestBody)
				return
			}
			data = &d
		default:
			responses.BadRequest(w, r, msgUnsupportedPlatform)
			return
		}

		user, err := service.UpdateUserFromPlatform(platform, platformID, data)
		if err != nil {
			respondUpdateUserFailure(w, r, err)
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
			responses.Forbidden(w, r, msgNoPermissionToDeleteUsers)
			return
		}
		userID := r.PathValue("user_id")
		err := service.DeleteUser(userID)
		if err != nil {
			log.Println(logFailedToDeleteUser, err)
			responses.InternalServerError(w, r, msgFailedToDeleteUser)
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
			responses.Forbidden(w, r, msgNoPermissionToViewLinkedAccounts)
			return
		}
		links, err := service.GetUserLinkedAccounts(userID)
		if err != nil {
			responses.InternalServerError(w, r, msgFailedToGetLinkedAccounts)
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
			responses.Forbidden(w, r, msgNoPermissionToUnlinkPlatforms)
			return
		}
		platform := auth.Platform(r.PathValue("platform"))
		err := service.UnlinkPlatform(userID, platform)
		switch {
		case err == nil:
			responses.NoContent(w, r)
		case errors.Is(err, auth.ErrNotFound):
			responses.NotFound(w, r, msgPlatformNotLinked)
		case errors.Is(err, auth.ErrWouldLockAccount):
			responses.BadRequest(w, r, msgSetPasswordOrLinkBeforeUnlinking)
		default:
			responses.InternalServerError(w, r, msgFailedToUnlinkPlatform)
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
			responses.Forbidden(w, r, msgNoPermissionToUpdatePlatforms)
			return
		}
		platform := auth.Platform(r.PathValue("platform"))
		var body SetPlatformLoginEnabledRequest
		if err := responses.DecodeStruct(r, &body); err != nil || body.LoginEnabled == nil {
			responses.BadRequest(w, r, msgInvalidRequestBody)
			return
		}

		err := service.SetPlatformLoginEnabled(userID, platform, *body.LoginEnabled)
		switch {
		case err == nil:
			responses.NoContent(w, r)
		case errors.Is(err, auth.ErrNotFound):
			responses.NotFound(w, r, msgPlatformNotLinked)
		case errors.Is(err, auth.ErrWouldLockAccount):
			responses.BadRequest(w, r, msgSetPasswordOrLinkBeforeDisabling)
		case errors.Is(err, auth.ErrLinkedAccountUnverified):
			responses.BadRequest(w, r, msgLinkedAccountUnverified)
		default:
			responses.InternalServerError(w, r, msgFailedToUpdatePlatform)
		}
	}
}

// GetAccountSettingsHandler - Get a user's account settings
func GetAccountSettingsHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		userID := r.PathValue("user_id")
		if session.UserID != userID && !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, msgNoPermissionToViewSettings)
			return
		}
		settings, err := service.GetAccountSettings(userID)
		if err != nil {
			if errors.Is(err, auth.ErrNotFound) {
				responses.NotFound(w, r, msgUserNotFound)
				return
			}
			responses.InternalServerError(w, r, msgFailedToGetAccountSettings)
			return
		}
		responses.StructOK(w, r, settings)
	}
}

// UpdateAccountSettingsRequest - Body for UpdateAccountSettingsHandler. Each
// field is a pointer so a PATCH can update one setting without touching the
// others - a nil field means "leave this alone," the usual PATCH contract
// once there's more than one setting to change independently.
type UpdateAccountSettingsRequest struct {
	PasswordAuthEnabled *bool `json:"password_auth" xml:"password_auth"`
}

// UpdateAccountSettingsHandler - Update a user's account settings
func UpdateAccountSettingsHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		userID := r.PathValue("user_id")
		if session.UserID != userID && !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, msgNoPermissionToUpdateSettings)
			return
		}
		var body UpdateAccountSettingsRequest
		if err := responses.DecodeStruct(r, &body); err != nil || body.PasswordAuthEnabled == nil {
			responses.BadRequest(w, r, msgInvalidRequestBody)
			return
		}

		err := service.SetPasswordAuthEnabled(userID, *body.PasswordAuthEnabled)
		switch {
		case err == nil:
			responses.NoContent(w, r)
		case errors.Is(err, auth.ErrNotFound):
			responses.NotFound(w, r, msgUserNotFound)
		case errors.Is(err, auth.ErrWouldLockAccount):
			responses.BadRequest(w, r, msgEnableLoginMethodBeforeDisablingPassword)
		case errors.Is(err, auth.ErrNoPasswordSet):
			responses.BadRequest(w, r, msgSetPasswordBeforeEnablingPasswordLogin)
		default:
			responses.InternalServerError(w, r, msgFailedToUpdateUserSettings)
		}
	}
}
