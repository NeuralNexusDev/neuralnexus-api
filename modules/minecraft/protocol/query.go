package protocol

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"strconv"
)

// Gamespy Query Protocol version 4
// https://wiki.unrealadmin.org/UT3_query_protocol
const (
	GSQueryV4MSB       = 0xFE
	GSQueryV4LSB       = 0xFD
	QueryTypeChallenge = 0x09
	QueryTypeStat      = 0x00
)

var (
	QueryStatHeader      = []byte{GSQueryV4MSB, GSQueryV4LSB, QueryTypeStat}
	QueryChallengeHeader = []byte{GSQueryV4MSB, GSQueryV4LSB, QueryTypeChallenge}
	QueryStatPadding     = []byte{0xFF, 0xFF, 0xFF, 0x01}
)

// QueryChallengeRequest request a challenge token.
type QueryChallengeRequest struct {
	SessionID []byte
}

// ID returns the packet type for the query challenge request packet.
func (p *QueryChallengeRequest) ID() int32 {
	return QueryTypeChallenge
}

// NewQueryChallengeRequest creates a new QueryChallengeRequest with a packed timestamp as the session ID.
func NewQueryChallengeRequest() *QueryChallengeRequest {
	return &QueryChallengeRequest{packedTimestamp()}
}

// MarshalBinary serializes the QueryChallengeRequest into a byte slice.
func (p *QueryChallengeRequest) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	// magic + type + session ID
	buf.Grow(2 + 1 + 4)

	buf.Write(QueryChallengeHeader)
	buf.Write(p.SessionID)

	return buf.Bytes(), nil
}

// UnmarshalBinary deserializes the QueryChallengeRequest from a byte slice.
func (p *QueryChallengeRequest) UnmarshalBinary(data []byte) error {
	rd := bytes.NewReader(data)
	r := bufio.NewReader(rd)

	magic := make([]byte, 2)
	if _, err := r.Read(magic); err != nil {
		return err
	}
	if magic[0] != GSQueryV4MSB || magic[1] != GSQueryV4LSB {
		return ErrUnexpectedPacket
	}

	typeByte, err := r.ReadByte()
	if err != nil {
		return err
	}
	if typeByte != byte(p.ID()) {
		return ErrUnexpectedPacket
	}

	if _, err := r.Read(p.SessionID); err != nil {
		return err
	}

	return nil
}

// QueryChallengeResponse the response to a QueryChallengeRequest, with the challenge token.
type QueryChallengeResponse struct {
	SessionID      []byte
	ChallengeToken []byte
}

// ID returns the packet type for the query challenge response packet.
func (p *QueryChallengeResponse) ID() int32 {
	return QueryTypeChallenge
}

// UnmarshalBinary deserializes the QueryChallengeResponse from a byte slice.
func (p *QueryChallengeResponse) UnmarshalBinary(data []byte) error {
	rd := bytes.NewReader(data)
	r := bufio.NewReader(rd)

	typeByte, err := r.ReadByte()
	if err != nil {
		return err
	}
	if typeByte != byte(p.ID()) {
		return ErrUnexpectedPacket
	}

	p.SessionID = make([]byte, 4)
	if _, err := r.Read(p.SessionID); err != nil {
		return err
	}

	challengeStr, err := readNullTerminatedString(r)
	if err != nil {
		return err
	}

	challengeToken, err := strconv.ParseInt(challengeStr, 10, 32)
	if err != nil {
		return err
	}

	var challengeBE [4]byte
	binary.BigEndian.PutUint32(challengeBE[:], uint32(challengeToken))
	p.ChallengeToken = challengeBE[:]

	return nil
}

// MarshalBinary serializes the QueryChallengeResponse into a byte slice.
func (p *QueryChallengeResponse) MarshalBinary() ([]byte, error) {
	challengeInt := binary.BigEndian.Uint32(p.ChallengeToken)
	challengeStr := strconv.Itoa(int(challengeInt))

	var buf bytes.Buffer
	// type byte + session ID + challenge token string + null terminator
	buf.Grow(1 + 4 + len(challengeStr) + 1)

	buf.WriteByte(byte(p.ID()))
	buf.Write(p.SessionID)
	buf.WriteString(challengeStr)
	buf.WriteByte(0)

	return buf.Bytes(), nil
}

