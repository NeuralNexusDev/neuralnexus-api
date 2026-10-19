package link

import (
	"encoding/base64"
	"net/http"
	"net/url"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/permissions"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/goccy/go-json"
)

const (
	msgAuthenticationFailed             = "Authentication failed"
	msgInvalidRequest                   = "Invalid request"
	logInvalidState                     = "Invalid state"
	msgInvalidState                     = "Invalid state"
	msgLoginRequiredToLink              = "You must be logged in to link an account"
	msgInvalidPlatform                  = "Invalid platform"
	msgSessionExpired                   = "Your session has expired"
	msgMissingOAuthScope                = "The platform did not grant the required access"
	msgPlatformLoginDisabled            = "Login with this platform is disabled for this account"
	msgConflictingMicrosoftIdentities   = "This Microsoft account's Xbox Live and Minecraft: Java Edition identities belong to different accounts"
	msgPlatformAlreadyLinked            = "This platform account is already linked to a different account"
	msgSteamAccountNotFound             = "Steam account not found"
	msgSteamAccountMismatch             = "Steam account mismatch"
	msgInvalidRequestBody               = "Invalid request body"
	msgPlatformNotLinked                = "This platform isn't linked to this user"
	msgNoPermissionToViewLinkedAccounts = "You do not have permission to view this user's linked accounts"
	msgFailedToGetLinkedAccounts        = "Failed to get linked accounts"
	msgNoPermissionToUnlinkPlatforms    = "You do not have permission to unlink this user's platforms"
	msgSetPasswordOrLinkBeforeUnlinking = "Set a password or link another platform before unlinking your last one"
	msgFailedToUnlinkPlatform           = "Failed to unlink platform"
	msgNoPermissionToUpdatePlatforms    = "You do not have permission to update this user's platforms"
	msgSetPasswordOrLinkBeforeDisabling = "Set a password or link another platform before disabling your last login method"
	msgLinkedAccountUnverified          = "This linked account is unverified and can't be enabled for login"
	msgFailedToUpdatePlatform           = "Failed to update platform"
)

// OAuthHandler handles the OAuth route
func OAuthHandler(as account.Service, las Store, ss session.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		code := r.URL.Query().Get("code")
		if code == "" {
			reqctx.Log(ctx, "No code provided")
			redirectBadRequest(w, r, session.NN_SITE_URL, msgInvalidRequest)
			return
		}

		state, ok := decodeAndValidateState(w, r)
		if !ok {
			return
		}
		if !requireValidModeAndSession(w, r, state.Mode, state.RedirectURI) {
			return
		}

		var session *session.Session
		var err error
		switch state.Mode {
		case ModeLogin:
			session, err = ProcessOAuthLogin(ctx, as, las, ss, code, &state)
		case ModeLink:
			session, err = ProcessOAuthLink(r, las, code, &state)
		}
		if err != nil {
			reqctx.Log(ctx, "Failed to process OAuth:\n\t", err.Error())
			switch err {
			case ErrSessionNotFound:
				redirectUnauthorized(w, r, state.RedirectURI, msgLoginRequiredToLink)
			case ErrSessionExpired:
				redirectUnauthorized(w, r, state.RedirectURI, msgSessionExpired)
			case ErrInvalidPlatform:
				redirectBadRequest(w, r, state.RedirectURI, msgInvalidPlatform)
			case ErrNoScopeInToken:
				redirectBadRequest(w, r, state.RedirectURI, msgMissingOAuthScope)
			case ErrPlatformLoginDisabled:
				redirectProblem(w, r, state.RedirectURI, http.StatusForbidden, msgPlatformLoginDisabled)
			case ErrConflictingMicrosoftIdentities:
				redirectProblem(w, r, state.RedirectURI, http.StatusConflict, msgConflictingMicrosoftIdentities)
			case ErrPlatformAlreadyLinkedToDifferentAccount:
				redirectProblem(w, r, state.RedirectURI, http.StatusConflict, msgPlatformAlreadyLinked)
			default:
				redirectInternalServerError(w, r, state.RedirectURI, msgAuthenticationFailed)
			}
			return
		}

		if err := ss.SetSessionCookie(ctx, w, session); err != nil {
			redirectInternalServerError(w, r, state.RedirectURI, msgAuthenticationFailed)
			return
		}
		http.Redirect(w, r, state.RedirectURI, http.StatusSeeOther)
	}
}

