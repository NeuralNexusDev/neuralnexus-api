package gss

import (
	"strings"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/mcstatus"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/gsspb"
)

// ServerStatus - Server status interface
type ServerStatus interface {
	Normalize() *GameServerStatus
}

// GameServerStatus - Normalized game server status
type GameServerStatus struct {
	*gsspb.ServerStatus
	QueryType QueryType   `json:"query_type" xml:"query_type"`
	Raw       interface{} `json:"raw,omitempty" xml:"raw,omitempty"`
}

// NewGameServerStatus - Create a new game server status
func NewGameServerStatus(host string, port int, name string, mapName string, maxPlayers int, numPlayers int, players []*gsspb.Player, queryType QueryType, raw interface{}) *GameServerStatus {
	return &GameServerStatus{
		ServerStatus: &gsspb.ServerStatus{
			Host:       host,
			Port:       int32(port),
			Name:       name,
			MapName:    mapName,
			MaxPlayers: int32(maxPlayers),
			NumPlayers: int32(numPlayers),
			Players:    players,
			QueryType:  gsspb.QueryType(gsspb.QueryType_value[strings.ToUpper(string(queryType))]),
		},
		QueryType: queryType,
		Raw:       raw,
	}
}

// QueryType - Query type enum
type QueryType string

const (
	// QueryTypeUnknown - Query type unknown
	QueryTypeUnknown QueryType = "unknown"
	// QueryTypeMinecraft - Query type Minecraft
	QueryTypeMinecraft QueryType = "minecraft"
	// QueryTypeGameQ - Query type GameQ
	QueryTypeGameQ QueryType = "gameq"
	// QueryTypeGameDig - Query type GameDig
	QueryTypeGameDig QueryType = "gamedig"
)

// ParseQueryType - Parse query type
func ParseQueryType(queryType string) QueryType {
	switch queryType {
	case "minecraft":
		return QueryTypeMinecraft
	case "gameq":
		return QueryTypeGameQ
	case "gamedig":
		return QueryTypeGameDig
	default:
		return QueryTypeUnknown
	}
}

// Type alias
type mcServerStatus mcstatus.MCServerStatus

// Normalize - Normalize Minecraft response
func (mc *mcServerStatus) Normalize() *GameServerStatus {
	players := make([]*gsspb.Player, len(mc.Players))
	for i, v := range mc.Players {
		players[i] = &gsspb.Player{Name: v.Name, Id: v.Uuid}
	}

	return NewGameServerStatus(
		mc.Host,
		int(mc.Port),
		mc.Name,
		mc.Map,
		int(mc.MaxPlayers),
		int(mc.NumPlayers),
		players,
		QueryTypeMinecraft,
		mc,
	)
}

// GameQResponse - GameQ REST API response
type GameQResponse struct {
	Address    string   `json:"gq_address" xml:"gq_address"`
	Dedicated  string   `json:"gq_dedicated" xml:"gq_dedicated"`
	GameType   string   `json:"gq_gametype" xml:"gq_gametype"`
	HostName   string   `json:"gq_hostname" xml:"gq_hostname"`
	JoinLink   string   `json:"gq_joinlink" xml:"gq_joinlink"`
	MapName    string   `json:"gq_mapname" xml:"gq_mapname"`
	MaxPlayers int      `json:"gq_maxplayers" xml:"gq_maxplayers"`
	Name       string   `json:"gq_name" xml:"gq_name"`
	NumPlayers int      `json:"gq_numplayers" xml:"gq_numplayers"`
	Online     bool     `json:"gq_online" xml:"gq_online"`
	Password   string   `json:"gq_password" xml:"gq_password"`
	PortClient int      `json:"gq_port_client" xml:"gq_port_client"`
	PortQuery  int      `json:"gq_port_query" xml:"gq_port_query"`
	Protocol   string   `json:"gq_protocol" xml:"gq_protocol"`
	Transport  string   `json:"gq_transport" xml:"gq_transport"`
	Type       string   `json:"gq_type" xml:"gq_type"`
	Players    []string `json:"players" xml:"players"`
	Teams      []string `json:"teams" xml:"teams"`
}

