package ai_chat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNatsHandler_HandleRequest_Success(t *testing.T) {
	mockLLM := &MockLLMClient{Response: "今天天气不错哦～"}
	mockTTS := &MockTTSClient{URL: "https://example.com/tts.wav"}
	mockRepo := NewMockAgentRepo()

	mockRepo.Agents[42] = &AgentInfo{
		ID:     42,
		Name:   "小智",
		Voice:  "xiaozhi",
		Prompt: "你是一个智能助手",
	}

	handler := NewNatsHandler(nil, mockLLM, mockTTS, mockRepo)

	// Build request
	req := AIChatRequest{
		XiaozhiAgentID: 42,
		ConversationID: "conv-123",
		SessionID:      "sess-456",
		TriggerReason:  "mention",
		TriggerText:    "今天天气怎么样？",
		Context: ChatContext{
			RecentMessages: []ChatMessage{
				{Role: "user", Sender: "Alice", Content: "今天天气怎么样？"},
			},
			Personality:   "活泼",
			SpeakingStyle: "口语",
			Interests:     []string{"天气", "科技"},
		},
	}

	env := map[string]interface{}{
		"version":    "1.0",
		"event_id":   "123",
		"source":     "social",
		"subject":    SubjectAIChatRequest,
		"payload":    req,
	}

	data, _ := json.Marshal(env)

	reply, err := handler.HandleRequest(data)
	require.NoError(t, err)

	var result AIChatReply
	err = json.Unmarshal(reply, &result)
	require.NoError(t, err)

	assert.Equal(t, uint64(42), result.XiaozhiAgentID)
	assert.Equal(t, "conv-123", result.ConversationID)
	assert.Equal(t, "今天天气不错哦～", result.AIText)
	assert.Equal(t, "https://example.com/tts.wav", result.AudioURL)
}

func TestNatsHandler_HandleRequest_AgentNotFound(t *testing.T) {
	mockLLM := &MockLLMClient{Response: "test"}
	mockRepo := NewMockAgentRepo()

	handler := NewNatsHandler(nil, mockLLM, nil, mockRepo)

	req := AIChatRequest{
		XiaozhiAgentID: 999,
		ConversationID: "conv-123",
	}

	env := map[string]interface{}{
		"version":  "1.0",
		"event_id": "123",
		"source":   "social",
		"subject":  SubjectAIChatRequest,
		"payload":  req,
	}

	data, _ := json.Marshal(env)

	reply, err := handler.HandleRequest(data)
	require.NoError(t, err)

	var result AIChatErrorReply
	err = json.Unmarshal(reply, &result)
	require.NoError(t, err)

	assert.Equal(t, 404, result.Code)
	assert.Equal(t, "agent not found", result.Message)
}

func TestNatsHandler_HandleRequest_MissingAgentID(t *testing.T) {
	handler := NewNatsHandler(nil, nil, nil, nil)

	req := AIChatRequest{
		XiaozhiAgentID: 0,
		ConversationID: "conv-123",
	}

	env := map[string]interface{}{
		"version":  "1.0",
		"event_id": "123",
		"source":   "social",
		"subject":  SubjectAIChatRequest,
		"payload":  req,
	}

	data, _ := json.Marshal(env)

	reply, err := handler.HandleRequest(data)
	require.NoError(t, err)

	var result AIChatErrorReply
	err = json.Unmarshal(reply, &result)
	require.NoError(t, err)

	assert.Equal(t, 400, result.Code)
	assert.Equal(t, "missing xiaozhi_agent_id", result.Message)
}

func TestNatsHandler_HandleRequest_LLMError(t *testing.T) {
	mockLLM := &MockLLMClient{Error: assert.AnError}
	mockRepo := NewMockAgentRepo()
	mockRepo.Agents[42] = &AgentInfo{ID: 42, Name: "test"}

	handler := NewNatsHandler(nil, mockLLM, nil, mockRepo)

	req := AIChatRequest{
		XiaozhiAgentID: 42,
		ConversationID: "conv-123",
	}

	env := map[string]interface{}{
		"version":  "1.0",
		"event_id": "123",
		"source":   "social",
		"subject":  SubjectAIChatRequest,
		"payload":  req,
	}

	data, _ := json.Marshal(env)

	reply, err := handler.HandleRequest(data)
	require.NoError(t, err)

	var result AIChatErrorReply
	err = json.Unmarshal(reply, &result)
	require.NoError(t, err)

	assert.Equal(t, 500, result.Code)
}

func TestNatsHandler_BuildSystemPrompt(t *testing.T) {
	handler := NewNatsHandler(nil, nil, nil, nil)

	tests := []struct {
		name     string
		agent    *AgentInfo
		ctx      ChatContext
		contains []string
	}{
		{
			name:     "basic prompt",
			agent:    &AgentInfo{Prompt: "你是一个智能助手"},
			ctx:      ChatContext{},
			contains: []string{"你是一个智能助手"},
		},
		{
			name:  "with personality",
			agent: &AgentInfo{Prompt: "助手"},
			ctx: ChatContext{
				Personality:   "活泼",
				SpeakingStyle: "口语",
			},
			contains: []string{"助手", "活泼", "口语"},
		},
		{
			name:  "with interests",
			agent: &AgentInfo{Prompt: "助手"},
			ctx: ChatContext{
				Interests: []string{"天气", "科技"},
			},
			contains: []string{"助手", "天气", "科技"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := handler.buildSystemPrompt(tt.agent, tt.ctx)
			for _, s := range tt.contains {
				assert.Contains(t, prompt, s)
			}
		})
	}
}

func TestMockLLMClient_Generate(t *testing.T) {
	mock := &MockLLMClient{Response: "test response"}

	resp, err := mock.Generate(context.Background(), "system", nil)
	assert.NoError(t, err)
	assert.Equal(t, "test response", resp)

	mock.Error = assert.AnError
	_, err = mock.Generate(context.Background(), "system", nil)
	assert.Error(t, err)
}

func TestMockTTSClient_Synthesize(t *testing.T) {
	mock := &MockTTSClient{URL: "https://example.com/audio.wav"}

	url, err := mock.Synthesize(context.Background(), "test", "xiaozhi")
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com/audio.wav", url)

	mock.Error = assert.AnError
	_, err = mock.Synthesize(context.Background(), "test", "xiaozhi")
	assert.Error(t, err)
}

func TestMockAgentRepo_GetByXiaozhiID(t *testing.T) {
	repo := NewMockAgentRepo()
	repo.Agents[42] = &AgentInfo{ID: 42, Name: "test"}

	agent, err := repo.GetByXiaozhiID(42)
	assert.NoError(t, err)
	assert.Equal(t, "test", agent.Name)

	_, err = repo.GetByXiaozhiID(999)
	assert.Error(t, err)
}
