// Package main implements PromptOps - an AI Model Backend Switcher
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// AnthropicRequest represents an Anthropic API messages request
type AnthropicRequest struct {
	Model       string               `json:"model"`
	Messages    []AnthropicMessage   `json:"messages"`
	MaxTokens   int                  `json:"max_tokens,omitempty"`
	Temperature *float64             `json:"temperature,omitempty"`
	TopP        *float64             `json:"top_p,omitempty"`
	Stream      bool                 `json:"stream,omitempty"`
	System      interface{}          `json:"system,omitempty"` // Can be string or []AnthropicContentItem
}

// GetSystemText extracts text from system field, handling both string and array formats
func (r AnthropicRequest) GetSystemText() string {
	switch v := r.System.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			if contentMap, ok := item.(map[string]interface{}); ok {
				if text, ok := contentMap["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}

// AnthropicContentItem represents a content block in a message
type AnthropicContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// AnthropicMessage represents a message in the conversation
type AnthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // Can be string or []AnthropicContentItem
}

// GetContentText extracts text content from a message, handling both string and array formats
func (m AnthropicMessage) GetContentText() string {
	switch v := m.Content.(type) {
	case string:
		return v
	case []interface{}:
		var parts []string
		for _, item := range v {
			if contentMap, ok := item.(map[string]interface{}); ok {
				if text, ok := contentMap["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}

// AnthropicResponse represents an Anthropic API response
type AnthropicResponse struct {
	ID           string               `json:"id"`
	Type         string               `json:"type"`
	Role         string               `json:"role"`
	Model        string               `json:"model"`
	Content      []AnthropicContent   `json:"content"`
	StopReason   string               `json:"stop_reason,omitempty"`
	StopSequence string               `json:"stop_sequence,omitempty"`
	Usage        AnthropicUsage       `json:"usage"`
}

type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicStreamEvent represents a streaming event
type AnthropicStreamEvent struct {
	Type         string             `json:"type"`
	Message      *AnthropicResponse `json:"message,omitempty"`
	Index        int                `json:"index,omitempty"`
	ContentBlock *AnthropicContent  `json:"content_block,omitempty"`
	Delta        *AnthropicDelta    `json:"delta,omitempty"`
	StopReason   string             `json:"stop_reason,omitempty"`
	Usage        *AnthropicUsage    `json:"usage,omitempty"`
}

type AnthropicDelta struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// OpenAIRequest represents an OpenAI API chat completions request
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse represents an OpenAI API response
type OpenAIResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []OpenAIChoice   `json:"choices"`
	Usage   OpenAIUsage      `json:"usage"`
}

type OpenAIChoice struct {
	Index        int          `json:"index"`
	Message      OpenAIMessage `json:"message,omitempty"`
	Delta        *OpenAIMessage `json:"delta,omitempty"`
	FinishReason string       `json:"finish_reason"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIStreamEvent represents an OpenAI streaming event
type OpenAIStreamEvent struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
}

// OllamaProxy is the proxy server that translates Anthropic to OpenAI
type OllamaProxy struct {
	ollamaBaseURL string
	server        *http.Server
	modelMap      map[string]string
}

// NewOllamaProxy creates a new proxy instance
func NewOllamaProxy(ollamaBaseURL string, modelMap map[string]string) *OllamaProxy {
	if modelMap == nil {
		modelMap = map[string]string{
			"llama3.2":      "llama3.2:latest",
			"llama3.2:3b":   "llama3.2:3b",
			"codellama":     "codellama:latest",
			"phi3":          "phi3:latest",
			"mistral":       "mistral:latest",
			"llama3.3":      "llama3.3:latest",
		}
	}
	return &OllamaProxy{
		ollamaBaseURL: ollamaBaseURL,
		modelMap:      modelMap,
	}
}

// Start starts the proxy server on the given port
func (p *OllamaProxy) Start(port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/models", p.handleModels)
	mux.HandleFunc("/v1/messages", p.handleMessages)
	mux.HandleFunc("/", p.handleProxy)

	p.server = &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", port),
		Handler: mux,
	}

	// Start server in a goroutine
	go func() {
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Proxy server error: %v\n", err)
		}
	}()

	// Wait a moment for server to be ready
	time.Sleep(100 * time.Millisecond)
	return nil
}

// Stop stops the proxy server
func (p *OllamaProxy) Stop() error {
	if p.server != nil {
		return p.server.Close()
	}
	return nil
}

func (p *OllamaProxy) handleModels(w http.ResponseWriter, r *http.Request) {
	// Forward to Ollama's /v1/models endpoint
	resp, err := http.Get(p.ollamaBaseURL + "/models")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (p *OllamaProxy) handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read Anthropic request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var anthReq AnthropicRequest
	if err := json.Unmarshal(body, &anthReq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Map model name
	model := p.mapModel(anthReq.Model)

	// Build OpenAI request
	openaiReq := OpenAIRequest{
		Model:       model,
		MaxTokens:   anthReq.MaxTokens,
		Temperature: 0.7,
		TopP:        1.0,
		Stream:      anthReq.Stream,
	}

	if anthReq.Temperature != nil {
		openaiReq.Temperature = *anthReq.Temperature
	}
	if anthReq.TopP != nil {
		openaiReq.TopP = *anthReq.TopP
	}

	// Convert messages
	systemText := anthReq.GetSystemText()
	if systemText != "" {
		openaiReq.Messages = append(openaiReq.Messages, OpenAIMessage{
			Role:    "system",
			Content: systemText,
		})
	}

	for _, msg := range anthReq.Messages {
		role := msg.Role
		if role == "assistant" {
			role = "assistant"
		} else if role == "user" {
			role = "user"
		}
		openaiReq.Messages = append(openaiReq.Messages, OpenAIMessage{
			Role:    role,
			Content: msg.GetContentText(),
		})
	}

	// Send to Ollama
	openaiBody, err := json.Marshal(openaiReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if anthReq.Stream {
		p.handleStreaming(w, r, openaiBody)
	} else {
		p.handleNonStreaming(w, openaiBody, anthReq.Model)
	}
}

func (p *OllamaProxy) handleStreaming(w http.ResponseWriter, r *http.Request, openaiBody []byte) {
	req, err := http.NewRequest("POST", p.ollamaBaseURL+"/chat/completions", bytes.NewReader(openaiBody))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 0} // No timeout for streaming
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send message_start event
	msgStart := AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    generateID(),
			Type:  "message",
			Role:  "assistant",
			Model: "unknown",
			Content: []AnthropicContent{},
			Usage: AnthropicUsage{},
		},
	}
	writeSSE(w, msgStart)
	flusher.Flush()

	// Send content_block_start
	blockStart := AnthropicStreamEvent{
		Type:  "content_block_start",
		Index: 0,
		ContentBlock: &AnthropicContent{
			Type: "text",
			Text: "",
		},
	}
	writeSSE(w, blockStart)
	flusher.Flush()

	// Process OpenAI stream
	scanner := bufio.NewScanner(resp.Body)
	contentIndex := 0
	var fullContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var streamEvent OpenAIStreamEvent
		if err := json.Unmarshal([]byte(data), &streamEvent); err != nil {
			continue
		}

		if len(streamEvent.Choices) > 0 && streamEvent.Choices[0].Delta != nil {
			text := streamEvent.Choices[0].Delta.Content
			if text != "" {
				fullContent.WriteString(text)
				delta := AnthropicStreamEvent{
					Type:  "content_block_delta",
					Index: contentIndex,
					Delta: &AnthropicDelta{
						Type: "text_delta",
						Text: text,
					},
				}
				writeSSE(w, delta)
				flusher.Flush()
			}
		}
	}

	// Send content_block_stop
	blockStop := AnthropicStreamEvent{
		Type:  "content_block_stop",
		Index: contentIndex,
	}
	writeSSE(w, blockStop)
	flusher.Flush()

	// Send message_stop
	msgStop := AnthropicStreamEvent{
		Type: "message_stop",
	}
	writeSSE(w, msgStop)
	flusher.Flush()
}

func (p *OllamaProxy) handleNonStreaming(w http.ResponseWriter, openaiBody []byte, originalModel string) {
	req, err := http.NewRequest("POST", p.ollamaBaseURL+"/chat/completions", bytes.NewReader(openaiBody))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var openaiResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to Anthropic response
	anthResp := AnthropicResponse{
		ID:    generateID(),
		Type:  "message",
		Role:  "assistant",
		Model: originalModel,
		Usage: AnthropicUsage{
			InputTokens:  openaiResp.Usage.PromptTokens,
			OutputTokens: openaiResp.Usage.CompletionTokens,
		},
	}

	if len(openaiResp.Choices) > 0 {
		content := openaiResp.Choices[0].Message.Content
		anthResp.Content = []AnthropicContent{
			{Type: "text", Text: content},
		}
		if openaiResp.Choices[0].FinishReason == "stop" {
			anthResp.StopReason = "end_turn"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(anthResp)
}

func (p *OllamaProxy) handleProxy(w http.ResponseWriter, r *http.Request) {
	// Proxy all other requests to Ollama
	url := p.ollamaBaseURL + r.URL.Path
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req, err := http.NewRequest(r.Method, url, bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (p *OllamaProxy) mapModel(model string) string {
	// Check if we have a direct mapping
	if mapped, ok := p.modelMap[model]; ok {
		return mapped
	}
	// Return as-is if no mapping found
	return model
}

func generateID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

func writeSSE(w http.ResponseWriter, event AnthropicStreamEvent) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "data: %s\n\n", data)
}
