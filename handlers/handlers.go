package handlers

import "net/http"

func InjectHandler[T any](handler func(http.ResponseWriter, *http.Request, *T), dependency *T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handler(w, r, dependency)
	}
}
