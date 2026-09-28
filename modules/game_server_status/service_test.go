package gss

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

// fakeRoundTripper is a test-only http.RoundTripper that returns a canned
// response or a canned error, without making any real network call. It is
// swapped in for http.DefaultTransport (the seam QueryGameQ/QueryGameDig go
// through: both call http.Get directly, which uses http.DefaultClient and
// therefore http.DefaultTransport when none is set). Adapted from the
// pattern in modules/projects/projects_test.go.
type fakeRoundTripper struct {
	resp *http.Response
	err  error
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

// fakeResponse builds a minimal *http.Response suitable for a RoundTripper
// to return.
func fakeResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// swapTransport replaces the package-level http.DefaultTransport for the
// duration of a subtest and restores the original afterward.
func swapTransport(t *testing.T, rt http.RoundTripper) {
	t.Helper()
	orig := http.DefaultTransport
	http.DefaultTransport = rt
	t.Cleanup(func() {
		http.DefaultTransport = orig
	})
}

// closedTCPPort binds a listener on 127.0.0.1:0, reads back the OS-assigned
// port, then closes it immediately so nothing is listening there any more.
// Any connection attempt against it fails fast and deterministically
// (connection refused), without needing a real game server.
func closedTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind listener: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("failed to close listener: %v", err)
	}
	return port
}

func TestNewService(t *testing.T) {
	t.Run("SV-01_Accessor_ReturnsUsableService", func(t *testing.T) {
		svc := NewService()
		if svc == nil {
			t.Fatal("NewService() = nil, want non-nil")
		}
		if _, ok := svc.(*service); !ok {
			t.Errorf("NewService() concrete type = %T, want *service", svc)
		}
	})
}

func TestQueryGameQ(t *testing.T) {
	svc := NewService()

	t.Run("SV-02_HappyPath_DecodesFirstMapEntry", func(t *testing.T) {
		swapTransport(t, &fakeRoundTripper{resp: fakeResponse(http.StatusOK, `{"srv":{"gq_hostname":"host1","gq_port_query":1234,"gq_name":"Name1","gq_mapname":"map1","gq_maxplayers":10,"gq_numplayers":2,"gq_online":true,"players":["p1","p2"]}}`)})

		resp, err := svc.QueryGameQ("cs16", "1.2.3.4", 27015)
		if err != nil {
			t.Fatalf("QueryGameQ() error = %v, want nil", err)
		}
		if resp == nil {
			t.Fatal("QueryGameQ() response = nil, want non-nil")
		}
		if resp.HostName != "host1" || resp.PortQuery != 1234 || resp.Name != "Name1" || resp.MapName != "map1" || resp.MaxPlayers != 10 || resp.NumPlayers != 2 || !resp.Online {
			t.Errorf("QueryGameQ() response = %+v, want fields from host1/1234/Name1/map1/10/2/true", resp)
		}
		if len(resp.Players) != 2 || resp.Players[0] != "p1" || resp.Players[1] != "p2" {
			t.Errorf("QueryGameQ() players = %v, want [p1 p2]", resp.Players)
		}
	})

	t.Run("SV-03_ErrorPath_TransportError", func(t *testing.T) {
		swapTransport(t, &fakeRoundTripper{err: errors.New("simulated transport failure")})

		resp, err := svc.QueryGameQ("cs16", "1.2.3.4", 27015)
		if err == nil {
			t.Fatal("QueryGameQ() error = nil, want non-nil")
		}
		if err.Error() != "failed to query GameQ API" {
			t.Errorf("QueryGameQ() error = %q, want %q", err.Error(), "failed to query GameQ API")
		}
		if resp != nil {
			t.Errorf("QueryGameQ() response = %+v, want nil", resp)
		}
	})

	t.Run("SV-04_ErrorPath_NonOKStatus", func(t *testing.T) {
		swapTransport(t, &fakeRoundTripper{resp: fakeResponse(http.StatusInternalServerError, "upstream exploded")})

		resp, err := svc.QueryGameQ("cs16", "1.2.3.4", 27015)
		if err == nil {
			t.Fatal("QueryGameQ() error = nil, want non-nil")
		}
		if err.Error() != "failed to query GameQ API" {
			t.Errorf("QueryGameQ() error = %q, want %q", err.Error(), "failed to query GameQ API")
		}
		if resp != nil {
			t.Errorf("QueryGameQ() response = %+v, want nil", resp)
		}
	})

	t.Run("SV-05_ErrorPath_InvalidJSONBody", func(t *testing.T) {
		swapTransport(t, &fakeRoundTripper{resp: fakeResponse(http.StatusOK, "not json")})

		resp, err := svc.QueryGameQ("cs16", "1.2.3.4", 27015)
		if err == nil {
			t.Fatal("QueryGameQ() error = nil, want non-nil")
		}
		if err.Error() != "failed to decode response body" {
			t.Errorf("QueryGameQ() error = %q, want %q", err.Error(), "failed to decode response body")
		}
		if resp != nil {
			t.Errorf("QueryGameQ() response = %+v, want nil", resp)
		}
	})

	t.Run("SV-06_EdgeCase_EmptyResponseMap", func(t *testing.T) {
		swapTransport(t, &fakeRoundTripper{resp: fakeResponse(http.StatusOK, "{}")})

		resp, err := svc.QueryGameQ("cs16", "1.2.3.4", 27015)
		if err == nil {
			t.Fatal("QueryGameQ() error = nil, want non-nil")
		}
		if err.Error() != "no response from GameQ API" {
			t.Errorf("QueryGameQ() error = %q, want %q", err.Error(), "no response from GameQ API")
		}
		if resp != nil {
			t.Errorf("QueryGameQ() response = %+v, want nil", resp)
		}
	})
}

