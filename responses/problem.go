package responses

import (
	"encoding/xml"
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/problempb"
	"github.com/goccy/go-json"
	"google.golang.org/protobuf/proto"
)

// -------------- Structs --------------

// Problem -- Defined by https://www.rfc-editor.org/rfc/rfc9457.html#section-3
type Problem struct {
	*problempb.Problem
}

// NewProblem -- Create a new Problem
func NewProblem(Type string, Status int, Title string, Detail string, Instance string) *Problem {
	return &Problem{
		&problempb.Problem{
			Type:     Type,
			Status:   int32(Status),
			Title:    Title,
			Detail:   Detail,
			Instance: Instance,
		},
	}
}

// NewNotFoundProblem -- Create the Problem that NotFound sends
func NewNotFoundProblem(message string) *Problem {
	if message == "" {
		message = "The requested resource could not be found."
	}
	return NewProblem(
		"about:blank",
		http.StatusNotFound,
		"Not Found",
		message,
		"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/404",
	)
}

// SendProblemStruct -- Send a struct that embeds a Problem and adds extension members, as JSON or XML
func SendProblemStruct[T any](w http.ResponseWriter, r *http.Request, statusCode int, data T) {
	var content string = "application/problem+"
	var structBytes []byte
	if r.Header.Get("Accept") == "application/xml" {
		content += "xml"
		structBytes, _ = xml.Marshal(data)
	}
	if structBytes == nil {
		content += "json"
		structBytes, _ = json.Marshal(data)
	}
	w.Header().Set("Content-Type", content)
	w.WriteHeader(statusCode)
	w.Write(structBytes)
}

// SendProblem -- Send a Problem as JSON, XML or Protobuf
func (problem *Problem) SendProblem(w http.ResponseWriter, r *http.Request) {
	var content string = "application/problem+"
	var structBytes []byte
	switch accept := r.Header.Get("Accept"); accept {
	case "application/x-protobuf":
		content += "x-protobuf"
		if pb, ok := any(problem).(proto.Message); ok {
			structBytes, _ = proto.Marshal(pb)
		}
		if encoder, ok := any(problem).(ProtoEncoder); ok {
			structBytes, _ = proto.Marshal(encoder.ToProto())
		}
	case "application/xml":
		content += "xml"
		structBytes, _ = xml.Marshal(problem)
	}
	if structBytes == nil {
		content += "json"
		structBytes, _ = json.Marshal(problem)
	}
	w.Header().Set("Content-Type", content)
	w.WriteHeader(int(problem.Status))
	w.Write(structBytes)
}
