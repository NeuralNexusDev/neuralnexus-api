package twitch

import (
	"bytes"
	"context"
	"errors"
	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	"github.com/goccy/go-json"
	"github.com/nicklaw5/helix/v2"
	"log"
	"net/http"
	"strings"
	"time"
)

const EventSubStatusVersionRemoved = "version_removed"

// eventSubNotification Outlines the structure of the EventSub notification
type eventSubNotification struct {
	Subscription helix.EventSubSubscription `json:"subscription"`
	Challenge    string                     `json:"challenge"`
	Event        json.RawMessage            `json:"event"`
}

// validateEventSubNotification validates the EventSub notification
func validateEventSubNotification(header http.Header, body []byte) (*eventSubNotification, error) {
	if !helix.VerifyEventSubNotification(EVENTSUB_SECRET, header, string(body)) {
		return nil, errors.New("invalid signature")
	}
	var vals eventSubNotification
	err := json.NewDecoder(bytes.NewReader(body)).Decode(&vals)
	if err != nil {
		return nil, err
	}
	if vals.Subscription.CreatedAt.Before(time.Now().Add(-10 * time.Minute)) {
		return nil, errors.New("notification is too old")
	}
	return &vals, nil
}

// handleRevocation handles the EventSub revocation notifications
func handleRevocation(ctx context.Context, userId string, eventsub EventSubService, tokens auth.OAuthTokenStore, vals eventSubNotification) error {
	var err error
	switch vals.Subscription.Status {
	case helix.EventSubStatusAuthorizationRevoked:
		mw.LogRequest(ctx, userId, "EventSub authorization revoked")
		err = tokens.DeleteOAuthToken(vals.Subscription.Condition.BroadcasterUserID, auth.PlatformTwitch)
		if err != nil {
			mw.LogRequest(ctx, userId, "Failed to delete OAuth token:", err.Error())
			return errors.New("failed to delete OAuth token")
		}
		fallthrough
	case helix.EventSubStatusUserRemoved,
		helix.EventSubStatusNotificationFailuresExceeded,
		EventSubStatusVersionRemoved:
		mw.LogRequest(ctx, userId, "EventSub subscription removed")
		err = eventsub.RevokeEventSubSubscription(vals.Subscription.ID, vals.Subscription.Status)
		if err != nil {
			mw.LogRequest(ctx, userId, "Failed to revoke EventSub subscription:", err.Error())
			return errors.New("failed to revoke EventSub subscription")
		}
	default:
		mw.LogRequest(ctx, userId, "EventSub unknown revocation status:", vals.Subscription.Status)
		return errors.New("unknown EventSub revocation status")
	}
	return nil
}

// handleVerification handles the EventSub verification challenge
func handleVerification(w http.ResponseWriter, ctx context.Context, userId string, eventsub EventSubService, vals eventSubNotification) error {
	if vals.Challenge == "" || vals.Subscription.Status != helix.EventSubStatusPending {
		mw.LogRequest(ctx, userId, "EventSub unknown verification status:", vals.Subscription.Status)
		return errors.New("unknown EventSub verification status")
	}
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(vals.Challenge))
	mw.LogRequest(ctx, userId, "EventSub challenge received, responding with challenge")

	var err = eventsub.UpdateEventSubSubscriptionStatus(vals.Subscription.ID, helix.EventSubStatusEnabled)
	if err != nil {
		mw.LogRequest(ctx, userId, "Failed to update EventSub subscription:", err.Error())
	}
	return nil
}

// handleNotification handles the EventSub notifications
func handleNotification(ctx context.Context, userId string, eventsub EventSubService, tokens auth.OAuthTokenStore, vals eventSubNotification, linked auth.LinkAccountStore) error {
	var err error
	mw.LogRequest(ctx, userId, "EventSub notification type:", vals.Subscription.Type)
	switch vals.Subscription.Type {
	case helix.EventSubTypeChannelChatMessage:
		err = handleChannelChatMessage(ctx, userId, eventsub, tokens, vals, linked)
	default:
		mw.LogRequest(ctx, userId, "EventSub unknown notification type:", vals.Subscription.Type)
		return errors.New("unknown EventSub notification type")
	}
	if err != nil {
		mw.LogRequest(ctx, userId, "Failed to handle EventSub notification:", err.Error())
		return errors.New("failed to handle EventSub notification")
	}
	return nil
}

