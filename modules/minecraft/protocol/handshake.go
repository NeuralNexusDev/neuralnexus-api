package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
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

func NewIntentionPacket(protocolVersion int32, serverAddress string, serverPort uint16, intent Intent) *Intention {
	return &Intention{
		ProtocolVersion: protocolVersion,
		ServerAddress:   serverAddress,
		ServerPort:      serverPort,
		Intent:          intent,
	}
}

func (p *Intention) ID() int32 {
	return PacketIDIntention
}

var ErrServerAddressTooLong = errors.New("server address is longer than 255 bytes")

const MaxServerAddressLength = 255

// MarshalBinary returns the length-prefixed handshake packet.
func (p *Intention) MarshalBinary() ([]byte, error) {
	if len(p.ServerAddress) > MaxServerAddressLength {
		return nil, ErrServerAddressTooLong
	}

	var body bytes.Buffer
	// 20 bytes for 4 VarInts, + 2 bytes for the Port + length of ServerAddress
	body.Grow(len(p.ServerAddress) + 22)

	if err := WriteVarInt(&body, p.ID()); err != nil {
		return nil, err
	}
	if err := WriteVarInt(&body, p.ProtocolVersion); err != nil {
		return nil, err
	}

	if err := WriteUtf8MaxSize(&body, p.ServerAddress, MaxServerAddressLength); err != nil {
		return nil, err
	}

	var port [2]byte
	binary.BigEndian.PutUint16(port[:], p.ServerPort)
	body.Write(port[:])

	if err := WriteVarInt(&body, int32(p.Intent)); err != nil {
		return nil, err
	}

	var packet bytes.Buffer
	// body length plus 5 for VarInt
	packet.Grow(body.Len() + 5)

	if err := WriteVarInt(&packet, int32(body.Len())); err != nil {
		return nil, err
	}
	packet.Write(body.Bytes())

	return packet.Bytes(), nil
}

// UnmarshalBinary unmarshals the handshake packet from a byte slice.
func (p *Intention) UnmarshalBinary(b []byte) error {
	r := bytes.NewReader(b)

	packetID, err := ReadVarInt(r)
	if err != nil {
		return err
	}
	if packetID != p.ID() {
		return ErrUnexpectedPacket
	}

	p.ProtocolVersion, err = ReadVarInt(r)
	if err != nil {
		return err
	}

	p.ServerAddress, err = ReadUtf8MaxSize(r, MaxServerAddressLength)
	if err != nil {
		return err
	}

	var port [2]byte
	if _, err := io.ReadFull(r, port[:]); err != nil {
		return err
	}
	p.ServerPort = binary.BigEndian.Uint16(port[:])

	intent, err := ReadVarInt(r)
	if err != nil {
		return err
	}
	p.Intent = Intent(intent)

	return nil
}
