package authroutes

import (
	"encoding/base64"
	"github.com/goccy/go-json"
	"log"
	"net/http"
	"time"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/linking"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// -------------- Routes --------------

// ApplyRoutes applies the auth routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	db := database.GetDB("neuralnexus")
	rdb := database.GetRedis()
	store := auth.NewStore(db, rdb)
	account := auth.NewAccountService(store)
	session := auth.NewSessionService(store)
	user := auth.NewUserService(store)

	mux.HandleFunc("POST /api/v1/auth/login", LoginHandler(account, session))
	mux.Handle("POST /api/v1/auth/logout", mw.Auth(session, LogoutHandler(session)))

	mux.HandleFunc("/api/oauth", OAuthHandler(session, store))

	mux.HandleFunc("GET /api/v1/users/{user_id}", mw.Auth(session, GetUserHandler(user)))
	mux.HandleFunc("GET /api/v1/users/{user_id}/permissions", mw.Auth(session, GetUserPermissionsHandler(user)))
	mux.HandleFunc("GET /api/v1/users/{platform}/{platform_id}", mw.Auth(session, GetUserFromPlatformHandler(user)))
	mux.HandleFunc("PUT /api/v1/users/{user_id}", mw.Auth(session, UpdateUserHandler(user)))
	mux.HandleFunc("PUT /api/v1/users/{platform}/{platform_id}", mw.Auth(session, UpdateUserFromPlatformHandler(user)))
	// mux.HandleFunc("DELETE /api/v1/users/{user_id}", mw.Auth(session, DeleteUserHandler(service)))

	return mux
}

// Login struct for login request
type Login struct {
	Username string `json:"username" xml:"username" validate:"required_without=Email"`
	Email    string `json:"email" xml:"email" validate:"required_without=Username"`
	Password string `json:"password" xml:"password" validate:"required"`
}

// ReturnedJWT struct for JWT session
type ReturnedJWT struct {
	Session string `json:"session" xml:"session"`
}

// LoginHandler handles the login route
func LoginHandler(as auth.AccountService, ss auth.SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var login Login
		err := responses.DecodeStruct(r, &login)
		if err != nil {
			responses.BadRequest(w, r, "Invalid username or password")
			return
		}

		var account *auth.Account
		if login.Username != "" {
			account, err = as.GetAccountByUsername(login.Username)
		} else {
			account, err = as.GetAccountByEmail(login.Email)
		}
		if err != nil {
			responses.BadRequest(w, r, "Invalid username or password")
			return
		}

		if !account.ValidateUser(login.Password) {
			responses.BadRequest(w, r, "Invalid username or password")
			return
		}

		session, err := account.NewSession(time.Now().Add(time.Hour * 24).Unix())
		if err != nil {
			log.Println("Failed to create session:\n\t", err)
			responses.InternalServerError(w, r, "Authentication failed")
			return
		}

		jwt, err := ss.CreateJWT(session)
		if err != nil {
			log.Println("Failed to create JWT:\n\t", err)
			responses.InternalServerError(w, r, "Authentication failed")
			return
		}

		err = ss.AddSession(session)
		if err != nil {
			log.Println("Failed to add session:\n\t", err)
			responses.InternalServerError(w, r, "Authentication failed")
			return
		}
		responses.StructOK(w, r, ReturnedJWT{jwt})
	}
}

// LogoutHandler handles the logout route
func LogoutHandler(ss auth.SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*auth.Session)
		if session == nil {
			responses.BadRequest(w, r, "Invalid session")
			return
		}
		err := ss.DeleteSession(session.ID)
		if err != nil {
			log.Println("Failed to delete session:\n\t", err)
			responses.InternalServerError(w, r, "Failed to delete session")
			return
		}
		responses.NoContent(w, r)
	}
}

// OAuthHandler handles the Discord OAuth route
func OAuthHandler(ss auth.SessionService, store auth.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		code := r.URL.Query().Get("code")
		if code == "" {
			log.Println("No code provided")
			responses.BadRequest(w, r, "Invalid request")
			return
		}

		stateB64 := r.URL.Query().Get("state")
		if stateB64 == "" {
			log.Println("No state provided")
			responses.BadRequest(w, r, "Invalid request")
			return
		}
		stateBytes, err := base64.URLEncoding.DecodeString(stateB64)
		if err != nil {
			log.Println("Failed to decode state:\n\t", err)
			responses.BadRequest(w, r, "Invalid state")
			return
		}
		var state auth.OAuthState
		err = json.Unmarshal(stateBytes, &state)
		if err != nil {
			log.Println("Failed to unmarshal state:\n\t", err)
			responses.BadRequest(w, r, "Invalid state")
			return
		}

		var session *auth.Session
		if state.Platform == auth.PlatformDiscord {
			session, err = linking.DiscordOAuth(store, code, state)
			if err != nil {
				log.Println("Failed to authenticate with Discord:\n\t", err)
				responses.BadRequest(w, r, "Failed to authenticate with Discord")
				return
			}
		} else if state.Platform == auth.PlatformTwitch {
			session, err = linking.TwitchOAuth(store, code, state)
			if err != nil {
				log.Println("Failed to authenticate with Twitch:\n\t", err)
				responses.BadRequest(w, r, "Failed to authenticate with Twitch")
				return
			}
		}

		// If the state contains a redirect URI, set the session cookie and redirect
		if state.RedirectURI != "" {
			jwtString, err := ss.CreateJWT(session)
			if err != nil {
				log.Println("Failed to create JWT:\n\t", err)
				responses.InternalServerError(w, r, "Authentication failed")
				return
			}
			http.SetCookie(w, &http.Cookie{
				Name:    "session",
				Value:   jwtString,
				Domain:  ".neuralnexus.dev",
				Path:    "/",
				Expires: time.Unix(session.ExpiresAt, 0),
				Secure:  true,
			})

			http.Redirect(w, r, state.RedirectURI, http.StatusSeeOther)
			return
		}

		responses.StructOK(w, r, session)
	}
}
