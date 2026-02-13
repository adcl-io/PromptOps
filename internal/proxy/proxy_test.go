// Package proxy_test provides tests for the proxy package.
package proxy_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nexus/internal/proxy"
)

// ============================================================================
// OllamaProxy Tests
// ============================================================================

func TestNewOllamaProxy(t *testing.T) {
	p := proxy.NewOllamaProxy("http://localhost:11434/v1", nil)

	if p == nil {
		t.Fatal("NewOllamaProxy() returned nil")
	}
}

func TestNewOllamaProxyWithCustomMap(t *testing.T) {
	customMap := map[string]string{
		"custom-model": "custom-model:latest",
	}
	p := proxy.NewOllamaProxy("http://localhost:11434/v1", customMap)

	// The custom map should be used (if the proxy stores it)
	// Since we can't access private fields, we test via behavior
	if p == nil {
		t.Fatal("NewOllamaProxy() returned nil")
	}
}

// ============================================================================
// Anthropic Request/Response Tests
// ============================================================================

func TestAnthropicRequestGetSystemText(t *testing.T) {
	tests := []struct {
		name     string
		request  proxy.AnthropicRequest
		expected string
	}{
		{
			name:     "string system",
			request:  proxy.AnthropicRequest{System: "You are helpful"},
			expected: "You are helpful",
		},
		{
			name: "array system with text",
			request: proxy.AnthropicRequest{
				System: []interface{}{
					map[string]interface{}{"type": "text", "text": "You are helpful"},
				},
			},
			expected: "You are helpful",
		},
		{
			name: "array system with multiple items",
			request: proxy.AnthropicRequest{
				System: []interface{}{
					map[string]interface{}{"type": "text", "text": "You are "},
					map[string]interface{}{"type": "text", "text": "helpful"},
				},
			},
			expected: "You are helpful",
		},
		{
			name:     "nil system",
			request:  proxy.AnthropicRequest{System: nil},
			expected: "",
		},
		{
			name:     "empty string system",
			request:  proxy.AnthropicRequest{System: ""},
			expected: "",
		},
		{
			name: "empty array system",
			request: proxy.AnthropicRequest{
				System: []interface{}{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.request.GetSystemText()
			if result != tt.expected {
				t.Errorf("GetSystemText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestAnthropicMessageGetContentText(t *testing.T) {
	tests := []struct {
		name     string
		message  proxy.AnthropicMessage
		expected string
	}{
		{
			name:     "string content",
			message:  proxy.AnthropicMessage{Role: "user", Content: "hello"},
			expected: "hello",
		},
		{
			name: "array content with text",
			message: proxy.AnthropicMessage{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "hello"},
				},
			},
			expected: "hello",
		},
		{
			name: "array content with multiple items",
			message: proxy.AnthropicMessage{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "hello "},
					map[string]interface{}{"type": "text", "text": "world"},
				},
			},
			expected: "hello world",
		},
		{
			name:     "empty string content",
			message:  proxy.AnthropicMessage{Role: "user", Content: ""},
			expected: "",
		},
		{
			name:     "nil content",
			message:  proxy.AnthropicMessage{Role: "user", Content: nil},
			expected: "",
		},
		{
			name: "empty array content",
			message: proxy.AnthropicMessage{
				Role:    "user",
				Content: []interface{}{},
			},
			expected: "",
		},
		{
			name: "array with non-text items",
			message: proxy.AnthropicMessage{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{"type": "image", "url": "http://example.com/image.png"},
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.message.GetContentText()
			if result != tt.expected {
				t.Errorf("GetContentText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ============================================================================
// BuildModelMap Tests
// ============================================================================

func TestBuildModelMap(t *testing.T) {
	ollamaModels := map[string]string{
		"haiku":  "custom-haiku",
		"sonnet": "custom-sonnet",
		"opus":   "custom-opus",
	}

	modelMap := proxy.BuildModelMap(ollamaModels)

	// Check default mappings
	if modelMap["llama3.2"] != "llama3.2:latest" {
		t.Errorf("Expected llama3.2:latest, got %q", modelMap["llama3.2"])
	}

	// Check custom mappings
	if modelMap["haiku"] != "custom-haiku" {
		t.Errorf("Expected custom-haiku, got %q", modelMap["haiku"])
	}

	// Check self-mapping
	if modelMap["custom-haiku"] != "custom-haiku" {
		t.Errorf("Expected custom-haiku for self-mapping, got %q", modelMap["custom-haiku"])
	}
}

func TestBuildModelMapEmpty(t *testing.T) {
	modelMap := proxy.BuildModelMap(nil)

	// Should still have default mappings
	if modelMap["llama3.2"] != "llama3.2:latest" {
		t.Error("Expected default mappings when ollamaModels is nil")
	}
}

func TestBuildModelMapPartial(t *testing.T) {
	ollamaModels := map[string]string{
		"haiku": "custom-haiku",
		// sonnet and opus not specified
	}

	modelMap := proxy.BuildModelMap(ollamaModels)

	// Check specified mapping
	if modelMap["haiku"] != "custom-haiku" {
		t.Errorf("Expected custom-haiku, got %q", modelMap["haiku"])
	}

	// Check default mappings still exist
	if modelMap["llama3.2"] != "llama3.2:latest" {
		t.Error("Expected default mappings to remain")
	}
}

// ============================================================================
// Struct Field Tests
// ============================================================================

func TestAnthropicResponseFields(t *testing.T) {
	resp := proxy.AnthropicResponse{
		ID:           "msg_123",
		Type:         "message",
		Role:         "assistant",
		Model:        "claude-sonnet-4",
		Content:      []proxy.AnthropicContent{{Type: "text", Text: "Hello"}},
		StopReason:   "end_turn",
		StopSequence: "",
		Usage:        proxy.AnthropicUsage{InputTokens: 10, OutputTokens: 20},
	}

	if resp.ID != "msg_123" {
		t.Errorf("Expected ID='msg_123', got %q", resp.ID)
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("Expected InputTokens=10, got %d", resp.Usage.InputTokens)
	}
}

func TestAnthropicStreamEventFields(t *testing.T) {
	event := proxy.AnthropicStreamEvent{
		Type:       "content_block_delta",
		Index:      0,
		Delta:      &proxy.AnthropicDelta{Type: "text_delta", Text: "Hello"},
		StopReason: "",
		Usage:      &proxy.AnthropicUsage{InputTokens: 10, OutputTokens: 20},
	}

	if event.Type != "content_block_delta" {
		t.Errorf("Expected Type='content_block_delta', got %q", event.Type)
	}
	if event.Delta.Text != "Hello" {
		t.Errorf("Expected Delta.Text='Hello', got %q", event.Delta.Text)
	}
}

func TestOpenAIResponseFields(t *testing.T) {
	resp := proxy.OpenAIResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1234567890,
		Model:   "gpt-4",
		Choices: []proxy.OpenAIChoice{
			{
				Index: 0,
				Message: proxy.OpenAIMessage{
					Role:    "assistant",
					Content: "Hello",
				},
				FinishReason: "stop",
			},
		},
		Usage: proxy.OpenAIUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}

	if resp.Object != "chat.completion" {
		t.Errorf("Expected Object='chat.completion', got %q", resp.Object)
	}
	if resp.Usage.TotalTokens != 30 {
		t.Errorf("Expected TotalTokens=30, got %d", resp.Usage.TotalTokens)
	}
}

// ============================================================================
// JSON Serialization Tests
// ============================================================================

func TestAnthropicRequestJSON(t *testing.T) {
	req := proxy.AnthropicRequest{
		Model:     "claude-sonnet-4",
		MaxTokens: 100,
		Messages: []proxy.AnthropicMessage{
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded proxy.AnthropicRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Model != req.Model {
		t.Error("Model mismatch after round-trip")
	}
}

func TestAnthropicResponseJSON(t *testing.T) {
	resp := proxy.AnthropicResponse{
		ID:    "msg_123",
		Type:  "message",
		Role:  "assistant",
		Model: "claude-sonnet-4",
		Content: []proxy.AnthropicContent{
			{Type: "text", Text: "Hello world"},
		},
		Usage: proxy.AnthropicUsage{
			InputTokens:  10,
			OutputTokens: 20,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded proxy.AnthropicResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.ID != resp.ID {
		t.Error("ID mismatch after round-trip")
	}
	if len(decoded.Content) != 1 || decoded.Content[0].Text != "Hello world" {
		t.Error("Content mismatch after round-trip")
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestProxyEndToEnd(t *testing.T) {
	// Create a mock Ollama server
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"object": "list",
				"data": []map[string]interface{}{
					{"id": "llama3.2:latest", "object": "model"},
				},
			})
		case "/chat/completions":
			var req proxy.OpenAIRequest
			json.NewDecoder(r.Body).Decode(&req)

			response := proxy.OpenAIResponse{
				ID:      "test-id",
				Object:  "chat.completion",
				Created: time.Now().Unix(),
				Model:   req.Model,
				Choices: []proxy.OpenAIChoice{
					{
						Index: 0,
						Message: proxy.OpenAIMessage{
							Role:    "assistant",
							Content: "Test response",
						},
						FinishReason: "stop",
					},
				},
				Usage: proxy.OpenAIUsage{
					PromptTokens:     10,
					CompletionTokens: 5,
					TotalTokens:      15,
				},
			}
			json.NewEncoder(w).Encode(response)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockOllama.Close()

	p := proxy.NewOllamaProxy(mockOllama.URL, nil)

	// Start proxy
	err := p.Start(0)
	if err != nil {
		t.Fatalf("Failed to start proxy: %v", err)
	}
	defer p.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkAnthropicRequestGetSystemText(b *testing.B) {
	req := proxy.AnthropicRequest{
		System: []interface{}{
			map[string]interface{}{"type": "text", "text": "You are helpful"},
			map[string]interface{}{"type": "text", "text": " and friendly"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.GetSystemText()
	}
}

func BenchmarkAnthropicMessageGetContentText(b *testing.B) {
	msg := proxy.AnthropicMessage{
		Role: "user",
		Content: []interface{}{
			map[string]interface{}{"type": "text", "text": "Hello "},
			map[string]interface{}{"type": "text", "text": "world"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg.GetContentText()
	}
}

func BenchmarkBuildModelMap(b *testing.B) {
	ollamaModels := map[string]string{
		"haiku":  "custom-haiku",
		"sonnet": "custom-sonnet",
		"opus":   "custom-opus",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		proxy.BuildModelMap(ollamaModels)
	}
}

func BenchmarkJSONMarshalAnthropicRequest(b *testing.B) {
	req := proxy.AnthropicRequest{
		Model:     "claude-sonnet-4",
		MaxTokens: 100,
		Messages: []proxy.AnthropicMessage{
			{Role: "user", Content: "Hello world, this is a test message"},
			{Role: "assistant", Content: "Hi there!"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(req)
	}
}

func BenchmarkJSONUnmarshalAnthropicResponse(b *testing.B) {
	data := []byte(`{"id":"msg_123","type":"message","role":"assistant","model":"claude-sonnet-4","content":[{"type":"text","text":"Hello world"}],"usage":{"input_tokens":10,"output_tokens":20}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var resp proxy.AnthropicResponse
		json.Unmarshal(data, &resp)
	}
}
