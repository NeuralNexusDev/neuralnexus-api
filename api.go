package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mw "github.com/NeuralNexusDev/neuralnexus-api/middleware"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/account"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/link"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/ratelimit"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/session"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/auth/user"
	bng "github.com/NeuralNexusDev/neuralnexus-api/modules/bee_name_generator"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/config"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/database"
	ds "github.com/NeuralNexusDev/neuralnexus-api/modules/datastore"
	nds "github.com/NeuralNexusDev/neuralnexus-api/modules/datastore/numbers"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/discord"
	gss "github.com/NeuralNexusDev/neuralnexus-api/modules/game_server_status"
	mcs "github.com/NeuralNexusDev/neuralnexus-api/modules/mcstatus"
	mc "github.com/NeuralNexusDev/neuralnexus-api/modules/minecraft"
	petpics "github.com/NeuralNexusDev/neuralnexus-api/modules/pet_pictures"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/projects"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/rbac"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/switchboard"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/teapot"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/twitch"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
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
func ApplyRoutes(
	mux *http.ServeMux,
	cfg *config.Config,
	nndb *pgxpool.Pool,
	rdb *redis.Client,
	session session.Service,
	accountStore account.Store,
	linkAccountStore link.Store,
	oauthTokenStore link.OAuthTokenStore,
	rateLimit ratelimit.Service) *http.ServeMux {
	mwAuth := mw.Auth(session)

	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		log.Fatal("DATABASE_URL is not set")
		return nil
	}

	dbUrl2 := os.Getenv("DATABASE_URL_2")
	if dbUrl2 == "" {
		log.Fatal("DATABASE_URL_2 is not set")
		return nil
	}

	// --------------- Auth ---------------
	rbacService := rbac.NewService(rbac.NewStore(nndb))
	settingsStore := account.NewSettingsStore(nndb)
	acct := account.NewService(accountStore, settingsStore, rbacService)
	linkingService := link.NewService(linkAccountStore, accountStore)
	usr := user.NewService(accountStore, linkAccountStore, linkingService, settingsStore, rbacService)

	loginRateLimit := mw.RateLimitMiddleware(rateLimit, "login", 5, 5)

	mux.Handle("POST /api/v1/auth/login", loginRateLimit(account.LoginHandler(acct, session)))
	mux.Handle("POST /api/v1/auth/logout", loginRateLimit(mwAuth(account.LogoutHandler(session))))

	mux.Handle("/api/oauth", loginRateLimit(link.OAuthHandler(cfg, acct, linkAccountStore, session, cfg.SiteUrl)))
	mux.Handle("/api/openid", loginRateLimit(link.OpenIDHandler(acct, linkAccountStore, session, cfg.SiteUrl)))

	mux.Handle("GET /api/v1/users", mwAuth(user.ListUsersHandler(usr)))
	mux.Handle("GET /api/v1/users/{user_id}", mwAuth(user.GetUserHandler(usr)))
	mux.Handle("GET /api/v1/users/me", mwAuth(mw.SelfUserID(user.GetUserHandler(usr))))
	mux.Handle("GET /api/v1/users/{user_id}/permissions", mwAuth(user.GetUserPermissionsHandler(usr)))
	mux.Handle("GET /api/v1/users/me/permissions", mwAuth(mw.SelfUserID(user.GetUserPermissionsHandler(usr))))
	mux.Handle("GET /api/v1/users/{platform}/{platform_id}", mwAuth(user.GetUserFromPlatformHandler(usr)))
	mux.Handle("PUT /api/v1/users/{user_id}", mwAuth(user.UpdateUserHandler(usr)))
	mux.Handle("PUT /api/v1/users/{platform}/{platform_id}", mwAuth(user.UpdateUserFromPlatformHandler(usr)))
	mux.Handle("GET /api/v1/users/{user_id}/links", mwAuth(link.GetUserLinkedAccountsHandler(linkingService)))
	mux.Handle("GET /api/v1/users/me/links", mwAuth(mw.SelfUserID(link.GetUserLinkedAccountsHandler(linkingService))))
	mux.Handle("DELETE /api/v1/users/{user_id}/link/{platform}", mwAuth(link.UnlinkPlatformHandler(linkingService)))
	mux.Handle("DELETE /api/v1/users/me/link/{platform}", mwAuth(mw.SelfUserID(link.UnlinkPlatformHandler(linkingService))))
	mux.Handle("PATCH /api/v1/users/{user_id}/link/{platform}", mwAuth(link.SetPlatformLoginEnabledHandler(linkingService)))
	mux.Handle("PATCH /api/v1/users/me/link/{platform}", mwAuth(mw.SelfUserID(link.SetPlatformLoginEnabledHandler(linkingService))))
	mux.Handle("GET /api/v1/users/{user_id}/settings", mwAuth(user.GetAccountSettingsHandler(usr)))
	mux.Handle("GET /api/v1/users/me/settings", mwAuth(mw.SelfUserID(user.GetAccountSettingsHandler(usr))))
	mux.Handle("PATCH /api/v1/users/{user_id}/settings", mwAuth(user.UpdateAccountSettingsHandler(usr)))
	mux.Handle("PATCH /api/v1/users/me/settings", mwAuth(mw.SelfUserID(user.UpdateAccountSettingsHandler(usr))))
	// mux.HandleFunc("DELETE /api/v1/users/{user_id}", mwAuth(user.DeleteUserHandler(gssService)))

	mux.Handle("GET /api/v1/roles", mwAuth(rbac.ListRolesHandler(rbacService)))
	mux.Handle("POST /api/v1/roles", mwAuth(rbac.CreateRoleHandler(rbacService)))
	mux.Handle("GET /api/v1/roles/{id}", mwAuth(rbac.GetRoleHandler(rbacService)))
	mux.Handle("GET /api/v1/roles/name/{name}", mwAuth(rbac.GetRoleByNameHandler(rbacService)))
	mux.Handle("PATCH /api/v1/roles/{id}", mwAuth(rbac.UpdateRoleHandler(rbacService)))
	mux.Handle("DELETE /api/v1/roles/{id}", mwAuth(rbac.DeleteRoleHandler(rbacService)))
	mux.Handle("PUT /api/v1/roles/{id}/permissions/{permission_id}", mwAuth(rbac.AttachPermissionHandler(rbacService)))
	mux.Handle("DELETE /api/v1/roles/{id}/permissions/{permission_id}", mwAuth(rbac.DetachPermissionHandler(rbacService)))
	mux.Handle("GET /api/v1/permissions", mwAuth(rbac.ListPermissionsHandler(rbacService)))
	mux.Handle("POST /api/v1/permissions", mwAuth(rbac.CreatePermissionHandler(rbacService)))
	mux.Handle("GET /api/v1/permissions/{id}", mwAuth(rbac.GetPermissionHandler(rbacService)))
	mux.Handle("GET /api/v1/permissions/node/{node}", mwAuth(rbac.GetPermissionByNodeHandler(rbacService)))
	mux.Handle("DELETE /api/v1/permissions/{id}", mwAuth(rbac.DeletePermissionHandler(rbacService)))

	// --------------- Bee Name Generator ---------------
	bngStore := bng.NewStore(database.GetDB(dbUrl + "/bee_name_generator"))

	mux.Handle("GET /api/v1/bee-name-generator/name", bng.GetBeeNameHandler(bngStore))
	mux.Handle("POST /api/v1/bee-name-generator/name/{name}", mwAuth(bng.UploadBeeNameHandler(bngStore)))
	mux.Handle("DELETE /api/v1/bee-name-generator/name/{name}", mwAuth(bng.DeleteBeeNameHandler(bngStore)))
	mux.Handle("POST /api/v1/bee-name-generator/suggestion/{name}", bng.SubmitBeeNameHandler(bngStore))
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

	// --------------- Discord ---------------
	verify := mw.VerifyEd25519Middleware(cfg)
	mux.Handle("POST /api/discord/webhook", verify(discord.HandleDiscordWebhook()))
	mux.Handle("POST /api/discord/interactions", verify(discord.HandleDiscordInteraction()))

	// --------------- Game Server Status ---------------
	gssService := gss.NewService()
	mux.Handle("GET /api/v1/game-server-status/{game}", gss.GameServerStatusHandler(gssService))
	mux.Handle("GET /api/v1/game-server-status/simple/{game}", gss.SimpleGameServerStatus(gssService))

	// --------------- Minecraft ---------------
	endpoint := os.Getenv("S3_API_URL")
	if endpoint == "" {
		log.Fatal("S3_API_URL environment variable not set")
		return nil
	}
	accessKey := os.Getenv("S3_ACCESS_KEY_MCA_TEXTURE")
	if accessKey == "" {
		log.Fatal("S3_ACCESS_KEY_MCA_TEXTURE environment variable not set")
		return nil
	}
	secretKey := os.Getenv("S3_SECRET_KEY_MCA_TEXTURE")
	if secretKey == "" {
		log.Fatal("S3_SECRET_KEY_MCA_TEXTURE environment variable not set")
		return nil
	}
	mcStore := mc.NewStore(
		database.GetDB(dbUrl2+"/archive?sslmode=require"), rdb,
		database.GetS3(endpoint, accessKey, secretKey))
	mcService := mc.NewService(mcStore, nil, "https://"+endpoint+"/"+mc.S3Bucket+"/")

	mux.Handle("GET /api/v1/mc/mojang/lookup/name/{name}", mc.GetMojangPlayerByNameHandler(mcService))
	mux.Handle("GET /api/v1/mc/mojang/lookup/{uuid}", mc.GetMojangPlayerByUUIDHandler(mcService))
	mux.Handle("POST /api/v1/mc/mojang/lookup/bulk/byname", mc.GetMojangPlayersByNamesHandler(mcService))
	mux.Handle("GET /api/v1/mc/mojang/profile/{uuid}", mc.GetMojangProfileHandler(mcService))
	mux.Handle("GET /api/v1/mc/profile/{uuid}", mc.GetProfileHandler(mcService))
	mux.Handle("GET /api/v1/mc/profile/name/{name}", mc.GetProfileByNameHandler(mcService))
	mux.Handle("GET /api/v1/mc/profile/bedrock/{uuid}", mc.GetGeyserProfileHandler(mcService))
	mux.Handle("GET /api/v1/mc/profile/bedrock/name/{name}", mc.GetGeyserProfileByNameHandler(mcService))
	mux.Handle("GET /api/v1/mc/texture/{hash}", mc.GetTextureHandler(mcService))
	mux.Handle("GET /api/v1/mc/geyser/xuid/{gamertag}", mc.GetGeyserXUIDHandler(mcService))
	mux.Handle("GET /api/v1/mc/geyser/skin/{xuid}", mc.GetGeyserSkinHandler(mcService))

	// --------------- Minecraft Status ---------------
	mcsService := mcs.NewService()
	mux.Handle("GET /api/v1/mcstatus/{host}", mcs.ServerStatusHandler(mcsService))
	mux.Handle("GET /api/v1/mcstatus/icon/{host}", mcs.IconHandler(mcsService))
	mux.Handle("GET /api/v1/mcstatus/simple/{host}", mcs.SimpleStatusHandler(mcsService))

	// --------------- Pet Pictures ---------------
	petStore := petpics.NewStore(database.GetDB(dbUrl + "/pet_pictures"))
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
	twitchStore := twitch.NewStore(database.GetDB(dbUrl + "/twitch"))
	twitchService := twitch.NewService(twitchStore)
	mux.HandleFunc("POST /api/twitch/eventsub", twitch.HandleEventSub(cfg.Twitch, twitchService, oauthTokenStore, linkAccountStore))

	// --------------- Health Check ---------------
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return mux
}