// handleChannelChatMessage handles the EventSub chat message notifications
func handleChannelChatMessage(ctx context.Context, userId string, eventsub EventSubService, tokens auth.OAuthTokenStore, vals eventSubNotification, linked auth.LinkAccountStore) error {
	var err error
	var chatEvent helix.EventSubChannelChatMessageEvent
	err = json.NewDecoder(bytes.NewReader(vals.Event)).Decode(&chatEvent)
	if err != nil {
		mw.LogRequest(ctx, userId, "Failed to decode EventSub chat message event:", err.Error())
		return errors.New("failed to decode EventSub chat message event")
	}

	var message = chatEvent.Message.Text
	if !strings.HasPrefix(message, "!") {
		return nil
	}

	var args = strings.Split(message[1:], " ")
	if len(args) < 1 {
		mw.LogRequest(ctx, userId, "Chat message does not contain a command")
		// return errors.New("chat message does not contain a command")
	}
	switch args[0] {
	// !link platform platformNameOrId
	case "link":
		fromPlatform := auth.PlatformTwitch
		fromPlatformId := chatEvent.ChatterUserID

		toPlatform := auth.Platform(args[1])
		toPlatformId := args[2]

		if toPlatform == auth.PlatformMinecraft {
			fromLinkedAccount, _ := linked.GetLinkedAccountByPlatformID(fromPlatform, fromPlatformId)
			alreadyLinkedAccount, _ := linked.GetLinkedAccountByUserID(fromLinkedAccount.UserID, toPlatform)
			toLinkedAccount, _ := linked.GetLinkedAccountByPlatformName(toPlatform, toPlatformId)

			// Logic Matrix:
			// 1. The Twitch user is already linked to this Minecraft account
			// 2. The Twitch user is already linked to another Minecraft account

			if fromLinkedAccount != nil && toLinkedAccount != nil && fromLinkedAccount.UserID == toLinkedAccount.UserID {
				mw.LogRequest(ctx, userId, "Minecraft account is already linked to this user:", toLinkedAccount.PlatformUsername)
				// return errors.New("user is already linked to Minecraft account")
				// TODO: Reply with Twitch API
				return nil
			} else if fromLinkedAccount != nil && alreadyLinkedAccount != nil {
				if alreadyLinkedAccount.PlatformUsername == toLinkedAccount.PlatformUsername {
					mw.LogRequest(ctx, userId, "Minecraft account is already linked to this user:", alreadyLinkedAccount.PlatformUsername)
					// return errors.New("user is already linked to Minecraft account")
				}
			} else if toLinkedAccount != nil {
				mw.LogRequest(ctx, userId, "Minecraft account is already linked to another user:", toLinkedAccount.PlatformUsername)
				// return errors.New("user is already linked to Minecraft account")
				// TODO: Reply with Twitch API
				return nil
			} else if fromLinkedAccount != nil && fromLinkedAccount.Platform != fromPlatform {
			}

			// Check if the user is already linked to a Minecraft account
			if fromLinkedAccount != nil {
				tla, _ := linked.GetLinkedAccountByUserID(fromLinkedAccount.UserID, toPlatform)
				if tla != nil {
					mw.LogRequest(ctx, userId, "User is already linked to a Minecraft account:", tla.PlatformUsername)
					// return errors.New("user is already linked to Minecraft account")
					// TODO: Reply with Twitch API
					return nil
				}
			}

		} else {
			mw.LogRequest(ctx, userId, "Unsupported platform for linking:", string(toPlatform))
			// return errors.New("unsupported platform for linking")
			// TODO: Reply with Twitch API
			return nil
		}
	}

	log.Printf("User %s sent a chat message in channel %s: %s\n", chatEvent.ChatterUserID, chatEvent.BroadcasterUserID, chatEvent.Message.Text)
	return nil
}
