package link

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil/authtest"
)

func stAssertRawConnectionError(t *testing.T, err error) {
	t.Helper()
	testutil.AssertRawConnectionError(t, err, account.ErrNotFound, account.ErrEmailAlreadyExists, account.ErrUsernameAlreadyExists, ErrAlreadyLinked)
}

func stStoreWithUnreachableDB(t *testing.T) *store {
	return &store{db: testutil.UnreachablePool(t)}
}

func stLiveStore(t *testing.T) (account.Store, Store, account.SettingsStore) {
	t.Helper()
	s := stLiveFullStore(t)
	return s.Account(), s.LinkAccount(), s.AccountSettings()
}

func stLiveFullStore(t *testing.T) stLiveSet {
	t.Helper()
	return stLiveSet{db: authtest.LiveAuthPool(t)}
}

type stLiveSet struct {
	db *pgxpool.Pool
}

func (s stLiveSet) Account() account.Store { return account.NewStore(s.db) }

func (s stLiveSet) AccountSettings() account.SettingsStore {
	return account.NewSettingsStore(s.db)
}

func (s stLiveSet) LinkAccount() Store { return NewStore(s.db) }

func (s stLiveSet) OAuthToken() OAuthTokenStore { return NewOAuthTokenStore(s.db) }

func stSeedBareAccount(t *testing.T, as account.Store, userID string) {
	t.Helper()
	if err := as.AddAccountToDB(context.Background(), &account.Account{UserID: userID, Username: "sttest-" + userID}); err != nil {
		t.Fatalf("failed to seed account %s: %v", userID, err)
	}
}

func stSeedPasswordAccount(t *testing.T, as account.Store, userID string) *account.Account {
	t.Helper()
	a := &account.Account{UserID: userID, Username: "sttest-" + userID}
	if err := a.HashPassword(testutil.NewConfig().Auth.Account.Pepper, "sttest-password"); err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if err := as.AddAccountToDB(context.Background(), a); err != nil {
		t.Fatalf("failed to seed passworded account %s: %v", userID, err)
	}
	return a
}

func stSeedLink(t *testing.T, las Store, userID string, platform Platform, platformID string, verified, loginEnabled bool) {
	t.Helper()
	if err := las.AddLinkedAccountToDB(context.Background(), &LinkedAccount{
		UserID:           userID,
		Platform:         platform,
		PlatformUsername: "sttest",
		PlatformID:       platformID,
		Data:             map[string]string{},
		Verified:         verified,
		LoginEnabled:     loginEnabled,
	}); err != nil {
		t.Fatalf("failed to seed %s link for %s: %v", platform, userID, err)
	}
}

func TestLS28AddLinkedAccountToDB(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("LS-28_ConnectionErrorNotMistranslated", func(t *testing.T) {
		stAssertRawConnectionError(t, s.AddLinkedAccountToDB(context.Background(), &LinkedAccount{UserID: "u1", Platform: PlatformDiscord, PlatformID: "p1"}))
	})
}

func TestLS29UpdateLinkedAccount(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("LS-29_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.UpdateLinkedAccount(context.Background(), &LinkedAccount{UserID: "u1", Platform: PlatformDiscord}))
	})
}

