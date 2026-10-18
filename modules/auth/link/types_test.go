package link

import "testing"

type tyFakePlatformData struct {
	id, email, username, data string
}

func (f tyFakePlatformData) GetID() string { return f.id }

func (f tyFakePlatformData) GetEmail() string { return f.email }

func (f tyFakePlatformData) GetUsername() string { return f.username }

func (f tyFakePlatformData) GetData() string { return f.data }

func (f tyFakePlatformData) CreateLinkedAccount(_ string) *LinkedAccount {
	return nil
}

func TestTY26NewLinkedAccount(t *testing.T) {
	t.Run("LKT-26_BuildsVerifiedLoginEnabledLink", func(t *testing.T) {
		data := tyFakePlatformData{id: "p1", username: "alice"}
		la := NewLinkedAccount("u1", PlatformDiscord, "alice", "p1", data)

		if la.UserID != "u1" || la.Platform != PlatformDiscord || la.PlatformUsername != "alice" || la.PlatformID != "p1" {
			t.Errorf("got %+v, want UserID=u1 Platform=discord PlatformUsername=alice PlatformID=p1", la)
		}
		if la.Data != PlatformData(data) {
			t.Errorf("Data = %v, want %v", la.Data, data)
		}
		if !la.Verified || !la.LoginEnabled {
			t.Errorf("expected a freshly created link to be Verified and LoginEnabled, got Verified=%v LoginEnabled=%v", la.Verified, la.LoginEnabled)
		}
	})
}
