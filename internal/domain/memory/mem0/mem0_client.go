package mem0

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/hackers365/mem0-go/client"
	"github.com/hackers365/mem0-go/types"

	"xiaozhi-esp32-server-golang/internal/util"
	log "xiaozhi-esp32-server-golang/logger"
)

// Mem0Client 实现 MemoryProvider 和 EnhancedMemoryProvider 接口
type Mem0Client struct {
	client           *client.MemoryClient
	config           Mem0Config
	httpClient       *http.Client
	baseURL          string
	useTiSocialAPI   bool
	resolvedAgentIDs map[string]string
	mu               sync.RWMutex
	EnableSearch     bool    `mapstructure:"enable_search"`
	SearchThreshold  float64 `mapstructure:"search_threshold"`
	SearchTopk       int     `mapstructure:"search_topk"`
}

// Mem0Config 配置结构
type Mem0Config struct {
	APIKey           string `mapstructure:"api_key"`
	BaseUrl          string `mapstructure:"base_url"`
	TimeoutMS        int    `mapstructure:"timeout_ms"`
	OrganizationName string `mapstructure:"organization_name"`
	ProjectName      string `mapstructure:"project_name"`
	OrganizationID   string `mapstructure:"organization_id"`
	ProjectID        string `mapstructure:"project_id"`
}

type tiSocialMemoryResponse struct {
	Results []tiSocialMemoryItem `json:"results"`
}

type tiSocialMemoryItem struct {
	ID        string         `json:"id"`
	Memory    string         `json:"memory"`
	Score     *float64       `json:"score,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt string         `json:"created_at,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
}

type sidecarResolveAgentResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AgentID string `json:"agent_id"`
		ToyID   string `json:"toy_id"`
	} `json:"data"`
}

var (
	mem0Instance *Mem0Client
	mem0Once     sync.Once
	configOnce   sync.Once
)

// GetMem0ClientWithConfig 使用配置获取 Mem0 客户端单例
func GetMem0ClientWithConfig(config map[string]interface{}) (*Mem0Client, error) {
	var err error
	configOnce.Do(func() {
		var enableSearch bool = true
		var searchThreshold float64 = 0.5
		var searchTopk int = 3
		// 解析配置到结构体
		var mem0Cfg Mem0Config

		if enableSearchInterface, exists := config["enable_search"]; exists {
			if iEnableSearch, ok := enableSearchInterface.(bool); ok {
				enableSearch = iEnableSearch
			}
		}

		if searchThresholdInterface, exists := config["search_threshold"]; exists {
			if iSearchThreshold, ok := searchThresholdInterface.(float64); ok {
				searchThreshold = iSearchThreshold
			}
		}

		if searchTopkInterface, exists := config["search_top_k"]; exists {
			if iSearchTopk, ok := searchTopkInterface.(int); ok {
				searchTopk = iSearchTopk
			}
		} else if searchTopkInterface, exists := config["search_topk"]; exists {
			if iSearchTopk, ok := searchTopkInterface.(int); ok {
				searchTopk = iSearchTopk
			}
		}

		// 读取 API Key
		if apiKeyInterface, exists := config["api_key"]; exists {
			if apiKey, ok := apiKeyInterface.(string); ok {
				mem0Cfg.APIKey = apiKey
			} else {
				err = fmt.Errorf("mem0.api_key 必须是字符串")
				return
			}
		}

		// 读取 Host
		if hostInterface, exists := config["base_url"]; exists {
			if host, ok := hostInterface.(string); ok {
				mem0Cfg.BaseUrl = host
			} else {
				err = fmt.Errorf("mem0.host 必须是字符串")
				return
			}
		}
		if timeoutInterface, exists := config["timeout_ms"]; exists {
			switch timeout := timeoutInterface.(type) {
			case int:
				mem0Cfg.TimeoutMS = timeout
			case int64:
				mem0Cfg.TimeoutMS = int(timeout)
			case float64:
				mem0Cfg.TimeoutMS = int(timeout)
			}
		}

		// 验证必要配置
		// apps/memory 服务不需要 API key，允许为空
		// if mem0Cfg.APIKey == "" {
		// 	err = fmt.Errorf("mem0.api_key 配置缺失或为空")
		// 	return
		// }

		// 设置默认值
		if mem0Cfg.BaseUrl == "" {
			mem0Cfg.BaseUrl = "https://api.mem0.ai"
		}
		mem0Cfg.BaseUrl = strings.TrimRight(mem0Cfg.BaseUrl, "/")
		if mem0Cfg.TimeoutMS <= 0 {
			mem0Cfg.TimeoutMS = 10000
		}

		// 创建 mem0 客户端
		clientOptions := client.ClientOptions{
			APIKey:           mem0Cfg.APIKey,
			Host:             mem0Cfg.BaseUrl, // 使用 BaseUrl 作为 Host
			OrganizationName: mem0Cfg.OrganizationName,
			ProjectName:      mem0Cfg.ProjectName,
			OrganizationID:   mem0Cfg.OrganizationID,
			ProjectID:        mem0Cfg.ProjectID,
		}

		mem0Client, clientErr := client.NewMemoryClient(clientOptions)
		if clientErr != nil {
			err = fmt.Errorf("failed to create mem0 client: %w", clientErr)
			return
		}

		mem0Instance = &Mem0Client{
			client:           mem0Client,
			config:           mem0Cfg,
			httpClient:       &http.Client{Timeout: time.Duration(mem0Cfg.TimeoutMS) * time.Millisecond},
			baseURL:          mem0Cfg.BaseUrl,
			useTiSocialAPI:   shouldUseTiSocialMemoryAPI(mem0Cfg),
			resolvedAgentIDs: make(map[string]string),
			EnableSearch:     enableSearch,
			SearchThreshold:  searchThreshold,
			SearchTopk:       searchTopk,
		}

		log.Log().Infof("Mem0 客户端初始化成功, base_url: %s, tisocial_api: %v", mem0Cfg.BaseUrl, mem0Instance.useTiSocialAPI)
	})

	return mem0Instance, err
}

