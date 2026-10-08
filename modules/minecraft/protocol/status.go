package protocol

import (
	"bytes"
	"errors"
	"io"

	"github.com/goccy/go-json"
)

var StatusRequestPacket = []byte{0x01, PacketIDStatusRequest}

type StatusResponsePacket struct {
	StatusResponse StatusResponse
}

func (p StatusResponsePacket) ID() int32 {
	return PacketIDStatusResponse
}

var ErrStatusJSONTooLong = errors.New("status response JSON too large")

const MaxStatusJSONLength = 32767

func (p StatusResponsePacket) UnmarshalBinary(b []byte) error {
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

	if err := json.Unmarshal(data, &p); err != nil {
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
