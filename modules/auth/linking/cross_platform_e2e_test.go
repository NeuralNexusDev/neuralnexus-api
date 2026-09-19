package linking

import (
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
)

// This file exercises ProcessOAuthLink across combinations of DIFFERENT
// platforms stacking on one session's account - as opposed to the
// single-platform e2e tests in discord_e2e_test.go/twitch_e2e_test.go/
// microsoft_e2e_test.go, which only ever link one platform per test. It uses
// concurrentLinkAccountStore (from oauth_test.go) rather than the scripted
// mockLinkAccountStore, since these tests need a real, persistent store
// that remembers links made by an earlier ProcessOAuthLink call in the same
// test - a session linking Discord, then Twitch, then Minecraft in
// sequence, the way a real user actually would one link at a time.

// seedLink adds a linked_accounts row directly, simulating an identity
// that was linked in an earlier, separate operation (by this account or a
// different one).
func seedLink(t *testing.T, store *concurrentLinkAccountStore, userID string, platform auth.Platform, platformID string) {
	t.Helper()
	if err := store.AddLinkedAccountToDB(&auth.LinkedAccount{UserID: userID, Platform: platform, PlatformID: platformID}); err != nil {
		t.Fatalf("failed to seed %s link: %v", platform, err)
	}
}

// countLinksForUser counts how many linked_accounts rows belong to userID -
// used to assert nothing extra was added and nothing existing disappeared.
func countLinksForUser(store *concurrentLinkAccountStore, userID string) int {
	store.mu.Lock()
	defer store.mu.Unlock()
	n := 0
	for _, la := range store.byKey {
		if la.UserID == userID {
			n++
		}
	}
	return n
}

func newLinkSession(userID string) *auth.Session {
	return &auth.Session{ID: "s1", UserID: userID, ExpiresAt: time.Now().Add(time.Hour).Unix()}
}

// -------------- Two top-level OAuth platforms stacking --------------

func TestProcessOAuthLinkDiscordThenTwitchFresh(t *testing.T) {
	newDiscordOAuthServer(t)
	store := newConcurrentLinkAccountStore()
	session := newLinkSession("acct-a")

	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "discord-code", newDiscordOAuthState(ModeLink)); err != nil {
		t.Fatalf("Discord link failed: %v", err)
	}

	newTwitchOAuthServer(t)
	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "twitch-code", newTwitchOAuthState(ModeLink)); err != nil {
		t.Fatalf("Twitch link failed: %v", err)
	}

	if got := countLinksForUser(store, "acct-a"); got != 2 {
		t.Errorf("expected acct-a to have both Discord and Twitch linked, got %d links", got)
	}
}

func TestProcessOAuthLinkDiscordThenTwitchAlreadyThisAccountNoOp(t *testing.T) {
	newDiscordOAuthServer(t)
	newTwitchOAuthServer(t)
	store := newConcurrentLinkAccountStore()
	session := newLinkSession("acct-a")
	seedLink(t, store, "acct-a", auth.PlatformDiscord, "9999")
	seedLink(t, store, "acct-a", auth.PlatformTwitch, "8888")

	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "twitch-code", newTwitchOAuthState(ModeLink)); err != nil {
		t.Fatalf("expected re-linking an already-own Twitch identity to be a no-op, got error: %v", err)
	}
	if got := countLinksForUser(store, "acct-a"); got != 2 {
		t.Errorf("expected still exactly 2 links (no duplicate row), got %d", got)
	}
}

func TestProcessOAuthLinkDiscordThenTwitchAlreadyDifferentAccountRejected(t *testing.T) {
	newDiscordOAuthServer(t)
	newTwitchOAuthServer(t)
	store := newConcurrentLinkAccountStore()
	session := newLinkSession("acct-a")
	seedLink(t, store, "acct-a", auth.PlatformDiscord, "9999")
	seedLink(t, store, "acct-b", auth.PlatformTwitch, "8888")

	_, err := ProcessOAuthLink(linkRequestWithSession(session), store, "twitch-code", newTwitchOAuthState(ModeLink))
	if err == nil {
		t.Fatal("expected an error when the Twitch identity already belongs to a different account")
	}
	if got := countLinksForUser(store, "acct-a"); got != 1 {
		t.Errorf("expected acct-a's Discord link to be untouched and nothing new added, got %d links", got)
	}
}

// -------------- A Bedrock-only (Xbox Live) account plus a second platform --------------

