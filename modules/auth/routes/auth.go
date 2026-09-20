package authroutes

import (
	"encoding/base64"
	"github.com/goccy/go-json"
	"log"
	"net/http"
	"net/url"
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
			auth.DummyValidateUser(login.Password)
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
		http.SetCookie(w, sessionCookie(jwt, time.Unix(session.ExpiresAt, 0)))
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
		http.SetCookie(w, sessionCookie("", time.Unix(0, 0)))
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
		if !isAllowedRedirect(state.RedirectURI) {
			log.Println("Redirect URI is not allowed:\n\t", state.RedirectURI)
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
		case linking.ModeLink:
			if linkSession, ok := r.Context().Value(mw.SessionKey).(*auth.Session); !ok || linkSession == nil {
				responses.Unauthorized(w, r, "You must be logged in to link an account")
				return
			}
			session, err = linking.ProcessOAuthLink(r, las, code, &state)
		default:
			log.Println("Invalid mode")
			responses.BadRequest(w, r, "Invalid state")
			return
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
		http.SetCookie(w, sessionCookie(jwtString, time.Unix(session.ExpiresAt, 0)))

		http.Redirect(w, r, state.RedirectURI, http.StatusSeeOther)
	}
}

// OpenIDHandler handles the Steam OpenID 2.0 login/link callback. Unlike
// OAuthHandler, there's no code-for-token exchange - Steam's response is a
// signed assertion in the query string, verified via
// linking.VerifySteamOpenIDCallback's check_authentication round-trip.
func OpenIDHandler(as auth.AccountService, las auth.LinkAccountStore, ss auth.SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		if err := json.Unmarshal(stateBytes, &state); err != nil {
			log.Println("Failed to unmarshal state:\n\t", err)
			responses.BadRequest(w, r, "Invalid state")
			return
		}
		if state.Nonce == "" || state.RedirectURI == "" || state.Mode == "" {
			log.Println("Invalid state")
			responses.BadRequest(w, r, "Invalid state")
			return
		}
		if !isAllowedRedirect(state.RedirectURI) {
			log.Println("Redirect URI is not allowed:\n\t", state.RedirectURI)
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

		// Check the mode - and, for a link, that there's actually a session to
		// link to - before doing any of the expensive work below: there's no
		// point verifying the assertion with Steam or fetching the caller's
		// profile for a request that's going to be rejected anyway.
		switch state.Mode {
		case linking.ModeLogin:
		case linking.ModeLink:
			if linkSession, ok := r.Context().Value(mw.SessionKey).(*auth.Session); !ok || linkSession == nil {
				responses.Unauthorized(w, r, "You must be logged in to link an account")
				return
			}
		default:
			log.Println("Invalid mode")
			responses.BadRequest(w, r, "Invalid state")
			return
		}

		steamID64, err := linking.VerifySteamOpenIDCallback(r.URL.Query())
		if err != nil {
			log.Println("Failed to verify Steam OpenID callback:\n\t", err)
			responses.BadRequest(w, r, "Invalid state")
			return
		}

		user, err := linking.GetSteamUser(steamID64)
		if err != nil {
			log.Println("Failed to get Steam user:\n\t", err)
			responses.InternalServerError(w, r, "Authentication failed")
			return
		}

		var session *auth.Session
		switch state.Mode {
		case linking.ModeLogin:
			session, err = linking.ProcessSteamLogin(as, las, ss, user)
		case linking.ModeLink:
			session, err = linking.ProcessSteamLink(r, las, user)
		}

		if err != nil {
			log.Println("Failed to process Steam OpenID:\n\t", err)
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
		http.SetCookie(w, sessionCookie(jwtString, time.Unix(session.ExpiresAt, 0)))

		http.Redirect(w, r, state.RedirectURI, http.StatusSeeOther)
	}
}

// isAllowedRedirect reports whether redirectURI's scheme and host match
// NN_SITE_URL, rejecting an attacker-controlled state.RedirectURI rather
// than sending the browser (and its fresh session cookie) wherever it says.
func isAllowedRedirect(redirectURI string) bool {
	siteURL, err := url.Parse(auth.NN_SITE_URL)
	if err != nil {
		return false
	}
	target, err := url.Parse(redirectURI)
	if err != nil {
		return false
	}
	return target.Scheme == siteURL.Scheme && target.Host == siteURL.Host
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
