package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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
	Address  string
	UsingUDS bool
}

// NewAPIServer - Create a new API server
func NewAPIServer(address string, usingUDS bool) *APIServer {
	return &APIServer{
		Address:  address,
		UsingUDS: usingUDS,
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
		cors.AllowAll().Handler,
	)

	router := http.NewServeMux()
	authedRouter := http.NewServeMux()
	router, authedRouter = routerStack(router, authedRouter)
	router.Handle("/", middleware.AuthMiddleware(authedRouter))

	v1 := http.NewServeMux()
	v1.Handle("/api/v1/", http.StripPrefix("/api/v1", router))

	if s.UsingUDS {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			os.Remove("/tmp/echo.sock")
			os.Exit(1)
		}()

		if _, err := os.Stat(s.Address); err == nil {
			log.Printf("Removing existing socket file %s", s.Address)
			if err := os.Remove(s.Address); err != nil {
				return err
			}
		}

		socket, err := net.Listen("unix", s.Address)
		if err != nil {
			return err
		}
		server := http.Server{
			Addr:    s.Address,
			Handler: middlewareStack(v1),
		}
		log.Printf("API Server listening on %s", s.Address)
		return server.Serve(socket)
	} else {
		server := http.Server{
			Addr:    s.Address,
			Handler: middlewareStack(v1),
		}
		log.Printf("API Server listening on %s", s.Address)
		return server.ListenAndServe()
	}
}
