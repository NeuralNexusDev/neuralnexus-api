package discord

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/goccy/go-json"
)

func dcRequirePanic(t *testing.T, wantSubstr string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic containing %q, got none", wantSubstr)
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, wantSubstr) {
			t.Fatalf("panic = %v, want substring %q", r, wantSubstr)
		}
	}()
	fn()
}

func TestWebhookType_String(t *testing.T) {
	t.Run("WT-01_Ping", func(t *testing.T) {
		if got := WebhookTypePing.String(); got != "Ping" {
			t.Errorf("String() = %q, want %q", got, "Ping")
		}
	})
	t.Run("WT-02_Event", func(t *testing.T) {
		if got := WebhookTypeEvent.String(); got != "Event" {
			t.Errorf("String() = %q, want %q", got, "Event")
		}
	})
	t.Run("WT-03_OutOfRange", func(t *testing.T) {
		if got := WebhookType(99).String(); got != "WebhookType(99)" {
			t.Errorf("String() = %q, want %q", got, "WebhookType(99)")
		}
	})
}

func TestWebhookEventType_String(t *testing.T) {
	t.Run("WET-01_NamedConstants", func(t *testing.T) {
		cases := []struct {
			in   WebhookEventType
			want string
		}{
			{ApplicationAuthorized, "ApplicationAuthorized"},
			{ApplicationDeauthorized, "ApplicationDeauthorized"},
			{EntitlementCreate, "EntitlementCreate"},
			{EntitlementUpdate, "EntitlementUpdate"},
			{EntitlementDelete, "EntitlementDelete"},
			{QuestUserEnrollment, "QuestUserEnrollment"},
			{LobbyMessageCreate, "LobbyMessageCreate"},
			{LobbyMessageUpdate, "LobbyMessageUpdate"},
			{LobbyMessageDelete, "LobbyMessageDelete"},
			{GameDirectMessageCreate, "GameDirectMessageCreate"},
			{GameDirectMessageUpdate, "GameDirectMessageUpdate"},
			{GameDirectMessageDelete, "GameDirectMessageDelete"},
		}
		for _, c := range cases {
			t.Run(string(c.in), func(t *testing.T) {
				if got := c.in.String(); got != c.want {
					t.Errorf("String() = %q, want %q", got, c.want)
				}
			})
		}
	})
	t.Run("WET-02_Unrecognized", func(t *testing.T) {
		got := WebhookEventType("SOMETHING_ELSE").String()
		want := "WebhookEventType(SOMETHING_ELSE)"
		if got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})
}

func TestInstallationContext_String(t *testing.T) {
	t.Run("IC-01_NamedConstants", func(t *testing.T) {
		if got := GuildInstall.String(); got != "GuildInstall" {
			t.Errorf("String() = %q, want %q", got, "GuildInstall")
		}
		if got := UserInstall.String(); got != "UserInstall" {
			t.Errorf("String() = %q, want %q", got, "UserInstall")
		}
	})
	t.Run("IC-02_OutOfRange", func(t *testing.T) {
		got := InstallationContext(5).String()
		want := "InstallationContext(5)"
		if got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	})
}

func TestApplicationAuthorizedWebhookData_Type(t *testing.T) {
	t.Run("AAD-01_ReturnsConstant", func(t *testing.T) {
		d := ApplicationAuthorizedWebhookData{User: discordgo.User{ID: "u1"}}
		if got := d.Type(); got != ApplicationAuthorized {
			t.Errorf("Type() = %v, want %v", got, ApplicationAuthorized)
		}
	})
}

func TestApplicationDeauthorizedWebhookData_Type(t *testing.T) {
	t.Run("ADD-01_ReturnsConstant", func(t *testing.T) {
		d := ApplicationDeauthorizedWebhookData{User: discordgo.User{ID: "u1"}}
		if got := d.Type(); got != ApplicationDeauthorized {
			t.Errorf("Type() = %v, want %v", got, ApplicationDeauthorized)
		}
	})
}

func TestWebhookEvent_UnmarshalJSON(t *testing.T) {
	t.Run("WEU-01_ApplicationAuthorized", func(t *testing.T) {
		raw := `{"type":"APPLICATION_AUTHORIZED","timestamp":"2024-01-01T00:00:00Z","data":{"integration_type":0,"user":{"id":"u1"},"guild":{"id":"g1"}}}`
		var event WebhookEvent
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			t.Fatalf("UnmarshalJSON() error = %v", err)
		}
		if event.Type != ApplicationAuthorized {
			t.Errorf("Type = %v, want %v", event.Type, ApplicationAuthorized)
		}
		wantTime, _ := time.Parse(time.RFC3339, "2024-01-01T00:00:00Z")
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
		raw := `{"type":"APPLICATION_DEAUTHORIZED","timestamp":"2024-01-01T00:00:00Z","data":{"user":{"id":"u2"}}}`
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
		raw := `{"type":"ENTITLEMENT_CREATE","timestamp":"2024-01-01T00:00:00Z"}`
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
		raw := `{"type":"APPLICATION_AUTHORIZED","timestamp":"2024-01-01T00:00:00Z","data":{"integration_type":"zero"}}`
		var event WebhookEvent
		if err := json.Unmarshal([]byte(raw), &event); err == nil {
			t.Fatal("UnmarshalJSON() error = nil, want a decode error")
		}
	})
}

func TestWebhookEvent_ApplicationAuthorizedData(t *testing.T) {
	t.Run("WAD-01_CorrectType", func(t *testing.T) {
		want := ApplicationAuthorizedWebhookData{User: discordgo.User{ID: "u1"}}
		event := &WebhookEvent{Type: ApplicationAuthorized, Data: want}
		if got := event.ApplicationAuthorizedData(); !reflect.DeepEqual(got, want) {
			t.Errorf("ApplicationAuthorizedData() = %+v, want %+v", got, want)
		}
	})

	t.Run("WAD-02_WrongTypePanics", func(t *testing.T) {
		event := &WebhookEvent{Type: ApplicationDeauthorized}
		dcRequirePanic(t, "ApplicationAuthorizedData called on interaction of type ApplicationDeauthorized", func() {
			event.ApplicationAuthorizedData()
		})
	})
}

func TestWebhookEvent_ApplicationDeauthorizedData(t *testing.T) {
	t.Run("WDD-01_CorrectType", func(t *testing.T) {
		want := ApplicationDeauthorizedWebhookData{User: discordgo.User{ID: "u2"}}
		event := &WebhookEvent{Type: ApplicationDeauthorized, Data: want}
		if got := event.ApplicationDeauthorizedData(); got != want {
			t.Errorf("ApplicationDeauthorizedData() = %+v, want %+v", got, want)
		}
	})

	t.Run("WDD-02_WrongTypePanics", func(t *testing.T) {
		event := &WebhookEvent{Type: ApplicationAuthorized}
		dcRequirePanic(t, "ApplicationDeauthorizedData called on interaction of type ApplicationAuthorized", func() {
			event.ApplicationDeauthorizedData()
		})
	})
}
