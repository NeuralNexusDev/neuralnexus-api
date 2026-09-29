package gss

import (
	"testing"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/gsspb"
	"github.com/NeuralNexusDev/neuralnexus-api/modules/proto/mcstatuspb"
)

func TestNewGameServerStatus(t *testing.T) {
	t.Run("TY-01_HappyPath_RecognizedQueryType", func(t *testing.T) {
		players := []*gsspb.Player{{Name: "alice", Id: "u1"}}
		status := NewGameServerStatus("1.2.3.4", 25565, "Test Server", "world", 20, 3, players, QueryTypeMinecraft, "rawdata")

		if status.Host != "1.2.3.4" || status.Port != 25565 || status.Name != "Test Server" || status.MapName != "world" || status.MaxPlayers != 20 || status.NumPlayers != 3 {
			t.Errorf("NewGameServerStatus() ServerStatus fields = %+v, want host/port/name/map/max/num from 1.2.3.4/25565/Test Server/world/20/3", status.ServerStatus)
		}
		if len(status.Players) != 1 || status.Players[0].Name != "alice" || status.Players[0].Id != "u1" {
			t.Errorf("NewGameServerStatus() Players = %+v, want [{alice u1}]", status.Players)
		}
		if status.QueryType != QueryTypeMinecraft {
			t.Errorf("NewGameServerStatus() QueryType = %q, want %q", status.QueryType, QueryTypeMinecraft)
		}
		if status.Raw != "rawdata" {
			t.Errorf("NewGameServerStatus() Raw = %v, want %q", status.Raw, "rawdata")
		}
		if status.ServerStatus.QueryType != gsspb.QueryType_MINECRAFT {
			t.Errorf("NewGameServerStatus() embedded ServerStatus.QueryType = %v, want %v", status.ServerStatus.QueryType, gsspb.QueryType_MINECRAFT)
		}
	})

	t.Run("TY-02_EdgeCase_UnrecognizedQueryTypeString", func(t *testing.T) {
		status := NewGameServerStatus("1.2.3.4", 25565, "n", "m", 1, 0, nil, QueryType("bogus"), nil)

		if status.ServerStatus.QueryType != gsspb.QueryType_UNKNOWN {
			t.Errorf("NewGameServerStatus() embedded ServerStatus.QueryType = %v, want %v (zero-value fallback)", status.ServerStatus.QueryType, gsspb.QueryType_UNKNOWN)
		}
		if status.QueryType != QueryType("bogus") {
			t.Errorf("NewGameServerStatus() QueryType = %q, want %q", status.QueryType, "bogus")
		}
	})
}

