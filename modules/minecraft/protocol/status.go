package protocol

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/goccy/go-json"
)

// StatusRequestPacket a status request packet sent by the client to the server to request server information.
var StatusRequestPacket = []byte{0x01, PacketIDStatusRequest}

// StatusResponsePacket a status response packet sent by the server in response to a status request.
type StatusResponsePacket struct {
	StatusResponse StatusResponse
}

// ID returns the packet ID for the status response packet.
func (p *StatusResponsePacket) ID() int32 {
	return PacketIDStatusResponse
}

// MarshalBinary marshals the status response packet into a byte slice.
func (p *StatusResponsePacket) MarshalBinary() ([]byte, error) {
	jsonData, err := json.Marshal(p.StatusResponse)
	if err != nil {
		return nil, err
	}
	if len(jsonData) > MaxStringLength {
		return nil, ErrStringTooLong
	}

	var body bytes.Buffer
	// Packet ID + VarInt length of JSON + JSON data
	body.Grow(len(jsonData) + 10)

	if err := WriteVarInt(&body, p.ID()); err != nil {
		return nil, err
	}

	if err := WriteVarInt(&body, int32(len(jsonData))); err != nil {
		return nil, err
	}
	body.Write(jsonData)

	var packet bytes.Buffer
	// body length plus 5 for VarInt
	packet.Grow(body.Len() + 5)

	if err := WriteVarInt(&packet, int32(body.Len())); err != nil {
		return nil, err
	}
	packet.Write(body.Bytes())

	return packet.Bytes(), nil
}

// UnmarshalBinary unmarshals the status response packet from a byte slice.
func (p *StatusResponsePacket) UnmarshalBinary(b []byte) error {
	r := bytes.NewReader(b)

	packetID, err := ReadVarInt(r)
	if err != nil {
		return err
	}
	if packetID != p.ID() {
		return ErrUnexpectedPacket
	}

	jsonLength, err := ReadVarInt(r)
	if err != nil {
		return err
	}
	if jsonLength < 0 {
		return ErrStringNegativeLength
	}
	if jsonLength > MaxStringLength {
		return ErrStringTooLong
	}
	if int(jsonLength) > r.Len() {
		return io.ErrUnexpectedEOF
	}

	cursor := len(b) - r.Len()
	data := b[cursor : cursor+int(jsonLength)]

	if err := json.Unmarshal(data, &p.StatusResponse); err != nil {
		return err
	}
	return nil
}

// Version the server's advertised version
type Version struct {
	Name     string `json:"name"`
	Protocol int    `json:"protocol"`
}

// PlayerSample a player name and ID pair
type PlayerSample struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

// Players the player information in the server's status response
type Players struct {
	Max    int            `json:"max"`
	Online int            `json:"online"`
	Sample []PlayerSample `json:"sample"` // Optional, some servers include additional information
}

// StatusResponse the server's response to a status request, containing information about the server.
type StatusResponse struct {
	Version            Version   `json:"version"`
	Players            Players   `json:"players"`            // Optional
	Description        Component `json:"description"`        // Optional
	Favicon            string    `json:"favicon"`            // Optional
	EnforcesSecureChat *bool     `json:"enforcesSecureChat"` // version-specific
}

// PingRequestPacket a ping request packet sent by the client to the server to measure latency.
type PingRequestPacket struct {
	Timestamp int64
}

// NewPingRequestPacket creates a new ping request packet with the given timestamp.
func NewPingRequestPacket(timestamp int64) *PingRequestPacket {
	return &PingRequestPacket{timestamp}
}

// ID returns the packet ID for the ping request packet.
func (p *PingRequestPacket) ID() int32 {
	return PacketIDPingRequest
}

// MarshalBinary marshals the ping request packet into a byte slice.
func (p *PingRequestPacket) MarshalBinary() ([]byte, error) {
	return marshalPingPong(p.ID(), p.Timestamp)
}

// UnmarshalBinary unmarshals the ping request packet from a byte slice.
func (p *PingRequestPacket) UnmarshalBinary(b []byte) error {
	return unmarshalPingPong(p.ID(), &p.Timestamp, b)
}

// PongResponsePacket a pong response packet sent by the server in response to a ping request.
type PongResponsePacket struct {
	Timestamp int64
}

// ID returns the packet ID for the pong response packet.
func (p *PongResponsePacket) ID() int32 {
	return PacketIDPongResponse
}

// MarshalBinary marshals the pong response packet into a byte slice.
func (p *PongResponsePacket) MarshalBinary() ([]byte, error) {
	return marshalPingPong(p.ID(), p.Timestamp)
}

// UnmarshalBinary unmarshals the pong response packet from a byte slice.
func (p *PongResponsePacket) UnmarshalBinary(b []byte) error {
	return unmarshalPingPong(p.ID(), &p.Timestamp, b)
}

func marshalPingPong(id int32, ts int64) ([]byte, error) {
	var body bytes.Buffer
	// 5 for packet ID + 8 for timestamp
	body.Grow(13)

	if err := WriteVarInt(&body, id); err != nil {
		return nil, err
	}

	var tsBytes [8]byte
	binary.BigEndian.PutUint64(tsBytes[:], uint64(ts))
	body.Write(tsBytes[:])

	var packet bytes.Buffer
	// body length plus 5 for VarInt
	packet.Grow(body.Len() + 5)

	if err := WriteVarInt(&packet, int32(body.Len())); err != nil {
		return nil, err
	}
	packet.Write(body.Bytes())

	return packet.Bytes(), nil
}

func unmarshalPingPong(id int32, ts *int64, b []byte) error {
	r := bytes.NewReader(b)

	packetID, err := ReadVarInt(r)
	if err != nil {
		return err
	}
	if packetID != id {
		return ErrUnexpectedPacket
	}
	if r.Len() < 8 {
		return io.ErrUnexpectedEOF
	}

	cursor := len(b) - r.Len()
	tsBytes := b[cursor : cursor+8]

	*ts = int64(binary.BigEndian.Uint64(tsBytes))

	return nil
}
