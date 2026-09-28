package mcstatus

import (
	"net"
	"os"
	"strconv"
	"testing"
)

// svUnusedPort binds an ephemeral TCP port on 127.0.0.1 and immediately
// releases it, giving a port that is guaranteed to refuse new connection
// attempts at the moment this returns (nothing else can have bound it in
// between within a single test). Same "guaranteed connection-refused"
// technique used elsewhere in this repo's test suite for unreachable-store
// tests (see e.g. modules/minecraft/store_test.go's mcUnusedTCPPort).
//
// service.go's three methods make real calls over raw TCP/UDP to third-party
// libraries with no injectable transport seam, so this is the only
// deterministic, networkless way to exercise their error paths.
func svUnusedPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find an unused port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	if err := l.Close(); err != nil {
		t.Fatalf("failed to close probe listener: %v", err)
	}
	return port
}

// svLiveJavaServer reads MC_LIVE_JAVA_SERVER ("host:port") and skips the
// test when it is unset, so this suite still runs green without a real,
// reachable Minecraft Java server. Same self-skip pattern as
// modules/minecraft/store_test.go's TEST_POSTGRES_URL.
func svLiveJavaServer(t *testing.T) (string, int) {
	t.Helper()
	addr := os.Getenv("MC_LIVE_JAVA_SERVER")
	if addr == "" {
		t.Skip("MC_LIVE_JAVA_SERVER not set; skipping live Java server test")
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("invalid MC_LIVE_JAVA_SERVER %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("invalid port in MC_LIVE_JAVA_SERVER %q: %v", addr, err)
	}
	return host, port
}

// svLiveBedrockServer reads MC_LIVE_BEDROCK_SERVER ("host:port") and skips
// the test when it is unset, so this suite still runs green without a real,
// reachable Minecraft Bedrock server.
func svLiveBedrockServer(t *testing.T) (string, int) {
	t.Helper()
	addr := os.Getenv("MC_LIVE_BEDROCK_SERVER")
	if addr == "" {
		t.Skip("MC_LIVE_BEDROCK_SERVER not set; skipping live Bedrock server test")
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("invalid MC_LIVE_BEDROCK_SERVER %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("invalid port in MC_LIVE_BEDROCK_SERVER %q: %v", addr, err)
	}
	return host, port
}

func TestNewService(t *testing.T) {
	t.Run("SV-01_ReturnsNonNilService", func(t *testing.T) {
		s := NewService()
		if s == nil {
			t.Fatal("expected a non-nil MCStatusService")
		}
		if _, ok := s.(*service); !ok {
			t.Fatalf("expected a *service, got %T", s)
		}
	})
}

func TestService_GetJavaServerStatus(t *testing.T) {
	t.Run("SV-02_UnreachableHostReturnsError", func(t *testing.T) {
		port := svUnusedPort(t)
		s := NewService()

		status, err := s.GetJavaServerStatus("127.0.0.1", port, false, 0)

		if status != nil {
			t.Fatalf("expected nil status, got %+v", status)
		}
		if err == nil || err.Error() != "failed to get java server status" {
			t.Fatalf("expected \"failed to get java server status\", got %v", err)
		}
	})

	t.Run("SV-03_UnreachableHostWithQueryEnabledStillErrors", func(t *testing.T) {
		port := svUnusedPort(t)
		s := NewService()

		status, err := s.GetJavaServerStatus("127.0.0.1", port, true, port)

		if status != nil {
			t.Fatalf("expected nil status, got %+v", status)
		}
		if err == nil || err.Error() != "failed to get java server status" {
			t.Fatalf("expected \"failed to get java server status\", got %v", err)
		}
	})

	t.Run("SV-09_LiveServerReturnsStatus", func(t *testing.T) {
		host, port := svLiveJavaServer(t)
		s := NewService()

		status, err := s.GetJavaServerStatus(host, port, false, 0)

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if status == nil {
			t.Fatal("expected non-nil status")
		}
		if status.Host != host {
			t.Errorf("Host: expected %q, got %q", host, status.Host)
		}
		if status.Port != int32(port) {
			t.Errorf("Port: expected %d, got %d", port, status.Port)
		}
	})
}

func TestService_GetBedrockServerStatus(t *testing.T) {
	t.Run("SV-05_UnreachableHostReturnsError", func(t *testing.T) {
		port := svUnusedPort(t)
		s := NewService()

		status, err := s.GetBedrockServerStatus("127.0.0.1", port)

		if status != nil {
			t.Fatalf("expected nil status, got %+v", status)
		}
		if err == nil || err.Error() != "failed to get bedrock server status" {
			t.Fatalf("expected \"failed to get bedrock server status\", got %v", err)
		}
	})

	t.Run("SV-10_LiveServerReturnsStatus", func(t *testing.T) {
		host, port := svLiveBedrockServer(t)
		s := NewService()

		status, err := s.GetBedrockServerStatus(host, port)

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if status == nil {
			t.Fatal("expected non-nil status")
		}
	})
}

func TestService_GetServerStatus(t *testing.T) {
	t.Run("SV-07_JavaDispatch", func(t *testing.T) {
		port := svUnusedPort(t)
		s := NewService()

		_, err := s.GetServerStatus("127.0.0.1", port, false, false, 0)

		if err == nil || err.Error() != "failed to get java server status" {
			t.Fatalf("expected dispatch to GetJavaServerStatus (\"failed to get java server status\"), got %v", err)
		}
	})

	t.Run("SV-08_BedrockDispatch", func(t *testing.T) {
		port := svUnusedPort(t)
		s := NewService()

		_, err := s.GetServerStatus("127.0.0.1", port, true, false, 0)

		if err == nil || err.Error() != "failed to get bedrock server status" {
			t.Fatalf("expected dispatch to GetBedrockServerStatus (\"failed to get bedrock server status\"), got %v", err)
		}
	})
}