func TestQueryGameDig(t *testing.T) {
	svc := NewService()

	t.Run("SV-07_HappyPath_DecodesBody", func(t *testing.T) {
		swapTransport(t, &fakeRoundTripper{resp: fakeResponse(http.StatusOK, `{"name":"n","map":"m","numplayers":2,"maxplayers":10,"connect":"1.2.3.4:25566","queryPort":25566,"players":[{"name":"p1"},{"name":"p2"}]}`)})

		resp, err := svc.QueryGameDig("factorio", "1.2.3.4", 25566)
		if err != nil {
			t.Fatalf("QueryGameDig() error = %v, want nil", err)
		}
		if resp == nil {
			t.Fatal("QueryGameDig() response = nil, want non-nil")
		}
		if resp.Name != "n" || resp.Map != "m" || resp.NumPlayers != 2 || resp.MaxPlayers != 10 || resp.Connect != "1.2.3.4:25566" || resp.QueryPort != 25566 {
			t.Errorf("QueryGameDig() response = %+v, want fields from n/m/2/10/1.2.3.4:25566/25566", resp)
		}
		if len(resp.Players) != 2 || resp.Players[0].Name != "p1" || resp.Players[1].Name != "p2" {
			t.Errorf("QueryGameDig() players = %+v, want [{p1} {p2}]", resp.Players)
		}
	})

	t.Run("SV-08_ErrorPath_TransportError", func(t *testing.T) {
		swapTransport(t, &fakeRoundTripper{err: errors.New("simulated transport failure")})

		resp, err := svc.QueryGameDig("factorio", "1.2.3.4", 25566)
		if err == nil {
			t.Fatal("QueryGameDig() error = nil, want non-nil")
		}
		if err.Error() != "failed to query GameDig API" {
			t.Errorf("QueryGameDig() error = %q, want %q", err.Error(), "failed to query GameDig API")
		}
		if resp != nil {
			t.Errorf("QueryGameDig() response = %+v, want nil", resp)
		}
	})

	t.Run("SV-09_ErrorPath_NonOKStatus", func(t *testing.T) {
		swapTransport(t, &fakeRoundTripper{resp: fakeResponse(http.StatusInternalServerError, "upstream exploded")})

		resp, err := svc.QueryGameDig("factorio", "1.2.3.4", 25566)
		if err == nil {
			t.Fatal("QueryGameDig() error = nil, want non-nil")
		}
		if err.Error() != "failed to query GameDig API" {
			t.Errorf("QueryGameDig() error = %q, want %q", err.Error(), "failed to query GameDig API")
		}
		if resp != nil {
			t.Errorf("QueryGameDig() response = %+v, want nil", resp)
		}
	})

	t.Run("SV-10_ErrorPath_InvalidJSONBody", func(t *testing.T) {
		swapTransport(t, &fakeRoundTripper{resp: fakeResponse(http.StatusOK, "not json")})

		resp, err := svc.QueryGameDig("factorio", "1.2.3.4", 25566)
		if err == nil {
			t.Fatal("QueryGameDig() error = nil, want non-nil")
		}
		if err.Error() != "failed to decode response body" {
			t.Errorf("QueryGameDig() error = %q, want %q", err.Error(), "failed to decode response body")
		}
		if resp != nil {
			t.Errorf("QueryGameDig() response = %+v, want nil", resp)
		}
	})
}

