package linking

import (
	"errors"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
)

// -------------- resolveOrCreateAccountForMicrosoftUser tests --------------

func TestResolveOrCreateAccountForMicrosoftUserCreatesNewAccountWithBothIdentities(t *testing.T) {
	as := newMockAccountService()
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}
	java := &MinecraftData{Username: "JavaName"}

	a, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, java)
	if err != nil {
		t.Fatalf("resolveOrCreateAccountForMicrosoftUser returned error: %v", err)
	}
	if len(as.accounts) != 1 {
		t.Fatalf("expected exactly one account to be created, got %d", len(as.accounts))
	}
	if a.Username != "JavaName" {
		t.Errorf("expected the account to be seeded with the Java username when both identities are present, got %q", a.Username)
	}
	if len(als.addCalls) != 2 {
		t.Fatalf("expected both identities to be linked, got %d AddLinkedAccountToDB calls", len(als.addCalls))
	}
}

func TestResolveOrCreateAccountForMicrosoftUserBedrockOnlyCreatesAccount(t *testing.T) {
	as := newMockAccountService()
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}

	a, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, nil)
	if err != nil {
		t.Fatalf("resolveOrCreateAccountForMicrosoftUser returned error: %v", err)
	}
	if a.Username != "GamerTag" {
		t.Errorf("expected the account to be seeded with the gamertag when there's no Java profile, got %q", a.Username)
	}
	if len(als.addCalls) != 1 || als.addCalls[0].Platform != auth.PlatformXboxLive {
		t.Fatalf("expected exactly one Xbox Live link to be created, got: %+v", als.addCalls)
	}
}

func TestResolveOrCreateAccountForMicrosoftUserUsesExistingXboxAccountAndLinksJava(t *testing.T) {
	as := newMockAccountService()
	as.accounts["existing-xbox-acct"] = &auth.Account{UserID: "existing-xbox-acct", Username: "existing"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
			if platform == auth.PlatformXboxLive {
				return &auth.LinkedAccount{UserID: "existing-xbox-acct", Verified: true, LoginEnabled: true}, nil
			}
			return nil, auth.ErrNotFound
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}
	java := &MinecraftData{Username: "JavaName"}

	a, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, java)
	if err != nil {
		t.Fatalf("resolveOrCreateAccountForMicrosoftUser returned error: %v", err)
	}
	if a.UserID != "existing-xbox-acct" {
		t.Errorf("expected the existing account to be reused, got %q", a.UserID)
	}
	if len(as.accounts) != 1 {
		t.Errorf("expected no new account to be created, got %d", len(as.accounts))
	}
	if len(als.addCalls) != 1 || als.addCalls[0].Platform != auth.PlatformMinecraft || als.addCalls[0].UserID != "existing-xbox-acct" {
		t.Fatalf("expected the Java identity to be linked to the existing account, got: %+v", als.addCalls)
	}
}

func TestResolveOrCreateAccountForMicrosoftUserUsesExistingJavaAccountAndLinksXbox(t *testing.T) {
	as := newMockAccountService()
	as.accounts["existing-java-acct"] = &auth.Account{UserID: "existing-java-acct", Username: "existing"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
			if platform == auth.PlatformMinecraft {
				return &auth.LinkedAccount{UserID: "existing-java-acct", Verified: true, LoginEnabled: true}, nil
			}
			return nil, auth.ErrNotFound
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}
	java := &MinecraftData{Username: "JavaName"}

	a, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, java)
	if err != nil {
		t.Fatalf("resolveOrCreateAccountForMicrosoftUser returned error: %v", err)
	}
	if a.UserID != "existing-java-acct" {
		t.Errorf("expected the existing account to be reused, got %q", a.UserID)
	}
	if len(als.addCalls) != 1 || als.addCalls[0].Platform != auth.PlatformXboxLive || als.addCalls[0].UserID != "existing-java-acct" {
		t.Fatalf("expected the Xbox Live identity to be linked to the existing account, got: %+v", als.addCalls)
	}
}

