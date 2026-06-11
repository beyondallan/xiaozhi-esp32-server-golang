package chat

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	. "xiaozhi-esp32-server-golang/internal/data/client"
	. "xiaozhi-esp32-server-golang/internal/data/msg"
	log "xiaozhi-esp32-server-golang/logger"
)

// sharedEnvelope matches the Ti-social shared NATS envelope format
// (packages/pkg/nats/events.Envelope). We define it locally to avoid
// importing the shared module and its transitive dependencies.
type sharedEnvelope struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	TraceID   string    `json:"trace_id"`
	Source    string    `json:"source"`
	Payload   any       `json:"payload"`
}

// proximityReportPayload is the payload for the proximity.device.report event.
type proximityReportPayload struct {
	SourceDeviceID     string  `json:"source_device_id"`
	DetectedDeviceID   string  `json:"detected_device_id"`
	DetectedDeviceType string  `json:"detected_device_type,omitempty"`
	Action             string  `json:"action"`
	RSSI               int     `json:"rssi,omitempty"`
	Distance           float64 `json:"distance,omitempty"`
	Timestamp          int64   `json:"timestamp"`
}

// eventMessage is the parsed structure of an event-type client message.
type eventMessage struct {
	Event   string          `json:"event"`
	PayLoad json.RawMessage `json:"payload"`
}

// HandleEventMessage routes device event messages to the appropriate handler.
func (c *ChatManager) HandleEventMessage(msg *ClientMessage) error {
	var eventMsg eventMessage
	if err := json.Unmarshal(msg.PayLoad, &eventMsg); err != nil {
		log.Warnf("设备 %s 事件消息解析失败: %v", c.DeviceID, err)
		return nil // don't disconnect on bad event
	}

	switch eventMsg.Event {
	case EventDeviceProximity:
		return c.handleDeviceProximity(eventMsg.PayLoad)
	default:
		log.Debugf("设备 %s 未知事件类型: %s, 忽略", c.DeviceID, eventMsg.Event)
		return nil
	}
}

// handleDeviceProximity parses the proximity payload and publishes to NATS.
func (c *ChatManager) handleDeviceProximity(payload json.RawMessage) error {
	var p struct {
		Action             string  `json:"action"`
		DetectedDeviceID   string  `json:"detected_device_id"`
		DetectedDeviceType string  `json:"detected_device_type"`
		RSSI               int     `json:"rssi"`
		Distance           float64 `json:"distance"`
		Timestamp          int64   `json:"timestamp"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		log.Warnf("设备 %s 近场事件 payload 解析失败: %v", c.DeviceID, err)
		return nil
	}

	if p.Action == "" || p.DetectedDeviceID == "" {
		log.Warnf("设备 %s 近场事件缺少必填字段 action=%s detected=%s",
			c.DeviceID, p.Action, p.DetectedDeviceID)
		return nil
	}

	if c.natsPublisher == nil || !c.natsPublisher.IsConnected() {
		log.Warnf("设备 %s NATS 未连接，跳过近场事件发布", c.DeviceID)
		return nil
	}

	log.Infof("发布近场NATS事件 source_device=%s detected_device=%s action=%s rssi=%d distance=%.1f",
		c.DeviceID, p.DetectedDeviceID, p.Action, p.RSSI, p.Distance)

	envelope := sharedEnvelope{
		ID:        uuid.New().String(),
		Type:      "proximity.device.report",
		Version:   "v1",
		Timestamp: time.Now().UTC(),
		Source:    "xiaozhi-server",
		Payload: &proximityReportPayload{
			SourceDeviceID:     c.DeviceID,
			DetectedDeviceID:   p.DetectedDeviceID,
			DetectedDeviceType: p.DetectedDeviceType,
			Action:             p.Action,
			RSSI:               p.RSSI,
			Distance:           p.Distance,
			Timestamp:          p.Timestamp,
		},
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal proximity envelope: %w", err)
	}

	if err := c.natsPublisher.PublishJetStream(SubjectXiaozhiProximityReport, data); err != nil {
		log.Errorf("设备 %s 近场NATS事件发布失败: %v", c.DeviceID, err)
		return nil // don't disconnect on publish failure
	}

	log.Infof("近场NATS事件已发布 source_device=%s detected_device=%s action=%s",
		c.DeviceID, p.DetectedDeviceID, p.Action)
	return nil
}
