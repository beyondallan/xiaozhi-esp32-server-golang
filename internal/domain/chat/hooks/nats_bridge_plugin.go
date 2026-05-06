package hooks

import (
	"context"
	"sync/atomic"
	"time"

	natspkg "xiaozhi-esp32-server-golang/internal/pkg/nats"
	log "xiaozhi-esp32-server-golang/logger"
)

const (
	// NATS subjects for xiaozhi events
	SubjectASRResult    = "xiaozhi.chat.asr.result"
	SubjectLLMResponse  = "xiaozhi.chat.llm.response"
	SubjectTTSPlayback  = "xiaozhi.chat.tts.playback"
)

// NatsBridgePlugin bridges chat hook events to NATS message bus.
type NatsBridgePlugin struct {
	natsClient natspkg.NATSPublisher
	enabled    bool
	published  int64
}

// NatsBridgeRegistration creates a Registration for the NATS bridge plugin.
// This should be added to BuiltinRegistrations when NATS is configured.
func NatsBridgeRegistration(natsClient natspkg.NATSPublisher, enabled bool) Registration {
	plugin := NewNatsBridgePlugin(natsClient, enabled)
	return Registration{
		Meta: PluginMeta{
			Name:        "nats_bridge",
			Version:     "v1",
			Description: "Bridge chat hook events to NATS message bus",
			Priority:    50,
			Enabled:     enabled,
			Kind:        PluginKindObserver,
		},
		Lifecycle: plugin,
		Register: func(hub *Hub, meta PluginMeta) error {
			return plugin.Register(hub, meta)
		},
	}
}

// NewNatsBridgePlugin creates a new NatsBridgePlugin.
func NewNatsBridgePlugin(natsClient natspkg.NATSPublisher, enabled bool) *NatsBridgePlugin {
	return &NatsBridgePlugin{
		natsClient: natsClient,
		enabled:    enabled,
	}
}

func (p *NatsBridgePlugin) Name() string  { return "nats_bridge" }
func (p *NatsBridgePlugin) Priority() int { return 50 }

// Init implements Lifecycle interface.
func (p *NatsBridgePlugin) Init(ctx context.Context) error {
	return nil
}

// Close implements Lifecycle interface.
func (p *NatsBridgePlugin) Close() error {
	return nil
}

// Register registers all event handlers to the hub.
func (p *NatsBridgePlugin) Register(hub *Hub, meta PluginMeta) error {
	if !p.enabled || p.natsClient == nil {
		return nil
	}

	// Register ASR output observer
	hub.RegisterAsync(EventChatASROutput, p.Name(), p.Priority(), p.handleASROutput)

	// Register LLM output interceptor (only publish on IsEnd=true)
	hub.RegisterSync(EventChatLLMOutputRaw, p.Name(), p.Priority(), p.handleLLMOutputRaw)

	// Register TTS output stop observer
	hub.RegisterAsync(EventChatTTSOutputStop, p.Name(), p.Priority(), p.handleTTSOutputStop)

	return nil
}

// handleASROutput publishes ASR result to NATS (observer, async).
func (p *NatsBridgePlugin) handleASROutput(ctx Context, payload any) {
	if !p.enabled || p.natsClient == nil {
		return
	}

	data, ok := payload.(ASROutputData)
	if !ok {
		return
	}

	natsPayload := map[string]interface{}{
		"xiaozhi_device_id": ctx.DeviceID,
		"session_id":        ctx.SessionID,
		"speaker":           "user",
		"text":              data.Text,
		"is_final":          true,
	}

	go func() {
		if err := p.natsClient.Publish(SubjectASRResult, natsPayload); err != nil {
			log.Warnf("failed to publish ASR result to NATS: %v", err)
		} else {
			atomic.AddInt64(&p.published, 1)
		}
	}()
}

// handleLLMOutputRaw publishes LLM response to NATS (interceptor, sync).
// Only publishes when IsEnd=true to avoid streaming message explosion.
func (p *NatsBridgePlugin) handleLLMOutputRaw(ctx Context, payload any) (any, bool, error) {
	if !p.enabled || p.natsClient == nil {
		return payload, false, nil
	}

	data, ok := payload.(LLMOutputRawData)
	if !ok {
		return payload, false, nil
	}

	// Only publish on end of stream
	if data.IsEnd {
		natsPayload := map[string]interface{}{
			"xiaozhi_device_id": ctx.DeviceID,
			"session_id":        ctx.SessionID,
			"speaker":           "assistant",
			"text":              data.FullText,
			"is_streaming":      false,
		}

		go func() {
			if err := p.natsClient.Publish(SubjectLLMResponse, natsPayload); err != nil {
				log.Warnf("failed to publish LLM response to NATS: %v", err)
			} else {
				atomic.AddInt64(&p.published, 1)
			}
		}()
	}

	// Don't stop the chain
	return payload, false, nil
}

// handleTTSOutputStop publishes TTS playback completion to NATS (observer, async).
func (p *NatsBridgePlugin) handleTTSOutputStop(ctx Context, payload any) {
	if !p.enabled || p.natsClient == nil {
		return
	}

	natsPayload := map[string]interface{}{
		"xiaozhi_device_id": ctx.DeviceID,
		"session_id":        ctx.SessionID,
		"timestamp":         time.Now().UTC().Format(time.RFC3339),
	}

	go func() {
		if err := p.natsClient.Publish(SubjectTTSPlayback, natsPayload); err != nil {
			log.Warnf("failed to publish TTS playback to NATS: %v", err)
		} else {
			atomic.AddInt64(&p.published, 1)
		}
	}()
}

// PublishedCount returns the number of published messages.
func (p *NatsBridgePlugin) PublishedCount() int64 {
	return atomic.LoadInt64(&p.published)
}
