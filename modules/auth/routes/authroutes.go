package authroutes

import (
	"log"
	"net/http"
	"time"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	accountlinking "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// -------------- Routes --------------

// ApplyRoutes applies the auth routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	mux.HandleFunc("POST /api/v1/auth/login", LoginHandler)
	mux.Handle("POST /api/v1/auth/logout", mw.Auth(LogoutHandler))
	mux.HandleFunc("/api/oauth", OAuthHandler)
	return mux
}

// LoginHandler handles the login route
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var login struct {
		Username string `json:"username" xml:"username" validate:"required_without=Email"`
		Email    string `json:"email" xml:"email" validate:"required_without=Username"`
		Password string `json:"password" xml:"password" validate:"required"`
	}
	err := responses.DecodeStruct(r, &login)
	if err != nil {
		responses.SendAndEncodeBadRequest(w, r, "Invalid request body")
		return
	}

	var account database.Response[auth.Account]
	if login.Username != "" {
		account = auth.GetAccountByUsername(login.Username)
	} else {
		account = auth.GetAccountByEmail(login.Email)
	}
	if !account.Success {
		responses.SendAndEncodeBadRequest(w, r, "Invalid username or email")
		return
	}

	if !account.Data.ValidateUser(login.Password) {
		responses.SendAndEncodeBadRequest(w, r, "Invalid password")
		return
	}

	session := account.Data.NewSession(time.Now().Add(time.Hour * 24).Unix())
	auth.AddSessionToCache(session)
	responses.SendAndEncodeStruct(w, r, http.StatusOK, session)
	auth.AddSessionToDB(session)
}

// LogoutHandler handles the logout route
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value(mw.SessionKey).(auth.Session)
	auth.DeleteSessionFromCache(session.ID)
	responses.SendAndEncodeStruct(w, r, http.StatusOK, session)
	auth.DeleteSessionInDB(session.ID)
}

// OAuthHandler handles the Discord OAuth route
func OAuthHandler(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		responses.SendAndEncodeBadRequest(w, r, "Invalid request")
		return
	}
	state := r.URL.Query().Get("state")
	session, err := accountlinking.DiscordOAuth(code, state)
	if err != nil {
		log.Println("Failed to authenticate with Discord:\n\t", err)
		responses.SendAndEncodeBadRequest(w, r, "Failed to authenticate with Discord")
		return
	}
	responses.SendAndEncodeStruct(w, r, http.StatusOK, session)
}
