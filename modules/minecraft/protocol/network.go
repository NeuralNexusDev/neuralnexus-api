package protocol

import (
	"bufio"
	"context"
	"io"
	"net"
)

func openConnection(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	return conn, nil
}

func send(conn net.Conn, payloads ...[]byte) error {
	buffers := net.Buffers(payloads)
	if _, err := buffers.WriteTo(conn); err != nil {
		return err
	}
	return nil
}

func receive(r *bufio.Reader) ([]byte, error) {
	size, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	if size < 0 {
		return nil, ErrNegativePacketSize
	}
	if size > MaxPacketSize {
		return nil, ErrPacketTooLarge
	}

	body := make([]byte, size)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}
