package mcstatus

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"image"
	"image/png"
	"net"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf16"
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

func svUnusedUDPPort(t *testing.T) int {
	t.Helper()
	l, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find an unused UDP port: %v", err)
	}
	port := l.LocalAddr().(*net.UDPAddr).Port
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

type svJavaAnswers struct {
	modern  bool
	v16     bool
	v14     bool
	beta    bool
	favicon string
	delay   time.Duration
}

type svJavaServer struct {
	answers   svJavaAnswers
	conns17   atomic.Int32
	conns16   atomic.Int32
	conns14   atomic.Int32
	connsBeta atomic.Int32
}

func svNewJavaServer(t *testing.T, answers svJavaAnswers) (*svJavaServer, int) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	srv := &svJavaServer{answers: answers}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go srv.serve(conn)
		}
	}()
	return srv, l.Addr().(*net.TCPAddr).Port
}

func svLegacyReply(payload string) []byte {
	units := utf16.Encode([]rune(payload))
	out := []byte{0xFF}
	out = binary.BigEndian.AppendUint16(out, uint16(len(units)))
	for _, u := range units {
		out = binary.BigEndian.AppendUint16(out, u)
	}
	return out
}

func (s *svJavaServer) serve(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	r := bufio.NewReader(conn)
	first, err := r.Peek(1)
	if err != nil {
		return
	}
	if first[0] == 0xFE {
		buf := make([]byte, 64)
		n, _ := r.Read(buf)
		switch {
		case bytes.IndexByte(buf[:n], 0xFA) >= 0:
			s.conns16.Add(1)
			if s.answers.v16 {
				conn.Write(svLegacyReply("\u00a71\x00127\x001.6.4\x00legacy motd\x001\x0020"))
			}
		case n == 2:
			s.conns14.Add(1)
			if s.answers.v14 {
				conn.Write(svLegacyReply("legacy motd\u00a71\u00a720"))
			}
		default:
			s.connsBeta.Add(1)
			if s.answers.beta {
				conn.Write(svLegacyReply("legacy motd\u00a71\u00a720"))
			}
		}
		return
	}
	s.conns17.Add(1)
	if !s.answers.modern {
		return
	}
	time.Sleep(s.answers.delay)
	for i := 0; i < 2; i++ {
		length, err := binary.ReadUvarint(r)
		if err != nil {
			return
		}
		if _, err := r.Discard(int(length)); err != nil {
			return
		}
	}
	body := `{"version":{"name":"1.20","protocol":763},"players":{"max":20,"online":1},"description":{"text":"modern motd"}`
	if s.answers.favicon != "" {
		body += `,"favicon":"` + s.answers.favicon + `"`
	}
	body += "}"
	payload := binary.AppendUvarint(nil, 0)
	payload = binary.AppendUvarint(payload, uint64(len(body)))
	payload = append(payload, body...)
	packet := binary.AppendUvarint(nil, uint64(len(payload)))
	conn.Write(append(packet, payload...))
}

