package agent

import (
	"encoding/json"
	"fmt"

	natspkg "xiaozhi-esp32-server-golang/internal/pkg/nats"
	log "xiaozhi-esp32-server-golang/logger"
)

const (
	// NATS subjects for Agent lifecycle events (from social-server)
	SubjectAgentCreated = "social.agent.created"
	SubjectAgentUpdated = "social.agent.updated"
	SubjectAgentDeleted = "social.agent.deleted"

	// NATS subjects for sync results (to social-server)
	SubjectAgentSyncResult = "xiaozhi.agent.sync.result"
)

// Agent represents an AI agent in xiaozhi system.
type Agent struct {
	ID        uint64 `json:"id"`
	AgentID   uint64 `json:"agent_id"`   // social-server agent ID
	Name      string `json:"name"`
	Nickname  string `json:"nickname"`
	OwnerID   uint64 `json:"owner_id"`
	Config    AgentConfig `json:"config"`
}

// AgentConfig holds agent configuration.
type AgentConfig struct {
	LLMConfigID  string   `json:"llm_config_id"`
	TTSConfigID  string   `json:"tts_config_id"`
	Voice        string   `json:"voice"`
	MCPServices  []string `json:"mcp_services"`
}

// AgentCreatedEvent represents an agent created event from social-server.
type AgentCreatedEvent struct {
	AgentID  uint64       `json:"agent_id"`
	XiaozhiID *uint64     `json:"xiaozhi_id"`
	Name     string       `json:"name"`
	Nickname string       `json:"nickname"`
	OwnerID  uint64       `json:"owner_id"`
	Config   AgentConfig  `json:"config"`
}

// AgentUpdatedEvent represents an agent updated event from social-server.
type AgentUpdatedEvent struct {
	AgentID       uint64   `json:"agent_id"`
	XiaozhiID     uint64   `json:"xiaozhi_id"`
	UpdatedFields []string `json:"updated_fields"`
	Name          string   `json:"name,omitempty"`
	Nickname      string   `json:"nickname,omitempty"`
	CustomPrompt  string   `json:"custom_prompt,omitempty"`
}

// AgentDeletedEvent represents an agent deleted event from social-server.
type AgentDeletedEvent struct {
	AgentID   uint64 `json:"agent_id"`
	XiaozhiID uint64 `json:"xiaozhi_id"`
}

