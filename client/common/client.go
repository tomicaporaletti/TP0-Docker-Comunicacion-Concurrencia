package common

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

// estructura mínima para parsear la respuesta
type Response struct {
	Type   string
	Result string
	Reason string
}


// ClientConfig Configuration used by the client
type ClientConfig struct {
	ID            string
	ServerAddress string
	LoopAmount    int
	LoopPeriod    time.Duration
    DataFile      string
}

// Client Entity that encapsulates how
type Client struct {
	config ClientConfig
	conn   net.Conn
}

// NewClient Initializes a new client receiving the configuration
// as a parameter
func NewClient(config ClientConfig) *Client {
	client := &Client{
		config: config,
	}
	return client
}

// CreateClientSocket Initializes client socket. In case of
// failure, error is printed in stdout/stderr and exit 1
// is returned
func (c *Client) createClientSocket() error {
	conn, err := net.Dial("tcp", c.config.ServerAddress)
	if err != nil {
		log.Criticalf(
			"action: connect | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
		return err
	}
	c.conn = conn
	return nil
}



// Bucle principal del cliente, cortable por signal vía ctx
func (c *Client) StartClientLoop(ctx context.Context, maxBatch int) {
	agencyID, _ := strconv.Atoi(c.config.ID)

	// 1) Crear lector por batches (streaming)
	br, err := NewBatchFileReader(c.config.DataFile, agencyID, maxBatch)
	if err != nil {
		log.Criticalf("action: read_csv_open | result: fail | error: %v", err)
		return
	}
	defer br.Close()

	for {
		// 2) Permitir cancelación
		select {
		case <-ctx.Done():
			c.cleanup()
			return
		default:
		}

		// 3) Pedir el próximo batch desde el archivo (sin cargar todo)
		batch, off, err := br.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Errorf("action: batch_next | result: fail | error: %v", err)
			// si hay un error no-EOF, podés decidir: retry, skip, o abortar. Acá abortamos.
			return
		}

		// 4) Enviar el batch
		if len(batch) > 0 {
			if err := c.sendBatch(batch); err != nil {
				// Podrías loguear offset para debug
				log.Errorf("action: send_batch | result: fail | file_offset: %d | error: %v", off, err)
				return
			}
			log.Infof("action: batch_sent | result: success | count: %d | file_offset: %d", len(batch), off)
		}

		// 5) Respeto de pacing entre envíos
		select {
		case <-ctx.Done():
			c.cleanup()
			return
		case <-time.After(c.config.LoopPeriod):
		}
	}

	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)

	// Notificar fin y consultar ganadores
	if err := c.notifyEnd(agencyID); err != nil {
		return
	}
	c.queryWinners(agencyID)
}

// Envia un batch entero
func (c *Client) sendBatch(batch []BetRecord) error {
	if err := c.createClientSocket(); err != nil {
		return err
	}
	defer c.conn.Close()
	_ = c.conn.SetDeadline(time.Now().Add(15 * time.Second))

	payload, err := SerializeBatch(batch)
	if err != nil {
		log.Errorf("action: serialize_batch | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return err
	}

	if err := writeAll(c.conn, payload); err != nil {
		log.Errorf("action: send_batch | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return err
	}

	success, err := DecodeConfirmation(c.conn)
	if err != nil {
		log.Errorf("action: receive_batch | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return err
	}
	if success {
		log.Infof("action: apuesta_enviada | result: success | client_id: %v | cantidad: %d", c.config.ID, len(batch))
	} else {
		log.Errorf("action: apuesta_enviada | result: fail | client_id: %v | cantidad: %d", c.config.ID, len(batch))
	}
	return nil
}

// Notifica al servidor que ya termino de enviar todos los batches.
func (c *Client) notifyEnd(agencyID int) error {
	if err := c.createClientSocket(); err != nil {
		return err
	}
	defer c.conn.Close()
	_ = c.conn.SetDeadline(time.Now().Add(10 * time.Second))

	payload, _ := SerializeNotifyEnd(agencyID)
	if err := writeAll(c.conn, payload); err != nil {
		log.Errorf("action: notify_end | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return err
	}
	_, err := DecodeConfirmation(c.conn)
	if err != nil {
		log.Errorf("action: notify_end | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return err
	}
	log.Infof("action: notify_end | result: success | client_id: %v", c.config.ID)
	return nil
}

// Solicita al servidor que le devuelva el ganador
// Timeout mayor, porque puede tener que esperar hasta que todas las agencias terminen
func (c *Client) queryWinners(agencyID int) {
	if err := c.createClientSocket(); err != nil {
		return
	}
	defer c.conn.Close()

	payload, _ := SerializeQueryWinners(agencyID)
	if err := writeAll(c.conn, payload); err != nil {
		log.Errorf("action: query_winners | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return
	}

	winners, err := DecodeWinners(c.conn)
	if err != nil {
		log.Errorf("action: consulta_ganadores | result: fail | client_id: %v | error: %v", c.config.ID, err)
		return
	}

	log.Infof("action: consulta_ganadores | result: success | cant_ganadores: %d", len(winners))
}



// Cleanup final
func (c *Client) cleanup() {
	if c.conn != nil {
		c.conn.Close()
	}
	log.Infof("action: client_stop | result: success | client_id: %v", c.config.ID)
}

