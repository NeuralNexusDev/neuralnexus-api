package authroutes

import (
	"encoding/base64"
	"errors"
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

		if err = ss.AddSession(session); err != nil {
			log.Println("Failed to add session:\n\t", err)
			responses.InternalServerError(w, r, "Authentication failed")
			return
		}

		if err := createSessionJWTAndSetCookie(ss, w, session); err != nil {
			log.Println("Failed to create JWT:\n\t", err)
			responses.InternalServerError(w, r, "Authentication failed")
			return
		}
		responses.NoContent(w, r)
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
		code := r.URL.Query().Get("code")
		if code == "" {
			log.Println("No code provided")
			redirectBadRequest(w, r, auth.NN_SITE_URL, "Invalid request")
			return
		}

		state, ok := decodeAndValidateState(w, r)
		if !ok {
			return
		}
		if !requireValidModeAndSession(w, r, state.Mode, state.RedirectURI) {
			return
		}

		var session *auth.Session
		var err error
		switch state.Mode {
		case linking.ModeLogin:
			session, err = linking.ProcessOAuthLogin(as, las, ss, code, &state)
		case linking.ModeLink:
			session, err = linking.ProcessOAuthLink(r, las, code, &state)
		}
		if err != nil {
			log.Println("Failed to process OAuth:\n\t", err)
			redirectInternalServerError(w, r, state.RedirectURI, "Authentication failed")
			return
		}

		issueSessionAndRedirect(w, r, ss, session, state.RedirectURI)
	}
}

// OpenIDHandler handles the Steam OpenID 2.0 login/link callback.
func OpenIDHandler(as auth.AccountService, las auth.LinkAccountStore, ss auth.SessionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state, ok := decodeAndValidateState(w, r)
		if !ok {
			return
		}
		if !requireValidModeAndSession(w, r, state.Mode, state.RedirectURI) {
			return
		}

		steamID64, err := linking.VerifySteamOpenIDCallback(r.URL.Query())
		if err != nil {
			log.Println("Failed to verify Steam OpenID callback:\n\t", err)
			if errors.Is(err, linking.ErrInvalidAssertion) {
				redirectBadRequest(w, r, state.RedirectURI, "Invalid state")
			} else {
				redirectInternalServerError(w, r, state.RedirectURI, "Authentication failed")
			}
			return
		}

		user, err := linking.GetSteamUser(steamID64)
		if err != nil {
			log.Println("Failed to get Steam user:\n\t", err)
			redirectInternalServerError(w, r, state.RedirectURI, "Authentication failed")
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
			redirectInternalServerError(w, r, state.RedirectURI, "Authentication failed")
			return
		}

		issueSessionAndRedirect(w, r, ss, session, state.RedirectURI)
	}
}

// decodeAndValidateState decodes state from the query param, checking it
// against the redirect allowlist and the nonce cookie. Every failure here
// happens before state.RedirectURI can be trusted, so errors redirect to
// the site root rather than anywhere client-supplied.
func decodeAndValidateState(w http.ResponseWriter, r *http.Request) (linking.OAuthState, bool) {
	var state linking.OAuthState

	stateB64 := r.URL.Query().Get("state")
	if stateB64 == "" {
		log.Println("No state provided")
		redirectBadRequest(w, r, auth.NN_SITE_URL, "Invalid request")
		return state, false
	}
	stateBytes, err := base64.URLEncoding.DecodeString(stateB64)
	if err != nil {
		log.Println("Failed to decode state:\n\t", err)
		redirectBadRequest(w, r, auth.NN_SITE_URL, "Invalid state")
		return state, false
	}
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		log.Println("Failed to unmarshal state:\n\t", err)
		redirectBadRequest(w, r, auth.NN_SITE_URL, "Invalid state")
		return state, false
	}
	if state.Platform == "" || state.Nonce == "" || state.RedirectURI == "" || state.Mode == "" {
		log.Println("Invalid state")
		redirectBadRequest(w, r, auth.NN_SITE_URL, "Invalid state")
		return state, false
	}
	if !isAllowedRedirect(state.RedirectURI) {
		log.Println("Redirect URI is not allowed:\n\t", state.RedirectURI)
		redirectBadRequest(w, r, auth.NN_SITE_URL, "Invalid state")
		return state, false
	}

	cookie, err := r.Cookie("nonce")
	if err != nil {
		log.Println("Failed to get nonce cookie:\n\t", err)
		redirectBadRequest(w, r, auth.NN_SITE_URL, "Invalid state")
		return state, false
	}
	if cookie.Value != state.Nonce {
		log.Println("Nonce does not match")
		redirectBadRequest(w, r, auth.NN_SITE_URL, "Invalid state")
		return state, false
	}

	return state, true
}

