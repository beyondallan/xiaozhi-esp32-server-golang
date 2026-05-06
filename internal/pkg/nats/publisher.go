package nats

// NATSPublisher is the common interface for NATS event publishing.
// All modules use this interface to publish events without direct coupling.
type NATSPublisher interface {
	Publish(subject string, payload interface{}) error
}
