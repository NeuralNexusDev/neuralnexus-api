package authroutes

import (
	"log"
	"net/http"
	"time"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	accountlinking "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
	sess "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// -------------- Routes --------------

// ApplyRoutes applies the auth routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	db := database.GetDB("neuralnexus")
	rdb := database.GetRedis()
	acctStore := auth.NewAccountStore(db)
	sessStore := sess.NewSessionStore(db, rdb)
	alstore := accountlinking.NewStore(db)

	mux.HandleFunc("POST /api/v1/auth/login", LoginHandler(acctStore, sessStore))
	mux.Handle("POST /api/v1/auth/logout", mw.Auth(LogoutHandler(sessStore)))

	mux.HandleFunc("/api/oauth", OAuthHandler(acctStore, sessStore, alstore))
	return mux
}

// LoginHandler handles the login route
func LoginHandler(as auth.AccountStore, ss sess.SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var login struct {
			Username string `json:"username" xml:"username" validate:"required_without=Email"`
			Email    string `json:"email" xml:"email" validate:"required_without=Username"`
			Password string `json:"password" xml:"password" validate:"required"`
		}
		err := responses.DecodeStruct(r, &login)
		if err != nil {
			responses.BadRequest(w, r, "Invalid request body")
			return
		}

		var account *auth.Account
		if login.Username != "" {
			account, err = as.GetAccountByUsername(login.Username)
		} else {
			account, err = as.GetAccountByEmail(login.Email)
		}
		if err != nil {
			responses.BadRequest(w, r, "Invalid username or email")
			return
		}

		if !account.ValidateUser(login.Password) {
			responses.BadRequest(w, r, "Invalid password")
			return
		}

		session, err := account.NewSession(time.Now().Add(time.Hour * 24).Unix())
		if err != nil {
			responses.BadRequest(w, r, "Failed to create session")
			return
		}
		ss.AddSessionToCache(session)
		responses.StructOK(w, r, session)
		ss.AddSessionToDB(session)
	}
}

// LogoutHandler handles the logout route
func LogoutHandler(ss sess.SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*sess.Session)
		ss.DeleteSessionFromCache(session.ID)
		responses.StructOK(w, r, session)
		ss.DeleteSessionInDB(session.ID)
	}
}

// OAuthHandler handles the Discord OAuth route
func OAuthHandler(as auth.AccountStore, ss sess.SessionStore, las accountlinking.LinkAccountStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			responses.BadRequest(w, r, "Invalid request")
			return
		}
		state := any(r.URL.Query().Get("state")).(accountlinking.Platform)
		var session *sess.Session
		var err error
		if state == accountlinking.PlatformDiscord {
			session, err = accountlinking.DiscordOAuth(as, ss, las, code, state)
			if err != nil {
				log.Println("Failed to authenticate with Discord:\n\t", err)
				responses.BadRequest(w, r, "Failed to authenticate with Discord")
				return
			}
		} else if state == accountlinking.PlatformTwitch {
			session, err = accountlinking.TwitchOAuth(as, ss, las, code, state)
			if err != nil {
				log.Println("Failed to authenticate with Twitch:\n\t", err)
				responses.BadRequest(w, r, "Failed to authenticate with Twitch")
				return
			}
		}
		responses.StructOK(w, r, session)
	}
}
