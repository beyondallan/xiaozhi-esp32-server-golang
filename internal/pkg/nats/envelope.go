package nats

import (
	"encoding/json"
	"fmt"
	"time"
)

// Envelope is the standard message envelope for all NATS messages.
type Envelope struct {
	Version   string      `json:"version"`
	EventID   string      `json:"event_id"`
	Timestamp time.Time   `json:"timestamp"`
	Source    string      `json:"source"`
	Subject   string      `json:"subject"`
	Payload   interface{} `json:"payload"`
}

// ParseEnvelope parses a NATS message into an Envelope.
func ParseEnvelope(data []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parse envelope: %w", err)
	}
	return &env, nil
}

// ParsePayload parses the payload field of an Envelope into the given target.
func ParsePayload(payload interface{}, target interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}
	return nil
}

// Marshal marshals the Envelope to JSON bytes.
func (e *Envelope) Marshal() ([]byte, error) {
	return json.Marshal(e)
}
