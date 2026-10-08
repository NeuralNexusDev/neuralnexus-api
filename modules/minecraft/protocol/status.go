package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"github.com/goccy/go-json"
)

var StatusRequestPacket = []byte{0x01, PacketIDStatusRequest}

type StatusResponsePacket struct {
	StatusResponse StatusResponse
}

func (p *StatusResponsePacket) ID() int32 {
	return PacketIDStatusResponse
}

var ErrStatusJSONTooLong = errors.New("status response JSON too large")

const MaxStatusJSONLength = 32767

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
	if jsonLength > MaxStatusJSONLength {
		return ErrStatusJSONTooLong
	}

	data := make([]byte, jsonLength)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}

	if err := json.Unmarshal(data, &p.StatusResponse); err != nil {
		return err
	}

	return nil
}

type Version struct {
	Name     string `json:"name"`
	Protocol int    `json:"protocol"`
}

type PlayerSample struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

type Players struct {
	Max    int            `json:"max"`
	Online int            `json:"online"`
	Sample []PlayerSample `json:"sample"` // Optional, some servers include additional information
}

type StatusResponse struct {
	Version            Version   `json:"version"`
	Players            Players   `json:"players"`            // Optional
	Description        Component `json:"description"`        // Optional
	Favicon            string    `json:"favicon"`            // Optional
	EnforcesSecureChat *bool     `json:"enforcesSecureChat"` // version-specific
}

type TimeStampedPacket interface {
	Packet
	GetTimestamp() int64
}

type PingRequestPacket struct {
	Timestamp int64
}

func (p *PingRequestPacket) ID() int32 {
	return PacketIDPingRequest
}

func (p *PingRequestPacket) GetTimestamp() int64 {
	return p.Timestamp
}

func NewPingRequestPacket(timestamp int64) *PingRequestPacket {
	return &PingRequestPacket{
		Timestamp: timestamp,
	}
}

func marshalPingPong(p TimeStampedPacket) ([]byte, error) {
	var body bytes.Buffer
	if err := WriteVarInt(&body, p.ID()); err != nil {
		return nil, err
	}
	if err := binary.Write(&body, binary.BigEndian, p.GetTimestamp()); err != nil {
		return nil, err
	}

	var packet bytes.Buffer
	if err := WriteVarInt(&packet, int32(body.Len())); err != nil {
		return nil, err
	}
	packet.Write(body.Bytes())
	return packet.Bytes(), nil
}

func (p *PingRequestPacket) MarshalBinary() ([]byte, error) {
	return marshalPingPong(p)
}

func (p *PingRequestPacket) UnmarshalBinary(b []byte) error {
	r := bytes.NewReader(b)

	packetID, err := ReadVarInt(r)
	if err != nil {
		return err
	}
	if packetID != p.ID() {
		return ErrUnexpectedPacket
	}

	return binary.Read(r, binary.BigEndian, &p.Timestamp)
}

type PongResponsePacket struct {
	Timestamp int64
}

func (p *PongResponsePacket) ID() int32 {
	return PacketIDPongResponse
}

func (p *PongResponsePacket) GetTimestamp() int64 {
	return p.Timestamp
}

func (p *PongResponsePacket) MarshalBinary() ([]byte, error) {
	return marshalPingPong(p)
}

func (p *PongResponsePacket) UnmarshalBinary(b []byte) error {
	r := bytes.NewReader(b)

	packetID, err := ReadVarInt(r)
	if err != nil {
		return err
	}
	if packetID != p.ID() {
		return ErrUnexpectedPacket
	}

	return binary.Read(r, binary.BigEndian, &p.Timestamp)
}