// OpenIDHandler handles the Steam OpenID 2.0 login/link callback.
func OpenIDHandler(as account.Service, las Store, ss session.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		state, ok := decodeAndValidateState(w, r)
		if !ok {
			return
		}
		if !requireValidModeAndSession(w, r, state.Mode, state.RedirectURI) {
			return
		}

		steamID64, err := VerifySteamOpenIDCallback(ctx, r.URL.Query())
		if err != nil {
			reqctx.Log(ctx, "Failed to verify Steam OpenID callback:\n\t", err.Error())
			switch err {
			case ErrInvalidAssertion:
				redirectBadRequest(w, r, state.RedirectURI, msgInvalidState)
			default:
				redirectInternalServerError(w, r, state.RedirectURI, msgAuthenticationFailed)
			}
			return
		}

		user, err := GetSteamUser(ctx, steamID64)
		if err != nil {
			reqctx.Log(ctx, "Failed to get Steam user:\n\t", err.Error())
			switch err {
			case ErrSteamNoPlayers:
				redirectProblem(w, r, state.RedirectURI, http.StatusNotFound, msgSteamAccountNotFound)
			case ErrSteamIDMismatch:
				redirectBadRequest(w, r, state.RedirectURI, msgSteamAccountMismatch)
			default:
				redirectInternalServerError(w, r, state.RedirectURI, msgAuthenticationFailed)
			}
			return
		}

		var session *session.Session
		switch state.Mode {
		case ModeLogin:
			session, err = ProcessSteamLogin(ctx, as, las, ss, user)
		case ModeLink:
			session, err = ProcessSteamLink(r, las, user)
		}
		if err != nil {
			reqctx.Log(ctx, "Failed to process Steam OpenID:\n\t", err.Error())
			switch err {
			case ErrSessionNotFound:
				redirectUnauthorized(w, r, state.RedirectURI, msgLoginRequiredToLink)
			case ErrSessionExpired:
				redirectUnauthorized(w, r, state.RedirectURI, msgSessionExpired)
			case ErrPlatformLoginDisabled:
				redirectProblem(w, r, state.RedirectURI, http.StatusForbidden, msgPlatformLoginDisabled)
			case ErrPlatformAlreadyLinkedToDifferentAccount:
				redirectProblem(w, r, state.RedirectURI, http.StatusConflict, msgPlatformAlreadyLinked)
			default:
				redirectInternalServerError(w, r, state.RedirectURI, msgAuthenticationFailed)
			}
			return
		}

		if err := ss.SetSessionCookie(ctx, w, session); err != nil {
			redirectInternalServerError(w, r, state.RedirectURI, msgAuthenticationFailed)
			return
		}
		http.Redirect(w, r, state.RedirectURI, http.StatusSeeOther)
	}
}

// decodeAndValidateState decodes state from the query param, checking it
// against the redirect allowlist and the nonce cookie.
func decodeAndValidateState(w http.ResponseWriter, r *http.Request) (OAuthState, bool) {
	var state OAuthState

	stateB64 := r.URL.Query().Get("state")
	if stateB64 == "" {
		reqctx.Log(r.Context(), "No state provided")
		redirectBadRequest(w, r, session.NN_SITE_URL, msgInvalidRequest)
		return state, false
	}
	stateBytes, err := base64.URLEncoding.DecodeString(stateB64)
	if err != nil {
		reqctx.Log(r.Context(), "Failed to decode state:\n\t", err.Error())
		redirectBadRequest(w, r, session.NN_SITE_URL, msgInvalidState)
		return state, false
	}
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		reqctx.Log(r.Context(), "Failed to unmarshal state:\n\t", err.Error())
		redirectBadRequest(w, r, session.NN_SITE_URL, msgInvalidState)
		return state, false
	}
	if state.Platform == "" || state.Nonce == "" || state.RedirectURI == "" || state.Mode == "" {
		reqctx.Log(r.Context(), logInvalidState)
		redirectBadRequest(w, r, session.NN_SITE_URL, msgInvalidState)
		return state, false
	}
	if !isAllowedRedirect(state.RedirectURI) {
		reqctx.Log(r.Context(), "Redirect URI is not allowed:\n\t", state.RedirectURI)
		redirectBadRequest(w, r, session.NN_SITE_URL, msgInvalidState)
		return state, false
	}

	cookie, err := r.Cookie("nonce")
	if err != nil {
		reqctx.Log(r.Context(), "Failed to get nonce cookie:\n\t", err.Error())
		redirectBadRequest(w, r, session.NN_SITE_URL, msgInvalidState)
		return state, false
	}
	if cookie.Value != state.Nonce {
		reqctx.Log(r.Context(), "Nonce does not match")
		redirectBadRequest(w, r, session.NN_SITE_URL, msgInvalidState)
		return state, false
	}

	return state, true
}

// requireValidModeAndSession checks mode is recognized and, for ModeLink,
// that there's a live session to link to.
func requireValidModeAndSession(w http.ResponseWriter, r *http.Request, mode Mode, redirectURI string) bool {
	switch mode {
	case ModeLogin:
		return true
	case ModeLink:
		if session, ok := r.Context().Value(reqctx.SessionKey).(*session.Session); ok && session != nil && session.IsValid() {
			return true
		}
		redirectUnauthorized(w, r, redirectURI, msgLoginRequiredToLink)
		return false
	default:
		reqctx.Log(r.Context(), "Invalid mode")
		redirectBadRequest(w, r, redirectURI, msgInvalidState)
		return false
	}
}

