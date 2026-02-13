// Package backend_test provides tests for the backend package.
package backend_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"nexus/internal/backend"
	"nexus/internal/config"
)

// ============================================================================
// Backend Struct Tests
// ============================================================================

func TestBackendCodingTiers(t *testing.T) {
	validTiers := map[string]bool{"S": true, "A": true, "B": true, "C": true}

	registry := backend.NewRegistry()
	backends := registry.GetAll()

	for name, be := range backends {
		if !validTiers[be.CodingTier] {
			t.Errorf("Backend %s has invalid coding tier: %s", name, be.CodingTier)
		}
	}
}

func TestBackendPricing(t *testing.T) {
	registry := backend.NewRegistry()
	backends := registry.GetAll()

	for name, be := range backends {
		if be.InputPrice < 0 {
			t.Errorf("Backend %s has negative input price: %f", name, be.InputPrice)
		}
		if be.OutputPrice < 0 {
			t.Errorf("Backend %s has negative output price: %f", name, be.OutputPrice)
		}
	}
}

func TestBackendRequiredFields(t *testing.T) {
	registry := backend.NewRegistry()
	backends := registry.GetAll()

	for name, be := range backends {
		if be.Name == "" {
			t.Errorf("Backend %s has empty Name field", name)
		}
		if be.DisplayName == "" {
			t.Errorf("Backend %s has empty DisplayName field", name)
		}
		if be.AuthVar == "" {
			t.Errorf("Backend %s has empty AuthVar field", name)
		}
		if be.Provider == "" {
			t.Errorf("Backend %s has empty Provider field", name)
		}
		if be.CodingTier == "" {
			t.Errorf("Backend %s has empty CodingTier field", name)
		}
	}
}

func TestBackendConsistency(t *testing.T) {
	registry := backend.NewRegistry()
	backends := registry.GetAll()

	// Ensure map key matches backend name
	for key, be := range backends {
		if be.Name != key {
			t.Errorf("Backend map key %s doesn't match Name field %s", key, be.Name)
		}
	}
}

// ============================================================================
// Registry Tests
// ============================================================================

func TestNewRegistry(t *testing.T) {
	registry := backend.NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry() returned nil")
	}
}

func TestRegistryGet(t *testing.T) {
	registry := backend.NewRegistry()

	// Test getting existing backend
	be, ok := registry.Get("claude")
	if !ok {
		t.Error("Expected to find 'claude' backend")
	}
	if be.Name != "claude" {
		t.Errorf("Expected Name='claude', got %q", be.Name)
	}

	// Test getting non-existent backend
	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("Expected not to find 'nonexistent' backend")
	}
}

func TestRegistryGetAll(t *testing.T) {
	registry := backend.NewRegistry()
	backends := registry.GetAll()

	expectedBackends := []string{
		"claude", "zai", "kimi", "deepseek", "gemini",
		"mistral", "groq", "together", "openrouter", "openai", "ollama",
	}

	for _, name := range expectedBackends {
		if _, ok := backends[name]; !ok {
			t.Errorf("Expected backend %s to be in registry", name)
		}
	}
}

func TestRegistryGetOrdered(t *testing.T) {
	registry := backend.NewRegistry()
	ordered := registry.GetOrdered()

	expected := []string{
		"claude", "openai", "deepseek", "gemini", "mistral",
		"zai", "kimi", "groq", "together", "openrouter", "ollama",
	}

	if len(ordered) != len(expected) {
		t.Errorf("Expected %d backends, got %d", len(expected), len(ordered))
	}

	for i, name := range expected {
		if i >= len(ordered) || ordered[i] != name {
			t.Errorf("Expected ordered[%d]=%s, got %s", i, name, ordered[i])
		}
	}
}

// ============================================================================
// Backend-Specific Tests
// ============================================================================

func TestOllamaBackend(t *testing.T) {
	registry := backend.NewRegistry()
	be, ok := registry.Get("ollama")
	if !ok {
		t.Fatal("Ollama backend not found")
	}

	if be.Name != "ollama" {
		t.Errorf("Expected Name='ollama', got %q", be.Name)
	}

	if be.DisplayName != "Ollama" {
		t.Errorf("Expected DisplayName='Ollama', got %q", be.DisplayName)
	}

	if be.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("Expected BaseURL='http://localhost:11434/v1', got %q", be.BaseURL)
	}

	if be.InputPrice != 0.00 || be.OutputPrice != 0.00 {
		t.Errorf("Expected $0.00 pricing for local Ollama, got $%.2f/$%.2f", be.InputPrice, be.OutputPrice)
	}

	if be.AuthVar != "OLLAMA_API_KEY" {
		t.Errorf("Expected AuthVar='OLLAMA_API_KEY', got %q", be.AuthVar)
	}

	if be.CodingTier != "B" {
		t.Errorf("Expected CodingTier='B', got %q", be.CodingTier)
	}
}

func TestClaudeBackend(t *testing.T) {
	registry := backend.NewRegistry()
	be, ok := registry.Get("claude")
	if !ok {
		t.Fatal("Claude backend not found")
	}

	if be.AuthVar != "ANTHROPIC_API_KEY" {
		t.Errorf("Expected AuthVar='ANTHROPIC_API_KEY', got %q", be.AuthVar)
	}

	if be.CodingTier != "S" {
		t.Errorf("Expected CodingTier='S', got %q", be.CodingTier)
	}

	if be.InputPrice != 3.00 {
		t.Errorf("Expected InputPrice=3.00, got %.2f", be.InputPrice)
	}

	if be.OutputPrice != 15.00 {
		t.Errorf("Expected OutputPrice=15.00, got %.2f", be.OutputPrice)
	}
}

