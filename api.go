package main

import (
	"github.com/NeuralNexusDev/neuralnexus-api/modules/twitch"
	"github.com/jackc/pgx/v5/pgxpool"
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
	bng "github.com/NeuralNexusDev/neuralnexus-api/modules/bee_name_generator"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	ds "github.com/NeuralNexusDev/neuralnexus-api/modules/datastore"
	nds "github.com/NeuralNexusDev/neuralnexus-api/modules/datastore/numbers"
	gss "github.com/NeuralNexusDev/neuralnexus-api/modules/game_server_status"
	mcs "github.com/NeuralNexusDev/neuralnexus-api/modules/mcstatus"
	petpics "github.com/NeuralNexusDev/neuralnexus-api/modules/pet_pictures"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/projects"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/switchboard"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/teapot"
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
func ApplyRoutes(mux *http.ServeMux, nndb *pgxpool.Pool, session auth.SessionService, authStore auth.Store, rateLimit auth.RateLimitService) *http.ServeMux {
	mwAuth := mw.Auth(session)

	// --------------- Auth ---------------
	account := auth.NewAccountService(authStore)
	user := auth.NewUserService(authStore)

	loginRateLimit := mw.RateLimitMiddleware(rateLimit, "login", 5, 5)

	mux.Handle("POST /api/v1/auth/login", loginRateLimit(authroutes.LoginHandler(account, session)))
	mux.Handle("POST /api/v1/auth/logout", loginRateLimit(mwAuth(authroutes.LogoutHandler(session))))

	mux.Handle("/api/oauth", loginRateLimit(authroutes.OAuthHandler(account, authStore.LinkAccount(), session)))

	mux.Handle("GET /api/v1/users/{user_id}", mwAuth(authroutes.GetUserHandler(user)))
	mux.Handle("GET /api/v1/users/{user_id}/permissions", mwAuth(authroutes.GetUserPermissionsHandler(user)))
	mux.Handle("GET /api/v1/users/{platform}/{platform_id}", mwAuth(authroutes.GetUserFromPlatformHandler(user)))
	mux.Handle("PUT /api/v1/users/{user_id}", mwAuth(authroutes.UpdateUserHandler(user)))
	mux.Handle("PUT /api/v1/users/{platform}/{platform_id}", mwAuth(authroutes.UpdateUserFromPlatformHandler(user)))
	// mux.HandleFunc("DELETE /api/v1/users/{user_id}", mwAuth(authroutes.DeleteUserHandler(gssService)))

	// --------------- Bee Name Generator ---------------
	bngStore := bng.NewStore(database.GetDB("bee_name_generator"))

	mux.Handle("GET /api/v1/bee-name-generator/name", bng.GetBeeNameHandler(bngStore))
	mux.Handle("POST /api/v1/bee-name-generator/name/{name}", mwAuth(bng.UploadBeeNameHandler(bngStore)))
	mux.Handle("DELETE /api/v1/bee-name-generator/name/{name}", mwAuth(bng.DeleteBeeNameHandler(bngStore)))
	mux.Handle("POST /api/v1/bee-name-generator/suggestion/{name}", bng.SubmitBeeNameHandler(bngStore))
	mux.Handle("GET /api/v1/bee-name-generator/suggestion", mwAuth(bng.GetBeeNameSuggestionsHandler(bngStore)))
	mux.Handle("GET /api/v1/bee-name-generator/suggestion/{amount}", mwAuth(bng.GetBeeNameSuggestionsHandler(bngStore)))
	mux.Handle("PUT /api/v1/bee-name-generator/suggestion/{name}", mwAuth(bng.AcceptBeeNameSuggestionHandler(bngStore)))
	mux.Handle("DELETE /api/v1/bee-name-generator/suggestion/{name}", mwAuth(bng.RejectBeeNameSuggestionHandler(bngStore)))

	// --------------- Data Store ---------------
	dsStore := ds.NewStore(nndb)
	dsService := ds.NewService(dsStore)

	mux.Handle("POST /api/v1/datastore", mwAuth(ds.CreateDataStoreHandler(dsService)))
	mux.Handle("GET /api/v1/datastore", ds.ReadDataStoreHandler(dsService))
	mux.Handle("PUT /api/v1/datastore", mwAuth(ds.UpdateDataStoreHandler(dsService)))
	mux.Handle("DELETE /api/v1/datastore", mwAuth(ds.DeleteDataStoreHandler(dsService)))

	// --------------- Numbers Data Store ---------------
	nStore := nds.NewStore(nndb)
	nService := nds.NewService(nStore)

	mux.Handle("POST /api/v1/datastore/number", mwAuth(nds.CreateNumberHandler(nService)))
	mux.Handle("GET /api/v1/datastore/number", nds.ReadNumberHandler(nService))
	mux.Handle("PUT /api/v1/datastore/number", mwAuth(nds.UpdateNumberHandler(nService)))
	mux.Handle("DELETE /api/v1/datastore/number", mwAuth(nds.DeleteNumberHandler(nService)))

	// --------------- Game Server Status ---------------
	gssService := gss.NewService()
	mux.Handle("GET /api/v1/game-server-status/{game}", gss.GameServerStatusHandler(gssService))
	mux.Handle("GET /api/v1/game-server-status/simple/{game}", gss.SimpleGameServerStatus(gssService))

	// --------------- Minecraft Status ---------------
	mcsService := mcs.NewService()
	mux.Handle("GET /api/v1/mcstatus/{host}", mcs.ServerStatusHandler(mcsService))
	mux.Handle("GET /api/v1/mcstatus/icon/{host}", mcs.IconHandler(mcsService))
	mux.Handle("GET /api/v1/mcstatus/simple/{host}", mcs.SimpleStatusHandler(mcsService))

	// --------------- Pet Pictures ---------------
	petStore := petpics.NewStore(database.GetDB("pet_pictures"))
	petService := petpics.NewService(petStore)

	mux.Handle("POST /api/v1/pet-pictures/pets/{name}", mwAuth(petpics.CreatePetHandler(petService)))
	mux.Handle("POST /api/v1/pet-pictures/pets", mwAuth(petpics.CreatePetHandler(petService)))
	mux.Handle("GET /api/v1/pet-pictures/pets/{id}", petpics.GetPetHandler(petService))
	mux.Handle("GET /api/v1/pet-pictures/pets", petpics.GetPetHandler(petService))
	mux.Handle("PUT /api/v1/pet-pictures/pets", mwAuth(petpics.UpdatePetHandler(petService)))
	mux.Handle("GET /api/v1/pet-pictures/pictures/random", petpics.GetRandPetPictureByNameHandler(petService))
	mux.Handle("GET /api/v1/pet-pictures/pictures/{id}", petpics.GetPetPictureHandler(petService))
	mux.Handle("GET /api/v1/pet-pictures/pictures", petpics.GetPetPictureHandler(petService))
	mux.Handle("PUT /api/v1/pet-pictures/pictures", mwAuth(petpics.UpdatePetPictureHandler(petService)))
	mux.Handle("DELETE /api/v1/pet-pictures/pictures/{id}", mwAuth(petpics.DeletePetPictureHandler(petService)))
	mux.Handle("DELETE /api/v1/pet-pictures/pictures", mwAuth(petpics.DeletePetPictureHandler(petService)))

	// --------------- Projects ---------------
	mux.HandleFunc("GET /api/v1/projects/releases/{group}/{project}", projects.GetReleasesHandler)

	// --------------- Switchboard ---------------
	// mux.HandleFunc("GET /ws/v1/switchboard/relay", switchboard.ebSocketRelayHandler)
	mux.HandleFunc("GET /websocket/{id}", switchboard.WebSocketRelayHandler)

	// --------------- Teapot ---------------
	mux.HandleFunc("GET /api/v1/teapot", teapot.HandleTeapot)

	// --------------- Twitch ---------------
	twitchStore := twitch.NewStore(database.GetDB("twitch"))
	twitchService := twitch.NewService(twitchStore)
	mux.HandleFunc("POST /api/twitch/eventsub", twitch.HandleEventSub(twitchService, authStore.OAuthToken(), authStore.LinkAccount()))

	// --------------- Health Check ---------------
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return mux
}

// Setup - Setup the API server
func (s *APIServer) Setup() http.Handler {
	db := database.GetDB("neuralnexus")
	rdb := database.GetRedis()
	authStore := auth.NewStore(db, rdb)
	session := auth.NewSessionService(authStore)
	rateLimit := auth.NewRateLimitService(authStore)

	middlewareStack := mw.CreateStack(
		cors.AllowAll().Handler,
		mw.IPMiddleware,
		mw.SessionMiddleware(session),
		mw.RequestIDMiddleware,
		mw.RateLimitMiddleware(rateLimit, "default", 300, 60),
		mw.RequestLoggerMiddleware,
	)

	router := ApplyRoutes(http.NewServeMux(), db, session, authStore, rateLimit)

	// --------------- Static Files ---------------
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
