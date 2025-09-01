package common

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

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

func writeAll(conn net.Conn, s string) error {
	p 	:= append([]byte(s), '\n')
	for len(p) > 0 {
		n, err := conn.Write(p)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("socket closed while sending")
		}
		p = p[n:]
	}
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


// Una iteración: conectar → enviar → leer → cerrar
func (c *Client) runIteration(msgID int) error {
	if err := c.createClientSocket(); err != nil {
		return err
	}
	_ = c.conn.SetDeadline(time.Now().Add(15 * time.Second))

	line := fmt.Sprintf("[CLIENT %v] Message N°%v", c.config.ID, msgID)
	if err := writeAll(c.conn, line); err != nil {
		log.Errorf("action: send_message | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
		c.conn.Close()
		return err
	}

	msg, err := bufio.NewReader(c.conn).ReadString('\n')
	c.conn.Close()
	if err != nil {
		log.Errorf("action: receive_message | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
		return err
	}

	log.Infof("action: receive_message | result: success | client_id: %v | msg: %v",
		c.config.ID, msg)
	return nil
}


// Cleanup final
func (c *Client) cleanup() {
	if c.conn != nil {
		c.conn.Close()
	}
	log.Infof("action: client_stop | result: success | client_id: %v", c.config.ID)
}

