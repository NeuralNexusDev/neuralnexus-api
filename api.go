package main

import (
	"log"
	"net/http"

	"github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/beenamegenerator"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/cct_turtle"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/mcstatus"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/projects"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/switchboard"
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

	router := http.NewServeMux()

	// -------------- Routes --------------

	// -------------- Bee Name Generator --------------
	router.HandleFunc("GET /bee-name-generator", beenamegenerator.GetRoot)
	router.HandleFunc("GET /bee-name-generator/name", beenamegenerator.GetBeeNameHandler)
	router.HandleFunc("POST /bee-name-generator/name", beenamegenerator.UploadBeeNameHandler)
	router.HandleFunc("POST /bee-name-generator/name/{name}", beenamegenerator.UploadBeeNameHandler)
	router.HandleFunc("DELETE /bee-name-generator/name", beenamegenerator.DeleteBeeNameHandler)
	router.HandleFunc("DELETE /bee-name-generator/name/{name}", beenamegenerator.DeleteBeeNameHandler)
	router.HandleFunc("POST /bee-name-generator/suggestion", beenamegenerator.SubmitBeeNameHandler)
	router.HandleFunc("POST /bee-name-generator/suggestion/{name}", beenamegenerator.SubmitBeeNameHandler)
	router.HandleFunc("GET /bee-name-generator/suggestion", beenamegenerator.GetBeeNameSuggestionsHandler)
	router.HandleFunc("GET /bee-name-generator/suggestion/{amount}", beenamegenerator.GetBeeNameSuggestionsHandler)
	router.HandleFunc("PUT /bee-name-generator/suggestion", beenamegenerator.AcceptBeeNameSuggestionHandler)
	router.HandleFunc("PUT /bee-name-generator/suggestion/{name}", beenamegenerator.AcceptBeeNameSuggestionHandler)
	router.HandleFunc("DELETE /bee-name-generator/suggestion", beenamegenerator.RejectBeeNameSuggestionHandler)
	router.HandleFunc("DELETE /bee-name-generator/suggestion/{name}", beenamegenerator.RejectBeeNameSuggestionHandler)

	// -------------- CCT Turtle --------------
	router.HandleFunc("GET /ws/v1/cct-turtle/{label}", cct_turtle.WebSocketTurtleHandler)
	// e.GET("/cct-turtle/status", cct_turtle.GetTurtleStatus)
	// e.GET("/cct-turtle/status/:label", cct_turtle.GetTurtleStatus)
	router.HandleFunc("GET /cct-turtle/startup.lua", cct_turtle.GetTurtleCode)
	router.HandleFunc("GET /cct-turtle/updating_startup.lua", cct_turtle.GetTurtleUpdatingCode)
	router.HandleFunc("GET /cct-turtle/forward", cct_turtle.MoveTurtleForward)
	router.HandleFunc("GET /cct-turtle/forward/{label}", cct_turtle.MoveTurtleForward)
	router.HandleFunc("GET /cct-turtle/back", cct_turtle.MoveTurtleBackward)
	router.HandleFunc("GET /cct-turtle/back/{label}", cct_turtle.MoveTurtleBackward)
	router.HandleFunc("GET /cct-turtle/up", cct_turtle.MoveTurtleUp)
	router.HandleFunc("GET /cct-turtle/up/{label}", cct_turtle.MoveTurtleUp)
	router.HandleFunc("GET /cct-turtle/down", cct_turtle.MoveTurtleDown)
	router.HandleFunc("GET /cct-turtle/down/{label}", cct_turtle.MoveTurtleDown)
	router.HandleFunc("GET /cct-turtle/left", cct_turtle.TurnTurtleLeft)
	router.HandleFunc("GET /cct-turtle/left/{label}", cct_turtle.TurnTurtleLeft)
	router.HandleFunc("GET /cct-turtle/right", cct_turtle.TurnTurtleRight)
	router.HandleFunc("GET /cct-turtle/right/{label}", cct_turtle.TurnTurtleRight)
	router.HandleFunc("GET /cct-turtle/dig", cct_turtle.DigTurtle)
	router.HandleFunc("GET /cct-turtle/dig/{label}", cct_turtle.DigTurtle)
	router.HandleFunc("GET /cct-turtle/dig-up", cct_turtle.DigTurtleUp)
	router.HandleFunc("GET /cct-turtle/dig-up/{label}", cct_turtle.DigTurtleUp)
	router.HandleFunc("GET /cct-turtle/dig-down", cct_turtle.DigTurtleDown)
	router.HandleFunc("GET /cct-turtle/dig-down/{label}", cct_turtle.DigTurtleDown)

	// -------------- MC Status --------------
	mcstatusRouter := http.NewServeMux()
	mcstatusRouter.Handle("/mcstatus/", http.StripPrefix("/mcstatus", router))
	mcstatusRouter.HandleFunc("GET /", mcstatus.GetRoot)
	mcstatusRouter.HandleFunc("GET /{address}", mcstatus.GetServerStatus)
	mcstatusRouter.HandleFunc("GET /icon/{address}", mcstatus.GetIcon)

	// -------------- Projects --------------
	router.HandleFunc("GET /projects/releases/{group}/{project}", projects.GetReleasesHandler)

	// -------------- Switchboard --------------
	// router.HandleFunc("GET /ws/v1/switchboard/relay", switchboard.WebSocketRelayHandler)
	router.HandleFunc("GET /websocket/{id}", switchboard.WebSocketRelayHandler)

	v1 := http.NewServeMux()
	v1.Handle("/api/v1/", http.StripPrefix("/api/v1", router))

	middlewareChain := middleware.CreateStack(
		middleware.RequestLoggerMiddleware,
		cors.Default().Handler,
	)

	server := http.Server{
		Addr:    s.Address,
		Handler: middlewareChain(router),
	}

	log.Printf("API Server listening on %s", s.Address)
	return server.ListenAndServe()
}
