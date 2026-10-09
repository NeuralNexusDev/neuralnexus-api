package protocol

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
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
	MaxPacketSize   = 1<<21 - 1
	MaxStringLength = 1<<15 - 1
)

var (
	ErrUnexpectedPacket     = errors.New("unexpected packet")
	ErrNegativePacketSize   = errors.New("packet size cannot be negative")
	ErrPacketTooLarge       = errors.New("packet too large")
	ErrVarIntTooBig         = errors.New("VarInt too big")
	ErrStringNegativeLength = errors.New("string length is negative")
	ErrStringTooLong        = errors.New("string is too long")
	ErrTimestampMismatch    = errors.New("mismatched pong timestamp payload")
)

// StreamReader is an interface that combines io.Reader and io.ByteReader.
type StreamReader interface {
	io.Reader
	io.ByteReader
}

// StreamWriter is an interface that combines io.Writer and io.ByteWriter.
type StreamWriter interface {
	io.Writer
	io.ByteWriter
}

// Packet represents a Minecraft packet
type Packet interface {
	ID() int32
}

// ReadVarInt reads a Minecraft VarInt.
func ReadVarInt(r io.ByteReader) (int32, error) {
	var value uint32
	for position := uint(0); position < 32; position += 7 {
		currentByte, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, io.ErrUnexpectedEOF
			}
			return 0, err
		}

		value |= uint32(currentByte&0x7F) << position

		if currentByte&0x80 == 0 {
			return int32(value), nil
		}
	}
	return 0, ErrVarIntTooBig
}

// WriteVarInt writes a Minecraft VarInt.
func WriteVarInt(w io.ByteWriter, value int32) error {
	// Shifting an unsigned value fills with zeroes, like >>> does.
	v := uint32(value)
	for v&^0x7F != 0 {
		if err := w.WriteByte(byte(v&0x7F) | 0x80); err != nil {
			return err
		}
		v >>= 7
	}
	return w.WriteByte(byte(v))
}

// ReadUtf8MaxSize reads a Minecraft string, which is a VarInt length followed by UTF-8 bytes.
func ReadUtf8MaxSize(r StreamReader, maxSize int32) (string, error) {
	size, err := ReadVarInt(r)
	if err != nil {
		return "", err
	}
	if size < 0 {
		return "", ErrStringNegativeLength
	}
	if size > maxSize {
		return "", ErrStringTooLong
	}

	// Exit early if the reader has a known length and is too small
	if lr, ok := r.(interface{ Len() int }); ok {
		if int(size) > lr.Len() {
			return "", io.ErrUnexpectedEOF
		}
	}

	data, err := io.ReadAll(io.LimitReader(r, int64(size)))
	if err != nil {
		return "", err
	}
	if int32(len(data)) < size {
		return "", io.ErrUnexpectedEOF
	}

	return string(data), nil
}

// ReadUtf8 reads a Minecraft string, which is a VarInt length followed by UTF-8 bytes.
func ReadUtf8(r StreamReader) (string, error) {
	return ReadUtf8MaxSize(r, MaxStringLength)
}

// WriteUtf8MaxSize writes a Minecraft string, which is a VarInt length followed by UTF-8 bytes.
func WriteUtf8MaxSize(w StreamWriter, s string, maxSize int32) error {
	if len(s) > int(maxSize) {
		return ErrStringTooLong
	}
	if err := WriteVarInt(w, int32(len(s))); err != nil {
		return err
	}
	_, err := io.WriteString(w, s)
	return err
}

// WriteUtf8 writes a Minecraft string, which is a VarInt length followed by UTF-8 bytes.
func WriteUtf8(w StreamWriter, s string) error {
	return WriteUtf8MaxSize(w, s, MaxStringLength)
}

func sendAndReceive(ctx context.Context, addr string, payloads ...[]byte) ([]byte, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	buffers := net.Buffers(payloads)
	if _, err := buffers.WriteTo(conn); err != nil {
		return nil, err
	}

	lr := io.LimitReader(conn, MaxPacketSize)
	r := bufio.NewReader(lr)

	size, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	if size < 0 {
		return nil, ErrNegativePacketSize
	}
	if size > MaxPacketSize {
		return nil, ErrPacketTooLarge
	}

	body := make([]byte, size)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

// GetStatus performs a server list ping and returns the StatusResponse.
func GetStatus(ctx context.Context, addr, host string, port uint16, protocolVersion int32) (*StatusResponse, error) {
	handshake, err := NewIntentionPacket(protocolVersion, host, port, IntentStatus).MarshalBinary()
	if err != nil {
		return nil, err
	}

	body, err := sendAndReceive(ctx, addr, handshake, StatusRequestPacket)
	if err != nil {
		return nil, err
	}

	var response StatusResponsePacket
	if err := response.UnmarshalBinary(body); err != nil {
		return nil, err
	}

	return &response.StatusResponse, nil
}

// GetLatency ping a server and get the round-trip latency
func GetLatency(ctx context.Context, addr, host string, port uint16, protocolVersion int32) (time.Duration, error) {
	handshake, err := NewIntentionPacket(protocolVersion, host, port, IntentStatus).MarshalBinary()
	if err != nil {
		return 0, err
	}

	start := time.Now()
	sentTimestamp := start.UnixMilli()
	ping, err := NewPingRequestPacket(sentTimestamp).MarshalBinary()
	if err != nil {
		return 0, err
	}

	body, err := sendAndReceive(ctx, addr, handshake, ping)
	if err != nil {
		return 0, err
	}

	var response PongResponsePacket
	if err := response.UnmarshalBinary(body); err != nil {
		return 0, err
	}
	if response.Timestamp != sentTimestamp {
		return 0, ErrTimestampMismatch
	}

	return time.Since(start), nil
}
