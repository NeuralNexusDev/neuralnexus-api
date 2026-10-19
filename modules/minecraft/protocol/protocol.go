package protocol

import (
	"bufio"
	"context"
	"errors"
	"io"
	"time"
)

type PacketID int32

const (
	PacketIDIntention      = 0x00 // Handshake Serverbound `intention`
	PacketIDStatusRequest  = 0x00 // Status Serverbound `status_request`
	PacketIDPingRequest    = 0x01 // Status Serverbound `ping_request`
	PacketIDStatusResponse = 0x00 // Status Clientbound `status_response`
	PacketIDPongResponse   = 0x01 // Status Clientbound `pong_response`
)

const (
	MaxPacketSize = 1<<21 - 1
)

var (
	ErrUnexpectedPacket   = errors.New("unexpected packet")
	ErrNegativePacketSize = errors.New("packet size cannot be negative")
	ErrTimestampMismatch  = errors.New("mismatched pong timestamp payload")
)

// Packet represents a Minecraft packet
type Packet interface {
	ID() int32
}

func GetStatus(ctx context.Context, addr, host string, port uint16, protocolVersion int32) (*StatusResponse, time.Duration, error) {
	handshake, err := NewIntentionPacket(protocolVersion, host, port, IntentStatus).MarshalBinary()
	if err != nil {
		return nil, 0, err
	}

	conn, err := openConnection(ctx, "tcp", addr)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Close()

	lr := io.LimitReader(conn, MaxPacketSize)
	r := bufio.NewReader(lr)

	start := time.Now()
	if err := send(conn, handshake, StatusRequestPacket); err != nil {
		return nil, 0, err
	}

	body, err := receive(r)
	if err != nil {
		return nil, 0, err
	}
	delta := time.Since(start)

	var statusResponse StatusResponsePacket
	if err := statusResponse.UnmarshalBinary(body); err != nil {
		return nil, 0, err
	}

	return &statusResponse.StatusResponse, delta, nil
}

