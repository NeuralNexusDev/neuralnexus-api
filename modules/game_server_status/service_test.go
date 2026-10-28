package gss

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"testing/iotest"
	"time"

	"github.com/NeuralNexusDev/neuralnexus-api/modules/mcstatus"
	"github.com/NeuralNexusDev/neuralnexus-api/test/testutil"
)

func mcLiveServer(t *testing.T) (string, int) {
	t.Helper()
	addr := os.Getenv("MC_LIVE_TEST_SERVER")
	if addr == "" {
		t.Skip("MC_LIVE_TEST_SERVER not set; skipping live-Minecraft test")
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("MC_LIVE_TEST_SERVER %q is not host:port: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("MC_LIVE_TEST_SERVER port %q is not numeric: %v", portStr, err)
	}
	return host, port
}

func TestQueryGameQ(t *testing.T) {
	svc := NewService(testutil.NewConfig().GameServerStatus, mcstatus.NewService())

	t.Run("GSV-02_HappyPath_DecodesFirstMapEntry", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, `{"srv":{"gq_hostname":"host1","gq_port_query":1234,"gq_name":"Name1","gq_mapname":"map1","gq_maxplayers":10,"gq_numplayers":2,"gq_online":true,"players":["p1","p2"]}}`)})

		resp, err := svc.QueryGameQ(testutil.LogContext(context.Background()), "cs16", "1.2.3.4", 27015)
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

	t.Run("GSV-03_ErrorPath_TransportError", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Err: testutil.ErrTransportFailed})

		resp, err := svc.QueryGameQ(testutil.LogContext(context.Background()), "cs16", "1.2.3.4", 27015)
		if err == nil {
			t.Fatal("QueryGameQ() error = nil, want non-nil")
		}
		if !errors.Is(err, ErrGameQQuery) {
			t.Errorf("error = %v, want %v", err, ErrGameQQuery)
		}
		if resp != nil {
			t.Errorf("QueryGameQ() response = %+v, want nil", resp)
		}
	})

	t.Run("GSV-04_ErrorPath_NonOKStatus", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusInternalServerError, "upstream exploded")})

		resp, err := svc.QueryGameQ(testutil.LogContext(context.Background()), "cs16", "1.2.3.4", 27015)
		if err == nil {
			t.Fatal("QueryGameQ() error = nil, want non-nil")
		}
		if !errors.Is(err, ErrGameQQuery) {
			t.Errorf("QueryGameQ() error = %v, want %v", err, ErrGameQQuery)
		}
		if resp != nil {
			t.Errorf("QueryGameQ() response = %+v, want nil", resp)
		}
	})

	t.Run("GSV-05_ErrorPath_InvalidJSONBody", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, "not json")})

		resp, err := svc.QueryGameQ(testutil.LogContext(context.Background()), "cs16", "1.2.3.4", 27015)
		if err == nil {
			t.Fatal("QueryGameQ() error = nil, want non-nil")
		}
		if !errors.Is(err, ErrDecodeBody) {
			t.Errorf("QueryGameQ() error = %v, want %v", err, ErrDecodeBody)
		}
		if resp != nil {
			t.Errorf("QueryGameQ() response = %+v, want nil", resp)
		}
	})

	t.Run("GSV-06_EdgeCase_EmptyResponseMap", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, "{}")})

		resp, err := svc.QueryGameQ(testutil.LogContext(context.Background()), "cs16", "1.2.3.4", 27015)
		if err == nil {
			t.Fatal("QueryGameQ() error = nil, want non-nil")
		}
		if !errors.Is(err, ErrNoGameQResponse) {
			t.Errorf("QueryGameQ() error = %v, want %v", err, ErrNoGameQResponse)
		}
		if resp != nil {
			t.Errorf("QueryGameQ() response = %+v, want nil", resp)
		}
	})

	t.Run("GSV-30_ErrorPath_NonOKStatusBodyUnreadable", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(iotest.ErrReader(testutil.ErrBodyRead)), Header: make(http.Header)}
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: resp})

		got, err := svc.QueryGameQ(testutil.LogContext(context.Background()), "cs16", "1.2.3.4", 27015)
		if !errors.Is(err, ErrReadBody) {
			t.Errorf("error = %v, want %v", err, ErrReadBody)
		}
		if got != nil {
			t.Errorf("QueryGameQ() response = %+v, want nil", got)
		}
	})
}

