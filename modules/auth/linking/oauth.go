package linking

import (
	"context"
	"errors"
	"fmt"
	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/twitch"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"time"
)

// -------------- Structs --------------

// Mode describing how to handle the OAuth interaction
type Mode string

const (
	ModeLogin Mode = "login"
	ModeLink  Mode = "link"
)

// OAuthState used with the OAuth state URL parameter
type OAuthState struct {
	Platform    auth.Platform `json:"platform"`
	Nonce       string        `json:"nonce"`
	RedirectURI string        `json:"redirect_uri"`
	Mode        Mode          `json:"mode"`
}

// -------------- Functions --------------

// ExtCodeForToken exchanges the code for an access token and returns a auth.OAuthToken
func ExtCodeForToken(config *oauth2.Config, code string) (*auth.OAuthToken, error) {
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, errors.New("failed to exchange code for access token")
	}

	var scopes []string
	if rawScopes, ok := token.Extra("scope").([]interface{}); ok {
		for _, s := range rawScopes {
			if str, ok := s.(string); ok {
				scopes = append(scopes, str)
			}
		}
		// Discord is special and returns a single string if there's only one scope
	} else if rawScopes, ok := token.Extra("scope").(string); ok {
		scopes = []string{rawScopes}
	} else {
		return nil, errors.New("failed to get scope from token")
	}

	var scopedToken = &auth.OAuthToken{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry.Unix(),
		ExpiresIn:    token.ExpiresIn,
		Scope:        scopes,
	}

	return scopedToken, nil
}

// RefreshToken refreshes the token and returns a auth.OAuthToken
func RefreshToken(config *oauth2.Config, token *oauth2.Token) (*auth.OAuthToken, error) {
	newToken, err := config.TokenSource(context.Background(), token).Token()
	if err != nil {
		return nil, err
	}
	if newToken == nil {
		return nil, errors.New("failed to refresh token")
	}

	var scopes []string
	if rawScopes, ok := newToken.Extra("scope").([]interface{}); ok {
		for _, s := range rawScopes {
			if str, ok := s.(string); ok {
				scopes = append(scopes, str)
			}
		}
	} else if rawScopes, ok := newToken.Extra("scope").(string); ok {
		scopes = []string{rawScopes}
	} else {
		return nil, errors.New("failed to get scope from token")
	}

	var scopedToken = &auth.OAuthToken{
		AccessToken:  newToken.AccessToken,
		TokenType:    newToken.TokenType,
		RefreshToken: newToken.RefreshToken,
		Expiry:       newToken.Expiry.Unix(),
		ExpiresIn:    newToken.ExpiresIn,
		Scope:        scopes,
	}

	return scopedToken, nil
}

// ProcessOAuthLogin processes the OAuth2 code and returns a session
func ProcessOAuthLogin(as auth.AccountService, las auth.LinkAccountStore, ss auth.SessionService, code string, state *OAuthState) (*auth.Session, error) {
	var err error
	var config *oauth2.Config
	switch state.Platform {
	case auth.PlatformDiscord:
		config = discordConfig
	case auth.PlatformTwitch:
		config = twitch.Config
	case auth.PlatformMinecraft, auth.PlatformXboxLive:
		config = MicrosoftConfig
	case auth.PlatformMicrosoft:
		config = MicrosoftLoginConfig
	default:
		return nil, errors.New("invalid platform")
	}
	var token *auth.OAuthToken
	token, err = ExtCodeForToken(config, code)
	if err != nil {
		return nil, err
	}

	var a *auth.Account
	if state.Platform == auth.PlatformMinecraft || state.Platform == auth.PlatformXboxLive {
		var xbox *XboxLiveData
		var java *MinecraftData
		var xboxErr error
		if state.Platform == auth.PlatformXboxLive {
			xbox, xboxErr = GetXboxUser(token)
		} else {
			xbox, java, xboxErr = GetXboxAndMinecraftUser(token)
		}
		if xboxErr != nil {
			if xbox == nil {
				return nil, xboxErr
			}
			// Xbox Live succeeded; the failure was only in the optional
			// Java-ownership check, which isn't blocking.
			log.Println("Java profile lookup failed during Microsoft OAuth login, continuing with Xbox Live identity only:\n\t", xboxErr)
			java = nil
		}
		a, err = resolveOrCreateAccountForMicrosoftUser(as, las, xbox, java)
		if err != nil {
			return nil, err
		}
	} else {
		var user auth.PlatformData
		switch state.Platform {
		case auth.PlatformDiscord:
			user, err = GetDiscordUser(token)
		case auth.PlatformTwitch:
			user, err = twitch.GetUser(token)
		case auth.PlatformMicrosoft:
			user, err = GetMicrosoftUser(token)
		default:
			return nil, errors.New("invalid platform")
		}
		if err != nil {
			return nil, err
		}
		a, err = resolveOrCreateAccountForPlatformUser(as, las, state.Platform, user)
		if err != nil {
			return nil, err
		}
	}

	session, err := a.NewSession(time.Now().Add(time.Hour * 24).Unix())
	if err != nil {
		return nil, err
	}

	if err = ss.AddSession(session); err != nil {
		return nil, err
	}
	return session, nil
}

