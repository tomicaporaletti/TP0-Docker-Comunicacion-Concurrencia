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


// StartClientLoop implementa el bucle principal: lee batches en streaming desde
// el CSV y los envía secuencialmente; al finalizar, notifica el fin y consulta ganadores.
func (c *Client) StartClientLoop(ctx context.Context, maxBatch int) {
	agencyID, _ := strconv.Atoi(c.config.ID)
	br, err := NewBatchFileReader(c.config.DataFile, agencyID, maxBatch)
	if err != nil {
		log.Criticalf("action: read_csv_open | result: fail | error: %v", err)
		return
	}
	defer br.Close()

	if err := c.loopBatches(ctx, br); err != nil {
		return
	}
	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
}

// loopBatches itera pidiendo batches al lector y enviándolos; respeta cancelación
// por contexto y pacing entre envíos.
func (c *Client) loopBatches(ctx context.Context, br *BatchFileReader) error {
	for {
		if c.ctxCancelled(ctx) {
			c.cleanup()
			return errors.New("cancelled")
		}
		batch, off, err := br.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			log.Errorf("action: batch_next | result: fail | error: %v", err)
			return err
		}
		if err := c.sendIfAny(batch, off); err != nil {
			return err
		}
		if c.waitOrCancel(ctx, c.config.LoopPeriod) {
			c.cleanup()
			return errors.New("cancelled")
		}
	}
	return nil
}

// sendIfAny envía un batch no vacío y registra métricas/offset para debugging.
func (c *Client) sendIfAny(batch []BetRecord, off int64) error {
	if len(batch) == 0 {
		return nil
	}
	if err := c.sendBatch(batch); err != nil {
		log.Errorf("action: send_batch | result: fail | file_offset: %d | error: %v", off, err)
		return err
	}
	log.Infof("action: batch_sent | result: success | count: %d | file_offset: %d", len(batch), off)
	return nil
}

// waitOrCancel espera el período configurado o sale si el contexto fue cancelado.
func (c *Client) waitOrCancel(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return true
	case <-time.After(d):
		return false
	}
}

// ctxCancelled consulta no bloqueante si el contexto ya fue cancelado.
func (c *Client) ctxCancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}




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



// Cleanup final
func (c *Client) cleanup() {
	if c.conn != nil {
		c.conn.Close()
	}
	log.Infof("action: client_stop | result: success | client_id: %v", c.config.ID)
}

