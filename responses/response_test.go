package responses

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/problempb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
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

type rrDecoded struct {
	XMLName xml.Name `json:"-" xml:"item"`
	A       int      `json:"a" xml:"a"`
}

var errRRBoom = errors.New("boom")

func rrDecodeRequest(contentType string, contentLength int64, body io.Reader) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", body)
	r.ContentLength = contentLength
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	return r
}

func rrRecovered(f func()) (recovered any) {
	defer func() { recovered = recover() }()
	f()
	return nil
}

func rrRequirePanicMessage(t *testing.T, recovered any) {
	t.Helper()
	msg, ok := recovered.(string)
	if !ok || msg == "" {
		t.Fatalf("recovered = %#v, want a non-empty string", recovered)
	}
}

func TestDecodeStruct(t *testing.T) {
	protoBody, err := proto.Marshal(wrapperspb.String("hello"))
	if err != nil {
		t.Fatalf("failed to marshal the protobuf body: %v", err)
	}

	t.Run("RR-05_JSONNegativeContentLengthDecodes", func(t *testing.T) {
		var got rrDecoded
		r := rrDecodeRequest("application/json", -1, strings.NewReader(`{"a":1}`))

		if err := DecodeStruct(r, &got); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if got.A != 1 {
			t.Errorf("A = %d, want 1", got.A)
		}
	})

	t.Run("RR-06_ProtobufNegativeContentLengthDecodes", func(t *testing.T) {
		got := &wrapperspb.StringValue{}
		r := rrDecodeRequest("application/x-protobuf", -1, bytes.NewReader(protoBody))

		if err := DecodeStruct(r, &got); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if got.Value != "hello" {
			t.Errorf("Value = %q, want hello", got.Value)
		}
	})

	t.Run("RR-07_OneByteReadsDecodeProtobuf", func(t *testing.T) {
		got := &wrapperspb.StringValue{}
		r := rrDecodeRequest("application/x-protobuf", int64(len(protoBody)), iotest.OneByteReader(bytes.NewReader(protoBody)))

		if err := DecodeStruct(r, &got); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if got.Value != "hello" {
			t.Errorf("Value = %q, want hello", got.Value)
		}
	})

	t.Run("RR-08_HugeContentLengthPanics", func(t *testing.T) {
		cases := []struct {
			name        string
			contentType string
		}{
			{"JSON", "application/json"},
			{"XML", "application/xml"},
			{"Protobuf", "application/x-protobuf"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				var got rrDecoded
				r := rrDecodeRequest(c.contentType, 1<<40, strings.NewReader(`{"a":1}`))

				recovered := rrRecovered(func() { _ = DecodeStruct(r, &got) })

				rrRequirePanicMessage(t, recovered)
			})
		}
	})

	t.Run("RR-09_BodyOverLimitWithoutContentLengthPanics", func(t *testing.T) {
		var got string
		r := rrDecodeRequest("application/json", -1, bytes.NewReader(bytes.Repeat([]byte("a"), maxBodyBytes+1)))

		recovered := rrRecovered(func() { _ = DecodeStruct(r, &got) })

		rrRequirePanicMessage(t, recovered)
	})

	t.Run("RR-10_BodyExactlyAtLimitDecodes", func(t *testing.T) {
		var got string
		body := `"` + strings.Repeat("a", maxBodyBytes-2) + `"`
		if len(body) != maxBodyBytes {
			t.Fatalf("body is %d bytes, want %d", len(body), maxBodyBytes)
		}
		r := rrDecodeRequest("application/json", -1, strings.NewReader(body))

		if err := DecodeStruct(r, &got); err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if len(got) != maxBodyBytes-2 {
			t.Errorf("decoded %d bytes, want %d", len(got), maxBodyBytes-2)
		}
	})

	t.Run("RR-11_ReadErrorIsReturned", func(t *testing.T) {
		var got rrDecoded
		r := rrDecodeRequest("application/json", -1, iotest.ErrReader(errRRBoom))
		var err error

		recovered := rrRecovered(func() { err = DecodeStruct(r, &got) })

		if recovered != nil {
			t.Fatalf("panicked with %#v, want an error", recovered)
		}
		if !errors.Is(err, errRRBoom) {
			t.Errorf("err = %v, want it to wrap errRRBoom", err)
		}
	})

	t.Run("RR-12_NullBodyPanics", func(t *testing.T) {
		var got *rrDecoded
		r := rrDecodeRequest("application/json", -1, strings.NewReader("null"))

		recovered := rrRecovered(func() { _ = DecodeStruct(r, &got) })

		rrRequirePanicMessage(t, recovered)

		var empty *rrDecoded
		r = rrDecodeRequest("application/json", -1, strings.NewReader("{}"))
		if err := DecodeStruct(r, &empty); err != nil {
			t.Fatalf("err = %v, want nil for {}", err)
		}
		if empty == nil {
			t.Error("decoded {} to a nil pointer, want a non-nil one")
		}
	})

	t.Run("RR-13_ProtobufIntoUnsupportedTypePanics", func(t *testing.T) {
		var got rrDecoded
		r := rrDecodeRequest("application/x-protobuf", int64(len(protoBody)), bytes.NewReader(protoBody))

		recovered := rrRecovered(func() { _ = DecodeStruct(r, &got) })

		rrRequirePanicMessage(t, recovered)
	})

	t.Run("RR-14_OrdinaryBodiesStillDecode", func(t *testing.T) {
		var fromJSON rrDecoded
		r := rrDecodeRequest("application/json", 7, strings.NewReader(`{"a":2}`))
		if err := DecodeStruct(r, &fromJSON); err != nil || fromJSON.A != 2 {
			t.Errorf("JSON: A = %d, err = %v, want 2 and nil", fromJSON.A, err)
		}

		var fromXML rrDecoded
		r = rrDecodeRequest("application/xml", -1, strings.NewReader(`<item><a>3</a></item>`))
		if err := DecodeStruct(r, &fromXML); err != nil || fromXML.A != 3 {
			t.Errorf("XML: A = %d, err = %v, want 3 and nil", fromXML.A, err)
		}

		var bad rrDecoded
		r = rrDecodeRequest("application/json", -1, strings.NewReader(`{"a":`))
		var err error
		recovered := rrRecovered(func() { err = DecodeStruct(r, &bad) })
		if recovered != nil {
			t.Fatalf("malformed JSON panicked with %#v, want an error", recovered)
		}
		if err == nil {
			t.Error("malformed JSON returned nil, want an error")
		}
	})

	t.Run("RR-15_NilDataPanics", func(t *testing.T) {
		r := rrDecodeRequest("application/json", -1, strings.NewReader(`{"a":1}`))

		recovered := rrRecovered(func() { _ = DecodeStruct[rrDecoded](r, nil) })

		rrRequirePanicMessage(t, recovered)
	})
}
