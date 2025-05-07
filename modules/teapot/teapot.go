package teapot

import (
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// -------------- Routes --------------

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	mux.HandleFunc("GET /api/v1/teapot", func(w http.ResponseWriter, r *http.Request) {
		responses.NewProblem(
			"about:blank",
			http.StatusTeapot,
			"I'm a teapot",
			"You requested a cup of coffee, but I'm a teapot.",
			"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/418",
		).SendProblem(w, r)
	})
	return mux
}