// resolveOrCreateAccountForPlatformUser resolves the auth.Account linked to
// the given platform user, creating a new placeholder account and linking it
// if none exists yet. If AddLinkedAccountToDB fails for any reason, the
// placeholder account created above is now orphaned and is cleaned up before
// deciding how to handle the error: on auth.ErrAlreadyLinked (a concurrent
// request won the race to link this exact platform account first) the
// winner's linked account/account are re-fetched and returned instead of
// treating it as a hard failure; any other error is returned as-is (wrapped
// together with a cleanup failure, if the cleanup itself also failed).
func resolveOrCreateAccountForPlatformUser(as auth.AccountService, las auth.LinkAccountStore, platform auth.Platform, user auth.PlatformData) (*auth.Account, error) {
	la, err := las.GetLinkedAccountByPlatformID(platform, user.GetID())
	if err == nil {
		return as.GetAccountByID(la.UserID)
	}
	if !errors.Is(err, auth.ErrNotFound) {
		return nil, err
	}

	a, err := auth.NewPasswordLessAccount(user.GetUsername(), user.GetEmail())
	if err != nil {
		return nil, err
	}
	if err := as.AddAccount(a); err != nil {
		return nil, err
	}

	la = auth.NewLinkedAccount(a.UserID, platform, user.GetUsername(), user.GetID(), user)
	if err := las.AddLinkedAccountToDB(la); err != nil {
		// Whatever went wrong, the account created above is now orphaned -
		// clean it up before deciding how to handle err.
		if delErr := as.DeleteAccount(a.UserID); delErr != nil {
			return nil, fmt.Errorf("failed to link account (%w) and failed to clean up the orphaned placeholder account: %w", err, delErr)
		}
		if !errors.Is(err, auth.ErrAlreadyLinked) {
			return nil, err
		}
		// Lost the race: use the winner's account instead.
		la, err = las.GetLinkedAccountByPlatformID(platform, user.GetID())
		if err != nil {
			return nil, err
		}
		a, err = as.GetAccountByID(la.UserID)
		if err != nil {
			return nil, err
		}
	}
	return a, nil
}

// ProcessOAuthLink links an account to an existing user
func ProcessOAuthLink(r *http.Request, las auth.LinkAccountStore, code string, state *OAuthState) (*auth.Session, error) {
	// Get session from request context first: there's no point exchanging
	// the OAuth code or calling out to the platform's API for a session
	// that's missing or already expired.
	session, ok := r.Context().Value(mw.SessionKey).(*auth.Session)
	if !ok || session == nil {
		return nil, errors.New("session not found")
	}
	if !session.IsValid() {
		return nil, errors.New("session expired")
	}

	var err error
	var config *oauth2.Config
	switch state.Platform {
	case auth.PlatformDiscord:
		config = discordConfig
	case auth.PlatformTwitch:
		config = twitch.Config
	case auth.PlatformMinecraft, auth.PlatformXboxLive:
		config = MicrosoftConfig
	case auth.PlatformMicrosoft:
		config = MicrosoftLoginConfig
	default:
		return nil, errors.New("invalid platform")
	}
	var token *auth.OAuthToken
	token, err = ExtCodeForToken(config, code)
	if err != nil {
		return nil, err
	}

	if state.Platform == auth.PlatformMinecraft || state.Platform == auth.PlatformXboxLive {
		var xbox *XboxLiveData
		var java *MinecraftData
		var xboxErr error
		if state.Platform == auth.PlatformXboxLive {
			xbox, xboxErr = GetXboxUser(token)
		} else {
			xbox, java, xboxErr = GetXboxAndMinecraftUser(token)
		}
		if xboxErr != nil {
			if xbox == nil {
				return nil, xboxErr
			}
			// Xbox Live succeeded; the failure was only in the optional
			// Java-ownership check, which isn't blocking.
			log.Println("Java profile lookup failed during Microsoft OAuth link, continuing with Xbox Live identity only:\n\t", xboxErr)
			java = nil
		}

		// Check both identities up front, before committing either link:
		// linking Xbox and only then rejecting Java would leave Xbox linked
		// with no way to undo it (AddLinkedAccountToDB has no delete).
		if err := checkPlatformUserBelongsToSession(las, session, auth.PlatformXboxLive, xbox); err != nil {
			return nil, err
		}
		if java != nil {
			if err := checkPlatformUserBelongsToSession(las, session, auth.PlatformMinecraft, java); err != nil {
				return nil, err
			}
		}

		if _, err := linkPlatformUserToSession(las, session, auth.PlatformXboxLive, xbox); err != nil {
			return nil, err
		}
		if java != nil {
			if _, err := linkPlatformUserToSession(las, session, auth.PlatformMinecraft, java); err != nil {
				return nil, err
			}
		}
		return session, nil
	}

	var user auth.PlatformData
	switch state.Platform {
	case auth.PlatformDiscord:
		user, err = GetDiscordUser(token)
	case auth.PlatformTwitch:
		user, err = twitch.GetUser(token)
	case auth.PlatformMicrosoft:
		user, err = GetMicrosoftUser(token)
	default:
		return nil, errors.New("invalid platform")
	}
	if err != nil {
		return nil, err
	}

	return linkPlatformUserToSession(las, session, state.Platform, user)
}

