package numbersds

import (
	"log"
	"net/http"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/responses"
)

// ApplyRoutes - Apply the routes
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {
	nStore := NewStore(database.GetDB("neuralnexus"))
	nService := NewService(nStore)
	mux.Handle("POST /api/v1/numbers", mw.Auth(CreateNumberHandler(nService)))
	return mux
}

// CreateNumberHandler - Create a new number
func CreateNumberHandler(s NumberService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var n *NumberData
		err := responses.DecodeStruct(r, &n)
		if err != nil {
			log.Println("Bad body:\n\t", err)
			responses.SendAndEncodeBadRequest(w, r, "")
			return
		}
		err = s.Create(n)
		if err != nil {
			log.Println("Failed to create number:\n\t", err)
			responses.SendAndEncodeInternalServerError(w, r, "Failed to create number")
			return
		}
	}
}
