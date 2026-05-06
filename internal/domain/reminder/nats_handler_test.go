package reminder

import (
	"context"
	"encoding/json"
	"testing"

	natslib "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func marshalJSON(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}

func TestNatsHandler_HandleVoiceReminder_Success(t *testing.T) {
	mockDeviceMgr := NewMockDeviceManager()
	mockDeviceMgr.Devices[123] = &Device{ID: 123, ClientID: "client-123"}

	mockTTS := &MockTTSClient{AudioData: []byte("audio-data")}
	mockPlayer := NewMockAudioPlayer()

	handler := NewNatsHandler(nil, mockDeviceMgr, mockTTS, mockPlayer)

	event := VoiceReminderEvent{
		AccountID:       100,
		XiaozhiDeviceID: 123,
		ReminderID:      200,
		Text:            "提醒：该去开会了",
		Repeat:          false,
	}

	// Build envelope data
	env := map[string]interface{}{
		"version":  "1.0",
		"event_id": "123",
		"source":   "social",
		"subject":  SubjectReminderVoice,
		"payload":  event,
	}

	data := marshalJSON(env)

	handler.handleVoiceReminder(&natslib.Msg{Data: data})

	assert.Contains(t, mockPlayer.PlayedDevices, uint64(123))
}

func TestNatsHandler_HandleVoiceReminder_DeviceOffline(t *testing.T) {
	mockDeviceMgr := NewMockDeviceManager()
	// Device not in manager = offline

	mockTTS := &MockTTSClient{AudioData: []byte("audio-data")}
	mockPlayer := NewMockAudioPlayer()

	handler := NewNatsHandler(nil, mockDeviceMgr, mockTTS, mockPlayer)

	event := VoiceReminderEvent{
		AccountID:       100,
		XiaozhiDeviceID: 999,
		ReminderID:      200,
		Text:            "提醒：该去开会了",
	}

	env := map[string]interface{}{
		"version":  "1.0",
		"event_id": "123",
		"source":   "social",
		"subject":  SubjectReminderVoice,
		"payload":  event,
	}

	data := marshalJSON(env)

	handler.handleVoiceReminder(&natslib.Msg{Data: data})

	// Should not play audio when device is offline
	assert.Empty(t, mockPlayer.PlayedDevices)
}

func TestNatsHandler_HandleVoiceReminder_TTSError(t *testing.T) {
	mockDeviceMgr := NewMockDeviceManager()
	mockDeviceMgr.Devices[123] = &Device{ID: 123, ClientID: "client-123"}

	mockTTS := &MockTTSClient{Error: assert.AnError}
	mockPlayer := NewMockAudioPlayer()

	handler := NewNatsHandler(nil, mockDeviceMgr, mockTTS, mockPlayer)

	event := VoiceReminderEvent{
		AccountID:       100,
		XiaozhiDeviceID: 123,
		ReminderID:      200,
		Text:            "提醒：该去开会了",
	}

	env := map[string]interface{}{
		"version":  "1.0",
		"event_id": "123",
		"source":   "social",
		"subject":  SubjectReminderVoice,
		"payload":  event,
	}

	data := marshalJSON(env)

	handler.handleVoiceReminder(&natslib.Msg{Data: data})

	// Should not play audio when TTS fails
	assert.Empty(t, mockPlayer.PlayedDevices)
}

func TestMockDeviceManager_GetDevice(t *testing.T) {
	mgr := NewMockDeviceManager()
	mgr.Devices[123] = &Device{ID: 123, ClientID: "test"}

	device, err := mgr.GetDevice(123)
	require.NoError(t, err)
	assert.Equal(t, uint64(123), device.ID)

	_, err = mgr.GetDevice(999)
	assert.Error(t, err)
}

func TestMockTTSClient_Synthesize(t *testing.T) {
	mock := &MockTTSClient{AudioData: []byte("test-audio")}

	data, err := mock.Synthesize(context.Background(), "test", "")
	assert.NoError(t, err)
	assert.Equal(t, []byte("test-audio"), data)

	mock.Error = assert.AnError
	_, err = mock.Synthesize(context.Background(), "test", "")
	assert.Error(t, err)
}

func TestMockAudioPlayer_PlayAudio(t *testing.T) {
	mock := NewMockAudioPlayer()

	err := mock.PlayAudio(123, []byte("audio"))
	assert.NoError(t, err)
	assert.Contains(t, mock.PlayedDevices, uint64(123))

	mock.Error = assert.AnError
	err = mock.PlayAudio(456, []byte("audio"))
	assert.Error(t, err)
}

func TestNatsHandler_RepeatPlayback_DeviceOffline(t *testing.T) {
	mockDeviceMgr := NewMockDeviceManager()
	mockDeviceMgr.Devices[123] = &Device{ID: 123, ClientID: "client-123"}

	mockPlayer := NewMockAudioPlayer()

	handler := NewNatsHandler(nil, mockDeviceMgr, nil, mockPlayer)

	// Remove device after first check to simulate going offline
	delete(mockDeviceMgr.Devices, 123)

	// repeatPlayback should exit early when device is offline
	handler.repeatPlayback(123, []byte("audio"), 3)

	// Should not play any audio since device went offline
	assert.Empty(t, mockPlayer.PlayedDevices)
}