// errConflictingMicrosoftIdentities is returned when a Microsoft account's
// Xbox Live and Minecraft: Java Edition identities are linked to two
// different NN accounts.
var errConflictingMicrosoftIdentities = errors.New("this Microsoft account's Xbox Live and Minecraft: Java Edition identities are linked to two different accounts; unlink one before linking via Microsoft again")

// resolveOrCreateAccountForMicrosoftUser resolves the auth.Account for a
// Microsoft-authenticated login, given the caller's Xbox Live identity
// (always present) and Java Edition profile (present only if owned).
// Whichever identity/identities aren't linked yet get linked to the
// resolved (or freshly created) account.
func resolveOrCreateAccountForMicrosoftUser(as auth.AccountService, las auth.LinkAccountStore, xbox *XboxLiveData, java *MinecraftData) (*auth.Account, error) {
	xboxAccountID, err := existingAccountIDForPlatformUser(las, auth.PlatformXboxLive, xbox.GetID())
	if err != nil {
		return nil, err
	}
	var javaAccountID string
	if java != nil {
		javaAccountID, err = existingAccountIDForPlatformUser(las, auth.PlatformMinecraft, java.GetID())
		if err != nil {
			return nil, err
		}
	}
	if xboxAccountID != "" && javaAccountID != "" && xboxAccountID != javaAccountID {
		return nil, errConflictingMicrosoftIdentities
	}

	accountID := xboxAccountID
	if accountID == "" {
		accountID = javaAccountID
	}

	var a *auth.Account
	isNewAccount := accountID == ""
	if isNewAccount {
		// Prefer the Java username when both identities are present - it's
		// the player's own chosen name, unlike the Xbox gamertag.
		username := xbox.GetUsername()
		if java != nil {
			username = java.GetUsername()
		}
		a, err = auth.NewPasswordLessAccount(username, "")
		if err != nil {
			return nil, err
		}
		if err := as.AddAccount(a); err != nil {
			return nil, err
		}
	} else {
		a, err = as.GetAccountByID(accountID)
		if err != nil {
			return nil, err
		}
	}

	if xboxAccountID == "" {
		a, isNewAccount, err = ensureMicrosoftIdentityLinked(as, las, a, isNewAccount, auth.PlatformXboxLive, xbox)
		if err != nil {
			return nil, err
		}
	}
	if java != nil && javaAccountID == "" {
		a, _, err = ensureMicrosoftIdentityLinked(as, las, a, isNewAccount, auth.PlatformMinecraft, java)
		if err != nil {
			return nil, err
		}
	}

	return a, nil
}

