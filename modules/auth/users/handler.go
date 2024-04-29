package users

import (
	"net/http"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	accountlinking "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	db := database.GetDB("neuralnexus")
	service := NewService(auth.NewAccountStore(db), accountlinking.NewStore(db))
	mux.HandleFunc("GET /api/v1/users/{user_id}", mw.Auth(GetUserHandler(service)))
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
			responses.SendAndEncodeNotFound(w, r, "User not found")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, user)
	}
}

// UpdateUserHandler - Update a user
func UpdateUserHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("user_id")
		var user auth.Account
		err := responses.DecodeStruct(r, &user)
		if err != nil {
			responses.SendAndEncodeBadRequest(w, r, "Invalid request body")
			return
		}
		user.UserID = userID
		updatedUser, err := service.UpdateUser(&user)
		if err != nil {
			responses.SendAndEncodeBadRequest(w, r, "Failed to update user")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, updatedUser)
	}
}

// UpdateUserFromPlatformHandler - Update a user from a platform
func UpdateUserFromPlatformHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		platform := accountlinking.Platform(r.PathValue("platform"))
		platformID := r.PathValue("platform_id")
		var data accountlinking.Data
		err := responses.DecodeStruct(r, &data)
		if err != nil {
			responses.SendAndEncodeBadRequest(w, r, "Invalid request body")
			return
		}
		user, err := service.UpdateUserFromPlatform(platform, platformID, data)
		if err != nil {
			responses.SendAndEncodeBadRequest(w, r, "Failed to update user")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, user)
	}
}

// DeleteUserHandler - Delete a user
func DeleteUserHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("user_id")
		err := service.DeleteUser(userID)
		if err != nil {
			responses.SendAndEncodeBadRequest(w, r, "Failed to delete user")
			return
		}
		responses.SendAndEncodeNoContent(w, r)
	}
}
