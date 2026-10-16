package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	PacketIDBedrockPing = 0x01
	PacketIDBedrockPong = 0x1C
)

const (
	GameTypeSurvival = iota
	GameTypeCreative
	GameTypeAdventure
	_
	_
	GameTypeDefault
	GameTypeSpectator
)

var (
	ErrInvalidMagic      = errors.New("invalid magic")
	ErrInvalidPongString = errors.New("invalid pong string")
)

// RaknetMagic
// https://wiki.bedrock.dev/servers/raknet
var RaknetMagic = []byte{0x00, 0xff, 0xff, 0x00, 0xfe, 0xfe, 0xfe, 0xfe, 0xfd, 0xfd, 0xfd, 0xfd, 0x12, 0x34, 0x56, 0x78}

// UnconnectedPingPacket represents an unconnected ping packet sent by the client
// https://wiki.bedrock.dev/servers/raknet#unconnected-pings
type UnconnectedPingPacket struct {
	ClientAliveTime uint64 // Client alive time in ms (unsigned long long)
	Magic           []byte // Magic
	ClientGUID      uint64 // Client GUID
}

// ID returns the packet type for the UnconnectedPingPacket.
func (p *UnconnectedPingPacket) ID() int32 {
	return PacketIDBedrockPing
}

// NewUnconnectedPingPacket creates a new UnconnectedPingPacket with the current time as the ClientAliveTime and ClientGUID.
func NewUnconnectedPingPacket() *UnconnectedPingPacket {
	return &UnconnectedPingPacket{
		ClientAliveTime: uint64(time.Now().UnixMilli()),
		Magic:           RaknetMagic,
		ClientGUID:      uint64(time.Now().UnixNano()),
	}
}

// MarshalBinary serializes the UnconnectedPingPacket into a byte slice.
func (p *UnconnectedPingPacket) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, byte(p.ID())); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, p.ClientAliveTime); err != nil {
		return nil, err
	}
	if _, err := buf.Write(p.Magic); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, p.ClientGUID); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshalBinary deserializes the UnconnectedPingPacket from a byte slice.
func (p *UnconnectedPingPacket) UnmarshalBinary(data []byte) error {
	r := bytes.NewReader(data)

	var packetID uint8
	if err := binary.Read(r, binary.BigEndian, &packetID); err != nil {
		return err
	}
	if packetID != uint8(p.ID()) {
		return ErrUnexpectedPacket
	}

	if err := binary.Read(r, binary.BigEndian, &p.ClientAliveTime); err != nil {
		return err
	}

	magic := make([]byte, len(RaknetMagic))
	if _, err := r.Read(magic); err != nil {
		return err
	}
	if !bytes.Equal(magic, RaknetMagic) {
		return ErrInvalidMagic
	}
	p.Magic = magic

	if err := binary.Read(r, binary.BigEndian, &p.ClientGUID); err != nil {
		return err
	}

	return nil
}

// UnconnectedPongPacket
// https://wiki.bedrock.dev/servers/raknet#unconnected-pongs
type UnconnectedPongPacket struct {
	ClientAliveTime uint64 // Client alive time in ms (recorded from previous ping)
	ServerGUID      uint64 // Server GUID
	Magic           []byte // Magic
	Edition         string // Edition (MCPE or MCEE for Education Edition)
	MOTDLine1       string // MOTD line 1
	ProtocolVersion int32  // Protocol Version
	VersionName     string // Version Name
	PlayerCount     int    // Player Count
	MaxPlayerCount  int    // Max Player Count
	ServerUniqueID  string // Server Unique ID
	MOTDLine2       string // MOTD line 2
	GameMode        string // Game mode
	GameModeN       uint8  // Game mode (numeric)
	PortIPv4        uint16 // Port (IPv4)
	PortIPv6        uint16 // Port (IPv6)
}

// ID returns the packet type for the UnconnectedPongPacket.
func (p *UnconnectedPongPacket) ID() int32 {
	return PacketIDBedrockPong
}

