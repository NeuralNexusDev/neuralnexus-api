package mcstatus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/internal/testerrors"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/dreamscached/minequery/v2"
)

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

func hdDecodeProblem(t *testing.T, body []byte) responses.Problem {
	t.Helper()
	var p responses.Problem
	if err := json.Unmarshal(body, &p); err != nil {
		t.Fatalf("failed to decode problem body %q: %v", body, err)
	}
	return p
}

func hdValidStatus(raw interface{}) *MCServerStatus {
	return NewServerStatus("", 0, "Test Server", "Welcome", "world", 20, 5, nil, "1.20.1", "", ServerTypeJava, raw, nil)
}

var (
	hdBedrockIconColor = color.RGBA{R: 255, A: 255}
	hdDefaultIconColor = color.RGBA{B: 255, A: 255}
	hdLegacyIconColor  = color.RGBA{G: 255, A: 255}
)

func hdUseIconDir(t *testing.T, withIcons bool) {
	t.Helper()
	dir := t.TempDir()
	if withIcons {
		for name, c := range map[string]color.RGBA{bedrockIconFile: hdBedrockIconColor, defaultIconFile: hdDefaultIconColor, legacyIconFile: hdLegacyIconColor} {
			img := image.NewRGBA(image.Rect(0, 0, 1, 1))
			img.Set(0, 0, c)
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				t.Fatalf("encode %s: %v", name, err)
			}
			if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}
	}
	prev := iconDir
	iconDir = dir
	t.Cleanup(func() { iconDir = prev })
}

