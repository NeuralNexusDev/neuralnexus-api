package responses

import (
	"encoding/xml"
	"net/http"

	"github.com/goccy/go-json"
)

// -------------- Structs --------------

// ProblemResponse -- Defined by https://www.rfc-editor.org/rfc/rfc9457.html#section-3
type ProblemResponse struct {
	Type     string `json:"type" xml:"type"`
	Status   int    `json:"status" xml:"status"`
	Title    string `json:"title" xml:"title"`
	Detail   string `json:"detail" xml:"detail"`
	Instance string `json:"instance" xml:"instance"`
}

// NewProblemResponse -- Create a new ProblemResponse
func NewProblemResponse(Type string, Status int, Title string, Detail string, Instance string) *ProblemResponse {
	return &ProblemResponse{
		Type:     Type,
		Status:   Status,
		Title:    Title,
		Detail:   Detail,
		Instance: Instance,
	}
}

// SendAndEncodeProblem -- Send a ProblemResponse as JSON or XML
func (problem *ProblemResponse) SendAndEncodeProblem(w http.ResponseWriter, r *http.Request) {
	var problemBytes []byte
	if r.Header.Get("Accept") == "application/xml" {
		w.Header().Set("Content-Type", "application/problem+xml")
		problemBytes, _ = xml.Marshal(problem)
	} else {
		w.Header().Set("Content-Type", "application/problem+json")
		problemBytes, _ = json.Marshal(problem)
	}
	w.WriteHeader(problem.Status)
	w.Write(problemBytes)
}
