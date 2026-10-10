package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strconv"
	"strings"
)

var (
	ErrInvalidPort             = errors.New("invalid port number")
	ErrInvalidPluginChannel    = errors.New("invalid plugin channel")
	ErrMalformedLegacyResponse = errors.New("malformed legacy response")
	ErrInvalidProtocolVersion  = errors.New("invalid protocol version")
	ErrInvalidPlayerCount      = errors.New("invalid player count")
)

// 1.6 - 1.6.4 Legacy Ping Protocol
const (
	LegacyIDPingPacket          = 0xFE
	LegacyPingPayload           = 0x01
	LegacyIDPluginMessagePacket = 0xFA
	LegacyIDDisconnectPacket    = 0xFF

	SectionHeader    = "§1"
	NullDelimiter    = "\x00"
	SectionDelimiter = "§"
)

var (
	LegacyPingPreamble = []byte{
		LegacyIDPingPacket, LegacyPingPayload, // packet identifier for a server list ping and its payload
		LegacyIDPluginMessagePacket, // packet identifier for a plugin message
		0x00, 0x0B,                  // length of the string in code units (11 bytes), as a short
		0x00, 0x4D, 0x00, 0x43, 0x00, 0x7C, // MC|
		0x00, 0x50, 0x00, 0x69, 0x00, 0x6E, 0x00, 0x67, // Ping
		0x00, 0x48, 0x00, 0x6F, 0x00, 0x73, 0x00, 0x74, // Host
	}
)

// LegacyPingRequest legacy ping request packet for Minecraft versions 1.6 - 1.6.4.
type LegacyPingRequest struct {
	ProtocolVersion uint8
	Hostname        string
	Port            uint32
}

// ID returns the packet ID for the legacy ping request packet
func (p *LegacyPingRequest) ID() int32 {
	return LegacyIDPingPacket
}

// MarshalBinary marshals the legacy ping request packet into a byte slice.
func (p *LegacyPingRequest) MarshalBinary() ([]byte, error) {
	// Payload length = 1 (protocol version) + 2 (hostname char len) + hostname bytes + 4 (port int)
	payloadLength := uint16(1 + 2 + len(p.Hostname)*2 + 4)

	var body bytes.Buffer
	// 3 packet identifier, payload, plugin message id, 2 + 11*2 bytes for "MC|PingHost", 2 payload length
	body.Grow(3 + 2 + 11*2 + 2 + int(payloadLength))

	// Write the legacy ping preamble (packet identifiers and plugin message channel)
	body.Write(LegacyPingPreamble)

	var buf2 [2]byte
	binary.BigEndian.PutUint16(buf2[:], payloadLength)
	body.Write(buf2[:])

	body.WriteByte(p.ProtocolVersion)

	binary.BigEndian.PutUint16(buf2[:], uint16(len(p.Hostname)))
	body.Write(buf2[:])

	// hostname as a UTF-16BE string
	for _, r := range p.Hostname {
		binary.BigEndian.PutUint16(buf2[:], uint16(r))
		body.Write(buf2[:])
	}

	var buf4 [4]byte
	binary.BigEndian.PutUint32(buf4[:], p.Port)
	body.Write(buf4[:])

	return body.Bytes(), nil
}

// UnmarshalBinary unmarshals the legacy ping request packet from a byte slice.
func (p *LegacyPingRequest) UnmarshalBinary(data []byte) error {
	r := bytes.NewReader(data)

	// Verify the preamble (packet identifiers and plugin message channel)
	var preamble [27]byte
	if _, err := io.ReadFull(r, preamble[:]); err != nil {
		return err
	}
	if !bytes.Equal(preamble[:], LegacyPingPreamble) {
		return ErrInvalidPluginChannel
	}

	// Read payload length
	payloadLen, err := readUint16(r)
	if err != nil {
		return err
	}

	// Limit the reader to the payload length
	lr := io.LimitReader(r, int64(payloadLen))

	// Read Protocol Version
	version, err := readUint8(lr)
	if err != nil {
		return err
	}
	p.ProtocolVersion = version

	// Read hostname
	hostname, err := readUTF16BEString(lr)
	if err != nil {
		return err
	}
	p.Hostname = hostname

	// Read port
	port, err := readUint32(lr)
	if err != nil {
		return err
	}
	if port > 65535 {
		return ErrInvalidPort
	}
	p.Port = port

	return nil
}

