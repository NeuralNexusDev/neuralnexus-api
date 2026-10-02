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
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	target := "/mcstatus/" + url.PathEscape(host)
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

	t.Run("HD-35_Ipv6BracketedWithPort", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "[2001:db8::1]:25570", "")

		ServerStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "2001:db8::1" || call.port != 25570 || call.isBedrock {
			t.Fatalf("call = %+v, want host 2001:db8::1, port 25570, bedrock false", call)
		}
	})

	t.Run("HD-36_Ipv6BracketedNoPortBedrock", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "[2001:db8::1]", "bedrock=true")

		ServerStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "2001:db8::1" || call.port != 19132 || !call.isBedrock {
			t.Fatalf("call = %+v, want host 2001:db8::1, port 19132, bedrock true", call)
		}
	})

	t.Run("HD-37_Ipv6BareLiteralDefaultsPort", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "2001:db8::1", "")

		ServerStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "2001:db8::1" || call.port != 25565 || call.isBedrock {
			t.Fatalf("call = %+v, want host 2001:db8::1, port 25565, bedrock false", call)
		}
	})

	t.Run("HD-41_Ipv6BracketedWithPortBedrock", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "[2001:db8::1]:19133", "bedrock=true")

		ServerStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "2001:db8::1" || call.port != 19133 || !call.isBedrock {
			t.Fatalf("call = %+v, want host 2001:db8::1, port 19133, bedrock true", call)
		}
	})

	t.Run("HD-42_Ipv6BracketedNoPortJava", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "[2001:db8::1]", "")

		ServerStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "2001:db8::1" || call.port != 25565 || call.isBedrock {
			t.Fatalf("call = %+v, want host 2001:db8::1, port 25565, bedrock false", call)
		}
	})

	t.Run("HD-43_Ipv6BareLiteralBedrock", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "2001:db8::1", "bedrock=true")

		ServerStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "2001:db8::1" || call.port != 19132 || !call.isBedrock {
			t.Fatalf("call = %+v, want host 2001:db8::1, port 19132, bedrock true", call)
		}
	})

	for _, tc := range []struct {
		name     string
		input    string
		query    string
		wantHost string
		wantPort int
	}{
		{"Domain", "example.com", "", "example.com", 25565},
		{"DomainWithPort", "example.com:25570", "", "example.com", 25570},
		{"DomainBedrockDefault", "example.com", "bedrock=true", "example.com", 19132},
		{"DomainUppercaseLowered", "Example.COM:25570", "", "example.com", 25570},
		{"DomainUnderscore", "a_b.example.com", "", "a_b.example.com", 25565},
		{"DomainHyphen", "my-server.example.com", "", "my-server.example.com", 25565},
		{"DomainNumericLabel", "123.example.com", "", "123.example.com", 25565},
		{"DomainLabelAtLimit", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com", "", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com", 25565},
		{"DomainAtLengthLimit", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc.ddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", "", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc.ddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd", 25565},
		{"Ipv4", "192.168.1.1", "", "192.168.1.1", 25565},
		{"PortWithLeadingZeros", "a.com:00080", "", "a.com", 80},
		{"Ipv4WithPort", "192.168.1.1:25570", "", "192.168.1.1", 25570},
		{"Ipv4BedrockDefault", "192.168.1.1", "bedrock=true", "192.168.1.1", 19132},
		{"Ipv6BareCanonicalised", "2001:DB8:0:0:0:0:0:1", "", "2001:db8::1", 25565},
		{"Ipv6BareBedrockDefault", "2001:db8::1", "bedrock=true", "2001:db8::1", 19132},
		{"Ipv6BracketedWithPort", "[2001:db8::1]:25570", "", "2001:db8::1", 25570},
		{"Ipv6BracketedNoPort", "[2001:db8::1]", "", "2001:db8::1", 25565},
		{"Ipv6BracketedBedrockWithPort", "[2001:db8::1]:19133", "bedrock=true", "2001:db8::1", 19133},
		{"Ipv6BracketedBedrockNoPort", "[2001:db8::1]", "bedrock=true", "2001:db8::1", 19132},
		{"Ipv6Loopback", "::1", "", "::1", 25565},
		{"Ipv6MappedIpv4", "::ffff:1.2.3.4", "", "::ffff:1.2.3.4", 25565},
		{"Ipv6Ipv4Tail", "::1.2.3.4", "", "::102:304", 25565},
	} {
		t.Run("HD-45_AcceptedHost_"+tc.name, func(t *testing.T) {
			mock := &hdMockService{serverStatus: hdValidStatus(nil)}
			req := hdRequest(t, tc.input, tc.query)

			ServerStatusHandler(mock)(httptest.NewRecorder(), req)

			if len(mock.serverCalls) != 1 {
				t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
			}
			if call := mock.serverCalls[0]; call.host != tc.wantHost || call.port != tc.wantPort {
				t.Fatalf("call = %+v, want host %s, port %d", call, tc.wantHost, tc.wantPort)
			}
		})
	}

	for _, tc := range []struct {
		name  string
		input string
	}{
		{"Localhost", "localhost"},
		{"Empty", ""},
		{"PortNotNumeric", "example.com:abc"},
		{"PortEmpty", "example.com:"},
		{"PortZero", "example.com:0"},
		{"PortTooLarge", "example.com:65536"},
		{"PortTooLong", "example.com:123456"},
		{"PortOnly", ":25565"},
		{"LabelLeadingHyphen", "-bad.example.com"},
		{"LabelTrailingHyphen", "bad-.example.com"},
		{"EmptyLabel", "a..example.com"},
		{"TrailingDot", "example.com."},
		{"LeadingDot", ".example.com"},
		{"LabelTooLong", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.example.com"},
		{"NameTooLong", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.ccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc.ddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddde"},
		{"InvalidCharacter", "exa$mple.com"},
		{"NonAscii", "bücher.example.com"},
		{"Ipv4OctetTooLarge", "256.1.1.1"},
		{"Ipv4LeadingZero", "01.2.3.4"},
		{"Ipv4ThreeOctets", "1.2.3"},
		{"Ipv4FiveOctets", "1.2.3.4.5"},
		{"Ipv4Bracketed", "[1.2.3.4]"},
		{"NameBracketed", "[example.com]"},
		{"Ipv6Unclosed", "[::1"},
		{"Ipv6EmptyBrackets", "[]"},
		{"Ipv6TextAfterBracket", "[::1]x"},
		{"Ipv6TextAfterBracketThenPort", "[::1]x80"},
		{"Ipv6HextetTooLarge", "[12345::1]"},
		{"NumericLastLabelAfterName", "example.123"},
		{"PortWithLeadingZerosTooLong", "a.com:0000080"},
		{"PortWithSixDigits", "a.com:000080"},
		{"Ipv6BracketedBadPort", "[::1]:abc"},
		{"Ipv6BracketedPortZero", "[::1]:0"},
		{"Ipv6DoubleBracketed", "[[::1]]"},
		{"Ipv6BareZone", "fe80::1%eth0"},
		{"Ipv6BracketedZone", "[fe80::1%eth0]"},
		{"Ipv6MappedLeadingZero", "::ffff:01.2.3.4"},
		{"Ipv6BareWithPortDigits", "2001:db8::1:25565"},
		{"Ipv6NotHex", "g::1"},
		{"Ipv6TooManyGroups", "1:2:3:4:5:6:7:8:9"},
		{"MultipleColonsNotIpv6", "a:b:c"},
	} {
		t.Run("HD-46_RejectedHost_"+tc.name, func(t *testing.T) {
			mock := &hdMockService{serverStatus: hdValidStatus(nil)}
			req := hdRequest(t, tc.input, "")
			w := httptest.NewRecorder()

			ServerStatusHandler(mock)(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", w.Code)
			}
			if p := hdDecodeProblem(t, w.Body.Bytes()); p.Detail != msgInvalidHost {
				t.Fatalf("expected detail %q, got %q", msgInvalidHost, p.Detail)
			}
			if len(mock.serverCalls) != 0 {
				t.Fatalf("expected no lookup, got %d", len(mock.serverCalls))
			}
		})
	}

	t.Run("HD-47_OfflineProblemCarriesCanonicalHostAndPort", func(t *testing.T) {
		mock := &hdMockService{serverErr: ErrJavaStatus}
		req := hdRequest(t, "Example.COM:25570", "")
		w := httptest.NewRecorder()

		ServerStatusHandler(mock)(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
			t.Fatalf("expected Content-Type application/problem+json, got %q", ct)
		}
		var body struct {
			Detail string `json:"detail"`
			Host   string `json:"host"`
			Port   int    `json:"port"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		if body.Detail != msgJavaStatusFailed || body.Host != "example.com" || body.Port != 25570 {
			t.Fatalf("body = %+v, want detail %q, host example.com, port 25570", body, msgJavaStatusFailed)
		}
		var members map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &members); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		if _, ok := members["XMLName"]; ok {
			t.Fatalf("body = %v, want no XMLName member", members)
		}
	})

	t.Run("HD-48_UnrecognizedErrorProblemHasNoHostOrPort", func(t *testing.T) {
		mock := &hdMockService{serverErr: testerrors.ErrBoom}
		req := hdRequest(t, "example.com:25570", "")
		w := httptest.NewRecorder()

		ServerStatusHandler(mock)(w, req)

		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		if _, ok := body["host"]; ok {
			t.Fatalf("body = %v, want no host member on an internal error", body)
		}
		if _, ok := body["port"]; ok {
			t.Fatalf("body = %v, want no port member on an internal error", body)
		}
	})

	t.Run("HD-52_OfflineProblemXmlHasProblemRoot", func(t *testing.T) {
		mock := &hdMockService{serverErr: ErrJavaStatus}
		req := hdRequest(t, "example.com:25570", "")
		req.Header.Set("Accept", "application/xml")
		w := httptest.NewRecorder()

		ServerStatusHandler(mock)(w, req)

		if ct := w.Header().Get("Content-Type"); ct != "application/problem+xml" {
			t.Fatalf("expected Content-Type application/problem+xml, got %q", ct)
		}
		body := w.Body.String()
		if !strings.HasPrefix(body, "<Problem>") || !strings.Contains(body, "<host>example.com</host>") || !strings.Contains(body, "<port>25570</port>") {
			t.Fatalf("body = %q, want a Problem root with the host and port elements", body)
		}
	})

	t.Run("HD-53_OfflineProblemProtobufIsThePlainProblem", func(t *testing.T) {
		mock := &hdMockService{serverErr: ErrJavaStatus}
		req := hdRequest(t, "example.com:25570", "")
		req.Header.Set("Accept", "application/x-protobuf")
		w := httptest.NewRecorder()

		ServerStatusHandler(mock)(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/problem+x-protobuf" {
			t.Fatalf("expected Content-Type application/problem+x-protobuf, got %q", ct)
		}
	})

	t.Run("HD-54_BedrockOfflineProblemCarriesDefaultPort", func(t *testing.T) {
		mock := &hdMockService{serverErr: ErrBedrockStatus}
		req := hdRequest(t, "Example.COM", "bedrock=true")
		w := httptest.NewRecorder()

		ServerStatusHandler(mock)(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
		var body struct {
			Detail string `json:"detail"`
			Host   string `json:"host"`
			Port   int    `json:"port"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		if body.Detail != msgBedrockStatusFailed || body.Host != "example.com" || body.Port != 19132 {
			t.Fatalf("body = %+v, want detail %q, host example.com, port 19132", body, msgBedrockStatusFailed)
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

	t.Run("HD-09_LookupFailureServesDefaultIcon", func(t *testing.T) {
		hdUseIconDir(t, true)
		mock := &hdMockService{javaErr: ErrJavaStatus}
		req := hdRequest(t, "example.com:25565", "")
		w := httptest.NewRecorder()

		IconHandler(mock)(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != "image/png" {
			t.Fatalf("expected Content-Type image/png, got %q", ct)
		}
		if got := hdDecodePixel(t, w.Body.Bytes()); got != hdDefaultIconColor {
			t.Fatalf("pixel = %v, want the default icon %v", got, hdDefaultIconColor)
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

	t.Run("HD-40_Ipv6BracketedWithPort", func(t *testing.T) {
		mock := &hdMockService{javaStatus: hdValidStatus(nil)}
		req := hdRequest(t, "[2001:db8::1]:25570", "")

		IconHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.javaCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.javaCalls))
		}
		if call := mock.javaCalls[0]; call.host != "2001:db8::1" || call.port != 25570 {
			t.Fatalf("call = %+v, want host 2001:db8::1, port 25570", call)
		}
	})

	t.Run("HD-50_MalformedHostIsRefused", func(t *testing.T) {
		mock := &hdMockService{}
		req := hdRequest(t, "localhost", "")
		w := httptest.NewRecorder()

		IconHandler(mock)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
		if p := hdDecodeProblem(t, w.Body.Bytes()); p.Detail != msgInvalidHost {
			t.Fatalf("expected detail %q, got %q", msgInvalidHost, p.Detail)
		}
		if len(mock.javaCalls) != 0 {
			t.Fatalf("expected no lookup, got %d", len(mock.javaCalls))
		}
	})

	t.Run("HD-51_MalformedHostIsRefusedForBedrock", func(t *testing.T) {
		mock := &hdMockService{}
		req := hdRequest(t, "localhost", "bedrock=true")
		w := httptest.NewRecorder()

		IconHandler(mock)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
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

	t.Run("HD-39_Ipv6BracketedWithPort", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "[2001:db8::1]:25570", "")

		SimpleStatusHandler(mock)(httptest.NewRecorder(), req)

		if len(mock.serverCalls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mock.serverCalls))
		}
		if call := mock.serverCalls[0]; call.host != "2001:db8::1" || call.port != 25570 || call.isBedrock {
			t.Fatalf("call = %+v, want host 2001:db8::1, port 25570, bedrock false", call)
		}
	})

	t.Run("HD-49_MalformedHostIsRefused", func(t *testing.T) {
		mock := &hdMockService{serverStatus: hdValidStatus(nil)}
		req := hdRequest(t, "localhost", "")
		w := httptest.NewRecorder()

		SimpleStatusHandler(mock)(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
		if p := hdDecodeProblem(t, w.Body.Bytes()); p.Detail != msgInvalidHost {
			t.Fatalf("expected detail %q, got %q", msgInvalidHost, p.Detail)
		}
		if len(mock.serverCalls) != 0 {
			t.Fatalf("expected no lookup, got %d", len(mock.serverCalls))
		}
	})
}