func hdDecodePixel(t *testing.T, body []byte) color.RGBA {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("expected a valid PNG body, decode failed: %v", err)
	}
	return color.RGBAModel.Convert(img.At(0, 0)).(color.RGBA)
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
		mock := &hdMockService{serverErr: ErrJavaStatus}
		req := hdRequest(t, "mc.example.com:25565", "")
		w := httptest.NewRecorder()

		ServerStatusHandler(mock)(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
		p := hdDecodeProblem(t, w.Body.Bytes())
		if p.Detail != msgJavaStatusFailed {
			t.Fatalf("expected detail %q, got %q", msgJavaStatusFailed, p.Detail)
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
		if got := mock.serverCalls[0].host; got != "example.com" {
			t.Fatalf("expected host example.com, got %q", got)
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
		if got := mock.serverCalls[0].host; got != "example.com" {
			t.Fatalf("expected host example.com, got %q", got)
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
		if call.host != "example.com" {
			t.Fatalf("expected the bare host, got %q", call.host)
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

	for _, tc := range []struct {
		name       string
		err        error
		wantStatus int
		want       string
	}{
		{"JavaStatus", ErrJavaStatus, http.StatusNotFound, msgJavaStatusFailed},
		{"BedrockStatus", ErrBedrockStatus, http.StatusNotFound, msgBedrockStatusFailed},
		{"BedrockStatusWithCause", fmt.Errorf("%w: %w", ErrBedrockStatus, testerrors.ErrTransportFailed), http.StatusNotFound, msgBedrockStatusFailed},
		{"Unrecognized", testerrors.ErrBoom, http.StatusInternalServerError, msgFailedToGetServerStatus},
	} {
		t.Run("HD-16_"+tc.name, func(t *testing.T) {
			mock := &hdMockService{serverErr: tc.err}
			req := hdRequest(t, "mc.example.com:25565", "")
			w := httptest.NewRecorder()

			ServerStatusHandler(mock)(w, req)

			if w.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d", tc.wantStatus, w.Code)
			}
			if p := hdDecodeProblem(t, w.Body.Bytes()); p.Detail != tc.want {
				t.Fatalf("expected detail %q, got %q", tc.want, p.Detail)
			}
		})
	}

	for _, tc := range []struct {
		name  string
		value string
	}{
		{"Zero", "0"},
		{"Negative", "-5"},
		{"TooLarge", "65536"},
		{"NotANumber", "abc"},
	} {
		t.Run("HD-18_ServerStatus_InvalidQueryPort_"+tc.name, func(t *testing.T) {
			mock := &hdMockService{serverStatus: hdValidStatus(nil)}
			req := hdRequest(t, "mc.example.com:25570", "query=true&query_port="+tc.value)

			ServerStatusHandler(mock)(httptest.NewRecorder(), req)

			if len(mock.serverCalls) != 1 {
				t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
			}
			if got := mock.serverCalls[0].queryPort; got != 25570 {
				t.Fatalf("queryPort = %d, want the server port 25570", got)
			}
		})
	}

	for _, bound := range []string{"1", "65535"} {
		t.Run("HD-18_ServerStatus_BoundaryAccepted_"+bound, func(t *testing.T) {
			mock := &hdMockService{serverStatus: hdValidStatus(nil)}
			req := hdRequest(t, "mc.example.com:25570", "query=true&query_port="+bound)

			ServerStatusHandler(mock)(httptest.NewRecorder(), req)

			want, _ := strconv.Atoi(bound)
			if len(mock.serverCalls) != 1 || mock.serverCalls[0].queryPort != want {
				t.Fatalf("calls = %+v, want one call with queryPort %d", mock.serverCalls, want)
			}
		})
	}

	t.Run("HD-21_HostPortSplitJava", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "mc.example.com:25570", "")

		ServerStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "mc.example.com" || call.port != 25570 || call.isBedrock {
			t.Fatalf("call = %+v, want host mc.example.com, port 25570, java", call)
		}
	})

	t.Run("HD-22_HostPortSplitBedrock", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "mc.example.com:19140", "bedrock=true")

		ServerStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "mc.example.com" || call.port != 19140 || !call.isBedrock {
			t.Fatalf("call = %+v, want host mc.example.com, port 19140, bedrock", call)
		}
	})

	t.Run("HD-27_NonNumericSuffixKeptJava", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "mc.example.com:abc", "")

		ServerStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "mc.example.com:abc" || call.port != 25565 || call.isBedrock {
			t.Fatalf("call = %+v, want host mc.example.com:abc, port 25565, bedrock false", call)
		}
	})

	t.Run("HD-28_NonNumericSuffixKeptBedrock", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "mc.example.com:abc", "bedrock=true")

		ServerStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "mc.example.com:abc" || call.port != 19132 || !call.isBedrock {
			t.Fatalf("call = %+v, want host mc.example.com:abc, port 19132, bedrock true", call)
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
		if len(mock.javaCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.javaCalls))
		}
		if call := mock.javaCalls[0]; call.queryEnabled || call.queryPort != 0 {
			t.Fatalf("call = %+v, want queryEnabled false and queryPort 0", call)
		}
	})

	t.Run("HD-09_ErrorReturnsNotFound", func(t *testing.T) {
		mock := &hdMockService{javaErr: ErrJavaStatus}
		req := hdRequest(t, "example.com:25565", "")
		w := httptest.NewRecorder()

		IconHandler(mock)(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
		p := hdDecodeProblem(t, w.Body.Bytes())
		if p.Detail != msgJavaStatusFailed {
			t.Fatalf("expected detail %q, got %q", msgJavaStatusFailed, p.Detail)
		}
		if len(mock.javaCalls) != 1 {
			t.Fatalf("expected exactly 1 call, got %d", len(mock.javaCalls))
		}
	})

	t.Run("HD-17_UnrecognizedErrorReturnsInternalServerError", func(t *testing.T) {
		mock := &hdMockService{javaErr: testerrors.ErrBoom}
		req := hdRequest(t, "example.com:25565", "")
		w := httptest.NewRecorder()

		IconHandler(mock)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", w.Code)
		}
		if p := hdDecodeProblem(t, w.Body.Bytes()); p.Detail != msgFailedToGetServerStatus {
			t.Fatalf("expected detail %q, got %q", msgFailedToGetServerStatus, p.Detail)
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
		if got := mock.javaCalls[0].host; got != "example.com" {
			t.Fatalf("expected host example.com, got %q", got)
		}
	})

	t.Run("HD-11_BedrockServesBedrockIcon", func(t *testing.T) {
		hdUseIconDir(t, true)
		mock := &hdMockService{javaErr: ErrJavaStatus}
		req := hdRequest(t, "example.com:19132", "bedrock=true")
		w := httptest.NewRecorder()

		IconHandler(mock)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/png" {
			t.Fatalf("expected Content-Type image/png, got %q", ct)
		}
		if got := hdDecodePixel(t, w.Body.Bytes()); got != hdBedrockIconColor {
			t.Fatalf("pixel = %v, want the bedrock icon %v", got, hdBedrockIconColor)
		}
		if len(mock.javaCalls) != 0 {
			t.Fatalf("expected no Java status lookup, got %d", len(mock.javaCalls))
		}
	})

	t.Run("HD-23_HostPortSplit", func(t *testing.T) {
		icon := image.NewRGBA(image.Rect(0, 0, 1, 1))
		status := NewServerStatus("", 0, "Test", "MOTD", "", 20, 1, nil, "1.20.1", "", ServerTypeJava, nil, icon)
		mock := &hdMockService{javaStatus: status}
		req := hdRequest(t, "mc.example.com:25570", "")

		IconHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.javaCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.javaCalls))
		}
		if call := mock.javaCalls[0]; call.host != "mc.example.com" || call.port != 25570 {
			t.Fatalf("call = %+v, want host mc.example.com, port 25570", call)
		}
	})

	t.Run("HD-31_NoIconServesDefaultIcon", func(t *testing.T) {
		hdUseIconDir(t, true)
		mock := &hdMockService{javaStatus: hdValidStatus(nil)}
		req := hdRequest(t, "example.com:25565", "")
		w := httptest.NewRecorder()

		IconHandler(mock)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if got := hdDecodePixel(t, w.Body.Bytes()); got != hdDefaultIconColor {
			t.Fatalf("pixel = %v, want the default icon %v", got, hdDefaultIconColor)
		}
	})

	t.Run("HD-34_LegacyNoIconServesLegacyIcon", func(t *testing.T) {
		hdUseIconDir(t, true)
		mock := &hdMockService{javaStatus: GetPing16Status(&minequery.Status16{})}
		req := hdRequest(t, "example.com:25565", "")
		w := httptest.NewRecorder()

		IconHandler(mock)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if got := hdDecodePixel(t, w.Body.Bytes()); got != hdLegacyIconColor {
			t.Fatalf("pixel = %v, want the legacy icon %v", got, hdLegacyIconColor)
		}
	})

	t.Run("HD-32_MissingDefaultIconReturns500", func(t *testing.T) {
		hdUseIconDir(t, false)
		mock := &hdMockService{javaStatus: hdValidStatus(nil)}
		req := hdRequest(t, "example.com:25565", "")
		w := httptest.NewRecorder()

		IconHandler(mock)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", w.Code)
		}
		if p := hdDecodeProblem(t, w.Body.Bytes()); p.Detail != msgIconUnavailable {
			t.Fatalf("detail = %q, want %q", p.Detail, msgIconUnavailable)
		}
	})

	t.Run("HD-33_MissingBedrockIconReturns500", func(t *testing.T) {
		hdUseIconDir(t, false)
		mock := &hdMockService{}
		req := hdRequest(t, "example.com:19132", "bedrock=true")
		w := httptest.NewRecorder()

		IconHandler(mock)(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", w.Code)
		}
		if p := hdDecodeProblem(t, w.Body.Bytes()); p.Detail != msgIconUnavailable {
			t.Fatalf("detail = %q, want %q", p.Detail, msgIconUnavailable)
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
		mock := &hdMockService{serverErr: ErrJavaStatus}
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
		if got := mock.serverCalls[0].host; got != "example.com" {
			t.Fatalf("expected host example.com, got %q", got)
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
		if !call.isBedrock || !call.queryEnabled || call.queryPort != 1234 || call.port != 25566 || call.host != "example.com" {
			t.Fatalf("expected flags forwarded (bedrock=true, query=true, queryPort=1234, host=example.com, port=25566), got %+v", call)
		}
	})

	for _, tc := range []struct {
		name  string
		value string
	}{
		{"Zero", "0"},
		{"Negative", "-5"},
		{"TooLarge", "65536"},
		{"NotANumber", "abc"},
	} {
		t.Run("HD-19_SimpleStatus_InvalidQueryPort_"+tc.name, func(t *testing.T) {
			mock := &hdMockService{serverStatus: hdValidStatus(nil)}
			req := hdRequest(t, "mc.example.com:25570", "query=true&query_port="+tc.value)

			SimpleStatusHandler(mock)(httptest.NewRecorder(), req)

			if len(mock.serverCalls) != 1 {
				t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
			}
			if got := mock.serverCalls[0].queryPort; got != 25570 {
				t.Fatalf("queryPort = %d, want the server port 25570", got)
			}
		})
	}

	for _, bound := range []string{"1", "65535"} {
		t.Run("HD-19_SimpleStatus_BoundaryAccepted_"+bound, func(t *testing.T) {
			mock := &hdMockService{serverStatus: hdValidStatus(nil)}
			req := hdRequest(t, "mc.example.com:25570", "query=true&query_port="+bound)

			SimpleStatusHandler(mock)(httptest.NewRecorder(), req)

			want, _ := strconv.Atoi(bound)
			if len(mock.serverCalls) != 1 || mock.serverCalls[0].queryPort != want {
				t.Fatalf("calls = %+v, want one call with queryPort %d", mock.serverCalls, want)
			}
		})
	}

	t.Run("HD-20_MissingQueryPortDefaultsToPort", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "mc.example.com:25570", "query=true")

		SimpleStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.queryPort != call.port || call.port != 25570 {
			t.Fatalf("call = %+v, want queryPort equal to port 25570", call)
		}
	})

	t.Run("HD-24_HostPortSplitJava", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "mc.example.com:25570", "")

		SimpleStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "mc.example.com" || call.port != 25570 || call.isBedrock {
			t.Fatalf("call = %+v, want host mc.example.com, port 25570, java", call)
		}
	})

	t.Run("HD-25_HostPortSplitBedrock", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "mc.example.com:19140", "bedrock=true")

		SimpleStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "mc.example.com" || call.port != 19140 || !call.isBedrock {
			t.Fatalf("call = %+v, want host mc.example.com, port 19140, bedrock", call)
		}
	})

	t.Run("HD-26_NoPortSuffixDefaultsJavaPort", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "example.com", "")

		SimpleStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "example.com" || call.port != 25565 {
			t.Fatalf("call = %+v, want host example.com, port 25565", call)
		}
	})

	t.Run("HD-29_NonNumericSuffixKeptJava", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "mc.example.com:abc", "")

		SimpleStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "mc.example.com:abc" || call.port != 25565 || call.isBedrock {
			t.Fatalf("call = %+v, want host mc.example.com:abc, port 25565, bedrock false", call)
		}
	})

	t.Run("HD-30_NonNumericSuffixKeptBedrock", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "mc.example.com:abc", "bedrock=true")

		SimpleStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "mc.example.com:abc" || call.port != 19132 || !call.isBedrock {
			t.Fatalf("call = %+v, want host mc.example.com:abc, port 19132, bedrock true", call)
		}
	})
}
