package users

import (
	"net/http"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	accountlinking "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
	perms "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	sess "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	db := database.GetDB("neuralnexus")
	service := NewService(auth.NewAccountStore(db), accountlinking.NewStore(db))
	mux.HandleFunc("GET /api/v1/users/{user_id}", mw.Auth(GetUserHandler(service)))
	mux.HandleFunc("GET /api/v1/users/{user_id}/permissions", mw.Auth(GetUserPermissionsHandler(service)))
	mux.HandleFunc("GET /api/v1/users/{platform}/{platform_id}", mw.Auth(GetUserFromPlatformHandler(service)))
	mux.HandleFunc("PUT /api/v1/users/{user_id}", mw.Auth(UpdateUserHandler(service)))
	mux.HandleFunc("PUT /api/v1/users/{platform}/{platform_id}", mw.Auth(UpdateUserFromPlatformHandler(service)))
	mux.HandleFunc("DELETE /api/v1/users/{user_id}", mw.Auth(DeleteUserHandler(service)))
	return mux
}

// GetUserHandler - Get a user
func GetUserHandler(service Service) http.HandlerFunc {
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
func GetUserFromPlatformHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*sess.Session)
		if !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, "You do not have permission to get users")
			return
		}
		platform := accountlinking.Platform(r.PathValue("platform"))
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
func GetUserPermissionsHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*sess.Session)
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
func UpdateUserHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*sess.Session)
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
		updatedUser, err := service.UpdateUser(&user)
		if err != nil {
			responses.BadRequest(w, r, "Failed to update user")
			return
		}
		responses.StructOK(w, r, updatedUser)
	}
}

// UpdateUserFromPlatformHandler - Update a user from a platform
func UpdateUserFromPlatformHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*sess.Session)
		if !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, "You do not have permission to update users")
			return
		}
		platform := accountlinking.Platform(r.PathValue("platform"))
		platformID := r.PathValue("platform_id")
		var data accountlinking.Data
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
func DeleteUserHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*sess.Session)
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
