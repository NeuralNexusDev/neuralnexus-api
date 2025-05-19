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
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// Login struct for login request
type Login struct {
	Username string `json:"username" xml:"username" validate:"required_without=GetEmail"`
	Email    string `json:"email" xml:"email" validate:"required_without=GetUsername"`
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

// OAuthHandler handles the OAuth route
func OAuthHandler(as auth.AccountService, las auth.LinkAccountStore, ss auth.SessionService) http.HandlerFunc {
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
		var state linking.OAuthState
		err = json.Unmarshal(stateBytes, &state)
		if err != nil {
			log.Println("Failed to unmarshal state:\n\t", err)
			responses.BadRequest(w, r, "Invalid state")
			return
		}
		if state.Platform == "" || state.Nonce == "" || state.RedirectURI == "" || state.Mode == "" {
			log.Println("Invalid state")
			responses.BadRequest(w, r, "Invalid state")
			return
		}

		// Verify that the nonce matches the value in the browser's cookie
		cookie, err := r.Cookie("nonce")
		if err != nil {
			log.Println("Failed to get nonce cookie:\n\t", err)
			responses.BadRequest(w, r, "Invalid state")
			return
		}
		if cookie.Value != state.Nonce {
			log.Println("Nonce does not match")
			responses.BadRequest(w, r, "Invalid state")
			return
		}

		var session *auth.Session
		switch state.Mode {
		case linking.ModeLogin:
			session, err = linking.ProcessOAuthLogin(as, las, ss, code, &state)
		default:
			log.Println("Invalid mode")
			responses.BadRequest(w, r, "Invalid state")
		}

		if err != nil {
			log.Println("Failed to process OAuth:\n\t", err)
			responses.InternalServerError(w, r, "Authentication failed")
			return
		}

		// Set the session cookie and redirect the user
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
	}
}