// QueryStatRequest request for server statistics.
type QueryStatRequest struct {
	SessionID      []byte
	ChallengeToken []byte
}

// NewQueryStatRequest creates a new QueryStatRequest with a packed timestamp as the session ID.
func NewQueryStatRequest(challengeToken []byte) *QueryStatRequest {
	return &QueryStatRequest{
		SessionID:      packedTimestamp(),
		ChallengeToken: challengeToken,
	}
}

// ID returns the packet type for the query stat request packet.
func (p *QueryStatRequest) ID() int32 {
	return QueryTypeStat
}

// MarshalBinary serializes the QueryStatRequest into a byte slice.
func (p *QueryStatRequest) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	// magic + type + session ID + challenge token + padding
	buf.Grow(2 + 1 + 4 + 4 + 4)
	buf.Write(QueryStatHeader)
	buf.Write(p.SessionID)
	buf.Write(p.ChallengeToken)
	buf.Write(QueryStatPadding)

	return buf.Bytes(), nil
}

// QueryStatResponse the response to a QueryStatRequest, containing server statistics.
type QueryStatResponse struct {
	SessionID []byte
	KVPs      map[string]string
	Players   []string
}

// ID returns the packet type for the query stat response packet.
func (p *QueryStatResponse) ID() int32 {
	return QueryTypeStat
}

// UnmarshalBinary deserializes the QueryStatResponse from a byte slice.
func (p *QueryStatResponse) UnmarshalBinary(data []byte) error {
	rd := bytes.NewReader(data)
	r := bufio.NewReader(rd)

	typeByte, err := r.ReadByte()
	if err != nil {
		return err
	}
	if typeByte != byte(p.ID()) {
		return ErrUnexpectedPacket
	}

	p.SessionID = make([]byte, 4)
	if _, err := r.Read(p.SessionID); err != nil {
		return err
	}

	// Skip splitnum\x00, numPackets: \0xXX, and leading object number byte
	_, err = r.Read(make([]byte, 11))
	if err != nil {
		return err
	}

	// TODO: BungeeCord returns the MOTD (in the "hostname" entry) with the section symbol as a raw 0xA7

	// Read key-value pairs
	p.KVPs = make(map[string]string)
	kvps, err := readNullTerminatedStrings(r)
	if err != nil {
		return err
	}
	for i := 0; i < len(kvps); i += 2 {
		p.KVPs[kvps[i]] = kvps[i+1]
	}

	// Skip padding: \x01player_\x00\x00
	if _, err := r.Read(make([]byte, 10)); err != nil {
		return err
	}

	// Read player names
	p.Players, err = readNullTerminatedStrings(r)
	if err != nil {
		return err
	}

	return nil
}

// -------------------- Basic Query --------------------

// https://minecraft.wiki/w/Query

// plugins [SERVER_MOD_NAME[: PLUGIN_NAME(; PLUGIN_NAME...)]]

// QueryBasicStatRequest is a request packet for querying basic server statistics from a Minecraft server.
type QueryBasicStatRequest struct {
	SessionID      []byte
	ChallengeToken []byte
}

// NewQueryBasicStatRequest creates a new QueryChallengeRequest with a packed timestamp as the session ID.
func NewQueryBasicStatRequest(challengeToken []byte) *QueryBasicStatRequest {
	return &QueryBasicStatRequest{
		SessionID:      packedTimestamp(),
		ChallengeToken: challengeToken,
	}
}

// ID returns the packet type for the query basic stat request packet.
func (p *QueryBasicStatRequest) ID() int32 {
	return QueryTypeStat
}

// MarshalBinary serializes the QueryBasicStatRequest into a byte slice.
func (p *QueryBasicStatRequest) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	// magic + type + session ID + challenge token
	buf.Grow(2 + 1 + 4 + 4)
	buf.Write(QueryStatHeader)
	buf.Write(p.SessionID)
	buf.Write(p.ChallengeToken)

	return buf.Bytes(), nil
}