func TestQueryGameDig(t *testing.T) {
	svc := NewService(testutil.NewConfig().GameServerStatus, mcstatus.NewService())

	t.Run("GSV-07_HappyPath_DecodesBody", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, `{"name":"n","map":"m","numplayers":2,"maxplayers":10,"connect":"1.2.3.4:25566","queryPort":25566,"players":[{"name":"p1"},{"name":"p2"}]}`)})

		resp, err := svc.QueryGameDig(testutil.LogContext(context.Background()), "factorio", "1.2.3.4", 25566)
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

	t.Run("GSV-08_ErrorPath_TransportError", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Err: testutil.ErrTransportFailed})

		resp, err := svc.QueryGameDig(testutil.LogContext(context.Background()), "factorio", "1.2.3.4", 25566)
		if err == nil {
			t.Fatal("QueryGameDig() error = nil, want non-nil")
		}
		if !errors.Is(err, ErrGameDigQuery) {
			t.Errorf("error = %v, want %v", err, ErrGameDigQuery)
		}
		if resp != nil {
			t.Errorf("QueryGameDig() response = %+v, want nil", resp)
		}
	})

	t.Run("GSV-09_ErrorPath_NonOKStatus", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusInternalServerError, "upstream exploded")})

		resp, err := svc.QueryGameDig(testutil.LogContext(context.Background()), "factorio", "1.2.3.4", 25566)
		if err == nil {
			t.Fatal("QueryGameDig() error = nil, want non-nil")
		}
		if !errors.Is(err, ErrGameDigQuery) {
			t.Errorf("QueryGameDig() error = %v, want %v", err, ErrGameDigQuery)
		}
		if resp != nil {
			t.Errorf("QueryGameDig() response = %+v, want nil", resp)
		}
	})

	t.Run("GSV-10_ErrorPath_InvalidJSONBody", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, "not json")})

		resp, err := svc.QueryGameDig(testutil.LogContext(context.Background()), "factorio", "1.2.3.4", 25566)
		if err == nil {
			t.Fatal("QueryGameDig() error = nil, want non-nil")
		}
		if !errors.Is(err, ErrDecodeBody) {
			t.Errorf("QueryGameDig() error = %v, want %v", err, ErrDecodeBody)
		}
		if resp != nil {
			t.Errorf("QueryGameDig() response = %+v, want nil", resp)
		}
	})

	t.Run("GSV-31_ErrorPath_NonOKStatusBodyUnreadable", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(iotest.ErrReader(testutil.ErrBodyRead)), Header: make(http.Header)}
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: resp})

		got, err := svc.QueryGameDig(testutil.LogContext(context.Background()), "valheim", "1.2.3.4", 27015)
		if !errors.Is(err, ErrReadBody) {
			t.Errorf("error = %v, want %v", err, ErrReadBody)
		}
		if got != nil {
			t.Errorf("QueryGameDig() response = %+v, want nil", got)
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
		{"GSV-11_HappyPath_MinecraftExclusiveExplicitMatch", "bedrock", QueryTypeMinecraft, QueryTypeMinecraft, true},
		{"GSV-12_HappyPath_MinecraftExclusiveUnknownResolves", "bedrock", QueryTypeUnknown, QueryTypeMinecraft, true},
		{"GSV-13_EdgeCase_MinecraftExclusiveWrongListType", "bedrock", QueryTypeGameQ, QueryTypeUnknown, false},
		{"GSV-14_HappyPath_GameQExclusiveExplicitMatch", "aa3", QueryTypeGameQ, QueryTypeGameQ, true},
		{"GSV-15_HappyPath_GameQExclusiveUnknownResolves", "aa3", QueryTypeUnknown, QueryTypeGameQ, true},
		{"GSV-16_EdgeCase_GameQExclusiveWrongListType", "aa3", QueryTypeGameDig, QueryTypeUnknown, false},
		{"GSV-17_HappyPath_GameDigExclusiveExplicitMatch", "aoc", QueryTypeGameDig, QueryTypeGameDig, true},
		{"GSV-18_HappyPath_GameDigExclusiveUnknownResolves", "aoc", QueryTypeUnknown, QueryTypeGameDig, true},
		{"GSV-19_EdgeCase_GameNotInAnyList", "not-a-real-game-xyz", QueryTypeUnknown, QueryTypeUnknown, false},
		{"GSV-20_EdgeCase_MultiListGameFallsThroughToMatchingList", "minecraft", QueryTypeGameQ, QueryTypeGameQ, true},
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
	svc := NewService(testutil.NewConfig().GameServerStatus, mcstatus.NewService())

	t.Run("GSV-21_ErrorPath_UnsupportedGame", func(t *testing.T) {
		status, err := svc.QueryGameServer(testutil.LogContext(context.Background()), "not-a-real-game-xyz", "1.2.3.4", 25565, QueryTypeUnknown)
		if err == nil {
			t.Fatal("QueryGameServer() error = nil, want non-nil")
		}
		if !errors.Is(err, ErrGameUnsupported) {
			t.Errorf("QueryGameServer() error = %v, want %v", err, ErrGameUnsupported)
		}
		if status != nil {
			t.Errorf("QueryGameServer() status = %+v, want nil", status)
		}
	})

	t.Run("GSV-22_ErrorPath_JavaDispatchPropagatesError", func(t *testing.T) {
		port := testutil.UnusedTCPPort(t)
		status, err := svc.QueryGameServer(testutil.LogContext(context.Background()), "minecraft", "127.0.0.1", port, QueryTypeMinecraft)
		if err == nil {
			t.Fatal("QueryGameServer() error = nil, want non-nil")
		}
		if status != nil {
			t.Errorf("QueryGameServer() status = %+v, want nil", status)
		}
	})

	t.Run("GSV-23_ErrorPath_BedrockDispatchPropagatesError", func(t *testing.T) {
		port := testutil.UnusedTCPPort(t)
		status, err := svc.QueryGameServer(testutil.LogContext(context.Background()), "bedrock", "127.0.0.1", port, QueryTypeMinecraft)
		if err == nil {
			t.Fatal("QueryGameServer() error = nil, want non-nil")
		}
		if status != nil {
			t.Errorf("QueryGameServer() status = %+v, want nil", status)
		}
	})

	t.Run("GSV-24_HappyPath_MinecraftDispatch", func(t *testing.T) {
		host, port := mcLiveServer(t)
		game := "minecraft"
		if os.Getenv("MC_LIVE_TEST_BEDROCK") != "" {
			game = "bedrock"
		}
		status, err := svc.QueryGameServer(testutil.LogContext(context.Background()), game, host, port, QueryTypeMinecraft)
		if err != nil {
			t.Fatalf("QueryGameServer() error = %v, want nil", err)
		}
		if status == nil {
			t.Fatal("QueryGameServer() status = nil, want non-nil")
		}
		if status.QueryType != QueryTypeMinecraft {
			t.Errorf("QueryGameServer() status.QueryType = %q, want %q", status.QueryType, QueryTypeMinecraft)
		}
	})

	t.Run("GSV-25_HappyPath_GameQDispatch", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, `{"srv":{"gq_hostname":"host1","gq_port_query":1234,"gq_name":"Name1","gq_mapname":"map1","gq_maxplayers":10,"gq_numplayers":2,"gq_online":true,"players":["p1"]}}`)})

		status, err := svc.QueryGameServer(testutil.LogContext(context.Background()), "aa3", "1.2.3.4", 27015, QueryTypeGameQ)
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

	t.Run("GSV-26_ErrorPath_GameQDispatchOffline", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, `{"srv":{"gq_hostname":"host1","gq_online":false}}`)})

		status, err := svc.QueryGameServer(testutil.LogContext(context.Background()), "aa3", "1.2.3.4", 27015, QueryTypeGameQ)
		if err == nil {
			t.Fatal("QueryGameServer() error = nil, want non-nil")
		}
		if !errors.Is(err, ErrServerOffline) {
			t.Errorf("QueryGameServer() error = %v, want %v", err, ErrServerOffline)
		}
		if status != nil {
			t.Errorf("QueryGameServer() status = %+v, want nil", status)
		}
	})

	t.Run("GSV-27_ErrorPath_GameQDispatchTransportError", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Err: testutil.ErrTransportFailed})

		status, err := svc.QueryGameServer(testutil.LogContext(context.Background()), "aa3", "1.2.3.4", 27015, QueryTypeGameQ)
		if err == nil {
			t.Fatal("QueryGameServer() error = nil, want non-nil")
		}
		if !errors.Is(err, ErrGameQQuery) {
			t.Errorf("QueryGameServer() error = %v, want %v", err, ErrGameQQuery)
		}
		if status != nil {
			t.Errorf("QueryGameServer() status = %+v, want nil", status)
		}
	})

	t.Run("GSV-28_HappyPath_GameDigDispatch", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Resp: testutil.FakeResponse(http.StatusOK, `{"name":"n","map":"m","numplayers":2,"maxplayers":10,"connect":"1.2.3.4:25566","queryPort":25566,"players":[{"name":"p1"}]}`)})

		status, err := svc.QueryGameServer(testutil.LogContext(context.Background()), "aoc", "1.2.3.4", 25566, QueryTypeGameDig)
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

	t.Run("GSV-29_ErrorPath_GameDigDispatchTransportError", func(t *testing.T) {
		testutil.SwapTransport(t, &testutil.FakeRoundTripper{Err: testutil.ErrTransportFailed})

		status, err := svc.QueryGameServer(testutil.LogContext(context.Background()), "aoc", "1.2.3.4", 25566, QueryTypeGameDig)
		if err == nil {
			t.Fatal("QueryGameServer() error = nil, want non-nil")
		}
		if !errors.Is(err, ErrGameDigQuery) {
			t.Errorf("QueryGameServer() error = %v, want %v", err, ErrGameDigQuery)
		}
		if status != nil {
			t.Errorf("QueryGameServer() status = %+v, want nil", status)
		}
	})
}

