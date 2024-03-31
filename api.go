package main

import (
	"log"
	"net/http"

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
	router.HandleFunc("GET /api/v1/bee-name-generator", beenamegenerator.GetRoot)
	router.HandleFunc("GET /api/v1/bee-name-generator/name", beenamegenerator.GetBeeNameHandler)
	router.HandleFunc("POST /api/v1/bee-name-generator/name", beenamegenerator.UploadBeeNameHandler)
	router.HandleFunc("POST /api/v1/bee-name-generator/name/{name}", beenamegenerator.UploadBeeNameHandler)
	router.HandleFunc("DELETE /api/v1/bee-name-generator/name", beenamegenerator.DeleteBeeNameHandler)
	router.HandleFunc("DELETE /api/v1/bee-name-generator/name/{name}", beenamegenerator.DeleteBeeNameHandler)
	router.HandleFunc("POST /api/v1/bee-name-generator/suggestion", beenamegenerator.SubmitBeeNameHandler)
	router.HandleFunc("POST /api/v1/bee-name-generator/suggestion/{name}", beenamegenerator.SubmitBeeNameHandler)
	router.HandleFunc("GET /api/v1/bee-name-generator/suggestion", beenamegenerator.GetBeeNameSuggestionsHandler)
	router.HandleFunc("GET /api/v1/bee-name-generator/suggestion/{amount}", beenamegenerator.GetBeeNameSuggestionsHandler)
	router.HandleFunc("PUT /api/v1/bee-name-generator/suggestion", beenamegenerator.AcceptBeeNameSuggestionHandler)
	router.HandleFunc("PUT /api/v1/bee-name-generator/suggestion/{name}", beenamegenerator.AcceptBeeNameSuggestionHandler)
	router.HandleFunc("DELETE /api/v1/bee-name-generator/suggestion", beenamegenerator.RejectBeeNameSuggestionHandler)
	router.HandleFunc("DELETE /api/v1/bee-name-generator/suggestion/{name}", beenamegenerator.RejectBeeNameSuggestionHandler)

	// -------------- CCT Turtle --------------
	router.HandleFunc("GET /ws/v1/cct-turtle/{label}", cct_turtle.WebSocketTurtleHandler)
	// e.GET("/api/v1/cct-turtle/status", cct_turtle.GetTurtleStatus)
	// e.GET("/api/v1/cct-turtle/status/:label", cct_turtle.GetTurtleStatus)
	router.HandleFunc("GET /api/v1/cct-turtle/startup.lua", cct_turtle.GetTurtleCode)
	router.HandleFunc("GET /api/v1/cct-turtle/updating_startup.lua", cct_turtle.GetTurtleUpdatingCode)
	router.HandleFunc("GET /api/v1/cct-turtle/forward", cct_turtle.MoveTurtleForward)
	router.HandleFunc("GET /api/v1/cct-turtle/forward/{label}", cct_turtle.MoveTurtleForward)
	router.HandleFunc("GET /api/v1/cct-turtle/back", cct_turtle.MoveTurtleBackward)
	router.HandleFunc("GET /api/v1/cct-turtle/back/{label}", cct_turtle.MoveTurtleBackward)
	router.HandleFunc("GET /api/v1/cct-turtle/up", cct_turtle.MoveTurtleUp)
	router.HandleFunc("GET /api/v1/cct-turtle/up/{label}", cct_turtle.MoveTurtleUp)
	router.HandleFunc("GET /api/v1/cct-turtle/down", cct_turtle.MoveTurtleDown)
	router.HandleFunc("GET /api/v1/cct-turtle/down/{label}", cct_turtle.MoveTurtleDown)
	router.HandleFunc("GET /api/v1/cct-turtle/left", cct_turtle.TurnTurtleLeft)
	router.HandleFunc("GET /api/v1/cct-turtle/left/{label}", cct_turtle.TurnTurtleLeft)
	router.HandleFunc("GET /api/v1/cct-turtle/right", cct_turtle.TurnTurtleRight)
	router.HandleFunc("GET /api/v1/cct-turtle/right/{label}", cct_turtle.TurnTurtleRight)
	router.HandleFunc("GET /api/v1/cct-turtle/dig", cct_turtle.DigTurtle)
	router.HandleFunc("GET /api/v1/cct-turtle/dig/{label}", cct_turtle.DigTurtle)
	router.HandleFunc("GET /api/v1/cct-turtle/dig-up", cct_turtle.DigTurtleUp)
	router.HandleFunc("GET /api/v1/cct-turtle/dig-up/{label}", cct_turtle.DigTurtleUp)
	router.HandleFunc("GET /api/v1/cct-turtle/dig-down", cct_turtle.DigTurtleDown)
	router.HandleFunc("GET /api/v1/cct-turtle/dig-down/{label}", cct_turtle.DigTurtleDown)

	// -------------- MC Status --------------
	router.HandleFunc("GET /api/v1/mcstatus", mcstatus.GetRoot)
	router.HandleFunc("GET /api/v1/mcstatus/{address}", mcstatus.GetServerStatus)
	router.HandleFunc("GET /api/v1/mcstatus/icon/{address}", mcstatus.GetIcon)

	// -------------- Projects --------------
	router.HandleFunc("GET /api/v1/projects/releases/{group}/{project}", projects.GetReleasesHandler)

	// -------------- Switchboard --------------
	// router.HandleFunc("GET /ws/v1/switchboard/relay", switchboard.WebSocketRelayHandler)
	router.HandleFunc("GET /websocket/{id}", switchboard.WebSocketRelayHandler)

	middlewareChain := MiddlewareChain(RequestLoggerMiddleware)

	server := http.Server{
		Addr:    s.Address,
		Handler: cors.Default().Handler(middlewareChain(router)),
	}

	log.Printf("API Server listening on %s", s.Address)
	return server.ListenAndServe()
}

// -------------- Middleware --------------
type Middleware func(http.Handler) http.HandlerFunc

func MiddlewareChain(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.HandlerFunc {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next.ServeHTTP
	}
}

// RequestLoggerMiddleware - Log all requests
func RequestLoggerMiddleware(next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
