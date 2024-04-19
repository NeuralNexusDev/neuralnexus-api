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
	db := database.GetDB("neuralnexus")
	rdb := database.GetRedis()
	acctStore := auth.NewAccountStore(db)
	sessStore := auth.NewSessionStore(db, rdb)
	alstore := accountlinking.NewStore(db)

	mux.HandleFunc("POST /api/v1/auth/login", LoginHandler(acctStore, sessStore))
	mux.Handle("POST /api/v1/auth/logout", mw.Auth(LogoutHandler(sessStore)))

	mux.HandleFunc("/api/oauth", OAuthHandler(acctStore, sessStore, alstore))
	return mux
}

// LoginHandler handles the login route
func LoginHandler(as auth.AccountStore, ss auth.SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		var account *auth.Account
		if login.Username != "" {
			account, err = as.GetAccountByUsername(login.Username)
		} else {
			account, err = as.GetAccountByEmail(login.Email)
		}
		if err != nil {
			responses.SendAndEncodeBadRequest(w, r, "Invalid username or email")
			return
		}

		if !account.ValidateUser(login.Password) {
			responses.SendAndEncodeBadRequest(w, r, "Invalid password")
			return
		}

		session := account.NewSession(time.Now().Add(time.Hour * 24).Unix())
		ss.AddSessionToCache(session)
		responses.SendAndEncodeStruct(w, r, http.StatusOK, session)
		ss.AddSessionToDB(session)
	}
}

// LogoutHandler handles the logout route
func LogoutHandler(ss auth.SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		ss.DeleteSessionFromCache(session.ID)
		responses.SendAndEncodeStruct(w, r, http.StatusOK, session)
		ss.DeleteSessionInDB(session.ID)
	}
}

// OAuthHandler handles the Discord OAuth route
func OAuthHandler(as auth.AccountStore, ss auth.SessionStore, las accountlinking.LinkAccountStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			responses.SendAndEncodeBadRequest(w, r, "Invalid request")
			return
		}
		state := r.URL.Query().Get("state")
		session, err := accountlinking.DiscordOAuth(as, ss, las, code, state)
		if err != nil {
			log.Println("Failed to authenticate with Discord:\n\t", err)
			responses.SendAndEncodeBadRequest(w, r, "Failed to authenticate with Discord")
			return
		}
		responses.SendAndEncodeStruct(w, r, http.StatusOK, session)
	}
}