// AgentSyncResult represents the sync result sent back to social-server.
type AgentSyncResult struct {
	AgentID   uint64 `json:"agent_id"`
	XiaozhiID uint64 `json:"xiaozhi_id"`
	Action    string `json:"action"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// AgentRepository defines the interface for agent data access.
type AgentRepository interface {
	Create(agent *Agent) error
	Update(agent *Agent) error
	Delete(id uint64) error
	GetByID(id uint64) (*Agent, error)
	GetByAgentID(agentID uint64) (*Agent, error)
}

// NatsSyncHandler handles Agent lifecycle sync events from social-server.
type NatsSyncHandler struct {
	natsClient natspkg.NATSPublisher
	repo       AgentRepository
	dispatcher *natspkg.EventDispatcher
}

// NewNatsSyncHandler creates a new NatsSyncHandler.
func NewNatsSyncHandler(natsClient *natspkg.Client, repo AgentRepository) *NatsSyncHandler {
	return &NatsSyncHandler{
		natsClient: natsClient,
		repo:       repo,
		dispatcher: natspkg.NewEventDispatcher(natsClient),
	}
}

// Subscribe subscribes to Agent lifecycle events.
func (h *NatsSyncHandler) Subscribe() error {
	h.dispatcher.Subscribe(SubjectAgentCreated, h.handleAgentCreated)
	h.dispatcher.Subscribe(SubjectAgentUpdated, h.handleAgentUpdated)
	h.dispatcher.Subscribe(SubjectAgentDeleted, h.handleAgentDeleted)
	return h.dispatcher.Start()
}

// handleAgentCreated handles agent created events.
func (h *NatsSyncHandler) handleAgentCreated(subject string, data []byte) error {
	env, err := natspkg.ParseEnvelope(data)
	if err != nil {
		return fmt.Errorf("parse envelope: %w", err)
	}

	var event AgentCreatedEvent
	if err := natspkg.ParsePayload(env.Payload, &event); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// Create agent in xiaozhi database
	agent := &Agent{
		AgentID:  event.AgentID,
		Name:     event.Name,
		Nickname: event.Nickname,
		OwnerID:  event.OwnerID,
		Config:   event.Config,
	}

	if err := h.repo.Create(agent); err != nil {
		log.Errorf("failed to create agent: %v", err)
		h.publishSyncResult(event.AgentID, 0, "create", "failed", err.Error())
		return nil // Don't retry, error is recorded
	}

	log.Infof("agent created: agent_id=%d, xiaozhi_id=%d", event.AgentID, agent.ID)
	h.publishSyncResult(event.AgentID, agent.ID, "create", "success", "")
	return nil
}

// handleAgentUpdated handles agent updated events.
func (h *NatsSyncHandler) handleAgentUpdated(subject string, data []byte) error {
	env, err := natspkg.ParseEnvelope(data)
	if err != nil {
		return fmt.Errorf("parse envelope: %w", err)
	}

	var event AgentUpdatedEvent
	if err := natspkg.ParsePayload(env.Payload, &event); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// Find existing agent
	agent, err := h.repo.GetByAgentID(event.AgentID)
	if err != nil {
		log.Errorf("agent not found: %v", err)
		h.publishSyncResult(event.AgentID, 0, "update", "failed", "agent not found")
		return nil
	}

	// Update fields
	if event.Name != "" {
		agent.Name = event.Name
	}
	if event.Nickname != "" {
		agent.Nickname = event.Nickname
	}

	if err := h.repo.Update(agent); err != nil {
		log.Errorf("failed to update agent: %v", err)
		h.publishSyncResult(event.AgentID, agent.ID, "update", "failed", err.Error())
		return nil
	}

	log.Infof("agent updated: agent_id=%d", event.AgentID)
	h.publishSyncResult(event.AgentID, agent.ID, "update", "success", "")
	return nil
}

// handleAgentDeleted handles agent deleted events.
func (h *NatsSyncHandler) handleAgentDeleted(subject string, data []byte) error {
	env, err := natspkg.ParseEnvelope(data)
	if err != nil {
		return fmt.Errorf("parse envelope: %w", err)
	}

	var event AgentDeletedEvent
	if err := natspkg.ParsePayload(env.Payload, &event); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// Find existing agent
	agent, err := h.repo.GetByAgentID(event.AgentID)
	if err != nil {
		log.Errorf("agent not found: %v", err)
		h.publishSyncResult(event.AgentID, 0, "delete", "failed", "agent not found")
		return nil
	}

	if err := h.repo.Delete(agent.ID); err != nil {
		log.Errorf("failed to delete agent: %v", err)
		h.publishSyncResult(event.AgentID, agent.ID, "delete", "failed", err.Error())
		return nil
	}

	log.Infof("agent deleted: agent_id=%d", event.AgentID)
	h.publishSyncResult(event.AgentID, agent.ID, "delete", "success", "")
	return nil
}

// publishSyncResult publishes sync result to NATS.
func (h *NatsSyncHandler) publishSyncResult(agentID, xiaozhiID uint64, action, status, errMsg string) {
	result := AgentSyncResult{
		AgentID:   agentID,
		XiaozhiID: xiaozhiID,
		Action:    action,
		Status:    status,
		Error:     errMsg,
	}

	if err := h.natsClient.Publish(SubjectAgentSyncResult, result); err != nil {
		log.Warnf("failed to publish sync result: %v", err)
	}
}

// MockAgentRepository is a mock implementation for testing.
type MockAgentRepository struct {
	agents map[uint64]*Agent
	nextID uint64
}

// NewMockAgentRepository creates a new mock repository.
func NewMockAgentRepository() *MockAgentRepository {
	return &MockAgentRepository{
		agents: make(map[uint64]*Agent),
		nextID: 1,
	}
}

func (m *MockAgentRepository) Create(agent *Agent) error {
	agent.ID = m.nextID
	m.nextID++
	m.agents[agent.ID] = agent
	return nil
}

func (m *MockAgentRepository) Update(agent *Agent) error {
	m.agents[agent.ID] = agent
	return nil
}

func (m *MockAgentRepository) Delete(id uint64) error {
	delete(m.agents, id)
	return nil
}

func (m *MockAgentRepository) GetByID(id uint64) (*Agent, error) {
	agent, ok := m.agents[id]
	if !ok {
		return nil, fmt.Errorf("agent not found: %d", id)
	}
	return agent, nil
}

func (m *MockAgentRepository) GetByAgentID(agentID uint64) (*Agent, error) {
	for _, agent := range m.agents {
		if agent.AgentID == agentID {
			return agent, nil
		}
	}
	return nil, fmt.Errorf("agent not found: %d", agentID)
}

// marshalJSON helper for testing
func marshalJSON(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}
