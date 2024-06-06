package archive

import (
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
)

// ApplyRoutes - Apply routes to the router
func ApplyRoutes(router *http.ServeMux) *http.ServeMux {
	bucket := NewS3Store(database.GetS3())
	bucket.MakeBucket()
	// service := NewService(bucket)
	return router
}
