package common

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
)

// Constantes del protocolo
const (
	MSG_TYPE_BET     = 1
	MSG_TYPE_CONFIRM = 100
)

// writeAll: evita short-write
func writeAll(conn net.Conn, data []byte) error {
	total := 0
	for total < len(data) {
		n, err := conn.Write(data[total:])
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("socket closed while sending")
		}
		total += n
	}
	return nil
}

// encodeString serializa como [1 byte length][bytes]
func encodeString(s string) ([]byte, error) {
	if len(s) > 255 {
		return nil, fmt.Errorf("string too long")
	}
	buf := []byte{byte(len(s))}
	buf = append(buf, []byte(s)...)
	return buf, nil
}

// SerializeBet arma el mensaje binario para enviar una apuesta
/*
    Protocolo:
      [1 byte type]
      [4 bytes agency]
      [dni str] [first str] [last str] [birth str]
      [4 bytes number]

    Protocolo usa Big Endiann
*/
func SerializeBet(b *BetMessage) ([]byte, error) {
	var out bytes.Buffer

	// tipo
	out.WriteByte(MSG_TYPE_BET)

	// agency: 4 bytes big-endian
	agencyInt, err := strconv.Atoi(b.Agency)
	if err != nil {
		return nil, err
	}
	binary.Write(&out, binary.BigEndian, uint32(agencyInt))

	// campos string
	for _, s := range []string{b.Document, b.FirstName, b.LastName, b.Birthdate} {
		part, err := encodeString(s)
		if err != nil {
			return nil, err
		}
		out.Write(part)
	}

	// number: 4 bytes big-endian
	binary.Write(&out, binary.BigEndian, uint32(b.Number))

	return out.Bytes(), nil
}

// DecodeConfirmation lee 2 bytes y devuelve success/fail
func DecodeConfirmation(conn net.Conn) (bool, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return false, err
	}
	if header[0] != MSG_TYPE_CONFIRM {
		return false, fmt.Errorf("unexpected msg type %d", header[0])
	}
	return header[1] == 1, nil
}