// Normalize - Normalize GameQ response
func (gq *GameQResponse) Normalize() *GameServerStatus {
	players := make([]*gsspb.Player, len(gq.Players))
	for i, v := range gq.Players {
		players[i] = &gsspb.Player{Name: v}
	}

	return NewGameServerStatus(
		gq.HostName,
		gq.PortQuery,
		gq.Name,
		gq.MapName,
		gq.MaxPlayers,
		gq.NumPlayers,
		players,
		QueryTypeGameQ,
		gq,
	)
}

// GameDigResponse - GameDig REST API response
type GameDigResponse struct {
	Name       string          `json:"name" xml:"name"`
	Map        string          `json:"map" xml:"map"`
	Password   bool            `json:"password" xml:"password"`
	NumPlayers int             `json:"numplayers" xml:"numplayers"`
	MaxPlayers int             `json:"maxplayers" xml:"maxplayers"`
	Players    []GameDigPlayer `json:"players" xml:"players"`
	Bots       []GameDigPlayer `json:"bots" xml:"bots"`
	Connect    string          `json:"connect" xml:"connect"`
	Ping       int             `json:"ping" xml:"ping"`
	QueryPort  int             `json:"queryPort" xml:"queryPort"`
	Raw        interface{}     `json:"raw" xml:"raw"`
}

// GameDigPlayer - GameDig player
type GameDigPlayer struct {
	Name string      `json:"name" xml:"name"`
	Raw  interface{} `json:"raw" xml:"raw"`
}

// Normalize - Normalize GameDig response
func (gd *GameDigResponse) Normalize() *GameServerStatus {
	players := make([]*gsspb.Player, len(gd.Players))
	for i, v := range gd.Players {
		players[i] = &gsspb.Player{Name: v.Name}
	}

	return NewGameServerStatus(
		gd.Connect,
		gd.QueryPort,
		gd.Name,
		gd.Map,
		gd.MaxPlayers,
		gd.NumPlayers,
		players,
		QueryTypeGameDig,
		gd,
	)
}

// Minecraft List
var MinecraftList = [...]string{
	"minecraft", "bedrock", "minecraftpe", "minecraftbe",
}

// List of supported GameQ games
var GameQList = [...]string{
	"aa3",
	"aapg",
	"arkse",
	"arma3",
	"arma",
	"armedassault2oa",
	"armedassault3",
	"ase",
	"atlas",
	"avorion",
	"barotrauma",
	"batt1944",
	"bf1942",
	"bf2",
	"bf3",
	"bf4",
	"bfbc2",
	"bfh",
	"blackmesa",
	"brink",
	"cfx",
	"cfxplayers",
	"citadel",
	"cod2",
	"cod4",
	"codmw2",
	"codmw3",
	"cod",
	"coduo",
	"codwaw",
	"conanexiles",
	"contagion",
	"crysis2",
	"crysis",
	"crysiswars",
	"cs15",
	"cs16",
	"cs2d",
	"cscz",
	"csgo",
	"css",
	"dal",
	"dayzmod",
	"dayz",
	"dod",
	"dods",
	"doom3",
	"dow",
	"eco",
	"egs",
	"et",
	"etqw",
	"ffe",
	"ffow",
	"fof",
	"gamespy2",
	"gamespy3",
	"gamespy4",
	"gamespy",
	"gmod",
	"grav",
	"gta5m",
	"gtan",
	"gtar",
	"had2",
	"halo",
	"hl1",
	"hl2dm",
	"hll",
	"http",
	"hurtworld",
	"insurgency",
	"insurgencysand",
	"jediacademy",
	"jedioutcast",
	"justcause2",
	"justcause3",
	"killingfloor2",
	"killingfloor",
	"kingpin",
	"l4d2",
	"l4d",
	"lhmp",
	"lifeisfeudal",
	"m2mp",
	"minecraftbe",
	"minecraftpe",
	"minecraft",
	"miscreated",
	"modiverse",
	"mohaa",
	"mordhau",
	"mta",
	"mumble",
	"nmrih",
	"ns2",
	"of",
	"openttd",
	"pixark",
	"postscriptum",
	"projectrealitybf2",
	"quake2",
	"quake3",
	"quake4",
	"quakelive",
	"raknet",
	"redorchestra2",
	"redorchestraostfront",
	"rf2",
	"rfactor2",
	"rfactor",
	"risingstorm2",
	"rust",
	"samp",
	"sco",
	"serioussam",
	"sevendaystodie",
	"ship",
	"sof2",
	"soldat",
	"source",
	"spaceengineers",
	"squad",
	"starmade",
	"stormworks",
	"swat4",
	"teamspeak2",
	"teamspeak3",
	"teeworlds",
	"terraria",
	"tf2",
	"theforrest",
	"tibia",
	"tshock",
	"unreal2",
	"unturned",
	"urbanterror",
	"ut2004",
	"ut3",
	"ut",
	"valheim",
	"ventrilo",
	"vrising",
	"warsow",
	"won",
	"wurm",
	"zomboid",
}