// GetLatency ping a server and get the round-trip latency
func GetLatency(ctx context.Context, addr, host string, port uint16, protocolVersion int32) (time.Duration, error) {
	handshake, err := NewIntentionPacket(protocolVersion, host, port, IntentStatus).MarshalBinary()
	if err != nil {
		return 0, err
	}

	conn, err := openConnection(ctx, "tcp", addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	lr := io.LimitReader(conn, MaxPacketSize)
	r := bufio.NewReader(lr)

	start := time.Now()
	sentTimestamp := start.UnixMilli()
	ping, err := NewPingRequestPacket(sentTimestamp).MarshalBinary()
	if err != nil {
		return 0, err
	}
	if err := send(conn, handshake, ping); err != nil {
		return 0, err
	}

	body, err := receive(r)
	if err != nil {
		return 0, err
	}
	delta := time.Since(start)

	var response PongResponsePacket
	if err := response.UnmarshalBinary(body); err != nil {
		return 0, err
	}
	if response.Timestamp != sentTimestamp {
		return 0, ErrTimestampMismatch
	}

	return delta, nil
}

func GetStatusLegacy(ctx context.Context, addr, host string, port uint32, protocolVersion uint8) (*LegacyPingResponse, time.Duration, error) {
	ping, err := NewLegacyPingRequest(protocolVersion, host, port).MarshalBinary()
	if err != nil {
		return nil, 0, err
	}

	conn, err := openConnection(ctx, "tcp", addr)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Close()

	lr := io.LimitReader(conn, MaxPacketSize)
	r := bufio.NewReader(lr)

	start := time.Now()
	if err := send(conn, ping); err != nil {
		return nil, 0, err
	}

	body, err := receiveNoLen(r)
	if err != nil {
		return nil, 0, err
	}
	delta := time.Since(start)

	var pingResponse LegacyPingResponse
	if err := pingResponse.UnmarshalBinary(body); err != nil {
		return nil, 0, err
	}

	return &pingResponse, delta, nil
}

func GetQueryStatusBasic(ctx context.Context, addr string) (*QueryBasicStatResponse, time.Duration, error) {
	conn, err := openConnection(ctx, "udp", addr)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Close()

	lr := io.LimitReader(conn, MaxPacketSize)
	r := bufio.NewReader(lr)

	challengeRequest, err := NewQueryChallengeRequest().MarshalBinary()
	if err != nil {
		return nil, 0, err
	}

	start := time.Now()
	if _, err := conn.Write(challengeRequest); err != nil {
		return nil, 0, err
	}

	body, err := receiveMax(r, 18)
	if err != nil {
		return nil, 0, err
	}
	delta := time.Since(start)

	var challengeResponse QueryChallengeResponse
	if err := challengeResponse.UnmarshalBinary(body); err != nil {
		return nil, 0, err
	}

	basicStatRequest := NewQueryBasicStatRequest(challengeResponse.ChallengeToken)
	basicStat, err := basicStatRequest.MarshalBinary()
	if err != nil {
		return nil, 0, err
	}

	if _, err := conn.Write(basicStat); err != nil {
		return nil, 0, err
	}

	body2 := make([]byte, 1024)
	n2, err := r.Read(body2)
	if err != nil {
		return nil, 0, err
	}
	body2 = body2[:n2]

	var basicStatResponse QueryBasicStatResponse
	if err := basicStatResponse.UnmarshalBinary(body2); err != nil {
		return nil, 0, err
	}

	return &basicStatResponse, delta, nil
}

func GetQueryStatus(ctx context.Context, addr string) (*QueryStatResponse, time.Duration, error) {
	conn, err := openConnection(ctx, "udp", addr)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Close()

	lr := io.LimitReader(conn, MaxPacketSize)
	r := bufio.NewReader(lr)

	challengeRequest, err := NewQueryChallengeRequest().MarshalBinary()
	if err != nil {
		return nil, 0, err
	}

	start := time.Now()
	if _, err := conn.Write(challengeRequest); err != nil {
		return nil, 0, err
	}

	body, err := receiveMax(r, 18)
	if err != nil {
		return nil, 0, err
	}
	delta := time.Since(start)

	var challengeResponse QueryChallengeResponse
	if err := challengeResponse.UnmarshalBinary(body); err != nil {
		return nil, 0, err
	}

	statRequest := NewQueryStatRequest(challengeResponse.ChallengeToken)
	Stat, err := statRequest.MarshalBinary()
	if err != nil {
		return nil, 0, err
	}

	if _, err := conn.Write(Stat); err != nil {
		return nil, 0, err
	}

	body2 := make([]byte, 1024)
	n2, err := r.Read(body2)
	if err != nil {
		return nil, 0, err
	}
	body2 = body2[:n2]

	var statResponse QueryStatResponse
	if err := statResponse.UnmarshalBinary(body2); err != nil {
		return nil, 0, err
	}

	return &statResponse, delta, nil
}

func GetBedrockStatus(ctx context.Context, addr string) (*UnconnectedPongPacket, time.Duration, error) {
	conn, err := openConnection(ctx, "udp", addr)
	if err != nil {
		return nil, 0, err
	}
	defer conn.Close()

	lr := io.LimitReader(conn, MaxPacketSize)
	r := bufio.NewReader(lr)

	ping, err := NewUnconnectedPingPacket().MarshalBinary()
	if err != nil {
		return nil, 0, err
	}

	start := time.Now()
	if _, err := conn.Write(ping); err != nil {
		return nil, 0, err
	}

	body, err := receiveMax(r, 1024)
	if err != nil {
		return nil, 0, err
	}

	delta := time.Since(start)

	var pong UnconnectedPongPacket
	if err := pong.UnmarshalBinary(body); err != nil {
		return nil, 0, err
	}

	return &pong, delta, nil
}
