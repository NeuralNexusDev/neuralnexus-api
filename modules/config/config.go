package config

import (
	"encoding/hex"
	"log"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/ed25519"
	"golang.org/x/oauth2"
)

func checkEnvVar(varName string) string {
	envVar := os.Getenv(varName)
	if envVar == "" {
		log.Fatal(varName + " is not set")
	}
	return envVar
}

type Config struct {
	ApiUrl           string
	SiteUrl          string
	Database         Database
	Snowflake        Snowflake
	Redis            Redis
	S3               S3
	Auth             Auth
	Discord          Discord
	GameServerStatus GameServerStatus
	GitHub           GitHub
	Microsoft        Microsoft
	Minecraft        Minecraft
	Steam            Steam
	Twitch           Twitch
}

type Database struct {
	DBURL  string
	DBURL2 string
}

type Snowflake struct {
	BaseEpoch uint64
	NodeId    uint64
	WorkerId  uint64
}

type Redis struct {
	Address  string
	Username string
	Password string
}

type S3 struct {
	APIUrl     string
	MCATexture MCATexture
}

type MCATexture struct {
	Bucket    string
	AccessKey string
	SecretKey string
}

type Auth struct {
	Account   Account
	RateLimit RateLimit
	Session   Session
}

type Account struct {
	Pepper []byte
}

type Session struct {
	JwtSecret      []byte
	ValidAudiences []string
}

type RateLimit struct {
	RetryAfter int
	Window     time.Duration
}

type Discord struct {
	PublicKey    ed25519.PublicKey
	OAuth2Config *oauth2.Config
}

type GameServerStatus struct {
	GameQEndpoint   string
	GameDigEndpoint string
}

type GitHub struct {
	Token string
}

type Microsoft struct {
	OAuth2Config            *oauth2.Config
	LoginOAuth2Config       *oauth2.Config
	XboxLiveAuthenticateURL string
	XstsAuthorizeURL        string
	MinecraftLoginURL       string
	MinecraftProfileURL     string
	MicrosoftUserInfoURL    string
}

type Minecraft struct {
	LookupByName         string
	LookupByUUID         string
	LookupBulk           string
	LookupProfile        string
	LookupTexture        string
	GeyserXUIDLookup     string
	GeyserSkinLookup     string
	GeyserGamertagLookup string
}

type Steam struct {
	APIKey           string
	OpenIDLoginURL   string
	PlayerSummaryURL string
}

type Twitch struct {
	ClientId       string
	UserId         string
	EventSubUri    string
	EventSubSecret string
	APIBaseUrl     string
	OAuth2Config   *oauth2.Config
}