func TestResolveOrCreateAccountForMicrosoftUserBothAlreadyLinkedSameAccountIsNoOp(t *testing.T) {
	as := newMockAccountService()
	as.accounts["acct1"] = &auth.Account{UserID: "acct1"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "acct1", Verified: true, LoginEnabled: true}, nil
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}
	java := &MinecraftData{Username: "JavaName"}

	a, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, java)
	if err != nil {
		t.Fatalf("resolveOrCreateAccountForMicrosoftUser returned error: %v", err)
	}
	if a.UserID != "acct1" {
		t.Errorf("expected acct1 to be returned, got %q", a.UserID)
	}
	if len(als.addCalls) != 0 {
		t.Errorf("expected no new links when both identities are already linked to the same account, got %d", len(als.addCalls))
	}
	if len(as.accounts) != 1 {
		t.Errorf("expected no new account to be created, got %d", len(as.accounts))
	}
}

// TestResolveOrCreateAccountForMicrosoftUserConflictingAccountsReturnsError covers
// the case where a Microsoft account's Xbox Live and Java identities were
// somehow linked to two different NN accounts on separate occasions.
// resolveOrCreateAccountForMicrosoftUser must refuse to silently pick one,
// the same way linkPlatformUserToSession refuses to silently steal an
// identity already linked elsewhere.
func TestResolveOrCreateAccountForMicrosoftUserConflictingAccountsReturnsError(t *testing.T) {
	as := newMockAccountService()
	as.accounts["xbox-acct"] = &auth.Account{UserID: "xbox-acct"}
	as.accounts["java-acct"] = &auth.Account{UserID: "java-acct"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
			if platform == auth.PlatformXboxLive {
				return &auth.LinkedAccount{UserID: "xbox-acct"}, nil
			}
			return &auth.LinkedAccount{UserID: "java-acct"}, nil
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}
	java := &MinecraftData{Username: "JavaName"}

	_, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, java)
	if err == nil {
		t.Fatal("expected an error when Xbox Live and Java identities resolve to two different accounts")
	}
	if len(als.addCalls) != 0 {
		t.Errorf("expected no linking attempts on conflict, got %d", len(als.addCalls))
	}
	if len(as.accounts) != 2 {
		t.Errorf("expected neither existing account to be touched, got %d accounts", len(as.accounts))
	}
}

func TestResolveOrCreateAccountForMicrosoftUserLookupErrorPropagates(t *testing.T) {
	as := newMockAccountService()
	wantErr := errors.New("db exploded")
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, wantErr
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}

	_, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, nil)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected the lookup error to propagate, got: %v", err)
	}
	if len(as.accounts) != 0 {
		t.Error("no account should be created when the initial lookup fails for an unexpected reason")
	}
}

// TestResolveOrCreateAccountForMicrosoftUserRaceLostOnFreshAccountUsesWinner
// mirrors TestResolveOrCreateAccountForPlatformUserRaceLostCleansUpAndUsesWinner
// for the Microsoft path: two concurrent Microsoft logins for the same Xbox
// identity both find it unlinked, both create a placeholder account, and
// only one AddLinkedAccountToDB wins. The loser must clean up its orphaned
// placeholder and adopt the winner's account instead of erroring out.
func TestResolveOrCreateAccountForMicrosoftUserRaceLostOnFreshAccountUsesWinner(t *testing.T) {
	as := newMockAccountService()
	as.accounts["winner1"] = &auth.Account{UserID: "winner1"}

	lookupCall := 0
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
			lookupCall++
			if lookupCall == 1 {
				return nil, auth.ErrNotFound
			}
			return &auth.LinkedAccount{UserID: "winner1", Platform: platform, PlatformID: platformID}, nil
		},
		addFunc: func(*auth.LinkedAccount) error {
			return auth.ErrAlreadyLinked
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}

	a, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, nil)
	if err != nil {
		t.Fatalf("resolveOrCreateAccountForMicrosoftUser returned error: %v", err)
	}
	if a.UserID != "winner1" {
		t.Errorf("expected the winner's account to be returned, got %q", a.UserID)
	}
	if len(as.accounts) != 1 {
		t.Errorf("expected only the winner's account to remain (placeholder cleaned up), got %d: %v", len(as.accounts), as.accounts)
	}
	if _, stillThere := as.accounts["winner1"]; !stillThere {
		t.Error("cleanup deleted the winner's account instead of the placeholder")
	}
}

