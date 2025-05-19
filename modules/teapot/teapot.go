package teapot

import (
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// HandleTeapot - Handle teapot requests
func HandleTeapot(w http.ResponseWriter, r *http.Request) {
	responses.NewProblem(
		"about:blank",
		http.StatusTeapot,
		"I'm a teapot",
		"You requested a cup of coffee, but I'm a teapot.",
		"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/418",
	).SendProblem(w, r)
}
