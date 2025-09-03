package common

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

// Constantes del protocolo
const (
	MSG_TYPE_BATCH      = 2
	MSG_TYPE_NOTIFY_END = 3
	MSG_TYPE_QUERY_WIN  = 4
	MSG_TYPE_CONFIRM    = 100
	MSG_TYPE_WINNERS    = 101
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

// Serializa una sola apuesta
func SerializeOneBet(b BetRecord) ([]byte, error) {
    var out bytes.Buffer
    binary.Write(&out, binary.BigEndian, uint32(b.Agency))
    for _, s := range []string{b.Document, b.FirstName, b.LastName, b.Birthdate} {
        part, err := encodeString(s)
        if err != nil {
            return nil, err
        }
        out.Write(part)
    }
    binary.Write(&out, binary.BigEndian, uint32(b.Number))
    return out.Bytes(), nil
}

// SerializeBatch serializa un slice de apuestas como batch
// Protocolo: [1 byte type=2][2 bytes cantidad][cada bet como en SerializeBet pero sin type]
func SerializeBatch(bets []BetRecord) ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte(MSG_TYPE_BATCH)

	// cantidad de bets (2 bytes big endian)
	binary.Write(&out, binary.BigEndian, uint16(len(bets)))

    for _, b := range bets {
        betBytes, err := SerializeOneBet(b)
        if err != nil {
            return nil, err
        }
        out.Write(betBytes)
    }
	return out.Bytes(), nil
}


// SerializeNotifyEnd arma el mensaje de notificación 
// Protocolo: [1 byte type=3][4 bytes agency]
func SerializeNotifyEnd(agency int) ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte(MSG_TYPE_NOTIFY_END)
	binary.Write(&out, binary.BigEndian, uint32(agency))
	return out.Bytes(), nil
}

// SerializeQueryWinners arma el mensaje de consulta 
// Protocolo: [1 byte type=4][4 bytes agency]
func SerializeQueryWinners(agency int) ([]byte, error) {
	var out bytes.Buffer
	out.WriteByte(MSG_TYPE_QUERY_WIN)
	binary.Write(&out, binary.BigEndian, uint32(agency))
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


// DecodeWinners lee los ganadores
// Protocolo: [1 byte type=101][2 bytes cantidad de strings][dni str...]
func DecodeWinners(conn net.Conn) ([]string, error) {
	header := make([]byte, 3)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}
	if header[0] != MSG_TYPE_WINNERS {
		return nil, fmt.Errorf("unexpected msg type %d", header[0])
	}
	count := int(binary.BigEndian.Uint16(header[1:3]))
	var winners []string
	for i := 0; i < count; i++ {
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return nil, err
		}
		size := int(lenBuf[0])
		data := make([]byte, size)
		if _, err := io.ReadFull(conn, data); err != nil {
			return nil, err
		}
		winners = append(winners, string(data))
	}
	return winners, nil
}