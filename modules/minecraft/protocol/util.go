package protocol

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"time"
	"unicode/utf16"
)

const (
	MaxStringLength = 1<<15 - 1
)

var (
	ErrPacketTooLarge       = errors.New("packet too large")
	ErrVarIntTooBig         = errors.New("VarInt too big")
	ErrStringNegativeLength = errors.New("string length is negative")
	ErrStringTooLong        = errors.New("string is too long")
)

// StreamReader is an interface that combines io.Reader and io.ByteReader.
type StreamReader interface {
	io.Reader
	io.ByteReader
}

// StreamWriter is an interface that combines io.Writer and io.ByteWriter.
type StreamWriter interface {
	io.Writer
	io.ByteWriter
}

// ------------------------------ MC Protocol ------------------------------

// readVarInt reads a Minecraft VarInt.
func readVarInt(r io.ByteReader) (int32, error) {
	var value uint32
	for position := uint(0); position < 32; position += 7 {
		currentByte, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, io.ErrUnexpectedEOF
			}
			return 0, err
		}

		value |= uint32(currentByte&0x7F) << position

		if currentByte&0x80 == 0 {
			return int32(value), nil
		}
	}
	return 0, ErrVarIntTooBig
}

// writeVarInt writes a Minecraft VarInt.
func writeVarInt(w io.ByteWriter, value int32) error {
	// Shifting an unsigned value fills with zeroes, like >>> does.
	v := uint32(value)
	for v&^0x7F != 0 {
		if err := w.WriteByte(byte(v&0x7F) | 0x80); err != nil {
			return err
		}
		v >>= 7
	}
	return w.WriteByte(byte(v))
}

// readUtf8MaxSize reads a Minecraft string, which is a VarInt length followed by UTF-8 bytes.
func readUtf8MaxSize(r StreamReader, maxSize int32) (string, error) {
	size, err := readVarInt(r)
	if err != nil {
		return "", err
	}
	if size < 0 {
		return "", ErrStringNegativeLength
	}
	if size > maxSize {
		return "", ErrStringTooLong
	}

	// Exit early if the reader has a known length and is too small
	if lr, ok := r.(interface{ Len() int }); ok {
		if int(size) > lr.Len() {
			return "", io.ErrUnexpectedEOF
		}
	}

	data, err := io.ReadAll(io.LimitReader(r, int64(size)))
	if err != nil {
		return "", err
	}
	if int32(len(data)) < size {
		return "", io.ErrUnexpectedEOF
	}

	return string(data), nil
}

// readUtf8 reads a Minecraft string, which is a VarInt length followed by UTF-8 bytes.
func readUtf8(r StreamReader) (string, error) {
	return readUtf8MaxSize(r, MaxStringLength)
}

// writeUtf8MaxSize writes a Minecraft string, which is a VarInt length followed by UTF-8 bytes.
func writeUtf8MaxSize(w StreamWriter, s string, maxSize int32) error {
	if len(s) > int(maxSize) {
		return ErrStringTooLong
	}
	if err := writeVarInt(w, int32(len(s))); err != nil {
		return err
	}
	_, err := io.WriteString(w, s)
	return err
}

// writeUtf8 writes a Minecraft string, which is a VarInt length followed by UTF-8 bytes.
func writeUtf8(w StreamWriter, s string) error {
	return writeUtf8MaxSize(w, s, MaxStringLength)
}

// ------------------------------ Endian ------------------------------

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

// readUint16BE reads a big-endian 2-byte integer from any io.Reader.
func readUint16BE(r io.Reader) (uint16, error) {
	var buf [2]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(buf[:]), nil
}

// writeUint16BE writes a big-endian 2-byte integer to any io.Writer.
func writeUint16BE(w io.Writer, v uint16) error {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

// readUint32BE reads a big-endian 4-byte integer from any io.Reader.
func readUint32BE(r io.Reader) (uint32, error) {
	var buf [4]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint32(buf[:]), nil
}

// writeUint32BE writes a big-endian 4-byte integer to any io.Writer.
func writeUint32BE(w io.Writer, v uint32) error {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], v)
	_, err := w.Write(buf[:])
	return err
}

// ------------------------------ Legacy Ping Strings ------------------------------

// readUTF16BEString reads a 2-byte length in UTF-16 code units followed by the UTF-16BE
// code units from the reader, returning a Go string.
func readUTF16BEString(r io.Reader, maxLen uint16) (string, error) {
	charLen, err := readUint16BE(r)
	if err != nil {
		return "", err
	}
	if charLen == 0 {
		return "", nil
	}
	if charLen > maxLen {
		return "", ErrStringTooLong
	}

	units := make([]uint16, charLen)
	if err := binary.Read(r, binary.BigEndian, units); err != nil {
		return "", err
	}
	return string(utf16.Decode(units)), nil
}

// writeUTF16BEUnits writes UTF-16 code units as a 2-byte length followed by the UTF-16BE code units.
func writeUTF16BEUnits(w io.Writer, units []uint16) error {
	if err := writeUint16BE(w, uint16(len(units))); err != nil {
		return err
	}

	var unitBytes [2]byte
	for _, u := range units {
		binary.BigEndian.PutUint16(unitBytes[:], u)
		if _, err := w.Write(unitBytes[:]); err != nil {
			return err
		}
	}
	return nil
}

// ------------------------------ Null-Terminated Strings ------------------------------

func readNullTerminatedString(r *bufio.Reader) (string, error) {
	str, err := r.ReadString('\x00')
	if err != nil {
		return "", err
	}
	return str[:len(str)-1], nil
}

func readNullTerminatedStrings(r *bufio.Reader) ([]string, error) {
	var strings []string
	for {
		// Peek to check to see if the text two bytes are \0x00\0x01
		peek, err := r.Peek(2)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		// Special case for Minecraft's Query protocol implementation
		if len(peek) == 2 && peek[0] == 0x00 && (peek[1]&0x0F) == 0x01 {
			// Consume the null terminator
			if _, err := r.Discard(1); err != nil {
				return nil, err
			}
			break
		}
		str, err := readNullTerminatedString(r)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		strings = append(strings, str)
	}
	return strings, nil
}

// ------------------------------ GameSpy v4 ------------------------------

func packedTimestamp() []byte {
	now := int32(time.Now().Unix())
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(now))
	return buf[:]
}