// ensureMicrosoftIdentityLinked links the given identity to account a.
// isNewAccount marks a as a bare placeholder with nothing else linked to it
// yet, the only state where it's safe to delete on a lost race; otherwise a
// lost race surfaces as a conflict for the caller to retry.
func ensureMicrosoftIdentityLinked(as auth.AccountService, las auth.LinkAccountStore, a *auth.Account, isNewAccount bool, platform auth.Platform, user auth.PlatformData) (*auth.Account, bool, error) {
	err := linkIdentityToAccountID(las, a.UserID, platform, user)
	if err == nil {
		return a, false, nil
	}
	if !errors.Is(err, auth.ErrAlreadyLinked) {
		if isNewAccount {
			if delErr := as.DeleteAccount(a.UserID); delErr != nil {
				return nil, false, fmt.Errorf("failed to link account (%w) and failed to clean up the orphaned placeholder account: %w", err, delErr)
			}
		}
		return nil, false, err
	}

	// We might already be the owner (a concurrent identical login won the
	// race for both identities) rather than facing a genuine conflict.
	actualOwnerID, lookupErr := existingAccountIDForPlatformUser(las, platform, user.GetID())
	if lookupErr != nil {
		return nil, false, lookupErr
	}
	if actualOwnerID == a.UserID {
		return a, false, nil
	}
	if !isNewAccount {
		return nil, false, errConflictingMicrosoftIdentities
	}

	if delErr := as.DeleteAccount(a.UserID); delErr != nil {
		return nil, false, fmt.Errorf("failed to link account (%w) and failed to clean up the orphaned placeholder account: %w", err, delErr)
	}
	winner, getErr := as.GetAccountByID(actualOwnerID)
	if getErr != nil {
		return nil, false, getErr
	}
	return winner, false, nil
}

// existingAccountIDForPlatformUser returns the account ID already linked to
// the given platform identity, or "" if none is linked yet.
func existingAccountIDForPlatformUser(las auth.LinkAccountStore, platform auth.Platform, platformID string) (string, error) {
	la, err := las.GetLinkedAccountByPlatformID(platform, platformID)
	if err == nil {
		return la.UserID, nil
	}
	if errors.Is(err, auth.ErrNotFound) {
		return "", nil
	}
	return "", err
}

// linkIdentityToAccountID links a platform identity to an already-resolved
// account ID. Unlike resolveOrCreateAccountForPlatformUser's login path,
// this doesn't retry against a concurrent winner on auth.ErrAlreadyLinked -
// existingAccountIDForPlatformUser already established the identity was
// unlinked moments ago, so a race here is left as a surfaced error for the
// user to retry rather than a silently resolved one.
func linkIdentityToAccountID(las auth.LinkAccountStore, accountID string, platform auth.Platform, user auth.PlatformData) error {
	la := auth.NewLinkedAccount(accountID, platform, user.GetUsername(), user.GetID(), user)
	return las.AddLinkedAccountToDB(la)
}

// errPlatformAlreadyLinkedToDifferentAccount is returned by both
// linkPlatformUserToSession and checkPlatformUserBelongsToSession when a
// platform identity belongs to an account other than the one being linked
// into - shared so the two checks (one that writes, one read-only) can't
// drift apart on wording.
var errPlatformAlreadyLinkedToDifferentAccount = errors.New("this platform account is already linked to a different account; log in with it directly if you want to use that account, or unlink it there first")

// linkPlatformUserToSession links the given platform identity to the
// session's account. If it's already linked to a different account, that's
// returned as an error; if it's already linked to this same account, linking
// is a no-op.
func linkPlatformUserToSession(las auth.LinkAccountStore, session *auth.Session, platform auth.Platform, user auth.PlatformData) (*auth.Session, error) {
	la, err := las.GetLinkedAccountByPlatformID(platform, user.GetID())
	switch {
	case err == nil:
		if session.UserID == la.UserID {
			return session, nil
		}
		return nil, errPlatformAlreadyLinkedToDifferentAccount
	case !errors.Is(err, auth.ErrNotFound):
		return nil, err
	}

	// Link account
	la = auth.NewLinkedAccount(session.UserID, platform, user.GetUsername(), user.GetID(), user)
	if err := las.AddLinkedAccountToDB(la); err != nil {
		return nil, errors.New("failed to link account")
	}

	return session, nil
}

// checkPlatformUserBelongsToSession verifies the given platform identity is
// either unlinked or already linked to the session's own account, without
// writing anything. Used to validate every identity a multi-identity
// Microsoft login carries up front, before committing any of their links -
// see the comment at its call site in ProcessOAuthLink for why that order
// matters.
func checkPlatformUserBelongsToSession(las auth.LinkAccountStore, session *auth.Session, platform auth.Platform, user auth.PlatformData) error {
	la, err := las.GetLinkedAccountByPlatformID(platform, user.GetID())
	if err == nil && la.UserID != session.UserID {
		return errPlatformAlreadyLinkedToDifferentAccount
	}
	if err != nil && !errors.Is(err, auth.ErrNotFound) {
		return err
	}
	return nil
}
