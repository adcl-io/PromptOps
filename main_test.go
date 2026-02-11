// main_test.go - Test coverage for PromptOps
package main

import (
	"strings"
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
		YoloModes: map[string]bool{
			"ollama": true,
		},
	}

	if !cfg.getYoloMode("ollama") {
		t.Error("Expected getYoloMode('ollama') to return true when YoloModes['ollama'] is true")
	}

	cfg.YoloModes["ollama"] = false
	if cfg.getYoloMode("ollama") {
		t.Error("Expected getYoloMode('ollama') to return false when YoloModes['ollama'] is false")
	}
}

func TestGetVersion(t *testing.T) {
	// Test that getVersion returns version when buildVersion is empty
	originalVersion := version
	originalBuildVersion := buildVersion
	defer func() {
		version = originalVersion
		buildVersion = originalBuildVersion
	}()

	version = "dev"
	buildVersion = ""
	if got := getVersion(); got != "dev" {
		t.Errorf("Expected getVersion() = 'dev' when buildVersion is empty, got %q", got)
	}

	// Test that getVersion prefers buildVersion when set
	buildVersion = "2.5.0"
	if got := getVersion(); got != "2.5.0" {
		t.Errorf("Expected getVersion() = '2.5.0' when buildVersion is set, got %q", got)
	}
}

func TestTruncateUnicode(t *testing.T) {
	// Test Unicode string truncation
	unicodeStr := "Hello世界这是一个很长的字符串"
	truncated := truncate(unicodeStr, 10)
	if len([]rune(truncated)) != 10 {
		t.Errorf("Expected truncated string to have 10 runes, got %d", len([]rune(truncated)))
	}
	if !strings.HasSuffix(truncated, "...") {
		t.Error("Expected truncated string to end with '...'")
	}

	// Test that we don't truncate short strings
	shortUnicode := "Hello世界"
	if truncate(shortUnicode, 10) != shortUnicode {
		t.Error("Expected short Unicode string to not be truncated")
	}

	// Test mixed ASCII and Unicode
	mixed := "Test测试Test测试Test"
	truncated = truncate(mixed, 12)
	if len([]rune(truncated)) != 12 {
		t.Errorf("Expected truncated mixed string to have 12 runes, got %d", len([]rune(truncated)))
	}
}

func TestOllamaEnvVarsWhitelisted(t *testing.T) {
	// Verify Ollama-specific environment variables are in the whitelist
	ollamaVars := []string{
		"OLLAMA_API_KEY",
		"OLLAMA_HAIKU_MODEL",
		"OLLAMA_SONNET_MODEL",
		"OLLAMA_OPUS_MODEL",
	}

	for _, v := range ollamaVars {
		if !allowedEnvVars[v] {
			t.Errorf("Ollama environment variable %s is not whitelisted in allowedEnvVars", v)
		}
	}
}

func TestFilterEnvironmentAllowsOllamaVars(t *testing.T) {
	// Test that filterEnvironment correctly passes Ollama variables
	testEnv := []string{
		"PATH=/usr/bin",
		"HOME=/home/user",
		"OLLAMA_HAIKU_MODEL=llama3.2",
		"OLLAMA_SONNET_MODEL=codellama",
		"OLLAMA_OPUS_MODEL=llama3.3",
		"ANTHROPIC_API_KEY=sk-ant-test123",
		"SOME_OTHER_VAR=should_be_filtered",
	}

	filtered := filterEnvironment(testEnv)

	// Build a map for easier checking
	filteredMap := make(map[string]string)
	for _, e := range filtered {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			filteredMap[parts[0]] = parts[1]
		}
	}

	// Check that Ollama vars are preserved
	if filteredMap["OLLAMA_HAIKU_MODEL"] != "llama3.2" {
		t.Errorf("OLLAMA_HAIKU_MODEL should be preserved, got %q", filteredMap["OLLAMA_HAIKU_MODEL"])
	}
	if filteredMap["OLLAMA_SONNET_MODEL"] != "codellama" {
		t.Errorf("OLLAMA_SONNET_MODEL should be preserved, got %q", filteredMap["OLLAMA_SONNET_MODEL"])
	}
	if filteredMap["OLLAMA_OPUS_MODEL"] != "llama3.3" {
		t.Errorf("OLLAMA_OPUS_MODEL should be preserved, got %q", filteredMap["OLLAMA_OPUS_MODEL"])
	}

	// Check that standard vars are preserved
	if filteredMap["PATH"] != "/usr/bin" {
		t.Error("PATH should be preserved")
	}

	// Check that non-whitelisted vars are filtered
	if _, exists := filteredMap["SOME_OTHER_VAR"]; exists {
		t.Error("SOME_OTHER_VAR should be filtered out")
	}
}
