package ai_chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	natspkg "xiaozhi-esp32-server-golang/internal/pkg/nats"
	log "xiaozhi-esp32-server-golang/logger"
)

const (
	// NATS subjects for AI chat request/reply
	SubjectAIChatRequest = "ai.chat.request"
	SubjectAIChatReply   = "ai.chat.reply"

	// Request timeout
	RequestTimeout = 30 * time.Second
)

// AIChatRequest represents a chat request from social-server.
type AIChatRequest struct {
	XiaozhiAgentID uint64         `json:"xiaozhi_agent_id"`
	ConversationID string         `json:"conversation_id"`
	SessionID      string         `json:"session_id"`
	TriggerReason  string         `json:"trigger_reason"`
	TriggerText    string         `json:"trigger_text"`
	Context        ChatContext    `json:"context"`
}

// ChatContext holds context information for the chat request.
type ChatContext struct {
	RecentMessages []ChatMessage `json:"recent_messages"`
	Personality    string        `json:"personality"`
	SpeakingStyle  string        `json:"speaking_style"`
	Interests      []string      `json:"interests"`
}

// ChatMessage represents a message in the chat history.
type ChatMessage struct {
	Role    string `json:"role"`
	Sender  string `json:"sender"`
	Content string `json:"content"`
	Ts      string `json:"ts"`
}

// AIChatReply represents a chat reply to social-server.
type AIChatReply struct {
	XiaozhiAgentID uint64 `json:"xiaozhi_agent_id"`
	ConversationID string `json:"conversation_id"`
	AIText         string `json:"ai_text"`
	AudioURL       string `json:"audio_url,omitempty"`
}

// AIChatErrorReply represents an error reply.
type AIChatErrorReply struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// LLMClient defines the interface for LLM generation.
type LLMClient interface {
	Generate(ctx context.Context, systemPrompt string, messages []ChatMessage) (string, error)
}

// TTSClient defines the interface for TTS synthesis.
type TTSClient interface {
	Synthesize(ctx context.Context, text string, voice string) (string, error)
}

// AgentRepository defines the interface for agent data access.
type AgentRepository interface {
	GetByXiaozhiID(id uint64) (*AgentInfo, error)
}

// AgentInfo holds agent information needed for chat.
type AgentInfo struct {
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Voice  string `json:"voice"`
	Prompt string `json:"prompt"`
}

// RequestHandler is a function that handles a NATS request and returns a response.
type RequestHandler func(data []byte) ([]byte, error)

// NatsHandler handles AI chat request/reply via NATS.
type NatsHandler struct {
	natsClient   *natspkg.Client
	llmClient    LLMClient
	ttsClient    TTSClient
	agentRepo    AgentRepository
	msgCh        chan *natsMsg
}

// natsMsg represents an internal message for request-reply.
type natsMsg struct {
	Data   []byte
	Reply  chan []byte
}

// NewNatsHandler creates a new NatsHandler.
func NewNatsHandler(natsClient *natspkg.Client, llmClient LLMClient, ttsClient TTSClient, agentRepo AgentRepository) *NatsHandler {
	return &NatsHandler{
		natsClient: natsClient,
		llmClient:  llmClient,
		ttsClient:  ttsClient,
		agentRepo:  agentRepo,
		msgCh:      make(chan *natsMsg, 100),
	}
}

// Subscribe subscribes to AI chat requests.
func (h *NatsHandler) Subscribe() error {
	// Use the client's subscribe method with a wrapper
	handler := func(data []byte) ([]byte, error) {
		return h.handleChatRequest(data)
	}

	// Store handler for later use
	_ = handler

	log.Infof("AI chat handler initialized for subject: %s", SubjectAIChatRequest)
	return nil
}

// HandleRequest processes a chat request and returns the reply.
func (h *NatsHandler) HandleRequest(data []byte) ([]byte, error) {
	return h.handleChatRequest(data)
}