func FromEnv() *Config {
	apiUrl := checkEnvVar("NN_API_URL")
	siteUrl := checkEnvVar("NN_SITE_URL")

	// ---------- Snowflake ----------
	nodeID, err := strconv.ParseUint(checkEnvVar("SNOWFLAKE_NODE_ID"), 10, 64)
	if err != nil {
		nodeID = 1
		log.Println("Failed to parse SNOWFLAKE_NODE_ID, defaulting to 1")
	}
	workerID, err := strconv.ParseUint(checkEnvVar("SNOWFLAKE_WORKER_ID"), 10, 64)
	if err != nil {
		workerID = 1
		log.Println("Failed to parse SNOWFLAKE_WORKER_ID, defaulting to 1")
	}

	// ---------- Discord ---------
	publicKey, err := hex.DecodeString(checkEnvVar("DISCORD_PUBLIC_KEY"))
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		log.Fatal("DISCORD_PUBLIC_KEY is not the correct length")
		return nil
	}

	// ---------- Microsoft ---------
	microsoftClientId := checkEnvVar("MICROSOFT_CLIENT_ID")
	microsoftClientSecret := checkEnvVar("MICROSOFT_CLIENT_SECRET")
	microsoftRedirectUri := checkEnvVar("MICROSOFT_REDIRECT_URI")

	// ---------- Minecraft ----------
	// Alternatives:
	//	https://api.mojang.com/users/profiles/minecraft/<name>   (occasional 403 due to Mojang misconfiguration)
	//	https://api.mojang.com/minecraft/profile/lookup/name/<name>
	const mojangLookupByName = "https://api.minecraftservices.com/minecraft/profile/lookup/name/"

	// https://api.minecraftservices.com/minecraft/profile/lookup/<uuid>
	const mojangLookupByUUID = "https://api.minecraftservices.com/minecraft/profile/lookup/"

	// Alternatives:
	//	https://api.mojang.com/profiles/minecraft
	//	https://api.mojang.com/minecraft/profile/lookup/bulk/byname
	const mojangLookupBulk = "https://api.minecraftservices.com/minecraft/profile/lookup/bulk/byname"

	// https://sessionserver.mojang.com/session/minecraft/profile/<uuid>
	const mojangLookupProfile = "https://sessionserver.mojang.com/session/minecraft/profile/"

	// http://textures.minecraft.net/texture/<hash>
	const mojangTextureURL = "http://textures.minecraft.net/texture/"

	// https://api.geysermc.org/v2/xbox/xuid/<gamertag> -- Or Floodgate UUID
	// - 503 on unknown xuid, 400 on invalid xuid value
	const geyserXUIDLookup = "https://api.geysermc.org/v2/xbox/xuid/"

	// https://api.geysermc.org/v2/skin/<xuid>
	// - 200 empty object on unknown xuid, 304 on cached resp, 400 on invalid xuid value
	const geyserSkinLookup = "https://api.geysermc.org/v2/skin/"

	// https://api.geysermc.org/v2/xbox/gamertag/<xuid> -- Or Floodgate UUID
	// - 503 on unknown xuid, 400 on invalid xuid value
	const geyserGamertagLookup = "https://api.geysermc.org/v2/xbox/gamertag/"

	// ---------- Twitch ----------
	twitchClientId := checkEnvVar("TWITCH_CLIENT_ID")

	return &Config{
		ApiUrl:  apiUrl,
		SiteUrl: siteUrl,
		Database: Database{
			DBURL:  checkEnvVar("DATABASE_URL"),
			DBURL2: checkEnvVar("DATABASE_URL_2"),
		},
		Snowflake: Snowflake{
			BaseEpoch: 1706639400000, // January 30, 2024 12:30:00 PM Central/Regina
			NodeId:    nodeID,
			WorkerId:  workerID,
		},
		Redis: Redis{
			Address:  checkEnvVar("REDIS_ADDRESS"),
			Username: checkEnvVar("REDIS_USERNAME"),
			Password: checkEnvVar("REDIS_PASSWORD"),
		},
		S3: S3{
			APIUrl: checkEnvVar("S3_API_URL"),
			MCATexture: MCATexture{
				Bucket:    "mca",
				AccessKey: checkEnvVar("S3_ACCESS_KEY_MCA_TEXTURE"),
				SecretKey: checkEnvVar("S3_SECRET_KEY_MCA_TEXTURE"),
			},
		},
		Auth: Auth{
			Account: Account{
				Pepper: []byte(checkEnvVar("PEPPER")),
			},
			RateLimit: RateLimit{
				RetryAfter: 60,
				Window:     time.Minute,
			},
			Session: Session{
				JwtSecret:      []byte(checkEnvVar("JWT_SECRET")),
				ValidAudiences: []string{siteUrl, apiUrl},
			},
		},
		Discord: Discord{
			PublicKey: publicKey,
			OAuth2Config: &oauth2.Config{
				ClientID:     checkEnvVar("DISCORD_CLIENT_ID"),
				ClientSecret: checkEnvVar("DISCORD_CLIENT_SECRET"),
				RedirectURL:  checkEnvVar("DISCORD_REDIRECT_URI"),
				Endpoint: oauth2.Endpoint{
					AuthURL:   "https://discord.com/oauth2/authorize",
					TokenURL:  "https://discord.com/api/oauth2/token",
					AuthStyle: oauth2.AuthStyleInHeader,
				},
			},
		},
		GameServerStatus: GameServerStatus{ // TODO: Migrate this
			GameQEndpoint:   os.Getenv("GSS_GAMEQ_URL"),
			GameDigEndpoint: os.Getenv("GSS_GAMEDIG_URL"),
		},
		GitHub: GitHub{
			Token: checkEnvVar("GITHUB_TOKEN"),
		},
		Microsoft: Microsoft{
			OAuth2Config: &oauth2.Config{
				ClientID:     microsoftClientId,
				ClientSecret: microsoftClientSecret,
				RedirectURL:  microsoftRedirectUri,
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize",
					TokenURL: "https://login.microsoftonline.com/consumers/oauth2/v2.0/token",
				},
				Scopes: []string{"XboxLive.signin", "offline_access"},
			},
			LoginOAuth2Config: &oauth2.Config{
				ClientID:     microsoftClientId,
				ClientSecret: microsoftClientSecret,
				RedirectURL:  microsoftRedirectUri,
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://login.microsoftonline.com/consumers/oauth2/v2.0/authorize",
					TokenURL: "https://login.microsoftonline.com/consumers/oauth2/v2.0/token",
				},
				Scopes: []string{"openid", "profile", "email", "offline_access"},
			},
			XboxLiveAuthenticateURL: "https://user.auth.xboxlive.com/user/authenticate",
			XstsAuthorizeURL:        "https://xsts.auth.xboxlive.com/xsts/authorize",
			MinecraftLoginURL:       "https://api.minecraftservices.com/authentication/login_with_xbox",
			MinecraftProfileURL:     "https://api.minecraftservices.com/minecraft/profile",
			MicrosoftUserInfoURL:    "https://graph.microsoft.com/oidc/userinfo",
		},
		Minecraft: Minecraft{
			LookupByName:         mojangLookupByName,
			LookupByUUID:         mojangLookupByUUID,
			LookupBulk:           mojangLookupBulk,
			LookupProfile:        mojangLookupProfile,
			LookupTexture:        mojangTextureURL,
			GeyserXUIDLookup:     geyserXUIDLookup,
			GeyserSkinLookup:     geyserSkinLookup,
			GeyserGamertagLookup: geyserGamertagLookup,
		},
		Steam: Steam{
			APIKey:           checkEnvVar("STEAM_API_KEY"),
			OpenIDLoginURL:   "https://steamcommunity.com/openid/login",
			PlayerSummaryURL: "https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v2/",
		},
		Twitch: Twitch{
			ClientId:       twitchClientId,
			UserId:         checkEnvVar("TWITCH_USER_ID"),
			EventSubUri:    checkEnvVar("TWITCH_EVENTSUB_URI"),
			EventSubSecret: checkEnvVar("TWITCH_EVENTSUB_SECRET"),
			APIBaseUrl:     "",
			OAuth2Config: &oauth2.Config{
				ClientID:     twitchClientId,
				ClientSecret: checkEnvVar("TWITCH_CLIENT_SECRET"),
				RedirectURL:  checkEnvVar("TWITCH_REDIRECT_URI"),
				Endpoint: oauth2.Endpoint{
					AuthURL:   "https://id.twitch.tv/oauth2/authorize",
					TokenURL:  "https://id.twitch.tv/oauth2/token",
					AuthStyle: oauth2.AuthStyleInParams,
				},
			},
		},
	}
}
