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
	"github.com/NeuralNexusDev/neuralnexus-api/public"
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

	// --------------- Auth ---------------
	rbacService := rbac.NewService(rbac.NewStore(nndb))
	settingsStore := account.NewSettingsStore(nndb)
	acct := account.NewService(accountStore, settingsStore, rbacService)
	linkingService := link.NewService(linkAccountStore, accountStore)
	usr := user.NewService(accountStore, linkAccountStore, linkingService, settingsStore, rbacService)

	loginRateLimit := mw.RateLimitMiddleware(cfg.Auth.RateLimit, rateLimit, "login", 5, 5)
	accountHandler := account.NewHandler(cfg.Auth.Account, acct, session)

	mux.Handle("POST /api/v1/auth/login", loginRateLimit(http.HandlerFunc(accountHandler.Login)))
	mux.Handle("POST /api/v1/auth/logout", loginRateLimit(mwAuth(http.HandlerFunc(accountHandler.Logout))))

	linkHandler := link.NewHandler(cfg, acct, linkAccountStore, session, linkingService, cfg.SiteUrl)
	mux.Handle("/api/oauth", loginRateLimit(http.HandlerFunc(linkHandler.OAuth)))
	mux.Handle("/api/openid", loginRateLimit(http.HandlerFunc(linkHandler.OpenID)))

	userHandler := user.NewHandler(usr)
	mux.Handle("GET /api/v1/users", mwAuth(http.HandlerFunc(userHandler.ListUsers)))
	mux.Handle("GET /api/v1/users/{user_id}", mwAuth(http.HandlerFunc(userHandler.GetUser)))
	mux.Handle("GET /api/v1/users/me", mwAuth(mw.SelfUserID(http.HandlerFunc(userHandler.GetUser))))
	mux.Handle("GET /api/v1/users/{user_id}/permissions", mwAuth(http.HandlerFunc(userHandler.GetUserPermissions)))
	mux.Handle("GET /api/v1/users/me/permissions", mwAuth(mw.SelfUserID(http.HandlerFunc(userHandler.GetUserPermissions))))
	mux.Handle("GET /api/v1/users/{platform}/{platform_id}", mwAuth(http.HandlerFunc(userHandler.GetUserFromPlatform)))
	mux.Handle("PUT /api/v1/users/{user_id}", mwAuth(http.HandlerFunc(userHandler.UpdateUser)))
	mux.Handle("PUT /api/v1/users/{platform}/{platform_id}", mwAuth(http.HandlerFunc(userHandler.UpdateUserFromPlatform)))
	mux.Handle("GET /api/v1/users/{user_id}/links", mwAuth(http.HandlerFunc(linkHandler.GetUserLinkedAccounts)))
	mux.Handle("GET /api/v1/users/me/links", mwAuth(mw.SelfUserID(http.HandlerFunc(linkHandler.GetUserLinkedAccounts))))
	mux.Handle("DELETE /api/v1/users/{user_id}/link/{platform}", mwAuth(http.HandlerFunc(linkHandler.UnlinkPlatform)))
	mux.Handle("DELETE /api/v1/users/me/link/{platform}", mwAuth(mw.SelfUserID(http.HandlerFunc(linkHandler.UnlinkPlatform))))
	mux.Handle("PATCH /api/v1/users/{user_id}/link/{platform}", mwAuth(http.HandlerFunc(linkHandler.SetPlatformLoginEnabled)))
	mux.Handle("PATCH /api/v1/users/me/link/{platform}", mwAuth(mw.SelfUserID(http.HandlerFunc(linkHandler.SetPlatformLoginEnabled))))
	mux.Handle("GET /api/v1/users/{user_id}/settings", mwAuth(http.HandlerFunc(userHandler.GetAccountSettings)))
	mux.Handle("GET /api/v1/users/me/settings", mwAuth(mw.SelfUserID(http.HandlerFunc(userHandler.GetAccountSettings))))
	mux.Handle("PATCH /api/v1/users/{user_id}/settings", mwAuth(http.HandlerFunc(userHandler.UpdateAccountSettings)))
	mux.Handle("PATCH /api/v1/users/me/settings", mwAuth(mw.SelfUserID(http.HandlerFunc(userHandler.UpdateAccountSettings))))
	// mux.HandleFunc("DELETE /api/v1/users/{user_id}", mwAuth(http.HandlerFunc(userHandler.DeleteUser)))

	rbacHandler := rbac.NewHandler(rbacService)
	mux.Handle("GET /api/v1/roles", mwAuth(http.HandlerFunc(rbacHandler.ListRoles)))
	mux.Handle("POST /api/v1/roles", mwAuth(http.HandlerFunc(rbacHandler.CreateRole)))
	mux.Handle("GET /api/v1/roles/{id}", mwAuth(http.HandlerFunc(rbacHandler.GetRole)))
	mux.Handle("GET /api/v1/roles/name/{name}", mwAuth(http.HandlerFunc(rbacHandler.GetRoleByName)))
	mux.Handle("PATCH /api/v1/roles/{id}", mwAuth(http.HandlerFunc(rbacHandler.UpdateRole)))
	mux.Handle("DELETE /api/v1/roles/{id}", mwAuth(http.HandlerFunc(rbacHandler.DeleteRole)))
	mux.Handle("PUT /api/v1/roles/{id}/permissions/{permission_id}", mwAuth(http.HandlerFunc(rbacHandler.AttachPermission)))
	mux.Handle("DELETE /api/v1/roles/{id}/permissions/{permission_id}", mwAuth(http.HandlerFunc(rbacHandler.DetachPermission)))
	mux.Handle("GET /api/v1/permissions", mwAuth(http.HandlerFunc(rbacHandler.ListPermissions)))
	mux.Handle("POST /api/v1/permissions", mwAuth(http.HandlerFunc(rbacHandler.CreatePermission)))
	mux.Handle("GET /api/v1/permissions/{id}", mwAuth(http.HandlerFunc(rbacHandler.GetPermission)))
	mux.Handle("GET /api/v1/permissions/node/{node}", mwAuth(http.HandlerFunc(rbacHandler.GetPermissionByNode)))
	mux.Handle("DELETE /api/v1/permissions/{id}", mwAuth(http.HandlerFunc(rbacHandler.DeletePermission)))

	// --------------- Bee Name Generator ---------------
	bngStore := bng.NewStore(database.GetDB(cfg.Database.DBURL + "/bee_name_generator"))
	bngHandler := bng.NewHandler(bngStore)

	mux.HandleFunc("GET /api/v1/bee-name-generator/name", bngHandler.GetBeeName)
	mux.Handle("POST /api/v1/bee-name-generator/name/{name}", mwAuth(http.HandlerFunc(bngHandler.UploadBeeName)))
	mux.Handle("DELETE /api/v1/bee-name-generator/name/{name}", mwAuth(http.HandlerFunc(bngHandler.DeleteBeeName)))
	mux.HandleFunc("POST /api/v1/bee-name-generator/suggestion/{name}", bngHandler.SubmitBeeName)
	mux.Handle("GET /api/v1/bee-name-generator/suggestion/{amount}", mwAuth(http.HandlerFunc(bngHandler.GetBeeNameSuggestions)))
	mux.Handle("PUT /api/v1/bee-name-generator/suggestion/{name}", mwAuth(http.HandlerFunc(bngHandler.AcceptBeeNameSuggestion)))
	mux.Handle("DELETE /api/v1/bee-name-generator/suggestion/{name}", mwAuth(http.HandlerFunc(bngHandler.RejectBeeNameSuggestion)))

	// --------------- Data Store ---------------
	dsStore := ds.NewStore(nndb)
	dsService := ds.NewService(dsStore)
	dsHandler := ds.NewHandler(dsService)

	mux.Handle("POST /api/v1/datastore", mwAuth(http.HandlerFunc(dsHandler.CreateDataStore)))
	mux.HandleFunc("GET /api/v1/datastore", dsHandler.ReadDataStore)
	mux.Handle("PUT /api/v1/datastore", mwAuth(http.HandlerFunc(dsHandler.UpdateDataStore)))
	mux.Handle("DELETE /api/v1/datastore", mwAuth(http.HandlerFunc(dsHandler.DeleteDataStore)))

	// --------------- Numbers Data Store ---------------
	nStore := nds.NewStore(nndb)
	nService := nds.NewService(nStore)
	numbersHandler := nds.NewHandler(nService)

	mux.Handle("POST /api/v1/datastore/number", mwAuth(http.HandlerFunc(numbersHandler.CreateNumber)))
	mux.HandleFunc("GET /api/v1/datastore/number", numbersHandler.ReadNumber)
	mux.Handle("PUT /api/v1/datastore/number", mwAuth(http.HandlerFunc(numbersHandler.UpdateNumber)))
	mux.Handle("DELETE /api/v1/datastore/number", mwAuth(http.HandlerFunc(numbersHandler.DeleteNumber)))

	// --------------- Discord ---------------
	verify := mw.VerifyEd25519Middleware(cfg)
	discordHandler := discord.NewHandler()
	mux.Handle("POST /api/discord/webhook", verify(http.HandlerFunc(discordHandler.Webhook)))
	mux.Handle("POST /api/discord/interactions", verify(http.HandlerFunc(discordHandler.Interaction)))

	// --------------- Game Server Status ---------------
	mcsService := mcs.NewService()
	gssService := gss.NewService(cfg.GameServerStatus, mcsService)
	gssHandler := gss.NewHandler(gssService)
	mux.HandleFunc("GET /api/v1/game-server-status/{game}", gssHandler.GameServerStatus)
	mux.HandleFunc("GET /api/v1/game-server-status/simple/{game}", gssHandler.SimpleGameServerStatus)

	// --------------- Minecraft ---------------
	mcStore := mc.NewStore(
		cfg.Minecraft,
		database.GetDB(cfg.Database.DBURL2+"/archive?sslmode=require"), rdb,
		database.GetS3(cfg.S3.APIUrl, cfg.S3.MCATexture.AccessKey, cfg.S3.MCATexture.SecretKey),
		cfg.S3.MCATexture.Bucket)
	mcService := mc.NewService(cfg.Minecraft, mcStore, nil, "https://"+cfg.S3.APIUrl+"/"+cfg.S3.MCATexture.Bucket+"/")
	mcHandler := mc.NewHandler(mcService)

	mux.HandleFunc("GET /api/v1/mc/mojang/lookup/name/{name}", mcHandler.GetMojangPlayerByName)
	mux.HandleFunc("GET /api/v1/mc/mojang/lookup/{uuid}", mcHandler.GetMojangPlayerByUUID)
	mux.HandleFunc("POST /api/v1/mc/mojang/lookup/bulk/byname", mcHandler.GetMojangPlayersByNames)
	mux.HandleFunc("GET /api/v1/mc/mojang/profile/{uuid}", mcHandler.GetMojangProfile)
	mux.HandleFunc("GET /api/v1/mc/profile/{uuid}", mcHandler.GetProfile)
	mux.HandleFunc("GET /api/v1/mc/profile/name/{name}", mcHandler.GetProfileByName)
	mux.HandleFunc("GET /api/v1/mc/geyser/gamertag/{xuid}", mcHandler.GetGeyserPlayerByXUID)
	mux.HandleFunc("GET /api/v1/mc/geyser/xuid/{gamertag}", mcHandler.GetGeyserPlayerByGamertag)
	mux.HandleFunc("GET /api/v1/mc/geyser/skin/{xuid}", mcHandler.GetGeyserSkin)
	mux.HandleFunc("GET /api/v1/mc/profile/bedrock/{uuid}", mcHandler.GetGeyserProfile)
	mux.HandleFunc("GET /api/v1/mc/profile/bedrock/xuid/{xuid}", mcHandler.GetGeyserProfileByXUID)
	mux.HandleFunc("GET /api/v1/mc/profile/bedrock/name/{name}", mcHandler.GetGeyserProfileByName)
	mux.HandleFunc("GET /api/v1/mc/texture/{hash}", mcHandler.GetTexture)

	// --------------- Minecraft Status ---------------
	mcsHandler := mcs.NewHandler(mcsService)
	mux.HandleFunc("GET /api/v1/mcstatus/{host}", mcsHandler.ServerStatus)
	mux.HandleFunc("GET /api/v1/mcstatus/icon/{host}", mcsHandler.Icon)
	mux.HandleFunc("GET /api/v1/mcstatus/simple/{host}", mcsHandler.SimpleStatus)

	// --------------- Pet Pictures ---------------
	petStore := petpics.NewStore(database.GetDB(cfg.Database.DBURL2 + "/pet_pictures"))
	petService := petpics.NewService(petStore)
	petHandler := petpics.NewHandler(petService)

	mux.Handle("POST /api/v1/pet-pictures/pets/{name}", mwAuth(http.HandlerFunc(petHandler.CreatePet)))
	mux.Handle("POST /api/v1/pet-pictures/pets", mwAuth(http.HandlerFunc(petHandler.CreatePet)))
	mux.HandleFunc("GET /api/v1/pet-pictures/pets/{id}", petHandler.GetPet)
	mux.HandleFunc("GET /api/v1/pet-pictures/pets", petHandler.GetPet)
	mux.Handle("PUT /api/v1/pet-pictures/pets", mwAuth(http.HandlerFunc(petHandler.UpdatePet)))
	mux.HandleFunc("GET /api/v1/pet-pictures/pictures/random", petHandler.GetRandPetPictureByName)
	mux.HandleFunc("GET /api/v1/pet-pictures/pictures/{id}", petHandler.GetPetPicture)
	mux.HandleFunc("GET /api/v1/pet-pictures/pictures", petHandler.GetPetPicture)
	mux.Handle("PUT /api/v1/pet-pictures/pictures", mwAuth(http.HandlerFunc(petHandler.UpdatePetPicture)))
	mux.Handle("DELETE /api/v1/pet-pictures/pictures/{id}", mwAuth(http.HandlerFunc(petHandler.DeletePetPicture)))
	mux.Handle("DELETE /api/v1/pet-pictures/pictures", mwAuth(http.HandlerFunc(petHandler.DeletePetPicture)))

	// --------------- Projects ---------------
	projectsHandler := projects.NewHandler(cfg.GitHub)
	mux.HandleFunc("GET /api/v1/projects/releases/{group}/{project}", projectsHandler.GetReleases)

	// --------------- Switchboard ---------------
	// mux.HandleFunc("GET /ws/v1/switchboard/relay", switchboard.ebSocketRelayHandler)
	mux.HandleFunc("GET /websocket/{id}", switchboard.WebSocketRelayHandler)

	// --------------- Teapot ---------------
	mux.HandleFunc("GET /api/v1/teapot", teapot.HandleTeapot)

	// --------------- Twitch ---------------
	twitchStore := twitch.NewStore(database.GetDB(cfg.Database.DBURL + "/twitch"))
	twitchService := twitch.NewService(twitchStore)
	twitchHandler := twitch.NewHandler(cfg.Twitch, twitchService, oauthTokenStore, linkAccountStore)
	mux.HandleFunc("POST /api/twitch/eventsub", twitchHandler.EventSub)

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
	database.InitSnowflake(cfg.Snowflake)
	db := database.GetDB(cfg.Database.DBURL + "/neuralnexus")
	rdb := database.GetRedis(cfg.Redis)
	sess := session.NewService(cfg, session.NewStore(db, rdb))
	accountStore := account.NewStore(db)
	linkAccountStore := link.NewStore(db)
	oauthTokenStore := link.NewOAuthTokenStore(db)
	rateLimit := ratelimit.NewService(ratelimit.NewStore(cfg.Auth.RateLimit, rdb))

	middlewareStack := mw.CreateStack(
		mw.NoSniffMiddleware,
		cors.New(cors.Options{
			AllowedOrigins:   []string{cfg.SiteUrl},
			AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
			AllowedHeaders:   []string{"Content-Type", "Authorization"},
			AllowCredentials: true,
		}).Handler,
		mw.IPMiddleware,
		mw.RequestIDMiddleware,
		mw.RecoveryMiddleware,
		mw.SessionMiddleware(sess),
		mw.RateLimitMiddleware(cfg.Auth.RateLimit, rateLimit, "default", 300, 60),
		mw.RequestLoggerMiddleware,
	)

	router := ApplyRoutes(http.NewServeMux(), cfg, db, rdb, sess, accountStore, linkAccountStore, oauthTokenStore, rateLimit)

	// --------------- Static Files ---------------
	router.Handle("/", http.FileServerFS(public.FS))
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
