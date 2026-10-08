package protocol

import (
	"bufio"
	"errors"
	"io"
	"net"
	"time"
)

type PacketID int32

const (
	PacketIDIntention      = 0x00 // Handshake Serverbound `intention`
	PacketIDStatusRequest  = 0x00 // Status Serverbound `status_request`
	PacketIDStatusResponse = 0x00 // Status Clientbound `status_response`
)

type Packet interface {
	ID() int32
}

var ErrVarIntTooBig = errors.New("VarInt too big")

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

var ErrUnexpectedPacket = errors.New("unexpected packet")
var ErrPacketTooLarge = errors.New("packet too large")

const MaxPacketSize = 2097151 // 2^21 - 1

// Status performs a server list ping and returns the StatusResponse.
func Status(addr, host string, port uint16, protocolVersion int32) (*StatusResponse, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return nil, err
	}

	handshake, err := NewIntentionPacket(protocolVersion, host, port, IntentStatus).MarshalBinary()
	if err != nil {
		return nil, err
	}
	// The handshake gets no reply; the status request (length 1, packet ID 0) is what asks for one.
	if _, err := conn.Write(append(handshake, StatusRequestPacket...)); err != nil {
		return nil, err
	}

	r := bufio.NewReader(conn)
	size, err := ReadVarInt(r) // packet length
	if err != nil {
		return nil, err
	}

	if size > MaxPacketSize {
		return nil, ErrPacketTooLarge
	}

	body := make([]byte, size)
	lr := io.LimitReader(conn, int64(size))
	if _, err = io.ReadAtLeast(lr, body, int(size)); err != nil {
		return nil, err
	}

	var response StatusResponsePacket
	if err := response.UnmarshalBinary(body); err != nil {
		return nil, err
	}

	return &response.StatusResponse, nil
}
