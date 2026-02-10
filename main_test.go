// main_test.go - Test coverage for PromptOps
package main

import (
	"testing"
	"time"
)

func TestMaskKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"short", "abc", "****"},
		{"exactly_8", "abcdefgh", "****"},
		{"9_chars", "abcdefghi", "abcd****fghi"},
		{"16_chars", "abcdefghijklmnop", "abcd****mnop"},
		{"long_key", "sk-ant-api03-abc123def456", "sk-a****f456"},
		{"empty", "", "****"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskKey(tt.key)
			if result != tt.expected {
				t.Errorf("maskKey(%q) = %q, want %q", tt.key, result, tt.expected)
			}
		})
	}
}

func TestFormatCurrency(t *testing.T) {
	tests := []struct {
		amount   float64
		expected string
	}{
		{0, "$0.00"},
		{1.5, "$1.50"},
		{100.999, "$101.00"},
		{0.01, "$0.01"},
		{-5.5, "$-5.50"},
	}

	for _, tt := range tests {
		result := formatCurrency(tt.amount)
		if result != tt.expected {
			t.Errorf("formatCurrency(%f) = %q, want %q", tt.amount, result, tt.expected)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s        string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"exact", 5, "exact"},
		{"toolong", 5, "to..."},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.s, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, result, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{100 * time.Microsecond, "100us"},
		{500 * time.Millisecond, "500ms"},
		{1500 * time.Millisecond, "1.5s"},
		{5 * time.Second, "5.0s"},
		{0, "0us"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.d)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, result, tt.expected)
		}
	}
}

func TestGenerateSessionID(t *testing.T) {
	// Test that IDs are unique
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateSessionID("test")
		if ids[id] {
			t.Errorf("Duplicate session ID generated: %s", id)
		}
		ids[id] = true

		// Verify format: name-timestamp-hex
		if len(id) < 20 {
			t.Errorf("Session ID too short: %s", id)
		}
	}
}

func TestBackendCodingTiers(t *testing.T) {
	// Verify all backends have valid coding tiers
	validTiers := map[string]bool{"S": true, "A": true, "B": true, "C": true}

	for name, be := range backends {
		if !validTiers[be.CodingTier] {
			t.Errorf("Backend %s has invalid coding tier: %s", name, be.CodingTier)
		}
	}
}

func TestBackendPricing(t *testing.T) {
	// Verify all backends have non-negative pricing
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
	// Verify all backends have required fields
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
	}
}

func TestOllamaBackend(t *testing.T) {
	// Verify Ollama backend configuration
	be, ok := backends["ollama"]
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
}

func TestGetYoloModeOllama(t *testing.T) {
	cfg := &Config{
		YoloModeOllama: true,
	}

	if !cfg.getYoloMode("ollama") {
		t.Error("Expected getYoloMode('ollama') to return true when YoloModeOllama is true")
	}

	cfg.YoloModeOllama = false
	if cfg.getYoloMode("ollama") {
		t.Error("Expected getYoloMode('ollama') to return false when YoloModeOllama is false")
	}
}