func TestDetermineOrVerifyQueryType(t *testing.T) {
	tests := []struct {
		id        string
		game      string
		queryType QueryType
		wantType  QueryType
		wantValid bool
	}{
		// "bedrock" is exclusive to MinecraftList (not present in GameQList or GameDigList).
		{"SV-11_HappyPath_MinecraftExclusiveExplicitMatch", "bedrock", QueryTypeMinecraft, QueryTypeMinecraft, true},
		{"SV-12_HappyPath_MinecraftExclusiveUnknownResolves", "bedrock", QueryTypeUnknown, QueryTypeMinecraft, true},
		{"SV-13_EdgeCase_MinecraftExclusiveWrongListType", "bedrock", QueryTypeGameQ, QueryTypeUnknown, false},
		// "aa3" is exclusive to GameQList (not present in MinecraftList or GameDigList).
		{"SV-14_HappyPath_GameQExclusiveExplicitMatch", "aa3", QueryTypeGameQ, QueryTypeGameQ, true},
		{"SV-15_HappyPath_GameQExclusiveUnknownResolves", "aa3", QueryTypeUnknown, QueryTypeGameQ, true},
		{"SV-16_EdgeCase_GameQExclusiveWrongListType", "aa3", QueryTypeGameDig, QueryTypeUnknown, false},
		// "aoc" is exclusive to GameDigList (not present in MinecraftList or GameQList).
		{"SV-17_HappyPath_GameDigExclusiveExplicitMatch", "aoc", QueryTypeGameDig, QueryTypeGameDig, true},
		{"SV-18_HappyPath_GameDigExclusiveUnknownResolves", "aoc", QueryTypeUnknown, QueryTypeGameDig, true},
		// Not present in any list at all.
		{"SV-19_EdgeCase_GameNotInAnyList", "not-a-real-game-xyz", QueryTypeUnknown, QueryTypeUnknown, false},
		// "minecraft" is present in all three lists. QueryTypeGameQ doesn't match
		// MinecraftList's switch, so that loop iteration breaks out without
		// returning; the GameQList loop then finds "minecraft" too and its
		// switch does match, resolving via fallthrough to the second list.
		{"SV-20_EdgeCase_MultiListGameFallsThroughToMatchingList", "minecraft", QueryTypeGameQ, QueryTypeGameQ, true},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			gotType, gotValid := DetermineOrVerifyQueryType(tc.game, tc.queryType)
			if gotType != tc.wantType || gotValid != tc.wantValid {
				t.Errorf("DetermineOrVerifyQueryType(%q, %q) = (%q, %v), want (%q, %v)",
					tc.game, tc.queryType, gotType, gotValid, tc.wantType, tc.wantValid)
			}
		})
	}
}