// LegacyPingResponse legacy ping response packet for Minecraft versions 1.6 - 1.6.4.
type LegacyPingResponse struct {
	ProtocolVersion uint8
	ServerVersion   string
	MOTD            string
	CurrentPlayers  int
	MaxPlayers      int
}

// ID returns the packet ID for the legacy ping response packet
func (p *LegacyPingResponse) ID() int32 {
	return LegacyIDDisconnectPacket
}

// UnmarshalBinary unmarshals the legacy ping response packet from a byte slice.
func (p *LegacyPingResponse) UnmarshalBinary(data []byte) error {
	r := bytes.NewReader(data)

	packetID, err := readUint8(r)
	if err != nil {
		return err
	}
	if packetID != uint8(p.ID()) {
		return ErrUnexpectedPacket
	}

	payload, err := readUTF16BEString(r)
	if err != nil {
		return err
	}

	// 1.6 format or Spigot 1.4 servers replying with 1.6 format (starts with §1\x00)
	if strings.HasPrefix(payload, SectionHeader+NullDelimiter) {
		fields := strings.Split(payload, NullDelimiter)
		if len(fields) != 6 {
			return ErrMalformedLegacyResponse
		}

		proto, err := strconv.Atoi(fields[1])
		if err != nil || proto < 0 || proto > 255 {
			return ErrInvalidProtocolVersion
		}
		p.ProtocolVersion = uint8(proto)
		p.ServerVersion = fields[2]
		p.MOTD = fields[3]

		currP, err := parsePlayerCount(fields[4])
		if err != nil {
			return err
		}
		p.CurrentPlayers = currP

		maxP, err := parsePlayerCount(fields[5])
		if err != nil {
			return err
		}
		p.MaxPlayers = maxP

		return nil
	}

	// 1.4 - 1.5.2 format: exactly 3 fields separated by section signs
	fields := strings.Split(payload, SectionDelimiter)
	if len(fields) != 3 {
		return ErrMalformedLegacyResponse
	}

	p.ProtocolVersion = 0
	p.ServerVersion = ""
	p.MOTD = fields[0]

	currP, err := parsePlayerCount(fields[1])
	if err != nil {
		return err
	}
	p.CurrentPlayers = currP

	maxP, err := parsePlayerCount(fields[2])
	if err != nil {
		return err
	}
	p.MaxPlayers = maxP

	return nil
}

// LegacyPingResponseV16 is a struct representing the legacy ping response packet for Minecraft versions 1.6 - 1.6.4.
type LegacyPingResponseV16 struct {
	ProtocolVersion uint8
	ServerVersion   string
	MOTD            string
	CurrentPlayers  int
	MaxPlayers      int
}

// ID returns the packet ID for the legacy ping response packet
func (p *LegacyPingResponseV16) ID() int32 {
	return LegacyIDDisconnectPacket
}

// MarshalBinary marshals the legacy ping response packet into a byte slice.
func (p *LegacyPingResponseV16) MarshalBinary() ([]byte, error) {
	payload := strings.Join([]string{
		SectionHeader,
		strconv.Itoa(int(p.ProtocolVersion)),
		p.ServerVersion,
		p.MOTD,
		strconv.Itoa(p.CurrentPlayers),
		strconv.Itoa(p.MaxPlayers),
	}, NullDelimiter)

	return marshalLegacyPayload(payload)
}

