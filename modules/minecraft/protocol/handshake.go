package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
)

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

func NewIntentionPacket(protocolVersion int32, serverAddress string, serverPort uint16, intent Intent) Intention {
	return Intention{
		ProtocolVersion: protocolVersion,
		ServerAddress:   serverAddress,
		ServerPort:      serverPort,
		Intent:          intent,
	}
}

func (p Intention) ID() int32 {
	return PacketIDIntention
}

var ErrServerAddressTooLong = errors.New("server address is longer than 255 bytes")

const MaxServerAddressLength = 255

// MarshalBinary returns the length-prefixed handshake packet.
func (p Intention) MarshalBinary() ([]byte, error) {
	var body bytes.Buffer
	if err := WriteVarInt(&body, p.ID()); err != nil {
		return nil, err
	}

	if len(p.ServerAddress) > MaxServerAddressLength {
		return nil, ErrServerAddressTooLong
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