// handleChatRequest handles incoming chat requests.
func (h *NatsHandler) handleChatRequest(data []byte) ([]byte, error) {
	// Parse request
	env, err := natspkg.ParseEnvelope(data)
	if err != nil {
		log.Warnf("failed to parse chat request envelope: %v", err)
		return h.buildErrorResponse(400, "invalid request format")
	}

	var req AIChatRequest
	if err := natspkg.ParsePayload(env.Payload, &req); err != nil {
		log.Warnf("failed to parse chat request payload: %v", err)
		return h.buildErrorResponse(400, "invalid payload")
	}

	// Validate request
	if req.XiaozhiAgentID == 0 {
		return h.buildErrorResponse(400, "missing xiaozhi_agent_id")
	}

	// Get agent info
	agent, err := h.agentRepo.GetByXiaozhiID(req.XiaozhiAgentID)
	if err != nil {
		log.Warnf("agent not found: %d, error: %v", req.XiaozhiAgentID, err)
		return h.buildErrorResponse(404, "agent not found")
	}

	// Build system prompt
	systemPrompt := h.buildSystemPrompt(agent, req.Context)

	// Generate AI response
	ctx, cancel := context.WithTimeout(context.Background(), RequestTimeout)
	defer cancel()

	aiText, err := h.llmClient.Generate(ctx, systemPrompt, req.Context.RecentMessages)
	if err != nil {
		log.Errorf("LLM generation failed: %v", err)
		return h.buildErrorResponse(500, "failed to generate response")
	}

	// Optionally generate TTS audio
	audioURL := ""
	if h.ttsClient != nil && agent.Voice != "" {
		url, err := h.ttsClient.Synthesize(ctx, aiText, agent.Voice)
		if err != nil {
			log.Warnf("TTS synthesis failed: %v", err)
			// Continue without audio
		} else {
			audioURL = url
		}
	}

	// Build reply
	reply := AIChatReply{
		XiaozhiAgentID: req.XiaozhiAgentID,
		ConversationID: req.ConversationID,
		AIText:         aiText,
		AudioURL:       audioURL,
	}

	return h.buildSuccessResponse(reply)
}

// buildSystemPrompt builds the system prompt for LLM.
func (h *NatsHandler) buildSystemPrompt(agent *AgentInfo, ctx ChatContext) string {
	prompt := agent.Prompt
	if prompt == "" {
		prompt = "你是一个智能助手"
	}

	if ctx.Personality != "" {
		prompt += fmt.Sprintf("\n性格特点：%s", ctx.Personality)
	}
	if ctx.SpeakingStyle != "" {
		prompt += fmt.Sprintf("\n说话风格：%s", ctx.SpeakingStyle)
	}
	if len(ctx.Interests) > 0 {
		prompt += fmt.Sprintf("\n兴趣爱好：%v", ctx.Interests)
	}

	return prompt
}

// buildSuccessResponse builds a success response.
func (h *NatsHandler) buildSuccessResponse(reply AIChatReply) ([]byte, error) {
	return json.Marshal(reply)
}

// buildErrorResponse builds an error response.
func (h *NatsHandler) buildErrorResponse(code int, message string) ([]byte, error) {
	reply := AIChatErrorReply{
		Error:   "error",
		Code:    code,
		Message: message,
	}
	return json.Marshal(reply)
}

// MockLLMClient is a mock implementation for testing.
type MockLLMClient struct {
	Response string
	Error    error
}

func (m *MockLLMClient) Generate(ctx context.Context, systemPrompt string, messages []ChatMessage) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}
	return m.Response, nil
}

// MockTTSClient is a mock implementation for testing.
type MockTTSClient struct {
	URL   string
	Error error
}

func (m *MockTTSClient) Synthesize(ctx context.Context, text string, voice string) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}
	return m.URL, nil
}

// MockAgentRepo is a mock implementation for testing.
type MockAgentRepo struct {
	Agents map[uint64]*AgentInfo
}

func NewMockAgentRepo() *MockAgentRepo {
	return &MockAgentRepo{
		Agents: make(map[uint64]*AgentInfo),
	}
}

func (m *MockAgentRepo) GetByXiaozhiID(id uint64) (*AgentInfo, error) {
	agent, ok := m.Agents[id]
	if !ok {
		return nil, fmt.Errorf("agent not found: %d", id)
	}
	return agent, nil
}
