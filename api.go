package main

import (
	"log"
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/middleware"
	authroutes "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/routes"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/beenamegenerator"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/cct_turtle"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/mcstatus"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/petpictures"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/switchboard"
	"github.com/NeuralNexusDev/neuralnexus-api/routes"
	"github.com/rs/cors"
)

// -------------- Structs --------------
type APIServer struct {
	Address string
}

// NewAPIServer - Create a new API server
func NewAPIServer(address string) *APIServer {
	return &APIServer{
		Address: address,
	}
}

// Run - Start the API server
func (s *APIServer) Run() error {
	routerStack := routes.CreateStack(
		authroutes.ApplyRoutes,
		beenamegenerator.ApplyRoutes,
		cct_turtle.ApplyRoutes,
		mcstatus.ApplyRoutes,
		petpictures.ApplyRoutes,
		switchboard.ApplyRoutes,
	)

	middlewareStack := middleware.CreateStack(
		middleware.RequestLoggerMiddleware,
		cors.New(cors.Options{
			AllowCredentials: true,
		}).Handler,
	)

	authedMiddlewareStack := middleware.CreateStack(
		middleware.AuthMiddleware,
	)

	router := http.NewServeMux()
	authedRouter := http.NewServeMux()
	router, authedRouter = routerStack(router, authedRouter)
	router.Handle("/", authedMiddlewareStack(authedRouter))

	v1 := http.NewServeMux()
	v1.Handle("/api/v1/", http.StripPrefix("/api/v1", router))

	server := http.Server{
		Addr:    s.Address,
		Handler: middlewareStack(v1),
	}

	log.Printf("API Server listening on %s", s.Address)
	return server.ListenAndServe()
}
