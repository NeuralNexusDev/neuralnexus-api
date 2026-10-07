package status

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"time"

	"github.com/goccy/go-json"
)

var ErrVarIntTooBig = errors.New("VarInt too big")

// ReadVarInt reads a Minecraft VarInt.
func ReadVarInt(r io.ByteReader) (int32, error) {
	var value uint32
	for position := uint(0); position < 32; position += 7 {
		currentByte, err := r.ReadByte()
		if err != nil {
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

type Intent int32

const (
	IntentStatus   Intent = 1
	IntentLogin    Intent = 2
	IntentTransfer Intent = 3
)

type Intention struct {
	ProtocolVersion int32
	ServerAddress   string
	ServerPort      uint16
	Intent          Intent
}

var ErrServerAddressTooLong = errors.New("server address is longer than 255 bytes")

// Marshal returns the length-prefixed handshake packet.
func (p Intention) Marshal() ([]byte, error) {
	if len(p.ServerAddress) > 255 {
		return nil, ErrServerAddressTooLong
	}

	var body bytes.Buffer
	if err := WriteVarInt(&body, 0x00); err != nil {
		return nil, err
	}
	if err := WriteVarInt(&body, p.ProtocolVersion); err != nil {
		return nil, err
	}
	if err := WriteVarInt(&body, int32(len(p.ServerAddress))); err != nil {
		return nil, err
	}
	body.WriteString(p.ServerAddress)
	if err := binary.Write(&body, binary.BigEndian, p.ServerPort); err != nil {
		return nil, err
	}
	if err := WriteVarInt(&body, int32(p.Intent)); err != nil {
		return nil, err
	}

	var packet bytes.Buffer
	if err := WriteVarInt(&packet, int32(body.Len())); err != nil {
		return nil, err
	}
	packet.Write(body.Bytes())
	return packet.Bytes(), nil
}

var ErrUnexpectedPacket = errors.New("unexpected packet")

// Status performs a server list ping and returns the status JSON.
func Status(addr, host string, port uint16, protocolVersion int32) (*StatusResponse, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return nil, err
	}

	handshake, err := Intention{
		ProtocolVersion: protocolVersion,
		ServerAddress:   host,
		ServerPort:      port,
		Intent:          IntentStatus,
	}.Marshal()
	if err != nil {
		return nil, err
	}
	// The handshake gets no reply; the status request (length 1, packet ID 0) is what asks for one.
	if _, err := conn.Write(append(handshake, 0x01, 0x00)); err != nil {
		return nil, err
	}

	r := bufio.NewReader(conn)
	if _, err := ReadVarInt(r); err != nil { // packet length
		return nil, err
	}
	packetID, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	if packetID != 0x00 {
		return nil, ErrUnexpectedPacket
	}
	jsonLength, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	data := make([]byte, jsonLength)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}

	var response StatusResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
