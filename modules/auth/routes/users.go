package authroutes

import (
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"net/http"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// GetUserHandler - Get a user
func GetUserHandler(service auth.UserService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("user_id")
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
		err := responses.DecodeStruct(r, &data)
		if err != nil {
			responses.BadRequest(w, r, "Invalid request body")
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