func svNewQueryServer(t *testing.T, delay time.Duration) int {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen udp: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	go func() {
		buf := make([]byte, 64)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			if n < 7 {
				continue
			}
			session := buf[3:7]
			if buf[2] == 9 {
				conn.WriteTo(append(append([]byte{9}, session...), []byte("12345\x00")...), addr)
				continue
			}
			time.Sleep(delay)
			out := append([]byte{0}, session...)
			out = append(out, 's', 'p', 'l', 'i', 't', 'n', 'u', 'm', 0, 0x80, 0)
			for _, kv := range [][2]string{
				{"hostname", "query motd"}, {"gametype", "SMP"}, {"game_id", "MINECRAFT"},
				{"version", "query-version"}, {"plugins", ""}, {"map", "world"},
				{"numplayers", "1"}, {"maxplayers", "20"}, {"hostport", "25565"}, {"hostip", "127.0.0.1"},
			} {
				out = append(out, kv[0]...)
				out = append(out, 0)
				out = append(out, kv[1]...)
				out = append(out, 0)
			}
			out = append(out, 0)
			out = append(out, 1, 'p', 'l', 'a', 'y', 'e', 'r', '_', 0, 0)
			out = append(out, "alice"...)
			out = append(out, 0, 0, 0)
			conn.WriteTo(out, addr)
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

func svFaviconDataURI(t *testing.T) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatalf("encode favicon: %v", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestService_GetJavaServerStatusProbeOrder(t *testing.T) {
	t.Run("SV-14_FirstSuccessfulPingWins", func(t *testing.T) {
		srv, port := svNewJavaServer(t, svJavaAnswers{modern: true, v16: true, v14: true, beta: true})

		status, err := NewService().GetJavaServerStatus("127.0.0.1", port, false, 0)

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if status.Legacy || status.Version != "1.20" {
			t.Fatalf("status = legacy %v, version %q; want the modern status", status.Legacy, status.Version)
		}
		if status.Host != "127.0.0.1" || status.Port != int32(port) {
			t.Fatalf("host %q, port %d; want the pinged host and port", status.Host, status.Port)
		}
		if got := srv.conns16.Load() + srv.conns14.Load() + srv.connsBeta.Load(); got != 0 {
			t.Fatalf("legacy pings = %d, want 0 once the modern ping succeeded", got)
		}
	})

	t.Run("SV-15_LegacyPingUsedWhenModernFails", func(t *testing.T) {
		srv, port := svNewJavaServer(t, svJavaAnswers{v16: true, v14: true, beta: true})

		status, err := NewService().GetJavaServerStatus("127.0.0.1", port, false, 0)

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if !status.Legacy || status.Version != "1.6" {
			t.Fatalf("status = legacy %v, version %q; want the 1.6 legacy status", status.Legacy, status.Version)
		}
		if got := srv.conns14.Load() + srv.connsBeta.Load(); got != 0 {
			t.Fatalf("1.4 and beta pings = %d, want 0 once the 1.6 ping succeeded", got)
		}
	})

	t.Run("SV-18_Ping14UsedWhenSixteenFails", func(t *testing.T) {
		srv, port := svNewJavaServer(t, svJavaAnswers{v14: true, beta: true})

		status, err := NewService().GetJavaServerStatus("127.0.0.1", port, false, 0)

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if !status.Legacy || status.Version != "1.4-1.5" {
			t.Fatalf("status = legacy %v, version %q; want the 1.4 legacy status", status.Legacy, status.Version)
		}
		if got := srv.connsBeta.Load(); got != 0 {
			t.Fatalf("beta pings = %d, want 0 once the 1.4 ping succeeded", got)
		}
	})

	t.Run("SV-19_BetaPingUsedWhenOnlyBetaAnswers", func(t *testing.T) {
		srv, port := svNewJavaServer(t, svJavaAnswers{beta: true})

		status, err := NewService().GetJavaServerStatus("127.0.0.1", port, false, 0)

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if !status.Legacy || status.Version != "b1.8-1.3" {
			t.Fatalf("status = legacy %v, version %q; want the beta 1.8 legacy status", status.Legacy, status.Version)
		}
		if srv.conns16.Load() != 1 || srv.conns14.Load() != 1 || srv.connsBeta.Load() != 1 {
			t.Fatalf("pings 1.6/1.4/beta = %d/%d/%d, want one of each", srv.conns16.Load(), srv.conns14.Load(), srv.connsBeta.Load())
		}
	})
}

func TestService_GetJavaServerStatusQueryMerge(t *testing.T) {
	t.Run("SV-20_QueryStatusKeepsPingIconAndFavicon", func(t *testing.T) {
		_, port := svNewJavaServer(t, svJavaAnswers{modern: true, favicon: svFaviconDataURI(t)})
		queryPort := svNewQueryServer(t, 0)

		status, err := NewService().GetJavaServerStatus("127.0.0.1", port, true, queryPort)

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if status.Version != "query-version" || status.Host != "127.0.0.1" || status.Port != int32(port) {
			t.Fatalf("status = version %q, host %q, port %d; want the query status for the pinged host and port", status.Version, status.Host, status.Port)
		}
		if status.Icon == nil || status.Favicon == "" {
			t.Fatalf("icon %v, favicon %q; want the ping's icon and favicon carried over", status.Icon, status.Favicon)
		}
	})
}

func TestService_GetJavaServerStatusQueryConcurrency(t *testing.T) {
	t.Run("SV-21_DeadPingAndDeadQueryReturnsError", func(t *testing.T) {
		port := svUnusedPort(t)
		queryPort := svUnusedUDPPort(t)

		start := time.Now()
		status, err := NewService().GetJavaServerStatus("127.0.0.1", port, true, queryPort)
		elapsed := time.Since(start)

		if status != nil || !errors.Is(err, ErrJavaStatus) {
			t.Fatalf("status = %v, err = %v; want nil and %v", status, err, ErrJavaStatus)
		}
		if elapsed > 2*time.Second {
			t.Fatalf("took %v, want a refused ping chain and refused query to fail fast", elapsed)
		}
	})

	t.Run("SV-22_QueryRunsWhilePingChainRuns", func(t *testing.T) {
		delay := time.Second
		_, port := svNewJavaServer(t, svJavaAnswers{modern: true, delay: delay})
		queryPort := svNewQueryServer(t, delay)

		start := time.Now()
		status, err := NewService().GetJavaServerStatus("127.0.0.1", port, true, queryPort)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if status.Version != "query-version" {
			t.Fatalf("Version = %q, want the query status", status.Version)
		}
		if elapsed >= 2*delay {
			t.Fatalf("took %v, want less than the %v a sequential ping then query would need", elapsed, 2*delay)
		}
	})

	t.Run("SV-23_QueryAnswersWhenEveryPingFails", func(t *testing.T) {
		port := svUnusedPort(t)
		queryPort := svNewQueryServer(t, 0)

		status, err := NewService().GetJavaServerStatus("127.0.0.1", port, true, queryPort)

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if status.Version != "query-version" || status.Host != "127.0.0.1" || status.Port != int32(port) {
			t.Fatalf("status = version %q, host %q, port %d; want the query status for the pinged host and port", status.Version, status.Host, status.Port)
		}
		if status.Icon != nil || status.Legacy {
			t.Fatalf("icon %v, legacy %v; want no icon and not legacy", status.Icon, status.Legacy)
		}
	})

	t.Run("SV-24_PingStatusWaitsForSlowQuery", func(t *testing.T) {
		_, port := svNewJavaServer(t, svJavaAnswers{modern: true})
		queryPort := svNewQueryServer(t, 500*time.Millisecond)

		status, err := NewService().GetJavaServerStatus("127.0.0.1", port, true, queryPort)

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if status.Version != "query-version" {
			t.Fatalf("Version = %q, want the query status once the slow query answered", status.Version)
		}
	})

	t.Run("SV-25_QueryOnlyStatusWaitsForSlowQuery", func(t *testing.T) {
		port := svUnusedPort(t)
		queryPort := svNewQueryServer(t, 500*time.Millisecond)

		status, err := NewService().GetJavaServerStatus("127.0.0.1", port, true, queryPort)

		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if status.Version != "query-version" {
			t.Fatalf("Version = %q, want the query status once the slow query answered", status.Version)
		}
	})
}

func TestMergeQueryStatus(t *testing.T) {
	t.Run("SV-16_QueryStatusKeepsPingIconFaviconAndLegacy", func(t *testing.T) {
		icon := image.NewRGBA(image.Rect(0, 0, 1, 1))
		ping := NewServerStatus("", 0, "", "", "", 0, 0, nil, "", "ping-favicon", ServerTypeJava, nil, icon)
		ping.Legacy = true
		query := NewServerStatus("", 0, "", "", "", 0, 0, nil, "", "", ServerTypeJava, nil, nil)

		got := mergeQueryStatus(ping, query)

		if got != query {
			t.Fatalf("expected the query status to be returned")
		}
		if got.Icon != icon || !got.Legacy {
			t.Fatalf("status = icon %v, legacy %v; want the ping's icon and legacy flag", got.Icon, got.Legacy)
		}
		if got.Favicon != "ping-favicon" {
			t.Fatalf("Favicon = %q, want the ping's favicon", got.Favicon)
		}
	})
}
