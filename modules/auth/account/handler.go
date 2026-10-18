package account

import (
	"errors"
	"log"
	"net/http"
	"time"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

const (
	msgInvalidUsernameOrPassword = "Invalid username or password"
	msgAuthenticationFailed      = "Authentication failed"
	msgInvalidSession            = "Invalid session"
	msgFailedToDeleteSession     = "Failed to delete session"
	logFailedToCreateJWT         = "Failed to create JWT:\n\t"
)

// Login struct for login request
type Login struct {
	Username string `json:"username" xml:"username" validate:"required_without=GetEmail"`
	Email    string `json:"email" xml:"email" validate:"required_without=GetUsername"`
	Password string `json:"password" xml:"password" validate:"required"`
}

// LoginHandler handles the login route
func LoginHandler(as Service, ss session.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var login Login
		err := responses.DecodeStruct(r, &login)
		if err != nil {
			responses.BadRequest(w, r, msgInvalidUsernameOrPassword)
			return
		}

		var account *Account
		if login.Username != "" {
			account, err = as.GetAccountByUsername(login.Username)
		} else {
			account, err = as.GetAccountByEmail(login.Email)
		}
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				log.Println("Failed to look up account:\n\t", err)
				responses.InternalServerError(w, r, msgAuthenticationFailed)
				return
			}
			DummyValidateUser(login.Password)
			responses.BadRequest(w, r, msgInvalidUsernameOrPassword)
			return
		}

		if !account.ValidateUser(login.Password) {
			responses.BadRequest(w, r, msgInvalidUsernameOrPassword)
			return
		}

		enabled, err := as.IsPasswordAuthEnabled(account.UserID)
		if err != nil {
			log.Println("Failed to check password auth setting:\n\t", err)
			responses.InternalServerError(w, r, msgAuthenticationFailed)
			return
		}
		if !enabled {
			responses.BadRequest(w, r, msgInvalidUsernameOrPassword)
			return
		}

		session, err := as.NewSession(account, time.Now().Add(time.Hour*24).Unix())
		if err != nil {
			log.Println("Failed to create session:\n\t", err)
			responses.InternalServerError(w, r, msgAuthenticationFailed)
			return
		}

		if err = ss.AddSession(session); err != nil {
			log.Println("Failed to add session:\n\t", err)
			responses.InternalServerError(w, r, msgAuthenticationFailed)
			return
		}

		if err := SetSessionCookie(ss, w, session); err != nil {
			log.Println(logFailedToCreateJWT, err)
			responses.InternalServerError(w, r, msgAuthenticationFailed)
			return
		}
		responses.NoContent(w, r)
	}
}

// LogoutHandler handles the logout route
func LogoutHandler(ss session.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(mw.SessionKey).(*session.Session)
		if session == nil {
			responses.BadRequest(w, r, msgInvalidSession)
			return
		}
		err := ss.DeleteSession(session.ID)
		if err != nil {
			log.Println("Failed to delete session:\n\t", err)
			responses.InternalServerError(w, r, msgFailedToDeleteSession)
			return
		}
		http.SetCookie(w, sessionCookie("", time.Unix(0, 0)))
		responses.NoContent(w, r)
	}
}

// SetSessionCookie creates a JWT for an already-persisted
// session and sets it as the session cookie.
func SetSessionCookie(ss session.Service, w http.ResponseWriter, session *session.Session) error {
	jwtString, err := ss.CreateJWT(session)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessionCookie(jwtString, time.Unix(session.ExpiresAt, 0)))
	return nil
}

// sessionCookie builds the session cookie
func sessionCookie(value string, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     mw.SessionCookieName,
		Value:    value,
		Domain:   ".neuralnexus.dev",
		Path:     "/",
		Expires:  expires,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}