// requireValidModeAndSession checks mode is recognized and, for ModeLink,
// that there's a live session to link to - before any network calls.
// redirectURI is state.RedirectURI, already cleared by decodeAndValidateState.
func requireValidModeAndSession(w http.ResponseWriter, r *http.Request, mode linking.Mode, redirectURI string) bool {
	switch mode {
	case linking.ModeLogin:
		return true
	case linking.ModeLink:
		if session, ok := r.Context().Value(mw.SessionKey).(*auth.Session); ok && session != nil && session.IsValid() {
			return true
		}
		redirectUnauthorized(w, r, redirectURI, "You must be logged in to link an account")
		return false
	default:
		log.Println("Invalid mode")
		redirectBadRequest(w, r, redirectURI, "Invalid state")
		return false
	}
}

// createSessionJWTAndSetCookie creates a JWT for an already-persisted
// session and sets it as the session cookie.
func createSessionJWTAndSetCookie(ss auth.SessionService, w http.ResponseWriter, session *auth.Session) error {
	jwtString, err := ss.CreateJWT(session)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessionCookie(jwtString, time.Unix(session.ExpiresAt, 0)))
	return nil
}

// issueSessionAndRedirect sets the session cookie and redirects to redirectURI.
func issueSessionAndRedirect(w http.ResponseWriter, r *http.Request, ss auth.SessionService, session *auth.Session, redirectURI string) {
	if err := createSessionJWTAndSetCookie(ss, w, session); err != nil {
		log.Println("Failed to create JWT:\n\t", err)
		redirectInternalServerError(w, r, redirectURI, "Authentication failed")
		return
	}
	http.Redirect(w, r, redirectURI, http.StatusSeeOther)
}

// redirectWithError redirects to target with an RFC 9457 problem, base64
// (URL-safe) encoded into a "problem" query param, so a browser mid OAuth/
// OpenID flow lands back on the frontend instead of a raw API response. The
// caller has already logged the real error; detail is always one of the
// same short, generic strings used elsewhere for direct API responses,
// never provider or internal detail.
func redirectWithError(w http.ResponseWriter, r *http.Request, target string, status int, title, detail string) {
	problemJSON, err := json.Marshal(responses.NewProblem("about:blank", status, title, detail, "").Problem)
	if err != nil {
		log.Println("Failed to marshal error problem:\n\t", err)
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	problemB64 := base64.URLEncoding.EncodeToString(problemJSON)

	redirectURL := target
	if u, err := url.Parse(target); err == nil {
		q := u.Query()
		q.Set("problem", problemB64)
		u.RawQuery = q.Encode()
		redirectURL = u.String()
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// redirectBadRequest, redirectUnauthorized, and redirectInternalServerError
// mirror responses.BadRequest/Unauthorized/InternalServerError's status and
// title, but as a redirect carrying a problem+json instead of a direct
// response body - see redirectWithError.
func redirectBadRequest(w http.ResponseWriter, r *http.Request, target, detail string) {
	redirectWithError(w, r, target, http.StatusBadRequest, "Bad Request", detail)
}
func redirectUnauthorized(w http.ResponseWriter, r *http.Request, target, detail string) {
	redirectWithError(w, r, target, http.StatusUnauthorized, "Unauthorized", detail)
}
func redirectInternalServerError(w http.ResponseWriter, r *http.Request, target, detail string) {
	redirectWithError(w, r, target, http.StatusInternalServerError, "Internal Server Error", detail)
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
