package reminder

import (
	"context"
	"fmt"
	"time"

	natslib "github.com/nats-io/nats.go"
	natspkg "xiaozhi-esp32-server-golang/internal/pkg/nats"
	log "xiaozhi-esp32-server-golang/logger"
)

const (
	// NATS subject for voice reminders (from social-server)
	SubjectReminderVoice = "social.reminder.voice"
)

// VoiceReminderEvent represents a voice reminder event from social-server.
type VoiceReminderEvent struct {
	AccountID      uint64 `json:"account_id"`
	XiaozhiDeviceID uint64 `json:"xiaozhi_device_id"`
	ReminderID     uint64 `json:"reminder_id"`
	Text           string `json:"text"`
	Repeat         bool   `json:"repeat"`
}

// DeviceManager defines the interface for device management.
type DeviceManager interface {
	GetDevice(id uint64) (*Device, error)
}

// Device represents a connected device.
type Device struct {
	ID       uint64
	ClientID string
}

// TTSClient defines the interface for TTS synthesis.
type TTSClient interface {
	Synthesize(ctx context.Context, text string, voice string) ([]byte, error)
}

// AudioPlayer defines the interface for playing audio on devices.
type AudioPlayer interface {
	PlayAudio(deviceID uint64, audioData []byte) error
}

// NatsHandler handles voice reminder events from social-server.
type NatsHandler struct {
	natsClient    *natspkg.Client
	deviceManager DeviceManager
	ttsClient     TTSClient
	audioPlayer   AudioPlayer
}

// NewNatsHandler creates a new NatsHandler.
func NewNatsHandler(natsClient *natspkg.Client, deviceManager DeviceManager, ttsClient TTSClient, audioPlayer AudioPlayer) *NatsHandler {
	return &NatsHandler{
		natsClient:    natsClient,
		deviceManager: deviceManager,
		ttsClient:     ttsClient,
		audioPlayer:   audioPlayer,
	}
}

// Subscribe subscribes to voice reminder events.
func (h *NatsHandler) Subscribe() error {
	_, err := h.natsClient.QueueSubscribe(SubjectReminderVoice, "reminder-player", h.handleVoiceReminder)
	if err != nil {
		return fmt.Errorf("subscribe to %s: %w", SubjectReminderVoice, err)
	}
	log.Infof("subscribed to %s", SubjectReminderVoice)
	return nil
}

// handleVoiceReminder handles incoming voice reminder events.
func (h *NatsHandler) handleVoiceReminder(msg *natslib.Msg) {
	// Parse envelope
	env, err := natspkg.ParseEnvelope(msg.Data)
	if err != nil {
		log.Warnf("failed to parse reminder envelope: %v", err)
		return
	}

	// Parse payload
	var event VoiceReminderEvent
	if err := natspkg.ParsePayload(env.Payload, &event); err != nil {
		log.Warnf("failed to parse reminder payload: %v", err)
		return
	}

	log.Infof("received voice reminder: device=%d, text=%s, repeat=%v",
		event.XiaozhiDeviceID, event.Text, event.Repeat)

	// Check if device is online
	device, err := h.deviceManager.GetDevice(event.XiaozhiDeviceID)
	if err != nil {
		log.Warnf("device not found: %d, error: %v", event.XiaozhiDeviceID, err)
		return // Ack message to avoid retry
	}

	if device == nil {
		log.Warnf("device not connected: %d", event.XiaozhiDeviceID)
		return // Ack message to avoid retry
	}

	// Synthesize TTS
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	audioData, err := h.ttsClient.Synthesize(ctx, event.Text, "")
	if err != nil {
		log.Warnf("TTS synthesis failed: %v", err)
		return
	}

	// Play audio on device
	if err := h.audioPlayer.PlayAudio(event.XiaozhiDeviceID, audioData); err != nil {
		log.Warnf("failed to play audio on device %d: %v", event.XiaozhiDeviceID, err)
		return
	}

	log.Infof("voice reminder played on device %d", event.XiaozhiDeviceID)

	// Handle repeat if needed
	if event.Repeat {
		go h.repeatPlayback(event.XiaozhiDeviceID, audioData, 3)
	}
}

// repeatPlayback repeats audio playback.
func (h *NatsHandler) repeatPlayback(deviceID uint64, audioData []byte, times int) {
	for i := 1; i < times; i++ {
		time.Sleep(2 * time.Second)

		// Check if device is still online
		device, err := h.deviceManager.GetDevice(deviceID)
		if err != nil || device == nil {
			log.Warnf("device %d no longer connected, stopping repeat", deviceID)
			return
		}

		if err := h.audioPlayer.PlayAudio(deviceID, audioData); err != nil {
			log.Warnf("failed to repeat audio on device %d: %v", deviceID, err)
			return
		}

		log.Infof("repeated playback %d/%d on device %d", i+1, times, deviceID)
	}
}

// MockDeviceManager is a mock implementation for testing.
type MockDeviceManager struct {
	Devices map[uint64]*Device
}

func NewMockDeviceManager() *MockDeviceManager {
	return &MockDeviceManager{
		Devices: make(map[uint64]*Device),
	}
}

func (m *MockDeviceManager) GetDevice(id uint64) (*Device, error) {
	device, ok := m.Devices[id]
	if !ok {
		return nil, fmt.Errorf("device not found: %d", id)
	}
	return device, nil
}

// MockTTSClient is a mock implementation for testing.
type MockTTSClient struct {
	AudioData []byte
	Error     error
}

func (m *MockTTSClient) Synthesize(ctx context.Context, text string, voice string) ([]byte, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.AudioData, nil
}

// MockAudioPlayer is a mock implementation for testing.
type MockAudioPlayer struct {
	PlayedDevices []uint64
	Error         error
}

func NewMockAudioPlayer() *MockAudioPlayer {
	return &MockAudioPlayer{
		PlayedDevices: make([]uint64, 0),
	}
}

func (m *MockAudioPlayer) PlayAudio(deviceID uint64, audioData []byte) error {
	if m.Error != nil {
		return m.Error
	}
	m.PlayedDevices = append(m.PlayedDevices, deviceID)
	return nil
}