// Setup - Setup the API server
func (s *APIServer) Setup() http.Handler {
	cfg := config.FromEnv()

	dbUrl := os.Getenv("DATABASE_URL")
	if dbUrl == "" {
		log.Fatal("DATABASE_URL is not set")
		return nil
	}

	siteUrl := os.Getenv("NN_SITE_URL")
	if siteUrl == "" {
		log.Fatal("NN_SITE_URL is not set")
		return nil
	}

	db := database.GetDB(dbUrl + "/neuralnexus")
	rdb := database.GetRedis()
	sess := session.NewService(cfg, session.NewStore(db, rdb))
	accountStore := account.NewStore(db)
	linkAccountStore := link.NewStore(db)
	oauthTokenStore := link.NewOAuthTokenStore(db)
	rateLimit := ratelimit.NewService(ratelimit.NewStore(rdb))

	middlewareStack := mw.CreateStack(
		mw.NoSniffMiddleware,
		cors.New(cors.Options{
			AllowedOrigins:   []string{siteUrl},
			AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
			AllowedHeaders:   []string{"Content-Type", "Authorization"},
			AllowCredentials: true,
		}).Handler,
		mw.IPMiddleware,
		mw.RequestIDMiddleware,
		mw.RecoveryMiddleware,
		mw.SessionMiddleware(sess),
		mw.RateLimitMiddleware(rateLimit, "default", 300, 60),
		mw.RequestLoggerMiddleware,
	)

	router := ApplyRoutes(http.NewServeMux(), cfg, db, rdb, sess, accountStore, linkAccountStore, oauthTokenStore, rateLimit)

	// --------------- Static Files ---------------
	router.Handle("/", http.FileServer(http.Dir("./public")))
	return middlewareStack(router)
}

// Run - Start the API server
func (s *APIServer) Run() error {
	server := http.Server{
		Addr:              s.Address,
		Handler:           s.Setup(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute,
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
	}
	log.Printf("API Server listening on %s", s.Address)
	return server.ListenAndServe()
}