func TestOpenAIBackend(t *testing.T) {
	registry := backend.NewRegistry()
	be, ok := registry.Get("openai")
	if !ok {
		t.Fatal("OpenAI backend not found")
	}

	if be.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("Expected BaseURL='https://api.openai.com/v1', got %q", be.BaseURL)
	}

	if be.HaikuModel != "gpt-4o-mini" {
		t.Errorf("Expected HaikuModel='gpt-4o-mini', got %q", be.HaikuModel)
	}

	if be.SonnetModel != "gpt-4o" {
		t.Errorf("Expected SonnetModel='gpt-4o', got %q", be.SonnetModel)
	}

	if be.OpusModel != "o1" {
		t.Errorf("Expected OpusModel='o1', got %q", be.OpusModel)
	}
}

func TestDeepSeekBackend(t *testing.T) {
	registry := backend.NewRegistry()
	be, ok := registry.Get("deepseek")
	if !ok {
		t.Fatal("DeepSeek backend not found")
	}

	if be.CodingTier != "S" {
		t.Errorf("Expected CodingTier='S', got %q", be.CodingTier)
	}

	if be.InputPrice != 0.27 {
		t.Errorf("Expected InputPrice=0.27, got %.2f", be.InputPrice)
	}

	if be.OutputPrice != 1.10 {
		t.Errorf("Expected OutputPrice=1.10, got %.2f", be.OutputPrice)
	}
}

// ============================================================================
// Health Check Tests
// ============================================================================

func TestRegistryCheckHealthNoAPIKey(t *testing.T) {
	registry := backend.NewRegistry()
	cfg := &config.Config{
		Keys: make(map[string]string),
	}

	// Test backend without API key (should skip)
	be := backend.Backend{
		Name:    "claude",
		AuthVar: "ANTHROPIC_API_KEY",
	}

	result := registry.CheckHealth(cfg, be)

	if result.Status != "skip" {
		t.Errorf("Expected status 'skip', got %q", result.Status)
	}

	if result.Message != "No API key configured" {
		t.Errorf("Expected message 'No API key configured', got %q", result.Message)
	}
}

func TestRegistryCheckHealthOllamaNoKey(t *testing.T) {
	registry := backend.NewRegistry()
	cfg := &config.Config{
		Keys: make(map[string]string),
	}

	// Test Ollama backend without API key (should not skip)
	be := backend.Backend{
		Name:    "ollama",
		AuthVar: "OLLAMA_API_KEY",
		BaseURL: "http://localhost:11434/v1",
	}

	result := registry.CheckHealth(cfg, be)

	// Should not skip due to missing key, but will likely error due to connection
	if result.Status == "skip" && result.Message == "No API key configured" {
		t.Error("Ollama should not skip health check without API key")
	}
}

func TestRegistryCheckHealthWithMockServer(t *testing.T) {
	// Create a mock API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object": "list"}`))
	}))
	defer mockServer.Close()

	registry := backend.NewRegistry()
	cfg := &config.Config{
		Keys: map[string]string{"TEST_API_KEY": "test-key"},
	}

	// Test backend with mock server
	be := backend.Backend{
		Name:    "test",
		AuthVar: "TEST_API_KEY",
		BaseURL: mockServer.URL,
	}

	result := registry.CheckHealth(cfg, be)

	if result.Status != "ok" {
		t.Errorf("Expected status 'ok', got %q (message: %s)", result.Status, result.Message)
	}

	if result.Latency <= 0 {
		t.Error("Expected positive latency")
	}
}

func TestRegistryCheckHealthErrorResponse(t *testing.T) {
	// Create a mock API server that returns error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer mockServer.Close()

	registry := backend.NewRegistry()
	cfg := &config.Config{
		Keys: map[string]string{"TEST_API_KEY": "test-key"},
	}

	be := backend.Backend{
		Name:    "test",
		AuthVar: "TEST_API_KEY",
		BaseURL: mockServer.URL,
	}

	result := registry.CheckHealth(cfg, be)

	if result.Status != "error" {
		t.Errorf("Expected status 'error', got %q", result.Status)
	}
}

func TestRegistryCheckHealthNoBaseURL(t *testing.T) {
	registry := backend.NewRegistry()
	cfg := &config.Config{
		Keys: map[string]string{"TEST_API_KEY": "test-key"},
	}

	// Test backend without BaseURL
	be := backend.Backend{
		Name:    "test",
		AuthVar: "TEST_API_KEY",
		BaseURL: "",
	}

	result := registry.CheckHealth(cfg, be)

	if result.Status != "skip" {
		t.Errorf("Expected status 'skip', got %q", result.Status)
	}

	if result.Message != "Health check not implemented" {
		t.Errorf("Expected 'Health check not implemented', got %q", result.Message)
	}
}

// ============================================================================
// State Manager Tests
// ============================================================================

func TestNewStateManager(t *testing.T) {
	cfg := &config.Config{
		StateFile: "/tmp/test-state",
	}

	sm := backend.NewStateManager(cfg)
	if sm == nil {
		t.Fatal("NewStateManager() returned nil")
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkRegistryGet(b *testing.B) {
	registry := backend.NewRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.Get("claude")
	}
}

func BenchmarkRegistryGetAll(b *testing.B) {
	registry := backend.NewRegistry()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		registry.GetAll()
	}
}
