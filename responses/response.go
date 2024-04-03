package responses

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
)

// -------------- Functions --------------
// SendAndEncodeStruct -- Send a struct as JSON or XML
func SendAndEncodeStruct(w http.ResponseWriter, r *http.Request, statusCode int, data interface{}) {
	w.WriteHeader(statusCode)
	if r.Header.Get("Content-Type") == "application/xml" {
		w.Header().Set("Content-Type", "application/xml")
		xml.NewEncoder(w).Encode(data)
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}
}

// SendAndEncodeProblem -- Send a ProblemResponse as JSON or XML
func SendAndEncodeProblem(w http.ResponseWriter, r *http.Request, statusCode int, problem *ProblemResponse) {
	w.WriteHeader(statusCode)
	if r.Header.Get("Content-Type") == "application/xml" {
		w.Header().Set("Content-Type", "application/problem+xml")
		xml.NewEncoder(w).Encode(problem)
	} else {
		w.Header().Set("Content-Type", "application/problem+json")
		json.NewEncoder(w).Encode(problem)
	}
}