// Init 初始化客户端
func (m *Mem0Client) Init() error {
	// 客户端已在创建时初始化
	log.Log().Info("Mem0 client initialized successfully")
	return nil
}

// Get 获取记忆（内部方法）
func (m *Mem0Client) Get(userID string) (interface{}, error) {
	if m.useTiSocialAPI {
		return m.tiSocialListMemories(context.Background(), userID, 100)
	}
	// 搜索用户的所有记忆
	results, err := m.client.Search("", &types.SearchOptions{
		MemoryOptions: types.MemoryOptions{
			UserID: userID,
		},
		Limit: 100, // 获取更多记忆
	})
	if err != nil {
		return nil, fmt.Errorf("failed to search memories for user %s: %w", userID, err)
	}

	return results, nil
}

// AddMessage 添加消息到记忆
func (m *Mem0Client) AddMessage(ctx context.Context, agentID string, msg schema.Message) error {
	if m.useTiSocialAPI {
		return m.tiSocialAddMessage(ctx, agentID, msg)
	}
	message := types.Message{
		Role:    string(msg.Role),
		Content: msg.Content,
	}
	// 添加记忆
	_, err := m.client.Add([]types.Message{message}, types.MemoryOptions{
		AgentID:   agentID,
		AsyncMode: true,
	})
	if err != nil {
		return fmt.Errorf("failed to add message to mem0 for user %s: %w", agentID, err)
	}

	log.Log().Debugf("Added message to mem0 for user %s: %s", agentID, message)
	return nil
}

