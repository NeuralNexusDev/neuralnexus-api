package teapot

import (
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// -------------- Routes --------------

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux, authedMux *http.ServeMux) (*http.ServeMux, *http.ServeMux) {
	mux.HandleFunc("GET /teapot", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responses.NewProblemResponse(
			"about:blank",
			http.StatusTeapot,
			"I'm a teapot",
			"I'm a little teapot, short and stout\nHere is my handle, here is my spout\n",
			"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/418",
		).SendAndEncodeProblem(w, r)
	}))
	return mux, authedMux
}
