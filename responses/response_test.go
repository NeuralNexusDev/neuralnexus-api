package responses

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/problempb"
	"google.golang.org/protobuf/proto"
)

func TestRR01_TooManyRequestsRetryAfterIsHTTPDate(t *testing.T) {
	prev := time.Local
	time.Local = time.FixedZone("EDT", -4*60*60)
	t.Cleanup(func() { time.Local = prev })

	t.Run("RR-01_RetryAfterParsesAsGMTHTTPDate", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		before := time.Now()

		TooManyRequests(w, r, 60, "slow down")

		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
		}
		got := w.Header().Get("Retry-After")
		if !strings.HasSuffix(got, " GMT") {
			t.Errorf("Retry-After = %q, want an HTTP-date ending in GMT", got)
		}
		parsed, err := http.ParseTime(got)
		if err != nil {
			t.Fatalf("Retry-After = %q does not parse as an HTTP-date: %v", got, err)
		}
		if d := parsed.Sub(before); d < 59*time.Second || d > 62*time.Second {
			t.Errorf("Retry-After = %v is %v after the request, want about 60s", parsed, d)
		}
	})
}

type rrExtendedProblem struct {
	XMLName xml.Name `json:"-" xml:"Problem"`
	*Problem
	Host string `json:"host" xml:"host"`
	Port int    `json:"port" xml:"port"`
}

func TestSendProblemStruct(t *testing.T) {
	t.Run("RR-02_ExtensionMembersSitAlongsideProblemMembers", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)

		problem := NewNotFoundProblem("gone")
		SendProblemStruct(w, r, problem, rrExtendedProblem{Problem: problem, Host: "example.com", Port: 25565})

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
			t.Fatalf("Content-Type = %q, want application/problem+json", ct)
		}
		var got map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("body is not JSON: %v", err)
		}
		if got["status"] != float64(404) || got["title"] != "Not Found" || got["detail"] != "gone" {
			t.Errorf("problem members = %v, want status 404, title Not Found, detail gone", got)
		}
		if got["host"] != "example.com" || got["port"] != float64(25565) {
			t.Errorf("extension members = %v, want host example.com and port 25565", got)
		}
	})

	t.Run("RR-03_XmlAcceptSendsProblemXml", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Accept", "application/xml")

		problem := NewNotFoundProblem("gone")
		SendProblemStruct(w, r, problem, rrExtendedProblem{Problem: problem, Host: "example.com", Port: 25565})

		if ct := w.Header().Get("Content-Type"); ct != "application/problem+xml" {
			t.Fatalf("Content-Type = %q, want application/problem+xml", ct)
		}
		body := w.Body.String()
		if !strings.HasPrefix(body, "<Problem>") || !strings.HasSuffix(body, "</Problem>") {
			t.Errorf("body = %q, want a Problem root element", body)
		}
		if !strings.Contains(body, "<host>example.com</host>") || !strings.Contains(body, "<port>25565</port>") {
			t.Errorf("body = %q, want the host and port extension members", body)
		}
	})

	t.Run("RR-04_ProtobufAcceptSendsThePlainProblem", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Accept", "application/x-protobuf")
		problem := NewNotFoundProblem("gone")

		SendProblemStruct(w, r, problem, rrExtendedProblem{Problem: problem, Host: "example.com", Port: 25565})

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
		}
		if ct := w.Header().Get("Content-Type"); ct != "application/problem+x-protobuf" {
			t.Fatalf("Content-Type = %q, want application/problem+x-protobuf", ct)
		}
		var got problempb.Problem
		if err := proto.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("body is not a Problem message: %v", err)
		}
		if got.Status != 404 || got.Detail != "gone" {
			t.Errorf("problem = %v, want status 404 and detail gone", &got)
		}
	})
}
