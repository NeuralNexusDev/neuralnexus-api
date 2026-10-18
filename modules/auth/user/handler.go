package user

import (
	"errors"
	authaccount "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	authsession "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"log"
	"net/http"
	"strconv"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

const (
	msgUserNotFound                             = "User not found"
	msgNoPermissionToUpdateUsers                = "You do not have permission to update users"
	msgInvalidRequestBody                       = "Invalid request body"
	msgFailedToUpdateUser                       = "Failed to update user"
	msgNoPermissionToGetUser                    = "You do not have permission to get this user"
	msgNoPermissionToGetUsers                   = "You do not have permission to get users"
	msgNoPermissionToGetUserPermissions         = "You do not have permission to get user permissions"
	msgUnsupportedPlatform                      = "Unsupported platform"
	msgNoPermissionToDeleteUsers                = "You do not have permission to delete users"
	msgFailedToDeleteUser                       = "Failed to delete user"
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
	msgInvalidLimit                             = "The limit must be a whole number from 1 to 200"
	msgInvalidOffset                            = "The offset must be a whole number of 0 or more"
	msgFailedToListUsers                        = "Failed to list users"
	logFailedToListUsers                        = "Failed to list users:\n\t"
)

const (
	defaultListUsersLimit = 50
	maxListUsersLimit     = 200
)

func respondUpdateUserFailure(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, authaccount.ErrNotFound):
		responses.NotFound(w, r, msgUserNotFound)
	case errors.Is(err, authaccount.ErrEmailAlreadyExists):
		responses.Conflict(w, r, msgEmailAlreadyExists)
	case errors.Is(err, authaccount.ErrUsernameAlreadyExists):
		responses.Conflict(w, r, msgUsernameAlreadyExists)
	case errors.Is(err, authaccount.ErrInvalidRoleID):
		responses.BadRequest(w, r, msgInvalidRoleID)
	case errors.Is(err, authaccount.ErrUnknownRoleID):
		responses.BadRequest(w, r, msgUnknownRoleID)
	default:
		log.Println(logFailedToUpdateUser, err)
		responses.InternalServerError(w, r, msgFailedToUpdateUser)
	}
}

// ListUsersHandler - List a page of users
func ListUsersHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*authsession.Session)
		if !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, msgNoPermissionToGetUsers)
			return
		}
		limit := defaultListUsersLimit
		if raw := r.URL.Query().Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > maxListUsersLimit {
				responses.BadRequest(w, r, msgInvalidLimit)
				return
			}
			limit = n
		}
		offset := 0
		if raw := r.URL.Query().Get("offset"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 {
				responses.BadRequest(w, r, msgInvalidOffset)
				return
			}
			offset = n
		}
		users, err := service.ListUsers(limit, offset)
		if err != nil {
			log.Println(logFailedToListUsers, err)
			responses.InternalServerError(w, r, msgFailedToListUsers)
			return
		}
		if users == nil {
			users = []*User{}
		}
		responses.StructOK(w, r, users)
	}
}

// GetUserHandler - Get a user
func GetUserHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*authsession.Session)
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
			if errors.Is(err, authaccount.ErrNotFound) {
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
func GetUserFromPlatformHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*authsession.Session)
		if !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, msgNoPermissionToGetUsers)
			return
		}
		platform := linking.Platform(r.PathValue("platform"))
		platformID := r.PathValue("platform_id")
		user, err := service.GetUserFromPlatform(platform, platformID)
		if err != nil {
			if errors.Is(err, authaccount.ErrNotFound) {
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
func GetUserPermissionsHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*authsession.Session)
		userID := r.PathValue("user_id")
		if session.UserID != userID && !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, msgNoPermissionToGetUserPermissions)
			return
		}
		permissions, err := service.GetUserPermissions(userID)
		if err != nil {
			if errors.Is(err, authaccount.ErrNotFound) {
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
func UpdateUserHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*authsession.Session)
		if !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, msgNoPermissionToUpdateUsers)
			return
		}
		userID := r.PathValue("user_id")
		var update UserUpdate
		err := responses.DecodeStruct(r, &update)
		if err != nil {
			responses.BadRequest(w, r, msgInvalidRequestBody)
			return
		}
		err = service.UpdateUser(userID, &update)
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
func UpdateUserFromPlatformHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*authsession.Session)
		if !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, msgNoPermissionToUpdateUsers)
			return
		}
		platform := linking.Platform(r.PathValue("platform"))
		platformID := r.PathValue("platform_id")

		var data linking.PlatformData
		switch platform {
		case linking.PlatformDiscord:
			var d linking.DiscordData
			if err := responses.DecodeStruct(r, &d); err != nil {
				responses.BadRequest(w, r, msgInvalidRequestBody)
				return
			}
			data = &d
		case linking.PlatformMinecraft:
			var d linking.MinecraftData
			if err := responses.DecodeStruct(r, &d); err != nil {
				responses.BadRequest(w, r, msgInvalidRequestBody)
				return
			}
			data = &d
		case linking.PlatformTwitch:
			var d linking.TwitchData
			if err := responses.DecodeStruct(r, &d); err != nil {
				responses.BadRequest(w, r, msgInvalidRequestBody)
				return
			}
			data = &d
		case linking.PlatformXboxLive:
			var d linking.XboxLiveData
			if err := responses.DecodeStruct(r, &d); err != nil {
				responses.BadRequest(w, r, msgInvalidRequestBody)
				return
			}
			data = &d
		case linking.PlatformMicrosoft:
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
func DeleteUserHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*authsession.Session)
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

// GetAccountSettingsHandler - Get a user's account settings
func GetAccountSettingsHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*authsession.Session)
		userID := r.PathValue("user_id")
		if session.UserID != userID && !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, msgNoPermissionToViewSettings)
			return
		}
		settings, err := service.GetAccountSettings(userID)
		if err != nil {
			if errors.Is(err, authaccount.ErrNotFound) {
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
func UpdateAccountSettingsHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*authsession.Session)
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
		case errors.Is(err, authaccount.ErrNotFound):
			responses.NotFound(w, r, msgUserNotFound)
		case errors.Is(err, authaccount.ErrWouldLockAccount):
			responses.BadRequest(w, r, msgEnableLoginMethodBeforeDisablingPassword)
		case errors.Is(err, authaccount.ErrNoPasswordSet):
			responses.BadRequest(w, r, msgSetPasswordBeforeEnablingPasswordLogin)
		default:
			responses.InternalServerError(w, r, msgFailedToUpdateUserSettings)
		}
	}
}
