package discord

import (
	"testing"
	"time"

	"github.com/goccy/go-json"
)

func TestWebhookEvent_UnmarshalJSON(t *testing.T) {
	t.Run("WEU-01_ApplicationAuthorized", func(t *testing.T) {
		raw := `{"type":"APPLICATION_AUTHORIZED","timestamp":"2024-01-01T00:00:00.000000","data":{"integration_type":0,"user":{"id":"u1"},"guild":{"id":"g1"}}}`
		var event WebhookEvent
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}
		if event.Type != ApplicationAuthorized {
			t.Errorf("Type = %v, want %v", event.Type, ApplicationAuthorized)
		}
		wantTime, _ := time.ParseInLocation(time.RFC3339Nano, "2024-01-01T00:00:00.000000Z", time.UTC)
		if !event.Timestamp.Equal(wantTime) {
			t.Errorf("Timestamp = %v, want %v", event.Timestamp, wantTime)
		}
		data, ok := event.Data.(ApplicationAuthorizedWebhookData)
		if !ok {
			t.Fatalf("Data type = %T, want ApplicationAuthorizedWebhookData", event.Data)
		}
		if data.IntegrationType == nil || *data.IntegrationType != GuildInstall {
			t.Errorf("IntegrationType = %v, want %v", data.IntegrationType, GuildInstall)
		}
		if data.User.ID != "u1" {
			t.Errorf("User.ID = %q, want %q", data.User.ID, "u1")
		}
		if data.Guild.ID != "g1" {
			t.Errorf("Guild.ID = %q, want %q", data.Guild.ID, "g1")
		}
	})

	t.Run("WEU-02_ApplicationDeauthorized", func(t *testing.T) {
		raw := `{"type":"APPLICATION_DEAUTHORIZED","timestamp":"2024-01-01T00:00:00.000000","data":{"user":{"id":"u2"}}}`
		var event WebhookEvent
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}
		data, ok := event.Data.(ApplicationDeauthorizedWebhookData)
		if !ok {
			t.Fatalf("Data type = %T, want ApplicationDeauthorizedWebhookData", event.Data)
		}
		if data.User.ID != "u2" {
			t.Errorf("User.ID = %q, want %q", data.User.ID, "u2")
		}
	})

	t.Run("WEU-03_UnhandledEventTypeLeavesDataNil", func(t *testing.T) {
		raw := `{"type":"ENTITLEMENT_CREATE","timestamp":"2024-01-01T00:00:00.000000"}`
		var event WebhookEvent
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}
		if event.Data != nil {
			t.Errorf("Data = %#v, want nil", event.Data)
		}
	})

	t.Run("WEU-04_MalformedTopLevelJSON", func(t *testing.T) {
		var event WebhookEvent
		if err := json.Unmarshal([]byte(`{not json`), &event); err == nil {
			t.Fatal("UnmarshalJSON() error = nil, want a decode error")
		}
	})

	t.Run("WEU-05_InnerDataTypeMismatch", func(t *testing.T) {
		raw := `{"type":"APPLICATION_AUTHORIZED","timestamp":"2024-01-01T00:00:00.000000","data":{"integration_type":"zero"}}`
		var event WebhookEvent
		if err := json.Unmarshal([]byte(raw), &event); err == nil {
			t.Fatal("UnmarshalJSON() error = nil, want a decode error")
		}
	})
}