// LegacyPingResponseV14 is a struct representing the legacy ping response packet for Minecraft versions 1.4 - 1.5.2.
type LegacyPingResponseV14 struct {
	MOTD           string
	CurrentPlayers int
	MaxPlayers     int
}

// ID returns the packet ID for the legacy ping response packet
func (p *LegacyPingResponseV14) ID() int32 {
	return LegacyIDDisconnectPacket
}

// MarshalBinary marshals the legacy ping response packet into a byte slice.
func (p *LegacyPingResponseV14) MarshalBinary() ([]byte, error) {
	payload := strings.Join([]string{
		p.MOTD,
		strconv.Itoa(p.CurrentPlayers),
		strconv.Itoa(p.MaxPlayers),
	}, SectionDelimiter)
	return marshalLegacyPayload(payload)
}

// marshalLegacyPayload marshals a legacy payload string into a byte slice
func marshalLegacyPayload(payload string) ([]byte, error) {
	runes := []rune(payload)
	// 1 byte (packet ID) + 2 bytes (string length prefix) + (len * 2 bytes for UTF-16BE)
	buf := bytes.NewBuffer(make([]byte, 0, 1+2+(len(runes)*2)))

	if err := writeUint8(buf, LegacyIDDisconnectPacket); err != nil {
		return nil, err
	}
	if err := writeUTF16BEString(buf, payload); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// parsePlayerCount parses a string into an integer representing the player count
func parsePlayerCount(s string) (int, error) {
	p, err := strconv.Atoi(s)
	if err != nil || p < 0 {
		return 0, ErrInvalidPlayerCount
	}
	return p, nil
}

// readUint8 reads a single byte from any io.Reader.
func readUint8(r io.Reader) (uint8, error) {
	var buf [1]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return buf[0], nil
}

// writeUint8 writes a single byte to any io.Writer.
func writeUint8(w io.Writer, v uint8) error {
	if bw, ok := w.(io.ByteWriter); ok {
		return bw.WriteByte(v)
	}
	_, err := w.Write([]byte{v})
	return err
}

// readUint16 reads a big-endian 2-byte integer from any io.Reader.
func readUint16(r io.Reader) (uint16, error) {
	var buf [2]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(buf[:]), nil
}

// writeUint16 writes a big-endian 2-byte integer to any io.Writer.
func writeUint16(w io.Writer, v uint16) error {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

// readUint32 reads a big-endian 4-byte integer from any io.Reader.
func readUint32(r io.Reader) (uint32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf[:]), nil
}

// writeUint32 writes a big-endian 4-byte integer to any io.Writer.
func writeUint32(w io.Writer, v uint32) error {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

// readUTF16BEString reads a 2-byte character length followed by that many
// UTF-16BE encoded characters from the reader, returning a Go string.
func readUTF16BEString(r io.Reader) (string, error) {
	charLen, err := readUint16(r)
	if err != nil {
		return "", err
	}
	if charLen == 0 {
		return "", nil
	}

	buf := make([]byte, charLen*2)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}

	runes := make([]rune, charLen)
	for i := range charLen {
		runes[i] = rune(binary.BigEndian.Uint16(buf[i*2:]))
	}
	return string(runes), nil
}

// writeUTF16BEString writes a Go string as a 2-byte character length followed by UTF-16BE encoded characters.
func writeUTF16BEString(w io.Writer, s string) error {
	runes := []rune(s)
	charLen := len(runes)
	if charLen > 65535 {
		return errors.New("string too long for UTF-16BE prefix")
	}

	var lenBytes [2]byte
	binary.BigEndian.PutUint16(lenBytes[:], uint16(charLen))
	if _, err := w.Write(lenBytes[:]); err != nil {
		return err
	}

	if charLen == 0 {
		return nil
	}

	var charBytes [2]byte
	for _, r := range runes {
		binary.BigEndian.PutUint16(charBytes[:], uint16(r))
		if _, err := w.Write(charBytes[:]); err != nil {
			return err
		}
	}
	return nil
}
