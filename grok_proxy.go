package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// GrokProxy patches Claude Code requests and responses for xAI compatibility.
//
// Request patches:
//   - Adds "required":[] to object schemas missing it (xAI strict validation)
//   - Rewrites "additionalProperties":{} to false
//
// Response patches:
//   - Strips "thinking" content blocks from streaming SSE responses
//   - Strips "thinking" content blocks from non-streaming JSON responses
//
// This allows reasoning models like grok-code-fast-1 to work with Claude Code.
type GrokProxy struct {
	targetBaseURL string
	apiKey        string
	server        *http.Server
}

func NewGrokProxy(targetBaseURL, apiKey string) *GrokProxy {
	return &GrokProxy{
		targetBaseURL: targetBaseURL,
		apiKey:        apiKey,
	}
}

func (p *GrokProxy) Start(port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handle)

	p.server = &http.Server{
		Addr:         fmt.Sprintf("localhost:%d", port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // no timeout for streaming
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Grok proxy error: %v\n", err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	return nil
}

func (p *GrokProxy) Stop() error {
	if p.server != nil {
		return p.server.Close()
	}
	return nil
}

func (p *GrokProxy) handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Patch the request body to fix tool schemas
	if r.Method == http.MethodPost && len(body) > 0 {
		body = patchToolSchemas(body)
	}

	// Forward to xAI
	url := p.targetBaseURL + r.URL.Path
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}

	req, err := http.NewRequest(r.Method, url, bytes.NewReader(body))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers, skip Content-Length (body size changes after patching)
	// and Host (must match target)
	for key, values := range r.Header {
		lower := strings.ToLower(key)
		if lower == "content-length" || lower == "host" {
			continue
		}
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Set("X-Api-Key", p.apiKey)
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.ContentLength = int64(len(body))

	client := &http.Client{
		Timeout: 0, // no timeout for streaming
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12},
			DisableCompression: true,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	ct := resp.Header.Get("Content-Type")
	isSSE := strings.Contains(ct, "text/event-stream")

	if isSSE {
		// Streaming: filter out thinking blocks from SSE events
		w.WriteHeader(resp.StatusCode)
		p.filterSSEThinking(w, resp.Body)
	} else if resp.StatusCode == http.StatusOK && strings.Contains(ct, "application/json") {
		// Non-streaming JSON: strip thinking from content array
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			w.WriteHeader(resp.StatusCode)
			return
		}
		respBody = stripThinkingFromJSON(respBody)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(respBody)))
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
	} else {
		// Pass through as-is (errors, other content types)
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

// filterSSEThinking reads SSE events from the upstream response and writes
// them to the client, skipping any events related to thinking content blocks.
func (p *GrokProxy) filterSSEThinking(w http.ResponseWriter, body io.Reader) {
	flusher, canFlush := w.(http.Flusher)
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 256*1024), 256*1024) // handle large events

	inThinkingBlock := false
	var eventLines []string

	for scanner.Scan() {
		line := scanner.Text()

		// Accumulate lines until we hit a blank line (end of SSE event)
		if line != "" {
			eventLines = append(eventLines, line)
			continue
		}

		// Blank line = end of event, process it
		if len(eventLines) == 0 {
			continue
		}

		shouldSkip := false
		for _, eLine := range eventLines {
			if !strings.HasPrefix(eLine, "data: ") {
				continue
			}
			data := strings.TrimPrefix(eLine, "data: ")

			var event map[string]interface{}
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			eventType, _ := event["type"].(string)

			switch eventType {
			case "content_block_start":
				if cb, ok := event["content_block"].(map[string]interface{}); ok {
					if cbType, _ := cb["type"].(string); cbType == "thinking" {
						inThinkingBlock = true
						shouldSkip = true
					}
				}
			case "content_block_delta":
				if inThinkingBlock {
					shouldSkip = true
				}
			case "content_block_stop":
				if inThinkingBlock {
					inThinkingBlock = false
					shouldSkip = true
				}
			}
		}

		if !shouldSkip {
			for _, eLine := range eventLines {
				fmt.Fprintf(w, "%s\n", eLine)
			}
			fmt.Fprint(w, "\n")
			if canFlush {
				flusher.Flush()
			}
		}

		eventLines = eventLines[:0]
	}

	// Flush any remaining lines
	if len(eventLines) > 0 {
		for _, eLine := range eventLines {
			fmt.Fprintf(w, "%s\n", eLine)
		}
		fmt.Fprint(w, "\n")
		if canFlush {
			flusher.Flush()
		}
	}
}

// stripThinkingFromJSON removes thinking content blocks from a non-streaming
// Anthropic API response.
func stripThinkingFromJSON(body []byte) []byte {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return body
	}
	content, ok := resp["content"].([]interface{})
	if !ok {
		return body
	}
	var filtered []interface{}
	for _, item := range content {
		if block, ok := item.(map[string]interface{}); ok {
			if blockType, _ := block["type"].(string); blockType == "thinking" {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	resp["content"] = filtered
	patched, err := json.Marshal(resp)
	if err != nil {
		return body
	}
	return patched
}

// patchToolSchemas fixes tool input schemas for xAI compatibility.
func patchToolSchemas(body []byte) []byte {
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}
	fixSchemaRequired(raw)
	patched, err := json.Marshal(raw)
	if err != nil {
		return body
	}
	return patched
}

func fixSchemaRequired(v interface{}) {
	switch val := v.(type) {
	case map[string]interface{}:
		_, hasProps := val["properties"]
		typeVal, _ := val["type"].(string)
		if hasProps || typeVal == "object" {
			req, exists := val["required"]
			if !exists || req == nil {
				val["required"] = []interface{}{}
			}
		}
		if ap, ok := val["additionalProperties"]; ok {
			if apMap, isMap := ap.(map[string]interface{}); isMap && len(apMap) == 0 {
				val["additionalProperties"] = false
			}
		}
		for _, child := range val {
			fixSchemaRequired(child)
		}
	case []interface{}:
		for _, item := range val {
			fixSchemaRequired(item)
		}
	}
}