func TestProcessOAuthLinkXboxLiveThenDiscordFresh(t *testing.T) {
	newFullChainServer(t, false) // Bedrock-only: only Xbox Live gets linked
	store := newConcurrentLinkAccountStore()
	session := newLinkSession("acct-a")

	mcState := newMinecraftOAuthState()
	mcState.Mode = ModeLink
	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "mc-code", mcState); err != nil {
		t.Fatalf("Xbox Live link failed: %v", err)
	}

	newDiscordOAuthServer(t)
	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "discord-code", newDiscordOAuthState(ModeLink)); err != nil {
		t.Fatalf("Discord link failed: %v", err)
	}

	if got := countLinksForUser(store, "acct-a"); got != 2 {
		t.Errorf("expected acct-a to have both Xbox Live and Discord linked, got %d links", got)
	}
}

func TestProcessOAuthLinkXboxLiveThenDiscordAlreadyDifferentAccountRejected(t *testing.T) {
	newFullChainServer(t, false)
	newDiscordOAuthServer(t)
	store := newConcurrentLinkAccountStore()
	session := newLinkSession("acct-a")

	mcState := newMinecraftOAuthState()
	mcState.Mode = ModeLink
	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "mc-code", mcState); err != nil {
		t.Fatalf("Xbox Live link failed: %v", err)
	}
	seedLink(t, store, "acct-b", auth.PlatformDiscord, "9999")

	_, err := ProcessOAuthLink(linkRequestWithSession(session), store, "discord-code", newDiscordOAuthState(ModeLink))
	if err == nil {
		t.Fatal("expected an error when the Discord identity already belongs to a different account")
	}
	if got := countLinksForUser(store, "acct-a"); got != 1 {
		t.Errorf("expected acct-a's Xbox Live link to be untouched and nothing new added, got %d links", got)
	}
}

// TestProcessOAuthLinkTwitchThenMinecraftXboxAlreadyDifferentAccountRejected
// verifies that rejecting a Minecraft link (because Xbox Live already
// belongs to someone else) leaves an unrelated, previously-linked platform
// (Twitch) on the session's account completely untouched.
func TestProcessOAuthLinkTwitchThenMinecraftXboxAlreadyDifferentAccountRejected(t *testing.T) {
	newTwitchOAuthServer(t)
	newFullChainServer(t, false)
	store := newConcurrentLinkAccountStore()
	session := newLinkSession("acct-a")

	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "twitch-code", newTwitchOAuthState(ModeLink)); err != nil {
		t.Fatalf("Twitch link failed: %v", err)
	}
	seedLink(t, store, "acct-b", auth.PlatformXboxLive, "9999")

	mcState := newMinecraftOAuthState()
	mcState.Mode = ModeLink
	_, err := ProcessOAuthLink(linkRequestWithSession(session), store, "mc-code", mcState)
	if err == nil {
		t.Fatal("expected an error when the Xbox Live identity already belongs to a different account")
	}
	if got := countLinksForUser(store, "acct-a"); got != 1 {
		t.Errorf("expected acct-a's Twitch link to be untouched and nothing new added, got %d links", got)
	}
}

// -------------- Multi-platform accounts (3+ platforms already linked) --------------

// TestProcessOAuthLinkMultiPlatformMinecraftRejectionLeavesOtherPlatformsIntact
// is the highest-value case from the review pipeline's cross-platform
// matrix: an account with TWO already-linked platforms (Discord, Xbox Live)
// attempts a Minecraft link whose Java identity belongs to a third,
// unrelated account. The rejection must not collaterally touch either
// pre-existing link - nothing in ProcessOAuthLink should ever scan or
// mutate "all of a session's links" when only one identity conflicts.
func TestProcessOAuthLinkMultiPlatformMinecraftRejectionLeavesOtherPlatformsIntact(t *testing.T) {
	store := newConcurrentLinkAccountStore()
	session := newLinkSession("acct-a")

	newDiscordOAuthServer(t)
	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "discord-code", newDiscordOAuthState(ModeLink)); err != nil {
		t.Fatalf("Discord link failed: %v", err)
	}
	newFullChainServer(t, false)
	mcState := newMinecraftOAuthState()
	mcState.Mode = ModeLink
	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "mc-code-1", mcState); err != nil {
		t.Fatalf("Xbox Live link failed: %v", err)
	}
	if got := countLinksForUser(store, "acct-a"); got != 2 {
		t.Fatalf("expected 2 links after Discord+Xbox Live setup, got %d", got)
	}

	// Java identity already belongs to a completely different account.
	seedLink(t, store, "acct-c", auth.PlatformMinecraft, "069a79f4-44e9-4726-a5be-fca90e38aaf6")

	// Xbox Live resolves as already-this-account (no-op); Java conflicts.
	newFullChainServer(t, true)
	_, err := ProcessOAuthLink(linkRequestWithSession(session), store, "mc-code-2", mcState)
	if err == nil {
		t.Fatal("expected an error when the Java identity already belongs to a different account")
	}
	if got := countLinksForUser(store, "acct-a"); got != 2 {
		t.Errorf("expected acct-a's Discord and Xbox Live links to remain untouched with nothing new added, got %d links", got)
	}
	if la, lookupErr := store.GetLinkedAccountByPlatformID(auth.PlatformDiscord, "9999"); lookupErr != nil || la.UserID != "acct-a" {
		t.Error("expected the Discord link to still belong to acct-a")
	}
	if la, lookupErr := store.GetLinkedAccountByPlatformID(auth.PlatformXboxLive, "9999"); lookupErr != nil || la.UserID != "acct-a" {
		t.Error("expected the Xbox Live link to still belong to acct-a")
	}
}

