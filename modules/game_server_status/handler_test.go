package gss

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/mcstatus"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/gsspb"
	"github.com/goccy/go-json"
)

type fakeGSSService struct {
	status *GameServerStatus
	err    error

	calledGame      string
	calledHost      string
	calledPort      int
	calledQueryType QueryType
	called          bool
}

func (f *fakeGSSService) QueryGameQ(game string, host string, port int) (*GameQResponse, error) {
	return nil, errors.New("fakeGSSService.QueryGameQ not used by these tests")
}

func (f *fakeGSSService) QueryGameDig(game string, host string, port int) (*GameDigResponse, error) {
	return nil, errors.New("fakeGSSService.QueryGameDig not used by these tests")
}

func (f *fakeGSSService) QueryGameServer(game string, host string, port int, queryType QueryType) (*GameServerStatus, error) {
	f.called = true
	f.calledGame = game
	f.calledHost = host
	f.calledPort = port
	f.calledQueryType = queryType
	return f.status, f.err
}

func newTestStatus(raw interface{}) *GameServerStatus {
	return NewGameServerStatus(
		"1.2.3.4", 25565, "Test Server", "world", 20, 3,
		[]*gsspb.Player{{Name: "alice", Id: "u1"}},
		QueryTypeMinecraft,
		raw,
	)
}