func TestLS30GetLinkedAccountByPlatformID(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("LS-30_ConnectionError", func(t *testing.T) {
		got, err := s.GetLinkedAccountByPlatformID(context.Background(), PlatformDiscord, "p1")
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestLS31GetLinkedAccountByPlatformName(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("LS-31_ConnectionError", func(t *testing.T) {
		got, err := s.GetLinkedAccountByPlatformName(context.Background(), PlatformDiscord, "alice")
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestLS32GetLinkedAccountByUserID(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("LS-32_ConnectionError", func(t *testing.T) {
		got, err := s.GetLinkedAccountByUserID(context.Background(), "u1", PlatformDiscord)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestLS33GetLinkedAccountsByUserID(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("LS-33_ConnectionError", func(t *testing.T) {
		got, err := s.GetLinkedAccountsByUserID(context.Background(), "u1")
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestLS34DeleteLinkedAccount(t *testing.T) {
	s := stStoreWithUnreachableDB(t)
	t.Run("LS-34_BeginFailsBeforeGuardLogic", func(t *testing.T) {
		stAssertRawConnectionError(t, s.DeleteLinkedAccount(context.Background(), "u1", PlatformDiscord))
	})
}

func TestLS35to36SetLinkedAccountLoginEnabled(t *testing.T) {
	t.Run("LS-35_EnableConnectionError", func(t *testing.T) {
		s := stStoreWithUnreachableDB(t)
		stAssertRawConnectionError(t, s.SetLinkedAccountLoginEnabled(context.Background(), "u1", PlatformDiscord, true))
	})

	t.Run("LS-36_DisableBeginFailsBeforeGuardLogic", func(t *testing.T) {
		s := stStoreWithUnreachableDB(t)
		stAssertRawConnectionError(t, s.SetLinkedAccountLoginEnabled(context.Background(), "u1", PlatformDiscord, false))
	})
}

func TestLS54DeleteLinkedAccountBlockedAsLastLoginMethod(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("LS-54_BlockedAsLastLoginMethod", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000010")
		stSeedLink(t, las, "910000000000000010", PlatformDiscord, "sttest-discord-10", true, true)

		err := las.DeleteLinkedAccount(context.Background(), "910000000000000010", PlatformDiscord)
		if !errors.Is(err, account.ErrWouldLockAccount) {
			t.Fatalf("expected ErrWouldLockAccount, got: %v", err)
		}
		if _, err := las.GetLinkedAccountByUserID(context.Background(), "910000000000000010", PlatformDiscord); err != nil {
			t.Errorf("expected the blocked link to still exist, got: %v", err)
		}
	})
}

func TestLS55DeleteLinkedAccountAllowedWithPassword(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("LS-55_AllowedWithPassword", func(t *testing.T) {
		a := stSeedPasswordAccount(t, as, "910000000000000011")
		stSeedLink(t, las, a.UserID, PlatformDiscord, "sttest-discord-11", true, true)

		if err := las.DeleteLinkedAccount(context.Background(), a.UserID, PlatformDiscord); err != nil {
			t.Fatalf("expected the delete to succeed, got: %v", err)
		}
		if _, err := las.GetLinkedAccountByUserID(context.Background(), a.UserID, PlatformDiscord); !errors.Is(err, account.ErrNotFound) {
			t.Errorf("expected the link to be gone, got: %v", err)
		}
	})
}

func TestLS56DeleteLinkedAccountAllowedWithAnotherEnabledLink(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("LS-56_AllowedWithAnotherEnabledLink", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000012")
		stSeedLink(t, las, "910000000000000012", PlatformDiscord, "sttest-discord-12", true, true)
		stSeedLink(t, las, "910000000000000012", PlatformTwitch, "sttest-twitch-12", true, true)

		if err := las.DeleteLinkedAccount(context.Background(), "910000000000000012", PlatformDiscord); err != nil {
			t.Fatalf("expected the delete to succeed, got: %v", err)
		}
		if _, err := las.GetLinkedAccountByUserID(context.Background(), "910000000000000012", PlatformTwitch); err != nil {
			t.Errorf("expected the Twitch link to remain untouched, got: %v", err)
		}
	})
}

func TestLS57DeleteLinkedAccountNotFound(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("LS-57_NotFound", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000013")
		err := las.DeleteLinkedAccount(context.Background(), "910000000000000013", PlatformDiscord)
		if !errors.Is(err, account.ErrNotFound) {
			t.Errorf("expected ErrNotFound for a platform that was never linked, got: %v", err)
		}
	})
}

func TestLS58DeleteLinkedAccountIgnoresDisabledOtherLink(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("LS-58_IgnoresDisabledOtherLink", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000014")
		stSeedLink(t, las, "910000000000000014", PlatformDiscord, "sttest-discord-14", true, true)
		stSeedLink(t, las, "910000000000000014", PlatformTwitch, "sttest-twitch-14", true, false)

		err := las.DeleteLinkedAccount(context.Background(), "910000000000000014", PlatformDiscord)
		if !errors.Is(err, account.ErrWouldLockAccount) {
			t.Fatalf("expected ErrWouldLockAccount (the other link is disabled, so it doesn't count), got: %v", err)
		}
	})
}

func TestLS59DeleteLinkedAccountBlockedWhenPasswordAuthDisabled(t *testing.T) {
	as, las, ass := stLiveStore(t)
	t.Run("LS-59_BlockedWhenPasswordAuthDisabled", func(t *testing.T) {
		a := stSeedPasswordAccount(t, as, "910000000000000015")
		stSeedLink(t, las, a.UserID, PlatformDiscord, "sttest-discord-15", true, true)
		if err := ass.SetPasswordAuthEnabled(context.Background(), a.UserID, false); err != nil {
			t.Fatalf("expected disabling password auth to succeed with a linked account present, got: %v", err)
		}

		err := las.DeleteLinkedAccount(context.Background(), a.UserID, PlatformDiscord)
		if !errors.Is(err, account.ErrWouldLockAccount) {
			t.Fatalf("expected ErrWouldLockAccount once password auth is disabled and this is the only link, got: %v", err)
		}
	})
}

func TestLS60DeleteLinkedAccountConcurrentDifferentPlatformsNeverBothSucceed(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("LS-60_ConcurrentDifferentPlatformsNeverBothSucceed", func(t *testing.T) {
		const trials = 50
		for i := 0; i < trials; i++ {
			userID := fmt.Sprintf("91000000000001%04d", i)
			stSeedBareAccount(t, as, userID)
			stSeedLink(t, las, userID, PlatformDiscord, "sttest-discord-"+userID, true, true)
			stSeedLink(t, las, userID, PlatformTwitch, "sttest-twitch-"+userID, true, true)

			var wg sync.WaitGroup
			results := make([]error, 2)
			wg.Add(2)
			go func() {
				defer wg.Done()
				results[0] = las.DeleteLinkedAccount(context.Background(), userID, PlatformDiscord)
			}()
			go func() {
				defer wg.Done()
				results[1] = las.DeleteLinkedAccount(context.Background(), userID, PlatformTwitch)
			}()
			wg.Wait()

			successes := 0
			for _, err := range results {
				switch {
				case err == nil:
					successes++
				case errors.Is(err, account.ErrWouldLockAccount):
				default:
					t.Fatalf("trial %d: unexpected error: %v", i, err)
				}
			}
			if successes != 1 {
				t.Fatalf("trial %d: expected exactly 1 of 2 concurrent deletes to succeed, got %d (results: %v)", i, successes, results)
			}

			remaining, err := las.GetLinkedAccountsByUserID(context.Background(), userID)
			if err != nil {
				t.Fatalf("trial %d: GetLinkedAccountsByUserID returned error: %v", i, err)
			}
			if len(remaining) != 1 {
				t.Fatalf("trial %d: expected exactly 1 link to remain (never 0), got %d", i, len(remaining))
			}
		}
	})
}

func TestLS61SetLinkedAccountLoginEnabledEnable(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("LS-61_EnableOnVerifiedRow", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000016")
		stSeedLink(t, las, "910000000000000016", PlatformDiscord, "sttest-discord-16", true, false)

		if err := las.SetLinkedAccountLoginEnabled(context.Background(), "910000000000000016", PlatformDiscord, true); err != nil {
			t.Fatalf("expected the enable to succeed, got: %v", err)
		}
		la, err := las.GetLinkedAccountByUserID(context.Background(), "910000000000000016", PlatformDiscord)
		if err != nil {
			t.Fatalf("GetLinkedAccountByUserID returned error: %v", err)
		}
		if !la.LoginEnabled {
			t.Error("expected LoginEnabled to be true")
		}
	})
}

func TestLS62SetLinkedAccountLoginEnabledCannotEnableUnverified(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("LS-62_CannotEnableUnverified", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000017")
		stSeedLink(t, las, "910000000000000017", PlatformDiscord, "sttest-discord-17", false, false)

		err := las.SetLinkedAccountLoginEnabled(context.Background(), "910000000000000017", PlatformDiscord, true)
		if !errors.Is(err, ErrLinkedAccountUnverified) {
			t.Fatalf("expected ErrLinkedAccountUnverified, got: %v", err)
		}
	})
}

func TestLS63SetLinkedAccountLoginEnabledBlockedAsLastLoginMethod(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("LS-63_BlockedAsLastLoginMethod", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000018")
		stSeedLink(t, las, "910000000000000018", PlatformDiscord, "sttest-discord-18", true, true)

		err := las.SetLinkedAccountLoginEnabled(context.Background(), "910000000000000018", PlatformDiscord, false)
		if !errors.Is(err, account.ErrWouldLockAccount) {
			t.Fatalf("expected ErrWouldLockAccount, got: %v", err)
		}
		la, err := las.GetLinkedAccountByUserID(context.Background(), "910000000000000018", PlatformDiscord)
		if err != nil {
			t.Fatalf("GetLinkedAccountByUserID returned error: %v", err)
		}
		if !la.LoginEnabled {
			t.Error("expected LoginEnabled to remain true after a blocked disable")
		}
	})
}

func TestLS64SetLinkedAccountLoginEnabledAllowedWithAnotherEnabledLink(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("LS-64_AllowedWithAnotherEnabledLink", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000019")
		stSeedLink(t, las, "910000000000000019", PlatformDiscord, "sttest-discord-19", true, true)
		stSeedLink(t, las, "910000000000000019", PlatformTwitch, "sttest-twitch-19", true, true)

		if err := las.SetLinkedAccountLoginEnabled(context.Background(), "910000000000000019", PlatformDiscord, false); err != nil {
			t.Fatalf("expected the disable to succeed, got: %v", err)
		}
		la, err := las.GetLinkedAccountByUserID(context.Background(), "910000000000000019", PlatformDiscord)
		if err != nil {
			t.Fatalf("GetLinkedAccountByUserID returned error: %v", err)
		}
		if la.LoginEnabled {
			t.Error("expected LoginEnabled to be false")
		}
	})
}

func TestLS65SetLinkedAccountLoginEnabledBlockedWhenPasswordAuthDisabled(t *testing.T) {
	as, las, ass := stLiveStore(t)
	t.Run("LS-65_BlockedWhenPasswordAuthDisabled", func(t *testing.T) {
		a := stSeedPasswordAccount(t, as, "910000000000000020")
		stSeedLink(t, las, a.UserID, PlatformDiscord, "sttest-discord-20", true, true)
		if err := ass.SetPasswordAuthEnabled(context.Background(), a.UserID, false); err != nil {
			t.Fatalf("expected disabling password auth to succeed, got: %v", err)
		}

		err := las.SetLinkedAccountLoginEnabled(context.Background(), a.UserID, PlatformDiscord, false)
		if !errors.Is(err, account.ErrWouldLockAccount) {
			t.Fatalf("expected ErrWouldLockAccount once password auth is disabled and this is the only enabled link, got: %v", err)
		}
	})
}

func TestLS66SetLinkedAccountLoginEnabledConcurrentDifferentPlatformsNeverBothSucceed(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("LS-66_ConcurrentDifferentPlatformsNeverBothSucceed", func(t *testing.T) {
		const trials = 50
		for i := 0; i < trials; i++ {
			userID := fmt.Sprintf("91000000000002%04d", i)
			stSeedBareAccount(t, as, userID)
			stSeedLink(t, las, userID, PlatformDiscord, "sttest-discord-"+userID, true, true)
			stSeedLink(t, las, userID, PlatformTwitch, "sttest-twitch-"+userID, true, true)

			var wg sync.WaitGroup
			results := make([]error, 2)
			wg.Add(2)
			go func() {
				defer wg.Done()
				results[0] = las.SetLinkedAccountLoginEnabled(context.Background(), userID, PlatformDiscord, false)
			}()
			go func() {
				defer wg.Done()
				results[1] = las.SetLinkedAccountLoginEnabled(context.Background(), userID, PlatformTwitch, false)
			}()
			wg.Wait()

			successes := 0
			for _, err := range results {
				switch {
				case err == nil:
					successes++
				case errors.Is(err, account.ErrWouldLockAccount):
				default:
					t.Fatalf("trial %d: unexpected error: %v", i, err)
				}
			}
			if successes != 1 {
				t.Fatalf("trial %d: expected exactly 1 of 2 concurrent disables to succeed, got %d (results: %v)", i, successes, results)
			}

			links, err := las.GetLinkedAccountsByUserID(context.Background(), userID)
			if err != nil {
				t.Fatalf("trial %d: GetLinkedAccountsByUserID returned error: %v", i, err)
			}
			enabledCount := 0
			for _, la := range links {
				if la.LoginEnabled {
					enabledCount++
				}
			}
			if enabledCount != 1 {
				t.Fatalf("trial %d: expected exactly 1 login-enabled link to remain (never 0), got %d", i, enabledCount)
			}
		}
	})
}

func TestLS74GetLinkedAccountsByUserID(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("LS-74_ReturnsAllLinkedPlatforms", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000026")
		stSeedLink(t, las, "910000000000000026", PlatformDiscord, "sttest-discord-26", true, true)
		stSeedLink(t, las, "910000000000000026", PlatformTwitch, "sttest-twitch-26", true, true)

		links, err := las.GetLinkedAccountsByUserID(context.Background(), "910000000000000026")
		if err != nil {
			t.Fatalf("GetLinkedAccountsByUserID returned error: %v", err)
		}
		if len(links) != 2 {
			t.Fatalf("expected 2 linked accounts, got %d: %+v", len(links), links)
		}
	})
}

func TestLS75AddLinkedAccountToDBDuplicatePlatformID(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("LS-75_DuplicatePlatformIDTranslatesToSentinel", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000027")
		stSeedBareAccount(t, as, "910000000000000028")
		stSeedLink(t, las, "910000000000000027", PlatformDiscord, "sttest-dupplatform", true, true)

		err := las.AddLinkedAccountToDB(context.Background(), &LinkedAccount{
			UserID:           "910000000000000028",
			Platform:         PlatformDiscord,
			PlatformUsername: "sttest",
			PlatformID:       "sttest-dupplatform",
			Data:             map[string]string{},
			Verified:         true,
			LoginEnabled:     true,
		})
		if !errors.Is(err, ErrAlreadyLinked) {
			t.Errorf("expected ErrAlreadyLinked, got %v", err)
		}
	})
}

func TestLS79AddLinkedAccountToDBSecondAccountOfPlatform(t *testing.T) {
	as, las, _ := stLiveStore(t)
	t.Run("LS-79_AddSecondAccountOfPlatformUserLinked", func(t *testing.T) {
		stSeedBareAccount(t, as, "910000000000000029")
		stSeedLink(t, las, "910000000000000029", PlatformDiscord, "sttest-second-first", true, true)

		err := las.AddLinkedAccountToDB(context.Background(), &LinkedAccount{
			UserID:           "910000000000000029",
			Platform:         PlatformDiscord,
			PlatformUsername: "sttest",
			PlatformID:       "sttest-second-other",
			Data:             map[string]string{},
			Verified:         true,
			LoginEnabled:     true,
		})
		if !errors.Is(err, ErrUserAlreadyLinkedToPlatform) {
			t.Errorf("expected ErrUserAlreadyLinkedToPlatform, got %v", err)
		}
	})
}

func TestLS77to78_NotFoundSentinels(t *testing.T) {
	s := stLiveFullStore(t)

	t.Run("LS-77_GetLinkedAccountByPlatformNameNotFound", func(t *testing.T) {
		got, err := s.LinkAccount().GetLinkedAccountByPlatformName(context.Background(), PlatformDiscord, "sttest-missing-name")
		if got != nil || !errors.Is(err, account.ErrNotFound) {
			t.Errorf("GetLinkedAccountByPlatformName() = (%v, %v), want (nil, %v)", got, err, account.ErrNotFound)
		}
	})

	t.Run("LS-78_GetLinkedAccountByPlatformNameDuplicate", func(t *testing.T) {
		stSeedBareAccount(t, s.Account(), "910000000000000102")
		stSeedBareAccount(t, s.Account(), "910000000000000103")
		stSeedLink(t, s.LinkAccount(), "910000000000000102", PlatformDiscord, "sttest-dup-102", true, true)
		stSeedLink(t, s.LinkAccount(), "910000000000000103", PlatformDiscord, "sttest-dup-103", true, true)

		got, err := s.LinkAccount().GetLinkedAccountByPlatformName(context.Background(), PlatformDiscord, "sttest")
		if got != nil || !errors.Is(err, ErrDuplicateLinkedAccount) {
			t.Errorf("GetLinkedAccountByPlatformName() = (%v, %v), want (nil, %v)", got, err, ErrDuplicateLinkedAccount)
		}
	})
}

func TestST69SetPasswordAuthEnabledEnableAllowedWithPassword(t *testing.T) {
	as, las, ass := stLiveStore(t)
	t.Run("LKS-69_EnableAllowedWithPassword", func(t *testing.T) {
		a := stSeedPasswordAccount(t, as, "910000000000000023")
		stSeedLink(t, las, a.UserID, PlatformDiscord, "sttest-discord-23", true, true)

		if err := ass.SetPasswordAuthEnabled(context.Background(), a.UserID, false); err != nil {
			t.Fatalf("expected the disable to succeed, got: %v", err)
		}
		if err := ass.SetPasswordAuthEnabled(context.Background(), a.UserID, true); err != nil {
			t.Fatalf("expected the re-enable to succeed, got: %v", err)
		}
		settings, err := ass.GetAccountSettings(context.Background(), a.UserID)
		if err != nil {
			t.Fatalf("GetAccountSettings returned error: %v", err)
		}
		if !settings.PasswordAuthEnabled {
			t.Error("expected password auth to be re-enabled")
		}
	})
}

func TestST71SetPasswordAuthEnabledDisableAllowedWithLinkedAccount(t *testing.T) {
	as, las, ass := stLiveStore(t)
	t.Run("LKS-71_DisableAllowedWithLinkedAccount", func(t *testing.T) {
		a := stSeedPasswordAccount(t, as, "910000000000000025")
		stSeedLink(t, las, a.UserID, PlatformDiscord, "sttest-discord-25", true, true)

		if err := ass.SetPasswordAuthEnabled(context.Background(), a.UserID, false); err != nil {
			t.Fatalf("expected the disable to succeed, got: %v", err)
		}
		settings, err := ass.GetAccountSettings(context.Background(), a.UserID)
		if err != nil {
			t.Fatalf("GetAccountSettings returned error: %v", err)
		}
		if settings.PasswordAuthEnabled {
			t.Error("expected password auth to be disabled")
		}
	})
}

func TestST72SetPasswordAuthEnabledConcurrentWithDeleteLinkedAccountNeverBothSucceed(t *testing.T) {
	as, las, ass := stLiveStore(t)
	t.Run("LKS-72_ConcurrentWithDeleteLinkedAccountNeverBothSucceed", func(t *testing.T) {
		const trials = 50
		for i := 0; i < trials; i++ {
			userID := fmt.Sprintf("91000000000003%04d", i)
			a := stSeedPasswordAccount(t, as, userID)
			stSeedLink(t, las, userID, PlatformDiscord, "sttest-discord-"+userID, true, true)

			var wg sync.WaitGroup
			results := make([]error, 2)
			wg.Add(2)
			go func() {
				defer wg.Done()
				results[0] = ass.SetPasswordAuthEnabled(context.Background(), a.UserID, false)
			}()
			go func() {
				defer wg.Done()
				results[1] = las.DeleteLinkedAccount(context.Background(), a.UserID, PlatformDiscord)
			}()
			wg.Wait()

			successes := 0
			for _, err := range results {
				switch {
				case err == nil:
					successes++
				case errors.Is(err, account.ErrWouldLockAccount):
				default:
					t.Fatalf("trial %d: unexpected error: %v", i, err)
				}
			}
			if successes != 1 {
				t.Fatalf("trial %d: expected exactly 1 of 2 concurrent operations to succeed, got %d (results: %v)", i, successes, results)
			}

			settings, err := ass.GetAccountSettings(context.Background(), userID)
			if err != nil {
				t.Fatalf("trial %d: GetAccountSettings returned error: %v", i, err)
			}
			links, err := las.GetLinkedAccountsByUserID(context.Background(), userID)
			if err != nil {
				t.Fatalf("trial %d: GetLinkedAccountsByUserID returned error: %v", i, err)
			}
			if !settings.PasswordAuthEnabled && len(links) == 0 {
				t.Fatalf("trial %d: account left with NO usable login method (password disabled and link gone)", i)
			}
		}
	})
}

func TestST73SetPasswordAuthEnabledConcurrentWithSetLinkedAccountLoginEnabledNeverBothSucceed(t *testing.T) {
	as, las, ass := stLiveStore(t)
	t.Run("LKS-73_ConcurrentWithSetLinkedAccountLoginEnabledNeverBothSucceed", func(t *testing.T) {
		const trials = 50
		for i := 0; i < trials; i++ {
			userID := fmt.Sprintf("91000000000004%04d", i)
			a := stSeedPasswordAccount(t, as, userID)
			stSeedLink(t, las, userID, PlatformDiscord, "sttest-discord-"+userID, true, true)

			var wg sync.WaitGroup
			results := make([]error, 2)
			wg.Add(2)
			go func() {
				defer wg.Done()
				results[0] = ass.SetPasswordAuthEnabled(context.Background(), a.UserID, false)
			}()
			go func() {
				defer wg.Done()
				results[1] = las.SetLinkedAccountLoginEnabled(context.Background(), a.UserID, PlatformDiscord, false)
			}()
			wg.Wait()

			successes := 0
			for _, err := range results {
				switch {
				case err == nil:
					successes++
				case errors.Is(err, account.ErrWouldLockAccount):
				default:
					t.Fatalf("trial %d: unexpected error: %v", i, err)
				}
			}
			if successes != 1 {
				t.Fatalf("trial %d: expected exactly 1 of 2 concurrent operations to succeed, got %d (results: %v)", i, successes, results)
			}

			settings, err := ass.GetAccountSettings(context.Background(), userID)
			if err != nil {
				t.Fatalf("trial %d: GetAccountSettings returned error: %v", i, err)
			}
			la, err := las.GetLinkedAccountByUserID(context.Background(), userID, PlatformDiscord)
			if err != nil {
				t.Fatalf("trial %d: GetLinkedAccountByUserID returned error: %v", i, err)
			}
			if !settings.PasswordAuthEnabled && !la.LoginEnabled {
				t.Fatalf("trial %d: account left with NO usable login method (password disabled and link disabled)", i)
			}
		}
	})
}