// redirectWithError redirects to target with an RFC 9457 problem base64
// (URL-safe) encoded into a "problem" query param.
func redirectWithError(w http.ResponseWriter, r *http.Request, target string, status int, title, detail string) {
	problemJSON, err := json.Marshal(responses.NewProblem("about:blank", status, title, detail, "").Problem)
	if err != nil {
		reqctx.Log(r.Context(), "Failed to marshal error problem:\n\t", err.Error())
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
// mirror responses.BadRequest/Unauthorized/InternalServerError.
func redirectBadRequest(w http.ResponseWriter, r *http.Request, target, detail string) {
	redirectWithError(w, r, target, http.StatusBadRequest, "Bad Request", detail)
}
func redirectUnauthorized(w http.ResponseWriter, r *http.Request, target, detail string) {
	redirectWithError(w, r, target, http.StatusUnauthorized, "Unauthorized", detail)
}
func redirectProblem(w http.ResponseWriter, r *http.Request, target string, status int, detail string) {
	redirectWithError(w, r, target, status, http.StatusText(status), detail)
}
func redirectInternalServerError(w http.ResponseWriter, r *http.Request, target, detail string) {
	redirectWithError(w, r, target, http.StatusInternalServerError, "Internal Server Error", detail)
}

// isAllowedRedirect reports whether redirectURI's scheme and host match NN_SITE_URL.
func isAllowedRedirect(redirectURI string) bool {
	siteURL, err := url.Parse(session.NN_SITE_URL)
	if err != nil {
		return false
	}
	target, err := url.Parse(redirectURI)
	if err != nil {
		return false
	}
	return target.Scheme == siteURL.Scheme && target.Host == siteURL.Host
}

// GetUserLinkedAccountsHandler - List a user's linked platforms
func GetUserLinkedAccountsHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(reqctx.SessionKey).(*session.Session)
		userID := r.PathValue("user_id")
		if session.UserID != userID && !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, msgNoPermissionToViewLinkedAccounts)
			return
		}
		links, err := service.GetUserLinkedAccounts(r.Context(), userID)
		if err != nil {
			responses.InternalServerError(w, r, msgFailedToGetLinkedAccounts)
			return
		}
		responses.StructOK(w, r, links)
	}
}

// UnlinkPlatformHandler - Unlink a platform from a user
func UnlinkPlatformHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(reqctx.SessionKey).(*session.Session)
		userID := r.PathValue("user_id")
		if session.UserID != userID && !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, msgNoPermissionToUnlinkPlatforms)
			return
		}
		platform := Platform(r.PathValue("platform"))
		err := service.UnlinkPlatform(r.Context(), userID, platform)
		switch err {
		case nil:
			responses.NoContent(w, r)
		case account.ErrNotFound:
			responses.NotFound(w, r, msgPlatformNotLinked)
		case account.ErrWouldLockAccount:
			responses.BadRequest(w, r, msgSetPasswordOrLinkBeforeUnlinking)
		default:
			responses.InternalServerError(w, r, msgFailedToUnlinkPlatform)
		}
	}
}

// SetPlatformLoginEnabledRequest - Body for SetPlatformLoginEnabledHandler
type SetPlatformLoginEnabledRequest struct {
	// LoginEnabled is a pointer so a missing field is rejected instead of
	// silently defaulting to false.
	LoginEnabled *bool `json:"login_enabled" xml:"login_enabled"`
}

// SetPlatformLoginEnabledHandler - Toggle whether a linked platform can log in
func SetPlatformLoginEnabledHandler(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := r.Context().Value(reqctx.SessionKey).(*session.Session)
		userID := r.PathValue("user_id")
		if session.UserID != userID && !session.HasPermission(perms.ScopeAdminUsers) {
			responses.Forbidden(w, r, msgNoPermissionToUpdatePlatforms)
			return
		}
		platform := Platform(r.PathValue("platform"))
		var body SetPlatformLoginEnabledRequest
		if err := responses.DecodeStruct(r, &body); err != nil || body.LoginEnabled == nil {
			responses.BadRequest(w, r, msgInvalidRequestBody)
			return
		}

		err := service.SetPlatformLoginEnabled(r.Context(), userID, platform, *body.LoginEnabled)
		switch err {
		case nil:
			responses.NoContent(w, r)
		case account.ErrNotFound:
			responses.NotFound(w, r, msgPlatformNotLinked)
		case account.ErrWouldLockAccount:
			responses.BadRequest(w, r, msgSetPasswordOrLinkBeforeDisabling)
		case ErrLinkedAccountUnverified:
			responses.BadRequest(w, r, msgLinkedAccountUnverified)
		default:
			responses.InternalServerError(w, r, msgFailedToUpdatePlatform)
		}
	}
}
