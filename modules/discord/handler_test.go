package discord

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/bwmarrin/discordgo"
	"github.com/goccy/go-json"
)

// dcRequest builds a POST request with the given body and Content-Type.
// mw.LogRequest, called on most branches under test, does an unchecked type
// assertion on ctx.Value(mw.RemoteAddrKey).(string) and
// ctx.Value(mw.RequestIDKey).(int); a request missing either would panic
// there instead of in the code path under test, so every request carries both.
func dcRequest(t *testing.T, body string, contentType string) *http.Request {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	if contentType != "" {
		r.Header.Set(ContentType, contentType)
	}
	ctx := context.WithValue(r.Context(), mw.RemoteAddrKey, "127.0.0.1")
	ctx = context.WithValue(ctx, mw.RequestIDKey, 1)
	return r.WithContext(ctx)
}

func dcRequireDetail(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var p responses.Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatalf("failed to decode problem body %q: %v", w.Body.String(), err)
	}
	if p.Detail != want {
		t.Fatalf("detail = %q, want %q", p.Detail, want)
	}
}

func dcRequireStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, want, w.Body.String())
	}
}

func TestHandleDiscordWebhook(t *testing.T) {
	t.Run("HW-01_WrongContentType", func(t *testing.T) {
		h := HandleDiscordWebhook()
		w := httptest.NewRecorder()
		h(w, dcRequest(t, `{}`, ""))
		dcRequireStatus(t, w, http.StatusUnsupportedMediaType)
		dcRequireDetail(t, w, msgRequestMustBeJSON)
	})

	t.Run("HW-02_InvalidJSONBody", func(t *testing.T) {
		h := HandleDiscordWebhook()
		w := httptest.NewRecorder()
		h(w, dcRequest(t, `{not json`, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusBadRequest)
		dcRequireDetail(t, w, msgInvalidRequestBody)
	})

	t.Run("HW-03_VersionMismatchStillProcessed", func(t *testing.T) {
		h := HandleDiscordWebhook()
		w := httptest.NewRecorder()
		h(w, dcRequest(t, `{"version":2,"type":0}`, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusNoContent)
	})

	t.Run("HW-04_Ping", func(t *testing.T) {
		h := HandleDiscordWebhook()
		w := httptest.NewRecorder()
		h(w, dcRequest(t, `{"version":1,"type":0}`, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusNoContent)
	})

	t.Run("HW-05_AuthorizedGuildInstall", func(t *testing.T) {
		h := HandleDiscordWebhook()
		w := httptest.NewRecorder()
		body := `{"version":1,"type":1,"event":{"type":"APPLICATION_AUTHORIZED","timestamp":"2024-01-01T00:00:00.000000","data":{"integration_type":0,"guild":{"id":"g1"}}}}`
		h(w, dcRequest(t, body, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusNoContent)
	})

	t.Run("HW-06_AuthorizedUserInstall", func(t *testing.T) {
		h := HandleDiscordWebhook()
		w := httptest.NewRecorder()
		body := `{"version":1,"type":1,"event":{"type":"APPLICATION_AUTHORIZED","timestamp":"2024-01-01T00:00:00.000000","data":{"integration_type":1,"user":{"id":"u1"}}}}`
		h(w, dcRequest(t, body, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusNoContent)
	})

	t.Run("HW-07_AuthorizedMissingIntegrationType", func(t *testing.T) {
		h := HandleDiscordWebhook()
		w := httptest.NewRecorder()
		body := `{"version":1,"type":1,"event":{"type":"APPLICATION_AUTHORIZED","timestamp":"2024-01-01T00:00:00.000000","data":{"guild":{"id":"g1"}}}}`
		h(w, dcRequest(t, body, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusBadRequest)
		dcRequireDetail(t, w, msgInvalidEventData)
	})

	t.Run("HW-08_AuthorizedOutOfRangeIntegrationType", func(t *testing.T) {
		h := HandleDiscordWebhook()
		w := httptest.NewRecorder()
		body := `{"version":1,"type":1,"event":{"type":"APPLICATION_AUTHORIZED","timestamp":"2024-01-01T00:00:00.000000","data":{"integration_type":2}}}`
		h(w, dcRequest(t, body, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusBadRequest)
		dcRequireDetail(t, w, msgInvalidEventData)
	})

	t.Run("HW-09_Deauthorized", func(t *testing.T) {
		h := HandleDiscordWebhook()
		w := httptest.NewRecorder()
		body := `{"version":1,"type":1,"event":{"type":"APPLICATION_DEAUTHORIZED","timestamp":"2024-01-01T00:00:00.000000","data":{"user":{"id":"u2"}}}}`
		h(w, dcRequest(t, body, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusNoContent)
	})

	t.Run("HW-10_GroupedUnhandledEventType", func(t *testing.T) {
		h := HandleDiscordWebhook()
		w := httptest.NewRecorder()
		body := `{"version":1,"type":1,"event":{"type":"ENTITLEMENT_CREATE","timestamp":"2024-01-01T00:00:00.000000"}}`
		h(w, dcRequest(t, body, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusNoContent)
	})

	t.Run("HW-11_UnrecognizedEventType", func(t *testing.T) {
		h := HandleDiscordWebhook()
		w := httptest.NewRecorder()
		body := `{"version":1,"type":1,"event":{"type":"SOME_FUTURE_EVENT","timestamp":"2024-01-01T00:00:00.000000"}}`
		h(w, dcRequest(t, body, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusNoContent)
	})

	t.Run("HW-12_NonNilEventZeroType", func(t *testing.T) {
		h := HandleDiscordWebhook()
		w := httptest.NewRecorder()
		body := `{"version":1,"type":1,"event":{"type":"","timestamp":"2024-01-01T00:00:00.000000"}}`
		h(w, dcRequest(t, body, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusNoContent)
	})

	t.Run("HW-13_UnrecognizedWebhookType", func(t *testing.T) {
		h := HandleDiscordWebhook()
		w := httptest.NewRecorder()
		h(w, dcRequest(t, `{"version":1,"type":99}`, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusNoContent)
	})
}

func TestHandleDiscordInteraction(t *testing.T) {
	t.Run("HI-01_WrongContentType", func(t *testing.T) {
		h := HandleDiscordInteraction()
		w := httptest.NewRecorder()
		h(w, dcRequest(t, `{}`, ""))
		dcRequireStatus(t, w, http.StatusUnsupportedMediaType)
		dcRequireDetail(t, w, msgRequestMustBeJSON)
	})

	t.Run("HI-02_InvalidJSONBody", func(t *testing.T) {
		h := HandleDiscordInteraction()
		w := httptest.NewRecorder()
		h(w, dcRequest(t, `{not json`, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusBadRequest)
		dcRequireDetail(t, w, msgInvalidRequestBody)
	})

	t.Run("HI-03_Ping", func(t *testing.T) {
		h := HandleDiscordInteraction()
		w := httptest.NewRecorder()
		h(w, dcRequest(t, `{"type":1}`, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusOK)

		var got struct {
			Type discordgo.InteractionType `json:"type"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("response body did not decode: %v (body: %s)", err, w.Body.String())
		}
		if got.Type != discordgo.InteractionPing {
			t.Errorf("response type = %v, want InteractionPing", got.Type)
		}
	})

	t.Run("HI-04_GroupedUnhandledTypes", func(t *testing.T) {
		cases := []struct {
			name string
			body string
		}{
			{"ApplicationCommand", `{"type":2,"data":{"id":"1","name":"cmd","type":1}}`},
			{"MessageComponent", `{"type":3,"data":{"custom_id":"btn1","component_type":2}}`},
			{"ApplicationCommandAutocomplete", `{"type":4,"data":{"id":"1","name":"cmd","type":1}}`},
			{"ModalSubmit", `{"type":5,"data":{"custom_id":"modal1","components":[]}}`},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				h := HandleDiscordInteraction()
				w := httptest.NewRecorder()
				h(w, dcRequest(t, c.body, ApplicationJSON))
				dcRequireStatus(t, w, http.StatusNoContent)
			})
		}
	})

	t.Run("HI-05_UnrecognizedInteractionType", func(t *testing.T) {
		h := HandleDiscordInteraction()
		w := httptest.NewRecorder()
		h(w, dcRequest(t, `{"type":99}`, ApplicationJSON))
		dcRequireStatus(t, w, http.StatusNoContent)
	})
}
