package nats

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvelope_Marshal(t *testing.T) {
	env := Envelope{
		Version:   "1.0",
		EventID:   "123456",
		Timestamp: time.Date(2026, 4, 30, 14, 32, 1, 0, time.UTC),
		Source:    "xiaozhi",
		Subject:   "test.subject",
		Payload:   map[string]string{"key": "value"},
	}

	data, err := env.Marshal()
	require.NoError(t, err)

	var parsed Envelope
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "1.0", parsed.Version)
	assert.Equal(t, "123456", parsed.EventID)
	assert.Equal(t, "xiaozhi", parsed.Source)
	assert.Equal(t, "test.subject", parsed.Subject)
}

func TestParseEnvelope(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name: "valid envelope",
			data: []byte(`{
				"version": "1.0",
				"event_id": "123",
				"timestamp": "2026-04-30T14:32:01Z",
				"source": "xiaozhi",
				"subject": "test.subject",
				"payload": {"key": "value"}
			}`),
			wantErr: false,
		},
		{
			name:    "invalid json",
			data:    []byte(`{invalid`),
			wantErr: true,
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := ParseEnvelope(tt.data)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, env)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, env)
				assert.Equal(t, "1.0", env.Version)
				assert.Equal(t, "xiaozhi", env.Source)
			}
		})
	}
}

func TestParsePayload(t *testing.T) {
	type TestPayload struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name    string
		payload interface{}
		target  *TestPayload
		want    *TestPayload
		wantErr bool
	}{
		{
			name:    "valid payload",
			payload: map[string]interface{}{"name": "test", "value": 42},
			target:  &TestPayload{},
			want:    &TestPayload{Name: "test", Value: 42},
			wantErr: false,
		},
		{
			name:    "nil payload",
			payload: nil,
			target:  &TestPayload{},
			want:    &TestPayload{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ParsePayload(tt.payload, tt.target)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, tt.target)
			}
		})
	}
}

func TestClient_IsConnected_Nil(t *testing.T) {
	var c *Client
	assert.False(t, c.IsConnected())
	assert.Equal(t, "", c.ConnectedUrl())
}

func TestConfig_Defaults(t *testing.T) {
	cfg := Config{
		URL:           "nats://localhost:4222",
		MaxReconnects: -1,
		ReconnectWait: 2,
		PingInterval:  30,
	}

	assert.Equal(t, "nats://localhost:4222", cfg.URL)
	assert.Equal(t, -1, cfg.MaxReconnects)
	assert.Equal(t, 2, cfg.ReconnectWait)
	assert.Equal(t, 30, cfg.PingInterval)
}
