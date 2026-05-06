package nats

import (
	"fmt"

	natslib "github.com/nats-io/nats.go"
	log "github.com/sirupsen/logrus"
)

// EventHandler is a callback function for handling NATS events.
type EventHandler func(subject string, data []byte) error

// EventDispatcher manages NATS subscriptions and dispatches events to handlers.
type EventDispatcher struct {
	client     *Client
	subjects   map[string][]EventHandler
	subscribed map[string]bool
}

// NewEventDispatcher creates a new EventDispatcher.
func NewEventDispatcher(client *Client) *EventDispatcher {
	return &EventDispatcher{
		client:     client,
		subjects:   make(map[string][]EventHandler),
		subscribed: make(map[string]bool),
	}
}

// Subscribe registers a handler for a NATS subject.
func (d *EventDispatcher) Subscribe(subject string, handler EventHandler) {
	d.subjects[subject] = append(d.subjects[subject], handler)
}

// Start subscribes to all registered subjects and begins dispatching events.
func (d *EventDispatcher) Start() error {
	for subject, handlers := range d.subjects {
		if d.subscribed[subject] {
			continue
		}

		h := handlers // capture for closure
		subj := subject
		_, err := d.client.conn.Subscribe(subject, func(msg *natslib.Msg) {
			for _, handler := range h {
				if err := handler(subj, msg.Data); err != nil {
					log.WithFields(log.Fields{
						"subject": subj,
						"error":   err,
					}).Error("failed to handle NATS event")
				}
			}
		})
		if err != nil {
			return fmt.Errorf("subscribe to %s: %w", subject, err)
		}
		d.subscribed[subject] = true
		log.WithField("subject", subject).Info("subscribed to NATS subject")
	}
	return nil
}
