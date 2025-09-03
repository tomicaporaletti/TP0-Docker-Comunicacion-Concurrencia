package common

import (
	"context"
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
    bets, err := readBetsFromCSV(c.config.DataFile, agencyID)
    if err != nil {
        log.Criticalf("action: read_csv | result: fail | error: %v", err)
        return
    }

    batches := splitIntoBatches(bets, maxBatch)
    for _, batch := range batches {
        select {
        case <-ctx.Done():
            c.cleanup()
            return
        default:
            if err := c.sendBatch(batch); err != nil {
                return
            }
            time.Sleep(c.config.LoopPeriod)
        }
    }
    log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)

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
	_ = c.conn.SetDeadline(time.Now().Add(60 * time.Second))

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

