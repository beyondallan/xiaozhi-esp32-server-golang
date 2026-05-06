package hooks

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockNATSPublisher struct {
	mu        sync.Mutex
	published []PublishedMessage
	publishErr error
}

type PublishedMessage struct {
	Subject string
	Payload interface{}
}

func (m *MockNATSPublisher) Publish(subject string, payload interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.publishErr != nil {
		return m.publishErr
	}
	m.published = append(m.published, PublishedMessage{Subject: subject, Payload: payload})
	return nil
}

func (m *MockNATSPublisher) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.published)
}

func (m *MockNATSPublisher) Has(subject string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, msg := range m.published {
		if msg.Subject == subject {
			return true
		}
	}
	return false
}

func TestNatsBridgePlugin_Init(t *testing.T) {
	mock := &MockNATSPublisher{}
	plugin := NewNatsBridgePlugin(mock, true)

	err := plugin.Init(context.Background())
	assert.NoError(t, err)

	err = plugin.Close()
	assert.NoError(t, err)
}

func TestNatsBridgePlugin_Disabled(t *testing.T) {
	mock := &MockNATSPublisher{}
	plugin := NewNatsBridgePlugin(mock, false)

	hub := newTestHub(t)
	meta := PluginMeta{Name: "nats_bridge", Enabled: false}

	err := plugin.Register(hub, meta)
	assert.NoError(t, err)

	// Should not register any handlers when disabled
	assert.Equal(t, 0, mock.Count())
}

func TestNatsBridgePlugin_NilPublisher(t *testing.T) {
	plugin := NewNatsBridgePlugin(nil, true)

	hub := newTestHub(t)
	meta := PluginMeta{Name: "nats_bridge", Enabled: true}

	err := plugin.Register(hub, meta)
	assert.NoError(t, err)
}

func TestNatsBridgePlugin_HandleASROutput(t *testing.T) {
	mock := &MockNATSPublisher{}
	plugin := NewNatsBridgePlugin(mock, true)

	ctx := Context{
		Ctx:       context.Background(),
		SessionID: "test-session",
		DeviceID:  "device-123",
	}

	payload := ASROutputData{
		Text: "hello world",
	}

	plugin.handleASROutput(ctx, payload)

	// Wait for goroutine
	assert.Eventually(t, func() bool {
		return mock.Count() == 1
	}, 100*time.Millisecond, 10*time.Millisecond)

	assert.True(t, mock.Has(SubjectASRResult))
}

func TestNatsBridgePlugin_HandleLLMOutputRaw_Streaming(t *testing.T) {
	mock := &MockNATSPublisher{}
	plugin := NewNatsBridgePlugin(mock, true)

	ctx := Context{
		Ctx:       context.Background(),
		SessionID: "test-session",
		DeviceID:  "device-123",
	}

	// Streaming delta (IsEnd=false) should not publish
	payload := LLMOutputRawData{
		Delta:    "partial",
		FullText: "partial",
		IsEnd:    false,
	}

	result, stop, err := plugin.handleLLMOutputRaw(ctx, payload)
	assert.NoError(t, err)
	assert.False(t, stop)
	assert.Equal(t, payload, result)
	assert.Equal(t, 0, mock.Count())
}

func TestNatsBridgePlugin_HandleLLMOutputRaw_EndOfStream(t *testing.T) {
	mock := &MockNATSPublisher{}
	plugin := NewNatsBridgePlugin(mock, true)

	ctx := Context{
		Ctx:       context.Background(),
		SessionID: "test-session",
		DeviceID:  "device-123",
	}

	// End of stream (IsEnd=true) should publish
	payload := LLMOutputRawData{
		Delta:    "",
		FullText: "complete response",
		IsEnd:    true,
	}

	result, stop, err := plugin.handleLLMOutputRaw(ctx, payload)
	assert.NoError(t, err)
	assert.False(t, stop)
	assert.Equal(t, payload, result)

	// Wait for goroutine
	assert.Eventually(t, func() bool {
		return mock.Count() == 1
	}, 100*time.Millisecond, 10*time.Millisecond)

	assert.True(t, mock.Has(SubjectLLMResponse))
}

func TestNatsBridgePlugin_HandleTTSOutputStop(t *testing.T) {
	mock := &MockNATSPublisher{}
	plugin := NewNatsBridgePlugin(mock, true)

	ctx := Context{
		Ctx:       context.Background(),
		SessionID: "test-session",
		DeviceID:  "device-123",
	}

	payload := TTSOutputStopData{}

	plugin.handleTTSOutputStop(ctx, payload)

	// Wait for goroutine
	assert.Eventually(t, func() bool {
		return mock.Count() == 1
	}, 100*time.Millisecond, 10*time.Millisecond)

	assert.True(t, mock.Has(SubjectTTSPlayback))
}

func TestNatsBridgePlugin_PublishedCount(t *testing.T) {
	mock := &MockNATSPublisher{}
	plugin := NewNatsBridgePlugin(mock, true)

	assert.Equal(t, int64(0), plugin.PublishedCount())

	// Simulate publishing
	ctx := Context{
		Ctx:       context.Background(),
		SessionID: "test-session",
		DeviceID:  "device-123",
	}

	plugin.handleASROutput(ctx, ASROutputData{Text: "test"})

	assert.Eventually(t, func() bool {
		return plugin.PublishedCount() == 1
	}, 100*time.Millisecond, 10*time.Millisecond)
}

func TestNatsBridgePlugin_PublishError(t *testing.T) {
	mock := &MockNATSPublisher{
		publishErr: assert.AnError,
	}
	plugin := NewNatsBridgePlugin(mock, true)

	ctx := Context{
		Ctx:       context.Background(),
		SessionID: "test-session",
		DeviceID:  "device-123",
	}

	// Should not panic on error
	plugin.handleASROutput(ctx, ASROutputData{Text: "test"})

	assert.Eventually(t, func() bool {
		return plugin.PublishedCount() == 0
	}, 100*time.Millisecond, 10*time.Millisecond)
}

func TestNatsBridgeRegistration(t *testing.T) {
	mock := &MockNATSPublisher{}
	reg := NatsBridgeRegistration(mock, true)

	require.NotNil(t, reg)
	assert.Equal(t, "nats_bridge", reg.Meta.Name)
	assert.Equal(t, 50, reg.Meta.Priority)
	assert.True(t, reg.Meta.Enabled)
	assert.NotNil(t, reg.Lifecycle)
	assert.NotNil(t, reg.Register)
}

func TestNatsBridgePlugin_NameAndPriority(t *testing.T) {
	plugin := NewNatsBridgePlugin(&MockNATSPublisher{}, true)
	assert.Equal(t, "nats_bridge", plugin.Name())
	assert.Equal(t, 50, plugin.Priority())
}

func TestNatsBridgePlugin_Register_Success(t *testing.T) {
	mock := &MockNATSPublisher{}
	plugin := NewNatsBridgePlugin(mock, true)

	hub := &Hub{}
	meta := PluginMeta{Name: "nats_bridge", Enabled: true}

	err := plugin.Register(hub, meta)
	assert.NoError(t, err)
}

func newTestHub(t *testing.T) *Hub {
	t.Helper()
	return &Hub{}
}
