// Package screen is the engine's client: it draws what the engine tells it to
// draw and hands back keys. No business rule lives here.
package screen

import (
	"encoding/json"
	"net"
	"sync"

	"github.com/andreluiz/tesseract/internal/protocol"
)

// Client is the screen's conversation with the engine.
type Client struct {
	conn    net.Conn
	encoder *json.Encoder
	decoder *json.Decoder
	mutex   sync.Mutex
}

// Connect opens the conversation with the engine.
func Connect(socketPath string) (*Client, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:    conn,
		encoder: json.NewEncoder(conn),
		decoder: json.NewDecoder(conn),
	}, nil
}

// Send sends a request to the engine.
func (c *Client) Send(kind string, data any) error {
	envelope, err := protocol.Pack(kind, data)
	if err != nil {
		return err
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.encoder.Encode(envelope)
}

// Receive waits for the next message from the engine.
func (c *Client) Receive() (protocol.Message, error) {
	var envelope protocol.Message
	err := c.decoder.Decode(&envelope)
	return envelope, err
}

// Close ends the conversation. The engine keeps running — nothing dies.
func (c *Client) Close() error { return c.conn.Close() }
