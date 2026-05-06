package nats

import (
	"encoding/json"
	"fmt"
	"time"

	natslib "github.com/nats-io/nats.go"
)

// Config holds the NATS configuration.
type Config struct {
	URL           string
	MaxReconnects int
	ReconnectWait int // seconds
	PingInterval  int // seconds
}

// Client wraps the NATS connection and JetStream context.
type Client struct {
	conn *natslib.Conn
	js   natslib.JetStreamContext
}

// New creates a new NATS client.
func New(cfg Config) (*Client, error) {
	opts := []natslib.Option{
		natslib.MaxReconnects(cfg.MaxReconnects),
		natslib.ReconnectWait(time.Duration(cfg.ReconnectWait) * time.Second),
		natslib.DisconnectErrHandler(func(nc *natslib.Conn, err error) {
			fmt.Printf("NATS disconnected: %v\n", err)
		}),
		natslib.ReconnectHandler(func(nc *natslib.Conn) {
			fmt.Printf("NATS reconnected to %s\n", nc.ConnectedUrl())
		}),
		natslib.ClosedHandler(func(nc *natslib.Conn) {
			fmt.Printf("NATS connection closed\n")
		}),
	}

	if cfg.PingInterval > 0 {
		opts = append(opts, natslib.PingInterval(time.Duration(cfg.PingInterval)*time.Second))
	}

	conn, err := natslib.Connect(cfg.URL, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &Client{
		conn: conn,
		js:   js,
	}, nil
}

// Publish publishes a message to the given subject.
func (c *Client) Publish(subject string, payload interface{}) error {
	envelope := Envelope{
		Version:   "1.0",
		EventID:   fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now().UTC(),
		Source:    "xiaozhi",
		Subject:   subject,
		Payload:   payload,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal envelope: %w", err)
	}

	return c.conn.Publish(subject, data)
}

// PublishAsync publishes a message asynchronously.
func (c *Client) PublishAsync(subject string, payload interface{}) (natslib.PubAckFuture, error) {
	envelope := Envelope{
		Version:   "1.0",
		EventID:   fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now().UTC(),
		Source:    "xiaozhi",
		Subject:   subject,
		Payload:   payload,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal envelope: %w", err)
	}

	return c.js.PublishAsync(subject, data)
}

// Subscribe subscribes to the given subject.
func (c *Client) Subscribe(subject string, handler func(msg *natslib.Msg)) (*natslib.Subscription, error) {
	return c.conn.Subscribe(subject, handler)
}

// QueueSubscribe subscribes to the given subject with a queue group.
func (c *Client) QueueSubscribe(subject, queue string, handler func(msg *natslib.Msg)) (*natslib.Subscription, error) {
	return c.conn.QueueSubscribe(subject, queue, handler)
}

// JetStreamSubscribe subscribes to a JetStream subject.
func (c *Client) JetStreamSubscribe(subject, durable string, handler func(msg *natslib.Msg)) (*natslib.Subscription, error) {
	return c.js.Subscribe(subject, handler, natslib.Durable(durable), natslib.AckExplicit())
}

// Request sends a request and waits for a reply.
func (c *Client) Request(subject string, payload interface{}, timeout time.Duration) (*natslib.Msg, error) {
	envelope := Envelope{
		Version:   "1.0",
		EventID:   fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now().UTC(),
		Source:    "xiaozhi",
		Subject:   subject,
		Payload:   payload,
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal envelope: %w", err)
	}

	return c.conn.Request(subject, data, timeout)
}

// PublishWithRetry publishes a message with retry logic.
func (c *Client) PublishWithRetry(subject string, payload interface{}, maxRetries int) error {
	var lastErr error
	for i := 0; i <= maxRetries; i++ {
		if err := c.Publish(subject, payload); err != nil {
			lastErr = err
			if i < maxRetries {
				time.Sleep(time.Duration(1<<uint(i)) * 100 * time.Millisecond)
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("publish failed after %d retries: %w", maxRetries, lastErr)
}

// Close closes the NATS connection.
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// IsConnected checks if the client is connected.
func (c *Client) IsConnected() bool {
	return c != nil && c.conn != nil && c.conn.IsConnected()
}

// ConnectedUrl returns the URL of the connected server.
func (c *Client) ConnectedUrl() string {
	if c == nil || c.conn == nil {
		return ""
	}
	return c.conn.ConnectedUrl()
}

// CreateStream creates a JetStream stream if it doesn't exist.
func (c *Client) CreateStream(name string, subjects []string) error {
	streamInfo, err := c.js.StreamInfo(name)
	if err == nil && streamInfo != nil {
		return nil // stream already exists
	}

	_, err = c.js.AddStream(&natslib.StreamConfig{
		Name:     name,
		Subjects: subjects,
		Storage:  natslib.FileStorage,
		MaxAge:   7 * 24 * time.Hour, // 7 days retention
	})
	if err != nil {
		return fmt.Errorf("failed to create stream %s: %w", name, err)
	}

	return nil
}

// DeleteStream deletes a JetStream stream.
func (c *Client) DeleteStream(name string) error {
	return c.js.DeleteStream(name)
}
