package common

import (
	"context"
	"encoding/json"
	"net"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

// estructura mínima para parsear la respuesta
type Response struct {
	Type   string `json:"type"`
	Result string `json:"result"`
	Reason string `json:"reason,omitempty"`
}


// ClientConfig Configuration used by the client
type ClientConfig struct {
	ID            string
	ServerAddress string
	LoopAmount    int
	LoopPeriod    time.Duration
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
func (c *Client) StartClientLoop(ctx context.Context) {
	for msgID := 1; msgID <= c.config.LoopAmount; msgID++ {
		select {
		case <-ctx.Done():
			// Señal recibida, terminar graceful
			c.cleanup()
			return
		default:
			if err := c.runIteration(msgID); err != nil {
				return
			}
			time.Sleep(c.config.LoopPeriod)
		}
	}
	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
}


// runIteration: conectar → enviar apuesta → leer respuesta → cerrar
func (c *Client) runIteration(_msgID int) error {
	if err := c.createClientSocket(); err != nil {
		return err
	}
	defer c.conn.Close()
	_ = c.conn.SetDeadline(time.Now().Add(15 * time.Second))

	// 1. Construir payload
	bet, payload, err := c.buildPayload()
	if err != nil {
		return err
	}

	// 2. Enviar apuesta
	if err := c.sendBet(payload, bet); err != nil {
		return err
	}

	// 3. Leer confirmación
	return c.readConfirmation(bet)
}


// buildPayload: arma la apuesta desde env y la serializa binario
func (c *Client) buildPayload() (*BetMessage, []byte, error) {
	bet, err := NewBetFromEnv(c.config.ID)
	if err != nil {
		c.logFail("build_bet", "", 0, err)
		return nil, nil, err
	}
	payload, err := SerializeBet(bet)
	if err != nil {
		c.logFail("serialize_bet", bet.Document, bet.Number, err)
		return nil, nil, err
	}
	return bet, payload, nil
}

// sendBet: escribe en el socket
func (c *Client) sendBet(payload []byte, bet *BetMessage) error {
	if err := writeAll(c.conn, payload); err != nil {
		c.logFail("send_message", bet.Document, bet.Number, err)
		return err
	}
	return nil
}

// readConfirmation: espera el ACK del server
func (c *Client) readConfirmation(bet *BetMessage) error {
	success, err := DecodeConfirmation(c.conn)
	if err != nil {
		c.logFail("receive_message", bet.Document, bet.Number, err)
		return err
	}
	if success {
		log.Infof("action: apuesta_enviada | result: success | dni: %v | numero: %v",
			bet.Document, bet.Number)
	} else {
		log.Errorf("action: apuesta_enviada | result: fail | dni: %v | numero: %v",
			bet.Document, bet.Number)
	}
	return nil
}

func (c *Client) handleResponse(resp string, bet *BetMessage) {
	var r Response
	if err := json.Unmarshal([]byte(resp), &r); err != nil {
		log.Errorf("action: parse_response | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
		return
	}

	if r.Result == "success" {
		log.Infof("action: apuesta_enviada | result: success | dni: %v | numero: %v",
			bet.Document, bet.Number)
	} else {
		log.Errorf("action: apuesta_enviada | result: fail | dni: %v | numero: %v | reason: %v",
			bet.Document, bet.Number, r.Reason)
	}
}
// logFail helper genérico de error
func (c *Client) logFail(action, dni string, numero int, err error) {
	log.Errorf("action: %s | result: fail | client_id: %v | dni: %v | numero: %v | error: %v",
		action, c.config.ID, dni, numero, err)
}



// Cleanup final
func (c *Client) cleanup() {
	if c.conn != nil {
		c.conn.Close()
	}
	log.Infof("action: client_stop | result: success | client_id: %v", c.config.ID)
}

