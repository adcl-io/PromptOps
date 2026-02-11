package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestOllamaProxy(t *testing.T) {
	// Start proxy
	proxy := NewOllamaProxy("http://localhost:11434/v1", nil)
	if err := proxy.Start(18081); err != nil {
		t.Fatalf("Failed to start proxy: %v", err)
	}
	defer proxy.Stop()

	// Wait for server to be ready
	time.Sleep(200 * time.Millisecond)

	// Test /v1/models endpoint
	t.Run("models endpoint", func(t *testing.T) {
		resp, err := http.Get("http://localhost:18081/v1/models")
		if err != nil {
			t.Fatalf("Failed to get models: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		t.Logf("Models response: %s", string(body))
	})

	// Test /v1/messages endpoint (non-streaming)
	t.Run("messages endpoint", func(t *testing.T) {
		anthReq := AnthropicRequest{
			Model:     "llama3.2:latest",
			MaxTokens: 50,
			Messages: []AnthropicMessage{
				{Role: "user", Content: "Say hello"},
			},
		}

		body, _ := json.Marshal(anthReq)
		resp, err := http.Post("http://localhost:18081/v1/messages", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Failed to post message: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
			return
		}

		var anthResp AnthropicResponse
		if err := json.NewDecoder(resp.Body).Decode(&anthResp); err != nil {
			t.Errorf("Failed to decode response: %v", err)
			return
		}

		if anthResp.Type != "message" {
			t.Errorf("Expected type 'message', got '%s'", anthResp.Type)
		}

		if len(anthResp.Content) == 0 {
			t.Error("Expected non-empty content")
		} else {
			t.Logf("Response content: %s", anthResp.Content[0].Text)
		}
	})

	// Test array content format (like Claude Code sends)
	t.Run("array content format", func(t *testing.T) {
		// Send raw JSON with array content
		rawJSON := `{
			"model": "llama3.2:latest",
			"max_tokens": 50,
			"messages": [
				{
					"role": "user",
					"content": [
						{"type": "text", "text": "Say hello"}
					]
				}
			]
		}`

		resp, err := http.Post("http://localhost:18081/v1/messages", "application/json", bytes.NewReader([]byte(rawJSON)))
		if err != nil {
			t.Fatalf("Failed to post message: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Expected status 200, got %d. Body: %s", resp.StatusCode, string(body))
			return
		}

		var anthResp AnthropicResponse
		if err := json.NewDecoder(resp.Body).Decode(&anthResp); err != nil {
			t.Errorf("Failed to decode response: %v", err)
			return
		}

		if anthResp.Type != "message" {
			t.Errorf("Expected type 'message', got '%s'", anthResp.Type)
		}

		if len(anthResp.Content) == 0 {
			t.Error("Expected non-empty content")
		} else {
			t.Logf("Response content: %s", anthResp.Content[0].Text)
		}
	})
}

func TestAnthropicRequestGetSystemText(t *testing.T) {
	tests := []struct {
		name     string
		request  AnthropicRequest
		expected string
	}{
		{
			name:     "string system",
			request:  AnthropicRequest{System: "You are helpful"},
			expected: "You are helpful",
		},
		{
			name: "array system with text",
			request: AnthropicRequest{
				System: []interface{}{
					map[string]interface{}{"type": "text", "text": "You are helpful"},
				},
			},
			expected: "You are helpful",
		},
		{
			name:     "nil system",
			request:  AnthropicRequest{System: nil},
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
		message  AnthropicMessage
		expected string
	}{
		{
			name:     "string content",
			message:  AnthropicMessage{Role: "user", Content: "hello"},
			expected: "hello",
		},
		{
			name: "array content with text",
			message: AnthropicMessage{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "hello"},
				},
			},
			expected: "hello",
		},
		{
			name: "array content with multiple items",
			message: AnthropicMessage{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{"type": "text", "text": "hello "},
					map[string]interface{}{"type": "text", "text": "world"},
				},
			},
			expected: "hello world",
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
