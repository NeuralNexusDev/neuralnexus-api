package responses

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
)

// -------------- Functions --------------
// SendAndEncodeStruct -- Send a struct as JSON or XML
func SendAndEncodeStruct[T any](w http.ResponseWriter, r *http.Request, statusCode int, data T) {
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

// DecodeStruct -- Decode a struct from JSON or XML
func DecodeStruct[T any](r *http.Request, data T) error {
	if r.Header.Get("Content-Type") == "application/xml" {
		return xml.NewDecoder(r.Body).Decode(data)
	}
	return json.NewDecoder(r.Body).Decode(data)
}