// UnmarshalBinary deserializes the QueryBasicStatRequest from a byte slice.
func (p *QueryBasicStatRequest) UnmarshalBinary(data []byte) error {
	rd := bytes.NewReader(data)
	r := bufio.NewReader(rd)

	magic := make([]byte, 2)
	if _, err := r.Read(magic); err != nil {
		return err
	}
	if magic[0] != GSQueryV4MSB || magic[1] != GSQueryV4LSB {
		return ErrUnexpectedPacket
	}

	typeByte, err := r.ReadByte()
	if err != nil {
		return err
	}
	if typeByte != byte(p.ID()) {
		return ErrUnexpectedPacket
	}

	p.SessionID = make([]byte, 4)
	if _, err := r.Read(p.SessionID); err != nil {
		return err
	}

	p.ChallengeToken = make([]byte, 4)
	if _, err := r.Read(p.ChallengeToken); err != nil {
		return err
	}

	return nil
}

// QueryBasicStatResponse is a response packet containing statistics from the Minecraft server.
type QueryBasicStatResponse struct {
	SessionID  []byte
	MOTD       string
	GameType   string
	Map        string
	NumPlayers int
	MaxPlayers int
	HostPort   uint16
	HostIP     string
}

// ID returns the packet type for the query basic stat response packet.
func (p *QueryBasicStatResponse) ID() int32 {
	return QueryTypeStat
}

// UnmarshalBinary deserializes the QueryBasicStatResponse from a byte slice.
func (p *QueryBasicStatResponse) UnmarshalBinary(data []byte) error {
	rd := bytes.NewReader(data)
	r := bufio.NewReader(rd)

	typeByte, err := r.ReadByte()
	if err != nil {
		return err
	}
	if typeByte != byte(p.ID()) {
		return ErrUnexpectedPacket
	}

	p.SessionID = make([]byte, 4)
	if _, err := r.Read(p.SessionID); err != nil {
		return err
	}

	p.MOTD, err = readNullTerminatedString(r)
	if err != nil {
		return err
	}
	p.GameType, err = readNullTerminatedString(r)
	if err != nil {
		return err
	}
	p.Map, err = readNullTerminatedString(r)
	if err != nil {
		return err
	}
	numPlayers, err := readNullTerminatedString(r)
	if err != nil {
		return err
	}
	maxPlayers, err := readNullTerminatedString(r)
	if err != nil {
		return err
	}

	p.NumPlayers, err = strconv.Atoi(numPlayers)
	if err != nil {
		return err
	}

	p.MaxPlayers, err = strconv.Atoi(maxPlayers)
	if err != nil {
		return err
	}

	var buf [2]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return err
	}
	p.HostPort = binary.LittleEndian.Uint16(buf[:])

	p.HostIP, err = readNullTerminatedString(r)
	if err != nil {
		return err
	}

	return nil
}

// MarshalBinary serializes the QueryBasicStatResponse into a byte slice.
func (p *QueryBasicStatResponse) MarshalBinary() ([]byte, error) {
	numPlayers := strconv.Itoa(p.NumPlayers)
	maxPlayers := strconv.Itoa(p.MaxPlayers)

	var buf bytes.Buffer
	// type byte + session ID + MOTD + gametype + map + numplayers + maxplayers + hostport + hostip
	buf.Grow(1 + 4 + len(p.MOTD) + 1 + len(p.GameType) + 1 + len(p.Map) + 1 +
		len(numPlayers) + 1 + len(maxPlayers) + 1 + 2 + len(p.HostIP) + 1)

	buf.WriteByte(byte(p.ID()))
	buf.Write(p.SessionID)
	buf.WriteString(p.MOTD)
	buf.WriteByte(0)
	buf.WriteString(p.GameType)
	buf.WriteByte(0)
	buf.WriteString(p.Map)
	buf.WriteByte(0)
	buf.WriteString(numPlayers)
	buf.WriteByte(0)
	buf.WriteString(maxPlayers)
	buf.WriteByte(0)

	hostPortBytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(hostPortBytes, p.HostPort)
	buf.Write(hostPortBytes)

	buf.WriteString(p.HostIP)
	buf.WriteByte(0)

	return buf.Bytes(), nil
}