func TestQueryGameServer(t *testing.T) {
	svc := NewService()

	t.Run("SV-21_ErrorPath_UnsupportedGame", func(t *testing.T) {
		status, err := svc.QueryGameServer("not-a-real-game-xyz", "1.2.3.4", 25565, QueryTypeUnknown)
		if err == nil {
			t.Fatal("QueryGameServer() error = nil, want non-nil")
		}
		if err.Error() != "this game is not supported, or the given query type doesn't support this game" {
			t.Errorf("QueryGameServer() error = %q, want the unsupported-game message", err.Error())
		}
		if status != nil {
			t.Errorf("QueryGameServer() status = %+v, want nil", status)
		}
	})

	// SV-22/SV-23 point at a closed local TCP port so any connection attempt
	// fails fast (connection refused) and deterministically, without a real
	// Minecraft server.
	t.Run("SV-22_ErrorPath_JavaDispatchPropagatesError", func(t *testing.T) {
		port := closedTCPPort(t)
		status, err := svc.QueryGameServer("minecraft", "127.0.0.1", port, QueryTypeMinecraft)
		if err == nil {
			t.Fatal("QueryGameServer() error = nil, want non-nil")
		}
		if status != nil {
			t.Errorf("QueryGameServer() status = %+v, want nil", status)
		}
	})

	t.Run("SV-23_ErrorPath_BedrockDispatchPropagatesError", func(t *testing.T) {
		port := closedTCPPort(t)
		status, err := svc.QueryGameServer("bedrock", "127.0.0.1", port, QueryTypeMinecraft)
		if err == nil {
			t.Fatal("QueryGameServer() error = nil, want non-nil")
		}
		if status != nil {
			t.Errorf("QueryGameServer() status = %+v, want nil", status)
		}
	})

	// A successful-query (Java or Bedrock) scenario is intentionally not
	// covered here: QueryGameServer constructs mcstatus.NewService() inline
	// instead of accepting an mcstatus.MCStatusService as an injected
	// dependency, so there is no mock seam, and a genuine success response
	// also needs a live Minecraft server. Not testable as currently written.

	t.Run("SV-25_HappyPath_GameQDispatch", func(t *testing.T) {
		swapTransport(t, &fakeRoundTripper{resp: fakeResponse(http.StatusOK, `{"srv":{"gq_hostname":"host1","gq_port_query":1234,"gq_name":"Name1","gq_mapname":"map1","gq_maxplayers":10,"gq_numplayers":2,"gq_online":true,"players":["p1"]}}`)})

		status, err := svc.QueryGameServer("aa3", "1.2.3.4", 27015, QueryTypeGameQ)
		if err != nil {
			t.Fatalf("QueryGameServer() error = %v, want nil", err)
		}
		if status == nil {
			t.Fatal("QueryGameServer() status = nil, want non-nil")
		}
		if status.QueryType != QueryTypeGameQ {
			t.Errorf("QueryGameServer() status.QueryType = %q, want %q", status.QueryType, QueryTypeGameQ)
		}
		if status.Name != "Name1" || status.MapName != "map1" || status.MaxPlayers != 10 || status.NumPlayers != 2 {
			t.Errorf("QueryGameServer() status = %+v, want fields from Name1/map1/10/2", status.ServerStatus)
		}
		raw, ok := status.Raw.(*GameQResponse)
		if !ok || raw.HostName != "host1" {
			t.Errorf("QueryGameServer() status.Raw = %+v (type %T), want *GameQResponse with HostName host1", status.Raw, status.Raw)
		}
	})

	t.Run("SV-26_ErrorPath_GameQDispatchOffline", func(t *testing.T) {
		swapTransport(t, &fakeRoundTripper{resp: fakeResponse(http.StatusOK, `{"srv":{"gq_hostname":"host1","gq_online":false}}`)})

		status, err := svc.QueryGameServer("aa3", "1.2.3.4", 27015, QueryTypeGameQ)
		if err == nil {
			t.Fatal("QueryGameServer() error = nil, want non-nil")
		}
		if err.Error() != "server is offline" {
			t.Errorf("QueryGameServer() error = %q, want %q", err.Error(), "server is offline")
		}
		if status != nil {
			t.Errorf("QueryGameServer() status = %+v, want nil", status)
		}
	})

	t.Run("SV-27_ErrorPath_GameQDispatchTransportError", func(t *testing.T) {
		swapTransport(t, &fakeRoundTripper{err: errors.New("simulated transport failure")})

		status, err := svc.QueryGameServer("aa3", "1.2.3.4", 27015, QueryTypeGameQ)
		if err == nil {
			t.Fatal("QueryGameServer() error = nil, want non-nil")
		}
		if err.Error() != "failed to query GameQ API" {
			t.Errorf("QueryGameServer() error = %q, want %q", err.Error(), "failed to query GameQ API")
		}
		if status != nil {
			t.Errorf("QueryGameServer() status = %+v, want nil", status)
		}
	})

	t.Run("SV-28_HappyPath_GameDigDispatch", func(t *testing.T) {
		swapTransport(t, &fakeRoundTripper{resp: fakeResponse(http.StatusOK, `{"name":"n","map":"m","numplayers":2,"maxplayers":10,"connect":"1.2.3.4:25566","queryPort":25566,"players":[{"name":"p1"}]}`)})

		status, err := svc.QueryGameServer("aoc", "1.2.3.4", 25566, QueryTypeGameDig)
		if err != nil {
			t.Fatalf("QueryGameServer() error = %v, want nil", err)
		}
		if status == nil {
			t.Fatal("QueryGameServer() status = nil, want non-nil")
		}
		if status.QueryType != QueryTypeGameDig {
			t.Errorf("QueryGameServer() status.QueryType = %q, want %q", status.QueryType, QueryTypeGameDig)
		}
		if status.Name != "n" || status.MapName != "m" || status.MaxPlayers != 10 || status.NumPlayers != 2 {
			t.Errorf("QueryGameServer() status = %+v, want fields from n/m/10/2", status.ServerStatus)
		}
		raw, ok := status.Raw.(*GameDigResponse)
		if !ok || raw.Connect != "1.2.3.4:25566" {
			t.Errorf("QueryGameServer() status.Raw = %+v (type %T), want *GameDigResponse with Connect 1.2.3.4:25566", status.Raw, status.Raw)
		}
	})

	t.Run("SV-29_ErrorPath_GameDigDispatchTransportError", func(t *testing.T) {
		swapTransport(t, &fakeRoundTripper{err: errors.New("simulated transport failure")})

		status, err := svc.QueryGameServer("aoc", "1.2.3.4", 25566, QueryTypeGameDig)
		if err == nil {
			t.Fatal("QueryGameServer() error = nil, want non-nil")
		}
		if err.Error() != "failed to query GameDig API" {
			t.Errorf("QueryGameServer() error = %q, want %q", err.Error(), "failed to query GameDig API")
		}
		if status != nil {
			t.Errorf("QueryGameServer() status = %+v, want nil", status)
		}
	})
}
