package mcstatus

import (
	"bytes"
	"errors"
	"net"
	"os"
	"strconv"
	"testing"
	"time"
)

// svUnusedPort binds and immediately releases a TCP port, which then refuses
// connections fast and deterministically - the only reliable, networkless
// way to exercise service.go's error paths, since its three methods call raw
// TCP/UDP libraries with no injectable transport seam.
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
		if !errors.Is(err, ErrJavaStatus) {
			t.Fatalf("expected %v, got %v", ErrJavaStatus, err)
		}
	})

	t.Run("SV-03_UnreachableHostWithQueryEnabledStillErrors", func(t *testing.T) {
		port := svUnusedPort(t)
		s := NewService()

		status, err := s.GetJavaServerStatus("127.0.0.1", port, true, port)

		if status != nil {
			t.Fatalf("expected nil status, got %+v", status)
		}
		if !errors.Is(err, ErrJavaStatus) {
			t.Fatalf("expected %v, got %v", ErrJavaStatus, err)
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
		var netErr net.Error
		if !errors.Is(err, ErrBedrockStatus) || !errors.As(err, &netErr) {
			t.Fatalf("expected an error wrapping %v and a net.Error, got %v", ErrBedrockStatus, err)
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

		if !errors.Is(err, ErrJavaStatus) {
			t.Fatalf("expected dispatch to GetJavaServerStatus (%v), got %v", ErrJavaStatus, err)
		}
	})

	t.Run("SV-08_BedrockDispatch", func(t *testing.T) {
		port := svUnusedPort(t)
		s := NewService()

		_, err := s.GetServerStatus("127.0.0.1", port, true, false, 0)

		if !errors.Is(err, ErrBedrockStatus) {
			t.Fatalf("expected dispatch to GetBedrockServerStatus (%v), got %v", ErrBedrockStatus, err)
		}
	})
}

type svUDPRecorder struct {
	conn    *net.UDPConn
	packets chan []byte
}

func svNewUDPRecorder(t *testing.T, port int) *svUDPRecorder {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: port})
	if err != nil {
		t.Fatalf("failed to listen on UDP port %d: %v", port, err)
	}
	r := &svUDPRecorder{conn: conn, packets: make(chan []byte, 4)}
	t.Cleanup(func() { conn.Close() })
	go func() {
		buf := make([]byte, 1024)
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			select {
			case r.packets <- append([]byte(nil), buf[:n]...):
			default:
			}
			conn.WriteToUDP([]byte{0}, addr)
		}
	}()
	return r
}

func (r *svUDPRecorder) port() int { return r.conn.LocalAddr().(*net.UDPAddr).Port }

func svQueryHandshakePrefix() []byte { return []byte{0xFE, 0xFD, 0x09} }

func svRequireHandshake(t *testing.T, r *svUDPRecorder) {
	t.Helper()
	select {
	case pkt := <-r.packets:
		if !bytes.HasPrefix(pkt, svQueryHandshakePrefix()) {
			t.Errorf("packet = %x, want a query handshake starting %x", pkt, svQueryHandshakePrefix())
		}
	case <-time.After(3 * time.Second):
		t.Errorf("no query handshake reached UDP port %d", r.port())
	}
}

func svRequireNoPacket(t *testing.T, r *svUDPRecorder) {
	t.Helper()
	select {
	case pkt := <-r.packets:
		t.Errorf("unexpected packet %x on UDP port %d", pkt, r.port())
	case <-time.After(200 * time.Millisecond):
	}
}

func TestService_GetJavaServerStatusQueryPort(t *testing.T) {
	t.Run("SV-11_QueryGoesToQueryPort", func(t *testing.T) {
		serverPort := svNewUDPRecorder(t, 0)
		queryPort := svNewUDPRecorder(t, 0)
		s := NewService()

		_, err := s.GetJavaServerStatus("127.0.0.1", serverPort.port(), true, queryPort.port())

		if !errors.Is(err, ErrJavaStatus) {
			t.Fatalf("expected %v, got %v", ErrJavaStatus, err)
		}
		svRequireHandshake(t, queryPort)
		svRequireNoPacket(t, serverPort)
	})

	t.Run("SV-12_QueryGoesToServerPortWhenQueryPortEqualsIt", func(t *testing.T) {
		serverPort := svNewUDPRecorder(t, 0)
		s := NewService()

		_, err := s.GetJavaServerStatus("127.0.0.1", serverPort.port(), true, serverPort.port())

		if !errors.Is(err, ErrJavaStatus) {
			t.Fatalf("expected %v, got %v", ErrJavaStatus, err)
		}
		svRequireHandshake(t, serverPort)
	})

	t.Run("SV-13_NoQueryWhenQueryDisabled", func(t *testing.T) {
		serverPort := svNewUDPRecorder(t, 0)
		queryPort := svNewUDPRecorder(t, 0)
		s := NewService()

		_, err := s.GetJavaServerStatus("127.0.0.1", serverPort.port(), false, queryPort.port())

		if !errors.Is(err, ErrJavaStatus) {
			t.Fatalf("expected %v, got %v", ErrJavaStatus, err)
		}
		svRequireNoPacket(t, queryPort)
		svRequireNoPacket(t, serverPort)
	})
}