// GetMessages 获取用户的消息历史
func (m *Mem0Client) GetMessages(ctx context.Context, agentID string, count int) ([]*schema.Message, error) {
	if m.useTiSocialAPI {
		memories, err := m.tiSocialListMemories(ctx, agentID, count)
		if err != nil {
			return nil, err
		}
		messages := make([]*schema.Message, 0, len(memories))
		for _, memory := range memories {
			if strings.TrimSpace(memory.Memory) == "" {
				continue
			}
			messages = append(messages, &schema.Message{
				Role:    schema.Assistant,
				Content: memory.Memory,
			})
		}
		return messages, nil
	}
	var memoryOptions = types.MemoryOptions{
		AgentID: agentID,
	}

	results, err := m.client.GetAll(&types.SearchOptions{
		MemoryOptions: memoryOptions,
		Limit:         count,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get messages for user %s: %w", agentID, err)
	}

	// 转换为 schema.Message 格式
	var messages []*schema.Message
	for _, result := range results {
		// 从 metadata 中提取 role 和 content
		role := schema.Assistant // 默认角色
		content := result.Memory

		if result.Metadata != nil {
			if r, ok := result.Metadata["role"].(string); ok {
				switch r {
				case "user":
					role = schema.User
				case "assistant":
					role = schema.Assistant
				case "system":
					role = schema.System
				}
			}
			if c, ok := result.Metadata["content"].(string); ok {
				content = c
			}
		}

		messages = append(messages, &schema.Message{
			Role:    role,
			Content: content,
		})
	}

	return messages, nil
}

// ResetMemory 重置用户记忆
func (m *Mem0Client) ResetMemory(ctx context.Context, userID string) error {
	if m.useTiSocialAPI {
		log.Log().Warnf("Ti-Social memory API does not support reset via Mem0 provider, user: %s", userID)
		return nil
	}

	// 删除用户的所有记忆
	err := m.client.DeleteUser(userID)
	if err != nil {
		return fmt.Errorf("failed to reset memory for user %s: %w", userID, err)
	}

	log.Log().Infof("Reset memory for user %s", userID)
	return nil
}

// GetContext 获取上下文（实现 EnhancedMemoryProvider 接口）
func (m *Mem0Client) GetContext(ctx context.Context, agentID string, maxToken int) (string, error) {
	return "", nil
}

func (m *Mem0Client) IsEnableSearch() bool {
	return m.EnableSearch
}

func (m *Mem0Client) Search(ctx context.Context, agentId string, query string, topK int, timeRangeDays int64) (string, error) {
	if !m.EnableSearch {
		return "", nil
	}
	topK = m.SearchTopk
	results, err := m.actionSearch(ctx, agentId, query, topK, m.SearchThreshold)
	if err != nil {
		return "", err
	}

	// 构建上下文字符串
	var msgList []string
	for _, result := range results {
		msgList = append(msgList, fmt.Sprintf("- %s [%s]", result.Memory, result.CreatedAt))
	}

	return strings.Join(msgList, "\n"), nil
}

func (m *Mem0Client) Flush(ctx context.Context, agentID string) error {
	return nil
}

func (m *Mem0Client) actionSearch(ctx context.Context, agentID string, query string, topK int, threshold float64) ([]types.Memory, error) {
	if m.useTiSocialAPI {
		return m.tiSocialSearch(ctx, agentID, query, topK, threshold)
	}
	// 搜索相关记忆
	results, err := m.client.Search(query, &types.SearchOptions{
		MemoryOptions: types.MemoryOptions{
			AgentID: agentID,
		},
		Limit:     topK,      // 获取topK条记忆
		Threshold: threshold, // 设置相似度阈值
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get context for user %s: %w", agentID, err)
	}

	log.Log().Debugf("成功从mem0获取上下文, agentID: %s, results长度: %d", agentID, len(results))
	return results, nil
}

// AddBatchMessages 批量添加消息
func (m *Mem0Client) AddBatchMessages(ctx context.Context, agentID string, messages []schema.Message) error {
	if m.useTiSocialAPI {
		for _, msg := range messages {
			if err := m.tiSocialAddMessage(ctx, agentID, msg); err != nil {
				return err
			}
		}
		return nil
	}

	// 准备批量消息
	var batchMessages []string
	for _, msg := range messages {
		message := fmt.Sprintf("%s: %s", msg.Role, msg.Content)
		batchMessages = append(batchMessages, message)
	}

	// 逐个添加记忆（mem0-go 可能不支持批量添加）
	for _, message := range batchMessages {
		_, err := m.client.Add(message, types.MemoryOptions{
			AgentID: agentID,
			Metadata: map[string]interface{}{
				"source": "xiaozhi-esp32",
				"batch":  true,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to add batch message to mem0 for user %s: %w", agentID, err)
		}
	}

	log.Log().Debugf("Added %d batch messages to mem0 for user %s", len(messages), agentID)
	return nil
}

// Close 关闭客户端
func (m *Mem0Client) Close() error {
	// mem0-go 客户端不需要显式关闭
	log.Log().Info("Mem0 client closed")
	return nil
}

func shouldUseTiSocialMemoryAPI(cfg Mem0Config) bool {
	if strings.TrimSpace(cfg.APIKey) != "" {
		return false
	}
	u, err := url.Parse(cfg.BaseUrl)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "memory" || strings.HasPrefix(host, "ti-memory") || strings.Contains(host, "memory")
}

func isUUIDLike(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 36 {
		return false
	}
	return value[8] == '-' && value[13] == '-' && value[18] == '-' && value[23] == '-'
}

func (m *Mem0Client) tiSocialMemoryAgentID(ctx context.Context, agentID string) (string, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return "", fmt.Errorf("agentID is empty")
	}
	if isUUIDLike(agentID) {
		return agentID, nil
	}

	m.mu.RLock()
	if m.resolvedAgentIDs != nil {
		if resolved := strings.TrimSpace(m.resolvedAgentIDs[agentID]); resolved != "" {
			m.mu.RUnlock()
			return resolved, nil
		}
	}
	m.mu.RUnlock()

	sidecarURL := strings.TrimRight(strings.TrimSpace(util.GetSidecarURL()), "/")
	if sidecarURL == "" {
		log.Log().Warnf("Ti-Social memory agent mapping skipped because sidecar_url is empty, using original agentID: %s", agentID)
		return agentID, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		sidecarURL+"/api/internal/memory/agents/"+url.PathEscape(agentID)+"/toy", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Internal "+util.GetManagerAuthToken())

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("sidecar returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var result sidecarResolveAgentResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", fmt.Errorf("decode sidecar resolve response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("sidecar resolve failed: code=%d message=%s", result.Code, result.Message)
	}
	memoryAgentID := strings.TrimSpace(result.Data.ToyID)
	if memoryAgentID == "" {
		return "", fmt.Errorf("sidecar resolve response missing toy_id")
	}

	m.mu.Lock()
	if m.resolvedAgentIDs == nil {
		m.resolvedAgentIDs = make(map[string]string)
	}
	m.resolvedAgentIDs[agentID] = memoryAgentID
	m.mu.Unlock()

	log.Log().Debugf("resolved Xiaozhi agentID to Ti-Social toyID for memory: agent=%s toy=%s", agentID, memoryAgentID)
	return memoryAgentID, nil
}

func (m *Mem0Client) tiSocialAddMessage(ctx context.Context, agentID string, msg schema.Message) error {
	memoryAgentID, err := m.tiSocialMemoryAgentID(ctx, agentID)
	if err != nil {
		return fmt.Errorf("resolve Ti-Social memory agent %s: %w", agentID, err)
	}
	payload := map[string]any{
		"content":  fmt.Sprintf("%s: %s", msg.Role, msg.Content),
		"agent_id": memoryAgentID,
		"metadata": map[string]any{
			"source":           "xiaozhi-esp32",
			"role":             string(msg.Role),
			"xiaozhi_agent_id": agentID,
			"memory_agent_id":  memoryAgentID,
		},
	}
	path := fmt.Sprintf("/v1/agents/%s/memory/ingest", url.PathEscape(memoryAgentID))
	if _, err := m.tiSocialRequest(ctx, http.MethodPost, path, payload); err != nil {
		return fmt.Errorf("failed to add message to Ti-Social memory for agent %s: %w", memoryAgentID, err)
	}
	return nil
}

func (m *Mem0Client) tiSocialSearch(ctx context.Context, agentID string, query string, topK int, threshold float64) ([]types.Memory, error) {
	if topK <= 0 {
		topK = m.SearchTopk
	}
	memoryAgentID, err := m.tiSocialMemoryAgentID(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("resolve Ti-Social memory agent %s: %w", agentID, err)
	}
	payload := map[string]any{
		"query":     query,
		"agent_id":  memoryAgentID,
		"top_k":     topK,
		"threshold": threshold,
	}
	path := fmt.Sprintf("/v1/agents/%s/memory/recall", url.PathEscape(memoryAgentID))
	items, err := m.tiSocialRequest(ctx, http.MethodPost, path, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to get context for agent %s: %w", memoryAgentID, err)
	}
	return toMem0Memories(items), nil
}

func (m *Mem0Client) tiSocialListMemories(ctx context.Context, agentID string, limit int) ([]tiSocialMemoryItem, error) {
	if limit <= 0 {
		limit = 20
	}
	memoryAgentID, err := m.tiSocialMemoryAgentID(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("resolve Ti-Social memory agent %s: %w", agentID, err)
	}
	path := fmt.Sprintf("/v1/agents/%s/memory?limit=%d", url.PathEscape(memoryAgentID), limit)
	items, err := m.tiSocialRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list Ti-Social memories for agent %s: %w", memoryAgentID, err)
	}
	return items, nil
}

func (m *Mem0Client) tiSocialRequest(ctx context.Context, method, path string, payload map[string]any) ([]tiSocialMemoryItem, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, m.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBytes))
	}
	var result tiSocialMemoryResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("decode Ti-Social memory response: %w", err)
	}
	return result.Results, nil
}

func toMem0Memories(items []tiSocialMemoryItem) []types.Memory {
	memories := make([]types.Memory, 0, len(items))
	for _, item := range items {
		memory := types.Memory{
			ID:       item.ID,
			Memory:   item.Memory,
			Metadata: item.Metadata,
		}
		if item.Score != nil {
			memory.Score = *item.Score
		}
		if createdAt, err := time.Parse(time.RFC3339, item.CreatedAt); err == nil {
			memory.CreatedAt = createdAt
		}
		if updatedAt, err := time.Parse(time.RFC3339, item.UpdatedAt); err == nil {
			memory.UpdatedAt = updatedAt
		}
		memories = append(memories, memory)
	}
	return memories
}

// 确保 Mem0Client 实现了所需的接口
// 注意：这里不能直接引用 memory 包，因为会造成循环导入
// 接口实现会在编译时自动检查
