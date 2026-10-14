package protocol

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"strconv"
)

// https://wiki.unrealadmin.org/UT3_query_protocol
// plugins [SERVER_MOD_NAME[: PLUGIN_NAME(; PLUGIN_NAME...)]]

const (
	QueryMagicMSB      = 0xFE
	QueryMagicLSB      = 0xFD
	QueryTypeChallenge = 0x09
	QueryTypeStat      = 0x00
)

var (
	QuerySessionId        = []byte{0x61, 0x42, 0x73, 0x34}
	QueryChallengePayload = []byte{
		QueryMagicMSB, QueryMagicLSB, QueryTypeChallenge,
		QuerySessionId[0], QuerySessionId[1], QuerySessionId[2], QuerySessionId[3],
	}
	QueryBasicStatPayload = []byte{
		QueryMagicMSB, QueryMagicLSB, QueryTypeStat,
		QuerySessionId[0], QuerySessionId[1], QuerySessionId[2], QuerySessionId[3],
	}
)

type QueryChallengeResponse struct {
	SessionID      []byte
	ChallengeToken []byte
}

func (p *QueryChallengeResponse) ID() int32 {
	return QueryTypeChallenge
}

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

	challengeStr, err := r.ReadString('\000')
	if err != nil {
		return err
	}
	challengeStr = challengeStr[:len(challengeStr)-1] // Remove the null terminator

	challengeToken, err := strconv.ParseInt(challengeStr, 10, 32)
	if err != nil {
		return err
	}

	var challengeBE [4]byte
	binary.BigEndian.PutUint32(challengeBE[:], uint32(challengeToken))

	p.ChallengeToken = challengeBE[:]

	return nil
}

func (p *QueryChallengeResponse) MarshalBinary() ([]byte, error) {
	challengeInt := binary.BigEndian.Uint32(p.ChallengeToken)
	challengeStr := strconv.Itoa(int(challengeInt))

	var buf bytes.Buffer
	// type byte + session ID + challenge token string + null terminator
	buf.Grow(1 + 4 + len(challengeStr) + 1)

	buf.WriteByte(byte(p.ID()))
	buf.Write(p.SessionID)
	buf.WriteString(challengeStr)
	buf.WriteByte(0) // null terminator

	return buf.Bytes(), nil
}

type QueryBasicStatRequest struct {
	SessionID      []byte
	ChallengeToken []byte
}

func NewQueryBasicStatRequest(sessionID, challengeToken []byte) *QueryBasicStatRequest {
	return &QueryBasicStatRequest{
		SessionID:      sessionID,
		ChallengeToken: challengeToken,
	}
}

func (p *QueryBasicStatRequest) ID() int32 {
	return QueryTypeStat
}

func (p *QueryBasicStatRequest) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	// magic + type + session ID + challenge token
	buf.Grow(2 + 1 + 4 + 4)
	buf.Write(QueryBasicStatPayload)
	buf.Write(p.ChallengeToken)

	return buf.Bytes(), nil
}

func (p *QueryBasicStatRequest) UnmarshalBinary(data []byte) error {
	rd := bytes.NewReader(data)
	r := bufio.NewReader(rd)

	magic := make([]byte, 2)
	if _, err := r.Read(magic); err != nil {
		return err
	}
	if magic[0] != QueryMagicMSB || magic[1] != QueryMagicLSB {
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

func (p *QueryBasicStatResponse) ID() int32 {
	return QueryTypeStat
}

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

type QueryFullStatRequest struct {
	SessionID      []byte
	ChallengeToken []byte
}

func NewQueryFullStatRequest(sessionID, challengeToken []byte) *QueryFullStatRequest {
	return &QueryFullStatRequest{
		SessionID:      sessionID,
		ChallengeToken: challengeToken,
	}
}

func (p *QueryFullStatRequest) ID() int32 {
	return QueryTypeStat
}

func (p *QueryFullStatRequest) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	// magic + type + session ID + challenge token + padding
	buf.Grow(2 + 1 + 4 + 4 + 4)
	buf.Write(QueryBasicStatPayload)
	buf.Write(p.ChallengeToken)
	buf.Write([]byte{0, 0, 0, 0}) // padding

	return buf.Bytes(), nil
}

type QueryFullStatResponse struct {
	SessionID []byte
	KVPs      map[string]string
	Players   []string
}

func (p *QueryFullStatResponse) ID() int32 {
	return QueryTypeStat
}

func (p *QueryFullStatResponse) UnmarshalBinary(data []byte) error {
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

	// TODO: Learn what the padding does
	// Skip the 11 bytes of padding
	//padding := make([]byte, 10)
	//if _, err := r.Read(padding); err != nil {
	//	return err
	//}

	_, err = readNullTerminatedString(r)
	if err != nil {
		return err
	}
	_, err = readNullTerminatedString(r)
	if err != nil {
		return err
	}

	p.KVPs = make(map[string]string)
	kvps, err := readNullTerminatedStrings(r)
	if err != nil {
		return err
	}
	for i := 0; i < len(kvps); i += 2 {
		p.KVPs[kvps[i]] = kvps[i+1]
	}

	// TODO: Learn what the padding does
	// Skip the 10 bytes of padding
	padding := make([]byte, 10)
	if _, err := r.Read(padding); err != nil {
		return err
	}

	p.Players, err = readNullTerminatedStrings(r)
	if err != nil {
		return err
	}

	return nil
}