func TestParseQueryType(t *testing.T) {
	tests := []struct {
		id    string
		input string
		want  QueryType
	}{
		{"TY-03_HappyPath_Minecraft", "minecraft", QueryTypeMinecraft},
		{"TY-04_HappyPath_GameQ", "gameq", QueryTypeGameQ},
		{"TY-05_HappyPath_GameDig", "gamedig", QueryTypeGameDig},
		{"TY-06_EdgeCase_EmptyString", "", QueryTypeUnknown},
		{"TY-06_EdgeCase_UnrecognizedString", "xyz", QueryTypeUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			if got := ParseQueryType(tc.input); got != tc.want {
				t.Errorf("ParseQueryType(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestMCServerStatusNormalize(t *testing.T) {
	t.Run("TY-07_HappyPath_PopulatedPlayers", func(t *testing.T) {
		mc := &mcServerStatus{
			ServerStatus: &mcstatuspb.ServerStatus{
				Host:       "1.2.3.4",
				Port:       25565,
				Name:       "MC Server",
				Map:        "world",
				MaxPlayers: 20,
				NumPlayers: 2,
				Players: []*mcstatuspb.Player{
					{Name: "alice", Uuid: "u1"},
					{Name: "bob", Uuid: "u2"},
				},
			},
		}

		got := mc.Normalize()

		if got.Host != "1.2.3.4" || got.Port != 25565 || got.Name != "MC Server" || got.MapName != "world" || got.MaxPlayers != 20 || got.NumPlayers != 2 {
			t.Errorf("Normalize() fields = %+v, want host/port/name/map/max/num from 1.2.3.4/25565/MC Server/world/20/2", got.ServerStatus)
		}
		if len(got.Players) != 2 || got.Players[0].Name != "alice" || got.Players[0].Id != "u1" || got.Players[1].Name != "bob" || got.Players[1].Id != "u2" {
			t.Errorf("Normalize() Players = %+v, want [{alice u1} {bob u2}]", got.Players)
		}
		if got.QueryType != QueryTypeMinecraft {
			t.Errorf("Normalize() QueryType = %q, want %q", got.QueryType, QueryTypeMinecraft)
		}
		if raw, ok := got.Raw.(*mcServerStatus); !ok || raw != mc {
			t.Errorf("Normalize() Raw = %v (type %T), want the original *mcServerStatus", got.Raw, got.Raw)
		}
	})

	t.Run("TY-08_EdgeCase_NilPlayers", func(t *testing.T) {
		mc := &mcServerStatus{
			ServerStatus: &mcstatuspb.ServerStatus{Host: "1.2.3.4", Port: 25565},
		}

		got := mc.Normalize()

		if got.Players == nil {
			t.Error("Normalize() Players = nil, want non-nil zero-length slice")
		}
		if len(got.Players) != 0 {
			t.Errorf("Normalize() Players = %+v, want empty", got.Players)
		}
	})
}

func TestGameQResponseNormalize(t *testing.T) {
	t.Run("TY-09_HappyPath_PopulatedPlayers", func(t *testing.T) {
		gq := &GameQResponse{
			HostName:   "host1",
			PortQuery:  27015,
			Name:       "GQ Server",
			MapName:    "de_dust2",
			MaxPlayers: 32,
			NumPlayers: 5,
			Players:    []string{"p1", "p2"},
		}

		got := gq.Normalize()

		if got.Host != "host1" || got.Port != 27015 || got.Name != "GQ Server" || got.MapName != "de_dust2" || got.MaxPlayers != 32 || got.NumPlayers != 5 {
			t.Errorf("Normalize() fields = %+v, want host/port/name/map/max/num from host1/27015/GQ Server/de_dust2/32/5", got.ServerStatus)
		}
		if len(got.Players) != 2 || got.Players[0].Name != "p1" || got.Players[0].Id != "" || got.Players[1].Name != "p2" {
			t.Errorf("Normalize() Players = %+v, want [{p1 } {p2 }] (empty Id)", got.Players)
		}
		if got.QueryType != QueryTypeGameQ {
			t.Errorf("Normalize() QueryType = %q, want %q", got.QueryType, QueryTypeGameQ)
		}
		if raw, ok := got.Raw.(*GameQResponse); !ok || raw != gq {
			t.Errorf("Normalize() Raw = %v (type %T), want the original *GameQResponse", got.Raw, got.Raw)
		}
	})

	t.Run("TY-10_EdgeCase_NilPlayers", func(t *testing.T) {
		gq := &GameQResponse{HostName: "host1", PortQuery: 27015}

		got := gq.Normalize()

		if got.Players == nil {
			t.Error("Normalize() Players = nil, want non-nil zero-length slice")
		}
		if len(got.Players) != 0 {
			t.Errorf("Normalize() Players = %+v, want empty", got.Players)
		}
	})
}

func TestGameDigResponseNormalize(t *testing.T) {
	t.Run("TY-11_HappyPath_PopulatedPlayers", func(t *testing.T) {
		gd := &GameDigResponse{
			Name:       "GD Server",
			Map:        "island",
			MaxPlayers: 16,
			NumPlayers: 4,
			Connect:    "1.2.3.4:25566",
			QueryPort:  25566,
			Players: []GameDigPlayer{
				{Name: "p1"},
				{Name: "p2"},
			},
		}

		got := gd.Normalize()

		if got.Host != "1.2.3.4:25566" || got.Port != 25566 || got.Name != "GD Server" || got.MapName != "island" || got.MaxPlayers != 16 || got.NumPlayers != 4 {
			t.Errorf("Normalize() fields = %+v, want host/port/name/map/max/num from 1.2.3.4:25566/25566/GD Server/island/16/4", got.ServerStatus)
		}
		if len(got.Players) != 2 || got.Players[0].Name != "p1" || got.Players[0].Id != "" || got.Players[1].Name != "p2" {
			t.Errorf("Normalize() Players = %+v, want [{p1 } {p2 }] (empty Id)", got.Players)
		}
		if got.QueryType != QueryTypeGameDig {
			t.Errorf("Normalize() QueryType = %q, want %q", got.QueryType, QueryTypeGameDig)
		}
		if raw, ok := got.Raw.(*GameDigResponse); !ok || raw != gd {
			t.Errorf("Normalize() Raw = %v (type %T), want the original *GameDigResponse", got.Raw, got.Raw)
		}
	})

	t.Run("TY-12_EdgeCase_NilPlayers", func(t *testing.T) {
		gd := &GameDigResponse{Connect: "1.2.3.4:25566", QueryPort: 25566}

		got := gd.Normalize()

		if got.Players == nil {
			t.Error("Normalize() Players = nil, want non-nil zero-length slice")
		}
		if len(got.Players) != 0 {
			t.Errorf("Normalize() Players = %+v, want empty", got.Players)
		}
	})
}
