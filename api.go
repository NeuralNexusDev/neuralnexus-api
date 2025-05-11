package main

import (
	"github.com/rs/cors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth"
	authroutes "github.com/NeuralNexusDev/neuralnexus-api/modules/auth/routes"
	beenamegenerator "github.com/NeuralNexusDev/neuralnexus-api/modules/bee_name_generator"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/datastore"
	numbersds "github.com/NeuralNexusDev/neuralnexus-api/modules/datastore/numbers"
	gss "github.com/NeuralNexusDev/neuralnexus-api/modules/game_server_status"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/mcstatus"
	petpictures "github.com/NeuralNexusDev/neuralnexus-api/modules/pet_pictures"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/projects"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/switchboard"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/teapot"
	"github.com/NeuralNexusDev/neuralnexus-api/routes"
)

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

// ApplyRoutes - Apply the routes to the API server
func ApplyRoutes(mux *http.ServeMux) *http.ServeMux {

	return mux
}

// Setup - Setup the API server
func (s *APIServer) Setup() http.Handler {
	routerStack := routes.CreateStack(
		authroutes.ApplyRoutes,
		beenamegenerator.ApplyRoutes,
		datastore.ApplyRoutes,
		numbersds.ApplyRoutes,
		gss.ApplyRoutes,
		mcstatus.ApplyRoutes,
		petpictures.ApplyRoutes,
		projects.ApplyRoutes,
		switchboard.ApplyRoutes,
		teapot.ApplyRoutes,
	)

	db := database.GetDB("neuralnexus")
	rdb := database.GetRedis()
	store := auth.NewStore(db, rdb)
	session := auth.NewSessionService(store)
	rateLimit := auth.NewRateLimitService(store)

	middlewareStack := mw.CreateStack(
		cors.AllowAll().Handler,
		mw.IPMiddleware,
		mw.SessionMiddleware(session),
		mw.RequestIDMiddleware,
		mw.RateLimitMiddleware(rateLimit),
		mw.RequestLoggerMiddleware,
	)

	router := routerStack(http.NewServeMux())
	router.Handle("/", http.FileServer(http.Dir("./public")))
	return middlewareStack(router)
}

// Run - Start the API server
func (s *APIServer) Run() error {
	server := http.Server{
		Addr:    s.Address,
		Handler: s.Setup(),
	}

	if s.UsingUDS {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-c
			os.Remove(s.Address)
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
		log.Printf("API Server listening on %s", s.Address)
		return server.Serve(socket)
	} else {
		log.Printf("API Server listening on %s", s.Address)
		return server.ListenAndServe()
	}
}
