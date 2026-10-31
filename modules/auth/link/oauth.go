package link

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/reqctx"
	"golang.org/x/oauth2"
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
	Platform    Platform `json:"platform"`
	Nonce       string   `json:"nonce"`
	RedirectURI string   `json:"redirect_uri"`
	Mode        Mode     `json:"mode"`
}

// -------------- Functions --------------

// ExtCodeForToken exchanges the code for an access token and returns a OAuthToken
func ExtCodeForToken(ctx context.Context, config *oauth2.Config, code string) (*OAuthToken, error) {
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, err
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
		return nil, ErrNoScopeInToken
	}

	var scopedToken = &OAuthToken{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry.Unix(),
		ExpiresIn:    token.ExpiresIn,
		Scope:        scopes,
	}

	return scopedToken, nil
}

// RefreshToken refreshes the token and returns a OAuthToken
func RefreshToken(ctx context.Context, config *oauth2.Config, token *oauth2.Token) (*OAuthToken, error) {
	newToken, err := config.TokenSource(ctx, token).Token()
	if err != nil {
		return nil, err
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
		return nil, ErrNoScopeInToken
	}

	var scopedToken = &OAuthToken{
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
func ProcessOAuthLogin(ctx context.Context, cfg *config.Config, as account.Service, las Store, ss session.Service, code string, state *OAuthState) (*session.Session, error) {
	var err error
	var config *oauth2.Config
	switch state.Platform {
	case PlatformDiscord:
		config = cfg.Discord.OAuth2Config
	case PlatformTwitch:
		config = cfg.Twitch.OAuth2Config
	case PlatformMinecraft, PlatformXboxLive:
		config = cfg.Microsoft.OAuth2Config
	case PlatformMicrosoft:
		config = cfg.Microsoft.LoginOAuth2Config
	default:
		return nil, ErrInvalidPlatform
	}
	var token *OAuthToken
	token, err = ExtCodeForToken(ctx, config, code)
	if err != nil {
		return nil, err
	}

	var a *account.Account
	if state.Platform == PlatformMinecraft || state.Platform == PlatformXboxLive {
		var xbox *XboxLiveData
		var java *MinecraftData
		var xboxErr error
		if state.Platform == PlatformXboxLive {
			xbox, xboxErr = GetXboxUser(ctx, cfg.Microsoft, token)
		} else {
			xbox, java, xboxErr = GetXboxAndMinecraftUser(ctx, cfg.Microsoft, token)
		}
		if xboxErr != nil {
			if xbox == nil {
				return nil, xboxErr
			}
			if _, lookupErr := las.GetLinkedAccountByPlatformID(ctx, PlatformXboxLive, xbox.GetID()); lookupErr != nil {
				return nil, xboxErr
			}
			reqctx.Log(ctx, "Minecraft: Java Edition lookup failed, continuing with the linked Xbox Live identity:\n\t", xboxErr.Error())
			java = nil
		}
		a, err = resolveOrCreateAccountForMicrosoftUser(ctx, as, las, xbox, java)
		if err != nil {
			return nil, err
		}
	} else {
		var user PlatformData
		switch state.Platform {
		case PlatformDiscord:
			user, err = GetDiscordUser(token)
		case PlatformTwitch:
			user, err = GetTwitchUser(ctx, cfg.Twitch, token)
		case PlatformMicrosoft:
			user, err = GetMicrosoftUser(ctx, cfg.Microsoft, token)
		default:
			return nil, ErrInvalidPlatform
		}
		if err != nil {
			return nil, err
		}
		a, err = resolveOrCreateAccountForPlatformUser(ctx, as, las, state.Platform, user)
		if err != nil {
			return nil, err
		}
	}

	session, err := as.NewSession(ctx, a, time.Now().Add(time.Hour*24).Unix())
	if err != nil {
		return nil, err
	}

	if err = ss.AddSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

var (
	ErrNoScopeInToken    = errors.New("failed to get scope from token")
	ErrInvalidPlatform   = errors.New("invalid platform")
	ErrSessionNotFound   = errors.New("session not found")
	ErrSessionExpired    = errors.New("session expired")
	ErrLinkAccountFailed = errors.New("failed to link account")
)

var ErrPlatformLoginDisabled = errors.New("this platform account is linked but disabled for login; use another linked platform or your password, or re-enable it first")

// resolveOrCreateAccountForPlatformUser resolves the account linked to
// the given platform user, creating a new placeholder account and linking it
// if none exists yet. If AddLinkedAccountToDB fails for any reason, the
// placeholder account created above is now orphaned and is cleaned up before
// deciding how to handle the error: on ErrAlreadyLinked (a concurrent
// request won the race to link this exact platform account first) the
// winner's linked account/account are re-fetched and returned instead of
// treating it as a hard failure; any other error is returned as-is (wrapped
// together with a cleanup failure, if the cleanup itself also failed).
func resolveOrCreateAccountForPlatformUser(ctx context.Context, as account.Service, las Store, platform Platform, user PlatformData) (*account.Account, error) {
	la, err := las.GetLinkedAccountByPlatformID(ctx, platform, user.GetID())
	if err == nil {
		if !(la.Verified && la.LoginEnabled) {
			return nil, ErrPlatformLoginDisabled
		}
		return as.GetAccountByID(ctx, la.UserID)
	}
	if err != account.ErrNotFound {
		return nil, err
	}

	a := account.NewPasswordLessAccount(user.GetUsername(), user.GetEmail())
	if err := as.AddAccount(ctx, a); err != nil {
		return nil, err
	}

	la = NewLinkedAccount(a.UserID, platform, user.GetUsername(), user.GetID(), user)
	if err := las.AddLinkedAccountToDB(context.WithoutCancel(ctx), la); err != nil {
		// Whatever went wrong, the account created above is now orphaned -
		// clean it up before deciding how to handle err.
		if delErr := as.DeleteAccount(context.WithoutCancel(ctx), a.UserID); delErr != nil {
			reqctx.Log(ctx, logLinkCleanupFailed, err.Error())
			return nil, delErr
		}
		if err != ErrAlreadyLinked {
			return nil, err
		}
		// Lost the race: use the winner's account instead.
		la, err = las.GetLinkedAccountByPlatformID(ctx, platform, user.GetID())
		if err != nil {
			return nil, err
		}
		a, err = as.GetAccountByID(ctx, la.UserID)
		if err != nil {
			return nil, err
		}
	}
	return a, nil
}

// ProcessOAuthLink links an account to an existing user
func ProcessOAuthLink(r *http.Request, cfg *config.Config, las Store, code string, state *OAuthState) (*session.Session, error) {
	ctx := r.Context()
	// Get session from request context first: there's no point exchanging
	// the OAuth code or calling out to the platform's API for a session
	// that's missing or already expired.
	session, ok := ctx.Value(reqctx.SessionKey).(*session.Session)
	if !ok || session == nil {
		return nil, ErrSessionNotFound
	}
	if !session.IsValid() {
		return nil, ErrSessionExpired
	}

	var err error
	var config *oauth2.Config
	switch state.Platform {
	case PlatformDiscord:
		config = cfg.Discord.OAuth2Config
	case PlatformTwitch:
		config = cfg.Twitch.OAuth2Config
	case PlatformMinecraft, PlatformXboxLive:
		config = cfg.Microsoft.OAuth2Config
	case PlatformMicrosoft:
		config = cfg.Microsoft.LoginOAuth2Config
	default:
		return nil, ErrInvalidPlatform
	}
	var token *OAuthToken
	token, err = ExtCodeForToken(ctx, config, code)
	if err != nil {
		return nil, err
	}

	if state.Platform == PlatformMinecraft || state.Platform == PlatformXboxLive {
		var xbox *XboxLiveData
		var java *MinecraftData
		var xboxErr error
		if state.Platform == PlatformXboxLive {
			xbox, xboxErr = GetXboxUser(ctx, cfg.Microsoft, token)
		} else {
			xbox, java, xboxErr = GetXboxAndMinecraftUser(ctx, cfg.Microsoft, token)
		}
		if xboxErr != nil {
			return nil, xboxErr
		}

		// Check both identities up front, before committing either link:
		// linking Xbox and only then rejecting Java would leave Xbox linked
		// with no way to undo it (AddLinkedAccountToDB has no delete).
		if la, err := las.GetLinkedAccountByPlatformID(ctx, PlatformXboxLive, xbox.GetID()); err == nil {
			if la.UserID != session.UserID {
				return nil, ErrPlatformAlreadyLinkedToDifferentAccount
			}
		} else if err != account.ErrNotFound {
			return nil, err
		}
		if java != nil {
			if la, err := las.GetLinkedAccountByPlatformID(ctx, PlatformMinecraft, java.GetID()); err == nil {
				if la.UserID != session.UserID {
					return nil, ErrPlatformAlreadyLinkedToDifferentAccount
				}
			} else if err != account.ErrNotFound {
				return nil, err
			}
		}

		if err := linkPlatformUserToSession(ctx, las, session.UserID, PlatformXboxLive, xbox); err != nil {
			return nil, err
		}
		if java != nil {
			if err := linkPlatformUserToSession(ctx, las, session.UserID, PlatformMinecraft, java); err != nil {
				return nil, err
			}
		}
		return session, nil
	}

	var user PlatformData
	switch state.Platform {
	case PlatformDiscord:
		user, err = GetDiscordUser(token)
	case PlatformTwitch:
		user, err = GetTwitchUser(ctx, cfg.Twitch, token)
	case PlatformMicrosoft:
		user, err = GetMicrosoftUser(ctx, cfg.Microsoft, token)
	default:
		return nil, ErrInvalidPlatform
	}
	if err != nil {
		return nil, err
	}

	if err := linkPlatformUserToSession(ctx, las, session.UserID, state.Platform, user); err != nil {
		return nil, err
	}
	return session, nil
}

var ErrConflictingMicrosoftIdentities = errors.New("this Microsoft account's Xbox Live and Minecraft: Java Edition identities are linked to two different accounts; unlink one before linking via Microsoft again")

// resolveOrCreateAccountForMicrosoftUser resolves the account for a
// Microsoft-authenticated login, given the caller's Xbox Live identity
// (always present) and Java Edition profile (present only if owned).
// Whichever identity/identities aren't linked yet get linked to the
// resolved (or freshly created) account.
func resolveOrCreateAccountForMicrosoftUser(ctx context.Context, as account.Service, las Store, xbox *XboxLiveData, java *MinecraftData) (*account.Account, error) {
	xboxLA, err := las.GetLinkedAccountByPlatformID(ctx, PlatformXboxLive, xbox.GetID())
	if err != nil {
		if err != account.ErrNotFound {
			return nil, err
		}
		xboxLA = nil
	}
	var javaLA *LinkedAccount
	if java != nil {
		javaLA, err = las.GetLinkedAccountByPlatformID(ctx, PlatformMinecraft, java.GetID())
		if err != nil {
			if err != account.ErrNotFound {
				return nil, err
			}
			javaLA = nil
		}
	}

	xboxAccountID, javaAccountID := "", ""
	if xboxLA != nil {
		xboxAccountID = xboxLA.UserID
	}
	if javaLA != nil {
		javaAccountID = javaLA.UserID
	}
	if xboxAccountID != "" && javaAccountID != "" && xboxAccountID != javaAccountID {
		return nil, ErrConflictingMicrosoftIdentities
	}

	// An ineligible identity only blocks login if the other one isn't
	// eligible either.
	xboxEligible := xboxLA != nil && xboxLA.Verified && xboxLA.LoginEnabled
	javaEligible := javaLA != nil && javaLA.Verified && javaLA.LoginEnabled
	if (xboxLA != nil || javaLA != nil) && !xboxEligible && !javaEligible {
		return nil, ErrPlatformLoginDisabled
	}

	accountID := xboxAccountID
	if accountID == "" {
		accountID = javaAccountID
	}

	var a *account.Account
	isNewAccount := accountID == ""
	if isNewAccount {
		// Prefer the Java username when both identities are present - it's
		// the player's own chosen name, unlike the Xbox gamertag.
		username := xbox.GetUsername()
		if java != nil {
			username = java.GetUsername()
		}
		a = account.NewPasswordLessAccount(username, "")
		if err := as.AddAccount(ctx, a); err != nil {
			return nil, err
		}
	} else {
		a, err = as.GetAccountByID(ctx, accountID)
		if err != nil {
			return nil, err
		}
	}

	if xboxAccountID == "" {
		a, isNewAccount, err = ensureMicrosoftIdentityLinked(ctx, as, las, a, isNewAccount, PlatformXboxLive, xbox)
		if err != nil {
			return nil, err
		}
	}
	if java != nil && javaAccountID == "" {
		a, _, err = ensureMicrosoftIdentityLinked(ctx, as, las, a, isNewAccount, PlatformMinecraft, java)
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
func ensureMicrosoftIdentityLinked(ctx context.Context, as account.Service, las Store, a *account.Account, isNewAccount bool, platform Platform, user PlatformData) (*account.Account, bool, error) {
	la := NewLinkedAccount(a.UserID, platform, user.GetUsername(), user.GetID(), user)
	err := las.AddLinkedAccountToDB(context.WithoutCancel(ctx), la)
	if err == nil {
		return a, false, nil
	}
	if err != ErrAlreadyLinked {
		if isNewAccount {
			if delErr := as.DeleteAccount(context.WithoutCancel(ctx), a.UserID); delErr != nil {
				reqctx.Log(ctx, logLinkCleanupFailed, err.Error())
				return nil, false, delErr
			}
		}
		return nil, false, err
	}

	// We might already be the owner (a concurrent identical login won the
	// race for both identities) rather than facing a genuine conflict.
	actualOwnerLA, lookupErr := las.GetLinkedAccountByPlatformID(ctx, platform, user.GetID())
	if lookupErr != nil && lookupErr != account.ErrNotFound {
		if isNewAccount {
			if delErr := as.DeleteAccount(context.WithoutCancel(ctx), a.UserID); delErr != nil {
				reqctx.Log(ctx, logLinkCleanupFailed, lookupErr.Error())
				return nil, false, delErr
			}
		}
		return nil, false, lookupErr
	}
	actualOwnerID := ""
	if lookupErr == nil {
		actualOwnerID = actualOwnerLA.UserID
	}
	if actualOwnerID == a.UserID {
		return a, false, nil
	}
	if !isNewAccount {
		return nil, false, ErrConflictingMicrosoftIdentities
	}

	if delErr := as.DeleteAccount(context.WithoutCancel(ctx), a.UserID); delErr != nil {
		reqctx.Log(ctx, logLinkCleanupFailed, err.Error())
		return nil, false, delErr
	}
	winner, getErr := as.GetAccountByID(ctx, actualOwnerID)
	if getErr != nil {
		return nil, false, getErr
	}
	return winner, false, nil
}

var ErrPlatformAlreadyLinkedToDifferentAccount = errors.New("this platform account is already linked to a different account; log in with it directly if you want to use that account, or unlink it there first")

// linkPlatformUserToSession links the given platform identity to userID. If
// it's already linked to a different account, that's returned as an error;
// if it's already linked to this same account, linking is a no-op.
func linkPlatformUserToSession(ctx context.Context, las Store, userID string, platform Platform, user PlatformData) error {
	la, err := las.GetLinkedAccountByPlatformID(ctx, platform, user.GetID())
	switch err {
	case nil:
		if userID == la.UserID {
			return nil
		}
		return ErrPlatformAlreadyLinkedToDifferentAccount
	case account.ErrNotFound:
	default:
		return err
	}

	// Link account
	la = NewLinkedAccount(userID, platform, user.GetUsername(), user.GetID(), user)
	if err := las.AddLinkedAccountToDB(context.WithoutCancel(ctx), la); err != nil {
		switch err {
		case ErrUserAlreadyLinkedToPlatform:
			return err
		case ErrAlreadyLinked:
			owner, getErr := las.GetLinkedAccountByPlatformID(ctx, platform, user.GetID())
			if getErr != nil {
				reqctx.Log(ctx, ErrLinkAccountFailed.Error(), getErr.Error())
				return ErrLinkAccountFailed
			}
			if owner.UserID == userID {
				return nil
			}
			return ErrPlatformAlreadyLinkedToDifferentAccount
		default:
			reqctx.Log(ctx, ErrLinkAccountFailed.Error(), err.Error())
			return ErrLinkAccountFailed
		}
	}

	return nil
}
