package teapot

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/problempb"
	"github.com/goccy/go-json"
	"google.golang.org/protobuf/proto"
)

const (
	wantType     = "about:blank"
	wantStatus   = http.StatusTeapot
	wantTitle    = "I'm a teapot"
	wantDetail   = "You requested a cup of coffee, but I'm a teapot."
	wantInstance = "https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/418"
)

// problemJSON and problemXML mirror the wire shape of responses.problem for
// decoding in tests, without depending on any exported type from that
// package's own test files.
type problemJSON struct {
	Type     string `json:"type"`
	Status   int32  `json:"status"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
}

type problemXML struct {
	Type     string `xml:"type"`
	Status   int32  `xml:"status"`
	Title    string `xml:"title"`
	Detail   string `xml:"detail"`
	Instance string `xml:"instance"`
}

func strPtr(s string) *string { return &s }

func assertProblemFields(t *testing.T, gotType string, gotStatus int32, gotTitle, gotDetail, gotInstance string) {
	t.Helper()
	if gotType != wantType {
		t.Errorf("type = %q, want %q", gotType, wantType)
	}
	if gotStatus != int32(wantStatus) {
		t.Errorf("status = %d, want %d", gotStatus, wantStatus)
	}
	if gotTitle != wantTitle {
		t.Errorf("title = %q, want %q", gotTitle, wantTitle)
	}
	if gotDetail != wantDetail {
		t.Errorf("detail = %q, want %q", gotDetail, wantDetail)
	}
	if gotInstance != wantInstance {
		t.Errorf("instance = %q, want %q", gotInstance, wantInstance)
	}
}

type wantFormat int

const (
	formatJSON wantFormat = iota
	formatXML
	formatProtobuf
)

func TestHandleTeapot(t *testing.T) {
	tests := []struct {
		name            string
		acceptHeader    *string
		wantFormat      wantFormat
		wantContentType string
	}{
		{
			name:            "TP-01_NoAcceptHeaderDefaultsToJSON",
			acceptHeader:    nil,
			wantFormat:      formatJSON,
			wantContentType: "application/problem+json",
		},
		{
			name:            "TP-02_AcceptXMLExactMatchUsesXML",
			acceptHeader:    strPtr("application/xml"),
			wantFormat:      formatXML,
			wantContentType: "application/problem+xml",
		},
		{
			name:            "TP-03_AcceptProtobufExactMatchUsesProtobuf",
			acceptHeader:    strPtr("application/x-protobuf"),
			wantFormat:      formatProtobuf,
			wantContentType: "application/problem+x-protobuf",
		},
		{
			name:            "TP-04_UnrecognizedAcceptFallsBackToJSON",
			acceptHeader:    strPtr("text/plain"),
			wantFormat:      formatJSON,
			wantContentType: "application/problem+json",
		},
		{
			name:            "TP-05_CaseMismatchedAcceptFallsBackToJSON",
			acceptHeader:    strPtr("APPLICATION/XML"),
			wantFormat:      formatJSON,
			wantContentType: "application/problem+json",
		},
		{
			name:            "TP-06_CompoundAcceptFallsBackToJSON",
			acceptHeader:    strPtr("application/xml, application/json;q=0.9"),
			wantFormat:      formatJSON,
			wantContentType: "application/problem+json",
		},
		{
			name:            "TP-07_EmptyAcceptFallsBackToJSON",
			acceptHeader:    strPtr(""),
			wantFormat:      formatJSON,
			wantContentType: "application/problem+json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/teapot", nil)
			if tt.acceptHeader != nil {
				req.Header.Set("Accept", *tt.acceptHeader)
			}
			rec := httptest.NewRecorder()

			HandleTeapot(rec, req)

			if rec.Code != http.StatusTeapot {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
			}
			if ct := rec.Header().Get("Content-Type"); ct != tt.wantContentType {
				t.Fatalf("Content-Type = %q, want %q", ct, tt.wantContentType)
			}

			switch tt.wantFormat {
			case formatJSON:
				var got problemJSON
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("json.Unmarshal: %v", err)
				}
				assertProblemFields(t, got.Type, got.Status, got.Title, got.Detail, got.Instance)
			case formatXML:
				var got problemXML
				if err := xml.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("xml.Unmarshal: %v", err)
				}
				assertProblemFields(t, got.Type, got.Status, got.Title, got.Detail, got.Instance)
			case formatProtobuf:
				var got problempb.Problem
				if err := proto.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("proto.Unmarshal: %v", err)
				}
				assertProblemFields(t, got.Type, got.Status, got.Title, got.Detail, got.Instance)
			}
		})
	}
}
