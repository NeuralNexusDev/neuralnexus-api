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
func SendAndEncodeProblem(w http.ResponseWriter, r *http.Request, problem *ProblemResponse) {
	w.WriteHeader(problem.Status)
	if r.Header.Get("Content-Type") == "application/xml" {
		w.Header().Set("Content-Type", "application/problem+xml")
		xml.NewEncoder(w).Encode(problem)
	} else {
		w.Header().Set("Content-Type", "application/problem+json")
		json.NewEncoder(w).Encode(problem)
	}
}

// SendAndEncodeForbidden -- Send a ForbiddenResponse as JSON or XML
func SendAndEncodeForbidden(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		message = "You do not have permission to access this resource."
	}
	problem := NewProblemResponse(
		"about:blank",
		http.StatusForbidden,
		"Forbidden",
		message,
		"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/403",
	)
	SendAndEncodeProblem(w, r, problem)
}

// SendAndEncodeNotFound -- Send a NotFoundResponse as JSON or XML
func SendAndEncodeNotFound(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		message = "The requested resource could not be found."
	}
	problem := NewProblemResponse(
		"about:blank",
		http.StatusNotFound,
		"Not Found",
		message,
		"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/404",
	)
	SendAndEncodeProblem(w, r, problem)
}

// SendAndEncodeBadRequest - Send and encode an invalid input problem
func SendAndEncodeBadRequest(w http.ResponseWriter, r *http.Request, message string) {
	problem := NewProblemResponse(
		"about:blank",
		http.StatusBadRequest,
		"Bad Request",
		message,
		"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/400",
	)
	SendAndEncodeProblem(w, r, problem)
}

// DecodeStruct -- Decode a struct from JSON or XML
func DecodeStruct[T any](r *http.Request, data *T) error {
	if r.Header.Get("Content-Type") == "application/xml" {
		return xml.NewDecoder(r.Body).Decode(data)
	}
	return json.NewDecoder(r.Body).Decode(data)
}