// List of supported GameDig games
var GameDigList = [...]string{
	"a2oa",
	"aaa",
	"aapg",
	"actionsource",
	"acwa",
	"ahl",
	"alienarena",
	"alienswarm",
	"americasarmy",
	"americasarmy2",
	"americasarmy3",
	"aoc",
	"aoe2",
	"aosc",
	"arma2",
	"arma3",
	"armagetronadvanced",
	"armareforger",
	"armaresistance",
	"asa",
	"ase",
	"asr08",
	"assettocorsa",
	"atlas",
	"avorion",
	"avp2",
	"avp2010",
	"baldursgate",
	"ballisticoverkill",
	"barotrauma",
	"bas",
	"basedefense",
	"battalion1944",
	"battlefield1942",
	"battlefield2",
	"battlefield2142",
	"battlefield3",
	"battlefield4",
	"battlefieldhardline",
	"battlefieldvietnam",
	"bbc2",
	"beammp",
	"blackmesa",
	"bladesymphony",
	"brainbread",
	"brainbread2",
	"breach",
	"breed",
	"brink",
	"c2d",
	"c3db",
	"cacr",
	"chaser",
	"chrome",
	"cmw",
	"cod",
	"cod2",
	"cod3",
	"cod4mw",
	"codbo3",
	"codenamecure",
	"codenameeagle",
	"codmw2",
	"codmw3",
	"coduo",
	"codwaw",
	"coj",
	"colonysurvival",
	"conanexiles",
	"contagion",
	"contractjack",
	"corekeeper",
	"counterstrike15",
	"counterstrike16",
	"counterstrike2",
	"crce",
	"creativerse",
	"crysis",
	"crysis2",
	"crysiswars",
	"cscz",
	"csgo",
	"css",
	"dab",
	"daikatana",
	"dal",
	"dayofdragons",
	"dayz",
	"dayzmod",
	"ddd",
	"ddpt",
	"deathmatchclassic",
	"deerhunter2005",
	"descent3",
	"deusex",
	"devastation",
	"dhe4445",
	"discord",
	"dmomam",
	"dnf2001",
	"dod",
	"dods",
	"doi",
	"doom3",
	"dootf",
	"dota2",
	"dow",
	"dst",
	"dtr2",
	"dystopia",
	"eco",
	"egs",
	"eldewrito",
	"empiresmod",
	"enshrouded",
	"etqw",
	"ets2",
	"f1c9902",
	"factorio",
	"farcry",
	"farcry2",
	"farmingsimulator19",
	"farmingsimulator22",
	"fear",
	"ffow",
	"fof",
	"formulaone2002",
	"fortressforever",
	"garrysmod",
	"gck",
	"geneshift",
	"globaloperations",
	"goldeneyesource",
	"groundbreach",
	"gta5f",
	"gtasam",
	"gtasamta",
	"gtasao",
	"gtavcmta",
	"gunmanchronicles",
	"gus",
	"halo",
	"halo2",
	"heretic2",
	"hexen2",
	"hiddendangerous2",
	"hl2d",
	"hld",
	"hlds",
	"hll",
	"hlof",
	"homefront",
	"homeworld2",
	"hurtworld",
	"i2cs",
	"i2s",
	"imic",
	"insurgency",
	"insurgencysandstorm",
	"ironstorm",
	"jb0n",
	"jc2m",
	"jc3m",
	"killingfloor",
	"killingfloor2",
	"kloc",
	"kpctnc",
	"kreedzclimbing",
	"kspd",
	"l4d",
	"l4d2",
	"m2m",
	"m2o",
	"mbe",
	"medievalengineers",
	"mgm",
	"minecraft",
	"mnc",
	"moe",
	"moh",
	"moha",
	"mohaa",
	"mohaab",
	"mohaas",
	"mohpa",
	"mohw",
	"mordhau",
	"mumble",
	"mutantfactions",
	"nab",
	"nascarthunder2004",
	"naturalselection",
	"naturalselection2",
	"netpanzer",
	"neverwinternights",
	"neverwinternights2",
	"nexuiz",
	"nfshp2",
	"nitrofamily",
	"nmrih",
	"nolf2asihw",
	"nucleardawn",
	"ofcwc",
	"ofr",
	"ohd",
	"onset",
	"openarena",
	"openttd",
	"painkiller",
	"palworld",
	"pce",
	"pixark",
	"postal2",
	"postscriptum",
	"prb2",
	"prey",
	"projectcars",
	"projectcars2",
	"projectzomboid",
	"pvak2",
	"q3a",
	"quake",
	"quake2",
	"quake4",
	"quakelive",
	"rainbowsix",
	"rallisportchallenge",
	"rallymasters",
	"rdkf",
	"rdr2r",
	"redline",
	"redorchestra",
	"redorchestra2",
	"rfactor",
	"ricochet",
	"risingworld",
	"ron",
	"roo4145",
	"ror2",
	"rs2rs",
	"rs2v",
	"rs3rs",
	"rtcw",
	"rune",
	"rust",
	"s2ats",
	"sdtd",
	"serioussam",
	"serioussam2",
	"shatteredhorizon",
	"shogo",
	"shootmania",
	"sin",
	"sinepisodes",
	"sof",
	"sof2",
	"soldat",
	"sotf",
	"spaceengineers",
	"squad",
	"stalker",
	"starbound",
	"starmade",
	"starsiege",
	"stbc",
	"stn",
	"stvef",
	"stvef2",
	"suicidesurvival",
	"svencoop",
	"swat4",
	"swb",
	"swb2",
	"swjk2jo",
	"swjkja",
	"swrc",
	"synergy",
	"t1s",
	"tacticalops",
	"tcgraw",
	"tcgraw2",
	"teamfactor",
	"teamfortress2",
	"teamspeak2",
	"teamspeak3",
	"terminus",
	"terrariatshock",
	"tfc",
	"theforest",
	"theforrest",
	"thefront",
	"thehidden",
	"theisle",
	"theship",
	"thespecialists",
	"thps3",
	"thps4",
	"thu2",
	"tie",
	"toh",
	"tonolf",
	"towerunite",
	"trackmania2",
	"trackmaniaforever",
	"tremulous",
	"tribesvengeance",
	"tron20",
	"turok2",
	"u2tax",
	"universalcombat",
	"unreal",
	"unrealtournament",
	"unrealtournament2003",
	"unrealtournament2004",
	"unrealtournament3",
	"unturned",
	"urbanterror",
	"v8sc",
	"valheim",
	"vampireslayer",
	"vcm",
	"ventrilo",
	"vietcong",
	"vietcong2",
	"vrising",
	"warfork",
	"warsow",
	"wet",
	"wolfenstein",
	"wot",
	"wurmunlimited",
	"xonotic",
	"xpandrally",
	"zombiemaster",
	"zps",
}
