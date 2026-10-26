package archive

import (
	"net/http"
)

// ApplyRoutes - Apply routes to the router
func ApplyRoutes(router *http.ServeMux) *http.ServeMux {
	// bucket := NewS3Store(database.GetS3Old())
	// bucket.MakeBucket()
	// service := NewService(bucket)
	return router
}