// TestResolveOrCreateAccountForMicrosoftUserRaceAlreadyUsIsNoOp covers the
// case where the "race" is actually us: a concurrent identical Microsoft
// login already linked this exact identity to the very account we resolved
// to (e.g. it linked Xbox moments before we tried Java on the same fresh
// account). That must resolve as a no-op, not a conflict or a deleted
// account.
func TestResolveOrCreateAccountForMicrosoftUserRaceAlreadyUsIsNoOp(t *testing.T) {
	as := newMockAccountService()
	as.accounts["acct1"] = &auth.Account{UserID: "acct1"}
	javaLookupCall := 0
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
			if platform == auth.PlatformXboxLive {
				return &auth.LinkedAccount{UserID: "acct1", Verified: true, LoginEnabled: true}, nil
			}
			// Java: the upfront pre-check finds it unlinked, but by the
			// time we try to link it, a concurrent identical Microsoft
			// login has already linked it to this same account (acct1).
			javaLookupCall++
			if javaLookupCall == 1 {
				return nil, auth.ErrNotFound
			}
			return &auth.LinkedAccount{UserID: "acct1"}, nil
		},
		addFunc: func(*auth.LinkedAccount) error {
			return auth.ErrAlreadyLinked
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}
	java := &MinecraftData{Username: "JavaName"}

	a, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, java)
	if err != nil {
		t.Fatalf("expected the self-race to resolve as a no-op, got error: %v", err)
	}
	if a.UserID != "acct1" {
		t.Errorf("expected acct1, got %q", a.UserID)
	}
	if len(as.accounts) != 1 {
		t.Errorf("expected acct1 to still be the only account (never deleted), got %d: %v", len(as.accounts), as.accounts)
	}
}

// TestResolveOrCreateAccountForMicrosoftUserRaceOnExistingAccountIsConflict
// covers a lost race for an identity when the account it's being linked to
// is *not* a fresh placeholder (it already has the other identity linked
// legitimately) - deleting it to defer to the "winner" would violate the
// linked_accounts foreign key, so this must surface as a conflict instead.
func TestResolveOrCreateAccountForMicrosoftUserRaceOnExistingAccountIsConflict(t *testing.T) {
	as := newMockAccountService()
	as.accounts["xbox-acct"] = &auth.Account{UserID: "xbox-acct"}
	as.accounts["someone-else"] = &auth.Account{UserID: "someone-else"}
	javaLookupCall := 0
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
			if platform == auth.PlatformXboxLive {
				return &auth.LinkedAccount{UserID: "xbox-acct", Verified: true, LoginEnabled: true}, nil
			}
			// Java: the upfront pre-check finds it unlinked, but by the
			// time we try to link it, a concurrent request has already
			// linked it to a different account - the race this test is for.
			javaLookupCall++
			if javaLookupCall == 1 {
				return nil, auth.ErrNotFound
			}
			return &auth.LinkedAccount{UserID: "someone-else"}, nil
		},
		addFunc: func(*auth.LinkedAccount) error {
			return auth.ErrAlreadyLinked
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}
	java := &MinecraftData{Username: "JavaName"}

	_, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, java)
	if !errors.Is(err, errConflictingMicrosoftIdentities) {
		t.Errorf("expected errConflictingMicrosoftIdentities, got: %v", err)
	}
	if len(as.accounts) != 2 {
		t.Errorf("expected neither account to be touched, got %d: %v", len(as.accounts), as.accounts)
	}
}

