package realtime

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// writeQueueSize bounds the per connection outbound queue. A client that cannot
// drain it is disconnected instead of stalling the delivery path.
const writeQueueSize = 64

// writeTimeout caps a single frame write so a stuck peer cannot pin a writer.
const writeTimeout = 10 * time.Second

// Connection is one authenticated WebSocket peer. Only the writer goroutine
// started by WriteLoop touches the socket; other goroutines queue payloads
// through SendRaw.
type Connection struct {
	ID       string
	UserUUID string

	socket *websocket.Conn
	send   chan []byte

	closeOnce sync.Once
	done      chan struct{}
}

func NewConnection(socket *websocket.Conn) *Connection {
	return &Connection{
		ID:     uuid.NewString(),
		socket: socket,
		send:   make(chan []byte, writeQueueSize),
		done:   make(chan struct{}),
	}
}

// Done is closed once the connection is torn down.
func (c *Connection) Done() <-chan struct{} { return c.done }

// SetReadDeadline bounds how long the next inbound frame may take. The read
// side belongs to the serving goroutine, which sets a fresh deadline per frame.
func (c *Connection) SetReadDeadline(deadline time.Time) error {
	return c.socket.SetReadDeadline(deadline)
}

// SetReadLimit caps the size of an inbound frame.
func (c *Connection) SetReadLimit(limit int64) { c.socket.SetReadLimit(limit) }

// ReadMessage reads one inbound frame. Only the serving goroutine calls it.
func (c *Connection) ReadMessage() (int, []byte, error) {
	return c.socket.ReadMessage()
}

// SendRaw queues a frame. It returns false when the queue is full, which means
// the peer is too slow to keep up; the caller should close the connection.
func (c *Connection) SendRaw(payload []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.send <- payload:
		return true
	default:
		return false
	}
}

// SendEnvelope marshals and queues an envelope, returning false when the frame
// could not be queued.
func (c *Connection) SendEnvelope(envelope Envelope) bool {
	payload, err := json.Marshal(envelope)
	if err != nil {
		return false
	}
	return c.SendRaw(payload)
}

// WriteLoop owns the socket's write side until the connection closes.
func (c *Connection) WriteLoop() {
	for {
		select {
		case <-c.done:
			return
		case payload := <-c.send:
			_ = c.socket.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.socket.WriteMessage(websocket.TextMessage, payload); err != nil {
				c.Close()
				return
			}
		}
	}
}

// Close tears the connection down exactly once and unblocks the writer.
func (c *Connection) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.socket != nil {
			_ = c.socket.Close()
		}
	})
}

// ReadPayloads reads text frames until the peer disconnects, the read deadline
// expires, or the payload exceeds the configured limit.
func (c *Connection) ReadPayloads(readTimeout time.Duration, handle func([]byte) error) error {
	for {
		if readTimeout > 0 {
			_ = c.socket.SetReadDeadline(time.Now().Add(readTimeout))
		}
		_, payload, err := c.socket.ReadMessage()
		if err != nil {
			return err
		}
		if err := handle(payload); err != nil {
			return err
		}
	}
}
