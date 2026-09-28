package mcstatus

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// hdProblem mirrors the JSON shape responses.NewProblem/.SendProblem emits,
// used to decode error response bodies in these tests.
type hdProblem struct {
	Type     string `json:"type"`
	Status   int    `json:"status"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
}

// hdCallArgs* record the arguments each mock method was invoked with, so
// tests can assert on what the handler forwarded without needing a real
// MCStatusService implementation.
type hdJavaCallArgs struct {
	host         string
	port         int
	queryEnabled bool
	queryPort    int
}

type hdServerCallArgs struct {
	host         string
	port         int
	isBedrock    bool
	queryEnabled bool
	queryPort    int
}

// hdMockService is a self-contained fake MCStatusService for handler-layer
// tests: handler.go's three handlers take MCStatusService as a parameter,
// so no network access is needed to exercise them.
type hdMockService struct {
	javaStatus *MCServerStatus
	javaErr    error
	javaCalls  []hdJavaCallArgs

	bedrockStatus *MCServerStatus
	bedrockErr    error

	serverStatus *MCServerStatus
	serverErr    error
	serverCalls  []hdServerCallArgs
}

func (m *hdMockService) GetJavaServerStatus(host string, port int, queryEnabled bool, queryPort int) (*MCServerStatus, error) {
	m.javaCalls = append(m.javaCalls, hdJavaCallArgs{host, port, queryEnabled, queryPort})
	return m.javaStatus, m.javaErr
}

func (m *hdMockService) GetBedrockServerStatus(host string, port int) (*MCServerStatus, error) {
	return m.bedrockStatus, m.bedrockErr
}

func (m *hdMockService) GetServerStatus(host string, port int, isBedrock bool, queryEnabled bool, queryPort int) (*MCServerStatus, error) {
	m.serverCalls = append(m.serverCalls, hdServerCallArgs{host, port, isBedrock, queryEnabled, queryPort})
	return m.serverStatus, m.serverErr
}

// hdRequest builds a request with the given raw query string and "host"
// path value set directly (bypassing routing, since these tests call the
// handler funcs directly rather than through a mux).
func hdRequest(t *testing.T, host string, rawQuery string) *http.Request {
	t.Helper()
	target := "/mcstatus/" + host
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.SetPathValue("host", host)
	return req
}

func hdDecodeJSON(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("failed to decode JSON body %q: %v", body, err)
	}
	return m
}

func hdDecodeProblem(t *testing.T, body []byte) hdProblem {
	t.Helper()
	var p hdProblem
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("failed to decode problem body %q: %v", body, err)
	}
	return p
}

func hdValidStatus(raw interface{}) *MCServerStatus {
	return NewServerStatus("", 0, "Test Server", "Welcome", "world", 20, 5, nil, "1.20.1", "", ServerTypeJava, raw, nil)
}

func TestServerStatusHandler(t *testing.T) {
	t.Run("HD-01_DefaultRawOmitsRawField", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus("raw-marker")}
		req := hdRequest(t, "mc.example.com:25565", "")
		w := httptest.NewRecorder()

		ServerStatusHandler(mock)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		body := hdDecodeJSON(t, w.Body.Bytes())
		if _, ok := body["raw"]; ok {
			t.Fatalf("expected \"raw\" key to be omitted, got %v", body["raw"])
		}
		if body["name"] != "Test Server" {
			t.Fatalf("expected name %q, got %v", "Test Server", body["name"])
		}
	})

	t.Run("HD-02_RawTrueKeepsRawField", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus("raw-marker")}
		req := hdRequest(t, "mc.example.com:25565", "raw=true")
		w := httptest.NewRecorder()

		ServerStatusHandler(mock)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		body := hdDecodeJSON(t, w.Body.Bytes())
		if body["raw"] != "raw-marker" {
			t.Fatalf("expected raw %q, got %v", "raw-marker", body["raw"])
		}
	})

	t.Run("HD-03_ErrorReturnsNotFound", func(t *testing.T) {
		mock := &hdMockService{serverErr: errors.New("server offline")}
		req := hdRequest(t, "mc.example.com:25565", "")
		w := httptest.NewRecorder()

		ServerStatusHandler(mock)(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
		p := hdDecodeProblem(t, w.Body.Bytes())
		if p.Detail != "server offline" {
			t.Fatalf("expected detail %q, got %q", "server offline", p.Detail)
		}
	})

	t.Run("HD-04_NoPortSuffixDefaultsJavaPort", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "example.com", "")
		w := httptest.NewRecorder()

		ServerStatusHandler(mock)(w, req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if got := mock.serverCalls[0].port; got != 25565 {
			t.Fatalf("expected default java port 25565, got %d", got)
		}
	})

	t.Run("HD-05_NoPortSuffixDefaultsBedrockPort", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "example.com", "bedrock=true")
		w := httptest.NewRecorder()

		ServerStatusHandler(mock)(w, req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if got := mock.serverCalls[0].port; got != 19132 {
			t.Fatalf("expected default bedrock port 19132, got %d", got)
		}
		if !mock.serverCalls[0].isBedrock {
			t.Fatalf("expected isBedrock=true to be forwarded")
		}
	})

	t.Run("HD-06_MissingQueryPortDefaultsToPort", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "example.com:25566", "")
		w := httptest.NewRecorder()

		ServerStatusHandler(mock)(w, req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		call := mock.serverCalls[0]
		if call.port != 25566 {
			t.Fatalf("expected parsed port 25566, got %d", call.port)
		}
		if call.queryPort != call.port {
			t.Fatalf("expected queryPort to default to port %d, got %d", call.port, call.queryPort)
		}
	})

	t.Run("HD-07_QueryFlagsForwarded", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "example.com:25566", "bedrock=true&query=true&query_port=9999")
		w := httptest.NewRecorder()

		ServerStatusHandler(mock)(w, req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		call := mock.serverCalls[0]
		if call.host != "example.com:25566" {
			t.Fatalf("expected host forwarded as-is, got %q", call.host)
		}
		if call.port != 25566 {
			t.Fatalf("expected port 25566, got %d", call.port)
		}
		if !call.isBedrock {
			t.Fatalf("expected isBedrock=true")
		}
		if !call.queryEnabled {
			t.Fatalf("expected queryEnabled=true")
		}
		if call.queryPort != 9999 {
			t.Fatalf("expected queryPort 9999, got %d", call.queryPort)
		}
	})
}

func TestIconHandler(t *testing.T) {
	t.Run("HD-08_HappyPathReturnsPNG", func(t *testing.T) {
		icon := image.NewRGBA(image.Rect(0, 0, 1, 1))
		status := NewServerStatus("", 0, "Test", "MOTD", "", 20, 1, nil, "1.20.1", "", ServerTypeJava, nil, icon)
		mock := &hdMockService{javaStatus: status}
		req := hdRequest(t, "example.com:25565", "")
		w := httptest.NewRecorder()

		IconHandler(mock)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/png" {
			t.Fatalf("expected Content-Type image/png, got %q", ct)
		}
		decoded, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
		if err != nil {
			t.Fatalf("expected a valid PNG body, decode failed: %v", err)
		}
		if decoded.Bounds() != icon.Bounds() {
			t.Fatalf("expected decoded bounds %v, got %v", icon.Bounds(), decoded.Bounds())
		}
	})

	t.Run("HD-09_ErrorReturnsNotFound", func(t *testing.T) {
		mock := &hdMockService{javaErr: errors.New("java offline")}
		req := hdRequest(t, "example.com:25565", "")
		w := httptest.NewRecorder()

		IconHandler(mock)(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
		p := hdDecodeProblem(t, w.Body.Bytes())
		if p.Detail != "java offline" {
			t.Fatalf("expected detail %q, got %q", "java offline", p.Detail)
		}
		if len(mock.javaCalls) != 1 {
			t.Fatalf("expected exactly 1 call, got %d", len(mock.javaCalls))
		}
	})

	t.Run("HD-10_NoPortSuffixDefaultsPort", func(t *testing.T) {
		icon := image.NewRGBA(image.Rect(0, 0, 1, 1))
		status := NewServerStatus("", 0, "Test", "MOTD", "", 20, 1, nil, "1.20.1", "", ServerTypeJava, nil, icon)
		mock := &hdMockService{javaStatus: status}
		req := hdRequest(t, "example.com", "")
		w := httptest.NewRecorder()

		IconHandler(mock)(w, req)

		if len(mock.javaCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.javaCalls))
		}
		if got := mock.javaCalls[0].port; got != 25565 {
			t.Fatalf("expected default port 25565, got %d", got)
		}
	})

	// httptest's ResponseRecorder ignores WriteHeader calls after the first,
	// so w.Code reliably reflects the first write; the recorded body may
	// contain bytes from a second write if the handler writes more than
	// once, so this only asserts the first-write invariants, not full body
	// equality.
	t.Run("HD-11_BedrockFallsThroughAfterBadRequest", func(t *testing.T) {
		mock := &hdMockService{javaErr: errors.New("java offline")}
		req := hdRequest(t, "example.com:19132", "bedrock=true")
		w := httptest.NewRecorder()

		IconHandler(mock)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
		if !bytes.Contains(w.Body.Bytes(), []byte("Bedrock servers do not have icons.")) {
			t.Fatalf("expected the BadRequest message to be present in the body, got %q", w.Body.String())
		}
	})
}

func TestSimpleStatusHandler(t *testing.T) {
	t.Run("HD-12_HappyPathOnline", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "example.com:25565", "")
		w := httptest.NewRecorder()

		SimpleStatusHandler(mock)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "text/plain" {
			t.Fatalf("expected Content-Type text/plain, got %q", ct)
		}
		body, err := io.ReadAll(w.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		if string(body) != "Online" {
			t.Fatalf("expected body %q, got %q", "Online", body)
		}
	})

	t.Run("HD-13_ErrorReturnsOffline", func(t *testing.T) {
		mock := &hdMockService{serverErr: errors.New("down")}
		req := hdRequest(t, "example.com:25565", "")
		w := httptest.NewRecorder()

		SimpleStatusHandler(mock)(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
		if w.Body.String() != "Offline" {
			t.Fatalf("expected body %q, got %q", "Offline", w.Body.String())
		}
	})

	t.Run("HD-14_NoPortSuffixDefaultsBedrockPort", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "example.com", "bedrock=true")
		w := httptest.NewRecorder()

		SimpleStatusHandler(mock)(w, req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if got := mock.serverCalls[0].port; got != 19132 {
			t.Fatalf("expected default bedrock port 19132, got %d", got)
		}
	})

	t.Run("HD-15_QueryFlagsForwarded", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "example.com:25566", "bedrock=true&query=true&query_port=1234")
		w := httptest.NewRecorder()

		SimpleStatusHandler(mock)(w, req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		call := mock.serverCalls[0]
		if !call.isBedrock || !call.queryEnabled || call.queryPort != 1234 || call.port != 25566 {
			t.Fatalf("expected flags forwarded (bedrock=true, query=true, queryPort=1234, port=25566), got %+v", call)
		}
	})
}
