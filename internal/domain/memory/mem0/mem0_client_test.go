package mem0

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestTiSocialAddMessageUsesAgentIngestEndpoint(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotPath = req.URL.Path
			if req.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", req.Method)
			}
			if err := json.NewDecoder(req.Body).Decode(&gotBody); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString(`{"results":[]}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	client := &Mem0Client{
		httpClient:     httpClient,
		baseURL:        "http://memory:8005",
		useTiSocialAPI: true,
	}

	err := client.AddMessage(context.Background(), "agent-1", schema.Message{
		Role:    schema.User,
		Content: "明天早上9点出门",
	})
	if err != nil {
		t.Fatalf("AddMessage returned error: %v", err)
	}
	if gotPath != "/v1/agents/agent-1/memory/ingest" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotBody["agent_id"] != "agent-1" {
		t.Fatalf("agent_id = %v", gotBody["agent_id"])
	}
	if gotBody["content"] != "user: 明天早上9点出门" {
		t.Fatalf("content = %v", gotBody["content"])
	}
}

func TestTiSocialSearchUsesAgentRecallEndpoint(t *testing.T) {
	createdAt := time.Now().UTC().Format(time.RFC3339)
	var gotPath string
	responseBody, err := json.Marshal(map[string]any{
		"results": []map[string]any{
			{
				"id":         "m1",
				"memory":     "用户明天早上9点出门",
				"score":      0.92,
				"created_at": createdAt,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			gotPath = req.URL.Path
			if req.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", req.Method)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(responseBody)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	client := &Mem0Client{
		httpClient:      httpClient,
		baseURL:         "http://memory:8005",
		useTiSocialAPI:  true,
		SearchThreshold: 0.5,
		SearchTopk:      3,
	}

	memories, err := client.actionSearch(context.Background(), "agent-1", "明天我要干什么", 3, 0.5)
	if err != nil {
		t.Fatalf("actionSearch returned error: %v", err)
	}
	if gotPath != "/v1/agents/agent-1/memory/recall" {
		t.Fatalf("path = %s", gotPath)
	}
	if len(memories) != 1 {
		t.Fatalf("len(memories) = %d", len(memories))
	}
	if memories[0].Memory != "用户明天早上9点出门" {
		t.Fatalf("memory = %s", memories[0].Memory)
	}
	if memories[0].Score != 0.92 {
		t.Fatalf("score = %v", memories[0].Score)
	}
}

func TestTiSocialSearchResolvesXiaozhiAgentIDToToyID(t *testing.T) {
	t.Setenv("SIDECAR_URL", "http://xiaozhi-sidecar:8006")

	const toyID = "48bd8eec-c247-4058-a338-c62860079b58"
	var gotMemoryPath string
	var sawSidecarResolve bool
	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Host {
			case "xiaozhi-sidecar:8006":
				sawSidecarResolve = true
				if req.URL.Path != "/api/internal/memory/agents/1/toy" {
					t.Fatalf("resolve path = %s", req.URL.Path)
				}
				if req.Header.Get("Authorization") != "Internal xiaozhi_admin_secret_key" {
					t.Fatalf("authorization = %s", req.Header.Get("Authorization"))
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(bytes.NewBufferString(
						`{"code":0,"message":"success","data":{"agent_id":"1","toy_id":"` + toyID + `"}}`,
					)),
					Header: make(http.Header),
				}, nil
			case "memory:8005":
				gotMemoryPath = req.URL.Path
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewBufferString(`{"results":[]}`)),
					Header:     make(http.Header),
				}, nil
			default:
				t.Fatalf("unexpected host: %s", req.URL.Host)
				return nil, nil
			}
		}),
	}

	client := &Mem0Client{
		httpClient:       httpClient,
		baseURL:          "http://memory:8005",
		useTiSocialAPI:   true,
		resolvedAgentIDs: make(map[string]string),
		SearchThreshold:  0.5,
		SearchTopk:       3,
	}

	_, err := client.actionSearch(context.Background(), "1", "明天我要干什么", 3, 0.5)
	if err != nil {
		t.Fatalf("actionSearch returned error: %v", err)
	}
	if !sawSidecarResolve {
		t.Fatal("expected sidecar resolve request")
	}
	if gotMemoryPath != "/v1/agents/"+toyID+"/memory/recall" {
		t.Fatalf("memory path = %s", gotMemoryPath)
	}
}
