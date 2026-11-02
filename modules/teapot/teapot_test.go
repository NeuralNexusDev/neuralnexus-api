package teapot

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/problempb"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
	"github.com/goccy/go-json"
	"google.golang.org/protobuf/proto"
)

const wantDetail = "You requested a cup of coffee, but I'm a teapot."

func strPtr(s string) *string { return &s }

func assertProblemDetail(t *testing.T, gotDetail string) {
	t.Helper()
	if gotDetail != wantDetail {
		t.Errorf("detail = %q, want %q", gotDetail, wantDetail)
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
				var got responses.Problem
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("json.Unmarshal: %v", err)
				}
				assertProblemDetail(t, got.Detail)
			case formatXML:
				var got responses.Problem
				if err := xml.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("xml.Unmarshal: %v", err)
				}
				assertProblemDetail(t, got.Detail)
			case formatProtobuf:
				var got problempb.Problem
				if err := proto.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("proto.Unmarshal: %v", err)
				}
				assertProblemDetail(t, got.Detail)
			}
		})
	}
}