// TestResolveOrCreateAccountForMicrosoftUserJavaRaceAfterXboxLinkedIsConflictNotDeletion
// is a regression test for a bug found by the review pipeline's play-around
// agent: ensureMicrosoftIdentityLinked's success paths returned the
// isNewAccount flag it was *called* with, unchanged, instead of false. So
// once Xbox linked successfully on a fresh account (still isNewAccount ==
// true from before that call), a subsequent genuine race on Java - losing
// to a different, pre-existing account - wrongly took the "bare placeholder,
// safe to delete" branch and deleted the account, even though it now held a
// real, already-committed Xbox Live link. That's not a bare placeholder
// anymore; it must surface as errConflictingMicrosoftIdentities instead,
// exactly like a race against an account that was never new to begin with.
func TestResolveOrCreateAccountForMicrosoftUserJavaRaceAfterXboxLinkedIsConflictNotDeletion(t *testing.T) {
	as := newMockAccountService()
	as.accounts["someone-else-java-acct"] = &auth.Account{UserID: "someone-else-java-acct"}
	javaLookupCall := 0
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
			if platform == auth.PlatformXboxLive {
				return nil, auth.ErrNotFound
			}
			// Java: the upfront pre-check finds it unlinked, but by the time
			// we try to link it - after Xbox already succeeded on our fresh
			// account - a concurrent request has linked it to a different,
			// pre-existing account.
			javaLookupCall++
			if javaLookupCall == 1 {
				return nil, auth.ErrNotFound
			}
			return &auth.LinkedAccount{UserID: "someone-else-java-acct"}, nil
		},
		addFunc: func(la *auth.LinkedAccount) error {
			if la.Platform == auth.PlatformXboxLive {
				return nil
			}
			return auth.ErrAlreadyLinked
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}
	java := &MinecraftData{Username: "JavaName"}

	_, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, java)
	if !errors.Is(err, errConflictingMicrosoftIdentities) {
		t.Fatalf("expected errConflictingMicrosoftIdentities, got: %v", err)
	}
	if len(as.deletedIDs) != 0 {
		t.Errorf("expected the account with its real, committed Xbox Live link to survive, but something was deleted: %v", as.deletedIDs)
	}
	if len(as.accounts) != 2 {
		t.Errorf("expected the new Xbox-linked account plus the untouched pre-existing Java-owner account, got %d: %v", len(as.accounts), as.accounts)
	}
}

// TestResolveOrCreateAccountForMicrosoftUserCleansUpPlaceholderOnGenericLinkError
// is a regression test from the review pipeline: ensureMicrosoftIdentityLinked
// only cleaned up the freshly-created placeholder account when the link
// failure was specifically auth.ErrAlreadyLinked (the race-recovery path).
// Any other AddLinkedAccountToDB failure - a dropped connection, timeout, or
// any other real DB error, all passed straight through unwrapped by the real
// store - returned immediately with no cleanup, permanently orphaning the
// account row. resolveOrCreateAccountForPlatformUser (the pre-existing
// Discord/Twitch path) already guards against exactly this by deleting the
// placeholder on ANY AddLinkedAccountToDB failure before even checking
// whether it was a race; this pins the Microsoft path to the same guarantee.
func TestResolveOrCreateAccountForMicrosoftUserCleansUpPlaceholderOnGenericLinkError(t *testing.T) {
	as := newMockAccountService()
	wantErr := errors.New("connection reset by peer")
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return nil, auth.ErrNotFound
		},
		addFunc: func(*auth.LinkedAccount) error {
			return wantErr
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}

	_, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, nil)
	if !errors.Is(err, wantErr) {
		t.Errorf("expected the original AddLinkedAccountToDB error to propagate, got: %v", err)
	}
	if len(as.accounts) != 0 {
		t.Errorf("expected the orphaned placeholder account to be cleaned up, got %d accounts remaining: %v", len(as.accounts), as.accounts)
	}
	if len(as.deletedIDs) != 1 {
		t.Errorf("expected DeleteAccount to be called once to clean up the orphaned account, got: %v", as.deletedIDs)
	}
}

