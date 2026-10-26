package account

import (
	"net/http"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

const (
	msgInvalidUsernameOrPassword = "Invalid username or password"
	msgAuthenticationFailed      = "Authentication failed"
	msgInvalidSession            = "Invalid session"
	msgFailedToDeleteSession     = "Failed to delete session"
	logFailedToCreateJWT         = "Failed to create JWT:\n\t"
)

var VoidCookie = &http.Cookie{
	Name:     session.CookieName,
	Value:    "",
	Domain:   ".neuralnexus.dev",
	Path:     "/",
	Expires:  time.Unix(0, 0),
	Secure:   true,
	HttpOnly: true,
	SameSite: http.SameSiteLaxMode,
}

// Login struct for login request
type Login struct {
	Username string `json:"username" xml:"username" validate:"required_without=GetEmail"`
	Email    string `json:"email" xml:"email" validate:"required_without=GetUsername"`
	Password string `json:"password" xml:"password" validate:"required"`
}

// LoginHandler handles the login route
func LoginHandler(cfg config.Account, as Service, ss session.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var login Login
		err := responses.DecodeStruct(r, &login)
		if err != nil {
			responses.BadRequest(w, r, msgInvalidUsernameOrPassword)
			return
		}
		ctx := r.Context()

		var account *Account
		if login.Username != "" {
			account, err = as.GetAccountByUsername(ctx, login.Username)
		} else {
			account, err = as.GetAccountByEmail(ctx, login.Email)
		}
		if err != nil {
			if err != ErrNotFound {
				reqctx.Log(ctx, "Failed to look up account:\n\t", err.Error())
				responses.InternalServerError(w, r, msgAuthenticationFailed)
				return
			}
			DummyValidateUser(cfg.Pepper, login.Password)
			responses.BadRequest(w, r, msgInvalidUsernameOrPassword)
			return
		}

		if !account.ValidateUser(cfg.Pepper, login.Password) {
			responses.BadRequest(w, r, msgInvalidUsernameOrPassword)
			return
		}

		enabled, err := as.IsPasswordAuthEnabled(ctx, account.UserID)
		if err != nil {
			reqctx.Log(ctx, "Failed to check password auth setting:\n\t", err.Error())
			responses.InternalServerError(w, r, msgAuthenticationFailed)
			return
		}
		if !enabled {
			responses.BadRequest(w, r, msgInvalidUsernameOrPassword)
			return
		}

		s, err := as.NewSession(ctx, account, time.Now().Add(time.Hour*24).Unix())
		if err != nil {
			reqctx.Log(ctx, "Failed to create session:\n\t", err.Error())
			responses.InternalServerError(w, r, msgAuthenticationFailed)
			return
		}

		if err = ss.AddSession(ctx, s); err != nil {
			reqctx.Log(ctx, "Failed to add session:\n\t", err.Error())
			responses.InternalServerError(w, r, msgAuthenticationFailed)
			return
		}

		if err := ss.SetSessionCookie(ctx, w, s); err != nil {
			reqctx.Log(ctx, logFailedToCreateJWT, err.Error())
			responses.InternalServerError(w, r, msgAuthenticationFailed)
			return
		}
		responses.NoContent(w, r)
	}
}

// LogoutHandler handles the logout route
func LogoutHandler(ss session.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s := r.Context().Value(reqctx.SessionKey).(*session.Session)
		if s == nil {
			responses.BadRequest(w, r, msgInvalidSession)
			return
		}
		err := ss.DeleteSession(r.Context(), s.ID)
		if err != nil {
			reqctx.Log(r.Context(), "Failed to delete session:\n\t", err.Error())
			responses.InternalServerError(w, r, msgFailedToDeleteSession)
			return
		}
		http.SetCookie(w, VoidCookie)
		responses.NoContent(w, r)
	}
}
