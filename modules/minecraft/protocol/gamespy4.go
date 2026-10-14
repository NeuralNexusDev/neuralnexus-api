package protocol

import (
	"bufio"
	"bytes"
	"encoding/binary"
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
	QueryStatHeader  = []byte{GSQueryV4MSB, GSQueryV4LSB, QueryTypeStat}
	QueryStatPadding = []byte{0xFF, 0xFF, 0xFF, 0x01}
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

	buf.Write([]byte{GSQueryV4MSB, GSQueryV4LSB, QueryTypeChallenge})
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