type blockingRoundTripper struct{ reached chan struct{} }

func (b *blockingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	close(b.reached)
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func TestQueryCancellation(t *testing.T) {
	svc := NewService(testutil.NewConfig().GameServerStatus, mcstatus.NewService())

	for _, tc := range []struct {
		id      string
		name    string
		call    func(ctx context.Context) error
		wantErr error
	}{
		{"GSV-32", "QueryGameQ", func(ctx context.Context) error { _, err := svc.QueryGameQ(ctx, "cs16", "1.2.3.4", 27015); return err }, ErrGameQQuery},
		{"GSV-33", "QueryGameDig", func(ctx context.Context) error {
			_, err := svc.QueryGameDig(ctx, "factorio", "1.2.3.4", 25566)
			return err
		}, ErrGameDigQuery},
	} {
		t.Run(tc.id+"_ErrorPath_CancelStopsOutboundGet_"+tc.name, func(t *testing.T) {
			rt := &blockingRoundTripper{reached: make(chan struct{})}
			testutil.SwapTransport(t, rt)
			ctx, cancel := context.WithCancel(testutil.LogContext(context.Background()))
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- tc.call(ctx) }()

			select {
			case <-rt.reached:
			case <-time.After(5 * time.Second):
				t.Fatal("transport was never reached")
			}
			cancel()

			select {
			case err := <-done:
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("error = %v, want %v", err, tc.wantErr)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("call did not return after the ctx was cancelled")
			}
		})
	}
}
