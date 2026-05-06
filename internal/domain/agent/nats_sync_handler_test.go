package agent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockNATSPublisher struct {
	published []MockPublishedMsg
	publishErr error
}

type MockPublishedMsg struct {
	Subject string
	Payload interface{}
}

func (m *MockNATSPublisher) Publish(subject string, payload interface{}) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.published = append(m.published, MockPublishedMsg{Subject: subject, Payload: payload})
	return nil
}

func newTestSyncHandler(repo AgentRepository) *NatsSyncHandler {
	return &NatsSyncHandler{
		natsClient: &MockNATSPublisher{},
		repo:       repo,
	}
}

func buildEnvelope(subject string, payload interface{}) []byte {
	env := map[string]interface{}{
		"version":  "1.0",
		"event_id": "test-123",
		"source":   "social",
		"subject":  subject,
		"payload":  payload,
	}
	data, _ := json.Marshal(env)
	return data
}

func TestMockAgentRepository_Create(t *testing.T) {
	repo := NewMockAgentRepository()

	agent := &Agent{
		AgentID: 100,
		Name:    "test-agent",
	}

	err := repo.Create(agent)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), agent.ID)

	saved, err := repo.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, "test-agent", saved.Name)
}

func TestMockAgentRepository_Update(t *testing.T) {
	repo := NewMockAgentRepository()

	agent := &Agent{
		AgentID: 100,
		Name:    "test-agent",
	}
	repo.Create(agent)

	agent.Name = "updated-agent"
	err := repo.Update(agent)
	require.NoError(t, err)

	saved, err := repo.GetByID(agent.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated-agent", saved.Name)
}

func TestMockAgentRepository_Delete(t *testing.T) {
	repo := NewMockAgentRepository()

	agent := &Agent{
		AgentID: 100,
		Name:    "test-agent",
	}
	repo.Create(agent)

	err := repo.Delete(agent.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(agent.ID)
	assert.Error(t, err)
}

func TestMockAgentRepository_GetByAgentID(t *testing.T) {
	repo := NewMockAgentRepository()

	agent := &Agent{
		AgentID: 100,
		Name:    "test-agent",
	}
	repo.Create(agent)

	found, err := repo.GetByAgentID(100)
	require.NoError(t, err)
	assert.Equal(t, "test-agent", found.Name)

	_, err = repo.GetByAgentID(999)
	assert.Error(t, err)
}

func TestAgentCreatedEvent_MarshalUnmarshal(t *testing.T) {
	event := AgentCreatedEvent{
		AgentID:  100,
		Name:     "test-agent",
		Nickname: "Test",
		OwnerID:  200,
		Config: AgentConfig{
			LLMConfigID: "llm-001",
			TTSConfigID: "tts-001",
			Voice:       "xiaozhi",
			MCPServices: []string{"mcp1", "mcp2"},
		},
	}

	data := marshalJSON(event)

	var parsed AgentCreatedEvent
	err := json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, event.AgentID, parsed.AgentID)
	assert.Equal(t, event.Name, parsed.Name)
	assert.Equal(t, event.Nickname, parsed.Nickname)
	assert.Equal(t, event.OwnerID, parsed.OwnerID)
	assert.Equal(t, event.Config.LLMConfigID, parsed.Config.LLMConfigID)
	assert.Equal(t, event.Config.Voice, parsed.Config.Voice)
}

func TestAgentSyncResult_MarshalUnmarshal(t *testing.T) {
	result := AgentSyncResult{
		AgentID:   100,
		XiaozhiID: 1,
		Action:    "create",
		Status:    "success",
		Error:     "",
	}

	data := marshalJSON(result)

	var parsed AgentSyncResult
	err := json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, result.AgentID, parsed.AgentID)
	assert.Equal(t, result.XiaozhiID, parsed.XiaozhiID)
	assert.Equal(t, result.Action, parsed.Action)
	assert.Equal(t, result.Status, parsed.Status)
}

func TestNatsSyncHandler_HandleAgentCreated_Success(t *testing.T) {
	repo := NewMockAgentRepository()
	handler := newTestSyncHandler(repo)

	event := AgentCreatedEvent{
		AgentID:  100,
		Name:     "test-agent",
		Nickname: "Test",
		OwnerID:  200,
		Config:   AgentConfig{Voice: "xiaozhi"},
	}

	data := buildEnvelope(SubjectAgentCreated, event)
	err := handler.handleAgentCreated(SubjectAgentCreated, data)
	require.NoError(t, err)

	agent, err := repo.GetByAgentID(100)
	require.NoError(t, err)
	assert.Equal(t, "test-agent", agent.Name)
	assert.Equal(t, "Test", agent.Nickname)
}

func TestNatsSyncHandler_HandleAgentCreated_InvalidEnvelope(t *testing.T) {
	handler := newTestSyncHandler(NewMockAgentRepository())

	err := handler.handleAgentCreated(SubjectAgentCreated, []byte("invalid"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse envelope")
}

func TestNatsSyncHandler_HandleAgentUpdated_Success(t *testing.T) {
	repo := NewMockAgentRepository()
	repo.Create(&Agent{AgentID: 100, Name: "old-name", Nickname: "old"})

	handler := newTestSyncHandler(repo)

	event := AgentUpdatedEvent{
		AgentID:  100,
		Name:     "new-name",
		Nickname: "new",
	}

	data := buildEnvelope(SubjectAgentUpdated, event)
	err := handler.handleAgentUpdated(SubjectAgentUpdated, data)
	require.NoError(t, err)

	agent, err := repo.GetByAgentID(100)
	require.NoError(t, err)
	assert.Equal(t, "new-name", agent.Name)
	assert.Equal(t, "new", agent.Nickname)
}

func TestNatsSyncHandler_HandleAgentUpdated_NotFound(t *testing.T) {
	handler := newTestSyncHandler(NewMockAgentRepository())

	event := AgentUpdatedEvent{AgentID: 999, Name: "test"}
	data := buildEnvelope(SubjectAgentUpdated, event)
	err := handler.handleAgentUpdated(SubjectAgentUpdated, data)
	require.NoError(t, err) // Returns nil to avoid retry
}

func TestNatsSyncHandler_HandleAgentDeleted_Success(t *testing.T) {
	repo := NewMockAgentRepository()
	agent := &Agent{AgentID: 100, Name: "to-delete"}
	repo.Create(agent)

	handler := newTestSyncHandler(repo)

	event := AgentDeletedEvent{AgentID: 100}
	data := buildEnvelope(SubjectAgentDeleted, event)
	err := handler.handleAgentDeleted(SubjectAgentDeleted, data)
	require.NoError(t, err)

	_, err = repo.GetByAgentID(100)
	assert.Error(t, err)
}

func TestNatsSyncHandler_HandleAgentDeleted_NotFound(t *testing.T) {
	handler := newTestSyncHandler(NewMockAgentRepository())

	event := AgentDeletedEvent{AgentID: 999}
	data := buildEnvelope(SubjectAgentDeleted, event)
	err := handler.handleAgentDeleted(SubjectAgentDeleted, data)
	require.NoError(t, err)
}