func TestGameServerStatusHandler(t *testing.T) {
	t.Run("HD-01_HappyPath_RawNotRequested", func(t *testing.T) {
		fake := &fakeGSSService{status: newTestStatus("rawdata")}
		req := httptest.NewRequest("GET", "/gss/minecraft/status?host=1.2.3.4&port=25565&query_type=minecraft", nil)
		req.SetPathValue("game", "minecraft")
		rec := httptest.NewRecorder()

		GameServerStatusHandler(fake)(rec, req)

		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if _, ok := body["raw"]; ok {
			t.Errorf(`body contains "raw" key = %v, want absent`, body["raw"])
		}
		if got := body["host"]; got != "1.2.3.4" {
			t.Errorf(`body["host"] = %v, want "1.2.3.4"`, got)
		}
		if !fake.called {
			t.Fatal("expected QueryGameServer to be called")
		}
		if fake.calledGame != "minecraft" || fake.calledHost != "1.2.3.4" || fake.calledPort != 25565 || fake.calledQueryType != QueryTypeMinecraft {
			t.Errorf("QueryGameServer called with (%q, %q, %d, %q), want (\"minecraft\", \"1.2.3.4\", 25565, \"minecraft\")",
				fake.calledGame, fake.calledHost, fake.calledPort, fake.calledQueryType)
		}
	})

	t.Run("HD-02_HappyPath_RawRequested", func(t *testing.T) {
		fake := &fakeGSSService{status: newTestStatus("rawdata")}
		req := httptest.NewRequest("GET", "/gss/minecraft/status?host=1.2.3.4&port=25565&raw=true", nil)
		req.SetPathValue("game", "minecraft")
		rec := httptest.NewRecorder()

		GameServerStatusHandler(fake)(rec, req)

		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if got, ok := body["raw"]; !ok || got != "rawdata" {
			t.Errorf(`body["raw"] = %v (present=%v), want "rawdata"`, got, ok)
		}
	})

	t.Run("HD-03_ErrorPath_MissingHost", func(t *testing.T) {
		fake := &fakeGSSService{status: newTestStatus(nil)}
		req := httptest.NewRequest("GET", "/gss/minecraft/status?port=25565", nil)
		req.SetPathValue("game", "minecraft")
		rec := httptest.NewRecorder()

		GameServerStatusHandler(fake)(rec, req)

		if rec.Code != 400 {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		var body map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if got := body["detail"]; got != msgInvalidHost {
			t.Errorf(`body["detail"] = %v, want %q`, got, msgInvalidHost)
		}
		if fake.called {
			t.Error("QueryGameServer should not have been called")
		}
	})

	t.Run("HD-04_ErrorPath_InvalidPort", func(t *testing.T) {
		fake := &fakeGSSService{status: newTestStatus(nil)}
		req := httptest.NewRequest("GET", "/gss/minecraft/status?host=1.2.3.4&port=abc", nil)
		req.SetPathValue("game", "minecraft")
		rec := httptest.NewRecorder()

		GameServerStatusHandler(fake)(rec, req)

		if rec.Code != 400 {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		var body map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if got := body["detail"]; got != msgInvalidPort {
			t.Errorf(`body["detail"] = %v, want %q`, got, msgInvalidPort)
		}
		if fake.called {
			t.Error("QueryGameServer should not have been called")
		}
	})

	t.Run("HD-05_ErrorPath_ServiceError", func(t *testing.T) {
		fake := &fakeGSSService{err: ErrServerOffline}
		req := httptest.NewRequest("GET", "/gss/minecraft/status?host=1.2.3.4&port=25565", nil)
		req.SetPathValue("game", "minecraft")
		rec := httptest.NewRecorder()

		GameServerStatusHandler(fake)(rec, req)

		if rec.Code != 404 {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
		var body map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if got := body["detail"]; got != msgServerOffline {
			t.Errorf(`body["detail"] = %v, want %q`, got, msgServerOffline)
		}
	})

	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"GameQQuery", ErrGameQQuery, msgGameQQueryFailed},
		{"GameQQueryWithCause", fmt.Errorf("%w: %w", ErrGameQQuery, testerrors.ErrTransportFailed), msgGameQQueryFailed},
		{"GameDigQuery", ErrGameDigQuery, msgGameDigQueryFailed},
		{"ReadBody", ErrReadBody, msgReadBodyFailed},
		{"DecodeBody", ErrDecodeBody, msgDecodeBodyFailed},
		{"NoGameQResponse", ErrNoGameQResponse, msgNoGameQResponse},
		{"GameUnsupported", ErrGameUnsupported, msgGameUnsupported},
		{"JavaStatus", mcstatus.ErrJavaStatus, msgJavaStatusFailed},
		{"BedrockStatus", mcstatus.ErrBedrockStatus, msgBedrockStatusFailed},
		{"Unrecognized", testerrors.ErrBoom, msgQueryFailed},
	} {
		t.Run("HD-11_ErrorPath_"+tc.name, func(t *testing.T) {
			fake := &fakeGSSService{err: tc.err}
			req := httptest.NewRequest("GET", "/gss/minecraft/status?host=1.2.3.4&port=25565", nil)
			req.SetPathValue("game", "minecraft")
			rec := httptest.NewRecorder()

			GameServerStatusHandler(fake)(rec, req)

			var body map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			if rec.Code != 404 || body["detail"] != tc.want {
				t.Errorf("status = %d, detail = %v, want 404 and %q", rec.Code, body["detail"], tc.want)
			}
		})
	}

	t.Run("HD-06_EdgeCase_UnrecognizedQueryTypeParam", func(t *testing.T) {
		fake := &fakeGSSService{status: newTestStatus(nil)}
		req := httptest.NewRequest("GET", "/gss/minecraft/status?host=1.2.3.4&port=25565&query_type=totallybogus", nil)
		req.SetPathValue("game", "minecraft")
		rec := httptest.NewRecorder()

		GameServerStatusHandler(fake)(rec, req)

		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if !fake.called {
			t.Fatal("expected QueryGameServer to be called")
		}
		if fake.calledQueryType != QueryTypeUnknown {
			t.Errorf("QueryGameServer called with queryType = %q, want %q", fake.calledQueryType, QueryTypeUnknown)
		}
	})
}

func TestSimpleGameServerStatus(t *testing.T) {
	t.Run("HD-07_HappyPath_Online", func(t *testing.T) {
		fake := &fakeGSSService{status: newTestStatus(nil)}
		req := httptest.NewRequest("GET", "/gss/minecraft/simple?host=1.2.3.4&port=25565", nil)
		req.SetPathValue("game", "minecraft")
		rec := httptest.NewRecorder()

		SimpleGameServerStatus(fake)(rec, req)

		if rec.Code != 200 {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != "text/plain" {
			t.Errorf("Content-Type = %q, want %q", got, "text/plain")
		}
		if got := rec.Body.String(); got != "Online" {
			t.Errorf("body = %q, want %q", got, "Online")
		}
	})

	t.Run("HD-08_ErrorPath_MissingHost", func(t *testing.T) {
		fake := &fakeGSSService{status: newTestStatus(nil)}
		req := httptest.NewRequest("GET", "/gss/minecraft/simple?port=25565", nil)
		req.SetPathValue("game", "minecraft")
		rec := httptest.NewRecorder()

		SimpleGameServerStatus(fake)(rec, req)

		if rec.Code != 400 {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if fake.called {
			t.Error("QueryGameServer should not have been called")
		}
	})

	t.Run("HD-09_ErrorPath_InvalidPort", func(t *testing.T) {
		fake := &fakeGSSService{status: newTestStatus(nil)}
		req := httptest.NewRequest("GET", "/gss/minecraft/simple?host=1.2.3.4&port=abc", nil)
		req.SetPathValue("game", "minecraft")
		rec := httptest.NewRecorder()

		SimpleGameServerStatus(fake)(rec, req)

		if rec.Code != 400 {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if fake.called {
			t.Error("QueryGameServer should not have been called")
		}
	})

	t.Run("HD-10_ErrorPath_Offline", func(t *testing.T) {
		fake := &fakeGSSService{err: ErrServerOffline}
		req := httptest.NewRequest("GET", "/gss/minecraft/simple?host=1.2.3.4&port=25565", nil)
		req.SetPathValue("game", "minecraft")
		rec := httptest.NewRecorder()

		SimpleGameServerStatus(fake)(rec, req)

		if rec.Code != 404 {
			t.Fatalf("status = %d, want 404", rec.Code)
		}
		if got := rec.Body.String(); got != "Offline" {
			t.Errorf("body = %q, want %q", got, "Offline")
		}
	})
}
