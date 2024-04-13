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

// DecodeStruct -- Decode a struct from JSON or XML
func DecodeStruct[T any](r *http.Request, data *T) error {
	if r.Header.Get("Content-Type") == "application/xml" {
		return xml.NewDecoder(r.Body).Decode(data)
	}
	return json.NewDecoder(r.Body).Decode(data)
}

// SendAndEncodeBadRequest - Send and encode an invalid input problem
func SendAndEncodeBadRequest(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		message = "The request body is invalid."
	}
	NewProblemResponse(
		"about:blank",
		http.StatusBadRequest,
		"Bad Request",
		message,
		"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/400",
	).SendAndEncodeProblem(w, r)
}

// SendAndEncodeUnauthorized -- Send an UnauthorizedResponse as JSON or XML
func SendAndEncodeUnauthorized(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		message = "You must be logged in to access this resource."
	}
	NewProblemResponse(
		"about:blank",
		http.StatusUnauthorized,
		"Unauthorized",
		message,
		"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/401",
	).SendAndEncodeProblem(w, r)
}

// SendAndEncodeForbidden -- Send a ForbiddenResponse as JSON or XML
func SendAndEncodeForbidden(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		message = "You do not have permission to access this resource."
	}
	NewProblemResponse(
		"about:blank",
		http.StatusForbidden,
		"Forbidden",
		message,
		"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/403",
	).SendAndEncodeProblem(w, r)
}

// SendAndEncodeNotFound -- Send a NotFoundResponse as JSON or XML
func SendAndEncodeNotFound(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		message = "The requested resource could not be found."
	}
	NewProblemResponse(
		"about:blank",
		http.StatusNotFound,
		"Not Found",
		message,
		"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/404",
	).SendAndEncodeProblem(w, r)
}

// SendAndEncodeInternalServerError -- Send an InternalServerErrorResponse as JSON or XML
func SendAndEncodeInternalServerError(w http.ResponseWriter, r *http.Request, message string) {
	if message == "" {
		message = "An internal server error occurred."
	}
	NewProblemResponse(
		"about:blank",
		http.StatusInternalServerError,
		"Internal Server Error",
		message,
		"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/500",
	).SendAndEncodeProblem(w, r)
}