// MarshalBinary serializes the UnconnectedPongPacket into a byte slice.
func (p *UnconnectedPongPacket) MarshalBinary() ([]byte, error) {
	buf := new(bytes.Buffer)

	if err := binary.Write(buf, binary.BigEndian, byte(p.ID())); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, p.ClientAliveTime); err != nil {
		return nil, err
	}
	if _, err := buf.Write(p.Magic); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, p.ServerGUID); err != nil {
		return nil, err
	}

	builder := new(strings.Builder)
	builder.Write([]byte(p.Edition))
	builder.WriteByte(';')
	builder.Write([]byte(p.MOTDLine1))
	builder.WriteByte(';')
	builder.WriteString(strconv.Itoa(int(p.ProtocolVersion)))
	builder.WriteByte(';')
	builder.Write([]byte(p.VersionName))
	builder.WriteByte(';')
	builder.WriteString(strconv.Itoa(p.PlayerCount))
	builder.WriteByte(';')
	builder.WriteString(strconv.Itoa(p.MaxPlayerCount))
	builder.WriteByte(';')
	builder.Write([]byte(p.ServerUniqueID))
	builder.WriteByte(';')
	builder.Write([]byte(p.MOTDLine2))
	builder.WriteByte(';')
	builder.Write([]byte(p.GameMode))
	builder.WriteByte(';')
	builder.WriteString(strconv.Itoa(int(p.GameModeN)))
	builder.WriteByte(';')
	builder.WriteString(strconv.Itoa(int(p.PortIPv4)))
	builder.WriteByte(';')
	builder.WriteString(strconv.Itoa(int(p.PortIPv6)))
	builder.WriteByte(';')
	str := builder.String()

	if err := binary.Write(buf, binary.BigEndian, uint16(len(str))); err != nil {
		return nil, err
	}
	if _, err := buf.Write([]byte(str)); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalBinary deserializes the UnconnectedPongPacket from a byte slice.
func (p *UnconnectedPongPacket) UnmarshalBinary(data []byte) error {
	r := bytes.NewReader(data)

	var packetID uint8
	if err := binary.Read(r, binary.BigEndian, &packetID); err != nil {
		return err
	}
	if packetID != uint8(p.ID()) {
		return ErrUnexpectedPacket
	}

	if err := binary.Read(r, binary.BigEndian, &p.ClientAliveTime); err != nil {
		return err
	}

	if err := binary.Read(r, binary.BigEndian, &p.ServerGUID); err != nil {
		return err
	}

	magic := make([]byte, len(RaknetMagic))
	if _, err := r.Read(magic); err != nil {
		return err
	}
	if !bytes.Equal(magic, RaknetMagic) {
		return ErrInvalidMagic
	}
	p.Magic = magic

	// Read string length uint16
	var strLen uint16
	if err := binary.Read(r, binary.BigEndian, &strLen); err != nil {
		return err
	}
	if strLen < 0 {
		return ErrStringNegativeLength
	}
	if strLen > MaxStringLength {
		return ErrStringTooLong
	}
	if int(strLen) > r.Len() {
		return ErrStringTooLong
	}

	// Read the string
	strBytes := make([]byte, strLen)
	if _, err := r.Read(strBytes); err != nil {
		return err
	}
	str := string(strBytes)

	// Split the string by semicolons
	fields := strings.Split(str, ";")
	if len(fields) < 13 {
		return ErrInvalidPongString
	}

	p.Edition = fields[0]
	p.MOTDLine1 = fields[1]

	protocolVersion, err := strconv.Atoi(fields[2])
	if err != nil {
		return err
	}
	p.ProtocolVersion = int32(protocolVersion)
	p.VersionName = fields[3]

	if p.PlayerCount, err = parsePlayerCount(fields[4]); err != nil {
		return err
	}
	if p.MaxPlayerCount, err = parsePlayerCount(fields[5]); err != nil {
		return err
	}

	p.ServerUniqueID = fields[6]
	p.MOTDLine2 = fields[7]
	p.GameMode = fields[8]
	gameMode, err := strconv.Atoi(fields[9])
	if err != nil {
		return err
	}
	p.GameModeN = uint8(gameMode)

	portIPv4, err := strconv.Atoi(fields[10])
	if err != nil {
		return err
	}
	p.PortIPv4 = uint16(portIPv4)

	portIPv6, err := strconv.Atoi(fields[11])
	if err != nil {
		return err
	}
	p.PortIPv6 = uint16(portIPv6)

	return nil
}