// -------------- login_enabled / verified eligibility --------------

// TestResolveOrCreateAccountForMicrosoftUserXboxDisabledNoJavaRejected is
// the core regression test for wanting to disable Xbox Live for login
// without losing Java: Xbox is linked to a real account but disabled for
// login, and this account doesn't own Java at all (java == nil, same as a
// genuine Bedrock-only player). This must be rejected outright, not treated
// as "Xbox unlinked" (which would create a brand new, duplicate account).
func TestResolveOrCreateAccountForMicrosoftUserXboxDisabledNoJavaRejected(t *testing.T) {
	as := newMockAccountService()
	as.accounts["existing-acct"] = &auth.Account{UserID: "existing-acct"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "existing-acct", Verified: true, LoginEnabled: false}, nil
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}

	_, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, nil)
	if !errors.Is(err, errPlatformLoginDisabled) {
		t.Fatalf("expected errPlatformLoginDisabled, got: %v", err)
	}
	if len(as.accounts) != 1 {
		t.Errorf("expected no new account to be created, got %d: %v", len(as.accounts), as.accounts)
	}
	if len(als.addCalls) != 0 {
		t.Error("expected no linking attempt")
	}
}

// TestResolveOrCreateAccountForMicrosoftUserXboxDisabledJavaEligibleSucceeds
// verifies the other half: disabling Xbox Live for login must NOT break
// login via Java on the same account - they're independent identities, and
// as long as one of them is eligible, login proceeds through it.
func TestResolveOrCreateAccountForMicrosoftUserXboxDisabledJavaEligibleSucceeds(t *testing.T) {
	as := newMockAccountService()
	as.accounts["existing-acct"] = &auth.Account{UserID: "existing-acct"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(platform auth.Platform, platformID string) (*auth.LinkedAccount, error) {
			if platform == auth.PlatformXboxLive {
				return &auth.LinkedAccount{UserID: "existing-acct", Verified: true, LoginEnabled: false}, nil
			}
			return &auth.LinkedAccount{UserID: "existing-acct", Verified: true, LoginEnabled: true}, nil
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}
	java := &MinecraftData{Username: "JavaName"}

	a, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, java)
	if err != nil {
		t.Fatalf("expected login to succeed via the still-eligible Java identity, got: %v", err)
	}
	if a.UserID != "existing-acct" {
		t.Errorf("expected existing-acct, got %q", a.UserID)
	}
}

// TestResolveOrCreateAccountForMicrosoftUserBothLinkedBothDisabledRejected
// covers both identities being linked (to the same account) but BOTH
// disabled for login - unlike the single-identity case, this confirms the
// eligibility check looks at every identity present, not just the first
// one checked.
func TestResolveOrCreateAccountForMicrosoftUserBothLinkedBothDisabledRejected(t *testing.T) {
	as := newMockAccountService()
	as.accounts["existing-acct"] = &auth.Account{UserID: "existing-acct"}
	als := &mockLinkAccountStore{
		getByPlatformIDFunc: func(auth.Platform, string) (*auth.LinkedAccount, error) {
			return &auth.LinkedAccount{UserID: "existing-acct", Verified: true, LoginEnabled: false}, nil
		},
	}
	xbox := &XboxLiveData{XUID: "xuid1", Gamertag: "GamerTag"}
	java := &MinecraftData{Username: "JavaName"}

	_, err := resolveOrCreateAccountForMicrosoftUser(as, als, xbox, java)
	if !errors.Is(err, errPlatformLoginDisabled) {
		t.Fatalf("expected errPlatformLoginDisabled, got: %v", err)
	}
}