// TestProcessOAuthLinkMultiPlatformMinecraftJavaAlreadyThisAccountNoOp
// mirrors the case above but the Java identity already belongs to THIS
// account (as if linked in an earlier, separate operation) - must resolve
// as a no-op, not a duplicate row and not an error, alongside the two other
// unrelated platform links.
func TestProcessOAuthLinkMultiPlatformMinecraftJavaAlreadyThisAccountNoOp(t *testing.T) {
	store := newConcurrentLinkAccountStore()
	session := newLinkSession("acct-a")

	newDiscordOAuthServer(t)
	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "discord-code", newDiscordOAuthState(ModeLink)); err != nil {
		t.Fatalf("Discord link failed: %v", err)
	}
	newFullChainServer(t, false)
	mcState := newMinecraftOAuthState()
	mcState.Mode = ModeLink
	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "mc-code-1", mcState); err != nil {
		t.Fatalf("Xbox Live link failed: %v", err)
	}
	seedLink(t, store, "acct-a", auth.PlatformMinecraft, "069a79f4-44e9-4726-a5be-fca90e38aaf6")
	if got := countLinksForUser(store, "acct-a"); got != 3 {
		t.Fatalf("expected 3 links after setup, got %d", got)
	}

	newFullChainServer(t, true)
	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "mc-code-2", mcState); err != nil {
		t.Fatalf("expected re-linking an already-own Java identity to be a no-op, got error: %v", err)
	}
	if got := countLinksForUser(store, "acct-a"); got != 3 {
		t.Errorf("expected still exactly 3 links (no duplicate row), got %d", got)
	}
}

// TestProcessOAuthLinkFourPlatformsOnOneAccount exercises the full breadth
// of auth.Platform values landing on a single account: Discord and Twitch
// already linked, then a fresh Minecraft link adds both Xbox Live and Java.
func TestProcessOAuthLinkFourPlatformsOnOneAccount(t *testing.T) {
	store := newConcurrentLinkAccountStore()
	session := newLinkSession("acct-a")

	newDiscordOAuthServer(t)
	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "discord-code", newDiscordOAuthState(ModeLink)); err != nil {
		t.Fatalf("Discord link failed: %v", err)
	}
	newTwitchOAuthServer(t)
	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "twitch-code", newTwitchOAuthState(ModeLink)); err != nil {
		t.Fatalf("Twitch link failed: %v", err)
	}

	newFullChainServer(t, true)
	mcState := newMinecraftOAuthState()
	mcState.Mode = ModeLink
	if _, err := ProcessOAuthLink(linkRequestWithSession(session), store, "mc-code", mcState); err != nil {
		t.Fatalf("Minecraft link failed: %v", err)
	}

	if got := countLinksForUser(store, "acct-a"); got != 4 {
		t.Fatalf("expected all four platform identities linked to acct-a, got %d", got)
	}
	fixedIDs := map[auth.Platform]string{
		auth.PlatformDiscord:   "9999",
		auth.PlatformTwitch:    "8888",
		auth.PlatformXboxLive:  "9999",
		auth.PlatformMinecraft: "069a79f4-44e9-4726-a5be-fca90e38aaf6",
	}
	for platform, id := range fixedIDs {
		if la, err := store.GetLinkedAccountByPlatformID(platform, id); err != nil || la.UserID != "acct-a" {
			t.Errorf("expected %s identity %q to be linked to acct-a, got la=%v err=%v", platform, id, la, err)
		}
	}
}
