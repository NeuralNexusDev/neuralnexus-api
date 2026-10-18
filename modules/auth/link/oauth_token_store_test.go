package link

import (
	"errors"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

func TestOT43AddOAuthTokenToDB(t *testing.T) {
	s := &oAuthTokenStore{db: testutil.UnreachablePool(t)}
	t.Run("OT-43_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.AddOAuthTokenToDB(&OAuthToken{UserID: "u1", Platform: PlatformDiscord}))
	})
}

func TestOT44GetOAuthTokenByUserID(t *testing.T) {
	s := &oAuthTokenStore{db: testutil.UnreachablePool(t)}
	t.Run("OT-44_ConnectionError", func(t *testing.T) {
		got, err := s.GetOAuthTokenByUserID("u1", PlatformDiscord)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
		stAssertRawConnectionError(t, err)
	})
}

func TestOT45UpdateOAuthToken(t *testing.T) {
	s := &oAuthTokenStore{db: testutil.UnreachablePool(t)}
	t.Run("OT-45_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.UpdateOAuthToken(&OAuthToken{UserID: "u1", Platform: PlatformDiscord}))
	})
}

func TestOT46DeleteOAuthToken(t *testing.T) {
	s := &oAuthTokenStore{db: testutil.UnreachablePool(t)}
	t.Run("OT-46_ConnectionError", func(t *testing.T) {
		stAssertRawConnectionError(t, s.DeleteOAuthToken("u1", PlatformDiscord))
	})
}

func TestOT79GetOAuthTokenByUserIDNotFound(t *testing.T) {
	s := stLiveFullStore(t)

	t.Run("OT-79_GetOAuthTokenByUserIDNotFound", func(t *testing.T) {
		got, err := s.OAuthToken().GetOAuthTokenByUserID("910000000000000104", PlatformDiscord)
		if got != nil || !errors.Is(err, account.ErrNotFound) {
			t.Errorf("GetOAuthTokenByUserID() = (%v, %v), want (nil, %v)", got, err, account.ErrNotFound)
		}
	})
}
